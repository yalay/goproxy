package ipfilter

import (
	"bytes"
	"logging"
	"net"
	"testing"
)

const iplist = "10.0.0.0 255.0.0.0\n172.16.0.0 255.240.0.0\n192.168.0.0 255.255.0.0"

func init() {
	logging.SetupDefault("", logging.LOG_INFO)
}

func TestIPList(t *testing.T) {
	buf := bytes.NewBufferString(iplist)
	iplist, err := ReadIPList(buf)
	if err != nil {
		t.Fatalf("ReadIPList failed: %s", err)
	}

	if !iplist.Contain(net.ParseIP("192.168.1.1")) {
		t.Fatalf("Contain wrong1.")
	}

	if iplist.Contain(net.ParseIP("211.80.90.25")) {
		t.Fatalf("Contain wrong2.")
	}
}
