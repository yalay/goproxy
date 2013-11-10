package ipfilter

import (
	"dns"
	"logging"
	"net"
	"time"
)

type IPEntry struct {
	t  time.Time
	ip net.IP
}

type DNSCache map[string]*IPEntry

func (dc DNSCache) free() {
	var dellist []string
	n := time.Now()
	for k, v := range dc {
		if n.Sub(v.t).Seconds() > 300 {
			dellist = append(dellist, k)
		}
	}
	for _, k := range dellist {
		delete(dc, k)
	}
	logging.Infof("%d dnscache records freed.", len(dellist))
	return
}

func (dc DNSCache) Lookup(hostname string) (ip net.IP, err error) {
	ipe, ok := dc[hostname]
	if ok {
		logging.Debugf("hostname %s cached.", hostname)
		return ipe.ip, nil
	}

	addrs, err := dns.LookupIP(hostname)
	if err != nil {
		return
	}

	ip = addrs[0]

	if len(dc) > 256 {
		dc.free()
	}
	dc[hostname] = &IPEntry{
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

var DefaultDNSCache = make(DNSCache, 0)
