package msocks

import (
	"fmt"
	"io"
	"math/rand"
	"net"
	"sync"
	"time"
)

type DelayCnt struct {
	lock  sync.Mutex
	delay time.Duration
	timer *time.Timer
	cnt   int
	do    func(int) error
}

func NewDelayCnt(delay time.Duration) (d *DelayCnt) {
	d = &DelayCnt{
		delay: delay,
	}
	return
}

func (d *DelayCnt) Add() {
	d.lock.Lock()
	defer d.lock.Unlock()

	d.cnt += 1
	if d.cnt >= WIN_SIZE {
		d.do(d.cnt)
		if d.timer != nil {
			d.timer.Stop()
			d.timer = nil
		}
		d.cnt = 0
	}

	if d.cnt != 0 && d.timer == nil {
		d.timer = time.AfterFunc(d.delay, func() {
			d.lock.Lock()
			defer d.lock.Unlock()
			if d.cnt > 0 {
				d.do(d.cnt)
			}
			d.timer = nil
			d.cnt = 0
		})
	}
	return
}

type Pipe struct {
	pr *io.PipeReader
	pw *io.PipeWriter
}

func NewPipe() (p *Pipe) {
	pr, pw := io.Pipe()
	p = &Pipe{pr: pr, pw: pw}
	return
}

func (p *Pipe) Read(data []byte) (n int, err error) {
	n, err = p.pr.Read(data)
	if err == io.ErrClosedPipe {
		err = io.EOF
	}
	return
}

func (p *Pipe) Write(data []byte) (n int, err error) {
	n, err = p.pw.Write(data)
	if err == io.ErrClosedPipe {
		err = io.EOF
	}
	return
}

func (p *Pipe) Close() (err error) {
	p.pr.Close()
	p.pw.Close()
	return
}

type ChanFrameSender chan Frame

func NewChanFrameSender(i int) ChanFrameSender {
	return make(chan Frame, i)
}

func (c ChanFrameSender) Len() int {
	return len(c)
}

func (c ChanFrameSender) RecvWithTimeout(t time.Duration) (f Frame) {
	ch_timeout := time.After(t)
	select {
	case f := <-c:
		return f
	case <-ch_timeout: // timeout
		return nil
	}
}

func (c ChanFrameSender) SendFrame(f Frame) (b bool) {
	defer func() { recover() }()
	select {
	case c <- f:
		return true
	default:
	}
	return
}

func (c ChanFrameSender) Close() (err error) {
	defer func() { recover() }()
	close(c)
	return
}

type Conn struct {
	Pipe
	ChanFrameSender
	SeqWriter
	DelayCnt
	Address    string
	sess       *Session
	streamid   uint16
	removefunc sync.Once
}

func NewConn(streamid uint16, sess *Session, address string) (c *Conn) {
	c = &Conn{
		Pipe:            *NewPipe(),
		ChanFrameSender: NewChanFrameSender(CHANLEN),
		SeqWriter:       *NewSeqWriter(sess),
		DelayCnt:        *NewDelayCnt(ACKDELAY),
		Address:         address,
		streamid:        streamid,
		sess:            sess,
	}
	c.DelayCnt.do = c.send_ack
	go c.Run()
	return
}

func (c *Conn) Run() {
	defer c.Close()

	var err error
	for {
		f, ok := <-c.ChanFrameSender
		if !ok {
			return
		}

		f.Debug()
		switch ft := f.(type) {
		default:
			logger.Err("unexpected package")
			return
		case *FrameData:
			logger.Infof("%p(%d) recved %d bytes from remote.",
				c.sess, ft.Streamid, len(ft.Data))
			c.DelayCnt.Add()
			_, err = c.Pipe.Write(ft.Data)
			switch err {
			case io.EOF:
				logger.Errf("%p(%d) buf is closed.",
					c.sess, c.streamid)
				return
			case nil:
			default:
				logger.Errf("%p(%d) buf is full.",
					c.sess, c.streamid)
				return
			}
		case *FrameAck:
			n := c.SeqWriter.Release(ft.Window)
			logger.Debugf("remote readed %d, window size maybe: %d.",
				ft.Window, n)
		case *FrameFin:
			logger.Infof("connection %p(%d) closed from remote.",
				c.sess, c.streamid)
			return
		}
	}
}

func (c *Conn) send_ack(n int) (err error) {
	logger.Debugf("%p(%d) send ack %d.", c.sess, c.streamid, n)
	// send readed bytes back

	err = c.SeqWriter.Ack(c.streamid, int32(n))
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
		case 4*1024 < size && size <= 8*1024:
			size /= 2
		}

		err = c.SeqWriter.Data(c.streamid, data[:size])
		// write closed, so we don't care window too much.
		if err != nil {
			logger.Err(err)
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
	c.removefunc.Do(func() {
		c.SeqWriter.Close(c.streamid)
		c.Pipe.Close()
		err := c.sess.RemovePorts(c.streamid)
		if err != nil {
			logger.Err(err)
		}
		c.ChanFrameSender.Close()
		logger.Infof("connection %p(%d) closed.", c.sess, c.streamid)
	})
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

func (c *Conn) GetWindowSize() (n uint32) {
	return c.SeqWriter.win
}

func (c *Conn) GetStatus() (s string) {
	if c.SeqWriter.Closed() {
		return "closed"
	} else {
		return "open"
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
