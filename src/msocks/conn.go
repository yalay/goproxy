package msocks

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

type Conn struct {
	sess     *Session
	streamid uint16
	rclosed  bool
	wclosed  bool

	ch_win  chan uint32
	buf     chan *FrameData
	bufhead *FrameData
	bufpos  int

	delaylock sync.Mutex
	delayack  *time.Timer
	delaycnt  int
}

func NewConn(streamid uint16, sess *Session) (c *Conn) {
	c = &Conn{
		streamid: streamid,
		sess:     sess,
		rclosed:  false,
		wclosed:  false,
		ch_win:   make(chan uint32, 3),
		buf:      make(chan *FrameData, 1024),
		// use 1024 as default channel length, 1024 * 1024 = 1M
		// that is the buffer before read
		// and it's the maxmium length of write window.
	}
	c.ch_win <- 256 * 1024
	// default value of write window is 256K.
	// that will be sent in 0.1s, so maxmium speed will be 2.56M/s = 20Mbps.
	return
}

func (c *Conn) SendAck(n int) {
	c.delaylock.Lock()
	defer c.delaylock.Unlock()

	if c.delayack == nil {
		// to avoid silly window symptom
		c.delayack = time.AfterFunc(100*time.Millisecond, func() {
			c.delaylock.Lock()
			defer c.delaylock.Unlock()
			if c.rclosed || c.wclosed {
				return
			}

			logger.Debugf("%p(%d) send ack %d.",
				c.sess, c.streamid, c.delaycnt)
			// send readed bytes back
			ft := &FrameAck{
				streamid: c.streamid,
				window:   uint32(c.delaycnt),
			}
			err := c.sess.WriteFrame(ft)
			if err != nil {
				logger.Err(err)
				// big trouble
			}
			c.delayack = nil
			c.delaycnt = 0
		})
	}

	c.delaycnt += n
	return
}

func (c *Conn) Read(data []byte) (n int, err error) {
	if c.bufhead == nil {
		c.bufhead = <-c.buf
		c.bufpos = 0
	}
	if c.bufhead == nil {
		return 0, io.EOF
	}

	n = len(c.bufhead.data) - c.bufpos
	if n > len(data) {
		n = len(data)
		copy(data, c.bufhead.data[c.bufpos:c.bufpos+n])
		logger.Debugf("read %d of head chunk at %d.",
			n, c.bufpos)
		c.bufpos += n
	} else {
		copy(data, c.bufhead.data[c.bufpos:])
		logger.Debugf("read all.")
		c.bufhead.Free()
		c.bufhead = nil

	}

	c.SendAck(n)
	return
}

func (c *Conn) OnRecv(f *FrameData) (err error) {
	if c.rclosed {
		return
	}
	if len(f.data) == 0 {
		return nil
	}

	logger.Debugf("%p(%d) recved %d bytes from remote.",
		c.sess, f.streamid, len(f.data))
	c.buf <- f
	return nil
}

func (c *Conn) acquire(num uint32) (n uint32) {
	n = <-c.ch_win
	if n > num {
		c.ch_win <- (n - num)
		n = num
	}
	return
}

func (c *Conn) release(num uint32) (n uint32) {
	select {
	case n = <-c.ch_win:
		n += num
		c.ch_win <- n
	default:
		n = num
		c.ch_win <- n
	}
	return
}

func (c *Conn) Write(data []byte) (n int, err error) {
	for len(data) > 0 {
		if c.wclosed {
			return n, io.EOF
		}

		size := uint32(len(data))
		// use 1024 as a chunk coz leakbuf 1k
		if size > 1024 {
			size = 1024
		}
		// check for window
		// if window <= 0, wait for window
		size = c.acquire(size)
		if size == 0 {
			return n, io.EOF
		}

		logger.Debugf("%p(%d) send chunk size %d at %d.",
			c.sess, c.streamid, size, n)
		ft := &FrameData{
			streamid: c.streamid,
			data:     data[:size],
		}

		err = c.sess.WriteFrame(ft)
		if err != nil {
			return
		}

		data = data[size:]
		n += int(size)
	}
	logger.Infof("%p(%d) send size %d.", c.sess, c.streamid, n)
	return
}

func (c *Conn) OnRead(window uint32) {
	c.release(window)
	// logger.Debugf("remote readed %d bytes, window: %d.",
	// 	window, c.window.Number())
	logger.Debugf("remote readed %d bytes.",
		window)
	return
}

func (c *Conn) MarkClose() {
	c.wclosed = true
	// weakup writer
	c.ch_win <- 0
	// weakup reader
	if !c.rclosed {
		c.buf <- nil
	}
	c.rclosed = true
}

func (c *Conn) Close() (err error) {
	if c.wclosed {
		return
	}

	logger.Infof("connection %p(%d) closing from local.", c.sess, c.streamid)
	f := &FrameFin{streamid: c.streamid}
	err = c.sess.WriteFrame(f)
	if err != nil {
		logger.Err(err)
	}
	c.wclosed = true

	if c.rclosed && c.wclosed {
		err = c.sess.RemovePorts(c.streamid)
		if err != nil {
			logger.Err(err)
		}
	}
	return
}

func (c *Conn) OnClose() (err error) {
	logger.Infof("connection %p(%d) closed from remote.", c.sess, c.streamid)
	if !c.rclosed {
		c.buf <- nil
	}
	c.rclosed = true

	if c.rclosed && c.wclosed {
		err = c.sess.RemovePorts(c.streamid)
		if err != nil {
			logger.Err(err)
		}
	}
	return
}

func (c *Conn) LocalAddr() net.Addr {
	return &Addr{
		c.sess.LocalAddr(),
		c.streamid,
	}
}

func (c *Conn) RemoteAddr() net.Addr {
	return &Addr{
		c.sess.RemoteAddr(),
		c.streamid,
	}
}

func (c *Conn) SetDeadline(t time.Time) error {
	return nil
}

func (c *Conn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *Conn) SetWriteDeadline(t time.Time) error {
	return nil
}

type Addr struct {
	net.Addr
	streamid uint16
}

func (a *Addr) String() (s string) {
	return fmt.Sprintf("%s(%d)", a.Addr.String(), a.streamid)
}
