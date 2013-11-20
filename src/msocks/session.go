package msocks

import (
	"errors"
	"fmt"
	"logging"
	"net"
	"sync"
	"time"
)

const (
	IDLECLOSE = 10 * time.Minute
)

var logger logging.Logger

func init() {
	var err error
	logger, err = logging.NewFileLogger("default", -1, "msocks")
	if err != nil {
		panic(err)
	}
}

type Session struct {
	flock sync.Mutex
	conn  net.Conn

	// lock ports before any ports op and id op
	plock   sync.Mutex
	next_id uint16
	ports   map[uint16]interface{}

	on_conn func(string, string, uint16) (*Conn, error)

	delayclose *time.Timer
}

func NewSession(conn net.Conn) (s *Session) {
	s = &Session{
		conn:  conn,
		ports: make(map[uint16]interface{}, 0),
	}
	// close session after 10 mins.
	s.delayclose = time.AfterFunc(IDLECLOSE, func() {
		s.conn.Close()
	})
	return
}

func (s *Session) LocalAddr() net.Addr {
	return s.conn.LocalAddr()
}

func (s *Session) RemoteAddr() net.Addr {
	return s.conn.RemoteAddr()
}

func (s *Session) PutIntoNextId(i interface{}) (id uint16, err error) {
	s.plock.Lock()
	defer s.plock.Unlock()

	startid := s.next_id
	_, ok := s.ports[s.next_id]
	for ok {
		s.next_id += 1
		if s.next_id == startid {
			err = errors.New("run out of stream id")
			logger.Err(err)
			return
		}
		_, ok = s.ports[s.next_id]
	}
	id = s.next_id
	logger.Debugf("put into next id(%d): %p.", id, i)
	s.next_id += 1

	s.ports[id] = i
	s.delayclose.Stop()
	return
}

func (s *Session) PutIntoId(id uint16, i interface{}) (err error) {
	logger.Debugf("put into id(%d): %p.", id, i)
	s.plock.Lock()
	defer s.plock.Unlock()

	s.ports[id] = i
	return
}

func (s *Session) WriteFrame(f Frame) (err error) {
	s.flock.Lock()
	defer s.flock.Unlock()
	return WriteFrame(s.conn, f)
}

func (s *Session) RemovePorts(streamid uint16) (err error) {
	logger.Noticef("remove ports: %p(%d).", s, streamid)
	s.plock.Lock()
	defer s.plock.Unlock()
	_, ok := s.ports[streamid]
	if ok {
		delete(s.ports, streamid)
	} else {
		err = fmt.Errorf("streamid not exist: %d.", streamid)
		logger.Err(err)
	}
	if len(s.ports) == 0 {
		s.delayclose.Reset(IDLECLOSE)
	}
	return
}

func (s *Session) Number() (n int) {
	return len(s.ports)
}

func (s *Session) Close() (err error) {
	logger.Warningf("close all connections for session(%d): %p.", len(s.ports), s)
	s.plock.Lock()
	defer s.plock.Unlock()

	for _, v := range s.ports {
		switch vt := v.(type) {
		case chan Frame:
			vt <- nil
		case *Conn:
			vt.MarkClose()
		}
	}

	s.ports = make(map[uint16]interface{}, 0)
	return
}

func (s *Session) on_syn(ft *FrameSyn) bool {
	_, ok := s.ports[ft.Streamid]
	if ok {
		logger.Err("frame sync stream id exist.")
		fr := NewFrameFAILED(ft.Streamid, ERR_IDEXIST)
		s.WriteFrame(fr)
		return true
	}

	// lock streamid temporary
	s.ports[ft.Streamid] = 1

	go func() {
		logger.Noticef("client(%p) try to connect: %s.", s, ft.Address)
		stream, err := s.on_conn("tcp", ft.Address, ft.Streamid)
		if err != nil {
			logger.Err(err)
			fr := NewFrameFAILED(ft.Streamid, ERR_CONNFAILED)
			s.WriteFrame(fr)

			s.RemovePorts(ft.Streamid)
			return
		}

		// update it, don't need to lock
		s.ports[ft.Streamid] = stream
		fr := NewFrameOK(ft.Streamid)
		err = s.WriteFrame(fr)
		if err != nil {
			logger.Err(err)
			return
		}
		logger.Debug("connect successed.")
		return
	}()
	return true
}

