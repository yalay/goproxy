package msocks

import (
	"errors"
	"fmt"
	"github.com/shell909090/goproxy/logging"
	"io"
	"math/rand"
	"net"
	"sync"
	"time"
)

const (
	RETRY_TIMES    = 6
	CHANLEN        = 128
	WIN_SIZE       = 100
	ACKDELAY       = 100 * time.Millisecond
	HALFCLOSE      = 20000 * time.Millisecond
	PINGTIME       = 10000 * time.Millisecond
	PINGRANDOM     = 3000
	TIMEOUT_COUNT  = 4
	GAMEOVER_COUNT = 60
	DIAL_TIMEOUT   = 30 * time.Second
	LOOKUP_TIMEOUT = 60 * time.Second
)

var errClosing = "use of closed network connection"

var ErrStreamNotExist = errors.New("stream not exist.")

var logger logging.Logger

func init() {
	var err error
	logger, err = logging.NewFileLogger("default", -1, "msocks")
	if err != nil {
		panic(err)
	}

	rand.Seed(time.Now().UnixNano())
}

type FrameSender interface {
	SendFrame(Frame) bool
	Close() error
}

type Session struct {
	flock sync.Mutex
	conn  net.Conn

	// lock ports before any ports op and id op
	plock   sync.Mutex
	next_id uint16
	ports   map[uint16]FrameSender

	on_conn func(*Session, string, uint16) (FrameSender, error)

	PingPong
}

func NewSession(conn net.Conn) (s *Session) {
	s = &Session{
		conn:  conn,
		ports: make(map[uint16]FrameSender, 0),
	}
	s.PingPong = *NewPingPong(s)
	logger.Noticef("session %p created.", s)
	return
}

func (s *Session) GetPorts() (ports map[uint16]*Conn) {
	s.flock.Lock()
	defer s.flock.Unlock()

	ports = make(map[uint16]*Conn, 0)
	for i, fs := range s.ports {
		switch c := fs.(type) {
		case *Conn:
			ports[i] = c
		case nil:
			ports[i] = nil
		}
	}
	return ports
}

func (s *Session) LocalAddr() net.Addr {
	return s.conn.LocalAddr()
}

func (s *Session) RemoteAddr() net.Addr {
	return s.conn.RemoteAddr()
}

func (s *Session) Write(b []byte) (n int, err error) {
	s.PingPong.Reset()
	logger.Debug("reset pingpong.")
	return s.WriteWithoutReset(b)
}

func (s *Session) WriteWithoutReset(b []byte) (n int, err error) {
	s.flock.Lock()
	defer s.flock.Unlock()
	n, err = s.conn.Write(b)
	if err != nil && err.Error() == errClosing {
		err = io.EOF
	}
	logger.Debugf("sess %p write len(%d), result %s.", s, len(b), err)
	if err != nil {
		return
	}
	if n != len(b) {
		err = io.ErrShortWrite
	}
	return
}

func (s *Session) Close() (err error) {
	logger.Warningf("close all(len:%d) for session: %p.", len(s.ports), s)
	defer s.conn.Close()
	for _, v := range s.ports {
		if v != nil {
			v.Close()
		}
	}
	return
}

func (s *Session) PutIntoNextId(fs FrameSender) (id uint16, err error) {
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
	s.next_id += 1
	logger.Debugf("put into next id %p(%d): %p.", s, id, fs)

	s.ports[id] = fs
	return
}

func (s *Session) PutIntoId(id uint16, fs FrameSender) {
	logger.Debugf("put into id %p(%d): %p.", s, id, fs)
	s.ports[id] = fs
	return
}

func (s *Session) RemovePorts(streamid uint16) (err error) {
	_, ok := s.ports[streamid]
	if !ok {
		return fmt.Errorf("streamid(%d) not exist.", streamid)
	}
	s.ports[streamid] = nil
	logger.Noticef("set %p(%d) => nil.", s, streamid)
	logger.Stack()
	time.AfterFunc(HALFCLOSE, func() {
		s.plock.Lock()
		defer s.plock.Unlock()
		delete(s.ports, streamid)
		logger.Noticef("remove ports %p(%d).", s, streamid)
	})
	return
}

