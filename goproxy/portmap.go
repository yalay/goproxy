package main

import (
	"net"

	"github.com/shell909090/goproxy/sutils"
)

func CreatePortmap(saddr, daddr string, dialer sutils.Dialer) (err error) {
	lsock, err := net.Listen("tcp", saddr)
	if err != nil {
		return
	}

	for {
		var sconn, dconn net.Conn

		sconn, err = lsock.Accept()
		if err != nil {
			log.Error("%s", err.Error())
			continue
		}

		dconn, err = dialer.Dial("tcp", daddr)
		if err != nil {
			log.Error("%s", err.Error())
			sconn.Close()
			continue
		}

		go CopyLink(dconn, sconn)
	}
}
