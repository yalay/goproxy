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
		logger.Err(err)
		return
	}
	buf := make([]byte, length)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		logger.Err(err)
		return
	}
	return string(buf), nil
}

func WriteString(w io.Writer, s string) (err error) {
	err = binary.Write(w, binary.BigEndian, uint16(len(s)))
	if err != nil {
		logger.Err(err)
		return
	}
	_, err = w.Write([]byte(s))
	if err != nil {
		logger.Err(err)
	}
	return
}

type Frame interface {
	Packed() (*bytes.Buffer, error)
	Unpack(r io.Reader) error
}

func WriteFrame(w io.Writer, f Frame) (err error) {
	buf, err := f.Packed()
	if err != nil {
		return
	}
	b := buf.Bytes()
	n, err := w.Write(b)
	if err != nil {
		return
	}
	if n != len(b) {
		err = io.ErrShortWrite
		logger.Err(err)
		return
	}
	return
}

func ReadFrame(r io.Reader) (f Frame, err error) {
	fb := new(FrameBase)
	err = binary.Read(r, binary.BigEndian, fb)
	if err != nil {
		logger.Err(err)
		return
	}

	switch fb.Type {
	default:
		err = fmt.Errorf("unknown frame type: %d, with length: %d, streamid: %d.", fb.Type, fb.Length, fb.Streamid)
		logger.Err(err)
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
	err = binary.Write(buf, binary.BigEndian, f)
	if err != nil {
		logger.Err(err)
	}
	return
}

func (f *FrameBase) Unpack(r io.Reader) (err error) {
	err = binary.Read(r, binary.BigEndian, f)
	if err != nil {
		logger.Err(err)
		return
	}
	return
}

type FrameOK struct {
	FrameBase
}

func NewFrameOK(streamid uint16) (f *FrameOK) {
	f = &FrameOK{
		FrameBase: FrameBase{
			Type:     MSG_OK,
			Streamid: streamid,
			Length:   0,
		},
	}
	return
}

func (f *FrameOK) Unpack(r io.Reader) (err error) {
	if f.FrameBase.Length != 0 {
		err = errors.New("frame ok with length not 0.")
		logger.Err(err)
		return
	}
	return
}

type FrameFAILED struct {
	FrameBase
	Errno uint32
}

func NewFrameFAILED(streamid uint16, errno uint32) (f *FrameFAILED) {
	f = &FrameFAILED{
		FrameBase: FrameBase{
			Type:     MSG_FAILED,
			Streamid: streamid,
			Length:   4,
		},
		Errno: errno,
	}
	return
}

func (f *FrameFAILED) Packed() (buf *bytes.Buffer, err error) {
	buf = bytes.NewBuffer(nil)
	err = binary.Write(buf, binary.BigEndian, f)
	if err != nil {
		logger.Err(err)
	}
	return
}

func (f *FrameFAILED) Unpack(r io.Reader) (err error) {
	err = binary.Read(r, binary.BigEndian, &f.Errno)
	if err != nil {
		logger.Err(err)
		return
	}

	if f.FrameBase.Length != 4 {
		err = errors.New("frame failed with length not 4.")
		logger.Err(err)
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
	f = &FrameAuth{
		FrameBase: FrameBase{
			Type:     MSG_AUTH,
			Streamid: streamid,
			Length:   uint16(len(username) + len(password) + 4),
		},
		Username: username,
		Password: password,
	}
	return
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

	if f.FrameBase.Length != uint16(len(f.Username)+len(f.Password)+4) {
		err = errors.New("frame auth length not match.")
		logger.Err(err)
	}
	return
}

type FrameData struct {
	FrameBase
	Data []byte
	Buf  []byte
}

func NewFrameData(streamid uint16, data []byte) (f *FrameData) {
	f = &FrameData{
		FrameBase: FrameBase{
			Type:     MSG_DATA,
			Streamid: streamid,
			Length:   uint16(len(data)),
		},
		Data: data,
		Buf:  data,
	}
	return
}

func (f *FrameData) Packed() (buf *bytes.Buffer, err error) {
	buf, err = f.FrameBase.Packed()
	if err != nil {
		return
	}

	n, err := buf.Write(f.Data)
	if err != nil {
		logger.Err(err)
		return
	}
	if n != len(f.Data) {
		err = io.ErrShortWrite
		logger.Err(err)
		return
	}

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
	if err != nil {
		logger.Err(err)
	}
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

func NewFrameSyn(streamid uint16, address string) (f *FrameSyn) {
	f = &FrameSyn{
		FrameBase: FrameBase{
			Type:     MSG_SYN,
			Streamid: streamid,
			Length:   uint16(len(address) + 2),
		},
		Address: address,
	}
	return
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

	if f.FrameBase.Length != uint16(len(f.Address)+2) {
		err = errors.New("frame sync length not match.")
		logger.Err(err)
	}
	return
}

type FrameAck struct {
	FrameBase
	Window uint32
}

func NewFrameAck(streamid uint16, window uint32) (f *FrameAck) {
	f = &FrameAck{
		FrameBase: FrameBase{
			Type:     MSG_ACK,
			Streamid: streamid,
			Length:   4,
		},
		Window: window,
	}
	return
}

func (f *FrameAck) Packed() (buf *bytes.Buffer, err error) {
	buf = bytes.NewBuffer(nil)
	err = binary.Write(buf, binary.BigEndian, f)
	if err != nil {
		logger.Err(err)
	}
	return
}

func (f *FrameAck) Unpack(r io.Reader) (err error) {
	err = binary.Read(r, binary.BigEndian, &f.Window)
	if err != nil {
		logger.Err(err)
		return
	}

	if f.FrameBase.Length != 4 {
		err = errors.New("frame ack with length not 4.")
		logger.Err(err)
		return
	}
	return
}

type FrameFin struct {
	FrameBase
}

func NewFrameFin(streamid uint16) (f *FrameFin) {
	f = &FrameFin{
		FrameBase: FrameBase{
			Type:     MSG_FIN,
			Streamid: streamid,
			Length:   0,
		},
	}
	return
}

func (f *FrameFin) Unpack(r io.Reader) (err error) {
	if f.FrameBase.Length != 0 {
		err = errors.New("frame fin with length not 0.")
		logger.Err(err)
		return
	}
	return
}

type FrameDns struct {
	FrameBase
	Hostname string
}

func NewFrameDns(streamid uint16, hostname string) (f *FrameDns) {
	f = &FrameDns{
		FrameBase: FrameBase{
			Type:     MSG_DNS,
			Streamid: streamid,
			Length:   uint16(len(hostname) + 2),
		},
		Hostname: hostname,
	}
	return
}

func (f *FrameDns) Packed() (buf *bytes.Buffer, err error) {
	buf, err = f.FrameBase.Packed()
	if err != nil {
		return
	}

	err = WriteString(buf, f.Hostname)
	return
}

func (f *FrameDns) Unpack(r io.Reader) (err error) {
	f.Hostname, err = ReadString(r)
	if err != nil {
		return
	}

	if f.FrameBase.Length != uint16(len(f.Hostname)+2) {
		err = errors.New("frame dns length not match.")
		logger.Err(err)
	}
	return
}

type FrameAddr struct {
	FrameBase
	Ipaddr []net.IP
}

func NewFrameAddr(streamid uint16, ipaddr []net.IP) (f *FrameAddr) {
	size := uint16(0)
	for _, o := range ipaddr {
		size += uint16(len(o) + 1)
	}
	f = &FrameAddr{
		FrameBase: FrameBase{
			Type:     MSG_ADDR,
			Streamid: streamid,
			Length:   size,
		},
		Ipaddr: ipaddr,
	}
	return
}

func (f *FrameAddr) Packed() (buf *bytes.Buffer, err error) {
	buf, err = f.FrameBase.Packed()
	if err != nil {
		return
	}

	for _, o := range f.Ipaddr {
		n := uint8(len(o))
		err = binary.Write(buf, binary.BigEndian, n)
		if err != nil {
			return
		}

		_, err = buf.Write(o)
		if err != nil {
			logger.Err(err)
			return
		}
	}

	return
}

func (f *FrameAddr) Unpack(r io.Reader) (err error) {
	var n uint8
	size := uint16(0)

	for size < f.FrameBase.Length {
		err = binary.Read(r, binary.BigEndian, &n)
		if err != nil {
			logger.Err(err)
			return
		}

		ip := make([]byte, n)
		_, err = io.ReadFull(r, ip)
		if err != nil {
			logger.Err(err)
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

// func ReadFrame(r io.Reader) (f Frame, err error) {
// 	var buf [2]byte
// 	_, err = r.Read(buf[:1])
// 	if err != nil {
// 		logger.Err(err)
// 		return
// 	}
// 	msgtype := uint8(buf[0])

// 	_, err = r.Read(buf[:])
// 	if err != nil {
// 		logger.Err(err)
// 		return
// 	}
// 	length := binary.BigEndian.Uint16(buf[:])

// 	_, err = r.Read(buf[:])
// 	if err != nil {
// 		logger.Err(err)
// 		return
// 	}
// 	streamid := binary.BigEndian.Uint16(buf[:])

// 	switch msgtype {
// 	default:
// 		err = fmt.Errorf("unknown frame type: %d, with length: %d, streamid: %d.", msgtype, length, streamid)
// 		logger.Err(err)
// 		return nil, err
// 	case MSG_OK:
// 		f = new(FrameOK)
// 	case MSG_FAILED:
// 		f = new(FrameFAILED)
// 	case MSG_AUTH:
// 		f = new(FrameAuth)
// 	case MSG_DATA:
// 		f = new(FrameData)
// 	case MSG_SYN:
// 		f = new(FrameSyn)
// 	case MSG_ACK:
// 		f = new(FrameAck)
// 	case MSG_FIN:
// 		f = new(FrameFin)
// 	case MSG_DNS:
// 		f = new(FrameDns)
// 	case MSG_ADDR:
// 		f = new(FrameAddr)
// 	}

// 	err = f.ReadFrame(length, streamid, r)
// 	if err != nil {
// 		logger.Err(err)
// 	}
// 	return f, err
// }

// func GetFrameBuf(w io.Writer, msgtype uint8, length uint16, streamid uint16) (buf *bufio.Writer, err error) {
// 	buf = bufio.NewWriterSize(w, int(length+5))
// 	err = binary.Write(buf, binary.BigEndian, msgtype)
// 	if err != nil {
// 		logger.Err(err)
// 		return
// 	}
// 	err = binary.Write(buf, binary.BigEndian, length)
// 	if err != nil {
// 		logger.Err(err)
// 		return
// 	}
// 	err = binary.Write(buf, binary.BigEndian, streamid)
// 	if err != nil {
// 		logger.Err(err)
// 		return
// 	}
// 	return
// }

// type FrameOK struct {
// 	streamid uint16
// }

// func (f *FrameOK) ReadFrame(length uint16, streamid uint16, r io.Reader) (err error) {
// 	if length != 0 {
// 		return errors.New("frame ok with length not 0")
// 	}
// 	f.streamid = streamid
// 	return
// }

// func (f *FrameOK) WriteFrame(w io.Writer) (err error) {
// 	buf, err := GetFrameBuf(w, MSG_OK, 0, f.streamid)
// 	if err != nil {
// 		return
// 	}
// 	defer buf.Flush()
// 	return
// }

// type FrameFAILED struct {
// 	streamid uint16
// 	errno    uint32
// }

// func (f *FrameFAILED) ReadFrame(length uint16, streamid uint16, r io.Reader) (err error) {
// 	if length != 4 {
// 		return errors.New("frame failed with length not 4")
// 	}
// 	f.streamid = streamid

// 	err = binary.Read(r, binary.BigEndian, &f.errno)
// 	if err != nil {
// 		return
// 	}
// 	return
// }

// func (f *FrameFAILED) WriteFrame(w io.Writer) (err error) {
// 	buf, err := GetFrameBuf(w, MSG_FAILED, 4, f.streamid)
// 	if err != nil {
// 		return
// 	}
// 	defer buf.Flush()
// 	err = binary.Write(buf, binary.BigEndian, f.errno)
// 	return
// }

// type FrameAuth struct {
// 	streamid uint16
// 	username string
// 	password string
// }

// func (f *FrameAuth) ReadFrame(length uint16, streamid uint16, r io.Reader) (err error) {
// 	f.streamid = streamid

// 	f.username, err = ReadString(r)
// 	if err != nil {
// 		return
// 	}
// 	f.password, err = ReadString(r)
// 	if err != nil {
// 		return
// 	}

// 	if int(length) != len(f.username)+len(f.password)+4 {
// 		return errors.New("frame auth length not match.")
// 	}
// 	return
// }

// func (f *FrameAuth) WriteFrame(w io.Writer) (err error) {
// 	buf, err := GetFrameBuf(w, MSG_AUTH,
// 		uint16(len(f.username)+len(f.password)+4), f.streamid)
// 	if err != nil {
// 		return
// 	}
// 	defer buf.Flush()
// 	err = WriteString(buf, f.username)
// 	if err != nil {
// 		return
// 	}
// 	return WriteString(buf, f.password)
// }

// type FrameData struct {
// 	streamid uint16
// 	data     []byte
// 	buf      []byte
// }

// func (f *FrameData) ReadFrame(length uint16, streamid uint16, r io.Reader) (err error) {
// 	f.streamid = streamid
// 	if length <= 1024 {
// 		f.buf = sutils.Klb.Get()
// 		f.data = f.buf[:length]
// 	} else {
// 		f.buf = make([]byte, length)
// 		f.data = f.buf
// 	}
// 	_, err = io.ReadFull(r, f.data)
// 	if err != nil {
// 		logger.Err(err)
// 	}
// 	return
// }

// func (f *FrameData) WriteFrame(w io.Writer) (err error) {
// 	buf, err := GetFrameBuf(w, MSG_DATA, uint16(len(f.data)), f.streamid)
// 	if err != nil {
// 		return
// 	}
// 	defer buf.Flush()
// 	_, err = buf.Write(f.data)
// 	if err != nil {
// 		logger.Err(err)
// 	}
// 	return
// }

// func (f *FrameData) Free() {
// 	if len(f.buf) == 1024 {
// 		sutils.Klb.Free(f.buf)
// 	}
// }

// type FrameSyn struct {
// 	streamid uint16
// 	address  string
// }

// func (f *FrameSyn) ReadFrame(length uint16, streamid uint16, r io.Reader) (err error) {
// 	f.streamid = streamid

// 	f.address, err = ReadString(r)
// 	if err != nil {
// 		return
// 	}
// 	if int(length) != len(f.address)+2 {
// 		return errors.New("frame syn length not match")
// 	}

// 	return
// }

// func (f *FrameSyn) WriteFrame(w io.Writer) (err error) {
// 	buf, err := GetFrameBuf(w, MSG_SYN, uint16(len(f.address)+2), f.streamid)
// 	if err != nil {
// 		return
// 	}
// 	defer buf.Flush()

// 	return WriteString(buf, f.address)
// }

// type FrameAck struct {
// 	streamid uint16
// 	window   uint32
// }

// func (f *FrameAck) ReadFrame(length uint16, streamid uint16, r io.Reader) (err error) {
// 	if length != 4 {
// 		return errors.New("frame fin with length not 4")
// 	}
// 	f.streamid = streamid
// 	err = binary.Read(r, binary.BigEndian, &f.window)
// 	return
// }

// func (f *FrameAck) WriteFrame(w io.Writer) (err error) {
// 	buf, err := GetFrameBuf(w, MSG_ACK, 4, f.streamid)
// 	if err != nil {
// 		return
// 	}
// 	defer buf.Flush()
// 	err = binary.Write(buf, binary.BigEndian, f.window)
// 	if err != nil {
// 		logger.Err(err)
// 	}
// 	return
// }

// type FrameFin struct {
// 	streamid uint16
// }

// func (f *FrameFin) ReadFrame(length uint16, streamid uint16, r io.Reader) (err error) {
// 	if length != 0 {
// 		return errors.New("frame fin with length not 0")
// 	}
// 	f.streamid = streamid
// 	return
// }

// func (f *FrameFin) WriteFrame(w io.Writer) (err error) {
// 	buf, err := GetFrameBuf(w, MSG_FIN, 0, f.streamid)
// 	if err != nil {
// 		return
// 	}
// 	defer buf.Flush()
// 	return
// }

// type FrameDns struct {
// 	streamid uint16
// 	hostname string
// }

// func (f *FrameDns) ReadFrame(length uint16, streamid uint16, r io.Reader) (err error) {
// 	f.streamid = streamid

// 	f.hostname, err = ReadString(r)
// 	if err != nil {
// 		return
// 	}

// 	if int(length) != len(f.hostname)+2 {
// 		return errors.New("frame dns length not match.")
// 	}
// 	return
// }

// func (f *FrameDns) WriteFrame(w io.Writer) (err error) {
// 	buf, err := GetFrameBuf(w, MSG_DNS,
// 		uint16(len(f.hostname)+2), f.streamid)
// 	if err != nil {
// 		return
// 	}
// 	defer buf.Flush()
// 	err = WriteString(buf, f.hostname)
// 	return
// }

// type FrameAddr struct {
// 	streamid uint16
// 	ipaddr   []net.IP
// }

// func (f *FrameAddr) ReadFrame(length uint16, streamid uint16, r io.Reader) (err error) {
// 	var n uint8
// 	size := uint16(0)
// 	f.streamid = streamid

// 	for size < length {
// 		err = binary.Read(r, binary.BigEndian, &n)
// 		if err != nil {
// 			logger.Err(err)
// 			return
// 		}

// 		ip := make([]byte, n)
// 		_, err = io.ReadFull(r, ip)
// 		if err != nil {
// 			logger.Err(err)
// 			return
// 		}

// 		f.ipaddr = append(f.ipaddr, ip)
// 		size += uint16(n + 1)
// 	}

// 	if length != size {
// 		return errors.New("frame addr length not match.")
// 	}
// 	return
// }

// func (f *FrameAddr) WriteFrame(w io.Writer) (err error) {
// 	size := uint16(0)
// 	for _, o := range f.ipaddr {
// 		size += uint16(len(o) + 1)
// 	}
// 	buf, err := GetFrameBuf(w, MSG_ADDR, size, f.streamid)
// 	defer buf.Flush()

// 	for _, o := range f.ipaddr {
// 		n := uint8(len(o))
// 		err = binary.Write(buf, binary.BigEndian, n)
// 		if err != nil {
// 			return
// 		}

// 		_, err = buf.Write(o)
// 		if err != nil {
// 			logger.Err(err)
// 			return
// 		}
// 	}

// 	return
// }
