package ipfilter

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"logging"
	"net"
	"os"
	"qsocks"
	"strings"
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

func (iplist IPList) Dial(hostname string, port uint16, white bool, dialer *qsocks.Dialer) (conn net.Conn, err error) {
	if iplist == nil {
		return net.Dial("tcp", fmt.Sprintf("%s:%d", hostname, port))
	}

	addr, err := DefaultDNSCache.ParseIP(hostname)
	if err != nil {
		return
	}

	if white {
		if !iplist.Contain(addr) {
			logging.Debug("ip %s in list, mode white.", addr)
			return net.Dial("tcp", fmt.Sprintf("%s:%d", hostname, port))
		}
	} else {
		if iplist.Contain(addr) {
			logging.Debug("ip %s in list, mode black.", addr)
			return net.Dial("tcp", fmt.Sprintf("%s:%d", hostname, port))
		}
	}

	return dialer.Dial(hostname, port)
}
