package msocks

import (
	"errors"
	"io"
	"net"

	"time"

	"github.com/shell909090/goproxy/sutils"
)

type MsocksServer struct {
	*SessionPool
	userpass map[string]string
	dialer   sutils.Dialer
}

func NewServer(auth map[string]string, dialer sutils.Dialer) (ms *MsocksServer, err error) {
	if dialer == nil {
		err = errors.New("empty dialer")
		log.Error("%s", err)
		return
	}
	ms = &MsocksServer{
		dialer:      dialer,
		SessionPool: CreateSessionPool(nil),
	}

	if auth != nil {
		ms.userpass = auth
	}
	return
}

func (ms *MsocksServer) on_auth(stream io.ReadWriteCloser) (err error) {
	f, err := ReadFrame(stream)
	if err != nil {
		return
	}

	ft, ok := f.(*FrameAuth)
	if !ok {
		return ErrUnexpectedPkg
	}

	log.Notice("auth with username: %s, password: %s.", ft.Username, ft.Password)
	if ms.userpass != nil {
		password1, ok := ms.userpass[ft.Username]
		if !ok || (ft.Password != password1) {
			fb := NewFrameResult(ft.Streamid, ERR_AUTH)
			buf, err := fb.Packed()
			_, err = stream.Write(buf.Bytes())
			if err != nil {
				return err
			}
			return ErrAuthFailed
		}
	}

	fb := NewFrameResult(ft.Streamid, ERR_NONE)
	buf, err := fb.Packed()
	if err != nil {
		return
	}

	_, err = stream.Write(buf.Bytes())
	if err != nil {
		return
	}

	log.Info("auth passed.")
	return
}

func (ms *MsocksServer) Handler(conn net.Conn) {
	log.Notice("connection come from: %s => %s.", conn.RemoteAddr(), conn.LocalAddr())

	ti := time.AfterFunc(AUTH_TIMEOUT*time.Second, func() {
		log.Notice(ErrAuthFailed.Error(), conn.RemoteAddr())
		conn.Close()
	})
	err := ms.on_auth(conn)
	if err != nil {
		log.Error("%s", err.Error())
		return
	}
	ti.Stop()

	sess := NewSession(conn)
	sess.next_id = 1
	sess.dialer = ms.dialer
	ms.Add(sess)
	defer ms.Remove(sess)
	sess.Run()

	log.Notice("server session %d quit: %s => %s.",
		sess.LocalPort(), conn.RemoteAddr(), conn.LocalAddr())
}

func (ms *MsocksServer) Serve(listener net.Listener) (err error) {
	var conn net.Conn

	for {
		conn, err = listener.Accept()
		if err != nil {
			log.Error("%s", err)
			continue
		}
		go func() {
			defer conn.Close()
			ms.Handler(conn)
		}()
	}
	return
}
