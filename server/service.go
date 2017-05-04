package main

import (
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/DimensionDataResearch/go-dd-cloud-compute/compute"
	"github.com/miekg/dns"
	"github.com/spf13/viper"

	dhcp "github.com/krolaw/dhcp4"
)

// Service represents the state for the DHCP service.
type Service struct {
	McpUser     string
	McpPassword string
	McpRegion   string

	InterfaceName string

	Client        *compute.Client
	NetworkDomain *compute.NetworkDomain
	VLAN          *compute.VLAN

	ServiceIP   net.IP
	DHCPOptions dhcp.Options

	EnableIPXE     bool
	IPXEPort       int
	TFTPServerName string
	PXEBootImage   string // PXE boot file (TFTP)
	IPXEBootScript string // iPXE boot script (HTTP)

	ServerMetadataByMACAddress     map[string]ServerMetadata
	StaticReservationsByMACAddress map[string]StaticReservation

	EnableDNS          bool
	DNSPort            int
	DNSSuffix          string
	DNSData            DNSData
	DNSFallbackAddress string
	dnsFallbackClient  *dns.Client

	LeasesByMACAddress map[string]*Lease
	LeaseDuration      time.Duration

	EnableDebugLogging bool

	listeners     *ServiceListeners
	stateLock     *sync.Mutex
	refreshTimer  *time.Ticker
	cancelRefresh chan bool
}

// NewService creates new Service state.
func NewService() *Service {
	service := &Service{
		ServerMetadataByMACAddress:     make(map[string]ServerMetadata),
		StaticReservationsByMACAddress: make(map[string]StaticReservation),
		LeasesByMACAddress:             make(map[string]*Lease),
		LeaseDuration:                  24 * time.Hour,
		DNSData:                        NewDNSData(),
		DHCPOptions: dhcp.Options{
			dhcp.OptionDomainNameServer: []byte{8, 8, 8, 8},
		},
		stateLock: &sync.Mutex{},
	}
	service.listeners = NewServiceListeners(service)

	return service
}

