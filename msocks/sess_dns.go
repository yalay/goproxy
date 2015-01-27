package msocks

import (
	"net"
	"time"

	"github.com/miekg/dns"
	"github.com/shell909090/goproxy/sutils"
)

func MakeDnsFrame(host string, t uint16, streamid uint16) (req *dns.Msg, f Frame, err error) {
	log.Debug("make a dns query for %s.", host)

	req = new(dns.Msg)
	req.Id = dns.Id()
	req.SetQuestion(dns.Fqdn(host), t)
	req.RecursionDesired = true

	b, err := req.Pack()
	if err != nil {
		return
	}

	f = NewFrameDns(streamid, b)
	return
}

func DebugDNS(r *dns.Msg, name string) {
	straddr := ""
	for _, a := range r.Answer {
		switch ta := a.(type) {
		case *dns.A:
			straddr += ta.A.String() + ","
		case *dns.AAAA:
			straddr += ta.AAAA.String() + ","
		}
	}
	log.Info("dns result for %s is %s.", name, straddr)
	return
}

func ParseDnsFrame(f Frame, req *dns.Msg) (addrs []net.IP, err error) {
	ft, ok := f.(*FrameDns)
	if !ok {
		return nil, ErrDnsMsgIllegal
	}

	res := new(dns.Msg)
	err = res.Unpack(ft.Data)
	if err != nil || !res.Response || res.Id != req.Id {
		return nil, ErrDnsMsgIllegal
	}

	if DEBUGDNS {
		DebugDNS(res, req.Question[0].Name)
	}
	for _, a := range res.Answer {
		switch ta := a.(type) {
		case *dns.A:
			addrs = append(addrs, ta.A)
		case *dns.AAAA:
			addrs = append(addrs, ta.AAAA)
		}
	}
	return
}

func (s *Session) LookupIP(host string) (addrs []net.IP, err error) {
	ip := net.ParseIP(host)
	if ip != nil {
		return []net.IP{ip}, nil
	}

	cfs := CreateChanFrameSender(0)
	streamid, err := s.PutIntoNextId(&cfs)
	if err != nil {
		return
	}
	defer func() {
		err := s.RemovePort(streamid)
		if err != nil {
			log.Error("%s", err.Error())
		}
	}()

	req, freq, err := MakeDnsFrame(host, dns.TypeA, streamid)
	if err != nil {
		return
	}

	err = s.SendFrame(freq)
	if err != nil {
		return
	}

	fres, err := cfs.RecvWithTimeout(DNS_TIMEOUT * time.Second)
	if err != nil {
		return
	}

	addrs, err = ParseDnsFrame(fres, req)
	return
}

func (s *Session) on_dns(ft *FrameDns) (err error) {
	req := new(dns.Msg)
	err = req.Unpack(ft.Data)
	if err != nil {
		return ErrDnsMsgIllegal
	}

	if req.Response {
		// ignore send fail, maybe just timeout.
		// should I log this ?
		s.sendFrameInChan(ft)
		return
	}

	log.Info("dns query for %s.", req.Question[0].Name)

	d, ok := sutils.DefaultLookuper.(*sutils.DnsLookup)
	if !ok {
		return ErrNoDnsServer
	}
	res, err := d.Exchange(req)
	if err != nil {
		return
	}

	if DEBUGDNS {
		DebugDNS(res, req.Question[0].Name)
	}

	// send response back from streamid
	b, err := res.Pack()
	if err != nil {
		return ErrDnsMsgIllegal
	}

	fr := NewFrameDns(ft.GetStreamid(), b)
	err = s.SendFrame(fr)
	return
}
