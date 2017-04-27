package main

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/DimensionDataResearch/go-dd-cloud-compute/compute"
	dhcp "github.com/krolaw/dhcp4"
)

// StaticReservation represents a static DHCP address reservation.
type StaticReservation struct {
	// The machine's MAC address.
	MACAddress string

	// The machine's host name.
	HostName string

	// The machine's IPv4 address.
	IPAddress net.IP
}

// Lease represents a DHCP address lease.
type Lease struct {
	// The MAC address of the machine to which the lease belongs.
	MACAddress string

	// The leased IPv4 address.
	IPAddress net.IP

	// The date and time when the lease expires.
	Expires time.Time
}

// IsExpired determines whether the lease has expired.
func (lease *Lease) IsExpired() bool {
	return time.Now().Sub(lease.Expires) >= 0
}

// ServeDHCP handles an incoming DHCP request.
func (service *Service) ServeDHCP(request dhcp.Packet, msgType dhcp.MessageType, options dhcp.Options) (response dhcp.Packet) {
	clientMACAddress := request.CHAddr().String()

	// Do know about a server in CloudControl with this MAC address?
	server := service.FindServerByMACAddress(clientMACAddress)

	switch msgType {
	case dhcp.Discover:
		log.Printf("Discover message from client with MAC address '%s' (IP '%s').", clientMACAddress, request.CIAddr().String())

		if server == nil {
			log.Printf("MAC address '%s' does not correspond to a server in CloudControl (no reply will be sent).", clientMACAddress)

			return service.noReply()
		}

		targetIP, ok := getIPAddressFromMACAddress(server, clientMACAddress)
		if !ok {
			log.Printf("MAC address '%s' does not correspond to a server in CloudControl (no reply will be sent).", clientMACAddress)

			return service.noReply()
		}

		return service.replyOffer(request, targetIP, options)

	case dhcp.Request:
		log.Printf("Request message from client with MAC address %s.", clientMACAddress)

		if server == nil {
			log.Printf("MAC address '%s' does not correspond to a server in CloudControl (no reply will be sent).", clientMACAddress)

			return service.replyNAK(request)
		}

		// Is this a renewal?
		existingLease, ok := service.LeasesByMACAddress[clientMACAddress]
		if ok && !existingLease.IsExpired() {
			log.Printf("Renew lease on IPv4 address %s for server '%s' (MAC address %s) and send ACK reply.",
				server.Name,
				existingLease.IPAddress.String(),
				clientMACAddress,
			)

			service.renewLease(existingLease)

			return service.replyACK(request, existingLease.IPAddress, options)
		}

		// New lease
		targetIP, ok := getIPAddressFromMACAddress(server, clientMACAddress)
		if !ok {
			log.Printf("Cannot resolve network adapter in server '%s' (%s) with MAC address %s; send NAK reply.",
				server.Name,
				server.ID,
				clientMACAddress,
			)

			return service.replyNAK(request)
		}

		log.Printf("Create lease on IPv4 address %s for server '%s' (MAC address %s) and send ACK reply.",
			server.Name,
			targetIP.String(),
			clientMACAddress,
		)
		newLease := service.createLease(clientMACAddress, targetIP)

		return service.replyACK(request, newLease.IPAddress, options)

	case dhcp.Release:
		log.Printf("Release message from client with MAC address %s.", clientMACAddress)

		if server == nil {
			log.Printf("MAC address '%s' does not correspond to a server in CloudControl (no reply will be sent).", clientMACAddress)

			return service.replyNAK(request)
		}

		existingLease, ok := service.LeasesByMACAddress[clientMACAddress]
		if ok && !existingLease.IsExpired() {
			log.Printf("Server '%s' (%s) requested requested termination of lease on IPv4 address %s (MAC address %s).",
				server.Name,
				server.ID,
				existingLease.IPAddress.String(),
				clientMACAddress,
			)

			service.renewLease(existingLease)

			return service.replyACK(request, existingLease.IPAddress, options)
		}

		log.Printf("Server '%s' (%s) requested requested termination of expired or non-existent lease (MAC address %s). Ignored.",
			server.Name,
			server.ID,
			clientMACAddress,
		)

		return service.noReply() // No reply is necessary for Release.
	}

	return service.replyNAK(request)
}

// Create an empty reply packet (i.e. no reply should be sent)
func (service *Service) noReply() dhcp.Packet {
	fmt.Println("NOREPLY")
	return dhcp.Packet{}
}

// Create an Offer reply packet.
func (service *Service) replyOffer(request dhcp.Packet, targetIP net.IP, options dhcp.Options) (response dhcp.Packet) {
	reply := dhcp.ReplyPacket(request, dhcp.Offer, service.ServiceIP,
		targetIP,
		service.LeaseDuration,
		service.DHCPOptions.SelectOrderOrAll(options[dhcp.OptionParameterRequestList]),
	)

	if service.EnableIPXE {
		// Consider making iPXE server IP configurable (user may want to host it elsewhere).
		// If so, use:
		/*
			reply.AddOption(dhcp.OptionSwapServer,
				[]byte(service.IPXEServerIP),
			)
		*/

		// Mark us as PXE-capable (legacy BIOS boot).
		reply.AddOption(dhcp.OptionVendorClassIdentifier,
			[]byte("PXEServer"),
		)

		reply.AddOption(dhcp.OptionTFTPServerName,
			[]byte(service.ServiceIP.String()),
		)

		userClass, ok := options[dhcp.OptionUserClass]
		if ok {
			if string(userClass) == "iPXE" {
				// This is an iPXE client; direct them to load the iPXE boot script.
				log.Printf("Client with MAC address '%s' is an iPXE client; directing them to boot script 'tftp://%s/%s'.",
					request.CHAddr().String(),
					service.ServiceIP,
					service.IPXEBootScript,
				)
				if service.IPXEBootScript != "" {
					reply.AddOption(dhcp.OptionBootFileName,
						[]byte(service.IPXEBootScript),
					)
				} else {
					log.Printf("Warning - iPXE client requested boot file, but no iPXE boot script has been configured (ipxe.boot_script / MCP_IPXE_BOOT_SCRIPT).")
				}
			} else {
				log.Printf("Client with MAC address '%s' has unknown user class '%s'.\n",
					request.CHAddr().String(),
					string(userClass),
				)
			}
		} else {
			// This is a PXE client; direct them to load the standard PXE boot image.
			log.Printf("Client with MAC address '%s' is a regular PXE (or non-PXE) client; directing them to iPXE boot image '%s'.",
				request.CHAddr().String(),
				service.PXEBootImage,
			)
			reply.AddOption(dhcp.OptionBootFileName,
				[]byte(service.PXEBootImage),
			)
		}
	}

	return reply
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

func getIPAddressFromMACAddress(server *compute.Server, macAddress string) (targetIP net.IP, ok bool) {
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
		return
	}

	targetIP = net.ParseIP(*targetAddress)
	ok = true

	return
}
