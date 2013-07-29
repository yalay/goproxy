package src

import (
	"bytes"
	"testing"
)

func TestFrameOKRead (t *testing.T) {
	buf := bytes.NewBuffer([]byte{0x00, 0x00, 0x00, 0x0A, 0x0A})

	f, err := ReadFrame(buf)
	if err != nil {
		t.Fatalf("ReadFrame failed")
	}

	ft, ok := f.(*FrameOK)
	if !ok || ft.streamid != 0x0a0a {
		t.Fatalf("FrameOK format wrong")
	}
}

func TestFrameOKWrite (t *testing.T) {
	f := new(FrameOK)
	f.streamid = 10
	buf := bytes.NewBuffer(nil)
	f.WriteFrame(buf)

	if bytes.Compare(buf.Bytes(), []byte{0x00, 0x00, 0x00, 0x00, 0x0A}) != 0 {
		t.Fatalf("FrameOK write wrong")
	}
}

func TestFrameFailedRead (t *testing.T) {
	buf := bytes.NewBuffer([]byte{0x01, 0x00, 0x04, 0x0A, 0x0A,
		0x00, 0x00, 0x00, 0x01})

	f, err := ReadFrame(buf)
	if err != nil {
		t.Fatalf("ReadFrame failed")
	}

	ft, ok := f.(*FrameFAILED)
	if !ok || ft.streamid != 0x0a0a {
		t.Fatalf("FrameOK format wrong")
	}

	if ft.errno != 1 {
		t.Fatalf("FrameOK format wrong")
	}
}

func TestFrameFailedWrite (t *testing.T) {
	f := new(FrameFAILED)
	f.streamid = 10
	f.errno = 32
	buf := bytes.NewBuffer(nil)
	f.WriteFrame(buf)

	if bytes.Compare(buf.Bytes(), []byte{0x01, 0x00, 0x04, 0x00, 0x0A, 0x00, 0x00, 0x00, 0x20}) != 0 {
		t.Fatalf("FrameFailed write wrong")
	}
	
}