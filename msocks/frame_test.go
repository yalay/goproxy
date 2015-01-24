package msocks

import (
	"bytes"
	"testing"
)

func TestFrameResultRead(t *testing.T) {
	buf := bytes.NewBuffer([]byte{MSG_RESULT, 0x00, 0x04, 0x0A, 0x0A,
		0x00, 0x00, 0x00, 0x01})

	f, err := ReadFrame(buf)
	if err != nil {
		t.Fatalf("Read FrameResult failed")
	}

	ft, ok := f.(*FrameResult)
	if !ok || ft.Streamid != 0x0a0a {
		t.Fatalf("FrameResult format wrong")
	}

	if ft.Errno != 1 {
		t.Fatalf("FrameResult body wrong")
	}
}

func TestFrameResultWrite(t *testing.T) {
	f := NewFrameResult(10, 32)
	buf, err := f.Packed()
	if err != nil {
		t.Error(err)
	}

	if bytes.Compare(buf.Bytes(), []byte{MSG_RESULT, 0x00, 0x04, 0x00, 0x0A,
		0x00, 0x00, 0x00, 0x20}) != 0 {
		t.Fatalf("FrameResult write wrong")
	}
}

func TestFrameAuthRead(t *testing.T) {
	buf := bytes.NewBuffer([]byte{MSG_AUTH, 0x00, 0x08, 0x0A, 0x0A,
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
	f := NewFrameAuth(10, "username", "password")
	buf, err := f.Packed()
	if err != nil {
		t.Error(err)
	}

	if bytes.Compare(buf.Bytes(), []byte{MSG_AUTH, 0x00, 0x14, 0x00, 0x0A,
		0x00, 0x08, 0x75, 0x73, 0x65, 0x72, 0x6e, 0x61, 0x6d, 0x65,
		0x00, 0x08, 0x70, 0x61, 0x73, 0x73, 0x77, 0x6f, 0x72, 0x64}) != 0 {
		t.Fatalf("FrameAuth write wrong")
	}
}

func TestFrameDataRead(t *testing.T) {
	buf := bytes.NewBuffer([]byte{MSG_DATA, 0x00, 0x03, 0x0A, 0x0A,
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
	f := NewFrameData(10, []byte{0x01, 0x02, 0x03})
	buf, err := f.Packed()
	if err != nil {
		t.Error(err)
	}

	if bytes.Compare(buf.Bytes(), []byte{MSG_DATA, 0x00, 0x03, 0x00, 0x0A,
		0x01, 0x02, 0x03}) != 0 {
		t.Fatalf("FrameData write wrong")
	}
}

func TestFrameSynRead(t *testing.T) {
	buf := bytes.NewBuffer([]byte{MSG_SYN, 0x00, 0x08, 0x0A, 0x0A,
		0x00, 0x02, 0x61, 0x62, 0x00, 0x02, 0x63, 0x64})

	f, err := ReadFrame(buf)
	if err != nil {
		t.Fatalf("Read FrameSyn failed")
	}

	ft, ok := f.(*FrameSyn)
	if !ok || ft.Streamid != 0x0a0a {
		t.Fatalf("FrameSyn format wrong")
	}

	if ft.Network != "ab" || ft.Address != "cd" {
		t.Fatalf("FrameSyn body wrong")
	}
}

func TestFrameSynWrite(t *testing.T) {
	f := NewFrameSyn(10, "cd", "ab")
	buf, err := f.Packed()
	if err != nil {
		t.Error(err)
	}

	if bytes.Compare(buf.Bytes(), []byte{MSG_SYN, 0x00, 0x08, 0x00, 0x0A,
		0x00, 0x02, 0x63, 0x64, 0x00, 0x02, 0x61, 0x62}) != 0 {
		t.Fatalf("FrameSyn write wrong")
	}
}

func TestFrameWndRead(t *testing.T) {
	buf := bytes.NewBuffer([]byte{MSG_WND, 0x00, 0x04, 0x0A, 0x0A,
		0x01, 0x02, 0x03, 0x04})

	f, err := ReadFrame(buf)
	if err != nil {
		t.Fatalf("Read FrameWnd failed")
	}

	ft, ok := f.(*FrameWnd)
	if !ok || ft.Streamid != 0x0a0a {
		t.Fatalf("FrameWnd format wrong")
	}

	if ft.Window != 0x01020304 {
		t.Fatalf("FrameWnd body wrong")
	}
}

func TestFrameWndWrite(t *testing.T) {
	f := NewFrameWnd(10, 0x04050607)
	buf, err := f.Packed()
	if err != nil {
		t.Error(err)
	}

	if bytes.Compare(buf.Bytes(), []byte{MSG_WND, 0x00, 0x04, 0x00, 0x0A,
		0x04, 0x05, 0x06, 0x07}) != 0 {
		t.Fatalf("FrameWnd write wrong")
	}
}

func TestFrameFinRead(t *testing.T) {
	buf := bytes.NewBuffer([]byte{MSG_FIN, 0x00, 0x00, 0x0A, 0x0A})

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
	f := NewFrameFin(10)
	buf, err := f.Packed()
	if err != nil {
		t.Error(err)
	}

	if bytes.Compare(buf.Bytes(), []byte{MSG_FIN, 0x00, 0x00, 0x00, 0x0A}) != 0 {
		t.Fatalf("FrameFin write wrong")
	}
}

func TestFrameRstRead(t *testing.T) {
	buf := bytes.NewBuffer([]byte{MSG_RST, 0x00, 0x00, 0x0A, 0x0A})

	f, err := ReadFrame(buf)
	if err != nil {
		t.Fatalf("Read FrameRst failed")
	}

	ft, ok := f.(*FrameRst)
	if !ok || ft.Streamid != 0x0a0a {
		t.Fatalf("FrameRst format wrong")
	}
}

func TestFrameRstWrite(t *testing.T) {
	f := NewFrameRst(10)
	buf, err := f.Packed()
	if err != nil {
		t.Error(err)
	}

	if bytes.Compare(buf.Bytes(), []byte{MSG_RST, 0x00, 0x00, 0x00, 0x0A}) != 0 {
		t.Fatalf("FrameFin write wrong")
	}
}

func TestFramePingRead(t *testing.T) {
	buf := bytes.NewBuffer([]byte{MSG_PING, 0x00, 0x00, 0x0A, 0x0A})

	f, err := ReadFrame(buf)
	if err != nil {
		t.Fatalf("Read FramePing failed")
	}

	ft, ok := f.(*FramePing)
	if !ok || ft.Streamid != 0x0a0a {
		t.Fatalf("FramePing format wrong")
	}
}

func TestFramePingWrite(t *testing.T) {
	f := NewFramePing()
	buf, err := f.Packed()
	if err != nil {
		t.Error(err)
	}

	if bytes.Compare(buf.Bytes(), []byte{MSG_PING, 0x00, 0x00, 0x00, 0x00}) != 0 {
		t.Fatalf("FramePing write wrong")
	}
}

func TestFrameDnsRead(t *testing.T) {
	buf := bytes.NewBuffer([]byte{MSG_DNS, 0x00, 0x03, 0x0A, 0x0A,
		0x01, 0x05, 0x07})

	f, err := ReadFrame(buf)
	if err != nil {
		t.Fatalf("Read FrameDns failed")
	}

	ft, ok := f.(*FrameDns)
	if !ok || ft.Streamid != 0x0a0a {
		t.Fatalf("FrameDns format wrong")
	}

	if bytes.Compare(ft.Data, []byte{0x01, 0x05, 0x07}) != 0 {
		t.Fatalf("FrameDns body wrong")
	}
}

func TestFrameDnsWrite(t *testing.T) {
	f := NewFrameDns(10, []byte{0x01, 0x02, 0x03})
	buf, err := f.Packed()
	if err != nil {
		t.Error(err)
	}

	if bytes.Compare(buf.Bytes(), []byte{MSG_DNS, 0x00, 0x03, 0x00, 0x0A,
		0x01, 0x02, 0x03}) != 0 {
		t.Fatalf("FrameDns write wrong")
	}
}
