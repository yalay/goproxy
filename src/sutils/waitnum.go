package sutils

import (
	"sync"
)

// 先不考虑数量上越界，即比buf更多的ack发送过来。

type WaitNum struct {
	l     sync.Mutex
	num   uint32
	ucond *sync.Cond
	// dcond sync.Cond
}

func NewWaitNum(num uint32) (wn *WaitNum) {
	wn = &WaitNum{num: num}
	wn.ucond = sync.NewCond(&wn.l)
	// wn.dcond = sync.NewCond(wn.l)
	return
}

func (wn *WaitNum) Number() (n uint32) {
	return wn.num
}

func (wn *WaitNum) Acquire(num uint32) (n uint32) {
	wn.l.Lock()
	defer wn.l.Unlock()

	for wn.num == 0 {
		wn.ucond.Wait()
	}

	n = num
	if n > wn.num {
		n = wn.num
	}
	wn.num -= n

	if wn.num > 0 {
		wn.ucond.Signal()
	}

	return
}

func (wn *WaitNum) Release(num uint32) (n uint32) {
	wn.l.Lock()
	defer wn.l.Unlock()

	n = num
	wn.num += n

	if wn.num > 0 {
		wn.ucond.Signal()
	}

	return
}
