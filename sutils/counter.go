package sutils

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

var (
	UPDATE_INTERVAL    uint32 = 10
	ErrUpdaterNotFound        = errors.New("updater not found")
)

type Updater interface {
	Update()
}

var (
	mu_set     sync.Mutex
	update_set map[Updater]struct{}
)

func init() {
	update_set = make(map[Updater]struct{}, 0)
	go func() {
		for {
			time.Sleep(time.Duration(UPDATE_INTERVAL) * time.Second)
			update_all()
		}
	}()
}

func update_all() {
	mu_set.Lock()
	defer mu_set.Unlock()

	for u, _ := range update_set {
		u.Update()
	}
}

func update_add(u Updater) {
	mu_set.Lock()
	defer mu_set.Unlock()

	update_set[u] = struct{}{}
}

func update_remove(u Updater) (err error) {
	mu_set.Lock()
	defer mu_set.Unlock()

	if _, ok := update_set[u]; !ok {
		return ErrUpdaterNotFound
	}
	delete(update_set, u)
	return
}

type SpeedCounter struct {
	cnt uint32
	Spd uint32
	All uint64
}

func NewSpeedCounter() (sc *SpeedCounter) {
	sc = &SpeedCounter{}
	update_add(sc)
	return sc
}

func (sc *SpeedCounter) Close() error {
	return update_remove(sc)
}

func (sc *SpeedCounter) Update() {
	c := atomic.SwapUint32(&sc.cnt, 0)
	sc.Spd = c / UPDATE_INTERVAL
	sc.All += uint64(c)
}

func (sc *SpeedCounter) Add(s uint32) uint32 {
	return atomic.AddUint32(&sc.cnt, s)
}
