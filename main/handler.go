package main

import (
	dhcp "github.com/krolaw/dhcp4"
)

// DHCPHandler handles incoming DHCP requests
type DHCPHandler struct{}

// ServeDHCP handles an incoming DHCP packet.
func (handler *DHCPHandler) ServeDHCP(request dhcp.Packet, msgType dhcp.MessageType, options dhcp.Options) (response dhcp.Packet) {
	return
}
