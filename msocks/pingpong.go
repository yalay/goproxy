package msocks

import (
	"io"
	"math/rand"
	"time"
)

type PingPong struct {
	ch       chan int
	cnt      int
	lastping time.Time
	w        io.WriteCloser
}

func NewPingPong(w io.WriteCloser) (p *PingPong) {
	p = &PingPong{
		ch:       make(chan int, 3),
		w:        w,
		lastping: time.Now(),
	}
	go p.Run()
	return
}

func (p *PingPong) Reset() {
	p.cnt = 0
}

func (p *PingPong) Ping() bool {
	log.Debug("ping: %p.", p.w)
	select {
	case p.ch <- 1:
	default:
		return false
	}
	p.lastping = time.Now()
	return true
}

func (p *PingPong) GetLastPing() (d time.Duration) {
	return time.Now().Sub(p.lastping)
}

func (p *PingPong) Pong() {
	log.Debug("pong: %p.", p.w)
	// use Write without trigger the reset
	b := NewFrameNoParam(MSG_PING, 0)
	_, err := p.w.Write(b)
	if err != nil {
		log.Error("%s", err)
	}
}

func (p *PingPong) Run() {
	for {
		timeout := time.After(TIMEOUT_COUNT * PINGTIME)
		select {
		case <-timeout:
			log.Warning("pingpong timeout: %p.", p.w)
			p.w.Close()
			return
		case <-p.ch:
			p.cnt += 1
			if p.cnt >= GAMEOVER_COUNT {
				log.Warning("pingpong gameover.")
				p.w.Close()
				return
			}

			pingtime := PINGTIME + time.Duration(rand.Intn(2*PINGRANDOM)-PINGRANDOM)*time.Millisecond
			log.Debug("pingtime: %d", pingtime/time.Millisecond)
			time.Sleep(pingtime)
			p.Pong()
		}
	}
}
