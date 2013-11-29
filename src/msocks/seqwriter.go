package msocks

import (
	"io"
	"sync"
)

type Window struct {
	c      *sync.Cond
	mu     *sync.Mutex
	closed bool
	win    uint32
}

func NewWindow(init uint32) (w *Window) {
	var mu sync.Mutex
	w = &Window{
		c:   sync.NewCond(&mu),
		mu:  &mu,
		win: init,
	}
	return
}

func (w *Window) Close() (err error) {
	w.closed = true
	w.c.Broadcast()
	return
}

func (w *Window) Acquire(num uint32) (n uint32) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for {
		switch {
		case w.closed:
			break
		case w.win == 0:
			w.c.Wait()
			continue
		case w.win < num:
			n = w.win
		case w.win > num:
			n = num
		}
		w.win -= n
		return
	}
	return
}

func (w *Window) Release(num uint32) (n uint32) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.win += num
	n = w.win
	w.c.Broadcast()
	return
}

// write in seq,
type SeqWriter struct {
	Window
	closed bool
	lock   sync.Mutex
	sess   *Session
}

func NewSeqWriter(sess *Session) (sw *SeqWriter) {
	return &SeqWriter{
		Window: *NewWindow(WIN_SIZE),
		sess:   sess,
	}
}

func (sw *SeqWriter) Ack(streamid uint16, n int32) (err error) {
	b := NewFrameOneInt(MSG_ACK, streamid, uint32(n))
	err = sw.WriteStream(streamid, b)
	if err == io.EOF {
		err = nil
	}
	return
}

func (sw *SeqWriter) Data(streamid uint16, data []byte) (err error) {
	// check for window
	if sw.Acquire(1) == 0 {
		// that mean closed
		return io.EOF
	}
	b, err := NewFrameData(streamid, data)
	if err != nil {
		logger.Err(err)
		return
	}
	err = sw.WriteStream(streamid, b)
	if err == io.EOF {
		err = nil
	}
	return
}

func (sw *SeqWriter) WriteStream(streamid uint16, b []byte) (err error) {
	sw.lock.Lock()
	defer sw.lock.Unlock()
	if sw.closed {
		return io.EOF
	}
	err = sw.sess.WriteStream(streamid, b)
	if err == io.EOF {
		sw.closed = true
	}
	return
}

func (sw *SeqWriter) Close(streamid uint16) (err error) {
	sw.lock.Lock()
	defer sw.lock.Unlock()
	if sw.closed {
		return io.EOF
	}
	sw.closed = true
	sw.Window.Close()

	// send fin if not closed yet.
	b := NewFrameNoParam(MSG_FIN, streamid)
	err = sw.sess.WriteStream(streamid, b)
	if err == io.EOF {
		err = nil
	}
	return
}

func (sw *SeqWriter) Closed() bool {
	return sw.closed
}
