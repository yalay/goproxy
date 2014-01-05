package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/shell909090/goproxy/cryptconn"
	"github.com/shell909090/goproxy/dns"
	"github.com/shell909090/goproxy/ipfilter"
	"github.com/shell909090/goproxy/logging"
	"github.com/shell909090/goproxy/msocks"
	"github.com/shell909090/goproxy/socks"
	"github.com/shell909090/goproxy/sutils"
	"net"
	"net/http"
	"os"
)

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

var logger logging.Logger

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

// func init() {
// 	var logfile string
// 	var loglevel string

// 	flag.StringVar(&runmode, "mode", "http", "server/socks5/http mode")
// 	flag.StringVar(&cipher, "cipher", "aes", "aes/des/tripledes/rc4")
// 	flag.StringVar(&keyfile, "keyfile", "", "key and iv file")
// 	flag.StringVar(&listenaddr, "listen", ":5233", "listen address")
// 	flag.StringVar(&username, "username", "", "username for connect")
// 	flag.StringVar(&password, "password", "", "password for connect")
// 	flag.StringVar(&passfile, "passfile", "", "password file")
// 	flag.StringVar(&blackfile, "black", "", "blacklist file")

// 	flag.StringVar(&logfile, "logfile", "", "log file")
// 	flag.StringVar(&loglevel, "loglevel", "WARNING", "log level")
// 	flag.Parse()

// 	lv, err := logging.GetLevelByName(loglevel)
// 	if err != nil {
// 		panic(err.Error())
// 	}
// 	err = logging.SetupDefault(logfile, lv)
// 	if err != nil {
// 		panic(err.Error())
// 	}

// 	logger, err = logging.NewFileLogger("default", -1, "goproxy")
// 	if err != nil {
// 		panic(err)
// 	}

// }

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

	lv, err := logging.GetLevelByName(cfg.Loglevel)
	if err != nil {
		return
	}

	err = logging.SetupDefault(cfg.Logfile, lv)
	if err != nil {
		return
	}

	logger, err = logging.NewFileLogger("default", -1, "goproxy")
	if err != nil {
		return
	}

	return
}

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	logger.Infof("%s mode start.", cfg.Mode)
	switch cfg.Mode {
	case "server":
		err = run_server(&cfg)
	case "socks5":
		err = run_client(&cfg)
	case "http":
		err = run_httproxy(&cfg)
	default:
		logger.Warning("not supported mode.")
	}
	if err != nil {
		logger.Err(err)
	}
	logger.Info("server stopped")
}
