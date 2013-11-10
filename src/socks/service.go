package socks

import (
	"bufio"
	"errors"
	"logging"
	"net"
)

func SocksHandler(conn net.Conn) (hostname string, port uint16, err error) {
	logging.Debug("connection comein")

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	methods, err := GetHandshake(reader)
	if err != nil {
		return
	}

	method := byte(0xff)
	for _, m := range methods {
		if m == 0 {
			method = 0
		}
	}
	SendHandshakeResponse(writer, method)
	if method == 0xff {
		err = errors.New("auth method wrong")
		logging.Err(err)
		return
	}
	logging.Debug("handshark ok")

	hostname, port, err = GetConnect(reader)
	if err != nil {
		// general SOCKS server failure
		SendConnectResponse(writer, 0x01)
		return
	}
	logging.Debug("dst:", hostname, port)

	return hostname, port, nil
}
