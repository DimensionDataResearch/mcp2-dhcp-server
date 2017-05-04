package main

import (
	"net"

	"golang.org/x/net/ipv4"

	dhcp "github.com/krolaw/dhcp4"
)

// DHCPServerConnection is a custom DHCP server connection that supports control of the underlying network connection.
//
// Its primary purpose is to make the DHCP server stoppable.
//
// TODO: Handle resulting error when connection is closed.
type DHCPServerConnection struct {
	targetInterfaceIndex int
	networkConnection    *ipv4.PacketConn
	controlMessage       *ipv4.ControlMessage
}

// NewDHCPServerConnection creates a new DHCP server connection.
func NewDHCPServerConnection(connection net.PacketConn, targetInterfaceIndex int) (*DHCPServerConnection, error) {
	networkConnection := ipv4.NewPacketConn(connection)
	err := networkConnection.SetControlMessage(ipv4.FlagInterface, true) // We filter by interface index.
	if err != nil {
		return nil, err
	}

	serverConnection := &DHCPServerConnection{
		targetInterfaceIndex: targetInterfaceIndex,
		networkConnection:    networkConnection,
	}

	return serverConnection, nil
}

var _ dhcp.ServeConn = &DHCPServerConnection{}

// Close the server's underlying network connection.
func (server *DHCPServerConnection) Close() error {
	return server.networkConnection.Close()
}

// ReadFrom reads data from the underlying network connection into the specified buffer.
func (server *DHCPServerConnection) ReadFrom(buffer []byte) (bytesRead int, sourceAddress net.Addr, err error) {
	bytesRead, server.controlMessage, sourceAddress, err = server.networkConnection.ReadFrom(buffer)
	if server.controlMessage != nil && server.controlMessage.IfIndex != server.targetInterfaceIndex { // Filter all other interfaces
		bytesRead = 0 // Packets < 240 are filtered in dns.Serve().
	}

	return
}

// WriteTo writes data from the specified buffer to the underlying network connection.
func (server *DHCPServerConnection) WriteTo(buffer []byte, destinationAddress net.Addr) (bytesWritten int, err error) {

	// ipv4 docs state that Src is "specify only", however testing by tfheen
	// shows that Src IS populated.  Therefore, to reuse the control message,
	// we set Src to nil to avoid the error "write udp4: invalid argument"
	server.controlMessage.Src = nil

	return server.networkConnection.WriteTo(buffer, server.controlMessage, destinationAddress)
}
