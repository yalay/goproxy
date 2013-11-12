package qsocks

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sutils"
)

type QsocksService struct {
	userpass map[string]string
}

func LoadPassfile(filename string) (userpass map[string]string, err error) {
	logger.Infof("load passfile from file %s.", filename)

	file, err := os.Open(filename)
	if err != nil {
		logger.Err(err)
		return
	}
	defer file.Close()
	userpass = make(map[string]string, 0)

	reader := bufio.NewReader(file)
QUIT:
	for {
		line, err := reader.ReadString('\n')
		switch err {
		case io.EOF:
			if len(line) == 0 {
				break QUIT
			}
		case nil:
		default:
			return nil, err
		}
		f := strings.SplitN(line, ":", 2)
		if len(f) < 2 {
			err = fmt.Errorf("format wrong: %s", line)
			logger.Err(err)
			return nil, err
		}
		userpass[strings.Trim(f[0], "\r\n ")] = strings.Trim(f[1], "\r\n ")
	}

	logger.Infof("userinfo loaded %d record(s).", len(userpass))
	return
}

func NewService(passfile string) (qs *QsocksService, err error) {
	qs = &QsocksService{}
	if passfile == "" {
		return qs, nil
	}
	qs.userpass, err = LoadPassfile(passfile)
	return
}

func (qs *QsocksService) Handler(conn net.Conn) {
	logger.Debugf("connection come from: %s => %s",
		conn.RemoteAddr(), conn.LocalAddr())

	username, password, err := GetAuth(conn)
	if err != nil {
		logger.Err(err)
		return
	}

	logger.Debugf("auth with username: %s, password: %s.", username, password)
	if qs.userpass != nil {
		password1, ok := qs.userpass[username]
		if !ok || (password != password1) {
			SendResponse(conn, 0x01)
			err = fmt.Errorf("failed with auth: %s:%s", username, password)
			logger.Err(err)
			return
		}
	}
	logger.Infof("auth passed with username: %s, password: %s.", username, password)

	req, err := GetReq(conn)
	if err != nil {
		return
	}

	switch req {
	case REQ_CONN:
		hostname, port, err := GetConn(conn)
		if err != nil {
			logger.Err(err)
			return
		}

		logger.Debugf("try connect to: %s:%d", hostname, port)
		dstconn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", hostname, port))
		if err != nil {
			logger.Err(err)
			return
		}

		SendResponse(conn, 0)
		sutils.CopyLink(conn, dstconn)
		logger.Err(err)
		return
	case REQ_DNS:
		SendResponse(conn, 0xff)
		err = errors.New("require DNS not support yet")
		logger.Err(err)
		return
	}
	return
}

func (qs *QsocksService) ServeTCP(listener net.Listener) (err error) {
	var conn net.Conn

	for {
		conn, err = listener.Accept()
		if err != nil {
			logger.Err(err)
			return
		}
		go func() {
			defer conn.Close()
			qs.Handler(conn)
		}()
	}
	return
}
