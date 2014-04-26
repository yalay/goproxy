package msocks

import (
	"container/list"
	"fmt"
	"io"
	"math/rand"
	"net"
	"sync"
	"time"
)

const (
	ST_UNKNOWN = iota
	ST_SYN_RECV
	ST_SYN_SENT
	ST_EST
	ST_CLOSE_WAIT
	ST_FIN_WAIT
	ST_TIME_WAIT
)

type Queue struct {
	lock   sync.Mutex
	ev     *sync.Cond
	queue  *list.List
	closed bool
}

func NewQueue() (q *Queue) {
	q = &Queue{
		queue:  list.New(),
		closed: false,
	}
	q.ev = sync.NewCond(&q.lock)
	return
}

func (q *Queue) Push(v interface{}) (err error) {
	q.lock.Lock()
	defer q.lock.Unlock()
	if q.closed {
		return ErrQueueClosed
	}
	q.queue.PushBack(v)
	q.ev.Signal()
	return
}

func (q *Queue) Pop(block bool) (v interface{}, err error) {
	q.lock.Lock()
	defer q.lock.Unlock()
	var e *list.Element
	for e = q.queue.Front(); e == nil; e = q.queue.Front() {
		if q.closed {
			return nil, ErrQueueClosed
		}
		if !block {
			return
		}
		q.ev.Wait()
	}
	v = e.Value
	q.queue.Remove(e)
	return
}

func (q *Queue) Close() (err error) {
	q.lock.Lock()
	defer q.lock.Unlock()
	if q.closed {
		return
	}
	q.closed = true
	q.ev.Broadcast()
	return
}

type Conn struct {
	lock     sync.Mutex
	status   uint8
	sess     *Session
	streamid uint16
	sender   FrameSender
	ch       chan uint32
	Address  string

	rlock    sync.Mutex // this should used to block reader and reader, not writer
	rbufsize uint32
	r_rest   []byte
	rqueue   *Queue

	wlock    sync.Mutex
	wbufsize uint32
	wev      *sync.Cond
}

func NewConn(status uint8, streamid uint16, sess *Session, address string) (c *Conn) {
	c = &Conn{
		status:   status,
		sess:     sess,
		streamid: streamid,
		sender:   sess,
		Address:  address,
		rqueue:   NewQueue(),
	}
	c.wev = sync.NewCond(&c.wlock)
	return
}

func (c *Conn) Final() {
	c.rqueue.Close()
	err := c.sess.RemovePorts(c.streamid)
	if err != nil {
		log.Error("%s", err)
	}
	log.Info("connection %p(%d) closed.", c.sess, c.streamid)
	c.status = ST_UNKNOWN
}

func (c *Conn) Close() (err error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	switch c.status {
	case ST_EST:
		fb := NewFrameFin(c.streamid)
		c.sender.SendFrame(fb)
		c.status = ST_FIN_WAIT
	case ST_CLOSE_WAIT:
		c.Final()
	}

	return
}

func (c *Conn) SendFrame(f Frame) bool {
	switch ft := f.(type) {
	default:
		log.Error("unexpected package")
		c.Close()
		return false
	case *FrameResult:
		return c.InConnect(ft.Errno)
	case *FrameData:
		return c.InData(ft)
	case *FrameWnd:
		return c.InWnd(ft)
	case *FrameFin:
		return c.InFin(ft)
	case *FrameRst:
		log.Debug("reset %p(%d), sender %p.", c.sess, ft.Streamid, c)
		c.Final()
	}
	return true
}

func (c *Conn) InConnect(errno uint32) bool {
	if c.status != ST_SYN_SENT {
		log.Error("frame ok or failed in conn which status is not syn")
		return false
	}
	if errno == ERR_NONE {
		c.status = ST_EST
		log.Notice("new conn: %p(%d) => %s.",
			c.sess, c.streamid, c.Address)
	} else {
		c.Final()
	}
	select {
	case c.ch <- errno:
	default:
	}
	return true
}

func (c *Conn) InData(ft *FrameData) bool {
	log.Info("%p(%d) recved %d bytes from remote.",
		c.sess, ft.Streamid, len(ft.Data))
	err := c.rqueue.Push(ft.Data)
	if err != nil {
		log.Debug("%s", err)
		return false
	}
	c.rbufsize += uint32(len(ft.Data))
	return true
}

