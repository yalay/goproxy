package msocks

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/shell909090/goproxy/sutils"
)

func DialSession(conn net.Conn, username, password string) (s *Session, err error) {
	ti := time.AfterFunc(AUTH_TIMEOUT*time.Millisecond, func() {
		log.Notice("wait too long time for auth, close conn %s.", conn.RemoteAddr())
		conn.Close()
	})
	defer func() {
		ti.Stop()
	}()

	log.Notice("auth with username: %s, password: %s.", username, password)
	fb := NewFrameAuth(0, username, password)
	buf, err := fb.Packed()
	if err != nil {
		return
	}

	_, err = conn.Write(buf.Bytes())
	if err != nil {
		return
	}

	f, err := ReadFrame(conn)
	if err != nil {
		return
	}

	ft, ok := f.(*FrameResult)
	if !ok {
		err = errors.New("unexpected package")
		log.Error("%s", err)
		return
	}

	if ft.Errno != ERR_NONE {
		conn.Close()
		err = fmt.Errorf("create connection failed with code: %d.",
			ft.Errno)
		log.Error("%s", err)
		return
	}

	log.Notice("auth ok.")
	s = NewSession(conn)
	s.pong()

	return
}

func (s *Session) Dial(network, address string) (c *Conn, err error) {
	c = NewConn(ST_SYN_SENT, 0, s, network, address)
	streamid, err := s.PutIntoNextId(c)
	if err != nil {
		return
	}
	c.streamid = streamid

	log.Info("try dial: %s => %s.",
		s.conn.RemoteAddr().String(), address)
	err = c.WaitForConn()
	if err != nil {
		return
	}

	return c, nil
}

func (s *Session) on_syn(ft *FrameSyn) bool {
	// lock streamid temporary, with status sync recved
	c := NewConn(ST_SYN_RECV, ft.Streamid, s, ft.Network, ft.Address)
	err := s.PutIntoId(ft.Streamid, c)
	if err != nil {
		log.Error("%s", err)

		fb := NewFrameResult(ft.Streamid, ERR_IDEXIST)
		err := s.SendFrame(fb)
		if err != nil {
			log.Error("%s", err)
			return false
		}
		return true
	}

	// it may toke long time to connect with target address
	// so we use goroutine to return back loop
	go func() {
		log.Debug("%s(%d) try to connect: %s:%s.",
			s.GetId(), ft.Streamid, ft.Network, ft.Address)

		// TODO: timeout
		conn, err := s.dialer.Dial(ft.Network, ft.Address)
		if err != nil {
			log.Error("%s", err)
			fb := NewFrameResult(ft.Streamid, ERR_CONNFAILED)
			err = s.SendFrame(fb)
			if err != nil {
				log.Error("%s", err)
			}
			c.Final()
			return
		}

		fb := NewFrameResult(ft.Streamid, ERR_NONE)
		err = s.SendFrame(fb)
		if err != nil {
			log.Error("%s", err)
			return
		}
		c.status = ST_EST

		go sutils.CopyLink(conn, c)
		log.Notice("server side %s(%d) connected %s.",
			s.GetId(), ft.Streamid, ft.Address)
		return
	}()
	return true
}
