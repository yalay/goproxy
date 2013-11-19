package msocks

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"sutils"
	"sync"
)

type DialerSetting struct {
	serveraddr string
	username   string
	password   string
}

type Dialer struct {
	sutils.Dialer
	dss      []*DialerSetting
	sesslock sync.Mutex
	sess     []*Session
}

func NewDialer(dialer sutils.Dialer, serveraddr string,
	username, password string) (md *Dialer, err error) {
	ds := &DialerSetting{
		serveraddr: serveraddr,
		username:   username,
		password:   password,
	}
	md = &Dialer{
		Dialer: dialer,
	}
	md.dss = append(md.dss, ds)

	return
}

func (md *Dialer) NewDialerSetting(serveraddr string,
	username, password string) (err error) {
	ds := &DialerSetting{
		serveraddr: serveraddr,
		username:   username,
		password:   password,
	}
	md.dss = append(md.dss, ds)
	return
}

func (md *Dialer) createConn() (conn net.Conn, err error) {
	n := rand.Intn(len(md.dss))
	ds := md.dss[n]

	logger.Infof("create connect, serveraddr: %s.",
		ds.serveraddr)
	conn, err = md.Dialer.Dial("tcp", ds.serveraddr)
	if err != nil {
		logger.Err(err)
		return
	}

	logger.Infof("auth with username: %s, password: %s.",
		ds.username, ds.password)
	fa := NewFrameAuth(0, ds.username, ds.password)
	err = WriteFrame(conn, fa)
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
		logger.Infof("auth ok.")
	case *FrameFAILED:
		conn.Close()
		err = fmt.Errorf("create connection failed with code: %d.",
			ft.Errno)
		logger.Err(err)
		return
	}

	return
}

func (md *Dialer) createSession() {
	var err error
	var conn net.Conn
	md.sesslock.Lock()
	defer md.sesslock.Unlock()

	if len(md.sess) > 0 {
		return
	}

	// retry
	for i := 0; i < 3; i++ {
		conn, err = md.createConn()
		if err != nil {
			logger.Err(err)
		} else {
			break
		}

	}
	if err != nil {
		return
	}

	logger.Debugf("create session.")
	sess := NewSession(conn)
	md.sess = append(md.sess, sess)

	go func() {
		sess.Run()
		// that's mean session is dead
		logger.Info("session runtime quit, reboot from connect.")

		idx := -1
		for i, o := range md.sess {
			if o == sess {
				idx = i
				break
			}
		}
		if idx == -1 {
			logger.Err("sess %p not found.", sess)
			return
		}

		copy(md.sess[idx:len(md.sess)-1], md.sess[idx+1:])
		md.sess = md.sess[:len(md.sess)-1]

		md.createSession()
	}()
}

func (md *Dialer) GetSess() (sess *Session) {
	// TODO: new session when too many connections.
	if len(md.sess) == 0 {
		md.createSession()
	}
	n := rand.Intn(len(md.sess))
	sess = md.sess[n]
	return
}

func (md *Dialer) Dial(network, address string) (conn net.Conn, err error) {
	sess := md.GetSess()
	logger.Infof("dial: %s => %s.",
		sess.conn.RemoteAddr().String(), address)

	// lock streamid and put chan for it
	ch := make(chan Frame, 0)
	streamid, err := sess.PutIntoNextId(ch)
	if err != nil {
		return
	}

	f := NewFrameSyn(streamid, address)
	err = sess.WriteFrame(f)
	if err != nil {
		return
	}

	// TODO: timeout?
	fr := <-ch
	switch fr.(type) {
	default:
		err = errors.New("unknown status")
		logger.Err(err)
		return
	case nil: // close all
		break
	case *FrameFAILED: // FAILED
		break
	case *FrameOK: // OK
		logger.Debugf("connect ok.")
		c := NewConn(streamid, sess)
		sess.PutIntoId(streamid, c)
		close(ch)
		return c, nil
	}

	err = sess.RemovePorts(streamid)
	if err != nil {
		logger.Err(err)
	}
	err = errors.New("connection failed.")
	logger.Err(err)
	close(ch)
	return
}

func (md *Dialer) LookupIP(hostname string) (ipaddr []net.IP, err error) {
	logger.Infof("lookup ip: %s", hostname)
	sess := md.GetSess()

	// lock streamid and put chan for it
	ch := make(chan Frame, 0)
	streamid, err := sess.PutIntoNextId(ch)
	if err != nil {
		return
	}

	f := NewFrameDns(streamid, hostname)
	err = sess.WriteFrame(f)
	if err != nil {
		return
	}

	// TODO: timeout?
	fr := <-ch
	switch frt := fr.(type) {
	default:
		err = errors.New("unknown status")
		logger.Err(err)
		return
	case nil: // close all
		break
	case *FrameFAILED: // FAILED
		break
	case *FrameAddr: // OK
		logger.Debugf("lookup ip ok.")
		ipaddr = frt.Ipaddr
		close(ch)
		return
	}

	err = sess.RemovePorts(streamid)
	if err != nil {
		logger.Err(err)
	}
	err = errors.New("lookup ip failed")
	logger.Err(err)
	close(ch)
	return
}
