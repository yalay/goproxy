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

type Dialer struct {
	sutils.Dialer
	serveraddr string
	username   string
	password   string
	lock       sync.Mutex
	sess       []*Session
}

func NewDialer(dialer sutils.Dialer, serveraddr string,
	username, password string) (d *Dialer, err error) {
	d = &Dialer{
		Dialer:     dialer,
		serveraddr: serveraddr,
		username:   username,
		password:   password,
	}
	return
}

func (d *Dialer) createConn() (conn net.Conn, err error) {
	logger.Noticef("create connect, serveraddr: %s.",
		d.serveraddr)
	conn, err = d.Dialer.Dial("tcp", d.serveraddr)
	if err != nil {
		logger.Err(err)
		return
	}

	logger.Noticef("auth with username: %s, password: %s.",
		d.username, d.password)
	b, err := NewFrameAuth(0, d.username, d.password)
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

func (d *Dialer) createSession() (err error) {
	var conn net.Conn
	d.lock.Lock()
	defer d.lock.Unlock()

	if len(d.sess) > 0 {
		return
	}

	// retry
	for i := uint(0); i < RETRY_TIMES; i++ {
		conn, err = d.createConn()
		if err != nil {
			logger.Err(err)
			time.Sleep((1 << i) * time.Second)
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
	sess.Ping()
	d.sess = append(d.sess, sess)

	go func() {
		sess.Run()
		// that's mean session is dead
		logger.Warning("session runtime quit, reboot from connect.")

		// remove from sess
		d.lock.Lock()
		idx := -1
		for i, o := range d.sess {
			if o == sess {
				idx = i
				break
			}
		}
		if idx == -1 {
			logger.Err("sess %p not found.", sess)
			d.lock.Unlock()
			return
		}
		copy(d.sess[idx:len(d.sess)-1], d.sess[idx+1:])
		d.sess = d.sess[:len(d.sess)-1]
		d.lock.Unlock()

		d.createSession()
	}()
	return
}

func (d *Dialer) GetSess() (sess *Session) {
	// TODO: new session when too many connections.
	switch len(d.sess) {
	case 0:
		err := d.createSession()
		if err != nil {
			logger.Err(err)
			// more civilized
			os.Exit(-1)
		}
		return d.sess[0]
	case 1:
		return d.sess[0]
	default:
		n := rand.Intn(len(d.sess))
		sess = d.sess[n]
		return
	}
	return
}

func FrameOrTimeout(ch chan Frame, t time.Duration) (f Frame) {
	ch_timeout := time.After(t)
	select {
	case f := <-ch:
		return f
	case <-ch_timeout: // timeout
		return nil
	}
}

func (d *Dialer) Dial(network, address string) (conn net.Conn, err error) {
	sess := d.GetSess()
	logger.Infof("try dial: %s => %s.",
		sess.conn.RemoteAddr().String(), address)

	// lock streamid and put chan for it
	ch := make(chan Frame, 1)
	streamid, err := sess.PutIntoNextId(ch)
	if err != nil {
		return
	}

	b, err := NewFrameOneString(MSG_SYN, streamid, address)
	if err != nil {
		logger.Err(err)
		return
	}
	_, err = sess.Write(b)
	if err != nil {
		logger.Err(err)
		return
	}

	fr := FrameOrTimeout(ch, DIAL_TIMEOUT)
	close(ch)

	switch frt := fr.(type) {
	default:
		err = errors.New("unknown status")
	case nil: // close all or timeout
		err = fmt.Errorf("connection failed for timeout(%d) or all closed(%p).", streamid, sess)
	case *FrameFAILED: // FAILED
		err = fmt.Errorf("connection failed for remote failed(%d): %d.",
			streamid, frt.Errno)
	case *FrameOK: // OK
		logger.Info("connect ok.")
	}

	if err != nil {
		logger.Err(err)
		sess.RemovePorts(streamid)
		return
	}

	c := NewConn(streamid, sess)
	sess.PutIntoId(streamid, c.ch_f)
	logger.Noticef("new conn: %p(%d) => %s.",
		sess, streamid, address)
	return c, nil
}

func (d *Dialer) LookupIP(hostname string) (ipaddr []net.IP, err error) {
	logger.Noticef("lookup ip: %s", hostname)
	sess := d.GetSess()

	// lock streamid and put chan for it
	ch := make(chan Frame, 1)
	streamid, err := sess.PutIntoNextId(ch)
	if err != nil {
		return
	}

	b, err := NewFrameOneString(MSG_DNS, streamid, hostname)
	if err != nil {
		logger.Err(err)
		return
	}
	_, err = sess.Write(b)
	if err != nil {
		return
	}

	fr := FrameOrTimeout(ch, LOOKUP_TIMEOUT)
	close(ch)

	switch frt := fr.(type) {
	default:
		err = errors.New("unknown status")
	case nil: // close all or timeout
		err = fmt.Errorf("lookup ip failed for timeout(%d) or all closed(%p).", streamid, sess)
	case *FrameFAILED: // FAILED
		err = fmt.Errorf("lookup ip failed for remote failed(%d): %d.",
			streamid, frt.Errno)
	case *FrameAddr: // OK
		logger.Infof("lookup ip ok.")
		ipaddr = frt.Ipaddr
		return
	}

	logger.Err(err)
	sess.RemovePorts(streamid)
	return
}
