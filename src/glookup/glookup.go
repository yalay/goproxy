package main

import (
	"dns"
	"flag"
	"fmt"
	"ipfilter"
	"logging"
)

func main() {
	flag.Parse()

	blacklist, err := ipfilter.ReadIPList("routes.list.gz")
	if err != nil {
		logging.Err(err)
		return
	}
	err = dns.LoadConfig("resolv.conf")
	if err != nil {
		logging.Err(err)
		return
	}

	addrs, _ := dns.LookupIP(flag.Arg(0))
	fmt.Println(flag.Arg(0))
	for _, addr := range addrs {
		fmt.Printf("\t%s\t%t\n", addr, blacklist.Contain(addr))
	}
}
