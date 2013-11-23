package msocks

import (
	"sync"
	"time"
)

type DelayDo struct {
	lock  sync.Mutex
	delay time.Duration
	timer *time.Timer
	cnt   int
	do    func(int) error
}

func NewDelayDo(delay time.Duration, do func(int) error) (d *DelayDo) {
	d = &DelayDo{
		delay: delay,
		do:    do,
	}
	return
}

func (d *DelayDo) Add(n int) {
	d.lock.Lock()
	defer d.lock.Unlock()

	if d.timer == nil {
		d.timer = time.AfterFunc(d.delay, func() {
			d.lock.Lock()
			defer d.lock.Unlock()
			// FIXME: error
			d.do(d.cnt)
			d.timer = nil
			d.cnt = 0
		})
	}
	d.cnt += n
	return
}
