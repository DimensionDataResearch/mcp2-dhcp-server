package main

import (
	"fmt"
	"log"

	dhcp "github.com/krolaw/dhcp4"
)

func main() {
	fmt.Println("MCP 2.0 DHCP Server")

	err := dhcp.ListenAndServe(&DHCPHandler{
		LeasesByMACAddress: make(map[string]*Lease),
	})
	if err != nil {
		log.Fatal(err)
	}
}
