package main

import (
	"log"

	"github.com/miekg/dns"
)

// ServeDNS handles an incoming DNS request.
func (service *Service) ServeDNS(send dns.ResponseWriter, request *dns.Msg) {
	data := service.DNSData

	// TODO: Add logging

	if len(request.Question) != 1 {
		// Anything we don't know how to handle, we just pass on to the fallback server.
		service.dnsFallback(send, request)

		return
	}

	question := request.Question[0]

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
			service.dnsSendNonExistentDomain(send, request)
		}

		break

	default:
		// Anything we don't know how to handle, we just pass on to the fallback server.
		service.dnsFallback(send, request)

		break
	}
}

func (service *Service) dnsSendResourceRecord(record dns.RR, send dns.ResponseWriter, request *dns.Msg) {
	response := new(dns.Msg)
	response.SetReply(request)
	response.Authoritative = true
	response.Answer = []dns.RR{record}

	send.WriteMsg(response)
}

func (service *Service) dnsSendServerFailure(send dns.ResponseWriter, request *dns.Msg) {
	response := new(dns.Msg)
	response.SetRcode(request, dns.RcodeServerFailure)

	send.WriteMsg(response)
}

func (service *Service) dnsSendNonExistentDomain(send dns.ResponseWriter, request *dns.Msg) {
	response := new(dns.Msg)
	response.SetRcode(request, dns.RcodeServerFailure)

	send.WriteMsg(response)
}

func (service *Service) dnsFallback(send dns.ResponseWriter, request *dns.Msg) {
	// TODO: Consider implementing a basic cache for forwarded requests / responses.

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
}
