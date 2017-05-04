package main

import (
	"log"

	"strings"

	"github.com/miekg/dns"
)

// ServeDNS handles an incoming DNS request.
func (service *Service) ServeDNS(send dns.ResponseWriter, request *dns.Msg) {
	data := service.DNSData

	if service.EnableDebugLogging {
		log.Printf("Received DNS query %d: %s", request.Id, request)
	}

	if len(request.Question) != 1 {
		// Anything we don't know how to handle, we just pass on to the fallback server.
		service.dnsFallback(send, request)

		return
	}

	question := request.Question[0]
	if service.shouldForward(question) {
		// Anything we don't know how to handle, we just pass on to the fallback server.
		service.dnsFallback(send, request)
	}

	switch question.Qtype {
	case dns.TypeA:
		typeARecord := data.FindA(question.Name)
		if typeARecord != nil {
			service.dnsSendResourceRecord(typeARecord, send, request)
		} else {
			service.dnsSendNonExistentDomain(send, request)
		}

		break

	case dns.TypeAAAA:
		typeAAAARecord := data.FindAAAA(question.Name)
		if typeAAAARecord != nil {
			service.dnsSendResourceRecord(typeAAAARecord, send, request)
		} else {
			service.dnsSendNonExistentDomain(send, request)
		}

		break

	case dns.TypePTR:
		typePTRRecord := data.FindPTR(question.Name)
		if typePTRRecord != nil {
			service.dnsSendResourceRecord(typePTRRecord, send, request)
		} else {
			// For PTR (reverse-lookup), we pass it on to the fallback server if there's no match locally.
			service.dnsFallback(send, request)
		}

		break

	default:
		// Anything we don't know how to handle, we just pass on to the fallback server.
		service.dnsFallback(send, request)

		break
	}
}

func (service *Service) shouldForward(question dns.Question) bool {
	if question.Qtype == dns.TypePTR {
		log.Printf("shouldForward = false (question.Qtype == dns.TypePTR))")
		return false // We always have a go at satisfying PTR queries, and only forward if we can't answer them.
	}

	isExternalDomain := !strings.HasSuffix(question.Name, service.DNSSuffix)
	log.Printf("shouldForward = %t ('%s', '%s'))",
		isExternalDomain, question.Name, service.DNSSuffix,
	)

	return isExternalDomain
}

func (service *Service) dnsSendResourceRecord(record dns.RR, send dns.ResponseWriter, request *dns.Msg) {
	if service.EnableDebugLogging {
		log.Printf("Replied with resource record to DNS query %d: %s", request.Id, record.Header())
	}

	response := new(dns.Msg)
	response.SetReply(request)
	response.Authoritative = true
	response.Answer = []dns.RR{record}

	send.WriteMsg(response)
}

func (service *Service) dnsSendServerFailure(send dns.ResponseWriter, request *dns.Msg) {
	if service.EnableDebugLogging {
		log.Printf("Replied SERVFAIL to DNS query %d.", request.Id)
	}

	response := new(dns.Msg)
	response.SetRcode(request, dns.RcodeServerFailure)

	send.WriteMsg(response)
}

func (service *Service) dnsSendNonExistentDomain(send dns.ResponseWriter, request *dns.Msg) {
	if service.EnableDebugLogging {
		log.Printf("Replied NXDOMAIN to DNS query %d.", request.Id)
	}

	response := new(dns.Msg)
	response.SetRcode(request, dns.RcodeServerFailure)

	send.WriteMsg(response)
}

func (service *Service) dnsFallback(send dns.ResponseWriter, request *dns.Msg) {
	// TODO: Consider implementing a basic cache for forwarded requests / responses.

	if service.EnableDebugLogging {
		log.Printf("Forwarding unhandled DNS query %d to %s...", request.Id, service.DNSFallbackAddress)
	}

	response, _, err := service.dnsFallbackClient.Exchange(request, service.DNSFallbackAddress)
	if err != nil {
		log.Printf("Unable to forward DNS request %d to '%s': %s ",
			request.Id, service.DNSFallbackAddress, err.Error(),
		)
	}
	response.Authoritative = false

	err = send.WriteMsg(response)
	if err != nil {
		log.Printf("Unable to forward DNS response %d to '%s': %s ",
			response.Id, service.DNSFallbackAddress, err.Error(),
		)
	}

	if service.EnableDebugLogging {
		log.Printf("Forwarded unhandled DNS query %d.", request.Id)
	}
}
