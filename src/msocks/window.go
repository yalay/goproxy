package msocks

import (
	"sync"
)

type Window struct {
	c      *sync.Cond
	mu     *sync.Mutex
	closed bool
	win    uint32
}

func NewWindow(init uint32) (w *Window) {
	var mu sync.Mutex
	w = &Window{
		c:   sync.NewCond(&mu),
		mu:  &mu,
		win: init,
	}
	return
}

func (w *Window) Close() (err error) {
	w.closed = true
	w.c.Broadcast()
	return
}

func (w *Window) Acquire(num uint32) (n uint32) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for {
		switch {
		case w.closed:
			break
		case w.win == 0:
			w.c.Wait()
			continue
		case w.win < num:
			n = w.win
		case w.win > num:
			n = num
		}
		w.win -= n
		return
	}
	return
}

func (w *Window) Release(num uint32) (n uint32) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.win += num
	n = num
	w.c.Broadcast()
	return
}
