package msocks

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"sync"

	"github.com/shell909090/goproxy/sutils"
)

type Session struct {
	wlock sync.Mutex
	conn  net.Conn

	closed  bool
	plock   sync.Mutex
	next_id uint16
	ports   map[uint16]FrameSender

	dialer sutils.Dialer
	*PingPong
	*SpeedCounter
}

func NewSession(conn net.Conn) (s *Session) {
	s = &Session{
		conn:   conn,
		closed: false,
		ports:  make(map[uint16]FrameSender, 0),
	}
	s.PingPong = NewPingPong(s)
	s.SpeedCounter = NewSpeedCounter(s)
	log.Notice("session %s created.", s.GetId())
	return
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

type ConnSlice []*Conn

func (cs ConnSlice) Len() int           { return len(cs) }
func (cs ConnSlice) Swap(i, j int)      { cs[i], cs[j] = cs[j], cs[i] }
func (cs ConnSlice) Less(i, j int) bool { return cs[i].streamid < cs[j].streamid }

func (s *Session) GetSortedPorts() (ports ConnSlice) {
	ports = s.GetPorts()
	sort.Sort(ports)
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
	s.next_id += 2
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
	log.Info("%s remove ports %d.", s.GetId(), streamid)
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

func (s *Session) SendFrame(f Frame) (err error) {
	f.Debug("send ")
	s.WriteBytes(uint32(f.GetSize()))

	buf, err := f.Packed()
	if err != nil {
		return
	}
	b := buf.Bytes()
	s.wlock.Lock()
	defer s.wlock.Unlock()

	n, err := s.conn.Write(b)
	if err != nil {
		return
	}
	if n != len(b) {
		return io.ErrShortWrite
	}
	log.Debug("sess %s write %d bytes.", s.GetId(), len(b))
	return
}

func (s *Session) CloseFrame() error {
	return s.Close()
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
		s.ReadBytes(uint32(f.GetSize()))

		switch ft := f.(type) {
		default:
			log.Error("%s", ErrUnexpectedPkg.Error())
			return
		case *FrameResult, *FrameData, *FrameWnd, *FrameFin, *FrameRst:
			err = s.sendFrameInChan(f)
			if err != nil {
				log.Error("%s(%d) send failed, err: %s.",
					s.GetId(), f.GetStreamid(), err.Error())
				return
			}
			s.PingPong.Reset()
		case *FrameSyn:
			err = s.on_syn(ft)
			if err != nil {
				log.Error("syn failed: %s", err.Error())
				return
			}
			s.PingPong.Reset()
		case *FrameDns:
			err = s.on_dns(ft)
			if err != nil {
				log.Error("dns failed: %s", err.Error())
				return
			}
			s.PingPong.Reset()
		case *FramePing:
			s.PingPong.ping()
		}
	}
}

// no drop, any error will reset main connection
func (s *Session) sendFrameInChan(f Frame) (err error) {
	streamid := f.GetStreamid()
	c, ok := s.ports[streamid]
	if !ok || c == nil {
		return ErrStreamNotExist
	}

	err = c.SendFrame(f)
	if err != nil {
		return
	}
	return nil
}
