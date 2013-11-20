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
	// from remote to local
	rclosed bool
	// from local to remote
	wclosed bool

	lock    sync.Mutex
	ch_win  chan uint32
	buf     chan *FrameData
	bufhead *FrameData
	bufpos  int

	delayack *time.Timer
	delaycnt int
}

func NewConn(streamid uint16, sess *Session) (c *Conn) {
	c = &Conn{
		streamid: streamid,
		sess:     sess,
		rclosed:  false,
		wclosed:  false,
		ch_win:   make(chan uint32, 10),
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
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.delayack == nil {
		// to avoid silly window symptom
		c.delayack = time.AfterFunc(100*time.Millisecond, func() {
			c.lock.Lock()
			defer c.lock.Unlock()
			if c.rclosed {
				return
			}

			logger.Debugf("%p(%d) send ack %d.",
				c.sess, c.streamid, c.delaycnt)
			// send readed bytes back
			ft := NewFrameAck(c.streamid, uint32(c.delaycnt))
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
	if c.rclosed {
		return 0, io.EOF
	}
	if c.bufhead == nil {
		c.bufhead = <-c.buf
		c.bufpos = 0
	}
	if c.bufhead == nil {
		return 0, io.EOF
	}

	n = len(c.bufhead.Data) - c.bufpos
	if n > len(data) {
		n = len(data)
		copy(data, c.bufhead.Data[c.bufpos:c.bufpos+n])
		logger.Debugf("read %d of head chunk at %d.",
			n, c.bufpos)
		c.bufpos += n
	} else {
		copy(data, c.bufhead.Data[c.bufpos:])
		logger.Debugf("read all.")
		c.bufhead.Free()
		c.bufhead = nil

	}

	c.SendAck(n)
	return
}

func (c *Conn) OnRecv(f *FrameData) (err error) {
	// to save time
	if c.rclosed {
		f.Free()
		return
	}
	if len(f.Data) == 0 {
		return nil
	}

	logger.Debugf("%p(%d) recved %d bytes from remote.",
		c.sess, f.Streamid, len(f.Data))
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
		ft := NewFrameData(c.streamid, data[:size])
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

func (c *Conn) release(num uint32) (n uint32) {
	n = num
	for {
		select {
		case m := <-c.ch_win:
			n += m
		default:
			c.ch_win <- n
			return
		}
	}
}

func (c *Conn) OnRead(window uint32) {
	n := c.release(window)
	logger.Debugf("remote readed %d bytes, window size maybe: %d.",
		window, n)
	return
}

func (c *Conn) MarkClose() {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.wclosed = true
	// weakup writer
	c.ch_win <- 0
	// weakup reader
	c.rclosed = true
	if !c.rclosed {
		c.buf <- nil
	}
}

func (c *Conn) Close() (err error) {
	c.lock.Lock()
	if c.wclosed {
		c.lock.Unlock()
		return
	}

	logger.Infof("connection %p(%d) closing from local.", c.sess, c.streamid)
	f := NewFrameFin(c.streamid)
	err = c.sess.WriteFrame(f)
	if err != nil {
		logger.Err(err)
	}
	c.wclosed = true
	c.ch_win <- 0
	c.lock.Unlock()

	if c.rclosed && c.wclosed {
		err = c.sess.RemovePorts(c.streamid)
		if err != nil {
			logger.Err(err)
		}
	}
	return
}

func (c *Conn) OnClose() (err error) {
	c.lock.Lock()
	if c.rclosed {
		c.lock.Unlock()
		return
	}

	logger.Infof("connection %p(%d) closed from remote.", c.sess, c.streamid)
	c.rclosed = true
	c.buf <- nil
	c.lock.Unlock()

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
