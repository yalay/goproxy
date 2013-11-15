package msocks

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"sutils"
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

const (
	ERR_AUTH = iota
	ERR_IDEXIST
	ERR_CONNFAILED
)

type Frame interface {
	ReadFrame(length uint16, streamid uint16, r io.Reader) (err error)
	WriteFrame(w io.Writer) (err error)
}

func ReadFrame(r io.Reader) (f Frame, err error) {
	var buf [2]byte
	_, err = r.Read(buf[:1])
	if err != nil {
		logger.Err(err)
		return
	}
	msgtype := uint8(buf[0])

	_, err = r.Read(buf[:])
	if err != nil {
		logger.Err(err)
		return
	}
	length := binary.BigEndian.Uint16(buf[:])

	_, err = r.Read(buf[:])
	if err != nil {
		logger.Err(err)
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
		logger.Err(err)
		return
	}
	err = binary.Write(buf, binary.BigEndian, length)
	if err != nil {
		logger.Err(err)
		return
	}
	err = binary.Write(buf, binary.BigEndian, streamid)
	if err != nil {
		logger.Err(err)
		return
	}
	return
}

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

func WriteString(w *bufio.Writer, s string) (err error) {
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

func SendOKFrame(w io.Writer, streamid uint16) (err error) {
	f := &FrameOK{streamid: streamid}
	return f.WriteFrame(w)
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

func SendFAILEDFrame(w io.Writer, streamid uint16, errno uint32) (err error) {
	f := &FrameFAILED{streamid: streamid, errno: errno}
	return f.WriteFrame(w)
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
	return WriteString(buf, f.password)
}

type FrameData struct {
	streamid uint16
	data     []byte
	buf      []byte
}

func (f *FrameData) ReadFrame(length uint16, streamid uint16, r io.Reader) (err error) {
	f.streamid = streamid
	if length <= 1024 {
		f.buf = sutils.Klb.Get()
		f.data = f.buf[:length]
	} else {
		f.buf = make([]byte, length)
		f.data = f.buf
	}
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

func (f *FrameData) Free() {
	if len(f.buf) == 1024 {
		sutils.Klb.Free(f.buf)
	}
}

type FrameSyn struct {
	streamid uint16
	address  string
}

func (f *FrameSyn) ReadFrame(length uint16, streamid uint16, r io.Reader) (err error) {
	f.streamid = streamid

	f.address, err = ReadString(r)
	if err != nil {
		return
	}
	if int(length) != len(f.address)+2 {
		return errors.New("frame syn length not match")
	}

	return
}

func (f *FrameSyn) WriteFrame(w io.Writer) (err error) {
	buf, err := GetFrameBuf(w, MSG_SYN, uint16(len(f.address)+2), f.streamid)
	if err != nil {
		return
	}
	defer buf.Flush()

	return WriteString(buf, f.address)
}

type FrameAck struct {
	streamid uint16
	window   uint32
}

func (f *FrameAck) ReadFrame(length uint16, streamid uint16, r io.Reader) (err error) {
	if length != 4 {
		return errors.New("frame fin with length not 4")
	}
	f.streamid = streamid
	err = binary.Read(r, binary.BigEndian, &f.window)
	return
}

func (f *FrameAck) WriteFrame(w io.Writer) (err error) {
	buf, err := GetFrameBuf(w, MSG_ACK, 4, f.streamid)
	if err != nil {
		return
	}
	defer buf.Flush()
	err = binary.Write(buf, binary.BigEndian, f.window)
	if err != nil {
		logger.Err(err)
	}
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
