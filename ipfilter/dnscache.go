package ipfilter

import (
	"net"
	"time"
)

type IPEntry struct {
	t  time.Time
	ip net.IP
}

type DNSCache struct {
	cache       map[string]*IPEntry
	lookup_func func(string) ([]net.IP, error)
}

func (dc DNSCache) free() {
	var dellist []string
	n := time.Now()
	for k, v := range dc.cache {
		if n.Sub(v.t).Seconds() > 300 {
			dellist = append(dellist, k)
		}
	}
	for _, k := range dellist {
		delete(dc.cache, k)
	}
	log.Info("%d dnscache records freed.", len(dellist))
	return
}

func (dc DNSCache) Lookup(hostname string) (ip net.IP, err error) {
	ipe, ok := dc.cache[hostname]
	if ok {
		log.Debug("hostname %s cached.", hostname)
		return ipe.ip, nil
	}

	addrs, err := dc.lookup_func(hostname)
	if err != nil {
		return
	}

	ip = addrs[0]

	if len(dc.cache) > 256 {
		dc.free()
	}
	dc.cache[hostname] = &IPEntry{
		t:  time.Now(),
		ip: ip,
	}

	return
}

func (dc DNSCache) ParseIP(hostname string) (addr net.IP, err error) {
	addr = net.ParseIP(hostname)
	if addr != nil {
		return
	}
	addr, err = dc.Lookup(hostname)
	return
}
