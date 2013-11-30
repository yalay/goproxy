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

func (d *DelayDo) Add() {
	d.lock.Lock()
	defer d.lock.Unlock()

	d.cnt += 1
	if d.cnt >= WIN_SIZE {
		d.do(d.cnt)
		if d.timer != nil {
			d.timer.Stop()
			d.timer = nil
		}
		d.cnt = 0
	}

	if d.cnt != 0 && d.timer == nil {
		d.timer = time.AfterFunc(d.delay, func() {
			d.lock.Lock()
			defer d.lock.Unlock()
			if d.cnt > 0 {
				d.do(d.cnt)
			}
			d.timer = nil
			d.cnt = 0
		})
	}
	return
}
