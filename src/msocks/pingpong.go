package msocks

import (
	"io"
	"math/rand"
	"time"
)

type PingPong struct {
	ch       chan int
	cnt      int
	pingtime time.Duration
	w        io.WriteCloser
}

func NewPingPong(pingtime time.Duration, w io.WriteCloser) (p *PingPong) {
	p = &PingPong{
		ch:       make(chan int, 3),
		pingtime: pingtime,
		w:        w,
	}
	go p.Run()
	return
}

func (p *PingPong) Reset() {
	p.cnt = 0
}

func (p *PingPong) Ping() bool {
	logger.Debugf("ping: %p.", p.w)
	select {
	case p.ch <- 1:
	default:
		return false
	}
	return true
}

func (p *PingPong) Pong() {
	logger.Debugf("pong: %p.", p.w)
	// use Write without trigger the reset
	b := NewFrameNoParam(MSG_PING, 0)
	_, err := p.w.Write(b)
	if err != nil {
		logger.Err(err)
	}
}

func (p *PingPong) Run() {
	for {
		timeout := time.After(6 * p.pingtime)
		select {
		case <-timeout:
			logger.Warningf("pingpong timeout: %p.", p.w)
			p.w.Close()
			return
		case <-p.ch:
			p.cnt += 1
			if p.cnt >= 20 {
				logger.Warning("pingpong gameover.")
				p.w.Close()
				return
			}

			pingtime := p.pingtime + time.Duration(rand.Intn(10)-5)*time.Second
			logger.Debugf("pingtime: %d", pingtime/time.Second)
			time.Sleep(pingtime)
			p.Pong()
		}
	}
}
