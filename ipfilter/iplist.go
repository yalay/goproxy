package ipfilter

import (
	"bufio"
	"compress/gzip"
	"encoding/binary"
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
	rest []*net.IPNet
	idx1 map[byte][]*net.IPNet
	idx2 map[uint16][]*net.IPNet
}

func ParseLine(line string) (ipnet *net.IPNet, err error) {
	_, ipnet, err = net.ParseCIDR(line)
	if err == nil {
		return
	}
	err = nil

	addrs := strings.Split(line, " ")

	ip := net.ParseIP(addrs[0])
	if x := ip.To4(); x != nil {
		ip = x
	}

	mask := net.ParseIP(addrs[1])
	if x := mask.To4(); x != nil {
		mask = x
	}

	ipnet = &net.IPNet{IP: ip, Mask: net.IPMask(mask)}
	return
}

func ReadIPList(f io.Reader) (filter *IPFilter, err error) {
	reader := bufio.NewReader(f)
	filter = &IPFilter{
		idx1: make(map[byte][]*net.IPNet),
		idx2: make(map[uint16][]*net.IPNet),
	}
	counter := 0

	var ipnet *net.IPNet
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
		line = strings.Trim(line, "\r\n ")

		ipnet, err = ParseLine(line)
		if err != nil {
			log.Error("%s", err)
			return nil, err
		}

		ones, _ := ipnet.Mask.Size()
		switch {
		case ones < 8:
			filter.rest = append(filter.rest, ipnet)
		case ones >= 8 && ones < 16:
			prefix := ipnet.IP[0]
			filter.idx1[prefix] = append(filter.idx1[prefix], ipnet)
		default:
			prefix := binary.BigEndian.Uint16(ipnet.IP[:2])
			filter.idx2[prefix] = append(filter.idx2[prefix], ipnet)
		}
		counter++
	}

	log.Info("blacklist loaded %d record(s), %d index1, %d index2 and %d no indexed.",
		counter, len(filter.idx1), len(filter.idx2), len(filter.rest))
	return
}

func ListConatins(iplist []*net.IPNet, ip net.IP) bool {
	for _, ipnet := range iplist {
		if ipnet.Contains(ip) {
			log.Debug("%s matched %s.", ip.String(), ipnet.String())
			return true
		}
	}
	return false
}

func (f IPFilter) Contain(ip net.IP) bool {
	if x := ip.To4(); x != nil {
		ip = x
	}

	prefix2 := binary.BigEndian.Uint16(ip[:2])
	if iplist, ok := f.idx2[prefix2]; ok {
		if ListConatins(iplist, ip) {
			return true
		}
	}

	prefix1 := ip[0]
	if iplist, ok := f.idx1[prefix1]; ok {
		if ListConatins(iplist, ip) {
			return true
		}
	}

	if ListConatins(f.rest, ip) {
		return true
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
