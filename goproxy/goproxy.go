package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/op/go-logging"
	"github.com/shell909090/goproxy/cryptconn"
	"github.com/shell909090/goproxy/dns"
	"github.com/shell909090/goproxy/ipfilter"
	"github.com/shell909090/goproxy/msocks"
	"github.com/shell909090/goproxy/socks"
	"github.com/shell909090/goproxy/sutils"
	stdlog "log"
	"net"
	"net/http"
	"os"
)

var log = logging.MustGetLogger("")

type Config struct {
	Mode   string
	Listen string
	Server string

	Logfile  string
	Loglevel string

	Cipher    string
	Keyfile   string
	Blackfile string

	Username string
	Password string
	Auth     map[string]string
}

func run_server(cfg *Config) (err error) {
	listener, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		return
	}

	listener, err = cryptconn.NewListener(
		listener, cfg.Cipher, cfg.Keyfile)
	if err != nil {
		return
	}

	s, err := msocks.NewService(cfg.Auth, sutils.DefaultTcpDialer)
	if err != nil {
		return
	}

	return s.Serve(listener)
}

func get_dialer(cfg *Config) (dialer sutils.Dialer, ndialer *msocks.Dialer, err error) {
	err = dns.LoadConfig("resolv.conf")
	if err != nil {
		err = dns.LoadConfig("/etc/goproxy/resolv.conf")
		if err != nil {
			return
		}
	}

	dialer = sutils.DefaultTcpDialer

	dialer, err = cryptconn.NewDialer(dialer, cfg.Cipher, cfg.Keyfile)
	if err != nil {
		return
	}

	ndialer, err = msocks.NewDialer(
		dialer, cfg.Server, cfg.Username, cfg.Password)
	if err != nil {
		return
	}
	dialer = ndialer

	if cfg.Blackfile != "" {
		dialer, err = ipfilter.NewFilteredDialer(
			dialer, sutils.DefaultTcpDialer, cfg.Blackfile)
		if err != nil {
			return
		}
	}

	return
}

func run_client(cfg *Config) (err error) {
	dialer, _, err := get_dialer(cfg)
	if err != nil {
		return
	}

	listener, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		return
	}

	s := socks.NewService(dialer)
	return s.Serve(listener)
}

func run_httproxy(cfg *Config) (err error) {
	dialer, ndialer, err := get_dialer(cfg)
	if err != nil {
		return
	}

	mux := http.NewServeMux()
	NewMsocksManager(ndialer).Register(mux)
	return http.ListenAndServe(cfg.Listen, NewProxy(dialer, mux))
}

func LoadConfig() (cfg Config, err error) {
	var configfile string
	flag.StringVar(&configfile, "config",
		"/etc/goproxy/config.json", "config file")
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

	file = os.Stderr
	if cfg.Logfile != "" {
		file, err = os.OpenFile(cfg.Logfile, os.O_RDWR|os.O_CREATE, 0666)
		if err != nil {
			log.Fatal(err)
		}
	}
	logBackend := logging.NewLogBackend(file, "", stdlog.LstdFlags|stdlog.Lshortfile)
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

	log.Info("%s mode start.", cfg.Mode)
	switch cfg.Mode {
	case "server":
		err = run_server(&cfg)
	case "socks5":
		err = run_client(&cfg)
	case "http":
		err = run_httproxy(&cfg)
	default:
		log.Warning("not supported mode.")
	}
	if err != nil {
		log.Error("%s", err)
	}
	log.Info("server stopped")
}
