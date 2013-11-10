package qsocks

import (
	"fmt"
	"net"
)

type QsocksDialer struct {
	serveraddr   string
	cryptWrapper func(net.Conn) (net.Conn, error)
	username     string
	password     string
}

func NewDialer(serveraddr string, cryptWrapper func(net.Conn) (net.Conn, error),
	username, password string) (qd *QsocksDialer) {
	return &QsocksDialer{
		serveraddr:   serveraddr,
		cryptWrapper: cryptWrapper,
		username:     username,
		password:     password,
	}
}

func (d *QsocksDialer) Dial(hostname string, port uint16) (conn net.Conn, err error) {
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
