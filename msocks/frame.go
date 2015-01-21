package msocks

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	MSG_UNKNOWN = iota
	MSG_RESULT
	MSG_AUTH
	MSG_DATA
	MSG_SYN
	MSG_WND
	MSG_FIN
	MSG_RST
	MSG_PING
	MSG_DNS
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
	GetSize() uint16
	Packed() (buf *bytes.Buffer, err error)
	Unpack(r io.Reader) error
	Debug(prefix string)
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
	case MSG_RESULT:
		f = &FrameResult{FrameBase: *fb}
	case MSG_AUTH:
		f = &FrameAuth{FrameBase: *fb}
	case MSG_DATA:
		f = &FrameData{FrameBase: *fb}
	case MSG_SYN:
		f = &FrameSyn{FrameBase: *fb}
	case MSG_WND:
		f = &FrameWnd{FrameBase: *fb}
	case MSG_FIN:
		f = &FrameFin{FrameBase: *fb}
	case MSG_RST:
		f = &FrameRst{FrameBase: *fb}
	case MSG_PING:
		f = &FramePing{FrameBase: *fb}
	case MSG_DNS:
		f = &FrameDns{FrameBase: *fb}
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

func (f *FrameBase) GetSize() uint16 {
	return f.Length
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

func (f *FrameBase) Debug(prefix string) {
	log.Debug("%sframe: type(%d), stream(%d), len(%d).",
		prefix, f.Type, f.Streamid, f.Length)
}

type FrameResult struct {
	FrameBase
	Errno uint32
}

func NewFrameResult(streamid uint16, errno uint32) (f *FrameResult) {
	return &FrameResult{
		FrameBase: FrameBase{
			Type:     MSG_RESULT,
			Streamid: streamid,
			Length:   4,
		},
		Errno: errno,
	}
}
func (f *FrameResult) Packed() (buf *bytes.Buffer, err error) {
	buf, err = f.FrameBase.Packed()
	if err != nil {
		return
	}
	binary.Write(buf, binary.BigEndian, f.Errno)
	return
}

func (f *FrameResult) Unpack(r io.Reader) (err error) {
	err = binary.Read(r, binary.BigEndian, &f.Errno)
	if err != nil {
		return
	}

	if f.Length != 4 {
		err = errors.New("frame result with length not 4.")
		return
	}
	return
}

type FrameAuth struct {
	FrameBase
	Username string
	Password string
}

func NewFrameAuth(streamid uint16, username, password string) (f *FrameAuth) {
	return &FrameAuth{
		FrameBase: FrameBase{
			Type:     MSG_AUTH,
			Streamid: streamid,
			Length:   uint16(len(username) + len(password) + 4),
		},
		Username: username,
		Password: password,
	}
}

func (f *FrameAuth) Packed() (buf *bytes.Buffer, err error) {
	buf, err = f.FrameBase.Packed()
	if err != nil {
		return
	}
	err = WriteString(buf, f.Username)
	if err != nil {
		return
	}
	err = WriteString(buf, f.Password)
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

	if f.Length != uint16(len(f.Username)+len(f.Password)+4) {
		err = errors.New("frame auth length not match.")
	}
	return
}

type FrameData struct {
	FrameBase
	Data []byte
}

func NewFrameData(streamid uint16, data []byte) (f *FrameData) {
	return &FrameData{
		FrameBase: FrameBase{
			Type:     MSG_DATA,
			Streamid: streamid,
			Length:   uint16(len(data)),
		},
		Data: data,
	}
}

func (f *FrameData) Packed() (buf *bytes.Buffer, err error) {
	buf, err = f.FrameBase.Packed()
	if err != nil {
		return
	}
	_, err = buf.Write(f.Data)
	return
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

func NewFrameSyn(streamid uint16, addr string) (f *FrameSyn) {
	return &FrameSyn{
		FrameBase: FrameBase{
			Type:     MSG_SYN,
			Streamid: streamid,
			Length:   uint16(len(addr) + 2),
		},
		Address: addr,
	}
}
func (f *FrameSyn) Packed() (buf *bytes.Buffer, err error) {
	buf, err = f.FrameBase.Packed()
	if err != nil {
		return
	}
	err = WriteString(buf, f.Address)
	return
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

func (f *FrameSyn) Debug(prefix string) {
	log.Debug("%sframe syn: stream(%d), len(%d), addr(%s).",
		prefix, f.Streamid, f.Length, f.Address)
}

type FrameWnd struct {
	FrameBase
	Window uint32
}

func NewFrameWnd(streamid uint16, window uint32) (f *FrameWnd) {
	return &FrameWnd{
		FrameBase: FrameBase{
			Type:     MSG_WND,
			Streamid: streamid,
			Length:   4,
		},
		Window: window,
	}
}
func (f *FrameWnd) Packed() (buf *bytes.Buffer, err error) {
	buf, err = f.FrameBase.Packed()
	if err != nil {
		return
	}
	binary.Write(buf, binary.BigEndian, f.Window)
	return
}

func (f *FrameWnd) Unpack(r io.Reader) (err error) {
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

func (f *FrameWnd) Debug(prefix string) {
	log.Debug("%sframe wnd: stream(%d), len(%d), window(%d).",
		prefix, f.Streamid, f.Length, f.Window)
}

type FrameFin struct {
	FrameBase
}

func NewFrameFin(streamid uint16) (f *FrameFin) {
	return &FrameFin{
		FrameBase: FrameBase{
			Type:     MSG_FIN,
			Streamid: streamid,
			Length:   0,
		},
	}
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

func NewFrameRst(streamid uint16) (f *FrameRst) {
	return &FrameRst{
		FrameBase: FrameBase{
			Type:     MSG_RST,
			Streamid: streamid,
			Length:   0,
		},
	}
}

func (f *FrameRst) Unpack(r io.Reader) (err error) {
	if f.Length != 0 {
		return errors.New("frame rst with length not 0.")
	}
	return
}

type FramePing struct {
	FrameBase
}

func NewFramePing() (f *FramePing) {
	return &FramePing{
		FrameBase: FrameBase{
			Type:     MSG_PING,
			Streamid: 0,
			Length:   0,
		},
	}
}

func (f *FramePing) Unpack(r io.Reader) (err error) {
	if f.Length != 0 {
		return errors.New("frame ping with length not 0.")
	}
	return
}

type FrameDns struct {
	FrameBase
	Data []byte
}

func NewFrameDns(streamid uint16, data []byte) (f *FrameDns) {
	return &FrameDns{
		FrameBase: FrameBase{
			Type:     MSG_DNS,
			Streamid: streamid,
			Length:   uint16(len(data)),
		},
		Data: data,
	}
}

func (f *FrameDns) Packed() (buf *bytes.Buffer, err error) {
	buf, err = f.FrameBase.Packed()
	if err != nil {
		return
	}
	_, err = buf.Write(f.Data)
	return
}

func (f *FrameDns) Unpack(r io.Reader) (err error) {
	f.Data = make([]byte, f.Length)
	_, err = io.ReadFull(r, f.Data)
	return
}

type FrameSender interface {
	SendFrame(Frame) error
	CloseFrame() error
}

type ChanFrameSender chan Frame

func CreateChanFrameSender(n int) ChanFrameSender {
	return make(chan Frame, n)
}

func (cfs *ChanFrameSender) SendFrame(f Frame) (err error) {
	*cfs <- f
	return
}

func (cfs *ChanFrameSender) CloseFrame() (err error) {
	// close(*cfs)
	return
}

func (cfs *ChanFrameSender) RecvWithTimeout(t time.Duration) (f Frame, err error) {
	var ok bool
	ch_timeout := time.After(t)
	select {
	case f, ok = <-*cfs:
		if !ok {
			return ERR_CLOSED
		}
	case <-ch_timeout:
		return ERR_TIMEOUT
	}
	return
}
