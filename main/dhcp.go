package main

import (
	"net"
	"time"

	"github.com/DimensionDataResearch/go-dd-cloud-compute/compute"
	dhcp "github.com/krolaw/dhcp4"
)

// Lease represents a DHCP address lease.
type Lease struct {
	// The MAC address of the machine to which the lease belongs.
	MACAddress string

	// The leased IPv4 address.
	IPAddress net.IP

	// The date and time when the lease expires.
	Expires time.Time
}

// ServeDHCP handles an incoming DHCP request.
func (service *Service) ServeDHCP(request dhcp.Packet, msgType dhcp.MessageType, options dhcp.Options) (response dhcp.Packet) {
	clientMACAddress := request.CHAddr().String()

	// Do know about a server in CloudControl with this MAC address?
	server := service.FindServerByMACAddress(clientMACAddress)
	if server == nil {
		return service.replyNAK(request)
	}

	switch msgType {
	// TODO: Handle dhcp.Discover, etc

	case dhcp.Request:
		// Is this a renewal?
		existingLease, ok := service.LeasesByMACAddress[clientMACAddress]
		if ok {
			service.renewLease(existingLease)

			return service.replyACK(request, existingLease.IPAddress, options)
		}

		// New lease
		targetIP := getIPAddressFromMACAddress(server, clientMACAddress)
		newLease := service.createLease(clientMACAddress, targetIP)

		return service.replyACK(request, newLease.IPAddress, options)
	}

	return service.replyNAK(request)
}

// Create an ACK reply packet.
func (service *Service) replyACK(request dhcp.Packet, targetIP net.IP, options dhcp.Options) (response dhcp.Packet) {
	return dhcp.ReplyPacket(request, dhcp.ACK, service.ServiceIP,
		targetIP,
		service.LeaseDuration,
		service.DHCPOptions.SelectOrderOrAll(options[dhcp.OptionParameterRequestList]),
	)
}

// Create a NAK reply packet.
func (service *Service) replyNAK(request dhcp.Packet) (response dhcp.Packet) {
	return dhcp.ReplyPacket(request, dhcp.NAK, service.ServiceIP,
		nil,
		0,
		nil,
	)
}

func (service *Service) createLease(clientMACAddress string, ipAddress net.IP) Lease {
	newLease := &Lease{
		MACAddress: clientMACAddress,
		IPAddress:  ipAddress,
		Expires:    time.Now().Add(service.LeaseDuration),
	}
	service.LeasesByMACAddress[clientMACAddress] = newLease

	return *newLease
}

// Renew lease.
func (service *Service) renewLease(lease *Lease) {
	lease.Expires = time.Now().Add(service.LeaseDuration)
}

// Remove expired leases.
func (service *Service) pruneLeases() {
	now := time.Now()

	var expired []string
	for macAddress := range service.LeasesByMACAddress {
		leaseExpires := service.LeasesByMACAddress[macAddress].Expires

		if now.Sub(leaseExpires) >= 0 {
			expired = append(expired, macAddress)
		}
	}

	for _, macAddress := range expired {
		delete(service.LeasesByMACAddress, macAddress)
	}
}

func getIPAddressFromMACAddress(server *compute.Server, macAddress string) net.IP {
	var targetAddress *string
	primaryNetworkAdapter := server.Network.PrimaryAdapter
	if *primaryNetworkAdapter.MACAddress == macAddress {
		targetAddress = primaryNetworkAdapter.PrivateIPv4Address
	} else {
		for _, additionalNetworkAdapter := range server.Network.AdditionalNetworkAdapters {
			if *additionalNetworkAdapter.MACAddress == macAddress {
				targetAddress = additionalNetworkAdapter.PrivateIPv4Address
			}
		}
	}
	if targetAddress == nil {
		return nil
	}

	return net.ParseIP(*targetAddress)
}
