package msocks

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sutils"
)

// TODO: compressed session?

const (
	MSG_OK = iota
	MSG_FAILED
	MSG_AUTH
	MSG_DATA
	MSG_SYN
	MSG_ACK
	MSG_FIN
	MSG_DNS
	MSG_ADDR
)

const (
	ERR_AUTH = iota
	ERR_IDEXIST
	ERR_CONNFAILED
)

func ReadString(r io.Reader) (s string, err error) {
	var length uint16
	err = binary.Read(r, binary.BigEndian, &length)
	if err != nil {
		return
	}
	buf := make([]byte, length)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return
	}
	return string(buf), nil
}

func WriteString(w io.Writer, s string) (err error) {
	err = binary.Write(w, binary.BigEndian, uint16(len(s)))
	if err != nil {
		return
	}
	_, err = w.Write([]byte(s))
	return
}

type Frame interface {
	// Packed() (*bytes.Buffer, error)
	Unpack(r io.Reader) error
}

// func WriteFrame(w io.Writer, f Frame) (err error) {
// 	buf, err := f.Packed()
// 	if err != nil {
// 		return
// 	}
// 	b := buf.Bytes()
// 	n, err := w.Write(b)
// 	if err != nil {
// 		return
// 	}
// 	if n != len(b) {
// 		err = io.ErrShortWrite
// 		return
// 	}
// 	return
// }

func ReadFrame(r io.Reader) (f Frame, err error) {
	fb := new(FrameBase)
	err = binary.Read(r, binary.BigEndian, fb)
	if err != nil {
		return
	}

	switch fb.Type {
	default:
		err = fmt.Errorf("unknown frame: type(%d), length(%d), streamid(%d).",
			fb.Type, fb.Length, fb.Streamid)
		return
	case MSG_OK:
		f = &FrameOK{FrameBase: *fb}
	case MSG_FAILED:
		f = &FrameFAILED{FrameBase: *fb}
	case MSG_AUTH:
		f = &FrameAuth{FrameBase: *fb}
	case MSG_DATA:
		f = &FrameData{FrameBase: *fb}
	case MSG_SYN:
		f = &FrameSyn{FrameBase: *fb}
	case MSG_ACK:
		f = &FrameAck{FrameBase: *fb}
	case MSG_FIN:
		f = &FrameFin{FrameBase: *fb}
	case MSG_DNS:
		f = &FrameDns{FrameBase: *fb}
	case MSG_ADDR:
		f = &FrameAddr{FrameBase: *fb}
	}
	err = f.Unpack(r)
	return
}

type FrameBase struct {
	Type     uint8
	Length   uint16
	Streamid uint16
}

func (f *FrameBase) Packed() (buf *bytes.Buffer, err error) {
	buf = bytes.NewBuffer(nil)
	buf.Grow(int(5 + f.Length))
	err = binary.Write(buf, binary.BigEndian, f)
	return
}

func (f *FrameBase) Unpack(r io.Reader) (err error) {
	err = binary.Read(r, binary.BigEndian, f)
	return
}

type FrameOK struct {
	FrameBase
}

func NewFrameOK(streamid uint16) (b []byte, err error) {
	f := &FrameBase{
		Type:     MSG_OK,
		Streamid: streamid,
		Length:   0,
	}
	buf, err := f.Packed()
	if err != nil {
		return
	}
	b = buf.Bytes()
	return
}

func (f *FrameOK) Unpack(r io.Reader) (err error) {
	if f.FrameBase.Length != 0 {
		err = errors.New("frame ok with length not 0.")
		return
	}
	return
}

type FrameFAILED struct {
	FrameBase
	Errno uint32
}

func NewFrameFAILED(streamid uint16, errno uint32) (b []byte, err error) {
	f := &FrameFAILED{
		FrameBase: FrameBase{
			Type:     MSG_FAILED,
			Streamid: streamid,
			Length:   4,
		},
		Errno: errno,
	}
	buf := bytes.NewBuffer(nil)
	buf.Grow(9)
	err = binary.Write(buf, binary.BigEndian, f)
	if err != nil {
		return
	}
	b = buf.Bytes()
	return
}

func (f *FrameFAILED) Unpack(r io.Reader) (err error) {
	err = binary.Read(r, binary.BigEndian, &f.Errno)
	if err != nil {
		return
	}

	if f.FrameBase.Length != 4 {
		err = errors.New("frame failed with length not 4.")
		return
	}
	return
}

type FrameAuth struct {
	FrameBase
	Username string
	Password string
}

func NewFrameAuth(streamid uint16, username, password string) (b []byte, err error) {
	f := &FrameBase{
		Type:     MSG_AUTH,
		Streamid: streamid,
		Length:   uint16(len(username) + len(password) + 4),
	}

	buf, err := f.Packed()
	if err != nil {
		return
	}

	err = WriteString(buf, username)
	if err != nil {
		return
	}

	err = WriteString(buf, password)
	if err != nil {
		return
	}

	b = buf.Bytes()
	return
}

func (f *FrameAuth) Unpack(r io.Reader) (err error) {
	f.Username, err = ReadString(r)
	if err != nil {
		return
	}

	f.Password, err = ReadString(r)
	if err != nil {
		return
	}

	if f.FrameBase.Length != uint16(len(f.Username)+len(f.Password)+4) {
		err = errors.New("frame auth length not match.")
	}
	return
}

