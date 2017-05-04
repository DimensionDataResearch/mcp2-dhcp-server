package main

import (
	"fmt"
	"log"

	"os"
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

	fmt.Println("Server is running.")
}
