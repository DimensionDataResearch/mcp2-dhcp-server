package main

import (
	"os"
	"strings"

	"github.com/DimensionDataResearch/go-dd-cloud-compute/compute"
)

// RefreshServerMetadata refreshes the map of MAC addresses to server metadata.
func (service *Service) RefreshServerMetadata() error {
	service.stateLock.Lock()
	defer service.stateLock.Unlock()

	serversByMACAddress := make(map[string]compute.Server)

	page := compute.DefaultPaging()
	page.PageSize = 50

	for {
		servers, err := service.Client.ListServersInNetworkDomain(service.NetworkDomain.ID, page)
		if err != nil {
			return err
		}
		if servers.IsEmpty() {
			break
		}

		for _, server := range servers.Items {
			// Ignore servers that are being deployed or destroyed.
			if server.Network.PrimaryAdapter.PrivateIPv4Address == nil {
				continue
			}

			primaryMACAddress := strings.ToLower(
				*server.Network.PrimaryAdapter.MACAddress,
			)
			service.ServersByMACAddress[primaryMACAddress] = server

			for _, additionalNetworkAdapter := range server.Network.AdditionalNetworkAdapters {
				// Ignore network adapters that are being deployed or destroyed.
				if additionalNetworkAdapter.PrivateIPv4Address == nil {
					continue
				}

				additionalMACAddress := strings.ToLower(
					*additionalNetworkAdapter.MACAddress,
				)
				service.ServersByMACAddress[additionalMACAddress] = server
			}
		}

		page.Next()
	}

	service.ServersByMACAddress = serversByMACAddress

	return nil
}

// FindServerByMACAddress finds the server (if any) posessing a network adapter with the specified MAC address.
func (service *Service) FindServerByMACAddress(macAddress string) *compute.Server {
	service.stateLock.Lock()
	defer service.stateLock.Unlock()

	macAddress = strings.ToLower(macAddress)

	// Fake up a matching server if there's a matching static address reservation.
	staticReservation, ok := service.StaticReservationsByMACAddress[macAddress]
	if ok {
		serverMACAddress := macAddress
		serverPrivateIPv4Address := staticReservation.IPAddress.String()

		return &compute.Server{
			Name: staticReservation.HostName,
			Network: compute.VirtualMachineNetwork{
				PrimaryAdapter: compute.VirtualMachineNetworkAdapter{
					MACAddress:         &serverMACAddress,
					PrivateIPv4Address: &serverPrivateIPv4Address,
				},
			},
		}
	}

	server, ok := service.ServersByMACAddress[macAddress]
	if ok {
		return &server
	}

	return nil
}

// Create a test server for calls from localhost.
func createTestServer() *compute.Server {
	localhost4 := os.Getenv("MCP_TEST_HOST_IPV4")
	if localhost4 == "" {
		localhost4 = "127.0.0.1"
	}

	localhost6 := os.Getenv("MCP_TEST_HOST_IPV6")
	if localhost6 == "" {
		localhost6 = "::1"
	}

	return &compute.Server{
		Name: os.Getenv("HOST"),
		Network: compute.VirtualMachineNetwork{
			PrimaryAdapter: compute.VirtualMachineNetworkAdapter{
				PrivateIPv4Address: &localhost4,
				PrivateIPv6Address: &localhost6,
			},
		},
	}
}
