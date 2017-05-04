package main

import (
	"fmt"
	"net"

	"github.com/DimensionDataResearch/go-dd-cloud-compute/compute"
	"github.com/miekg/dns"
)

// DNSData holds all data required for handling DNS requests.
type DNSData struct {
	v4Addresses    map[string]dns.A
	v6Addresses    map[string]dns.AAAA
	reverseLookups map[string]dns.PTR

	DefaultTTLSeconds uint32
}

// NewDNSData creates a new DNSData.
func NewDNSData() DNSData {
	return DNSData{
		v4Addresses:       make(map[string]dns.A),
		v6Addresses:       make(map[string]dns.AAAA),
		reverseLookups:    make(map[string]dns.PTR),
		DefaultTTLSeconds: 60,
	}
}

// FindA retrieves the A record (if one exists) for the specified name.
func (data *DNSData) FindA(name string) *dns.A {
	record, ok := data.v4Addresses[name]
	if ok {
		return &record
	}

	return nil
}

// FindAAAA retrieves the AAAA record (if one exists) for the specified name.
func (data *DNSData) FindAAAA(name string) *dns.AAAA {
	record, ok := data.v6Addresses[name]
	if ok {
		return &record
	}

	return nil
}

// FindPTR retrieves the PTR record (if one exists) for the specified ".arpa" address.
func (data *DNSData) FindPTR(arpa string) *dns.PTR {
	record, ok := data.reverseLookups[arpa]
	if ok {
		return &record
	}

	return nil
}

// Add a new set of records for the specified name and IPv4 / IPv6 address.
func (data *DNSData) Add(name string, ip net.IP) error {
	switch len(ip) {
	case net.IPv4len:
		data.addA(name, ip)

		return data.addPTR(name, ip)
	case net.IPv6len:
		data.addAAAA(name, ip)

		return data.addPTR(name, ip)
	default:
		return fmt.Errorf("IP address '%s' has unexpected length (%d)", ip, len(ip))
	}
}

// AddServer adds or updates the records for the specified CloudControl server.
func (data *DNSData) AddServer(server compute.Server) {
	data.AddNetworkAdapter(server.Name, server.Network.PrimaryAdapter)

	for _, additionalNetworkAdapter := range server.Network.AdditionalNetworkAdapters {
		data.AddNetworkAdapter(server.Name, additionalNetworkAdapter)
	}
}

// AddNetworkAdapter adds or updates the records for the specified CloudControl virtual network adapter.
func (data *DNSData) AddNetworkAdapter(name string, networkAdapter compute.VirtualMachineNetworkAdapter) {
	if networkAdapter.PrivateIPv4Address != nil {
		data.Add(name,
			net.ParseIP(*networkAdapter.PrivateIPv4Address),
		)
	}
	if networkAdapter.PrivateIPv6Address != nil {
		data.Add(name,
			net.ParseIP(*networkAdapter.PrivateIPv6Address),
		)
	}
}

// Remove any records that exist for the specified name.
func (data *DNSData) Remove(name string) error {
	aRecord, ok := data.v4Addresses[name]
	if ok {
		arpa, err := dns.ReverseAddr(aRecord.A.String())
		if err != nil {
			return err
		}

		delete(data.reverseLookups, arpa)
	}
	delete(data.v4Addresses, name)

	aaaaRecord, ok := data.v6Addresses[name]
	if ok {
		arpa, err := dns.ReverseAddr(aaaaRecord.AAAA.String())
		if err != nil {
			return err
		}

		delete(data.reverseLookups, arpa)
	}
	delete(data.v6Addresses, name)

	return nil
}

// RemoveServer removes any records that exist for the specified CloudControl server.
func (data *DNSData) RemoveServer(server compute.Server) {
	data.Remove(server.Name)
}

// Add an A record.
func (data *DNSData) addA(name string, ip net.IP) {
	data.v4Addresses[name] = dns.A{
		Hdr: dns.RR_Header{
			Name:   name,
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    data.DefaultTTLSeconds,
		},
		A: ip,
	}
}

// Add an AAAA record.
func (data *DNSData) addAAAA(name string, ip net.IP) {
	data.v6Addresses[name] = dns.AAAA{
		Hdr: dns.RR_Header{
			Name:   name,
			Rrtype: dns.TypeAAAA,
			Class:  dns.ClassINET,
			Ttl:    data.DefaultTTLSeconds,
		},
		AAAA: ip,
	}
}

// Add a PTR record.
func (data *DNSData) addPTR(name string, ip net.IP) error {
	arpa, err := dns.ReverseAddr(ip.String())
	if err != nil {
		return err
	}

	data.reverseLookups[arpa] = dns.PTR{
		Hdr: dns.RR_Header{
			Name:   arpa,
			Rrtype: dns.TypePTR,
			Class:  dns.ClassINET,
			Ttl:    data.DefaultTTLSeconds,
		},
		Ptr: name,
	}

	return nil
}
