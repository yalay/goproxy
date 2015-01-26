package msocks

import (
	"net"
	"time"

	"github.com/miekg/dns"
	"github.com/shell909090/goproxy/sutils"
)

func MakeDnsFrame(host string, t uint16, streamid uint16) (req *dns.Msg, f Frame, err error) {
	log.Info("make a dns query for %s.", host)

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

	straddr := ""
	for _, a := range res.Answer {
		switch ta := a.(type) {
		case *dns.A:
			addrs = append(addrs, ta.A)
			straddr += ta.A.String() + ","
		case *dns.AAAA:
			addrs = append(addrs, ta.AAAA)
			straddr += ta.AAAA.String() + ","
		}
	}
	log.Info("dns result for %s is %s.", req.Question[0].Name, straddr)
	return
}

func (s *Session) LookupIP(host string) (addrs []net.IP, err error) {
	cfs := CreateChanFrameSender(0)
	streamid, err := s.PutIntoNextId(&cfs)
	if err != nil {
		return
	}
	defer func() {
		err := s.RemovePorts(streamid)
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

func (s *Session) on_dns(ft *FrameDns) bool {
	m := new(dns.Msg)
	err := m.Unpack(ft.Data)
	if err != nil {
		log.Error("%s", ErrDnsMsgIllegal.Error())
		return false
	}

	if m.Response {
		// ignore send fail, maybe just timeout.
		// should I log this ?
		s.sendFrameInChan(ft)
		return true
	}

	log.Info("got a dns query for %s.", m.Question[0].Name)

	d, ok := sutils.DefaultLookuper.(*sutils.DnsLookup)
	if !ok {
		log.Error("got a dns query without a proper dns server")
		return false
	}
	r, err := d.Exchange(m)
	if err != nil {
		log.Error("dns query error: %s", err.Error())
		return true
	}

	straddr := ""
	for _, a := range r.Answer {
		switch ta := a.(type) {
		case *dns.A:
			straddr += ta.A.String() + ","
		case *dns.AAAA:
			straddr += ta.AAAA.String() + ","
		}
	}
	log.Info("dns result for %s is %s.", m.Question[0].Name, straddr)

	// send response back from streamid
	b, err := r.Pack()
	if err != nil {
		log.Error("dns pack failed, how that possible.")
		return false
	}

	fr := NewFrameDns(ft.GetStreamid(), b)
	err = s.SendFrame(fr)
	if err != nil {
		return false
	}
	return true
}
