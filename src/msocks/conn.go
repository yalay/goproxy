package msocks

import (
	"io"
	"net"
	"time"
)

type Stream interface {
	OnRecv([]byte) (int, error)
	OnClose() error
	OnRead(uint32)
}

type ServiceStream struct {
	streamid uint16
	window   uint32
	sess     *Session
	closed   bool
}

func (ss *ServiceStream) Read(data []byte) (n int, err error) {
	if ss.closed {
		return 0, io.EOF
	}
	// TODO: read from buf
	// send (max_window - bufsize) back
	err = ss.sess.OnRead(ss.streamid, 0)
	return
}

func (ss *ServiceStream) writeData(data []byte) (err error) {
	ft := &FrameData{
		streamid: ss.streamid,
		data:     data,
	}
	err = ss.sess.WriteFrame(ft)
	if err != nil {
		return
	}

	ss.window -= uint32(len(data))
	return
}

func (ss *ServiceStream) Write(data []byte) (n int, err error) {
	for len(data) > 0 {
		// check for window
		// if window <= 0, wait for window

		if ss.closed {
			return n, io.EOF
		}

		size := uint32(len(data))
		if ss.window < size {
			size = ss.window
		}

		err = ss.writeData(data[:size])
		if err != nil {
			return
		}

		data = data[size:]
		n += int(size)
	}
	return
}

func (ss *ServiceStream) Close() (err error) {
	if ss.closed {
		return
	}

	f := &FrameFin{streamid: ss.streamid}
	err = ss.sess.WriteFrame(f)
	if err != nil {
		logger.Err(err)
	}
	return
}

func (ss *ServiceStream) OnRecv(data []byte) (n int, err error) {
	// TODO: write to buf
	return
}

func (ss *ServiceStream) OnClose() (err error) {
	ss.closed = true
	return
}

func (ss *ServiceStream) OnRead(window uint32) {
	// TODO:
	// notice reader
	ss.window = window
	return
}

func (ss *ServiceStream) LocalAddr() net.Addr {
	return nil
}

func (ss *ServiceStream) RemoteAddr() net.Addr {
	return nil
}

func (ss *ServiceStream) SetDeadline(t time.Time) error {
	return nil
}

func (ss *ServiceStream) SetReadDeadline(t time.Time) error {
	return nil
}

func (ss *ServiceStream) SetWriteDeadline(t time.Time) error {
	return nil
}

// type Conn struct {
// 	s        *Session
// 	streamid uint16

// 	write_closed bool
// 	read_closed  bool

// 	write_window uint32
// 	pr           io.PipeReader // will this block?
// 	pw           io.PipeWriter
// 	conn         net.Conn
// }

// func (c *Conn) Read(p []byte) (n int, err error) {
// 	if c.read_closed {
// 		return 0, io.EOF
// 	}

// 	n, err = c.pr.Read(p)
// 	if err != nil {
// 		return
// 	}

// 	c.write_window += n
// 	// c.c.Write()
// 	// read data
// 	// send msg_ack back
// 	return
// }

// func (c *Conn) Write(p []byte) (n int, err error) {
// 	if c.write_closed {
// 		return 0, io.EOF
// 	}

// 	// check c.write_window
// 	f := &FrameData{streamid: c.streamid, data: p}
// 	err = c.s.WriteFrame(f)
// 	return
// }

// func (c *Conn) Close() error {
// 	c.write_closed = true
// 	f := &FrameFin{streamid: c.streamid}
// 	f.WriteFrame(c.s.w)
// 	if c.read_closed {
// 		c.on_close()
// 	}
// 	return nil
// }

// func (c *Conn) on_close() {
// 	c.idlock.Lock()
// 	defer c.idlock.Unlock()
// 	delete(c.s.streams, c.streamid)
// }

// func (c *Conn) LocalAddr() Addr {

// }

// func (c *Conn) RemoteAddr() Addr {

// }

// func (c *Conn) SetDeadline(t time.Time) error {

// }

// func (c *Conn) SetReadDeadline(t time.Time) error {

// }

// func (c *Conn) SetWriteDeadline(t time.Time) error {

// }
