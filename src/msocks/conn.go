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
	wclosed   bool
	closefunc sync.Once
	wlock     sync.Mutex
	ch_win    chan uint32

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

func (c *Conn) writeSafe(b []byte) (err error) {
	c.wlock.Lock()
	defer c.wlock.Unlock()
	if c.wclosed {
		return io.EOF
	}
	return c.sess.Write(b)
}

func (c *Conn) SendAck(n int) {
	c.delaylock.Lock()
	defer c.delaylock.Unlock()

	if c.delayack == nil {
		// to avoid silly window symptom
		c.delayack = time.AfterFunc(100*time.Millisecond, func() {
			logger.Debugf("%p(%d) send ack %d.",
				c.sess, c.streamid, c.delaycnt)
			// send readed bytes back
			b, err := NewFrameAck(c.streamid, uint32(c.delaycnt))
			if err != nil {
				logger.Err(err)
				return
			}

			// rclose will got wrong
			// When we close first, remote sent fin in air.
			// At this moment, wclosed setted and rclosed not set.
			// Check rclosed will send a ack, which remote already removed port.
			err = c.writeSafe(b)
			switch err {
			case nil:
			case io.EOF:
				return
			default:
				logger.Err(err)
				return
			}

			c.delaylock.Lock()
			defer c.delaylock.Unlock()
			c.delayack = nil
			c.delaycnt = 0
		})
	}

	c.delaycnt += n
	return
}

// TODO: one read in same time.
func (c *Conn) Read(data []byte) (n int, err error) {
	if c.rclosed {
		return 0, io.EOF
	}
	if c.bufhead == nil {
		c.bufhead = <-c.buf
		c.bufpos = 0
	}
	if c.bufhead == nil {
		// weak up next.
		c.buf <- nil
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
	select {
	case c.buf <- f:
		return nil
	default:
	}
	return fmt.Errorf("we are in big trouble, because %p(%d) c.buf is full.",
		c.sess, c.streamid)
}

func (c *Conn) acquire(num uint32) (n uint32) {
	n = <-c.ch_win
	switch {
	case n == 0: // weak up next
		c.ch_win <- 0
	case n > num:
		c.ch_win <- (n - num)
		n = num
	}
	return
}

func (c *Conn) Write(data []byte) (n int, err error) {
	var b []byte
	for len(data) > 0 {
		size := uint32(len(data))
		// use 1024 as a chunk coz leakbuf 1k
		// TODO: random size
		if size > 1024 {
			size = 1024
		}
		// check for window
		// if window <= 0, wait for window
		size = c.acquire(size)
		b, err = NewFrameData(c.streamid, data[:size])
		if err != nil {
			logger.Err(err)
			return
		}

		err = c.writeSafe(b)
		// write closed, so we don't care window too much.
		if err != nil {
			return
		}
		logger.Debugf("%p(%d) send chunk size %d at %d.",
			c.sess, c.streamid, size, n)

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
	c.wclosed = true
	c.rclosed = true
	// weakup writer
	c.ch_win <- 0
	// weakup reader
	if !c.rclosed {
		c.buf <- nil
	}
}

func (c *Conn) Close() (err error) {
	c.wlock.Lock()
	if c.wclosed {
		c.wlock.Unlock()
		return
	}
	c.wclosed = true
	c.wlock.Unlock()

	c.ch_win <- 0
	logger.Infof("connection %p:%d closing from local.", c.sess, c.streamid)

	b, err := NewFrameFin(c.streamid)
	if err != nil {
		logger.Err(err)
	} else {
		err = c.sess.Write(b)
		if err != nil {
			logger.Err(err)
		}
	}

	if c.rclosed {
		c.closefunc.Do(func() {
			err := c.sess.RemovePorts(c.streamid)
			if err != nil {
				logger.Err(err)
			}
		})
	}
	return
}

func (c *Conn) OnClose() (err error) {
	if c.rclosed {
		return
	}
	c.rclosed = true
	select {
	case c.buf <- nil:
	default:
	}
	logger.Infof("connection %p:%d closed from remote.", c.sess, c.streamid)

	if c.wclosed {
		c.closefunc.Do(func() {
			err := c.sess.RemovePorts(c.streamid)
			if err != nil {
				logger.Err(err)
			}
		})
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
