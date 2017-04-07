package main

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/DimensionDataResearch/go-dd-cloud-compute/compute"
	"github.com/mostlygeek/arp"
	"github.com/spf13/viper"

	dhcp "github.com/krolaw/dhcp4"
)

// Service represents the state for the DHCP service.
type Service struct {
	McpUser     string
	McpPassword string
	McpRegion   string

	Client        *compute.Client
	NetworkDomain *compute.NetworkDomain
	VLAN          *compute.VLAN

	ServiceIP   net.IP
	DHCPOptions dhcp.Options

	ServersByMACAddress map[string]compute.Server

	DHCPRangeStart net.IP
	DHCPRangeEnd   net.IP

	LeasesByMACAddress map[string]*Lease
	LeasesByIP         map[string]*Lease
	LeaseDuration      time.Duration

	stateLock     *sync.Mutex
	refreshTimer  *time.Timer
	cancelRefresh chan bool
}

// NewService creates new Service state.
func NewService() *Service {
	return &Service{
		ServersByMACAddress: make(map[string]compute.Server),
		LeasesByMACAddress:  make(map[string]*Lease),
		LeaseDuration:       24 * time.Hour,
		DHCPOptions: dhcp.Options{
			dhcp.OptionSubnetMask: []byte{255, 255, 255, 0}, // TODO: Get from config.
		},
		stateLock: &sync.Mutex{},
	}
}

// Initialize performs initial configuration of the Service.
func (service *Service) Initialize() error {
	viper.BindEnv("MCP_USER", "mcp.user")
	viper.BindEnv("MCP_PASSWORD", "mcp.password")
	viper.BindEnv("MCP_REGION", "mcp.region")
	viper.BindEnv("MCP_VLAN_ID", "network.vlan_id")

	viper.SetConfigType("yaml")
	viper.SetConfigFile("mcp2-dhcp-server.yml")

	err := viper.ReadInConfig()
	if err != nil {
		panic(err)
	}

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
	)
	if !vlanNetwork.Contains(service.ServiceIP) {
		return fmt.Errorf("Service IP address %s does not lie within the IP network (%s) of the target VLAN ('%s')",
			service.ServiceIP.String(),
			vlanCIDR,
			service.VLAN.Name,
		)
	}

	service.DHCPRangeStart = net.ParseIP(
		viper.GetString("network.start_ip"),
	)
	if !vlanNetwork.Contains(service.DHCPRangeStart) {
		return fmt.Errorf("DHCP range start address %s does not lie within the IP network (%s) of the target VLAN ('%s')",
			service.ServiceIP.String(),
			vlanCIDR,
			service.VLAN.Name,
		)
	}

	service.DHCPRangeEnd = net.ParseIP(
		viper.GetString("network.end_ip"),
	)
	if !vlanNetwork.Contains(service.DHCPRangeEnd) {
		return fmt.Errorf("DHCP range end address %s does not lie within the IP network (%s) of the target VLAN ('%s')",
			service.ServiceIP.String(),
			vlanCIDR,
			service.VLAN.Name,
		)
	}

	if !dhcp.IPLess(service.DHCPRangeStart, service.DHCPRangeEnd) {
		return fmt.Errorf("DHCP range start address %s greater than or equal to DHCP range start address %s",
			service.DHCPRangeStart,
			service.DHCPRangeEnd,
		)
	}

	return nil
}

// Start polling CloudControl for server metadata.
func (service *Service) Start() {
	service.stateLock.Lock()
	defer service.stateLock.Unlock()

	// Warm up caches.
	arp.CacheUpdate()
	err := service.RefreshServerMetadata()
	if err != nil {
		log.Printf("Error refreshing servers: %s",
			err.Error(),
		)
	}

	// Periodically scan the ARP cache so we can resolve MAC addresses from client IPs.
	arp.AutoRefresh(5 * time.Second)

	service.cancelRefresh = make(chan bool, 1)
	service.refreshTimer = time.NewTimer(10 * time.Second)

	go func() {
		cancelRefresh := service.cancelRefresh
		refreshTimer := service.refreshTimer.C

		for {
			select {
			case <-cancelRefresh:
				return // Stopped

			case <-refreshTimer:
				log.Printf("Refreshing server MAC addresses...")

				err := service.RefreshServerMetadata()
				if err != nil {
					log.Printf("Error refreshing servers: %s",
						err.Error(),
					)
				}

				log.Printf("Refreshed server MAC addresses.")
			}
		}
	}()
}

// Stop polling CloudControl for server metadata.
func (service *Service) Stop() {
	service.stateLock.Lock()
	defer service.stateLock.Unlock()

	if service.cancelRefresh != nil {
		service.cancelRefresh <- true
	}
	service.cancelRefresh = nil

	service.refreshTimer.Stop()
	service.refreshTimer = nil
}
