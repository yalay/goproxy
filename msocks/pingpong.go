package msocks

import (
	"math/rand"
	"sync/atomic"
	"time"
)

type PingPong struct {
	ch_ping  chan int
	cnt      int32
	lastping time.Time
	sender   FrameSender
	gameover bool
}

func NewPingPong(sender FrameSender) (p *PingPong) {
	p = &PingPong{
		ch_ping:  make(chan int, 1),
		cnt:      0,
		lastping: time.Now(),
		sender:   sender,
		gameover: false,
	}
	go p.loop()
	return
}

func (p *PingPong) IsGameOver() bool {
	return p.gameover
}

func (p *PingPong) Reset() {
	atomic.StoreInt32(&p.cnt, 0)
}

func (p *PingPong) GetLastPing() (d time.Duration) {
	return time.Now().Sub(p.lastping)
}

func (p *PingPong) ping() {
	log.Debug("ping: %p.", p.sender)
	p.lastping = time.Now()
	select {
	case p.ch_ping <- 1:
	default:
	}
}

func (p *PingPong) pong() {
	log.Debug("pong: %p.", p.sender)
	err := p.sender.SendFrame(frame_ping)
	if err != nil {
		log.Error("%s", err)
	}
}

func (p *PingPong) addCount() int32 {
	return atomic.AddInt32(&p.cnt, 1)
}

func (p *PingPong) loop() {
	for {
		select {
		case <-time.After(TIMEOUT_COUNT * PINGTIME * time.Millisecond):
			log.Warning("pingpong timeout: %p.", p.sender)
			p.sender.CloseFrame()
			return
		case <-p.ch_ping:
		}

		if p.addCount() >= GAMEOVER_COUNT {
			log.Warning("pingpong gameover.")
			p.gameover = true
			p.sender.CloseFrame()
			return
		}

		pingtime := PINGTIME + rand.Intn(2*PINGRANDOM) - PINGRANDOM
		log.Debug("pingtime: %d", pingtime)
		time.Sleep(time.Duration(pingtime) * time.Millisecond)
		p.pong()
	}
}
