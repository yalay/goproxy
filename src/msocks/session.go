package msocks

import (
	"errors"
	"fmt"
	"logging"
	"net"
	"sync"
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
	conn  net.Conn
	flock sync.Mutex

	// lock ports before any ports op and id op
	plock   sync.Mutex
	next_id uint16
	ports   map[uint16]interface{}

	on_conn func(string, string, uint16) (*Conn, error)
}

func NewSession(conn net.Conn) (s *Session) {
	return &Session{
		conn:  conn,
		ports: make(map[uint16]interface{}, 0),
	}
}

func (s *Session) LocalAddr() net.Addr {
	return s.conn.LocalAddr()
}

func (s *Session) RemoteAddr() net.Addr {
	return s.conn.RemoteAddr()
}

func (s *Session) PutIntoNextId(i interface{}) (id uint16, err error) {
	logger.Debugf("put into next id: %d.", i)
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
	logger.Debugf("next id is: %d.", id)
	s.next_id += 1

	s.ports[id] = i
	return
}

func (s *Session) WriteFrame(f Frame) (err error) {
	s.flock.Lock()
	defer s.flock.Unlock()
	return f.WriteFrame(s.conn)
}

func (s *Session) ClosePort(streamid uint16) (err error) {
	logger.Debugf("Close Port: %d.", streamid)
	s.plock.Lock()
	defer s.plock.Unlock()
	_, ok := s.ports[streamid]
	if ok {
		delete(s.ports, streamid)
	} else {
		err = fmt.Errorf("streamid not exist: %d.", streamid)
		logger.Err(err)
	}
	return
}

func (s *Session) Close() (err error) {
	logger.Infof("close all connections for session: %x.", s)
	s.plock.Lock()
	defer s.plock.Unlock()

	for _, v := range s.ports {
		switch vt := v.(type) {
		case chan int:
			vt <- 0
		case *Conn:
			vt.MarkClose()
		}
	}

	s.ports = make(map[uint16]interface{}, 0)
	return
}

func (s *Session) on_syn(ft *FrameSyn) bool {
	_, ok := s.ports[ft.streamid]
	if ok {
		logger.Err("frame sync stream id exist.")
		SendFAILEDFrame(s.conn, ft.streamid, ERR_IDEXIST)
		return false
	}

	// lock streamid temporary
	s.ports[ft.streamid] = 1

	go func() {
		logger.Infof("client try to connect: %s.", ft.address)
		stream, err := s.on_conn("tcp", ft.address, ft.streamid)
		if err != nil {
			SendFAILEDFrame(s.conn, ft.streamid, ERR_CONNFAILED)
			logger.Err(err)

			s.ClosePort(ft.streamid)
			return
		}

		// update it, don't need to lock
		s.ports[ft.streamid] = stream
		SendOKFrame(s.conn, ft.streamid)
		logger.Debug("connect successed.")
		return
	}()
	return true
}

func (s *Session) on_fin(ft *FrameFin) bool {
	i, ok := s.ports[ft.streamid]
	if !ok {
		logger.Err("frame fin stream id not exist")
		return false
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
	i, ok := s.ports[ft.streamid]
	if !ok {
		logger.Err("frame data stream id not exist")
		return false
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

func (s *Session) Run() {
	defer s.conn.Close()
	defer s.Close()

	for {
		f, err := ReadFrame(s.conn)
		// EOF, in client mode, try reconnect
		if err != nil {
			logger.Err(err)
			return
		}

		switch ft := f.(type) {
		default:
			logger.Err("unexpected package")
			return
		case *FrameOK:
			logger.Debugf("get package ok: %s.", ft)
			i, ok := s.ports[ft.streamid]
			if !ok {
				logger.Err("frame ack stream id not exist")
				return
			}
			ch, ok := i.(chan int)
			if !ok {
				logger.Err("unexpected ports type")
				return
			}
			ch <- 1
		case *FrameFAILED:
			logger.Debugf("get package failed: %s.", ft)
			i, ok := s.ports[ft.streamid]
			if !ok {
				logger.Err("frame ack stream id not exist")
				return
			}
			ch, ok := i.(chan int)
			if !ok {
				logger.Err("unexpected ports type")
				return
			}
			ch <- 0
		case *FrameData:
			logger.Debugf("get package data: %s.", ft)
			if !s.on_data(ft) {
				return
			}
		case *FrameSyn:
			logger.Debugf("get package syn: %s.", ft)
			if !s.on_syn(ft) {
				return
			}
		case *FrameAck:
			logger.Debugf("get package ack: %s.", ft)
			i, ok := s.ports[ft.streamid]
			if !ok {
				logger.Err("frame ack stream id not exist")
				return
			}
			it, ok := i.(*Conn)
			if !ok {
				logger.Err("unexpected ports type")
				return
			}
			it.OnRead(ft.window)
		case *FrameFin:
			logger.Debugf("get package fin: %s.", ft)
			if !s.on_fin(ft) {
				return
			}
		case *FrameRst:
			logger.Err("not support yet")
			return
			// 	stream, ok := s.streams[ft.streamid]
			// 	if !ok {
			// 		// failed
			// 		panic("frame rst stream id not exist")
			// 	}
			// 	stream.read_closed = true
			// 	stream.write_closed = true
			// 	stream.on_close()
		}
	}
}
