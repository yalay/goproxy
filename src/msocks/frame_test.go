package msocks

import (
	"bytes"
	"logging"
	"net"
	"testing"
)

func init() {
	logging.SetupDefault("", logging.LOG_INFO)
}

func TestFrameOKRead(t *testing.T) {
	buf := bytes.NewBuffer([]byte{0x00, 0x00, 0x00, 0x0A, 0x0A})

	f, err := ReadFrame(buf)
	if err != nil {
		t.Fatalf("Read FrameOK failed")
	}

	ft, ok := f.(*FrameOK)
	if !ok || ft.Streamid != 0x0a0a {
		t.Fatalf("FrameOK format wrong")
	}
}

func TestFrameOKWrite(t *testing.T) {
	b := NewFrameOK(10)

	if bytes.Compare(b, []byte{0x00, 0x00, 0x00, 0x00, 0x0A}) != 0 {
		t.Fatalf("FrameOK write wrong")
	}
}

func TestFrameFailedRead(t *testing.T) {
	buf := bytes.NewBuffer([]byte{0x01, 0x00, 0x04, 0x0A, 0x0A,
		0x00, 0x00, 0x00, 0x01})

	f, err := ReadFrame(buf)
	if err != nil {
		t.Fatalf("Read FrameFailed failed")
	}

	ft, ok := f.(*FrameFAILED)
	if !ok || ft.Streamid != 0x0a0a {
		t.Fatalf("FrameFailed format wrong")
	}

	if ft.Errno != 1 {
		t.Fatalf("FrameFailed body wrong")
	}
}

func TestFrameFailedWrite(t *testing.T) {
	b := NewFrameFAILED(10, 32)

	if bytes.Compare(b, []byte{0x01, 0x00, 0x04, 0x00, 0x0A, 0x00, 0x00, 0x00, 0x20}) != 0 {
		t.Fatalf("FrameFailed write wrong")
	}
}

func TestFrameAuthRead(t *testing.T) {
	buf := bytes.NewBuffer([]byte{0x02, 0x00, 0x08, 0x0A, 0x0A,
		0x00, 0x02, 0x61, 0x62, 0x00, 0x02, 0x63, 0x64})

	f, err := ReadFrame(buf)
	if err != nil {
		t.Fatalf("Read FrameAuth failed")
	}

	ft, ok := f.(*FrameAuth)
	if !ok || ft.Streamid != 0x0a0a {
		t.Fatalf("FrameAuth format wrong")
	}

	if ft.Username != "ab" || ft.Password != "cd" {
		t.Fatalf("FrameAuth body wrong")
	}
}

func TestFrameAuthWrite(t *testing.T) {
	b, err := NewFrameAuth(10, "username", "password")
	if err != nil {
		t.Error(err)
	}

	if bytes.Compare(b, []byte{0x02, 0x00, 0x14, 0x00, 0x0A,
		0x00, 0x08, 0x75, 0x73, 0x65, 0x72, 0x6e, 0x61, 0x6d, 0x65,
		0x00, 0x08, 0x70, 0x61, 0x73, 0x73, 0x77, 0x6f, 0x72, 0x64}) != 0 {
		t.Fatalf("FrameAuth write wrong")
	}
}

func TestFrameDataRead(t *testing.T) {
	buf := bytes.NewBuffer([]byte{0x03, 0x00, 0x03, 0x0A, 0x0A,
		0x01, 0x05, 0x07})

	f, err := ReadFrame(buf)
	if err != nil {
		t.Fatalf("Read FrameData failed")
	}

	ft, ok := f.(*FrameData)
	if !ok || ft.Streamid != 0x0a0a {
		t.Fatalf("FrameData format wrong")
	}

	if bytes.Compare(ft.Data, []byte{0x01, 0x05, 0x07}) != 0 {
		t.Fatalf("FrameData body wrong")
	}
}

func TestFrameDataWrite(t *testing.T) {
	b, err := NewFrameData(10, []byte{0x01, 0x02, 0x03})
	if err != nil {
		t.Error(err)
	}

	if bytes.Compare(b, []byte{0x03, 0x00, 0x03, 0x00, 0x0A,
		0x01, 0x02, 0x03}) != 0 {
		t.Fatalf("FrameData write wrong")
	}
}

func TestFrameSynRead(t *testing.T) {
	buf := bytes.NewBuffer([]byte{0x04, 0x00, 0x04, 0x0A, 0x0A,
		0x00, 0x02, 0x61, 0x62})

	f, err := ReadFrame(buf)
	if err != nil {
		t.Fatalf("Read FrameSyn failed")
	}

	ft, ok := f.(*FrameSyn)
	if !ok || ft.Streamid != 0x0a0a {
		t.Fatalf("FrameSyn format wrong")
	}

	if ft.Address != "ab" {
		t.Fatalf("FrameSyn body wrong")
	}
}

