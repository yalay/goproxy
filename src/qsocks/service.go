package qsocks

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"logging"
	"net"
	"os"
	"strings"
	"sutils"
)

type QsocksService struct {
	userpass     map[string]string
	cryptWrapper func(net.Conn) (net.Conn, error)
}

func LoadPassfile(filename string) (userpass map[string]string, err error) {
	file, err := os.Open(filename)
	if err != nil {
		logging.Err(err)
		return
	}
	defer file.Close()
	userpass = make(map[string]string, 0)

	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		switch err {
		case io.EOF:
			if len(line) == 0 {
				return nil, nil
			}
		case nil:
		default:
			return nil, err
		}
		f := strings.SplitN(line, ":", 2)
		if len(f) < 2 {
			err = fmt.Errorf("format wrong: %s", line)
			logging.Err(err)
			return nil, err
		}
		userpass[strings.Trim(f[0], "\r\n ")] = strings.Trim(f[1], "\r\n ")
	}

	return
}

func NewService(passfile string, cryptWrapper func(net.Conn) (net.Conn, error)) (qs *QsocksService, err error) {
	qs = &QsocksService{cryptWrapper: cryptWrapper}
	if passfile == "" {
		return qs, nil
	}
	qs.userpass, err = LoadPassfile(passfile)
	return
}

func (qs *QsocksService) QsocksHandler(conn net.Conn) (err error) {
	logging.Debug("connection comein")

	if qs.cryptWrapper != nil {
		conn, err = qs.cryptWrapper(conn)
		if err != nil {
			return
		}
	}

	username, password, err := GetAuth(conn)
	if err != nil {
		return
	}

	if qs.userpass != nil {
		password1, ok := qs.userpass[username]
		if !ok || (password != password1) {
			SendResponse(conn, 0x01)
			err = fmt.Errorf("failed with auth: %s:%s", username, password)
			logging.Err(err)
			return
		}
	}
	logging.Debug("qsocks auth passed")

	req, err := GetReq(conn)
	if err != nil {
		return
	}

	switch req {
	case REQ_CONN:
		hostname, port, err := GetConn(conn)
		if err != nil {
			return err
		}

		logging.Debugf("try connect to %s:%d", hostname, port)
		dstconn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", hostname, port))
		if err != nil {
			logging.Err(err)
			return err
		}

		SendResponse(conn, 0)
		sutils.CopyLink(conn, dstconn)
		return err
	case REQ_DNS:
		SendResponse(conn, 0xff)
		err = errors.New("require DNS not support yet")
		logging.Err(err)
		return
	}
	return
}
