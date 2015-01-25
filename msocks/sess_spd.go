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

func shrink_count(cnt *int64, bps *int64) {
	num := atomic.SwapInt64(cnt, 0)
	old := atomic.LoadInt64(bps)
	new := SHRINK_RATE*float64(old) + (1-SHRINK_RATE)*float64(num)/float64(SHRINK_TIME)
	atomic.StoreInt64(bps, int64(new))
}

func (sc *SpeedCounter) loop_count() {
	for !sc.s.closed {
		shrink_count(&sc.readcnt, &sc.readbps)
		shrink_count(&sc.writecnt, &sc.writebps)
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