func (c *Conn) InWnd(ft *FrameWnd) bool {
	c.wlock.Lock()
	defer c.wlock.Unlock()
	c.wbufsize -= ft.Window
	c.wev.Signal()
	log.Debug("remote readed %d, write buffer size: %d.",
		ft.Window, c.wbufsize)
	return true
}

func (c *Conn) InFin(ft *FrameFin) bool {
	log.Info("connection %p(%d) closed from remote.",
		c.sess, c.streamid)
	c.lock.Lock()
	defer c.lock.Unlock()

	// always need to close read pipe
	// coz fin means remote will never send data anymore
	c.rqueue.Close()
	switch c.status {
	case ST_EST:
		c.status = ST_CLOSE_WAIT
		// close read pipe but not sent fin back
		// wait reader to close
		return true
	case ST_FIN_WAIT:
		c.status = ST_TIME_WAIT
		// wait for 2*MSL and final
		time.AfterFunc(HALFCLOSE, c.Final)
		// in final rqueue.close will be call again, that's ok
		return true
	}
	// error
	return false
}

func (c *Conn) CloseFrame() error {
	// maybe conn closed
	c.rqueue.Close()
	return nil
}

func (c *Conn) Read(data []byte) (n int, err error) {
	var v interface{}
	c.rlock.Lock()
	defer c.rlock.Unlock()

	target := data[:]
	block := true
	for len(target) > 0 {
		if c.r_rest == nil {
			// reader should be blocked in here
			v, err = c.rqueue.Pop(block)
			if err == ErrQueueClosed {
				err = io.EOF
			}
			if err != nil {
				return
			}
			if v == nil {
				break
			}
			c.r_rest = v.([]byte)
		}

		size := copy(target, c.r_rest)
		target = target[size:]
		n += size
		block = false

		if len(c.r_rest) > size {
			c.r_rest = c.r_rest[size:]
		} else {
			// take all data in rest
			c.r_rest = nil
		}
	}

	c.rbufsize -= uint32(n)
	fb := NewFrameWnd(c.streamid, uint32(n))
	c.sender.SendFrame(fb)
	// TODO: 合并
	return
}

func (c *Conn) Write(data []byte) (n int, err error) {
	c.wlock.Lock()
	defer c.wlock.Unlock()

	for len(data) > 0 {
		size := uint32(len(data))
		// random size
		switch {
		case size > 8*1024:
			size = uint32(3*1024 + rand.Intn(1024))
		case 4*1024 < size && size <= 8*1024:
			size /= 2
		}

		err = c.WriteSlice(data[:size])

		if err != nil {
			log.Error("%s", err)
			return
		}
		log.Debug("%p(%d) send chunk size %d at %d.",
			c.sess, c.streamid, size, n)

		data = data[size:]
		n += int(size)
	}
	log.Info("%p(%d) send size %d.", c.sess, c.streamid, n)
	return
}

func (c *Conn) WriteSlice(data []byte) (err error) {
	f := NewFrameData(c.streamid, data)

	if c.status != ST_EST && c.status != ST_CLOSE_WAIT {
		log.Error("status %d found in write slice", c.status)
		panic("status error")
	}

	log.Debug("write buffer size: %d, write len: %d", c.wbufsize, len(data))
	for c.wbufsize+uint32(len(data)) > WINDOWSIZE {
		c.wev.Wait()
	}

	if !c.sender.SendFrame(f) {
		err = io.EOF
		return
	}
	c.wbufsize += uint32(len(data))
	c.wev.Signal()
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

func (c *Conn) GetStatus() (st string) {
	switch c.status {
	case ST_EST:
		return "ESTABLISHED"
	case ST_CLOSE_WAIT:
		return "CLOSEWAIT"
	case ST_FIN_WAIT:
		return "FINWAIT"
	case ST_TIME_WAIT:
		return "TIMEWAIT"
	}
	return "UNKNOWN"
}

func (c *Conn) GetReadBufSize() (n uint32) {
	return c.rbufsize
}

func (c *Conn) GetWriteBufSize() (n uint32) {
	return c.wbufsize
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
	return fmt.Sprintf("%s:%d:", a.Addr.String(), a.streamid)
}
