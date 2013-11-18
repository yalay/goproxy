package main

import (
	"cryptconn"
	"flag"
	"fmt"
	"ipfilter"
	"logging"
	"msocks"
	"sutils"
)

var cipher string
var keyfile string
var username string
var password string
var blackfile string
var logger logging.Logger

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

	lv, err := logging.GetLevelByName(loglevel)
	if err != nil {
		panic(err.Error())
	}
	err = logging.SetupDefault(logfile, lv)
	if err != nil {
		panic(err.Error())
	}

	logger, err = logging.NewFileLogger("default", -1, "glookup")
	if err != nil {
		panic(err)
	}
}

func main() {
	if len(flag.Args()) < 2 {
		logger.Err("args not enough")
		return
	}
	serveraddr := flag.Args()[0]

	blacklist, err := ipfilter.ReadIPListFile("routes.list.gz")
	if err != nil {
		logger.Err(err)
		return
	}

	var dialer sutils.Dialer
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

	ndialer, err := msocks.NewDialer(dialer, serveraddr, username, password)
	if err != nil {
		return
	}

	for _, hostname := range flag.Args()[1:] {
		addrs, err := ndialer.LookupIP(hostname)
		if err != nil {
			logger.Err(err)
			return
		}
		fmt.Println(hostname)
		for _, addr := range addrs {
			fmt.Printf("\t%s\t%t\n", addr, blacklist.Contain(addr))
		}

	}
	return
}
