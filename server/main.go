package main

import (
	"fmt"
	"log"

	"os"

	"net"

	dhcp "github.com/krolaw/dhcp4"
	dns "github.com/miekg/dns"
)

func main() {
	log.SetOutput(os.Stdout)

	log.Printf("MCP 2.0 DHCP server " + ProductVersion)

	fmt.Println("Server is initialising...")
	service := NewService()
	err := service.Initialize()
	if err != nil {
		log.Panic(err)
	}

	fmt.Println("Server is starting...")
	service.Start()

	listenInterface, err := net.InterfaceByName(service.InterfaceName)
	if err != nil {
		log.Panicf("Cannot find local network interface named '%s'.",
			service.InterfaceName,
		)
	}

	listenInterfaceIP, err := findFirstIPv4Address(listenInterface)
	if err != nil {
		log.Panicf("Cannot determine address for local network interface named '%s'.",
			service.InterfaceName,
		)
	}

	log.Print("Starting DHCP...")
	go func() {
		err := dhcp.ListenAndServeIf(service.InterfaceName, service)
		if err != nil {
			log.Panic(err)
		}
	}()

	if service.EnableDNS {
		log.Print("Starting DNS...")
		go func() {
			listenAddress := fmt.Sprintf("%s:%d",
				listenInterfaceIP, service.DNSPort,
			)

			// TODO: Explicitly create and store our server as a field in Service.
			server := &dns.Server{Addr: listenAddress, Net: "udp"}
			err = server.ListenAndServe()
			if err != nil {
				log.Panic(err)
			}
		}()
	}

	fmt.Println("Server is running.")
}

func findFirstIPv4Address(targetInterface *net.Interface) (net.IP, error) {
	addresses, err := targetInterface.Addrs()
	if err != nil {
		return nil, err
	}

	for _, address := range addresses {
		addressIP := net.ParseIP(address.String())
		if len(addressIP) == net.IPv4len {
			return addressIP, nil
		}
	}

	return nil, nil
}
