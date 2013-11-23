package msocks

type Window struct {
	ch_win chan uint32
}

func NewWindow(init uint32) (w *Window) {
	w = &Window{
		ch_win: make(chan uint32, 10),
	}
	w.ch_win <- init
	return
}

func (w *Window) Close() (err error) {
	w.ch_win <- 0
	return
}

func (w *Window) acquire(num uint32) (n uint32) {
	n = <-w.ch_win
	switch {
	case n == 0: // weak up next
		w.ch_win <- 0
	case n > num:
		w.ch_win <- (n - num)
		n = num
	}
	return
}

func (w *Window) release(num uint32) (n uint32) {
	n = num
	for {
		select {
		case m := <-w.ch_win:
			n += m
		default:
			w.ch_win <- n
			return
		}
	}
}
