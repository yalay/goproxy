package ipfilter

import (
	"bufio"
	"compress/gzip"
	"errors"
	"github.com/op/go-logging"
	"github.com/shell909090/goproxy/sutils"
	"io"
	"math/rand"
	"net"
	"os"
	"strings"
)

var log = logging.MustGetLogger("")

var ErrDNSNotFound = errors.New("dns not found")

type IPList []net.IPNet

func ReadIPList(f io.Reader) (iplist IPList, err error) {
	reader := bufio.NewReader(f)

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
		iplist = append(iplist, ipnet)
	}

	log.Info("blacklist loaded %d record(s).", len(iplist))
	return
}

func ReadIPListFile(filename string) (iplist IPList, err error) {
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

// FIXME: can be better?
func (iplist IPList) Contain(ip net.IP) bool {
	for _, ipnet := range iplist {
		if ipnet.Contains(ip) {
			log.Debug("%s matched %s", ipnet, ip)
			return true
		}
	}
	log.Debug("%s not matched.", ip)
	return false
}

type FilteredDialer struct {
	sutils.Dialer
	dialer   sutils.Dialer
	lookuper sutils.Lookuper
	iplist   IPList
}

func NewFilteredDialer(dialer sutils.Dialer, dialer2 sutils.Dialer,
	lookuper sutils.Lookuper, filename string) (fd *FilteredDialer, err error) {

	fd = &FilteredDialer{
		Dialer:   dialer,
		dialer:   dialer2,
		lookuper: CreateDNSCache(lookuper),
	}

	if filename != "" {
		fd.iplist, err = ReadIPListFile(filename)
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
	if err != nil || n == 0 {
		return nil
	}
	return addrs[rand.Intn(n)]
}

func (fd *FilteredDialer) Dial(network, address string) (conn net.Conn, err error) {
	log.Info("address: %s", address)
	if fd.iplist == nil {
		return fd.Dialer.Dial(network, address)
	}

	idx := strings.LastIndex(address, ":")
	if idx == -1 {
		err = errors.New("invaild address")
		log.Error("%s", err)
		return
	}
	hostname := address[:idx]

	addr := Getaddr(fd.lookuper, hostname)
	if addr == nil {
		return nil, ErrDNSNotFound
	}

	if fd.iplist.Contain(addr) {
		return fd.dialer.Dial(network, address)
	}

	return fd.Dialer.Dial(network, address)
}
