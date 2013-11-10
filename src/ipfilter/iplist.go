package ipfilter

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"logging"
	"net"
	"os"
	"strings"
	"sutils"
)

type IPList []net.IPNet

func ReadIPList(filename string) (iplist IPList, err error) {
	var f io.ReadCloser
	f, err = os.Open(filename)
	if err != nil {
		return
	}
	defer f.Close()

	if strings.HasSuffix(filename, ".gz") {
		f, err = gzip.NewReader(f)
		if err != nil {
			return
		}
	}

	reader := bufio.NewReader(f)
	for {
		line, err := reader.ReadString('\n')
		switch err {
		case io.EOF:
			if len(line) == 0 {
				return nil, nil
			}
		case nil:
		default:
			return nil, err
		}
		addrs := strings.Split(strings.Trim(line, "\r\n "), " ")
		ipnet := net.IPNet{
			IP:   net.ParseIP(addrs[0]),
			Mask: net.IPMask(net.ParseIP(addrs[1])),
		}
		iplist = append(iplist, ipnet)
	}

	logging.Info("blacklist loaded %d record(s).", len(iplist))
	return
}

func (iplist IPList) Contain(ip net.IP) bool {
	for _, ipnet := range iplist {
		if ipnet.Contains(ip) {
			logging.Debugf("%s matched %s", ipnet, ip)
			return true
		}
	}
	return false
}

type FilteredDialer struct {
	dialer sutils.Dialer
	iplist IPList
	white  bool
}

func NewFilteredDialer(filename string, white bool, dialer sutils.Dialer) (
	fd *FilteredDialer, err error) {
	fd = &FilteredDialer{
		dialer: dialer,
		white:  white,
	}

	fd.iplist, err = ReadIPList(filename)
	return
}

func (fd *FilteredDialer) Dial(hostname string, port uint16) (conn net.Conn, err error) {
	if fd.iplist == nil {
		return net.Dial("tcp", fmt.Sprintf("%s:%d", hostname, port))
	}

	addr, err := DefaultDNSCache.ParseIP(hostname)
	if err != nil {
		return
	}

	if fd.white {
		if !fd.iplist.Contain(addr) {
			logging.Debug("ip %s not in list, mode white.", addr)
			return net.Dial("tcp", fmt.Sprintf("%s:%d", hostname, port))
		}
	} else {
		if fd.iplist.Contain(addr) {
			logging.Debug("ip %s in list, mode black.", addr)
			return net.Dial("tcp", fmt.Sprintf("%s:%d", hostname, port))
		}
	}

	return fd.dialer.Dial(hostname, port)
}
