package main

import (
	dhcp "github.com/krolaw/dhcp4"
)

// DHCPHandler handles incoming DHCP requests
type DHCPHandler struct {
	// Address leases, keyed by MAC address.
	LeasesByMACAddress map[string]*Lease
}

// Lease represents a DHCP lease.
type Lease struct {
	// The MAC address of the machine to which the lease belongs.
	MACAddress string

	// The leased IPv4 address.
	IPAddress string
}

// ServeDHCP handles an incoming DHCP request.
func (handler *DHCPHandler) ServeDHCP(request dhcp.Packet, msgType dhcp.MessageType, options dhcp.Options) (response dhcp.Packet) {
	return
}
