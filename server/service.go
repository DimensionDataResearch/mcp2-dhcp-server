package main

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/DimensionDataResearch/go-dd-cloud-compute/compute"
	"github.com/spf13/viper"

	"strings"

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
	TFTPServerName string
	PXEBootImage   string // PXE boot file (TFTP)
	IPXEBootScript string // iPXE boot script (HTTP)

	ServerMetadataByMACAddress     map[string]ServerMetadata
	StaticReservationsByMACAddress map[string]StaticReservation

	LeasesByMACAddress map[string]*Lease
	LeaseDuration      time.Duration

	EnableDebugLogging bool

	stateLock     *sync.Mutex
	refreshTimer  *time.Ticker
	cancelRefresh chan bool
}

// NewService creates new Service state.
func NewService() *Service {
	return &Service{
		ServerMetadataByMACAddress:     make(map[string]ServerMetadata),
		StaticReservationsByMACAddress: make(map[string]StaticReservation),
		LeasesByMACAddress:             make(map[string]*Lease),
		LeaseDuration:                  24 * time.Hour,
		DHCPOptions: dhcp.Options{
			dhcp.OptionDomainNameServer: []byte{8, 8, 8, 8},
		},
		stateLock: &sync.Mutex{},
	}
}

// Initialize performs initial configuration of the Service.
func (service *Service) Initialize() error {
	// Defaults
	viper.SetDefault("debug", false)
	viper.SetDefault("ipxe.enable", false)
	viper.SetDefault("ipxe.boot_image", "undionly.kpxe")

	// Environment variables.
	viper.BindEnv("MCP_USER", "mcp.user")
	viper.BindEnv("MCP_PASSWORD", "mcp.password")
	viper.BindEnv("MCP_REGION", "mcp.region")
	viper.BindEnv("MCP_DHCP_DEBUG", "debug")
	viper.BindEnv("MCP_DHCP_INTERFACE", "network.interface")
	viper.BindEnv("MCP_DHCP_VLAN_ID", "network.vlan_id")
	viper.BindEnv("MCP_DHCP_SERVICE_IP", "network.service_ip")
	viper.BindEnv("MCP_IPXE_ENABLE", "ipxe.enable")
	viper.BindEnv("MCP_IPXE_BOOT_IMAGE", "ipxe.boot_image")
	viper.BindEnv("MCP_IPXE_BOOT_SCRIPT", "ipxe.boot_script")

	viper.SetConfigType("yaml")
	viper.AddConfigPath("./mcp2-dhcp-server.yml")
	viper.AddConfigPath("/etc/mcp2-dhcp-server.yml")

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

	service.DHCPOptions[dhcp.OptionSubnetMask] = vlanNetwork.Mask

	service.ServiceIP = net.ParseIP(
		viper.GetString("network.service_ip"),
	).To4()

	service.InterfaceName = viper.GetString("network.interface")
	if len(service.InterfaceName) == 0 {
		return fmt.Errorf("network.interface / MCP_DHCP_INTERFACE is required")
	}

	service.EnableIPXE = viper.GetBool("ipxe.enable")
	if service.EnableIPXE {
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

	return nil
}

// Start polling CloudControl for server metadata.
func (service *Service) Start() {
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
}

// Stop polling CloudControl for server metadata.
func (service *Service) Stop() {
	service.acquireStateLock("Stop")
	defer service.releaseStateLock("Stop")

	if service.cancelRefresh != nil {
		service.cancelRefresh <- true
	}
	service.cancelRefresh = nil

	service.refreshTimer.Stop()
	service.refreshTimer = nil
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
