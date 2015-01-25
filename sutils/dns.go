package sutils

import (
	"net"

	"github.com/miekg/dns"
)

type DnsLookup struct {
	Servers []string
	c       *dns.Client
}

func NewDnsLookup(Servers []string, dnsnet string) (d *DnsLookup) {
	d = &DnsLookup{
		Servers: Servers,
	}
	d.c = new(dns.Client)
	d.c.Net = dnsnet
	return d
}

func (d *DnsLookup) Exchange(m *dns.Msg) (r *dns.Msg, err error) {
	for _, srv := range d.Servers {
		r, _, err = d.c.Exchange(m, srv)
		if err != nil {
			continue
		}
		if len(r.Answer) > 0 {
			return
		}
	}
	return
}

func (d *DnsLookup) query(host string, t uint16, as []net.IP) (addrs []net.IP, err error) {
	addrs = as

	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(host), t)
	m.RecursionDesired = true

	r, err := d.Exchange(m)
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
