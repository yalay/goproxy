package msocks

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"os"
	"sutils"
	"sync"
	"time"
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

	logger.Noticef("create connect, serveraddr: %s.",
		ds.serveraddr)
	conn, err = md.Dialer.Dial("tcp", ds.serveraddr)
	if err != nil {
		logger.Err(err)
		return
	}

	logger.Noticef("auth with username: %s, password: %s.",
		ds.username, ds.password)
	b, err := NewFrameAuth(0, ds.username, ds.password)
	if err != nil {
		logger.Err(err)
		return
	}
	_, err = conn.Write(b)
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
		logger.Notice("auth ok.")
	case *FrameFAILED:
		conn.Close()
		err = fmt.Errorf("create connection failed with code: %d.",
			ft.Errno)
		logger.Err(err)
		return
	}

	return
}

func (md *Dialer) createSession() (err error) {
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
		logger.Crit("can't connect to host, quit.")
		return
	}

	logger.Noticef("create session.")
	sess := NewSession(conn)
	md.sess = append(md.sess, sess)

	go func() {
		sess.Run()
		// that's mean session is dead
		logger.Warning("session runtime quit, reboot from connect.")

		// remove from sess
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
	return
}

func (md *Dialer) GetSess() (sess *Session) {
	// TODO: new session when too many connections.
	if len(md.sess) == 0 {
		err := md.createSession()
		if err != nil {
			logger.Err(err)
			// more civilized
			os.Exit(-1)
		}
	}
	n := rand.Intn(len(md.sess))
	sess = md.sess[n]
	return
}

func (md *Dialer) Dial(network, address string) (conn net.Conn, err error) {
	sess := md.GetSess()
	logger.Infof("try dial: %s => %s.",
		sess.conn.RemoteAddr().String(), address)

	// lock streamid and put chan for it
	ch := make(chan Frame, 1)
	streamid, err := sess.PutIntoNextId(ch)
	if err != nil {
		return
	}

	b, err := NewFrameSyn(streamid, address)
	if err != nil {
		logger.Err(err)
		return
	}
	_, err = sess.Write(b)
	if err != nil {
		logger.Err(err)
		return
	}

	ch_timeout := time.After(DIAL_TIMEOUT)

	select {
	case fr := <-ch:
		switch frt := fr.(type) {
		default:
			err = errors.New("unknown status")
			logger.Err(err)
			return
		case nil: // close all
			err = fmt.Errorf("connection failed for all closed(%p).", sess)
			break
		case *FrameFAILED: // FAILED
			err = fmt.Errorf("connection failed for remote failed(%d): %d.",
				streamid, frt.Errno)
			break
		case *FrameOK: // OK
			logger.Info("connect ok.")
			c := NewConn(streamid, sess)
			sess.PutIntoId(streamid, c.ch_f)
			close(ch)
			logger.Noticef("new conn: %p(%d) => %s.",
				sess, streamid, address)
			return c, nil
		}
	case <-ch_timeout: // timeout
		err = fmt.Errorf("connection failed for timeout(%d).", streamid)
		break
	}

	logger.Err(err)
	sess.RemovePorts(streamid)
	close(ch)
	return
}

func (md *Dialer) LookupIP(hostname string) (ipaddr []net.IP, err error) {
	logger.Noticef("lookup ip: %s", hostname)
	sess := md.GetSess()

	// lock streamid and put chan for it
	ch := make(chan Frame, 1)
	streamid, err := sess.PutIntoNextId(ch)
	if err != nil {
		return
	}

	b, err := NewFrameDns(streamid, hostname)
	if err != nil {
		logger.Err(err)
		return
	}
	_, err = sess.Write(b)
	if err != nil {
		return
	}

	ch_timeout := time.After(LOOKUP_TIMEOUT)

	select {
	case fr := <-ch:
		switch frt := fr.(type) {
		default:
			err = errors.New("unknown status")
			logger.Err(err)
			return
		case nil: // close all
			err = fmt.Errorf("lookup ip failed for all closed(%p).", sess)
			break
		case *FrameFAILED: // FAILED
			err = fmt.Errorf("lookup ip failed for remote failed(%d): %d.",
				streamid, frt.Errno)
			break
		case *FrameAddr: // OK
			logger.Infof("lookup ip ok.")
			ipaddr = frt.Ipaddr
			close(ch)
			return
		}
	case <-ch_timeout: // timeout
		err = fmt.Errorf("lookup ip failed for timeout(%d).", streamid)
		break
	}

	logger.Err(err)
	sess.RemovePorts(streamid)
	close(ch)
	return
}
