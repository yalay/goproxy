package sutils

import (
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
