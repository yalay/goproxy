package msocks

import (
	"errors"
	"fmt"
	"github.com/op/go-logging"
	"github.com/shell909090/goproxy/sutils"
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
	WINDOWSIZE     = 1024 * 1024
)

var errClosing = "use of closed network connection"

var ErrStreamNotExist = errors.New("stream not exist.")
var ErrQueueClosed = errors.New("queue closed")

var log = logging.MustGetLogger("msocks")

func init() {
	rand.Seed(time.Now().UnixNano())
}

var frame_ping = NewFramePing()

type PingPong struct {
	ch       chan int
	cnt      int
	lastping time.Time
	sender   FrameSender
}

func NewPingPong(sender FrameSender) (p *PingPong) {
	p = &PingPong{
		ch:       make(chan int, 0),
		lastping: time.Now(),
		sender:   sender,
	}
	return
}

func (p *PingPong) Reset() {
	p.cnt = 0
}

func (p *PingPong) Ping() bool {
	log.Debug("ping: %p.", p.sender)
	p.lastping = time.Now()
	select {
	case p.ch <- 1:
	default:
	}

	p.cnt += 1
	if p.cnt >= GAMEOVER_COUNT {
		log.Warning("pingpong gameover.")
		p.sender.CloseFrame()
		return false
	}

	pingtime := PINGTIME + time.Duration(rand.Intn(2*PINGRANDOM)-PINGRANDOM)*time.Millisecond
	log.Debug("pingtime: %d", pingtime/time.Millisecond)

	go func() {
		time.Sleep(pingtime)
		p.Pong()

		timeout := time.After(TIMEOUT_COUNT * PINGTIME)
		select {
		case <-timeout:
			log.Warning("pingpong timeout: %p.", p.sender)
			p.sender.CloseFrame()
			return
		case <-p.ch:
			return
		}
	}()
	return true
}

func (p *PingPong) GetLastPing() (d time.Duration) {
	return time.Now().Sub(p.lastping)
}

func (p *PingPong) Pong() {
	log.Debug("pong: %p.", p.sender)
	p.sender.SendFrame(frame_ping)
}

type Session struct {
	wlock sync.Mutex
	conn  net.Conn

	plock   sync.Mutex
	next_id uint16
	ports   map[uint16]FrameSender

	dialer sutils.Dialer
	PingPong
}

func NewSession(conn net.Conn) (s *Session) {
	s = &Session{
		conn:  conn,
		ports: make(map[uint16]FrameSender, 0),
	}
	s.PingPong = *NewPingPong(s)
	log.Notice("session %p created.", s)
	return
}

func (s *Session) Close() (err error) {
	log.Warning("close all(len:%d) for session: %p.", len(s.ports), s)
	defer s.conn.Close()
	s.plock.Lock()
	defer s.plock.Unlock()

	for _, v := range s.ports {
		if v != nil {
			v.CloseFrame()
		}
	}
	return
}

func (s *Session) LocalAddr() net.Addr {
	return s.conn.LocalAddr()
}

func (s *Session) RemoteAddr() net.Addr {
	return s.conn.RemoteAddr()
}

func (s *Session) SendFrame(f Frame) bool {
	f.Debug("send ")

	buf, err := f.Packed()
	if err != nil {
		log.Error("%s", err)
		return false
	}
	b := buf.Bytes()
	s.wlock.Lock()
	defer s.wlock.Unlock()

	n, err := s.conn.Write(b)
	if err != nil && err.Error() == errClosing {
		err = io.EOF
	}
	log.Debug("sess %p write len(%d), result %p.", s, len(b), err)
	if n != len(b) {
		err = io.ErrShortWrite
	}
	if err != nil {
		log.Error("%s", err)
		return false
	}
	return true
}

func (s *Session) CloseFrame() error {
	return s.Close()
}

