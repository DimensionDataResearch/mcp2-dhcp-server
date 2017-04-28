package main

import (
	"fmt"
	"log"

	"os"

	dhcp "github.com/krolaw/dhcp4"
)

func main() {
	log.SetOutput(os.Stdout)

	log.Printf("MCP 2.0 DHCP server " + ProductVersion)

	fmt.Println("Server is initialising...")
	service := NewService()
	err := service.Initialize()
	if err != nil {
		panic(err)
	}

	fmt.Println("Server is starting...")
	service.Start()

	fmt.Println("Server is running.")
	err = dhcp.ListenAndServeIf(service.InterfaceName, service)
	if err != nil {
		log.Fatal(err)
	}
}
