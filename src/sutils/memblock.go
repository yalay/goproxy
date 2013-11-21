package sutils

type LeakBuf struct {
	blocksize int
	freelist  chan []byte
}

func NewLeakBuf(blocksize int, buffernum int) (lb *LeakBuf) {
	return &LeakBuf{
		blocksize: blocksize,
		freelist:  make(chan []byte, buffernum),
	}
}

func (lb *LeakBuf) Get() (b []byte) {
	select {
	case b = <-lb.freelist:
	default:
		b = make([]byte, lb.blocksize)
	}
	return
}

func (lb *LeakBuf) Free(b []byte) {
	select {
	case lb.freelist <- b:
	default:
	}
	return
}

var Klb = NewLeakBuf(1024, 256)
