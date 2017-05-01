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
	allServerTags, err := service.getAllServerTags()
	if err != nil {
		return nil, err
	}

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
			service.parseServerTags(serverMetadata, allServerTags)

			if service.EnableDebugLogging {
				log.Printf("\tMAC %s -> %s (%s)\n",
					primaryMACAddress,
					*primaryNetworkAdapter.PrivateIPv4Address,
					server.Name,
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

// Get tags for all servers, keyed by server Id.
func (service *Service) getAllServerTags() (map[string][]compute.TagDetail, error) {
	allServerTags := make(map[string][]compute.TagDetail)

	tagPage := compute.DefaultPaging()
	tagPage.PageSize = 50

	for {
		tags, err := service.Client.GetAssetTagsByType(compute.AssetTypeServer, service.NetworkDomain.DatacenterID, tagPage)
		if err != nil {
			if compute.IsAPIErrorCode(err, compute.ResponseCodeUnexpectedError) {
				break // CloudControl bug - going past last page of tags returns UNEXPECTED_ERROR (i.e. there are no more tags).
			}

			return nil, err
		}
		if tags.IsEmpty() {
			break // No more tags.
		}

		for _, tag := range tags.Items {
			serverTags, ok := allServerTags[tag.AssetID]
			if ok {
				serverTags = append(serverTags, tag)
			} else {
				serverTags = []compute.TagDetail{tag}
			}
			allServerTags[tag.AssetID] = serverTags
		}

		tagPage.Next()
	}

	return allServerTags, nil
}

// Update server metadata from tags (if any) applied to the specified server.
func (service *Service) parseServerTags(serverMetadata *ServerMetadata, allServerTags map[string][]compute.TagDetail) {
	serverTags, ok := allServerTags[serverMetadata.ID]
	if !ok {
		if service.EnableDebugLogging {
			log.Printf("\tNo tags for server '%s' (Id = '%s'); assuming default configuration.",
				serverMetadata.Name,
				serverMetadata.ID,
			)
		}

		return
	}

	for _, tag := range serverTags {
		switch tag.Name {
		case "pxe_boot_image":
			serverMetadata.PXEBootImage = tag.Value
		case "ipxe_profile":
			if serverMetadata.IPXEBootScript != "" {
				continue // ipxe_boot_script overrides ipxe_profile
			}

			// TODO: Add config item for URL template.
			serverMetadata.IPXEBootScript = fmt.Sprintf("http://%s:%d/?profile=%s",
				service.ServiceIP,
				service.IPXEPort,
				tag.Value,
			)
		case "ipxe_boot_script":
			serverMetadata.IPXEBootScript = tag.Value
		}
	}

	if service.EnableDebugLogging {
		log.Printf("\t%d tags for server '%s' (Id = '%s'):",
			len(serverTags),
			serverMetadata.Name,
			serverMetadata.ID,
		)

		if serverMetadata.PXEBootImage != "" {
			log.Printf("\t\tOverride PXE boot image: '%s'", serverMetadata.PXEBootImage)
		}
		if serverMetadata.IPXEBootScript != "" {
			log.Printf("\t\tOverride iPXE boot script: '%s'", serverMetadata.IPXEBootScript)
		}
	}
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
