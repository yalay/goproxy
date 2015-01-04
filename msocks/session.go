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
	"sync/atomic"
	"time"
)

const (
	PINGTIME       = 5000
	PINGRANDOM     = 1000
	TIMEOUT_COUNT  = 6
	GAMEOVER_COUNT = 60

	DIAL_RETRY   = 3
	DIAL_TIMEOUT = 30000

	WINDOWSIZE = 2 * 1024 * 1024

	AUTH_TIMEOUT      = 5000
	MIN_SESS_NUM      = 2
	MAX_CONN_PRE_SESS = 8

	SHRINK_TIME = 5000
	SHRINK_RATE = 0.9
)

const (
	ERR_NONE = iota
	ERR_AUTH
	ERR_IDEXIST
	ERR_CONNFAILED
	ERR_TIMEOUT
	ERR_CLOSED
)

var (
	ErrNoSession       = errors.New("session in pool but can't pick one.")
	ErrStreamNotExist  = errors.New("stream not exist.")
	ErrQueueClosed     = errors.New("queue closed")
	ErrUnexpectedPkg   = errors.New("unexpected package")
	ErrNotSyn          = errors.New("frame result in conn which status is not syn")
	ErrFinState        = errors.New("status not est or fin wait when get fin")
	ErrIdExist         = errors.New("frame sync stream id exist.")
	ErrState           = errors.New("status error")
	ErrSessionNotFound = errors.New("session not found")
)

var (
	log        = logging.MustGetLogger("msocks")
	frame_ping = NewFramePing()
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type PingPong struct {
	ch_ping  chan int
	cnt      int32
	lastping time.Time
	sender   FrameSender
	gameover bool
}

func NewPingPong(sender FrameSender) (p *PingPong) {
	p = &PingPong{
		ch_ping:  make(chan int, 1),
		cnt:      0,
		lastping: time.Now(),
		sender:   sender,
		gameover: false,
	}
	go p.loop()
	return
}

func (p *PingPong) IsGameOver() bool {
	return p.gameover
}

func (p *PingPong) Reset() {
	atomic.StoreInt32(&p.cnt, 0)
}

func (p *PingPong) GetLastPing() (d time.Duration) {
	return time.Now().Sub(p.lastping)
}

func (p *PingPong) ping() {
	log.Debug("ping: %p.", p.sender)
	p.lastping = time.Now()
	select {
	case p.ch_ping <- 1:
	default:
	}
}

func (p *PingPong) pong() {
	log.Debug("pong: %p.", p.sender)
	err := p.sender.SendFrame(frame_ping)
	if err != nil {
		log.Error("%s", err)
	}
}

func (p *PingPong) addCount() int32 {
	return atomic.AddInt32(&p.cnt, 1)
}

func (p *PingPong) loop() {
	for {
		select {
		case <-time.After(TIMEOUT_COUNT * PINGTIME * time.Millisecond):
			log.Warning("pingpong timeout: %p.", p.sender)
			p.sender.CloseFrame()
			return
		case <-p.ch_ping:
		}

		if p.addCount() >= GAMEOVER_COUNT {
			log.Warning("pingpong gameover.")
			p.gameover = true
			p.sender.CloseFrame()
			return
		}

		pingtime := PINGTIME + rand.Intn(2*PINGRANDOM) - PINGRANDOM
		log.Debug("pingtime: %d", pingtime)
		time.Sleep(time.Duration(pingtime) * time.Millisecond)
		p.pong()
	}
}

type Session struct {
	wlock sync.Mutex
	conn  net.Conn

	readcnt  int64
	readbps  int64
	writecnt int64
	writebps int64

	closed  bool
	plock   sync.Mutex
	next_id uint16
	ports   map[uint16]FrameSender

	dialer sutils.Dialer
	PingPong
}

func NewSession(conn net.Conn) (s *Session) {
	s = &Session{
		conn:   conn,
		closed: false,
		ports:  make(map[uint16]FrameSender, 0),
	}
	s.PingPong = *NewPingPong(s)
	log.Notice("session %s created.", s.GetId())
	go s.loop_count()
	return
}

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
	c = NewConn(ST_SYN_SENT, 0, s, address)
	streamid, err := s.PutIntoNextId(c)
	if err != nil {
		return
	}
	c.streamid = streamid

	log.Info("try dial: %s => %s.",
		s.conn.RemoteAddr().String(), address)
	err = c.WaitForConn(address)
	if err != nil {
		return
	}

	return c, nil
}

