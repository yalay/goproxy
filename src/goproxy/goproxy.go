package main

import (
	"cryptconn"
	"dns"
	"flag"
	"ipfilter"
	"logging"
	"net"
	"qsocks"
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
var logger *logging.Logger

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

	logger, err = logging.NewLogger("", -1, "goproxy")
	if err != nil {
		panic(err.Error())
	}
}

var cryptWrapper func(net.Conn) (net.Conn, error) = nil

func run_server() {
	var err error

	qs, err := qsocks.NewService(passfile, cryptWrapper)
	if err != nil {
		return
	}

	tcpAddr, err := net.ResolveTCPAddr("tcp", listenaddr)
	if err != nil {
		logging.Err(err)
		return
	}
	listener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		logging.Err(err)
		return
	}

	err = qs.ServeTCP(listener)
	if err != nil {
		logging.Err(err)
	}
}

func run_client() {
	var err error

	if len(flag.Args()) < 1 {
		panic("args not enough")
	}
	serveraddr := flag.Args()[0]

	err = dns.LoadConfig("resolv.conf")
	if err != nil {
		err = dns.LoadConfig("/etc/goproxy/resolv.conf")
		if err != nil {
			return
		}
	}

	var dialer sutils.Dialer
	dialer = qsocks.NewDialer(
		serveraddr, cryptWrapper, username, password)
	if err != nil {
		return
	}

	if blackfile != "" {
		dialer, err = ipfilter.NewFilteredDialer(
			blackfile, false, dialer)
		if err != nil {
			return
		}
	}

	ss := socks.NewService(dialer)

	tcpAddr, err := net.ResolveTCPAddr("tcp", listenaddr)
	if err != nil {
		logging.Err(err)
		return
	}
	listener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		logging.Err(err)
		return
	}

	err = ss.ServeTCP(listener)
	if err != nil {
		logging.Err(err)
	}
}

// var tspt http.Transport

// type Proxy struct{}

// func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
// 	logging.Info(r.Method, r.URL)

// 	if r.Method == "CONNECT" {
// 		p.Connect(w, r)
// 		return
// 	}

// 	r.RequestURI = ""
// 	r.Header.Del("Accept-Encoding")
// 	r.Header.Del("Proxy-Connection")
// 	r.Header.Del("Connection")

// 	resp, err := tspt.RoundTrip(r)
// 	if err != nil {
// 		logging.Err(err)
// 		return
// 	}
// 	defer resp.Body.Close()

// 	resp.Header.Del("Content-Length")
// 	for k, vv := range resp.Header {
// 		for _, v := range vv {
// 			w.Header().Add(k, v)
// 		}
// 	}
// 	w.WriteHeader(resp.StatusCode)
// 	_, err = sutils.CoreCopy(w, resp.Body)
// 	if err != nil {
// 		logging.Err(err)
// 		return
// 	}
// 	return
// }

// func (p *Proxy) Connect(w http.ResponseWriter, r *http.Request) {
// 	hij, ok := w.(http.Hijacker)
// 	if !ok {
// 		logging.Err("httpserver does not support hijacking")
// 		return
// 	}
// 	srcconn, _, err := hij.Hijack()
// 	if err != nil {
// 		logging.Err("Cannot hijack connection ", err)
// 		return
// 	}
// 	defer srcconn.Close()

// 	host := r.URL.Host
// 	if !strings.Contains(host, ":") {
// 		host += ":80"
// 	}
// 	dstconn, err := socks.DialConn("tcp", host)
// 	if err != nil {
// 		logging.Err(err)
// 		srcconn.Write([]byte("HTTP/1.0 502 OK\r\n\r\n"))
// 		return
// 	}
// 	defer dstconn.Close()
// 	srcconn.Write([]byte("HTTP/1.0 200 OK\r\n\r\n"))

// 	sutils.CopyLink(srcconn, dstconn)
// 	return
// }

// func run_httproxy() {
// 	if cryptWrapper == nil {
// 		logging.Warning("client mode without keyfile")
// 	}

// 	if len(flag.Args()) < 1 {
// 		panic("args not enough")
// 	}
// 	serveraddr := flag.Args()[0]

// 	err := dns.LoadConfig("resolv.conf")
// 	if err != nil {
// 		err = dns.LoadConfig("/etc/goproxy/resolv.conf")
// 		if err != nil {
// 			panic(err.Error())
// 		}
// 	}

// 	socks.InitDail(blackfile, serveraddr, cryptWrapper, username, password)

// 	tspt = http.Transport{Dial: socks.DialConn}
// 	http.ListenAndServe(listenaddr, &Proxy{})
// }

func main() {
	var err error

	if len(keyfile) > 0 {
		cryptWrapper, err = cryptconn.NewCryptWrapper(cipher, keyfile)
		if err != nil {
			logging.Err("crypto not work, cipher or keyfile wrong.")
			return
		}
	}

	if cryptWrapper == nil {
		logging.Warning("no vaild keyfile.")
	}

	logging.Infof("%s mode start", runmode)
	switch runmode {
	case "server":
		run_server()
	case "client":
		run_client()
		// case "httproxy":
		// 	run_httproxy()
	}
	logging.Info("server stopped")
}
