package msocks

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
)

// TODO: compressed session?

const (
	MSG_UNKNOWN = iota
	MSG_OK
	MSG_FAILED
	MSG_AUTH
	MSG_DATA
	MSG_SYN
	MSG_ACK
	MSG_FIN
	MSG_RST
	// MSG_DNS
	// MSG_ADDR
	MSG_PING
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
	GetStreamid() uint16
	Packed() (buf *bufio.Buffer, err error)
	Unpack(r io.Reader) error
	Debug()
}

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
	case MSG_RST:
		f = &FrameRst{FrameBase: *fb}
	// case MSG_DNS:
	// 	f = &FrameDns{FrameBase: *fb}
	// case MSG_ADDR:
	// 	f = &FrameAddr{FrameBase: *fb}
	case MSG_PING:
		f = &FramePing{FrameBase: *fb}
	}
	err = f.Unpack(r)
	return
}

type FrameBase struct {
	Type     uint8
	Length   uint16
	Streamid uint16
}

func (f *FrameBase) GetStreamid() uint16 {
	return f.Streamid
}

func (f *FrameBase) Packed() (buf *bytes.Buffer, err error) {
	buf = bytes.NewBuffer(nil)
	buf.Grow(int(5 + f.Length))
	binary.Write(buf, binary.BigEndian, f)
	return
}

func (f *FrameBase) Unpack(r io.Reader) (err error) {
	err = binary.Read(r, binary.BigEndian, f)
	return
}

func (f *FrameBase) Debug() {
	log.Debug("get package: type(%d), stream(%d), len(%d).",
		f.Type, f.Streamid, f.Length)
}

type FrameOK struct {
	FrameBase
}

func NewFrameNoParam(ftype uint8, streamid uint16) (b []byte) {
	f := &FrameBase{
		Type:     ftype,
		Streamid: streamid,
		Length:   0,
	}
	buf := f.Packed()
	return buf.Bytes()
}

func (f *FrameOK) Unpack(r io.Reader) (err error) {
	if f.Length != 0 {
		err = errors.New("frame ok with length not 0.")
	}
	return
}

type FrameFAILED struct {
	FrameBase
	Errno uint32
}

func NewFrameOneInt(ftype uint8, streamid uint16, i uint32) (b []byte) {
	f := &FrameBase{
		Type:     ftype,
		Streamid: streamid,
		Length:   4,
	}
	buf := f.Packed()
	binary.Write(buf, binary.BigEndian, i)
	return buf.Bytes()
}

func (f *FrameFAILED) Unpack(r io.Reader) (err error) {
	err = binary.Read(r, binary.BigEndian, &f.Errno)
	if err != nil {
		return
	}

	if f.Length != 4 {
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

	buf := f.Packed()

	err = WriteString(buf, username)
	if err != nil {
		return
	}

	err = WriteString(buf, password)
	if err != nil {
		return
	}

	return buf.Bytes(), nil
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

	if f.Length != uint16(len(f.Username)+len(f.Password)+4) {
		err = errors.New("frame auth length not match.")
	}
	return
}

type FrameData struct {
	FrameBase
	Data []byte
}

func NewFrameData(streamid uint16, data []byte) (b []byte, err error) {
	f := &FrameBase{
		Type:     MSG_DATA,
		Streamid: streamid,
		Length:   uint16(len(data)),
	}
	buf := f.Packed()

	_, err = buf.Write(data)
	if err != nil {
		return
	}

	return buf.Bytes(), nil
}

func (f *FrameData) Unpack(r io.Reader) (err error) {
	f.Data = make([]byte, f.Length)
	_, err = io.ReadFull(r, f.Data)
	return
}

type FrameSyn struct {
	FrameBase
	Address string
}

func NewFrameOneString(ftype uint8, streamid uint16, s string) (
	b []byte, err error) {
	f := &FrameBase{
		Type:     ftype,
		Streamid: streamid,
		Length:   uint16(len(s) + 2),
	}
	buf := f.Packed()

	err = WriteString(buf, s)
	if err != nil {
		return
	}

	return buf.Bytes(), nil
}

func (f *FrameSyn) Unpack(r io.Reader) (err error) {
	f.Address, err = ReadString(r)
	if err != nil {
		return
	}

	if f.Length != uint16(len(f.Address)+2) {
		err = errors.New("frame sync length not match.")
	}
	return
}

func (f *FrameSyn) Debug() {
	log.Debug("get package syn: stream(%d), len(%d), addr(%s).",
		f.Streamid, f.Length, f.Address)
}

type FrameAck struct {
	FrameBase
	Window uint32
}

func (f *FrameAck) Unpack(r io.Reader) (err error) {
	err = binary.Read(r, binary.BigEndian, &f.Window)
	if err != nil {
		return
	}

	if f.Length != 4 {
		err = errors.New("frame ack with length not 4.")
		return
	}
	return
}

func (f *FrameAck) Debug() {
	log.Debug("get package ack: stream(%d), len(%d), window(%d).",
		f.Streamid, f.Length, f.Window)
}

type FrameFin struct {
	FrameBase
}

func (f *FrameFin) Unpack(r io.Reader) (err error) {
	if f.Length != 0 {
		return errors.New("frame fin with length not 0.")
	}
	return
}

type FrameRst struct {
	FrameBase
}

func (f *FrameRst) Unpack(r io.Reader) (err error) {
	if f.Length != 0 {
		return errors.New("frame rst with length not 0.")
	}
	return
}

type FrameDns struct {
	FrameBase
	Hostname string
}

func (f *FrameDns) Unpack(r io.Reader) (err error) {
	f.Hostname, err = ReadString(r)
	if err != nil {
		return
	}

	if f.Length != uint16(len(f.Hostname)+2) {
		err = errors.New("frame dns length not match.")
	}
	return
}

func (f *FrameDns) Debug() {
	log.Debug("get package dns: stream(%d), len(%d), host(%s).",
		f.Streamid, f.Length, f.Hostname)
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
	buf := f.Packed()

	for _, o := range ipaddr {
		n := uint8(len(o))
		binary.Write(buf, binary.BigEndian, n)

		_, err = buf.Write(o)
		if err != nil {
			return
		}
	}

	return buf.Bytes(), nil
}

func (f *FrameAddr) Unpack(r io.Reader) (err error) {
	var n uint8
	size := uint16(0)

	for size < f.Length {
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

	if f.Length != size {
		return errors.New("frame addr length not match.")
	}
	return
}

type FramePing struct {
	FrameBase
}

func (f *FramePing) Unpack(r io.Reader) (err error) {
	if f.Length != 0 {
		return errors.New("frame ping with length not 0.")
	}
	return
}