// Initialize the service configuration.
func (service *Service) Initialize() error {
	// Defaults
	viper.SetDefault("debug", false)
	viper.SetDefault("dns.enable", false)
	viper.SetDefault("dns.port", 53)
	viper.SetDefault("dns.suffix", "mcp.")
	viper.SetDefault("dns.forward_to.address", "8.8.8.8")
	viper.SetDefault("dns.forward_to.port", 53)
	viper.SetDefault("ipxe.enable", false)
	viper.SetDefault("ipxe.port", 4777)
	viper.SetDefault("ipxe.boot_image", "undionly.kpxe")

	// Environment variables.
	viper.BindEnv("MCP_USER", "mcp.user")
	viper.BindEnv("MCP_PASSWORD", "mcp.password")
	viper.BindEnv("MCP_REGION", "mcp.region")
	viper.BindEnv("MCP_DHCP_DEBUG", "debug")
	viper.BindEnv("MCP_DHCP_INTERFACE", "network.interface")
	viper.BindEnv("MCP_DHCP_VLAN_ID", "network.vlan_id")
	viper.BindEnv("MCP_DHCP_SERVICE_IP", "network.service_ip")
	viper.BindEnv("MCP_DNS_ENABLE", "dns.enable")
	viper.BindEnv("MCP_DNS_SUFFIX", "dns.suffix")
	viper.BindEnv("MCP_DNS_PORT", "dns.port")
	viper.BindEnv("MCP_DNS_FORWARD_TO", "dns.forward_to.address")
	viper.BindEnv("MCP_DNS_FORWARD_TO_PORT", "dns.forward_to.port")
	viper.BindEnv("MCP_IPXE_ENABLE", "ipxe.enable")
	viper.BindEnv("MCP_IPXE_PORT", "ipxe.port")
	viper.BindEnv("MCP_IPXE_BOOT_IMAGE", "ipxe.boot_image")
	viper.BindEnv("MCP_IPXE_BOOT_SCRIPT", "ipxe.boot_script")

	viper.SetConfigType("yaml")
	viper.SetConfigName("mcp2-dhcp-server")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc")

	err := viper.ReadInConfig()
	if err != nil {
		panic(err)
	}

	service.EnableDebugLogging = viper.GetBool("debug")

	service.McpRegion = viper.GetString("mcp.region")
	service.McpUser = viper.GetString("mcp.user")
	service.McpPassword = viper.GetString("mcp.password")
	service.Client = compute.NewClient(service.McpRegion, service.McpUser, service.McpPassword)

	vlanID := viper.GetString("network.vlan_id")
	service.VLAN, err = service.Client.GetVLAN(vlanID)
	if err != nil {
		return err
	} else if service.VLAN == nil {
		return fmt.Errorf("Cannot find VLAN with Id '%s'", vlanID)
	}
	service.NetworkDomain, err = service.Client.GetNetworkDomain(service.VLAN.NetworkDomain.ID)
	if err != nil {
		return err
	} else if service.NetworkDomain == nil {
		return fmt.Errorf("Cannot find network domain with Id '%s'", service.VLAN.NetworkDomain.ID)
	}

	vlanCIDR := fmt.Sprintf("%s/%d",
		service.VLAN.IPv4Range.BaseAddress,
		service.VLAN.IPv4Range.PrefixSize,
	)
	_, vlanNetwork, err := net.ParseCIDR(vlanCIDR)
	if err != nil {
		return err
	}

	// Subnet mask and default gateway
	service.DHCPOptions[dhcp.OptionSubnetMask] = vlanNetwork.Mask
	service.DHCPOptions[dhcp.OptionRouter] = net.ParseIP(service.VLAN.IPv4GatewayAddress).To4()

	service.ServiceIP = net.ParseIP(
		viper.GetString("network.service_ip"),
	).To4()

	service.InterfaceName = viper.GetString("network.interface")
	if len(service.InterfaceName) == 0 {
		return fmt.Errorf("network.interface / MCP_DHCP_INTERFACE is required")
	}

	service.EnableDNS = viper.GetBool("dns.enable")
	if service.EnableDNS {
		service.DNSPort = viper.GetInt("dns.port")
		if service.DNSPort < 53 {
			return fmt.Errorf("dns.port (%d) is invalid", service.DNSPort)
		}

		service.DNSSuffix = viper.GetString("dns.suffix")
		if len(service.DNSSuffix) == 0 {
			return fmt.Errorf("dns.suffix / MCP_DNS_SUFFIX is optional, but cannot be empty")
		}
		service.DNSSuffix = dns.Fqdn(service.DNSSuffix) // Ensure trailing "."

		fallbackAddress := viper.GetString("dns.forward_to.address")
		if len(fallbackAddress) == 0 {
			return fmt.Errorf("dns.forward_to.address / MCP_DNS_FORWARD is optional, but cannot be empty")
		}

		fallbackPort := viper.GetInt("dns.forward_to.port")
		if fallbackPort == 0 {
			return fmt.Errorf("dns.forward_to.port / MCP_DNS_FORWARD_PORT is optional, but cannot be empty")
		}

		service.DNSFallbackAddress = fmt.Sprintf("%s:%d",
			fallbackAddress, fallbackPort,
		)

		service.dnsFallbackClient = &dns.Client{}
	}

	service.EnableIPXE = viper.GetBool("ipxe.enable")
	if service.EnableIPXE {
		service.IPXEPort = viper.GetInt("ipxe.port")
		service.TFTPServerName = service.ServiceIP.String()

		service.PXEBootImage = viper.GetString("ipxe.boot_image")
		if len(service.PXEBootImage) == 0 {
			return fmt.Errorf("ipxe.boot_image / MCP_IPXE_BOOT_IMAGE must be set if ipxe.enable / MCP_IPXE_ENABLE is true")
		}

		service.IPXEBootScript = viper.GetString("ipxe.boot_script")
		if len(service.IPXEBootScript) == 0 {
			return fmt.Errorf("ipxe.boot_script / MCP_IPXE_BOOT_SCRIPT must be set if ipxe.enable / MCP_IPXE_ENABLE is true")
		}
	}

	// Static reservations (for testing only)
	staticReservationsValue := viper.Get("network.static_reservations")
	if staticReservationsValue != nil {
		staticReservations := staticReservationsValue.([]interface{})
		for _, staticReservationValue := range staticReservations {
			staticReservation := staticReservationValue.(map[interface{}]interface{})

			reservation := StaticReservation{
				MACAddress: strings.ToLower(
					staticReservation["mac"].(string),
				),
				HostName: staticReservation["name"].(string),
				IPAddress: net.ParseIP(
					staticReservation["ipv4"].(string),
				),
			}
			fmt.Printf("Adding static IP reservation for %s (%s): %s\n",
				reservation.MACAddress,
				reservation.HostName,
				reservation.IPAddress,
			)
			service.StaticReservationsByMACAddress[reservation.MACAddress] = reservation
		}
	} else {
		fmt.Printf("No static reservations.\n")
	}

	// Ignore IP range if we have static reservations.
	if len(service.StaticReservationsByMACAddress) == 0 {
		if !vlanNetwork.Contains(service.ServiceIP) {
			return fmt.Errorf("Service IP address %s does not lie within the IP network (%s) of the target VLAN ('%s')",
				service.ServiceIP.String(),
				vlanCIDR,
				service.VLAN.Name,
			)
		}
	}

	err = service.listeners.Initialize()
	if err != nil {
		return err
	}

	go service.logListenerErrors()

	return nil
}

