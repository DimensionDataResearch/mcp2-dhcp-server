package main

import (
	"fmt"
	"log"

	dhcp "github.com/krolaw/dhcp4"
)

func main() {
	service := NewService()
	err := service.Initialize()
	if err != nil {
		panic(err)
	}

	// Start polling CloudControl for server metadata.
	service.Start()

	fmt.Println("MCP 2.0 DHCP server is running.")
	err = dhcp.ListenAndServe(service)
	if err != nil {
		log.Fatal(err)
	}
}
