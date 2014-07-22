package ipfilter

import (
	"errors"
	"github.com/shell909090/goproxy/sutils"
	"net"
	"sync"
	"time"
)

const maxCache = 512

var errType = errors.New("type error")

type DNSCache struct {
	mu       sync.Mutex
	cache    *Cache
	lookuper sutils.Lookuper
}

func CreateDNSCache(lookuper sutils.Lookuper) (dc *DNSCache) {
	dc = &DNSCache{
		cache:    New(maxCache),
		lookuper: lookuper,
	}
	return
}

func (dc DNSCache) LookupIP(hostname string) (addrs []net.IP, err error) {
	value, ok := dc.cache.Get(hostname)
	if ok {
		addrs, ok = value.([]net.IP)
		if !ok {
			err = errType
		}
		log.Debug("hostname %s cached.", hostname)
		return
	}

	addrs, err = dc.lookuper.LookupIP(hostname)
	if err != nil {
		return
	}

	dc.mu.Lock()
	defer dc.mu.Unlock()

	dc.cache.Add(hostname, addrs)
	return
}
