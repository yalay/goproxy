package msocks

import (
	"io"
	"sync"
)

// write in seq,
type SeqWriter struct {
	closed bool
	lock   sync.Mutex
	sess   *Session
}

func NewSeqWriter(sess *Session) (sw *SeqWriter) {
	return &SeqWriter{sess: sess}
}

func (sw *SeqWriter) Ack(streamid uint16, n int) (err error) {
	b := NewFrameOneInt(MSG_ACK, streamid, uint32(n))
	err = sw.WriteStream(streamid, b)
	if err == io.EOF {
		err = nil
	}
	return
}

func (sw *SeqWriter) Data(streamid uint16, data []byte) (err error) {
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
	if err != nil {
		logger.Err(err)
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

	// send fin if not closed yet.
	b := NewFrameNoParam(MSG_FIN, streamid)
	err = sw.sess.WriteStream(streamid, b)
	if err == io.EOF {
		err = nil
	}
	if err != nil {
		logger.Err(err)
	}
	return
}

func (sw *SeqWriter) Closed() bool {
	return sw.closed
}
