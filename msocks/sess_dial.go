package msocks

import (
	"net"
	"time"

	"github.com/shell909090/goproxy/sutils"
)

func (s *Session) Dial(network, address string) (c *Conn, err error) {
	c = NewConn(ST_SYN_SENT, 0, s, network, address)
	streamid, err := s.PutIntoNextId(c)
	if err != nil {
		return
	}
	c.streamid = streamid

	log.Info("try dial %s => %s.", s.conn.RemoteAddr().String(), address)
	err = c.WaitForConn()
	if err != nil {
		return
	}

	return c, nil
}

func (s *Session) on_syn(ft *FrameSyn) (err error) {
	// lock streamid temporary, with status sync recved
	c := NewConn(ST_SYN_RECV, ft.Streamid, s, ft.Network, ft.Address)
	err = s.PutIntoId(ft.Streamid, c)
	if err != nil {
		log.Error("%s", err)

		fb := NewFrameResult(ft.Streamid, ERR_IDEXIST)
		err := s.SendFrame(fb)
		if err != nil {
			return err
		}
		return nil
	}

	// it may toke long time to connect with target address
	// so we use goroutine to return back loop
	go func() {
		var err error
		var conn net.Conn
		log.Debug("try to connect %s => %s:%s.", c.String(), ft.Network, ft.Address)

		if dialer, ok := s.dialer.(*sutils.TcpDialer); ok {
			conn, err = dialer.DialTimeout(ft.Network, ft.Address, DIAL_TIMEOUT*time.Second)
		} else {
			conn, err = s.dialer.Dial(ft.Network, ft.Address)
		}

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
		log.Notice("connected %s => %s:%s.", c.String(), ft.Network, ft.Address)
		return
	}()
	return
}
