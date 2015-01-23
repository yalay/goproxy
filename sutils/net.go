package sutils

import (
	"net"

	"github.com/miekg/dns"
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

var DefaultLookuper Lookuper

func init() {
	conf, err := dns.ClientConfigFromFile("/etc/resolv.conf")
	if err != nil {
		return
	}

	var addrs []string
	for _, srv := range conf.Servers {
		addrs = append(addrs, net.JoinHostPort(srv, conf.Port))
	}

	DefaultLookuper = NewDnsLookup(addrs, "")
}
