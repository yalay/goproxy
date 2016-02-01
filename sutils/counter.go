package sutils

import (
	"sync"
	"sync/atomic"
)

const (
	UPDATE_INTERVAL = 10
)

type Updater interface {
	Update()
}

var (
	mu sync.Mutex
)

type Counter struct {
	cnt uint32
	spd uint32
}

func NewCounter() (c *Counter) {
	return &Counter{}
}

func (c *Counter) Update() {
	c.spd = atomic.SwapUint32(&c.cnt, 0) / UPDATE_INTERVAL
}

func (c *Counter) Add(s uint32) uint32 {
	return atomic.AddUint32(&c.cnt, s)
}

func (c *Counter) Get() uint32 {
	return c.spd
}
