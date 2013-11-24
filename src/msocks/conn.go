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
	ch_f     chan Frame
	// from local to remote
	removefunc sync.Once
	Window
	Bytebuf
	DelayDo
	SeqWriter
}

// use 1024 as default channel length, 1024 * 1024 = 1M
// that is the buffer before read
// and it's the maxmium length of write window.
// default value of write window is 256K.
// that will be sent in 0.1s, so maxmium speed will be 2.56M/s = 20Mbps.
func NewConn(streamid uint16, sess *Session) (c *Conn) {
	c = &Conn{
		streamid:  streamid,
		sess:      sess,
		ch_f:      make(chan Frame, 1024),
		Window:    *NewWindow(256 * 1024),
		Bytebuf:   *NewBytebuf(1024),
		DelayDo:   *NewDelayDo(ACKDELAY, nil),
		SeqWriter: *NewSeqWriter(sess),
	}
	c.DelayDo.do = c.send_ack
	go c.Run()
	return
}

// close all, weakup writer, reader, quit loop.
func (c *Conn) Final() {
	c.Bytebuf.Close()
	c.SeqWriter.Close()
	c.Window.Close()
	c.ch_f <- nil
	c.RemovePort()
}

// disconnected from session.
func (c *Conn) RemovePort() {
	c.removefunc.Do(func() {
		err := c.sess.RemovePorts(c.streamid)
		if err != nil {
			logger.Err(err)
		}
	})
}

func (c *Conn) Run() {
	defer c.RemovePort()

	var err error
	for {
		f := <-c.ch_f
		if f == nil {
			return
		}

		switch ft := f.(type) {
		default:
			logger.Err("unexpected package")
			c.Final()
			return
		case *FrameData:
			f.Debug()
			if len(ft.Data) == 0 {
				continue
			}
			logger.Debugf("%p(%d) recved %d bytes from remote.",
				c.sess, ft.Streamid, len(ft.Data))
			err = c.Append(ft)
			if err != nil {
				logger.Errf("big trouble, %p(%d) buf is full.",
					c.sess, c.streamid)
				c.Final()
				return
			}
		case *FrameAck:
			f.Debug()
			n := c.Release(ft.Window)
			logger.Debugf("remote readed %d bytes, window size maybe: %d.",
				ft.Window, n)
		case *FrameFin:
			f.Debug()
			c.Bytebuf.Close()
			c.Window.Close()
			logger.Infof("connection %p(%d) closed from remote.",
				c.sess, c.streamid)
			if c.SeqWriter.Closed() {
				c.RemovePort()
			}
			return
		}
	}
}

func (c *Conn) Read(data []byte) (n int, err error) {
	n, err = c.Bytebuf.Read(data)
	if err != nil {
		return
	}

	c.Add(n)
	return
}

func (c *Conn) send_ack(n int) (err error) {
	logger.Debugf("%p(%d) send ack %d.", c.sess, c.streamid, n)
	// send readed bytes back
	b := NewFrameAck(c.streamid, uint32(n))

	_, err = c.SeqWriter.Write(b)
	if err != nil && err != io.EOF {
		logger.Err(err)
		c.Final()
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
		size = c.Acquire(size)
		b, err = NewFrameData(c.streamid, data[:size])
		if err != nil {
			logger.Err(err)
			return
		}

		_, err = c.SeqWriter.Write(b)
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

func (c *Conn) Close() (err error) {
	// make sure just one will enter this func
	err = c.SeqWriter.Close()
	if err != nil {
		// ok for already closed
		return nil
	}

	c.Window.Close()
	logger.Infof("connection %p(%d) closing from local.", c.sess, c.streamid)

	// send fin if not closed yet.
	b := NewFrameFin(c.streamid)
	_, err = c.sess.Write(b)
	if err != nil {
		logger.Err(err)
	}

	if c.Bytebuf.Closed() {
		c.RemovePort()
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
