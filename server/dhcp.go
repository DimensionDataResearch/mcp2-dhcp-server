package main

import (
	"fmt"
	"log"
	"net"
	"strings"
	"time"

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
func (service *Service) ServeDHCP(request dhcp.Packet, msgType dhcp.MessageType, requestOptions dhcp.Options) (response dhcp.Packet) {
	switch msgType {
	case dhcp.Discover:
		response = service.handleDiscover(request, requestOptions)

	case dhcp.Request:
		response = service.handleRequest(request, requestOptions)

	case dhcp.Release:
		response = service.handleRelease(request, requestOptions)
	default:
		log.Printf("[TXN: %s] Ignoring unhandled DHCP message type (%s).",
			getTransactionID(request),
			msgType.String(),
		)

		response = service.replyNAK(request)
	}

	if response != nil {
		response.PadToMinSize() // Must add padding AFTER all other options.
	}

	return
}

// Handle a DHCP Discover packet.
func (service *Service) handleDiscover(request dhcp.Packet, requestOptions dhcp.Options) (response dhcp.Packet) {
	transactionID := getTransactionID(request)
	clientMACAddress := request.CHAddr().String()

	// Do we know about a server in CloudControl with this MAC address?
	serverMetadata := service.FindServerMetadataByMACAddress(clientMACAddress)

	log.Printf("[TXN: %s] Discover message from client with MAC address %s (IP '%s').",
		transactionID,
		clientMACAddress,
		request.CIAddr().String(),
	)

	if serverMetadata == nil {
		log.Printf("[TXN: %s] MAC address %s does not correspond to a server in CloudControl (no reply will be sent).",
			transactionID,
			clientMACAddress,
		)

		return service.noReply()
	}

	targetIP, ok := serverMetadata.IPv4ByMACAddress[clientMACAddress]
	if !ok {
		log.Printf("[TXN: %s] MAC address %s does not correspond to a network adapter in CloudControl (no reply will be sent).",
			transactionID,
			clientMACAddress,
		)

		return service.noReply()
	}

	return service.replyOffer(request, targetIP, requestOptions, *serverMetadata)
}

// Handle a DHCP Request packet.
func (service *Service) handleRequest(request dhcp.Packet, requestOptions dhcp.Options) (response dhcp.Packet) {
	transactionID := getTransactionID(request)
	clientMACAddress := request.CHAddr().String()

	// Do we know about a server in CloudControl with this MAC address?
	serverMetadata := service.FindServerMetadataByMACAddress(clientMACAddress)

	log.Printf("[TXN: %s] Request message from client with MAC address %s (IP '%s').",
		transactionID,
		clientMACAddress,
		request.CIAddr().String(),
	)

	if serverMetadata == nil {
		log.Printf("[TXN: %s] MAC address %s does not correspond to a server in CloudControl (no reply will be sent).",
			transactionID,
			clientMACAddress,
		)

		return service.replyNAK(request)
	}

	// Is this a renewal?
	existingLease, ok := service.LeasesByMACAddress[clientMACAddress]
	if ok && !existingLease.IsExpired() {
		log.Printf("[TXN: %s] Renew lease on IPv4 address %s for server %s and send ACK reply.",
			transactionID,
			existingLease.IPAddress.String(),
			serverMetadata.Name,
		)

		service.renewLease(existingLease)

		return service.replyACK(request, existingLease.IPAddress, requestOptions, *serverMetadata)
	}

	// New lease
	targetIP, ok := serverMetadata.IPv4ByMACAddress[clientMACAddress]
	if !ok {
		log.Printf("[TXN: %s] Cannot resolve network adapter in server %s (%s) with MAC address %s; send NAK reply.",
			transactionID,
			serverMetadata.Name,
			serverMetadata.ID,
			clientMACAddress,
		)

		return service.replyNAK(request)
	}

	log.Printf("[TXN: %s] Create lease on IPv4 address %s for server %s (MAC address %s) and send ACK reply.",
		transactionID,
		serverMetadata.Name,
		targetIP.String(),
		clientMACAddress,
	)
	newLease := service.createLease(clientMACAddress, targetIP)

	return service.replyACK(request, newLease.IPAddress, requestOptions, *serverMetadata)
}

// Handle a DHCP Release packet.
func (service *Service) handleRelease(request dhcp.Packet, requestOptions dhcp.Options) (response dhcp.Packet) {
	transactionID := getTransactionID(request)
	clientMACAddress := request.CHAddr().String()

	// Do we know about a server in CloudControl with this MAC address?
	serverMetadata := service.FindServerMetadataByMACAddress(clientMACAddress)

	log.Printf("[TXN: %s] Release message from client with MAC address %s (IP '%s').",
		transactionID,
		clientMACAddress,
		request.CIAddr().String(),
	)

	if serverMetadata == nil {
		log.Printf("MAC address %s does not correspond to a server in CloudControl (no reply will be sent).", clientMACAddress)

		return service.replyNAK(request)
	}

	existingLease, ok := service.LeasesByMACAddress[clientMACAddress]
	if ok && !existingLease.IsExpired() {
		log.Printf("[TXN: %s] Server '%s' (%s) requested termination of lease on IPv4 address %s.",
			transactionID,
			serverMetadata.Name,
			serverMetadata.ID,
			existingLease.IPAddress.String(),
		)

		service.expireLease(existingLease)
	} else {
		log.Printf("[TXN: %s] Server '%s' (%s) requested requested termination of expired or non-existent lease; request ignored.",
			transactionID,
			serverMetadata.Name,
			serverMetadata.ID,
		)
	}

	return service.noReply() // No reply is necessary for Release.
}

// Create an empty reply packet (i.e. no reply should be sent)
func (service *Service) noReply() dhcp.Packet {
	return dhcp.Packet{}
}

// Create an Offer reply packet (in response to Discover packet).
func (service *Service) replyOffer(request dhcp.Packet, targetIP net.IP, requestOptions dhcp.Options, serverMetadata ServerMetadata) (response dhcp.Packet) {
	reply := newReply(request, dhcp.Offer, service.ServiceIP,
		targetIP,
		service.LeaseDuration,
		service.DHCPOptions.SelectOrderOrAll(requestOptions[dhcp.OptionParameterRequestList]),
	)

	// Configure host name from server name.
	reply.AddOption(dhcp.OptionHostName,
		[]byte(serverMetadata.Name),
	)

	// Add DHCP options for PXE / iPXE, if required.
	if service.EnableIPXE && isPXEClient(requestOptions) {
		service.addIPXEOptions(request, requestOptions, serverMetadata, reply)
	}

	// Set the DHCP server identity (i.e. DHCP server address).
	reply.SetSIAddr(service.ServiceIP)

	return reply
}

// Create an ACK reply packet (in response to Request packet).
func (service *Service) replyACK(request dhcp.Packet, targetIP net.IP, requestOptions dhcp.Options, serverMetadata ServerMetadata) (response dhcp.Packet) {
	reply := newReply(request, dhcp.ACK, service.ServiceIP,
		targetIP,
		service.LeaseDuration,
		service.DHCPOptions.SelectOrderOrAll(requestOptions[dhcp.OptionParameterRequestList]),
	)

	// Configure host name from server name.
	reply.AddOption(dhcp.OptionHostName,
		[]byte(serverMetadata.Name),
	)

	// Add DHCP options for PXE / iPXE, if required.
	if service.EnableIPXE && isPXEClient(requestOptions) {
		service.addIPXEOptions(request, requestOptions, serverMetadata, reply)
	}

	// Set the DHCP server identity (i.e. DHCP server address).
	reply.SetSIAddr(service.ServiceIP)

	return reply
}

// Create a NAK reply packet (in response to Discover or Request packet)
func (service *Service) replyNAK(request dhcp.Packet) (response dhcp.Packet) {
	reply := newReply(request, dhcp.NAK, service.ServiceIP,
		nil,
		0,
		nil,
	)

	reply.SetSIAddr(service.ServiceIP)

	return reply
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
	service.acquireStateLock("renewLease")
	defer service.releaseStateLock("renewLease")

	lease.Expires = time.Now().Add(service.LeaseDuration)
}

// Remove a lease.
func (service *Service) expireLease(lease *Lease) {
	service.acquireStateLock("expireLease")
	defer service.releaseStateLock("expireLease")

	lease.Expires = time.Now()

	delete(service.LeasesByMACAddress, lease.MACAddress)
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

// Add options for PXE / iPXE to a DHCP response.
func (service *Service) addIPXEOptions(request dhcp.Packet, requestOptions dhcp.Options, serverMetadata ServerMetadata, reply dhcp.Packet) {
	transactionID := getTransactionID(request)

	if isIPXEClient(requestOptions) {
		// This is an iPXE client; direct them to load the iPXE boot script.
		log.Printf("[TXN: %s] Client with MAC address %s is an iPXE client; directing them to boot script '%s'.",
			transactionID,
			request.CHAddr().String(),
			service.getIPXEBootScript(serverMetadata),
		)

		service.addIPXEBootScript(serverMetadata, reply)
	} else {
		// This is a PXE client; direct them to load the standard PXE boot image.
		log.Printf("[TXN: %s] Client with MAC address %s is a regular PXE (or non-PXE) client; directing them to iPXE boot image 'tftp://%s/%s'.",
			transactionID,
			request.CHAddr().String(),
			service.ServiceIP,
			service.getPXEBootImage(serverMetadata),
		)

		service.addPXEBootImage(serverMetadata, reply)
	}
}

// Get the configured PXE boot image for the specified server.
func (service *Service) getPXEBootImage(serverMetadata ServerMetadata) string {
	ipxeBootScript := serverMetadata.PXEBootImage
	if ipxeBootScript == "" {
		ipxeBootScript = service.PXEBootImage
	}

	return ipxeBootScript
}

// Get the configured PXE boot image for the specified server.
func (service *Service) getIPXEBootScript(serverMetadata ServerMetadata) string {
	pxeBootImage := serverMetadata.IPXEBootScript
	if pxeBootImage == "" {
		pxeBootImage = service.IPXEBootScript
	}

	return pxeBootImage
}

// Add a PXE boot image (and TFTP server) to a DHCP response.
func (service *Service) addPXEBootImage(serverMetadata ServerMetadata, response dhcp.Packet) {
	pxeBootImage := service.getPXEBootImage(serverMetadata)

	addBootFile(response, pxeBootImage)
	addTFTPBootFile(response, service.TFTPServerName, pxeBootImage)
}

// Add an IPXE boot script URL to a DHCP response.
func (service *Service) addIPXEBootScript(serverMetadata ServerMetadata, response dhcp.Packet) {
	ipxeBootScript := service.getIPXEBootScript(serverMetadata)

	addBootFile(response, ipxeBootScript)
	addBootFileOption(response, ipxeBootScript)
}

// Get the DHCP transaction Id as a string.
func getTransactionID(request dhcp.Packet) string {
	xid := request.XId()

	return fmt.Sprintf("0x%02X%02X%02X%02X",
		xid[0], xid[1], xid[2], xid[3],
	)
}

// Get the DHCP user class from the request options.
func getUserClass(requestOptions dhcp.Options) string {
	userClass, ok := requestOptions[dhcp.OptionUserClass]
	if ok {
		return string(userClass)
	}

	return ""
}

// Get the DHCP vendor class identifier from the request options.
func getVendorClassIdentifier(requestOptions dhcp.Options) string {
	vendorClassIdentifier, ok := requestOptions[dhcp.OptionVendorClassIdentifier]
	if ok {
		return string(vendorClassIdentifier)
	}

	return ""
}

// Determine if the DHCP request comes from a PXE-capable client seeking a boot server.
func isPXEClient(requestOptions dhcp.Options) bool {
	return strings.HasPrefix(
		getVendorClassIdentifier(requestOptions),
		"PXEClient:",
	)
}

// Determine if the DHCP request comes from an iPXE client.
func isIPXEClient(requestOptions dhcp.Options) bool {
	return getUserClass(requestOptions) == "iPXE"
}

// Mark a DHCP response as coming from a (legacy BIOS boot) PXE-capable DHCP server.
func markAsPXEServer(response dhcp.Packet) {

	response.AddOption(dhcp.OptionVendorClassIdentifier,
		[]byte("PXEServer"),
	)
}

// Add a BOOTP-style boot file path to a DHCP response.
func addBootFile(response dhcp.Packet, bootFile string) {
	response.SetFile(
		[]byte(bootFile),
	)
}

// Add a DHCP-style boot file path option to a DHCP response.
func addBootFileOption(response dhcp.Packet, bootFile string) {
	response.AddOption(dhcp.OptionBootFileName,
		[]byte(bootFile),
	)
}

// Add DHCP TFTPServerName and BootFileName options (i.e. option 66, option 67) to a DHCP response.
func addTFTPBootFile(response dhcp.Packet, tftpServerName string, bootFile string) {
	addBootFileOption(response, bootFile)

	response.AddOption(dhcp.OptionTFTPServerName,
		[]byte(tftpServerName),
	)
}

// Create a reply packet.
func newReply(request dhcp.Packet, messageType dhcp.MessageType, serverIP, clientIP net.IP, leaseDuration time.Duration, options []dhcp.Option) (reply dhcp.Packet) {
	reply = dhcp.NewPacket(dhcp.BootReply)
	reply.SetXId(request.XId())
	reply.SetFlags(request.Flags())
	reply.SetYIAddr(clientIP)
	reply.SetGIAddr(request.GIAddr())
	reply.SetCHAddr(request.CHAddr())
	reply.AddOption(dhcp.OptionDHCPMessageType, []byte{byte(messageType)})
	reply.AddOption(dhcp.OptionServerIdentifier, []byte(serverIP))
	if leaseDuration > 0 {
		reply.AddOption(dhcp.OptionIPAddressLeaseTime, dhcp.OptionsLeaseTime(leaseDuration))
	}
	for _, option := range options {
		reply.AddOption(option.Code, option.Value)
	}

	// We don't add padding until ALL options have been added (DHCP packet implementation is a bit buggy).

	return reply
}
