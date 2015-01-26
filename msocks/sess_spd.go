package msocks

import (
	"sync/atomic"
	"time"
)

type SpeedCounter struct {
	readcnt  uint32
	readbps  uint32
	writecnt uint32
	writebps uint32
	s        *Session
}

func NewSpeedCounter(s *Session) (sc *SpeedCounter) {
	sc = &SpeedCounter{s: s}
	go sc.loop_count()
	return
}

func (sc *SpeedCounter) loop_count() {
	for !sc.s.closed {
		sc.readbps = atomic.SwapUint32(&sc.readcnt, 0) / SHRINK_TIME
		sc.writebps = atomic.SwapUint32(&sc.writebps, 0) / SHRINK_TIME
		time.Sleep(SHRINK_TIME * time.Second)
	}
}

func (sc *SpeedCounter) ReadBytes(s uint32) uint32 {
	return atomic.AddUint32(&sc.readcnt, s)
}

func (sc *SpeedCounter) WriteBytes(s uint32) uint32 {
	return atomic.AddUint32(&sc.writecnt, s)
}

func (sc *SpeedCounter) GetReadSpeed() uint32 {
	return sc.readbps
}

func (sc *SpeedCounter) GetWriteSpeed() uint32 {
	return sc.writebps
}
