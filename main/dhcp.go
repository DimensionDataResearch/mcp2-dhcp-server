package main

import (
	"net"
	"time"

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

	switch msgType {
	// TODO: Handle dhcp.Discover

	case dhcp.Request:
		// Is this a renewal?
		existingLease, ok := service.LeasesByMACAddress[clientMACAddress]
		if ok {
			service.renewLease(existingLease)

			return dhcp.ReplyPacket(request, dhcp.ACK, service.ServiceIP,
				existingLease.IPAddress,
				service.LeaseDuration,
				service.DHCPOptions.SelectOrderOrAll(options[dhcp.OptionParameterRequestList]),
			)
		}

		nextAvailableIP := service.DHCPRangeStart

		for {
			if !dhcp.IPInRange(service.DHCPRangeStart, service.DHCPRangeEnd, nextAvailableIP) {
				// We're out of addresses.
				return dhcp.ReplyPacket(request, dhcp.NAK, service.ServiceIP,
					nil,
					0,
					nil,
				)
			}

			// Address in use?
			_, ok := service.LeasesByIP[nextAvailableIP.String()]
			if ok {
				// Yep, so try next available.
				nextAvailableIP = dhcp.IPAdd(nextAvailableIP, 1)

				continue
			}

			// New lease
			newLease := service.createLease(clientMACAddress, nextAvailableIP)

			return dhcp.ReplyPacket(request, dhcp.ACK, service.ServiceIP,
				newLease.IPAddress,
				service.LeaseDuration,
				service.DHCPOptions.SelectOrderOrAll(options[dhcp.OptionParameterRequestList]),
			)
		}
	}

	return
}

func (service *Service) createLease(clientMACAddress string, ipAddress net.IP) Lease {
	newLease := &Lease{
		MACAddress: clientMACAddress,
		IPAddress:  ipAddress,
		Expires:    time.Now().Add(service.LeaseDuration),
	}
	service.LeasesByMACAddress[clientMACAddress] = newLease
	service.LeasesByIP[ipAddress.String()] = newLease

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
