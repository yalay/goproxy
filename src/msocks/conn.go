package msocks

import (
	"fmt"
	"io"
	"math/rand"
	"net"
	"sync"
	"time"
)

type Conn struct {
	sess       *Session
	streamid   uint16
	ch_f       chan Frame
	removefunc sync.Once
	dd         *DelayDo
	sw         *SeqWriter
	Pipe
}

func NewConn(streamid uint16, sess *Session) (c *Conn) {
	c = &Conn{
		streamid: streamid,
		sess:     sess,
		ch_f:     make(chan Frame, CHANLEN),
		dd:       NewDelayDo(ACKDELAY, nil),
		sw:       NewSeqWriter(sess),
		Pipe:     *NewPipe(),
	}
	c.dd.do = c.send_ack
	go c.Run()
	return
}

func (c *Conn) Run() {
	var err error
	for {
		f, ok := <-c.ch_f
		if !ok {
			c.CloseAll()
			return
		}
		c.dd.Add(1)

		switch ft := f.(type) {
		default:
			logger.Err("unexpected package")
			c.CloseAll()
			return
		case *FrameData:
			f.Debug()
			if len(ft.Data) == 0 {
				continue
			}
			logger.Debugf("%p(%d) recved %d bytes from remote.",
				c.sess, ft.Streamid, len(ft.Data))
			_, err = c.Pipe.Write(ft.Data)
			switch err {
			case io.EOF:
				logger.Errf("%p(%d) buf is closed.",
					c.sess, c.streamid)
				c.CloseAll()
				return
			case nil:
			default:
				logger.Errf("%p(%d) buf is full.",
					c.sess, c.streamid)
				c.CloseAll()
				return
			}
		case *FrameAck:
			f.Debug()
			n := c.sw.Release(ft.Window)
			logger.Debugf("remote readed %d, window size maybe: %d.",
				ft.Window, n)
		case *FrameFin:
			f.Debug()
			c.Pipe.Close()
			logger.Infof("connection %p(%d) closed from remote.",
				c.sess, c.streamid)
			if c.sw.Closed() {
				c.remove_port()
			}
			return
		}
	}
}

func (c *Conn) send_ack(n int) (err error) {
	logger.Debugf("%p(%d) send ack %d.", c.sess, c.streamid, n)
	// send readed bytes back

	err = c.sw.Ack(c.streamid, int32(n))
	if err != nil {
		logger.Err(err)
		c.Close()
	}
	return
}

func (c *Conn) Write(data []byte) (n int, err error) {
	for len(data) > 0 {
		size := uint32(len(data))
		// random size
		switch {
		case size > 8*1024:
			size = uint32(3*1024 + rand.Intn(1024))
		case 4*1024 < size && size < 8*1024:
			size /= 2
		}

		err = c.sw.Data(c.streamid, data[:size])
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

func (c *Conn) remove_port() {
	c.removefunc.Do(func() {
		err := c.sess.RemovePorts(c.streamid)
		if err != nil {
			logger.Err(err)
		}
		defer func() { recover() }()
		close(c.ch_f)
	})
}

func (c *Conn) Close() (err error) {
	// make sure just one will enter this func
	err = c.sw.Close(c.streamid)
	if err == io.EOF {
		// ok for already closed
		err = nil
	}
	if err != nil {
		return err
	}

	logger.Infof("connection %p(%d) closing from local.", c.sess, c.streamid)

	if c.Pipe.Closed {
		c.remove_port()
	}
	return
}

func (c *Conn) CloseAll() {
	c.sw.Close(c.streamid)
	c.Pipe.Close()
	c.remove_port()
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
