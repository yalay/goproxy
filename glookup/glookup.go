package main

import (
	"flag"
	"fmt"
	"github.com/op/go-logging"
	"github.com/shell909090/goproxy/dns"
	stdlog "log"
	"os"
)

var log = logging.MustGetLogger("")

func init() {
	var logfile string
	var loglevel string

	flag.StringVar(&logfile, "logfile", "", "log file")
	flag.StringVar(&loglevel, "loglevel", "WARNING", "log level")
	flag.Parse()

	var err error
	file := os.Stderr
	if logfile != "" {
		file, err = os.OpenFile(logfile,
			os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
		if err != nil {
			log.Fatal(err)
		}
	}
	logBackend := logging.NewLogBackend(file, "", stdlog.LstdFlags|stdlog.Lmicroseconds|stdlog.Lshortfile)
	logging.SetBackend(logBackend)

	logging.SetFormatter(logging.MustStringFormatter("%{level}: %{message}"))

	lv, err := logging.LogLevel(loglevel)
	if err != nil {
		panic(err.Error())
	}
	logging.SetLevel(lv, "")
}

func main() {
	if len(flag.Args()) < 1 {
		log.Error("args not enough")
		return
	}

	err := dns.LoadConfig("resolv.conf")
	if err != nil {
		err = dns.LoadConfig("/etc/goproxy/resolv.conf")
		if err != nil {
			return
		}
	}

	for _, hostname := range flag.Args() {
		addrs, err := dns.LookupIP(hostname)
		if err != nil {
			log.Error("%s", err)
			return
		}
		fmt.Println(hostname)
		for _, addr := range addrs {
			fmt.Printf("\t%s\n", addr)
		}

	}
	return
}