func (s *Session) GetPorts() (ports map[uint16]*Conn) {
	s.plock.Lock()
	defer s.plock.Unlock()

	ports = make(map[uint16]*Conn, 0)
	for i, fs := range s.ports {
		switch c := fs.(type) {
		case *Conn:
			ports[i] = c
		}
	}
	return ports
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
			log.Error("%s", err)
			return
		}
		_, ok = s.ports[s.next_id]
	}
	id = s.next_id
	s.next_id += 1
	log.Debug("put into next id %p(%d): %p.", s, id, fs)

	s.ports[id] = fs
	return
}

func (s *Session) PutIntoId(id uint16, fs FrameSender) {
	log.Debug("put into id %p(%d): %p.", s, id, fs)
	s.plock.Lock()
	defer s.plock.Unlock()

	s.ports[id] = fs
}

func (s *Session) RemovePorts(streamid uint16) (err error) {
	s.plock.Lock()
	defer s.plock.Unlock()

	_, ok := s.ports[streamid]
	if !ok {
		return fmt.Errorf("streamid(%d) not exist.", streamid)
	}
	delete(s.ports, streamid)
	log.Notice("remove ports %p(%d).", s, streamid)
	return
}

func (s *Session) Run() {
	defer s.Close()

	for {
		f, err := ReadFrame(s.conn)
		if err != nil {
			log.Error("%s", err)
			return
		}

		f.Debug("recv ")
		switch ft := f.(type) {
		default:
			log.Error("unexpected package")
			return
		case *FrameOK, *FrameFAILED, *FrameData, *FrameAck, *FrameFin, *FrameRst:
			if !s.sendFrameInChan(f) {
				return
			}
			s.PingPong.Reset()
		case *FrameSyn:
			if !s.on_syn(ft) {
				return
			}
			s.PingPong.Reset()
		case *FramePing:
			s.PingPong.Ping()
		}
	}
}

// In all of situation, drop frame if chan full.
// And if frame finally come, drop it too.
func (s *Session) sendFrameInChan(f Frame) (b bool) {
	streamid := f.GetStreamid()
	c, ok := s.ports[streamid]
	if !ok {
		fb := NewFrameRst(streamid)
		return s.SendFrame(fb)
	}
	if c == nil {
		return true
	}

	if !c.SendFrame(f) {
		log.Error("%p(%d) send failed.", s, streamid)
		if c.CloseFrame() != nil {
			return false
		}

		fb := NewFrameRst(streamid)
		if !s.SendFrame(fb) {
			return false
		}
		return s.RemovePorts(streamid) == nil
	}
	return true
}

func (s *Session) on_syn(ft *FrameSyn) bool {
	_, ok := s.ports[ft.Streamid]
	if ok {
		log.Error("frame sync stream id exist.")
		fb := NewFrameFAILED(ft.Streamid, ERR_IDEXIST)
		return s.SendFrame(fb)
	}

	// lock streamid temporary, do I need this?
	s.PutIntoId(ft.Streamid, nil)

	connect := func() (c *Conn, err error) {
		conn, err := s.dialer.Dial("tcp", ft.Address)
		if err != nil {
			return
		}

		c = NewConn(ft.Streamid, s, ft.Address)
		go sutils.CopyLink(conn, c)
		return c, nil
	}

	go func() {
		// TODO: timeout
		log.Debug("%p(%d) try to connect: %s.",
			s, ft.Streamid, ft.Address)
		c, err := connect()
		if err != nil {
			log.Error("%s", err)

			fb := NewFrameFAILED(ft.Streamid, ERR_CONNFAILED)
			if !s.SendFrame(fb) {
				return
			}

			err = s.RemovePorts(ft.Streamid)
			if err != nil {
				log.Error("%s", err)
			}
			return
		}

		// update it, don't need to lock
		s.PutIntoId(ft.Streamid, c)

		fb := NewFrameOK(ft.Streamid)
		if !s.SendFrame(fb) {
			return
		}
		log.Notice("%p(%d) connected %s.",
			s, ft.Streamid, ft.Address)
		return
	}()
	return true
}
