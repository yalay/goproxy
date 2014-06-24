// Copyright 2012 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dns

import "net"

var (
	LookupIP = lookupIPMerge
	lookupIP = goLookupIP
)

var lookupGroup singleflight

// lookupIPMerge wraps lookupIP, but makes sure that for any given
// host, only one lookup is in-flight at a time. The returned memory
// is always owned by the caller.
func lookupIPMerge(host string) (addrs []net.IP, err error) {
	addrsi, err, shared := lookupGroup.Do(host, func() (interface{}, error) {
		return lookupIP(host)
	})
	if err != nil {
		return nil, err
	}
	addrs = addrsi.([]net.IP)
	if shared {
		clone := make([]net.IP, len(addrs))
		copy(clone, addrs)
		addrs = clone
	}
	return addrs, nil
}

type Lookuper struct {
}

func (l *Lookuper) LookupIP(host string) (addrs []net.IP, err error) {
	return LookupIP(host)
}

var DefaultLookuper = &Lookuper{}
