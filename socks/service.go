package socks

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/shell909090/goproxy/sutils"
	"net"
)

type SocksService struct {
	dialer sutils.Dialer
}

func NewService(dialer sutils.Dialer) (ss *SocksService) {
	return &SocksService{dialer: dialer}
}

func (ss *SocksService) SocksHandler(conn net.Conn) (dstconn net.Conn, err error) {
	logger.Debugf("connection come from: %s => %s",
		conn.RemoteAddr(), conn.LocalAddr())

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
		logger.Err(err)
		return
	}
	logger.Debug("handshark ok")

	hostname, port, err := GetConnect(reader)
	if err != nil {
		// general SOCKS server failure
		SendConnectResponse(writer, 0x01)
		return
	}
	logger.Debugf("dst: %s:%d", hostname, port)

	dstconn, err = ss.dialer.Dial("tcp", fmt.Sprintf("%s:%d", hostname, port))
	if err != nil {
		// Connection refused
		SendConnectResponse(writer, 0x05)
		return
	}
	SendConnectResponse(writer, 0x00)

	return dstconn, nil
}

func (ss *SocksService) Serve(listener net.Listener) (err error) {
	var conn net.Conn

	for {
		conn, err = listener.Accept()
		if err != nil {
			logger.Err(err)
			return
		}
		go func() {
			defer conn.Close()

			dstconn, err := ss.SocksHandler(conn)
			if err != nil {
				return
			}

			sutils.CopyLink(conn, dstconn)
			return
		}()
	}
	return
}