func TestFrameSynWrite(t *testing.T) {
	b, err := NewFrameSyn(10, "cd")
	if err != nil {
		t.Error(err)
	}

	if bytes.Compare(b, []byte{0x04, 0x00, 0x04, 0x00, 0x0A,
		0x00, 0x02, 0x63, 0x64}) != 0 {
		t.Fatalf("FrameSyn write wrong")
	}
}

func TestFrameAckRead(t *testing.T) {
	buf := bytes.NewBuffer([]byte{0x05, 0x00, 0x04, 0x0A, 0x0A,
		0x01, 0x02, 0x03, 0x04})

	f, err := ReadFrame(buf)
	if err != nil {
		t.Fatalf("Read FrameAck failed")
	}

	ft, ok := f.(*FrameAck)
	if !ok || ft.Streamid != 0x0a0a {
		t.Fatalf("FrameAck format wrong")
	}

	if ft.Window != 0x01020304 {
		t.Fatalf("FrameAck body wrong")
	}
}

func TestFrameAckWrite(t *testing.T) {
	b := NewFrameAck(10, 0x04050607)

	if bytes.Compare(b, []byte{0x05, 0x00, 0x04, 0x00, 0x0A,
		0x04, 0x05, 0x06, 0x07}) != 0 {
		t.Fatalf("FrameAck write wrong")
	}
}

func TestFrameFinRead(t *testing.T) {
	buf := bytes.NewBuffer([]byte{0x06, 0x00, 0x00, 0x0A, 0x0A})

	f, err := ReadFrame(buf)
	if err != nil {
		t.Fatalf("Read FrameFin failed")
	}

	ft, ok := f.(*FrameFin)
	if !ok || ft.Streamid != 0x0a0a {
		t.Fatalf("FrameFin format wrong")
	}
}

func TestFrameFinWrite(t *testing.T) {
	b := NewFrameFin(10)

	if bytes.Compare(b, []byte{0x06, 0x00, 0x00, 0x00, 0x0A}) != 0 {
		t.Fatalf("FrameFin write wrong")
	}
}

func TestFrameDnsRead(t *testing.T) {
	buf := bytes.NewBuffer([]byte{0x07, 0x00, 0x04, 0x0A, 0x0A,
		0x00, 0x02, 0x61, 0x62})

	f, err := ReadFrame(buf)
	if err != nil {
		t.Fatalf("Read FrameDns failed")
	}

	ft, ok := f.(*FrameDns)
	if !ok || ft.Streamid != 0x0a0a {
		t.Fatalf("FrameDns format wrong")
	}

	if ft.Hostname != "ab" {
		t.Fatalf("FrameDns body wrong")
	}
}

func TestFrameDnsWrite(t *testing.T) {
	b, err := NewFrameDns(10, "cd")
	if err != nil {
		t.Error(err)
	}

	if bytes.Compare(b, []byte{0x07, 0x00, 0x04, 0x00, 0x0A,
		0x00, 0x02, 0x63, 0x64}) != 0 {
		t.Fatalf("FrameDns write wrong")
	}
}

func TestFrameAddrRead(t *testing.T) {
	buf := bytes.NewBuffer([]byte{0x08, 0x00, 0x05, 0x0A, 0x0A,
		0x04, 0x01, 0x02, 0x03, 0x04})

	f, err := ReadFrame(buf)
	if err != nil {
		t.Fatalf("Read FrameDns failed")
	}

	ft, ok := f.(*FrameAddr)
	if !ok || ft.Streamid != 0x0a0a {
		t.Fatalf("FrameDns format wrong")
	}

	if len(ft.Ipaddr) != 1 {
		t.Fatalf("length of ipaddr not match")
	}

	if bytes.Compare(ft.Ipaddr[0], []byte{0x01, 0x02, 0x03, 0x04}) != 0 {
		t.Fatalf("FrameAddr body wrong")
	}
}

func TestFrameAddrWrite(t *testing.T) {
	b, err := NewFrameAddr(10, []net.IP{[]byte{0x01, 0x02, 0x03, 0x04}})
	if err != nil {
		t.Error(err)
	}

	if bytes.Compare(b, []byte{0x08, 0x00, 0x05, 0x00, 0x0A,
		0x04, 0x01, 0x02, 0x03, 0x04}) != 0 {
		t.Fatalf("FrameAddr write wrong")
	}
}
