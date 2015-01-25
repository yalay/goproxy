package msocks

import (
	"sync/atomic"
	"time"
)

type SpeedCounter struct {
	s        *Session
	readcnt  int64
	readbps  int64
	writecnt int64
	writebps int64
}

func NewSpeedCounter(s *Session) (sc *SpeedCounter) {
	sc = &SpeedCounter{s: s}
	go sc.loop_count()
	return
}

func shrink_count(cnt *int64, bps *int64) bool {
	num := float64(atomic.SwapInt64(cnt, 0)) * (1 - SHRINK_RATE)
	for i := 0; i < 10; i++ {
		old := atomic.LoadInt64(bps)
		new := int64(float64(old)*SHRINK_RATE + num)
		if atomic.CompareAndSwapInt64(bps, old, new) {
			return true
		}
	}
	return false
}

func (sc *SpeedCounter) loop_count() {
	for !sc.s.closed {
		if !shrink_count(&sc.readcnt, &sc.readbps) {
			log.Error("shrink counter read failed")
		}
		if !shrink_count(&sc.writecnt, &sc.writebps) {
			log.Error("shrink counter write failed")
		}
		time.Sleep(SHRINK_TIME * time.Millisecond)
	}
}

func (sc *SpeedCounter) ReadBytes(s int64) (now int64) {
	return atomic.AddInt64(&sc.readcnt, s)
}

func (sc *SpeedCounter) WriteBytes(s int64) (now int64) {
	return atomic.AddInt64(&sc.writecnt, s)
}

func (sc *SpeedCounter) GetReadSpeed() int64 {
	return atomic.LoadInt64(&sc.readbps)
}

func (sc *SpeedCounter) GetWriteSpeed() int64 {
	return atomic.LoadInt64(&sc.writebps)
}
