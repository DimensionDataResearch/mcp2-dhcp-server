package main

import (
	"fmt"
	"log"
	"net"

	dhcp "github.com/krolaw/dhcp4"
	dns "github.com/miekg/dns"
)

// ServiceListeners represents the listeners for all services.
type ServiceListeners struct {
	Errors               <-chan error
	service              *Service
	listenInterface      *net.Interface
	listenIPv4Address    *net.IP
	dnsServer            *dns.Server
	dhcpServerConnection *DHCPServerConnection
	running              bool
	errorChannel         chan error
}

// IsRunning determines whether the listeners are currently running.
func (listeners *ServiceListeners) IsRunning() bool {
	return listeners.running
}

// NewServiceListeners creates a new ServiceListeners for the specified Service.
func NewServiceListeners(service *Service) *ServiceListeners {
	errorChannel := make(chan error, 5)

	return &ServiceListeners{
		Errors:       errorChannel,
		service:      service,
		errorChannel: errorChannel,
	}
}

// Initialize performs initialisation of the service listeners.
func (listeners *ServiceListeners) Initialize() error {
	log.Printf("Initialising service listeners (bound to local network interface '%s'...",
		listeners.service.InterfaceName,
	)

	err := listeners.findListenerInterface()
	if err != nil {
		return err
	}

	err = listeners.findFirstListenerIPv4Address()
	if err != nil {
		return err
	}

	return nil
}

// Start the service listeners.
func (listeners *ServiceListeners) Start() error {
	if listeners.listenInterface == nil {
		return fmt.Errorf("service listeners have not been initialised")
	}

	if listeners.running {
		return fmt.Errorf("listeners are already running")
	}

	log.Printf("Starting service listeners (bound to local network interface '%s' / %s...",
		listeners.service.InterfaceName,
		listeners.listenIPv4Address,
	)
	listeners.running = true

	go listeners.serveDHCP()

	if listeners.service.EnableDNS {
		go listeners.serveDNS()
	}

	return nil
}

// Stop the service listeners.
func (listeners *ServiceListeners) Stop() error {
	if listeners.listenInterface == nil {
		return fmt.Errorf("service listeners have not been initialised")
	}

	if listeners.running {
		return fmt.Errorf("listeners are not running")
	}

	log.Printf("Stopping service listeners (bound to local network interface '%s' / %s...",
		listeners.service.InterfaceName,
		listeners.listenIPv4Address,
	)
	listeners.running = false

	if listeners.dhcpServerConnection != nil {
		err := listeners.dhcpServerConnection.Close()
		if err != nil {
			return err
		}
		listeners.dhcpServerConnection = nil
	}

	if listeners.dnsServer != nil {
		err := listeners.dnsServer.Shutdown()
		if err != nil {
			return err
		}
		listeners.dnsServer = nil
	}

	return nil
}

func (listeners *ServiceListeners) serveDHCP() {
	networkConnection, err := net.ListenPacket("udp4", ":67") // Since DHCP is broadcast, we listen on all addresses but filter by interface index.
	if err != nil && listeners.running {
		listeners.errorChannel <- err

		return
	}

	dhcpServerConnection, err := NewDHCPServerConnection(networkConnection, listeners.listenInterface.Index)
	if err != nil {
		if listeners.service.EnableDebugLogging {
			log.Printf("DHCP server error: %#v", err)
		}

		if listeners.running {
			listeners.errorChannel <- err
		}

		return
	}
	listeners.dhcpServerConnection = dhcpServerConnection

	err = dhcp.Serve(listeners.dhcpServerConnection, listeners.service)
	if err != nil {
		if listeners.service.EnableDebugLogging {
			log.Printf("DNS server error: %#v", err)
		}

		if listeners.running {
			listeners.errorChannel <- err
		}
	}
}

func (listeners *ServiceListeners) serveDNS() {
	mux := dns.NewServeMux()
	mux.Handle(".", listeners.service)

	listeners.dnsServer = &dns.Server{
		Addr:    fmt.Sprintf("%s:%d", listeners.listenIPv4Address, listeners.service.DNSPort),
		Net:     "udp",
		Handler: mux,
	}

	err := listeners.dnsServer.ListenAndServe()
	if err != nil && listeners.running {
		listeners.errorChannel <- err
	}

	log.Printf("DNS server shutdown.")
}

func (listeners *ServiceListeners) findListenerInterface() error {
	listenInterface, err := net.InterfaceByName(listeners.service.InterfaceName)
	if err != nil {
		return fmt.Errorf("cannot find local network interface named '%s': %s",
			listeners.service.InterfaceName,
			err.Error(),
		)
	}
	listeners.listenInterface = listenInterface

	return nil
}

func (listeners *ServiceListeners) findFirstListenerIPv4Address() error {
	if listeners.listenInterface == nil {
		return fmt.Errorf("local network interface for service listeners has not been initialised")
	}

	addresses, err := listeners.listenInterface.Addrs()
	if err != nil {
		return err
	}

	for _, address := range addresses {
		interfaceAddress, ok := address.(*net.IPNet)
		if !ok {
			continue
		}

		addressIP := interfaceAddress.IP.To4()
		if len(addressIP) == net.IPv4len {
			listeners.listenIPv4Address = &addressIP

			return nil
		}
	}

	return fmt.Errorf("cannot find an IPv4 address bound to local network interface '%s'", listeners.service.InterfaceName)
}
