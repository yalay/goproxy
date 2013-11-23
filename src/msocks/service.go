package msocks

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

type MsocksService struct {
	userpass map[string]string
	dialer   sutils.Dialer
	sess     *Session
}

func LoadPassfile(filename string) (userpass map[string]string, err error) {
	logger.Noticef("load passfile from file %s.", filename)

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

func NewService(passfile string, dialer sutils.Dialer) (ms *MsocksService, err error) {
	if dialer == nil {
		err = errors.New("empty dialer")
		logger.Err(err)
		return
	}
	ms = &MsocksService{dialer: dialer}

	if passfile == "" {
		return ms, nil
	}
	ms.userpass, err = LoadPassfile(passfile)
	return
}

func (ms *MsocksService) on_conn(network, address string, streamid uint16) (ch chan Frame, err error) {
	conn, err := ms.dialer.Dial("tcp", address)
	if err != nil {
		return
	}

	c := NewConn(streamid, ms.sess)
	go sutils.CopyLink(conn, c)
	return c.ch_f, nil
}

func (ms *MsocksService) on_auth(stream io.ReadWriteCloser) bool {
	f, err := ReadFrame(stream)
	if err != nil {
		logger.Err(err)
		return false
	}

	ft, ok := f.(*FrameAuth)
	if !ok {
		logger.Err("unexpected package type")
		return false
	}

	logger.Noticef("auth with username: %s, password: %s.",
		ft.Username, ft.Password)
	if ms.userpass != nil {
		password1, ok := ms.userpass[ft.Username]
		if !ok || (ft.Password != password1) {
			logger.Err("auth failed.")
			b, err := NewFrameFAILED(ft.Streamid, ERR_AUTH)
			if err != nil {
				logger.Err(err)
				return false
			}
			_, err = stream.Write(b)
			if err != nil {
				logger.Err(err)
				return false
			}
			return false
		}
	}
	b, err := NewFrameOK(ft.Streamid)
	if err != nil {
		logger.Err(err)
		return false
	}
	_, err = stream.Write(b)
	if err != nil {
		logger.Err(err)
		return false
	}

	logger.Info("auth passed.")
	return true
}

func (ms *MsocksService) Handler(conn net.Conn) {
	logger.Noticef("connection come from: %s => %s.",
		conn.RemoteAddr(), conn.LocalAddr())

	if !ms.on_auth(conn) {
		conn.Close()
		return
	}

	ms.sess = NewSession(conn)
	ms.sess.on_conn = ms.on_conn
	ms.sess.Run()
	logger.Noticef("server session quit: %s => %s.",
		conn.RemoteAddr(), conn.LocalAddr())
}

func (ms *MsocksService) Serve(listener net.Listener) (err error) {
	var conn net.Conn

	for {
		conn, err = listener.Accept()
		if err != nil {
			logger.Err(err)
			return
		}
		go func() {
			defer conn.Close()
			ms.Handler(conn)
		}()
	}
	return
}
