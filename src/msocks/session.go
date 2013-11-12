package msocks

import (
	"errors"
	"io"
	"logging"
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
	conn  io.ReadWriteCloser
	flock sync.Mutex

	idlock  sync.Mutex
	next_id uint16
	ports   map[uint16]interface{}

	on_conn func(string, string, uint16) (Stream, error)
	re_conn func() error
}

func NewSession() (s *Session) {
	return &Session{}
}

func (s *Session) GetNextId() (id uint16, err error) {
	s.idlock.Lock()
	defer s.idlock.Unlock()

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
	return
}

func (s *Session) WriteFrame(f Frame) (err error) {
	s.flock.Lock()
	defer s.flock.Unlock()
	return f.WriteFrame(s.conn)
}

func (s *Session) OnRead(streamid uint16, window uint32) (err error) {
	ft := &FrameAck{
		streamid: streamid,
		window:   window,
	}
	return s.WriteFrame(ft)
}

func (s *Session) on_syn(ft *FrameSyn) bool {
	_, ok := s.ports[ft.streamid]
	if ok {
		logger.Info("frame sync stream id exist.")
		SendFAILEDFrame(s.conn, ft.streamid, ERR_IDEXIST)
		return false
	}

	// lock streamid temporary
	s.ports[ft.streamid] = 1

	go func() {
		stream, err := s.on_conn("tcp", ft.address, ft.streamid)
		if err != nil {
			SendFAILEDFrame(s.conn, ft.streamid, ERR_CONNFAILED)
			logger.Err(err)
			// free it
			delete(s.ports, ft.streamid)
			return
		}

		// update it
		s.ports[ft.streamid] = stream
		SendOKFrame(s.conn, ft.streamid)
		return
	}()
	return true
}

func (s *Session) on_data(ft *FrameData) bool {
	i, ok := s.ports[ft.streamid]
	if !ok {
		logger.Err("frame data stream id not exist")
		return false
	}

	it, ok := i.(Stream)
	if !ok {
		logger.Err("unexpected ports type")
		return false
	}

	_, err := it.OnRecv(ft.data)
	if err != nil {
		logger.Err(err)
	}
	return true
}

func (s *Session) Run() {
	for {
		f, err := ReadFrame(s.conn)
		// EOF, in client mode, try reconnect
		switch err {
		default:
			logger.Err(err)
			return
		case io.EOF:
			if s.re_conn == nil {
				logger.Info("EOF without reconnect, quit.")
				return
			}

			logger.Info("EOF, try reconnect.")
			err = s.re_conn()
			if err != nil {
				logger.Err(err)
				return
			}

		case nil:
		}

		switch ft := f.(type) {
		default:
			logger.Err("unexpected package")
			return
		case *FrameOK:
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
			if !s.on_data(ft) {
				return
			}
		case *FrameSyn:
			if !s.on_syn(ft) {
				return
			}
		case *FrameAck:
			i, ok := s.ports[ft.streamid]
			if !ok {
				logger.Err("frame ack stream id not exist")
				return
			}
			it, ok := i.(Stream)
			if !ok {
				logger.Err("unexpected ports type")
				return
			}
			it.OnRead(ft.window)
		case *FrameFin:
			i, ok := s.ports[ft.streamid]
			if !ok {
				logger.Err("frame fin stream id not exist")
				return
			}
			it, ok := i.(Stream)
			if !ok {
				logger.Err("unexpected ports type")
				return
			}
			err = it.OnClose()
			if err != nil {
				logger.Err(err)
				return
			}
			delete(s.ports, ft.streamid)
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
