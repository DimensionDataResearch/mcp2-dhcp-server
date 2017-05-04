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
	Errors            <-chan error
	service           *Service
	listenInterface   *net.Interface
	listenIPv4Address *net.IP
	dhcpListener      net.Listener
	dnsServer         *dns.Server
	errorChannel      chan error
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
	log.Printf("Starting service listeners (bound to local network interface '%s' / %s...",
		listeners.service.InterfaceName,
		listeners.listenIPv4Address,
	)

	if listeners.listenInterface == nil {
		return fmt.Errorf("service listeners have not been initialised")
	}

	go func() {
		err := listeners.serveDHCP()
		if err != nil {
			listeners.errorChannel <- err
		}
	}()

	if listeners.service.EnableDNS {
		go func() {
			err := listeners.serveDNS()
			if err != nil {
				listeners.errorChannel <- err
			}
		}()
	}

	return nil
}

// Stop the service listeners.
//
// TODO: Work out how to stop the DHCP listener (may need a custom implementation to make it stoppable).
func (listeners *ServiceListeners) Stop() error {
	log.Printf("Stopping service listeners (bound to local network interface '%s' / %s...",
		listeners.service.InterfaceName,
		listeners.listenIPv4Address,
	)

	if listeners.listenInterface == nil {
		return fmt.Errorf("service listeners have not been initialised")
	}

	if listeners.dnsServer != nil {
		err := listeners.dnsServer.Shutdown()
		if err != nil {
			return err
		}
	}

	return nil
}

func (listeners *ServiceListeners) serveDHCP() error {
	return dhcp.ListenAndServeIf(listeners.service.InterfaceName, listeners.service)
}

func (listeners *ServiceListeners) serveDNS() error {
	listeners.dnsServer = &dns.Server{
		Addr: fmt.Sprintf("%s:%d", listeners.listenIPv4Address, listeners.service.DNSPort),
		Net:  "udp",
	}

	return listeners.dnsServer.ListenAndServe()
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

		addressIP := interfaceAddress.IP
		if len(addressIP) == net.IPv4len {
			listeners.listenIPv4Address = &addressIP

			return nil
		}
	}

	return fmt.Errorf("cannot find an IPv4 address bound to local network interface '%s'", listeners.service.InterfaceName)
}
