package ipfilter

import (
	"bufio"
	"compress/gzip"
	"errors"
	"io"
	"logging"
	"msocks"
	"net"
	"os"
	"strings"
	"sutils"
)

type IPList []net.IPNet

var logger logging.Logger

func init() {
	var err error
	logger, err = logging.NewFileLogger("default", -1, "ipfilter")
	if err != nil {
		panic(err)
	}
}

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
			logger.Err(err)
			return nil, err
		}
		addrs := strings.Split(strings.Trim(line, "\r\n "), " ")
		ipnet := net.IPNet{
			IP:   net.ParseIP(addrs[0]),
			Mask: net.IPMask(net.ParseIP(addrs[1])),
		}
		iplist = append(iplist, ipnet)
	}

	logger.Infof("blacklist loaded %d record(s).", len(iplist))
	return
}

func ReadIPListFile(filename string) (iplist IPList, err error) {
	logger.Infof("load iplist from file %s.", filename)

	var f io.ReadCloser
	f, err = os.Open(filename)
	if err != nil {
		logger.Err(err)
		return
	}
	defer f.Close()

	if strings.HasSuffix(filename, ".gz") {
		f, err = gzip.NewReader(f)
		if err != nil {
			logger.Err(err)
			return
		}
	}

	return ReadIPList(f)
}

func (iplist IPList) Contain(ip net.IP) bool {
	for _, ipnet := range iplist {
		if ipnet.Contains(ip) {
			logger.Debugf("%s matched %s", ipnet, ip)
			return true
		}
	}
	logger.Debugf("%s not matched.", ip)
	return false
}

type FilteredDialer struct {
	msocks.Dialer
	dialer sutils.Dialer
	iplist IPList
	dns    *DNSCache
}

func NewFilteredDialer(dialer *msocks.Dialer, dialer2 sutils.Dialer,
	filename string) (fd *FilteredDialer, err error) {
	fd = &FilteredDialer{
		Dialer: *dialer,
		dialer: dialer2,
		dns: &DNSCache{
			cache:       make(map[string]*IPEntry, 0),
			lookup_func: dialer.LookupIP,
		},
	}

	if filename != "" {
		fd.iplist, err = ReadIPListFile(filename)
	}
	return
}

func (fd *FilteredDialer) Dial(network, address string) (conn net.Conn, err error) {
	logger.Infof("address: %s", address)
	if fd.iplist == nil {
		return fd.Dialer.Dial(network, address)
	}

	idx := strings.LastIndex(address, ":")
	if idx == -1 {
		err = errors.New("invaild address")
		logger.Err(err)
		return
	}
	hostname := address[:idx]

	addr, err := fd.dns.ParseIP(hostname)
	if err != nil {
		return
	}

	if fd.iplist.Contain(addr) {
		return fd.dialer.Dial(network, address)
	}

	return fd.Dialer.Dial(network, address)
}
