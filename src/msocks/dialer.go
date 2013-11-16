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
	username, password string) (md *MsocksDialer, err error) {
	md = &MsocksDialer{
		Dialer:     dialer,
		serveraddr: serveraddr,
		username:   username,
		password:   password,
	}

	conn, err := md.createConn()
	if err != nil {
		return
	}
	md.sess = NewSession(conn)

	go func() {
		var err error
		var conn net.Conn

		for {
			md.sess.Run()
			// that's mean session is dead
			logger.Info("session runtime quit, reboot from connect.")

			// retry
			for i := 0; i < 3; i++ {
				conn, err = md.createConn()
				if err != nil {
					logger.Err(err)
					continue
				}
			}

			if err != nil {
				panic(err)
			}
			md.sess = NewSession(conn)
		}
	}()
	return
}

func (md *MsocksDialer) createConn() (conn net.Conn, err error) {
	logger.Debugf("dailer create connect: %s.", md.serveraddr)
	conn, err = md.Dialer.Dial("tcp", md.serveraddr)
	if err != nil {
		logger.Err(err)
		return
	}

	logger.Debugf("auth with username: %s, password: %s.",
		md.username, md.password)
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
		logger.Debugf("auth ok.")
	case *FrameFAILED:
		conn.Close()
		err = fmt.Errorf("create connection failed with code: %d.",
			ft.errno)
		logger.Err(err)
		return
	}

	return
}

func (md *MsocksDialer) Dial(network, address string) (conn net.Conn, err error) {
	logger.Infof("dial to: %s.", address)

	// lock streamid and put chan for it
	ch := make(chan int, 0)
	streamid, err := md.sess.PutIntoNextId(ch)
	if err != nil {
		return
	}

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
		err = md.sess.ClosePort(streamid)
		if err != nil {
			logger.Err(err)
			// big trouble
		}
		err = errors.New("connection failed")
		logger.Err(err)
		return
	case 1: // OK
		logger.Debugf("connect ok.")
		c := NewConn(streamid, md.sess)
		md.sess.PutIntoId(streamid, c)
		return c, nil
	}
}
