package ipfilter

import (
	"bufio"
	"compress/gzip"
	"errors"
	"io"
	"math/rand"
	"net"
	"os"
	"strings"

	logging "github.com/op/go-logging"
	"github.com/shell909090/goproxy/sutils"
)

var log = logging.MustGetLogger("")

var ErrDNSNotFound = errors.New("dns not found")

type IPFilter struct {
	rest []net.IPNet
	idx  map[byte][]net.IPNet
}

func ReadIPList(f io.Reader) (filter *IPFilter, err error) {
	reader := bufio.NewReader(f)
	filter = &IPFilter{idx: make(map[byte][]net.IPNet)}
	counter := 0

QUIT:
	for {
		line, err := reader.ReadString('\n')
		switch err {
		case io.EOF:
			if len(line) == 0 {
				break QUIT
			}
		case nil:
		default:
			log.Error("%s", err)
			return nil, err
		}
		addrs := strings.Split(strings.Trim(line, "\r\n "), " ")
		ipnet := net.IPNet{
			IP:   net.ParseIP(addrs[0]),
			Mask: net.IPMask(net.ParseIP(addrs[1])),
		}

		if ipnet.Mask[0] == 255 {
			prefix := ipnet.IP[0]
			filter.idx[prefix] = append(filter.idx[prefix], ipnet)
		} else {
			filter.rest = append(filter.rest, ipnet)
		}
		counter++
	}

	log.Info("blacklist loaded %d record(s), %d indexed and %d no indexed.",
		counter, len(filter.idx), len(filter.rest))
	return
}

func (f IPFilter) Contain(ip net.IP) bool {
	prefix := ip[0]
	if iplist, ok := f.idx[prefix]; ok {
		for _, ipnet := range iplist {
			if ipnet.Contains(ip) {
				log.Debug("%s matched %s.", ip.String(), ipnet.String())
				return true
			}
		}
	}

	for _, ipnet := range f.rest {
		if ipnet.Contains(ip) {
			log.Debug("%s matched %s.", ip.String(), ipnet.String())
			return true
		}
	}

	log.Debug("%s not match anything.", ip.String())
	return false
}

func ReadIPListFile(filename string) (filter *IPFilter, err error) {
	log.Info("load iplist from file %s.", filename)

	var f io.ReadCloser
	f, err = os.Open(filename)
	if err != nil {
		log.Error("%s", err)
		return
	}
	defer f.Close()

	if strings.HasSuffix(filename, ".gz") {
		f, err = gzip.NewReader(f)
		if err != nil {
			log.Error("%s", err)
			return
		}
	}

	return ReadIPList(f)
}

// type IPList []net.IPNet

// func ReadIPList(f io.Reader) (iplist IPList, err error) {
// 	reader := bufio.NewReader(f)

// QUIT:
// 	for {
// 		line, err := reader.ReadString('\n')
// 		switch err {
// 		case io.EOF:
// 			if len(line) == 0 {
// 				break QUIT
// 			}
// 		case nil:
// 		default:
// 			log.Error("%s", err)
// 			return nil, err
// 		}
// 		addrs := strings.Split(strings.Trim(line, "\r\n "), " ")
// 		ipnet := net.IPNet{
// 			IP:   net.ParseIP(addrs[0]),
// 			Mask: net.IPMask(net.ParseIP(addrs[1])),
// 		}
// 		iplist = append(iplist, ipnet)
// 	}

// 	log.Info("blacklist loaded %d record(s).", len(iplist))
// 	return
// }

// func ReadIPListFile(filename string) (iplist IPList, err error) {
// 	log.Info("load iplist from file %s.", filename)

// 	var f io.ReadCloser
// 	f, err = os.Open(filename)
// 	if err != nil {
// 		log.Error("%s", err)
// 		return
// 	}
// 	defer f.Close()

// 	if strings.HasSuffix(filename, ".gz") {
// 		f, err = gzip.NewReader(f)
// 		if err != nil {
// 			log.Error("%s", err)
// 			return
// 		}
// 	}

// 	return ReadIPList(f)
// }

// // FIXME: can be better?
// func (iplist IPList) Contain(ip net.IP) bool {
// 	for _, ipnet := range iplist {
// 		if ipnet.Contains(ip) {
// 			log.Debug("%s matched %s.", ip.String(), ipnet.String())
// 			return true
// 		}
// 	}
// 	log.Debug("%s not match anything.", ip.String())
// 	return false
// }

type FilteredDialer struct {
	sutils.Dialer
	dialer   sutils.Dialer
	lookuper sutils.Lookuper
	filter   *IPFilter
}

func NewFilteredDialer(dialer sutils.Dialer, dialer2 sutils.Dialer,
	filename string) (fd *FilteredDialer, err error) {

	fd = &FilteredDialer{
		Dialer:   dialer,
		dialer:   dialer2,
		lookuper: CreateDNSCache(),
	}

	if filename != "" {
		fd.filter, err = ReadIPListFile(filename)
		return
	}
	return
}

func Getaddr(lookuper sutils.Lookuper, hostname string) (ip net.IP) {
	ip = net.ParseIP(hostname)
	if ip != nil {
		return
	}
	addrs, err := lookuper.LookupIP(hostname)

	n := len(addrs)
	if err != nil {
		return nil
	}
	switch n {
	case 0:
		return nil
	case 1:
		return addrs[0]
	default:
		return addrs[rand.Intn(n)]
	}
}

func (fd *FilteredDialer) Dial(network, address string) (conn net.Conn, err error) {
	log.Info("filter dial: %s", address)
	if fd.filter == nil {
		return fd.Dialer.Dial(network, address)
	}

	hostname, _, err := net.SplitHostPort(address)
	if err != nil {
		log.Error("%s", err.Error())
		return
	}

	addr := Getaddr(fd.lookuper, hostname)
	if addr == nil {
		return nil, ErrDNSNotFound
	}

	if fd.filter.Contain(addr) {
		return fd.dialer.Dial(network, address)
	}

	return fd.Dialer.Dial(network, address)
}
