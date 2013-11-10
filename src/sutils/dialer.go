package sutils

import (
	"net"
)

type Dialer interface {
	Dial(string, uint16) (net.Conn, error)
}

// func (d *Dialer) DialConn(network, addr string) (c net.Conn, err error) {
// 	addrs := strings.Split(addr, ":")
// 	hostname := addrs[0]
// 	port, err := strconv.Atoi(addrs[1])
// 	if err != nil {
// 		return
// 	}
// 	return d.Dail(hostname, uint16(port))
// }