func (s *Session) on_syn(ft *FrameSyn) bool {
	_, ok := s.ports[ft.Streamid]
	if ok {
		logger.Err("frame sync stream id exist.")
		b := NewFrameOneInt(MSG_FAILED, ft.Streamid, ERR_IDEXIST)
		_, err := s.Write(b)
		if err != nil {
			logger.Err(err)
			return false
		}
		return true
	}

	// lock streamid temporary, do I need this?
	s.PutIntoId(ft.Streamid, nil)

	go func() {
		// TODO: timeout
		logger.Debugf("%p(%d) try to connect: %s.",
			s, ft.Streamid, ft.Address)
		fs, err := s.on_conn(s, ft.Address, ft.Streamid)
		if err != nil {
			logger.Err(err)

			b := NewFrameOneInt(MSG_FAILED, ft.Streamid, ERR_CONNFAILED)
			_, err = s.Write(b)
			if err != nil {
				logger.Err(err)
				return
			}

			err = s.RemovePorts(ft.Streamid)
			if err != nil {
				logger.Err(err)
			}
			return
		}

		// update it, don't need to lock
		s.PutIntoId(ft.Streamid, fs)

		b := NewFrameNoParam(MSG_OK, ft.Streamid)
		_, err = s.Write(b)
		if err != nil {
			logger.Err(err)
			return
		}
		logger.Noticef("%p(%d) connected %s.",
			s, ft.Streamid, ft.Address)
		return
	}()
	return true
}

func (s *Session) on_rst(ft *FrameRst) {
	s.plock.Lock()
	defer s.plock.Unlock()

	c, ok := s.ports[ft.Streamid]
	if !ok {
		return
	}
	logger.Debugf("reset %p(%d), sender %p.", s, ft.Streamid, c)
	delete(s.ports, ft.Streamid)
	if c != nil {
		c.Close()
	}
}

func (s *Session) on_dns(ft *FrameDns) {
	// This will toke long time...
	ipaddr, err := net.LookupIP(ft.Hostname)
	if err != nil {
		logger.Err(err)
		ipaddr = make([]net.IP, 0)
	}

	b, err := NewFrameAddr(ft.Streamid, ipaddr)
	if err != nil {
		logger.Err(err)
		return
	}
	_, err = s.Write(b)
	if err != nil {
		logger.Err(err)
	}
	return
}

// In all of situation, drop frame if chan full.
// And if frame finally come, drop it too.
func (s *Session) sendFrameInChan(f Frame) (b bool) {
	streamid := f.GetStreamid()
	c, ok := s.ports[streamid]
	if !ok {
		// logger.Errf("%p(%d) not exist.", s, streamid)
		s.ports[streamid] = nil
		time.AfterFunc(HALFCLOSE, func() {
			s.plock.Lock()
			defer s.plock.Unlock()
			delete(s.ports, streamid)
		})

		buf := NewFrameNoParam(MSG_RST, streamid)
		_, err := s.WriteWithoutReset(buf)
		return err == nil
	}
	if c == nil {
		return true
	}

	b = c.SendFrame(f)
	if !b {
		logger.Errf("%p(%d) fulled or closed.", s, streamid)
		if c.Close() != nil {
			return false
		}
		buf := NewFrameNoParam(MSG_RST, streamid)
		_, err := s.WriteWithoutReset(buf)
		if err != nil {
			return false
		}
		return s.RemovePorts(streamid) == nil
	}
	return true
}

func (s *Session) Run() {
	defer s.Close()

	for {
		f, err := ReadFrame(s.conn)
		if err != nil {
			logger.Err(err)
			return
		}

		f.Debug()
		switch ft := f.(type) {
		default:
			logger.Err("unexpected package")
			return
		case *FrameOK, *FrameFAILED, *FrameData, *FrameAck, *FrameFin, *FrameAddr:
			if !s.sendFrameInChan(f) {
				return
			}
		case *FrameSyn:
			if !s.on_syn(ft) {
				return
			}
		case *FrameRst:
			s.on_rst(ft)
		case *FrameDns:
			go s.on_dns(ft)
		case *FramePing:
			s.PingPong.Ping()
		}
	}
}
