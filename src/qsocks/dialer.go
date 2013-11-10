package qsocks

import (
	"fmt"
	"net"
)

type Dialer struct {
	serveraddr   string
	cryptWrapper func(net.Conn) (net.Conn, error)
	username     string
	password     string
}

func NewDialer(serveraddr string, cryptWrapper func(net.Conn) (net.Conn, error),
	username, password string) (d *Dialer) {
	return &Dialer{
		serveraddr:   serveraddr,
		cryptWrapper: cryptWrapper,
		username:     username,
		password:     password,
	}
}

func (d *Dialer) Dial(hostname string, port uint16) (conn net.Conn, err error) {
	conn, err = net.Dial("tcp", d.serveraddr)
	if err != nil {
		return
	}

	if d.cryptWrapper != nil {
		conn, err = d.cryptWrapper(conn)
		if err != nil {
			return
		}
	}

	bufAuth, err := Auth(d.username, d.password)
	if err != nil {
		return
	}
	_, err = conn.Write(bufAuth)
	if err != nil {
		return
	}

	bufConn, err := Conn(hostname, port)
	if err != nil {
		return
	}
	_, err = conn.Write(bufConn)
	if err != nil {
		return
	}

	res, err := RecvResponse(conn)
	if err != nil {
		return
	}
	if res != 0 {
		return nil, fmt.Errorf("qsocks response %d", res)
	}
	return
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
