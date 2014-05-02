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
	PINGTIME       = 5000
	PINGRANDOM     = 1000
	TIMEOUT_COUNT  = 6
	GAMEOVER_COUNT = 60

	DIAL_RETRY   = 6
	DIAL_TIMEOUT = 30000
	WINDOWSIZE   = 2 * 1024 * 1024
	WND_DELAY    = 100
)

const (
	ERR_NONE = iota
	ERR_AUTH
	ERR_IDEXIST
	ERR_CONNFAILED
	ERR_TIMEOUT
)

var (
	ErrStreamNotExist = errors.New("stream not exist.")
	ErrQueueClosed    = errors.New("queue closed")
	ErrUnexpectedPkg  = errors.New("unexpected package")
	ErrNotSyn         = errors.New("frame result in conn which status is not syn")
	ErrFinState       = errors.New("status not est or fin wait when get fin")
	ErrIdExist        = errors.New("frame sync stream id exist.")
	ErrState          = errors.New("status error")
)

var (
	log        = logging.MustGetLogger("msocks")
	frame_ping = NewFramePing()
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

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

	pingtime := PINGTIME + rand.Intn(2*PINGRANDOM) - PINGRANDOM
	log.Debug("pingtime: %d", pingtime)

	go func() {
		time.Sleep(time.Duration(pingtime) * time.Millisecond)
		p.Pong()

		timeout := time.After(
			TIMEOUT_COUNT * PINGTIME * time.Millisecond)
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
	err := p.sender.SendFrame(frame_ping)
	if err != nil {
		log.Error("%s", err)
	}
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
	log.Warning("close all connects (%d) for session: %p.", len(s.ports), s)
	defer s.conn.Close()
	s.plock.Lock()
	defer s.plock.Unlock()

	for _, v := range s.ports {
		v.CloseFrame()
	}
	return
}

func (s *Session) LocalAddr() net.Addr {
	return s.conn.LocalAddr()
}

func (s *Session) RemoteAddr() net.Addr {
	return s.conn.RemoteAddr()
}

func (s *Session) SendFrame(f Frame) (err error) {
	f.Debug("send ")

	buf, err := f.Packed()
	if err != nil {
		return
	}
	b := buf.Bytes()
	s.wlock.Lock()
	defer s.wlock.Unlock()

	n, err := s.conn.Write(b)
	if err != nil {
		// switch err.Error() {
		// case errClosing, errReset:
		// 	err = io.EOF
		// }
		return
	}
	if n != len(b) {
		return io.ErrShortWrite
	}
	log.Debug("sess %p write len(%d).", s, len(b))
	return
}

func (s *Session) CloseFrame() error {
	return s.Close()
}

func (s *Session) GetPorts() (ports map[uint16]*Conn) {
	s.plock.Lock()
	defer s.plock.Unlock()

	ports = make(map[uint16]*Conn, 0)
	for i, fs := range s.ports {
		if c, ok := fs.(*Conn); ok {
			ports[i] = c
		}
	}
	return
}

func (s *Session) PutIntoNextId(fs FrameSender) (id uint16, err error) {
	s.plock.Lock()
	defer s.plock.Unlock()

	startid := s.next_id
	for _, ok := s.ports[s.next_id]; ok; _, ok = s.ports[s.next_id] {
		s.next_id += 1
		if s.next_id == startid {
			err = errors.New("run out of stream id")
			log.Error("%s", err)
			return
		}
	}
	id = s.next_id
	s.next_id += 1
	log.Debug("put into next id %p(%d): %p.", s, id, fs)

	s.ports[id] = fs
	return
}

func (s *Session) PutIntoId(id uint16, fs FrameSender) (err error) {
	log.Debug("put into id %p(%d): %p.", s, id, fs)
	s.plock.Lock()
	defer s.plock.Unlock()

	_, ok := s.ports[id]
	if ok {
		return ErrIdExist
	}

	s.ports[id] = fs
	return
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
		case *FrameResult, *FrameData, *FrameWnd, *FrameFin, *FrameRst:
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

// no drop, any error will reset main connection
func (s *Session) sendFrameInChan(f Frame) (b bool) {
	var err error
	streamid := f.GetStreamid()
	c, ok := s.ports[streamid]
	if !ok || c == nil {
		return false
	}

	err = c.SendFrame(f)
	if err != nil {
		log.Error("%p(%d) send failed, err: %s.", s, streamid, err)
		return false
	}
	return true
}

func (s *Session) on_syn(ft *FrameSyn) bool {
	// lock streamid temporary, with status sync recved
	c := NewConn(ST_SYN_RECV, ft.Streamid, s, ft.Address)
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
		log.Debug("%p(%d) try to connect: %s.",
			s, ft.Streamid, ft.Address)

		// TODO: timeout
		conn, err := s.dialer.Dial("tcp", ft.Address)
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
		log.Notice("server side %p(%d) connected %s.",
			s, ft.Streamid, ft.Address)
		return
	}()
	return true
}
