package msocks

import (
	"sync/atomic"
	"time"
)

type SpeedCounter struct {
	readcnt  int32
	readbps  int32
	writecnt int32
	writebps int32
	s        *Session
}

func NewSpeedCounter(s *Session) (sc *SpeedCounter) {
	sc = &SpeedCounter{s: s}
	go sc.loop_count()
	return
}

func (sc *SpeedCounter) loop_count() {
	for !sc.s.closed {
		num := atomic.SwapInt32(&sc.readcnt, 0)
		sc.readbps = int32(SHRINK_RATE*float64(sc.readbps) + (1-SHRINK_RATE)*float64(num)/float64(SHRINK_TIME))
		num = atomic.SwapInt32(&sc.writebps, 0)
		sc.writebps = int32(SHRINK_RATE*float64(sc.writebps) + (1-SHRINK_RATE)*float64(num)/float64(SHRINK_TIME))
		time.Sleep(SHRINK_TIME * time.Second)
	}
}

func (sc *SpeedCounter) ReadBytes(s int32) int32 {
	return atomic.AddInt32(&sc.readcnt, s)
}

func (sc *SpeedCounter) WriteBytes(s int32) int32 {
	return atomic.AddInt32(&sc.writecnt, s)
}

func (sc *SpeedCounter) GetReadSpeed() int32 {
	return sc.readbps
}

func (sc *SpeedCounter) GetWriteSpeed() int32 {
	return sc.writebps
}
