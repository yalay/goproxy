package main

import (
	"flag"
	"fmt"
	"github.com/op/go-logging"
	"github.com/shell909090/goproxy/cryptconn"
	"github.com/shell909090/goproxy/ipfilter"
	stdlog "log"
	"os"
	// "github.com/shell909090/goproxy/logging"
	"github.com/shell909090/goproxy/msocks"
	"github.com/shell909090/goproxy/sutils"
)

var log = logging.MustGetLogger("package.example")

var cipher string
var keyfile string
var username string
var password string
var blackfile string

// TODO: fit two mode

func init() {
	var logfile string
	var loglevel string

	flag.StringVar(&cipher, "cipher", "aes", "aes/des/tripledes/rc4")
	flag.StringVar(&keyfile, "keyfile", "key", "key and iv file")
	flag.StringVar(&username, "username", "", "username for connect")
	flag.StringVar(&password, "password", "", "password for connect")
	flag.StringVar(&blackfile, "black", "routes.list.gz", "blacklist file")

	flag.StringVar(&logfile, "logfile", "", "log file")
	flag.StringVar(&loglevel, "loglevel", "WARNING", "log level")
	flag.Parse()

	var err error
	file := os.Stderr
	if logfile != "" {
		file, err = os.Open(logfile)
		if err != nil {
			log.Fatal(err)
		}
	}
	logBackend := logging.NewLogBackend(file, "", stdlog.LstdFlags|stdlog.Lshortfile)
	logging.SetBackend(logBackend)

	logging.SetFormatter(logging.MustStringFormatter("%{level}: %{message}"))

	lv, err := logging.LogLevel(loglevel)
	if err != nil {
		panic(err.Error())
	}
	logging.SetLevel(lv, "")
}

func main() {
	if len(flag.Args()) < 2 {
		log.Error("args not enough")
		return
	}
	serveraddr := flag.Args()[0]

	blacklist, err := ipfilter.ReadIPListFile("routes.list.gz")
	if err != nil {
		log.Error("%s", err)
		return
	}

	var dialer sutils.Dialer
	dialer = sutils.DefaultTcpDialer
	if len(keyfile) > 0 {
		dialer, err = cryptconn.NewDialer(dialer, cipher, keyfile)
		if err != nil {
			log.Error("crypto not work, cipher or keyfile wrong.")
			return
		}
	} else {
		log.Warning("no vaild keyfile.")
	}

	ndialer, err := msocks.NewDialer(dialer, serveraddr, username, password)
	if err != nil {
		return
	}

	for _, hostname := range flag.Args()[1:] {
		addrs, err := ndialer.LookupIP(hostname)
		if err != nil {
			log.Error("%s", err)
			return
		}
		fmt.Println(hostname)
		for _, addr := range addrs {
			fmt.Printf("\t%s\t%t\n", addr, blacklist.Contain(addr))
		}

	}
	return
}
