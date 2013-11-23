package msocks

import (
	"errors"
	"io"
	"sync"
)

type Bytebuf struct {
	buf     chan *FrameData
	bufhead *FrameData
	bufpos  int
	lock    sync.Mutex
	closed  bool
}

func NewBytebuf(size int) (b *Bytebuf) {
	b = &Bytebuf{
		buf: make(chan *FrameData, size),
	}
	return
}

func (b *Bytebuf) Append(f *FrameData) (err error) {
	if b.closed {
		f.Free()
		return
	}
	select {
	case b.buf <- f:
	default:
		err = errors.New("buf full.")
	}
	return
}

func (b *Bytebuf) Read(data []byte) (n int, err error) {
	b.lock.Lock()
	defer b.lock.Unlock()
	if b.closed {
		return 0, io.EOF
	}
	if b.bufhead == nil {
		b.bufhead = <-b.buf
		b.bufpos = 0
	}
	if b.bufhead == nil {
		// weak up next.
		b.buf <- nil
		return 0, io.EOF
	}

	n = len(b.bufhead.Data) - b.bufpos
	if n > len(data) {
		n = len(data)
		copy(data, b.bufhead.Data[b.bufpos:b.bufpos+n])
		logger.Debugf("read %d of head chunk at %d.",
			n, b.bufpos)
		b.bufpos += n
	} else {
		copy(data, b.bufhead.Data[b.bufpos:])
		logger.Debugf("read all.")
		b.bufhead.Free()
		b.bufhead = nil
	}
	return
}

func (b *Bytebuf) Close() (err error) {
	b.closed = true
	b.buf <- nil
	return
}
