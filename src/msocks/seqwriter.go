package msocks

import (
	"errors"
	"io"
	"sync"
)

type SeqWriter struct {
	closed bool
	lock   sync.Mutex
	w      io.Writer
}

func NewSeqWriter(w io.Writer) (sw *SeqWriter) {
	return &SeqWriter{w: w}
}

func (sw *SeqWriter) Write(b []byte) (n int, err error) {
	sw.lock.Lock()
	defer sw.lock.Unlock()
	if sw.closed {
		return 0, io.EOF
	}
	n, err = sw.w.Write(b)
	if err != nil {
		logger.Err(err)
	}
	return
}

func (sw *SeqWriter) Close() (err error) {
	sw.lock.Lock()
	defer sw.lock.Unlock()
	if sw.closed {
		return errors.New("closed already.")
	}
	sw.closed = true
	return
}

func (sw *SeqWriter) Closed() bool {
	return sw.closed
}
