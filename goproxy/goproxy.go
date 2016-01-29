package main

import (
	"encoding/json"
	"flag"
	"fmt"
	stdlog "log"
	"net"
	"net/http"
	"os"

	logging "github.com/op/go-logging"
	"github.com/shell909090/goproxy/cryptconn"
	"github.com/shell909090/goproxy/ipfilter"
	"github.com/shell909090/goproxy/msocks"
	"github.com/shell909090/goproxy/pool"
	"github.com/shell909090/goproxy/sutils"
)

var log = logging.MustGetLogger("")

const TypeInternal = "internal"

type PortMap struct {
	Net string
	Src string
	Dst string
}

type Config struct {
	Mode   string
	Listen string
	Server string

	Logfile    string
	Loglevel   string
	AdminIface string

	DnsAddrs []string
	DnsNet   string

	Cipher    string
	Key       string
	Blackfile string

	MinSess int
	MaxConn int

	Username string
	Password string
	Auth     map[string]string

	Portmaps []PortMap
}

func httpserver(addr string, handler http.Handler) {
	for {
		err := http.ListenAndServe(addr, handler)
		if err != nil {
			log.Error("%s", err.Error())
			return
		}
	}
}

func run_server(cfg *Config) (err error) {
	listener, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		return
	}

	listener, err = cryptconn.NewListener(listener, cfg.Cipher, cfg.Key)
	if err != nil {
		return
	}

	svr, err := pool.NewServer(cfg.Auth, sutils.DefaultTcpDialer)
	if err != nil {
		return
	}

	if cfg.AdminIface != "" {
		mux := http.NewServeMux()
		NewMsocksManager(svr.SessionPool).Register(mux)
		go httpserver(cfg.AdminIface, mux)
	}

	return svr.Serve(listener)
}

func run_httproxy(cfg *Config) (err error) {
	var dialer sutils.Dialer

	dialer, err = cryptconn.NewDialer(sutils.DefaultTcpDialer, cfg.Cipher, cfg.Key)
	if err != nil {
		return
	}

	sf, err := msocks.NewSessionFactory(dialer, cfg.Server, cfg.Username, cfg.Password)
	if err != nil {
		return
	}

	sp := pool.CreateSessionPool(cfg.MinSess, cfg.MaxConn)
	sp.AddSessionFactory(sf)
	ndialer := sp

	// dialer, err = msocks.NewDialer(dialer, cfg.Server, cfg.Username, cfg.Password)
	// if err != nil {
	// 	return
	// }
	// ndialer := dialer.(*msocks.Dialer)

	if cfg.DnsNet == TypeInternal {
		sutils.DefaultLookuper = ndialer
	}

	if cfg.AdminIface != "" {
		mux := http.NewServeMux()
		NewMsocksManager(sp).Register(mux)
		go httpserver(cfg.AdminIface, mux)
	}

	if cfg.Blackfile != "" {
		dialer, err = ipfilter.NewFilteredDialer(
			dialer, sutils.DefaultTcpDialer, cfg.Blackfile)
		if err != nil {
			return
		}
	}

	for _, pm := range cfg.Portmaps {
		go CreatePortmap(pm, dialer)
	}

	return http.ListenAndServe(cfg.Listen, NewProxy(dialer))
}

func LoadConfig() (cfg Config, err error) {
	var configfile string
	flag.StringVar(&configfile, "config", "/etc/goproxy/config.json", "config file")
	flag.Parse()

	file, err := os.Open(configfile)
	if err != nil {
		return
	}
	defer file.Close()

	dec := json.NewDecoder(file)
	err = dec.Decode(&cfg)
	if err != nil {
		return
	}

	if cfg.Cipher == "" {
		cfg.Cipher = "aes"
	}
	return
}

func SetLogging(cfg Config) (err error) {
	var file *os.File
	file = os.Stdout

	if cfg.Logfile != "" {
		file, err = os.OpenFile(cfg.Logfile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
		if err != nil {
			log.Fatal(err)
		}
	}
	logBackend := logging.NewLogBackend(file, "",
		stdlog.LstdFlags|stdlog.Lmicroseconds|stdlog.Lshortfile)
	logging.SetBackend(logBackend)

	logging.SetFormatter(logging.MustStringFormatter("%{level}: %{message}"))

	lv, err := logging.LogLevel(cfg.Loglevel)
	if err != nil {
		panic(err.Error())
	}
	logging.SetLevel(lv, "")

	return
}

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	err = SetLogging(cfg)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	if len(cfg.DnsAddrs) > 0 && cfg.DnsNet != TypeInternal {
		sutils.DefaultLookuper = sutils.NewDnsLookup(cfg.DnsAddrs, cfg.DnsNet)
	}

	switch cfg.Mode {
	case "server":
		log.Notice("server mode start.")
		err = run_server(&cfg)
	case "http":
		log.Notice("http mode start.")
		err = run_httproxy(&cfg)
	default:
		log.Info("unknown mode")
		return
	}
	if err != nil {
		log.Error("%s", err)
	}
	log.Info("server stopped")
}
