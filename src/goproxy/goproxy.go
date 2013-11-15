package main

import (
	"cryptconn"
	"dns"
	"flag"
	"ipfilter"
	"logging"
	"net"
	"net/http"
	// qsocks to msocks
	"msocks"
	"socks"
	"sutils"
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

	flag.StringVar(&runmode, "mode", "", "server/client/httproxy mode")
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

	// qsocks to msocks
	qs, err := msocks.NewService(passfile, sutils.DefaultTcpDialer)
	if err != nil {
		return
	}

	err = qs.ServeTCP(listener)
	if err != nil {
		logger.Err(err)
	}
}

func get_dialer(serveraddr string) (dialer sutils.Dialer, err error) {
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

	// qsocks to msocks
	dialer, err = msocks.NewDialer(dialer, serveraddr, username, password)
	if err != nil {
		return
	}

	if blackfile != "" {
		dialer, err = ipfilter.NewFilteredDialer(
			sutils.DefaultTcpDialer, dialer, blackfile)
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

	dialer, err := get_dialer(serveraddr)
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
	err = ss.ServeTCP(listener)
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

	dialer, err := get_dialer(serveraddr)
	if err != nil {
		return
	}

	http.ListenAndServe(listenaddr, &Proxy{
		dialer: dialer,
		tspt:   http.Transport{Dial: dialer.Dial},
	})
}

func main() {
	logger.Infof("%s mode start.", runmode)
	switch runmode {
	case "server":
		run_server()
	case "client":
		run_client()
	case "httproxy":
		run_httproxy()
	}
	logger.Info("server stopped")
}
