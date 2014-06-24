package ipfilter

import (
	"github.com/shell909090/goproxy/sutils"
	"net"
	"sync"
	"time"
)

var cacheMaxAge = 300 * time.Second

type IPEntry struct {
	expire time.Time
	addrs  []net.IP
}

type DNSCache struct {
	mu       sync.Mutex
	cache    map[string]*IPEntry
	lookuper sutils.Lookuper
}

func CreateDNSCache(lookuper sutils.Lookuper) (dc *DNSCache) {
	dc = &DNSCache{
		cache:    make(map[string]*IPEntry, 0),
		lookuper: lookuper,
	}
	return
}

func (dc DNSCache) free() {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	var dellist []string
	n := time.Now()
	for k, v := range dc.cache {
		if n.After(v.expire) {
			dellist = append(dellist, k)
		}
	}
	for _, k := range dellist {
		delete(dc.cache, k)
	}
	log.Info("%d dnscache records freed.", len(dellist))
	return
}

func (dc DNSCache) LookupIP(hostname string) (addrs []net.IP, err error) {
	ipe, ok := dc.cache[hostname]
	if ok {
		log.Debug("hostname %s cached.", hostname)
		return ipe.addrs, nil
	}

	addrs, err = dc.lookuper.LookupIP(hostname)
	if err != nil {
		return
	}

	dc.mu.Lock()
	defer dc.mu.Unlock()
	dc.cache[hostname] = &IPEntry{
		expire: time.Now().Add(cacheMaxAge),
		addrs:  addrs,
	}

	if len(dc.cache) > 32 {
		go dc.free()
	}

	return
}