func (s *Session) on_fin(ft *FrameFin) bool {
	i, ok := s.ports[ft.Streamid]
	if !ok {
		logger.Err("frame fin stream id not exist")
		return true
	}
	it, ok := i.(*Conn)
	if !ok {
		logger.Err("unexpected ports type")
		return false
	}
	err := it.OnClose()
	if err != nil {
		logger.Err(err)
		return false
	}
	return true
}

func (s *Session) on_data(ft *FrameData) bool {
	i, ok := s.ports[ft.Streamid]
	if !ok {
		logger.Err("frame data stream id not exist")
		return true
	}

	it, ok := i.(*Conn)
	if !ok {
		logger.Err("unexpected ports type")
		return false
	}

	// never use ft again, you just lost it control.
	err := it.OnRecv(ft)
	if err != nil {
		logger.Err(err)
	}
	return true
}

func (s *Session) on_dns(ft *FrameDns) {
	ipaddr, err := net.LookupIP(ft.Hostname)
	if err != nil {
		logger.Err(err)
		ipaddr = make([]net.IP, 0)
	}

	fr := NewFrameAddr(ft.Streamid, ipaddr)
	err = s.WriteFrame(fr)
	if err != nil {
		logger.Err(err)
	}
	return
}

func (s *Session) sendFrameInChan(streamid uint16, f Frame) bool {
	i, ok := s.ports[streamid]
	if !ok {
		logger.Err("stream id not exist")
		return true
	}
	ch, ok := i.(chan Frame)
	if !ok {
		logger.Err("unexpected ports type")
		return false
	}
	ch <- f
	return true
}

func (s *Session) Run() {
	defer s.conn.Close()
	defer s.Close()

	for {
		f, err := ReadFrame(s.conn)
		// EOF, in client mode, try reconnect
		if err != nil {
			return
		}

		switch ft := f.(type) {
		default:
			logger.Err("unexpected package")
			return
		case *FrameOK:
			logger.Debugf("get package ok: %d.", ft.Streamid)
			if !s.sendFrameInChan(ft.Streamid, f) {
				return
			}
		case *FrameFAILED:
			logger.Debugf("get package failed: %d, errno: %d.",
				ft.Streamid, ft.Errno)
			if !s.sendFrameInChan(ft.Streamid, f) {
				return
			}
		case *FrameData:
			logger.Debugf("get package data: stream(%d), len(%d).",
				ft.Streamid, len(ft.Data))
			if !s.on_data(ft) {
				return
			}
		case *FrameSyn:
			logger.Debugf("get package syn: %d => %s.",
				ft.Streamid, ft.Address)
			if !s.on_syn(ft) {
				return
			}
		case *FrameAck:
			logger.Debugf("get package ack: %d, window: %d.",
				ft.Streamid, ft.Window)
			i, ok := s.ports[ft.Streamid]
			if !ok {
				logger.Errf("frame ack stream id(%d) not exist.",
					ft.Streamid)
				continue
			}
			it, ok := i.(*Conn)
			if !ok {
				logger.Err("unexpected ports type")
				return
			}
			it.OnRead(ft.Window)
		case *FrameFin:
			logger.Debugf("get package fin: %d.", ft.Streamid)
			if !s.on_fin(ft) {
				return
			}
		case *FrameDns:
			logger.Debugf("get package dns: %s, stream(%d).",
				ft.Hostname, ft.Streamid)
			go s.on_dns(ft)
		case *FrameAddr:
			logger.Debugf("get package addr: %d.", ft.Streamid)
			if !s.sendFrameInChan(ft.Streamid, f) {
				return
			}
		}
	}
}
