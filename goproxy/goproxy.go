package main

import (
	"flag"
	"github.com/shell909090/goproxy/cryptconn"
	"github.com/shell909090/goproxy/dns"
	"github.com/shell909090/goproxy/ipfilter"
	"github.com/shell909090/goproxy/logging"
	"github.com/shell909090/goproxy/msocks"
	"github.com/shell909090/goproxy/socks"
	"github.com/shell909090/goproxy/sutils"
	"net"
	"net/http"
)

var cipher string
var keyfile string
var listenaddr string
var username string
var password string
var passfile string
var blackfile string
var runmode string
var logger logging.Logger

func init() {
	var logfile string
	var loglevel string

	flag.StringVar(&runmode, "mode", "http", "server/socks5/http mode")
	flag.StringVar(&cipher, "cipher", "aes", "aes/des/tripledes/rc4")
	flag.StringVar(&keyfile, "keyfile", "", "key and iv file")
	flag.StringVar(&listenaddr, "listen", ":5233", "listen address")
	flag.StringVar(&username, "username", "", "username for connect")
	flag.StringVar(&password, "password", "", "password for connect")
	flag.StringVar(&passfile, "passfile", "", "password file")
	flag.StringVar(&blackfile, "black", "", "blacklist file")

	flag.StringVar(&logfile, "logfile", "", "log file")
	flag.StringVar(&loglevel, "loglevel", "WARNING", "log level")
	flag.Parse()

	lv, err := logging.GetLevelByName(loglevel)
	if err != nil {
		panic(err.Error())
	}
	err = logging.SetupDefault(logfile, lv)
	if err != nil {
		panic(err.Error())
	}

	logger, err = logging.NewFileLogger("default", -1, "goproxy")
	if err != nil {
		panic(err)
	}

}

func run_server() {
	tcpAddr, err := net.ResolveTCPAddr("tcp", listenaddr)
	if err != nil {
		logger.Err(err)
		return
	}

	var listener net.Listener
	listener, err = net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		logger.Err(err)
		return
	}

	if len(keyfile) > 0 {
		listener, err = cryptconn.NewListener(listener, cipher, keyfile)
		if err != nil {
			logger.Err("crypto not work, cipher or keyfile wrong.")
			return
		}
	} else {
		logger.Warning("no vaild keyfile.")
	}

	qs, err := msocks.NewService(passfile, sutils.DefaultTcpDialer)
	if err != nil {
		return
	}

	err = qs.Serve(listener)
	if err != nil {
		logger.Err(err)
	}
}

func get_dialer(serveraddr string) (dialer sutils.Dialer, ndialer *msocks.Dialer, err error) {
	err = dns.LoadConfig("resolv.conf")
	if err != nil {
		err = dns.LoadConfig("/etc/goproxy/resolv.conf")
		if err != nil {
			return
		}
	}

	dialer = sutils.DefaultTcpDialer

	if len(keyfile) > 0 {
		dialer, err = cryptconn.NewDialer(dialer, cipher, keyfile)
		if err != nil {
			logger.Err("crypto not work, cipher or keyfile wrong.")
			return
		}
	} else {
		logger.Warning("no vaild keyfile.")
	}

	ndialer, err = msocks.NewDialer(dialer, serveraddr, username, password)
	if err != nil {
		return
	}
	dialer = ndialer

	if blackfile != "" {
		dialer, err = ipfilter.NewFilteredDialer(
			dialer, sutils.DefaultTcpDialer, blackfile)
		if err != nil {
			return
		}
	}

	return
}

func run_client() {
	if len(flag.Args()) < 1 {
		logger.Err("args not enough")
		return
	}
	serveraddr := flag.Args()[0]

	dialer, _, err := get_dialer(serveraddr)
	if err != nil {
		return
	}

	tcpAddr, err := net.ResolveTCPAddr("tcp", listenaddr)
	if err != nil {
		logger.Err(err)
		return
	}

	listener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		logger.Err(err)
		return
	}

	ss := socks.NewService(dialer)
	err = ss.Serve(listener)
	if err != nil {
		logger.Err(err)
	}
}

func run_httproxy() {
	if len(flag.Args()) < 1 {
		logger.Err("args not enough")
		return
	}
	serveraddr := flag.Args()[0]

	dialer, ndialer, err := get_dialer(serveraddr)
	if err != nil {
		return
	}

	err = http.ListenAndServe(listenaddr, NewProxy(dialer, ndialer))
	if err != nil {
		logger.Err(err)
	}
}

func main() {
	logger.Infof("%s mode start.", runmode)
	switch runmode {
	case "server":
		run_server()
	case "socks5":
		run_client()
	case "http":
		run_httproxy()
	}
	logger.Info("server stopped")
}