func (s *Session) LookupIP(host string) (addrs []net.IP, err error) {
	return
}

func (s *Session) Close() (err error) {
	log.Warning("close all connects (%d) for session: %s.",
		len(s.ports), s.GetId())
	defer s.conn.Close()
	s.plock.Lock()
	defer s.plock.Unlock()

	for _, v := range s.ports {
		v.CloseFrame()
	}
	s.closed = true
	return
}

func (s *Session) LocalAddr() net.Addr {
	return s.conn.LocalAddr()
}

func (s *Session) RemoteAddr() net.Addr {
	return s.conn.RemoteAddr()
}

func (s *Session) LocalPort() int {
	addr, ok := s.LocalAddr().(*net.TCPAddr)
	if !ok {
		return -1
	}
	return addr.Port
}

func shrink_count(cnt *int64, bps *int64) bool {
	num := float64(atomic.SwapInt64(cnt, 0)) * (1 - SHRINK_RATE)
	for i := 0; i < 10; i++ {
		old := atomic.LoadInt64(bps)
		new := int64(float64(old)*SHRINK_RATE + num)
		if atomic.CompareAndSwapInt64(bps, old, new) {
			return true
		}
	}
	return false
}

func (s *Session) loop_count() {
	for !s.closed {
		if !shrink_count(&s.readcnt, &s.readbps) {
			log.Error("shrink counter read failed")
		}
		if !shrink_count(&s.writecnt, &s.writebps) {
			log.Error("shrink counter write failed")
		}
		time.Sleep(SHRINK_TIME * time.Millisecond)
	}
}

func (s *Session) GetReadSpeed() int64 {
	return atomic.LoadInt64(&s.readbps)
}

func (s *Session) GetWriteSpeed() int64 {
	return atomic.LoadInt64(&s.writebps)
}

func (s *Session) SendFrame(f Frame) (err error) {
	f.Debug("send ")
	atomic.AddInt64(&s.writecnt, int64(f.GetSize()))

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
	log.Debug("sess %s write len(%d).", s.GetId(), len(b))
	return
}

func (s *Session) CloseFrame() error {
	return s.Close()
}

func (s *Session) GetId() string {
	return fmt.Sprintf("%d", s.LocalPort())
}

func (s *Session) GetSize() int {
	return len(s.ports)
}

func (s *Session) GetPorts() (ports []*Conn) {
	s.plock.Lock()
	defer s.plock.Unlock()

	for _, fs := range s.ports {
		if c, ok := fs.(*Conn); ok {
			ports = append(ports, c)
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
	log.Debug("%s put into next id %d: %p.", s.GetId(), id, fs)

	s.ports[id] = fs
	return
}

func (s *Session) PutIntoId(id uint16, fs FrameSender) (err error) {
	log.Debug("%s put into id %d: %p.", s.GetId(), id, fs)
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
	log.Notice("%s remove ports %d.", s.GetId(), streamid)
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
		atomic.AddInt64(&s.readcnt, int64(f.GetSize()))

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
			s.PingPong.ping()
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
		log.Error("%s(%d) send failed, err: %s.",
			s.GetId(), streamid, err)
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
		log.Debug("%s(%d) try to connect: %s.",
			s.GetId(), ft.Streamid, ft.Address)

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
		log.Notice("server side %s(%d) connected %s.",
			s.GetId(), ft.Streamid, ft.Address)
		return
	}()
	return true
}