// Start the service.
func (service *Service) Start() error {
	service.acquireStateLock("Start")
	defer service.releaseStateLock("Start")

	log.Printf("Initialising CloudControl metadata cache...")
	err := service.refreshServerMetadataInternal(false /* we already have the state lock */)
	if err != nil {
		log.Printf("Error refreshing servers: %s",
			err.Error(),
		)
	}

	log.Printf("All caches initialised.")

	service.cancelRefresh = make(chan bool, 1)
	service.refreshTimer = time.NewTicker(30 * time.Second)

	go func() {
		cancelRefresh := service.cancelRefresh
		refreshTimer := service.refreshTimer.C

		for {
			select {
			case <-cancelRefresh:
				return // Stopped

			case <-refreshTimer:
				if service.EnableDebugLogging {
					log.Printf("Refreshing server MAC addresses...")
				}

				err := service.RefreshServerMetadata()
				if err != nil {
					log.Printf("Error refreshing servers: %s",
						err.Error(),
					)
				}

				if service.EnableDebugLogging {
					log.Printf("Refreshed server MAC addresses.")
				}
			}
		}
	}()

	err = service.listeners.Start()
	if err != nil {
		return fmt.Errorf("failed to start service listeners: %s",
			err.Error(),
		)
	}

	return nil
}

// Stop the service.
func (service *Service) Stop() error {
	service.acquireStateLock("Stop")
	defer service.releaseStateLock("Stop")

	err := service.listeners.Stop()
	if err != nil {
		return err
	}

	if service.cancelRefresh != nil {
		service.cancelRefresh <- true
	}
	service.cancelRefresh = nil

	service.refreshTimer.Stop()
	service.refreshTimer = nil

	return nil
}

func (service *Service) logListenerErrors() {
	for {
		err := <-service.listeners.Errors

		if service.EnableDebugLogging {
			log.Printf("Listener error: %#v", err.Error())
		} else {
			log.Printf("Listener error: %s", err.Error())
		}
	}
}

func (service *Service) acquireStateLock(reason string) {
	if service.EnableDebugLogging {
		log.Printf("Acquire state lock (%s).", reason)
	}

	service.stateLock.Lock()
}

func (service *Service) releaseStateLock(reason string) {
	if service.EnableDebugLogging {
		log.Printf("Release state lock (%s).", reason)
	}

	service.stateLock.Unlock()
}
