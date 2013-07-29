package src

import (
	"io"
	"net"
	"errors"
	"encoding/binary"
)

const (
	MSG_OK = iota
	MSG_FAILED
	MSG_AUTH
	MSG_DATA
	MSG_SYN
	MSG_ACK
	MSG_FIN
	MSG_RST
)

type Frame interface {
	ReadFrame (length uint16, streamid uint16, r io.Reader) (err error)
	WriteFrame (w io.Writer) (err error)
}

func ReadFrame (r io.Reader) (f Frame, err error) {
	var buf [2]byte
	_, err = r.Read(buf[:1])
	if err != nil { return }
	msgtype := uint8(buf[0])

	_, err = r.Read(buf[:])
	if err != nil { return }
	length := binary.BigEndian.Uint16(buf[:])

	_, err = r.Read(buf[:])
	if err != nil { return }
	streamid := binary.BigEndian.Uint16(buf[:])

	switch msgtype {
	case MSG_OK: f = new(FrameOK)
	case MSG_FAILED: f = new(FrameFAILED)
	case MSG_AUTH: f = new(FrameAuth)
	case MSG_DATA: f = new(FrameData)
	case MSG_SYN: f = new(FrameSyn)
	case MSG_ACK: f = new(FrameAck)
	case MSG_FIN: f = new(FrameFin)
	case MSG_RST: f = new(FrameRst)
	}

	f.ReadFrame(length, streamid, r)
	return
}

func GetFrameBuf(msgtype uint8, length uint16, streamid uint16) (buf []byte, err error) {
	buf = make([]byte, length + 5)
	buf[0] = byte(msgtype)
	binary.BigEndian.PutUint16(buf[1:3], length)
	binary.BigEndian.PutUint16(buf[3:5], streamid)
	return
}

type FrameOK struct {
	streamid uint16
}

func (f *FrameOK) ReadFrame (length uint16, streamid uint16, r io.Reader) (err error) {
	if length != 0 {
		return errors.New("frame ok with length not 0")
	}
	f.streamid = streamid
	return
}

func (f *FrameOK) WriteFrame (w io.Writer) (err error) {
	buf, err := GetFrameBuf(MSG_OK, 0, f.streamid)
	w.Write(buf)
	return
}

type FrameFAILED struct {
	streamid uint16
	errno uint32
}

func (f *FrameFAILED) ReadFrame (length uint16, streamid uint16, r io.Reader) (err error) {
	if length != 4 {
		return errors.New("frame ok with length not 0")
	}
	f.streamid = streamid

	var buf [4]byte
	_, err = r.Read(buf[:])
	if err != nil {
		return
	}
	
	f.errno = binary.BigEndian.Uint32(buf[:])
	return
}

func (f *FrameFAILED) WriteFrame (w io.Writer) (err error) {
	buf, err := GetFrameBuf(MSG_FAILED, 4, f.streamid)
	binary.BigEndian.PutUint32(buf[5:9], f.errno)
	w.Write(buf)
	return
}

type FrameAuth struct {
	streamid uint16
	username string
	password string
}

func (f *FrameAuth) ReadFrame (length uint16, streamid uint16, r io.Reader) (err error) {
	return
}

func (f *FrameAuth) WriteFrame (w io.Writer) (err error) {
	return
}

type FrameData struct {
	streamid uint16
	data []byte
}

func (f *FrameData) ReadFrame (length uint16, streamid uint16, r io.Reader) (err error) {
	return
}

func (f *FrameData) WriteFrame (w io.Writer) (err error) {
	return
}

const (
	ADDR_IPV4 = iota
	ADDR_IPV6
	ADDR_HOSTNAME
)

type FrameSyn struct {
	streamid uint16
	target string
}

func (fs *FrameSyn) ReadFrame (length uint16, streamid uint16, r io.Reader) (err error) {
	var buf [2]byte

	_, err = r.Read(buf[:1])
	if err != nil { return }
	addrtype := uint8(buf[0])

	_, err = r.Read(buf[:])
	if err != nil { return }
	_ = binary.BigEndian.Uint16(buf[:])

	switch addrtype {
	case ADDR_IPV4:
	case ADDR_IPV6:
	case ADDR_HOSTNAME:
	}
	
	return 
}

func (f *FrameSyn) WriteFrame (w io.Writer) (err error) {
	return
}

func (f *FrameSyn) GetTcpAddr () (addr net.TCPAddr, err error) {
	return
}

type FrameAck struct {
	streamid uint16
	move_window uint32
}

func (f *FrameAck) ReadFrame (length uint16, streamid uint16, r io.Reader) (err error) {
	return
}

func (f *FrameAck) WriteFrame (w io.Writer) (err error) {
	return
}

type FrameFin struct {
	streamid uint16
}

func (f *FrameFin) ReadFrame (length uint16, streamid uint16, r io.Reader) (err error) {
	return
}

func (f *FrameFin) WriteFrame (w io.Writer) (err error) {
	return
}

type FrameRst struct {
	streamid uint16
}

func (f *FrameRst) ReadFrame (length uint16, streamid uint16, r io.Reader) (err error) {
	return
}

func (f *FrameRst) WriteFrame (w io.Writer) (err error) {
	return
}
