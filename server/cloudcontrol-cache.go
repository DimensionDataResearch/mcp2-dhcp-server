package main

import (
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/DimensionDataResearch/go-dd-cloud-compute/compute"
)

// ServerMetadata represents metadata for a CloudControl server.
type ServerMetadata struct {
	ID               string
	Name             string
	IPv4ByMACAddress map[string]net.IP

	// If specified, overrides the default PXE boot image.
	PXEBootImage string

	// If specified, overrides the default PXE profile image.
	IPXEProfile string

	// If specified, overrides the default iPXE boot script URL (and IPXEProfile).
	IPXEBootScript string
}

// RefreshServerMetadata refreshes the map of MAC addresses to server metadata.
func (service *Service) RefreshServerMetadata() error {
	return service.refreshServerMetadataInternal(true)
}
func (service *Service) refreshServerMetadataInternal(acquireStateLock bool) error {
	serverMetadataByMACAddress, err := service.readServerMetadata()
	if err != nil {
		return err
	}

	if acquireStateLock {
		service.acquireStateLock("refreshServerMetadataInternal")
		defer service.releaseStateLock("refreshServerMetadataInternal")
	}
	service.ServerMetadataByMACAddress = serverMetadataByMACAddress

	return nil
}

// readServerMetadata creates a map of MAC addresses to server metadata from CloudControl.
func (service *Service) readServerMetadata() (map[string]ServerMetadata, error) {
	serverMetadataByMACAddress := make(map[string]ServerMetadata)

	page := compute.DefaultPaging()
	page.PageSize = 50

	for {
		servers, err := service.Client.ListServersInNetworkDomain(service.NetworkDomain.ID, page)
		if err != nil {
			return nil, err
		}
		if servers.IsEmpty() {
			break
		}

		for _, server := range servers.Items {
			// Ignore servers that are being deployed or destroyed.
			primaryNetworkAdapter := server.Network.PrimaryAdapter
			if primaryNetworkAdapter.PrivateIPv4Address == nil {
				continue
			}

			primaryMACAddress := strings.ToLower(
				*primaryNetworkAdapter.MACAddress,
			)
			serverMetadata := &ServerMetadata{
				ID:   server.ID,
				Name: server.Name,
				IPv4ByMACAddress: map[string]net.IP{
					primaryMACAddress: net.ParseIP(*primaryNetworkAdapter.PrivateIPv4Address),
				},
			}

			if service.EnableDebugLogging {
				log.Printf("\tMAC %s -> %s (%s)\n",
					primaryMACAddress,
					*primaryNetworkAdapter.PrivateIPv4Address,
					server.Name,
				)
			}

			// Enable overriding of PXE / iPXE metadata from tags.
			tagPage := compute.DefaultPaging()
			tagPage.PageSize = 50

			for {
				tags, err := service.Client.GetAssetTags(server.ID, compute.AssetTypeServer, tagPage)
				if err != nil {
					return nil, err
				}
				if tags.IsEmpty() {
					break
				}

				for _, tag := range tags.Items {
					switch tag.Name {
					case "pxe_boot_image":
						serverMetadata.PXEBootImage = tag.Value
					case "ipxe_profile":
						serverMetadata.IPXEProfile = tag.Value
					case "ipxe_boot_script":
						serverMetadata.IPXEBootScript = tag.Value
					}
				}

				tagPage.Next()
			}

			// If we have an IPXE profile, but no URL for the IPXE boot script, generate the URL.
			if serverMetadata.IPXEProfile != "" && serverMetadata.IPXEBootScript == "" {
				serverMetadata.IPXEBootScript = fmt.Sprintf("http://%s/?profile=%s",
					service.ServiceIP,
					serverMetadata.IPXEProfile,
				)
			}

			for _, additionalNetworkAdapter := range server.Network.AdditionalNetworkAdapters {
				// Ignore network adapters that are being deployed or destroyed.
				if additionalNetworkAdapter.PrivateIPv4Address == nil {
					continue
				}

				additionalMACAddress := strings.ToLower(
					*additionalNetworkAdapter.MACAddress,
				)
				serverMetadata.IPv4ByMACAddress[additionalMACAddress] = net.ParseIP(*additionalNetworkAdapter.PrivateIPv4Address)

				if service.EnableDebugLogging {
					log.Printf("\tMAC address %s -> %s (%s)\n",
						additionalMACAddress,
						*additionalNetworkAdapter.PrivateIPv4Address,
						server.Name,
					)
				}
			}

			// Enable lookup by any MAC address.
			for macAddress := range serverMetadata.IPv4ByMACAddress {
				serverMetadataByMACAddress[macAddress] = *serverMetadata
			}
		}

		page.Next()
	}

	return serverMetadataByMACAddress, nil
}

// FindServerMetadataByMACAddress finds the metadata for the server (if any) posessing a network adapter with the specified MAC address.
func (service *Service) FindServerMetadataByMACAddress(macAddress string) *ServerMetadata {
	service.acquireStateLock("FindServerMetadataByMACAddress")
	defer service.releaseStateLock("FindServerMetadataByMACAddress")

	macAddress = strings.ToLower(macAddress)

	// Fake up metadata for a matching server if there's a matching static address reservation.
	staticReservation, ok := service.StaticReservationsByMACAddress[macAddress]
	if ok {
		serverMACAddress := macAddress
		serverPrivateIPv4Address := staticReservation.IPAddress

		return &ServerMetadata{
			ID:   staticReservation.HostName,
			Name: staticReservation.HostName,
			IPv4ByMACAddress: map[string]net.IP{
				serverMACAddress: serverPrivateIPv4Address,
			},
		}
	}

	serverMetadata, ok := service.ServerMetadataByMACAddress[macAddress]
	if ok {
		return &serverMetadata
	}

	return nil
}
