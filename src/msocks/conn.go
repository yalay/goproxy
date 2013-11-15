package msocks

import (
	"io"
	"net"
	"sutils"
	"time"
)

type Conn struct {
	sess     *Session
	streamid uint16
	rclosed  bool
	wclosed  bool
	window   *sutils.WaitNum
	buf      chan *FrameData
	bufhead  *FrameData
	bufpos   int
}

func NewConn(streamid uint16, sess *Session) (c *Conn) {
	// use 1024 as default channel length, 1024 * 1024 = 1M
	// that is the buffer before read
	// and it's the maxmium length of write window.
	// BTW, default value of write window is 256K.
	c = &Conn{
		streamid: streamid,
		sess:     sess,
		rclosed:  false,
		wclosed:  false,
		window:   sutils.NewWaitNum(256 * 1024),
		buf:      make(chan *FrameData, 1024),
		bufpos:   0,
	}
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

	n = len(c.bufhead.data) - c.bufpos
	if n > len(data) {
		n = len(data)
		copy(data, c.bufhead.data[c.bufpos:c.bufpos+n])
		logger.Debugf("read %d of head chunk at %d.",
			n, c.bufpos)
		c.bufpos += n
	} else {
		copy(data, c.bufhead.data[c.bufpos:])
		logger.Debugf("read all of head chunk.")
		c.bufhead.Free()
		c.bufhead = nil

	}

	// TODO: silly window symptom
	// send readed bytes back
	ft := &FrameAck{
		streamid: c.streamid,
		window:   uint32(n),
	}
	err = c.sess.WriteFrame(ft)
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
		size = c.window.Acquire(size)

		logger.Debugf("send chunk size %d at %d.", size, n)
		ft := &FrameData{
			streamid: c.streamid,
			data:     data[:size],
		}

		err = c.sess.WriteFrame(ft)
		if err != nil {
			// TODO: we are in big trouble, should break link.
			return
		}

		data = data[size:]
		n += int(size)
	}
	return
}

func (c *Conn) MarkClose() {
	c.wclosed = true
	c.rclosed = true
}

func (c *Conn) Close() (err error) {
	if c.rclosed && c.wclosed {
		return
	}

	logger.Debugf("connection %x(%d) closing from local.", c, c.streamid)
	f := &FrameFin{streamid: c.streamid}
	err = c.sess.WriteFrame(f)
	if err != nil {
		logger.Err(err)
	}

	c.wclosed = true
	if c.rclosed && c.wclosed {
		err = c.sess.ClosePort(c.streamid)
		if err != nil {
			logger.Err(err)
		}
	}
	return
}

func (c *Conn) OnRecv(f *FrameData) (err error) {
	if c.rclosed {
		return
	}
	if len(f.data) == 0 {
		return nil
	}

	logger.Debugf("recved %d bytes from remote.", len(f.data))
	c.buf <- f
	return nil
}

func (c *Conn) OnClose() (err error) {
	logger.Debugf("connection %x(%d) closed from remote.", c, c.streamid)
	c.rclosed = true
	if c.rclosed && c.wclosed {
		err = c.sess.ClosePort(c.streamid)
		if err != nil {
			logger.Err(err)
		}
	}
	return
}

func (c *Conn) OnRead(window uint32) {
	logger.Debugf("remote readed %d bytes.", window)
	c.window.Release(window)
	return
}

// TODO: use user defined addr
func (c *Conn) LocalAddr() net.Addr {
	return c.sess.LocalAddr()
}

func (c *Conn) RemoteAddr() net.Addr {
	return c.sess.RemoteAddr()
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
