package main

import (
	"fmt"
	"log"

	"os"

	dhcp "github.com/krolaw/dhcp4"
)

func main() {
	log.SetOutput(os.Stdout)

	fmt.Println("MCP 2.0 DHCP server is initialising...")
	service := NewService()
	err := service.Initialize()
	if err != nil {
		panic(err)
	}

	fmt.Println("MCP 2.0 DHCP server is starting...")

	// Start polling CloudControl for server metadata.
	service.Start()

	fmt.Println("MCP 2.0 DHCP server is running.")
	err = dhcp.ListenAndServe(service)
	if err != nil {
		log.Fatal(err)
	}
}
