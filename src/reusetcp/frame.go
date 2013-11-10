package reusetcp

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"net"
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
	ReadFrame(length uint16, streamid uint16, r io.Reader) (err error)
	WriteFrame(w io.Writer) (err error)
}

func ReadFrame(r io.Reader) (f Frame, err error) {
	var buf [2]byte
	_, err = r.Read(buf[:1])
	if err != nil {
		return
	}
	msgtype := uint8(buf[0])

	_, err = r.Read(buf[:])
	if err != nil {
		return
	}
	length := binary.BigEndian.Uint16(buf[:])

	_, err = r.Read(buf[:])
	if err != nil {
		return
	}
	streamid := binary.BigEndian.Uint16(buf[:])

	switch msgtype {
	case MSG_OK:
		f = new(FrameOK)
	case MSG_FAILED:
		f = new(FrameFAILED)
	case MSG_AUTH:
		f = new(FrameAuth)
	case MSG_DATA:
		f = new(FrameData)
	case MSG_SYN:
		f = new(FrameSyn)
	case MSG_ACK:
		f = new(FrameAck)
	case MSG_FIN:
		f = new(FrameFin)
	case MSG_RST:
		f = new(FrameRst)
	}

	return f, f.ReadFrame(length, streamid, r)
}

func GetFrameBuf(w io.Writer, msgtype uint8, length uint16, streamid uint16) (buf *bufio.Writer, err error) {
	buf = bufio.NewWriterSize(w, int(length+5))
	err = binary.Write(buf, binary.BigEndian, msgtype)
	if err != nil {
		return
	}
	err = binary.Write(buf, binary.BigEndian, length)
	if err != nil {
		return
	}
	err = binary.Write(buf, binary.BigEndian, streamid)
	if err != nil {
		return
	}
	return
}

func ReadString(r io.Reader) (s string, err error) {
	var length uint16
	err = binary.Read(r, binary.BigEndian, &length)
	buf := make([]byte, length)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return
	}
	return string(buf), nil
}

func WriteString(w *bufio.Writer, s string) (err error) {
	err = binary.Write(w, binary.BigEndian, uint16(len(s)))
	if err != nil {
		return
	}
	_, err = w.Write([]byte(s))
	return
}

type FrameOK struct {
	streamid uint16
}

func (f *FrameOK) ReadFrame(length uint16, streamid uint16, r io.Reader) (err error) {
	if length != 0 {
		return errors.New("frame ok with length not 0")
	}
	f.streamid = streamid
	return
}

func (f *FrameOK) WriteFrame(w io.Writer) (err error) {
	buf, err := GetFrameBuf(w, MSG_OK, 0, f.streamid)
	if err != nil {
		return
	}
	defer buf.Flush()
	return
}

type FrameFAILED struct {
	streamid uint16
	errno    uint32
}

func (f *FrameFAILED) ReadFrame(length uint16, streamid uint16, r io.Reader) (err error) {
	if length != 4 {
		return errors.New("frame failed with length not 4")
	}
	f.streamid = streamid

	err = binary.Read(r, binary.BigEndian, &f.errno)
	if err != nil {
		return
	}
	return
}

func (f *FrameFAILED) WriteFrame(w io.Writer) (err error) {
	buf, err := GetFrameBuf(w, MSG_FAILED, 4, f.streamid)
	if err != nil {
		return
	}
	defer buf.Flush()
	err = binary.Write(buf, binary.BigEndian, f.errno)
	return
}

type FrameAuth struct {
	streamid uint16
	username string
	password string
}

func (f *FrameAuth) ReadFrame(length uint16, streamid uint16, r io.Reader) (err error) {
	f.streamid = streamid

	f.username, err = ReadString(r)
	if err != nil {
		return
	}
	f.password, err = ReadString(r)
	if err != nil {
		return
	}

	if int(length) != len(f.username)+len(f.password)+4 {
		return errors.New("frame auth length not match")
	}

	return
}

func (f *FrameAuth) WriteFrame(w io.Writer) (err error) {
	buf, err := GetFrameBuf(w, MSG_AUTH,
		uint16(len(f.username)+len(f.password)+4), f.streamid)
	if err != nil {
		return
	}
	defer buf.Flush()
	err = WriteString(buf, f.username)
	if err != nil {
		return
	}
	err = WriteString(buf, f.password)
	return
}

