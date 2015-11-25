package msocks

import (
	"fmt"
	"net"
	"time"

	"github.com/shell909090/goproxy/sutils"
)

type SessionFactory struct {
	sutils.Dialer
	serveraddr string
	username   string
	password   string
}

func NewSessionFactory(dialer sutils.Dialer, serveraddr, username, password string) (d *Dialer, err error) {
	d = &SessionFactory{
		Dialer:     dialer,
		serveraddr: serveraddr,
		username:   username,
		password:   password,
	}
	return
}

// Do I really need sleep? When tcp connect, syn retris will took 2 min(127s).
// I retry this retry 3 times, it will be 381s = 6mins 21s, right?
// I think that's really enough for most of time.
// REMEMBER: don't modify net.ipv4.tcp_syn_retries, unless you know what you do.
func (sf *SessionFactory) CreateSession() (sess *Session, err error) {
	var conn net.Conn
	for i := 0; i < DIAL_RETRY; i++ {
		log.Notice("msocks try to connect %s.", d.serveraddr)

		conn, err = d.Dialer.Dial("tcp", d.serveraddr)
		if err != nil {
			log.Error("%s", err)
			continue
		}

		sess, err = DialSession(conn, d.username, d.password)
		if err != nil {
			log.Error("%s", err)
			continue
		}
		break
	}
	if err != nil {
		log.Critical("can't connect to host, quit.")
		return
	}
	log.Notice("session created.")
	return
}

func DialSession(conn net.Conn, username, password string) (s *Session, err error) {
	ti := time.AfterFunc(AUTH_TIMEOUT*time.Second, func() {
		log.Notice(ErrAuthFailed.Error(), conn.RemoteAddr())
		conn.Close()
	})
	defer func() {
		ti.Stop()
	}()

	log.Notice("auth with username: %s, password: %s.", username, password)
	fb := NewFrameAuth(0, username, password)
	buf, err := fb.Packed()
	if err != nil {
		return
	}

	_, err = conn.Write(buf.Bytes())
	if err != nil {
		return
	}

	f, err := ReadFrame(conn)
	if err != nil {
		return
	}

	ft, ok := f.(*FrameResult)
	if !ok {
		return nil, ErrUnexpectedPkg
	}

	if ft.Errno != ERR_NONE {
		conn.Close()
		return nil, fmt.Errorf("create connection failed with code: %d.", ft.Errno)
	}

	log.Notice("auth passwd.")
	s = NewSession(conn)
	// s.pong()
	return
}
