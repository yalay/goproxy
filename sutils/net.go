package sutils

import (
	"github.com/miekg/dns"
	"net"
)

type Dialer interface {
	Dial(string, string) (net.Conn, error)
}

type TcpDialer struct {
}

func (td *TcpDialer) Dial(network, address string) (conn net.Conn, err error) {
	return net.Dial(network, address)
}

var DefaultTcpDialer = &TcpDialer{}

type Lookuper interface {
	LookupIP(host string) (addrs []net.IP, err error)
}

type NetLookupIP struct {
}

func (n *NetLookupIP) LookupIP(host string) (addrs []net.IP, err error) {
	return net.LookupIP(host)
}

type DnsLookup struct {
	sockaddr string
	c        *dns.Client
}

func NewDnsLookup(sockaddr string, dnsnet string) (d *DnsLookup) {
	d = &DnsLookup{
		sockaddr: sockaddr,
	}
	d.c = new(dns.Client)
	d.c.Net = dnsnet
	return d
}

func (d *DnsLookup) query(host string, t uint16, as []net.IP) (addrs []net.IP, err error) {
	addrs = as

	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(host), t)
	m.RecursionDesired = true

	r, _, err := d.c.Exchange(m, d.sockaddr)
	if err != nil {
		return
	}

	for _, a := range r.Answer {
		switch ta := a.(type) {
		case *dns.A:
			addrs = append(addrs, ta.A)
		case *dns.AAAA:
			addrs = append(addrs, ta.AAAA)
		}
	}
	return
}

func (d *DnsLookup) LookupIP(host string) (addrs []net.IP, err error) {
	addrs, err = d.query(host, dns.TypeA, addrs)
	if err != nil {
		return
	}
	addrs, err = d.query(host, dns.TypeAAAA, addrs)
	return
}

var DefaultLookuper = &NetLookupIP{}