type FrameData struct {
	FrameBase
	Data []byte
	Buf  []byte
}

func NewFrameData(streamid uint16, data []byte) (b []byte, err error) {
	f := &FrameBase{
		Type:     MSG_DATA,
		Streamid: streamid,
		Length:   uint16(len(data)),
	}
	buf, err := f.Packed()
	if err != nil {
		return
	}

	n, err := buf.Write(data)
	if err != nil {
		return
	}
	if n != len(data) {
		err = io.ErrShortWrite
		return
	}

	b = buf.Bytes()
	return
}

func (f *FrameData) Unpack(r io.Reader) (err error) {
	if f.FrameBase.Length <= 1024 {
		f.Buf = sutils.Klb.Get()
		f.Data = f.Buf[:f.FrameBase.Length]
	} else {
		f.Buf = make([]byte, f.FrameBase.Length)
		f.Data = f.Buf
	}
	_, err = io.ReadFull(r, f.Data)
	return
}

func (f *FrameData) Free() {
	if len(f.Buf) == 1024 {
		sutils.Klb.Free(f.Buf)
	}
}

type FrameSyn struct {
	FrameBase
	Address string
}

func NewFrameSyn(streamid uint16, address string) (b []byte, err error) {
	f := &FrameBase{
		Type:     MSG_SYN,
		Streamid: streamid,
		Length:   uint16(len(address) + 2),
	}
	buf, err := f.Packed()
	if err != nil {
		return
	}

	err = WriteString(buf, address)
	if err != nil {
		return
	}

	b = buf.Bytes()
	return
}

func (f *FrameSyn) Packed() (buf *bytes.Buffer, err error) {
	return
}

func (f *FrameSyn) Unpack(r io.Reader) (err error) {
	f.Address, err = ReadString(r)
	if err != nil {
		return
	}

	if f.FrameBase.Length != uint16(len(f.Address)+2) {
		err = errors.New("frame sync length not match.")
	}
	return
}

type FrameAck struct {
	FrameBase
	Window uint32
}

func NewFrameAck(streamid uint16, window uint32) (b []byte, err error) {
	f := &FrameBase{
		Type:     MSG_ACK,
		Streamid: streamid,
		Length:   4,
	}
	buf := bytes.NewBuffer(nil)
	buf.Grow(9)
	err = binary.Write(buf, binary.BigEndian, f)
	if err != nil {
		return
	}

	err = binary.Write(buf, binary.BigEndian, window)
	if err != nil {
		return
	}

	b = buf.Bytes()
	return
}

func (f *FrameAck) Unpack(r io.Reader) (err error) {
	err = binary.Read(r, binary.BigEndian, &f.Window)
	if err != nil {
		return
	}

	if f.FrameBase.Length != 4 {
		err = errors.New("frame ack with length not 4.")
		return
	}
	return
}

type FrameFin struct {
	FrameBase
}

func NewFrameFin(streamid uint16) (b []byte, err error) {
	f := &FrameBase{
		Type:     MSG_FIN,
		Streamid: streamid,
		Length:   0,
	}
	buf, err := f.Packed()
	if err != nil {
		return
	}
	b = buf.Bytes()
	return
}

func (f *FrameFin) Unpack(r io.Reader) (err error) {
	if f.FrameBase.Length != 0 {
		return errors.New("frame fin with length not 0.")
	}
	return
}

type FrameDns struct {
	FrameBase
	Hostname string
}

func NewFrameDns(streamid uint16, hostname string) (b []byte, err error) {
	f := &FrameBase{
		Type:     MSG_DNS,
		Streamid: streamid,
		Length:   uint16(len(hostname) + 2),
	}
	buf, err := f.Packed()
	if err != nil {
		return
	}

	err = WriteString(buf, hostname)
	if err != nil {
		return
	}

	b = buf.Bytes()
	return
}

func (f *FrameDns) Unpack(r io.Reader) (err error) {
	f.Hostname, err = ReadString(r)
	if err != nil {
		return
	}

	if f.FrameBase.Length != uint16(len(f.Hostname)+2) {
		err = errors.New("frame dns length not match.")
	}
	return
}

type FrameAddr struct {
	FrameBase
	Ipaddr []net.IP
}

func NewFrameAddr(streamid uint16, ipaddr []net.IP) (b []byte, err error) {
	size := uint16(0)
	for _, o := range ipaddr {
		size += uint16(len(o) + 1)
	}
	f := &FrameBase{
		Type:     MSG_ADDR,
		Streamid: streamid,
		Length:   size,
	}
	buf, err := f.Packed()
	if err != nil {
		return
	}

	for _, o := range ipaddr {
		n := uint8(len(o))
		err = binary.Write(buf, binary.BigEndian, n)
		if err != nil {
			return
		}

		_, err = buf.Write(o)
		if err != nil {
			return
		}
	}

	b = buf.Bytes()
	return
}

func (f *FrameAddr) Unpack(r io.Reader) (err error) {
	var n uint8
	size := uint16(0)

	for size < f.FrameBase.Length {
		err = binary.Read(r, binary.BigEndian, &n)
		if err != nil {
			return
		}

		ip := make([]byte, n)
		_, err = io.ReadFull(r, ip)
		if err != nil {
			return
		}

		f.Ipaddr = append(f.Ipaddr, ip)
		size += uint16(n + 1)
	}

	if f.FrameBase.Length != size {
		return errors.New("frame addr length not match.")
	}
	return
}
