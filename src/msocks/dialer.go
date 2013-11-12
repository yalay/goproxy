package msocks

import (
	"errors"
	"fmt"
	"net"
	"sutils"
)

type MsocksDialer struct {
	sutils.Dialer
	serveraddr string
	username   string
	password   string
	sess       *Session
}

func NewDialer(dialer sutils.Dialer, serveraddr string,
	username, password string) (md *MsocksDialer) {
	md = &MsocksDialer{
		Dialer:     dialer,
		serveraddr: serveraddr,
		username:   username,
		password:   password,
		sess:       NewSession(),
	}

	md.sess.re_conn = md.createConn
	return
}

func (md *MsocksDialer) createConn() (err error) {
	conn, err := md.Dialer.Dial("tcp", md.serveraddr)
	if err != nil {
		logger.Err(err)
		return
	}

	fa := &FrameAuth{
		username: md.username,
		password: md.password,
	}
	err = fa.WriteFrame(conn)
	if err != nil {
		return
	}

	f, err := ReadFrame(conn)
	if err != nil {
		return
	}

	switch ft := f.(type) {
	default:
		err = errors.New("unexpected package")
		logger.Err(err)
		return
	case *FrameOK:
	case *FrameFAILED:
		conn.Close()
		err = fmt.Errorf("create connection failed with code: %d", ft.errno)
		logger.Err(err)
		return err
	}

	md.sess.conn = conn
	go md.sess.Run()
	return
}

func (md *MsocksDialer) Dial(network, address string) (conn net.Conn, err error) {
	// chan
	if md.sess.conn == nil {
		md.createConn()
	}

	streamid, err := md.sess.GetNextId()
	if err != nil {
		return
	}

	// lock streamid and put chan for it
	ch := make(chan int, 0)
	md.sess.ports[streamid] = ch

	f := &FrameSyn{
		streamid: streamid,
		address:  address,
	}
	err = md.sess.WriteFrame(f)
	if err != nil {
		return
	}

	st := <-ch
	close(ch)
	switch st {
	default:
		err = errors.New("unknown status")
		logger.Err(err)
		return
	case 0: // FAILED
		err = errors.New("connection failed")
		logger.Err(err)
		return
	case 1: // OK
	}

	s := &ServiceStream{
		streamid: streamid,
		sess:     md.sess,
		closed:   false,
	}
	return s, nil
}
