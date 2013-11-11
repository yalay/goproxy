package qsocks

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sutils"
)

type QsocksDialer struct {
	sutils.Dialer
	serveraddr string
	username   string
	password   string
}

func NewDialer(dialer sutils.Dialer, serveraddr string,
	username, password string) (qd *QsocksDialer) {
	return &QsocksDialer{
		Dialer:     dialer,
		serveraddr: serveraddr,
		username:   username,
		password:   password,
	}
}

func (d *QsocksDialer) Dial(network, address string) (conn net.Conn, err error) {
	conn, err = d.Dialer.Dial(network, d.serveraddr)
	if err != nil {
		return
	}

	bufAuth, err := Auth(d.username, d.password)
	if err != nil {
		return
	}
	_, err = conn.Write(bufAuth)
	if err != nil {
		return
	}

	idx := strings.LastIndex(address, ":")
	if idx == -1 {
		err = errors.New("invaild address")
		logger.Err(err)
		return
	}
	hostname := address[:idx]
	port, err := strconv.Atoi(address[idx+1:])
	if err != nil {
		logger.Err(err)
		return
	}
	logger.Debugf("dialer %s => %s:%d", d.serveraddr, hostname, port)

	bufConn, err := Conn(hostname, uint16(port))
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
	switch res {
	case 0:
	case 1:
		err = errors.New("auth failed.")
		logger.Err(err)
		return
	default:
		err = fmt.Errorf("response %d", res)
		logger.Err(err)
		return
	}
	return
}