type FrameData struct {
	streamid uint16
	data     []byte
}

func (f *FrameData) ReadFrame(length uint16, streamid uint16, r io.Reader) (err error) {
	f.streamid = streamid

	f.data = make([]byte, length)
	_, err = io.ReadFull(r, f.data)
	return
}

func (f *FrameData) WriteFrame(w io.Writer) (err error) {
	buf, err := GetFrameBuf(w, MSG_DATA, uint16(len(f.data)), f.streamid)
	if err != nil {
		return
	}
	defer buf.Flush()
	_, err = buf.Write(f.data)
	return
}

const (
	ADDR_HOSTNAME = iota
	ADDR_IPV4
	ADDR_IPV6
)

type FrameSyn struct {
	streamid uint16
	port     uint16
	target   string
}

func (f *FrameSyn) ReadFrame(length uint16, streamid uint16, r io.Reader) (err error) {
	f.streamid = streamid

	err = binary.Read(r, binary.BigEndian, &f.port)
	if err != nil {
		return
	}

	var addrtype uint8
	err = binary.Read(r, binary.BigEndian, &addrtype)
	if err != nil {
		return
	}

	switch addrtype {
	case ADDR_IPV4:
		panic("addr ipv4 not support yet")
	case ADDR_IPV6:
		panic("addr ipv6 not support yet")
	case ADDR_HOSTNAME:
		f.target, err = ReadString(r)
		if err != nil {
			return
		}

		if len(f.target)+5 != int(length) {
			return errors.New("frame syn length not match")
		}
	}

	return
}

func (f *FrameSyn) WriteFrame(w io.Writer) (err error) {
	buf, err := GetFrameBuf(w, MSG_SYN, uint16(len(f.target)+5), f.streamid)
	if err != nil {
		return
	}
	defer buf.Flush()

	err = binary.Write(buf, binary.BigEndian, f.port)
	if err != nil {
		return
	}
	err = binary.Write(buf, binary.BigEndian, uint8(ADDR_HOSTNAME))
	if err != nil {
		return
	}

	err = WriteString(buf, f.target)
	return
}

func (f *FrameSyn) Dial() (conn *net.TCPConn, err error) {
	conn, err = net.Dial("tcp", fmt.Fprint("%s:%d", fs.target, fs.port))
	return
}

type FrameAck struct {
	streamid    uint16
	move_window uint32
}

func (f *FrameAck) ReadFrame(length uint16, streamid uint16, r io.Reader) (err error) {
	if length != 4 {
		return errors.New("frame fin with length not 4")
	}
	f.streamid = streamid
	err = binary.Read(r, binary.BigEndian, &f.move_window)
	return
}

func (f *FrameAck) WriteFrame(w io.Writer) (err error) {
	buf, err := GetFrameBuf(w, MSG_ACK, 4, f.streamid)
	if err != nil {
		return
	}
	defer buf.Flush()
	err = binary.Write(buf, binary.BigEndian, f.move_window)
	return
}

type FrameFin struct {
	streamid uint16
}

func (f *FrameFin) ReadFrame(length uint16, streamid uint16, r io.Reader) (err error) {
	if length != 0 {
		return errors.New("frame fin with length not 0")
	}
	f.streamid = streamid
	return
}

func (f *FrameFin) WriteFrame(w io.Writer) (err error) {
	buf, err := GetFrameBuf(w, MSG_FIN, 0, f.streamid)
	if err != nil {
		return
	}
	defer buf.Flush()
	return
}

type FrameRst struct {
	streamid uint16
}

func (f *FrameRst) ReadFrame(length uint16, streamid uint16, r io.Reader) (err error) {
	if length != 0 {
		return errors.New("frame rst with length not 0")
	}
	f.streamid = streamid
	return
}

func (f *FrameRst) WriteFrame(w io.Writer) (err error) {
	buf, err := GetFrameBuf(w, MSG_RST, 0, f.streamid)
	if err != nil {
		return
	}
	defer buf.Flush()
	return
}
