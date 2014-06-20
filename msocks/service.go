package msocks

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/shell909090/goproxy/sutils"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

func RecvWithTimeout(ch chan uint32, t time.Duration) (errno uint32) {
	ch_timeout := time.After(t)
	select {
	case errno = <-ch:
	case <-ch_timeout:
		return ERR_TIMEOUT
	}
	return
}

type Dialer struct {
	sutils.Dialer
	serveraddr string
	username   string
	password   string
	lock       sync.Mutex
	sess       *Session
}

func NewDialer(dialer sutils.Dialer, serveraddr string,
	username, password string) (d *Dialer, err error) {
	d = &Dialer{
		Dialer:     dialer,
		serveraddr: serveraddr,
		username:   username,
		password:   password,
	}
	return
}

func (d *Dialer) Cutoff() {
	d.lock.Lock()
	defer d.lock.Unlock()
	if d.sess != nil {
		d.sess.Close()
	}
}

func (d *Dialer) createConn() (conn net.Conn, err error) {
	log.Notice("create connect, serveraddr: %s.",
		d.serveraddr)

	conn, err = d.Dialer.Dial("tcp", d.serveraddr)
	if err != nil {
		return
	}

	log.Notice("auth with username: %s, password: %s.",
		d.username, d.password)
	fb := NewFrameAuth(0, d.username, d.password)
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
		err = errors.New("unexpected package")
		log.Error("%s", err)
		return
	}

	if ft.Errno != ERR_NONE {
		conn.Close()
		err = fmt.Errorf("create connection failed with code: %d.",
			ft.Errno)
		log.Error("%s", err)
		return
	}

	log.Notice("auth ok.")
	return
}

func (d *Dialer) createSession() (err error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	if d.sess != nil {
		return
	}

	// retry
	var conn net.Conn
	for i := uint(0); i < DIAL_RETRY; i++ {
		conn, err = d.createConn()
		if err != nil {
			log.Error("%s", err)
			time.Sleep((1 << i) * time.Second)
		} else {
			break
		}
	}
	if err != nil {
		log.Critical("can't connect to host, quit.")
		return
	}

	log.Notice("create session.")
	d.sess = NewSession(conn)
	d.sess.Ping()

	go func() {
		d.sess.Run()
		// that's mean session is dead
		log.Warning("session runtime quit, reboot from connect.")

		// remove from sess
		d.lock.Lock()
		d.sess = nil
		d.lock.Unlock()

		d.createSession()
	}()
	return
}

func (d *Dialer) GetSess(create bool) (sess *Session) {
	if d.sess == nil && create {
		err := d.createSession()
		if err != nil {
			log.Error("%s", err)
		}
	}
	return d.sess
}

func (d *Dialer) Dial(network, address string) (conn net.Conn, err error) {
	sess := d.GetSess(true)
	if sess == nil {
		panic("can't connect to host")
	}
	log.Info("try dial: %s => %s.",
		sess.conn.RemoteAddr().String(), address)

	c := NewConn(ST_SYN_SENT, 0, sess, address)
	c.ch = make(chan uint32, 0)
	streamid, err := sess.PutIntoNextId(c)
	if err != nil {
		return
	}
	c.streamid = streamid

	fb := NewFrameSyn(streamid, address)
	err = sess.SendFrame(fb)
	if err != nil {
		log.Error("%s", err)
		c.Final()
		return
	}

	errno := RecvWithTimeout(c.ch, DIAL_TIMEOUT*time.Millisecond)
	if errno != ERR_NONE {
		log.Error("connection failed for remote failed(%d): %d.",
			streamid, errno)
		c.Final()
	} else {
		log.Notice("connect successed: %p(%d) => %s.",
			sess, streamid, address)
	}
	c.ch = nil

	return c, nil
}

type MsocksService struct {
	userpass map[string]string
	dialer   sutils.Dialer
}

func LoadPassfile(filename string) (userpass map[string]string, err error) {
	log.Notice("load passfile from file %s.", filename)

	file, err := os.Open(filename)
	if err != nil {
		log.Error("%s", err)
		return
	}
	defer file.Close()
	userpass = make(map[string]string, 0)

	reader := bufio.NewReader(file)
QUIT:
	for {
		line, err := reader.ReadString('\n')
		switch err {
		case io.EOF:
			if len(line) == 0 {
				break QUIT
			}
		case nil:
		default:
			return nil, err
		}
		f := strings.SplitN(line, ":", 2)
		if len(f) < 2 {
			err = fmt.Errorf("format wrong: %s", line)
			log.Error("%s", err)
			return nil, err
		}
		userpass[strings.Trim(f[0], "\r\n ")] = strings.Trim(f[1], "\r\n ")
	}

	log.Info("userinfo loaded %d record(s).", len(userpass))
	return
}

func NewService(auth map[string]string, dialer sutils.Dialer) (ms *MsocksService, err error) {
	if dialer == nil {
		err = errors.New("empty dialer")
		log.Error("%s", err)
		return
	}
	ms = &MsocksService{dialer: dialer}

	if auth != nil {
		ms.userpass = auth
	}
	return
}

func (ms *MsocksService) on_auth(stream io.ReadWriteCloser) bool {
	f, err := ReadFrame(stream)
	if err != nil {
		log.Error("%s", err)
		return false
	}

	ft, ok := f.(*FrameAuth)
	if !ok {
		log.Error("unexpected package type")
		return false
	}

	log.Notice("auth with username: %s, password: %s.",
		ft.Username, ft.Password)
	if ms.userpass != nil {
		password1, ok := ms.userpass[ft.Username]
		if !ok || (ft.Password != password1) {
			log.Error("auth failed.")
			fb := NewFrameResult(ft.Streamid, ERR_AUTH)
			buf, err := fb.Packed()
			_, err = stream.Write(buf.Bytes())
			if err != nil {
				log.Error("%s", err)
				return false
			}
			return false
		}
	}
	fb := NewFrameResult(ft.Streamid, ERR_NONE)
	buf, err := fb.Packed()
	if err != nil {
		log.Error("%s", err)
		return false
	}
	_, err = stream.Write(buf.Bytes())
	if err != nil {
		log.Error("%s", err)
		return false
	}

	log.Info("auth passed.")
	return true
}

func (ms *MsocksService) Handler(conn net.Conn) {
	log.Notice("connection come from: %s => %s.",
		conn.RemoteAddr(), conn.LocalAddr())

	ti := time.AfterFunc(AUTH_TIMEOUT*time.Millisecond, func() {
		log.Notice("wait too long time for auth, close conn %s.", conn.RemoteAddr())
		conn.Close()
	})
	if !ms.on_auth(conn) {
		return
	}
	ti.Stop()

	sess := NewSession(conn)
	sess.dialer = ms.dialer
	sess.Run()
	log.Notice("server session %p quit: %s => %s.",
		sess, conn.RemoteAddr(), conn.LocalAddr())
}

func (ms *MsocksService) Serve(listener net.Listener) (err error) {
	var conn net.Conn

	for {
		conn, err = listener.Accept()
		if err != nil {
			log.Error("%s", err)
			return
		}
		go func() {
			defer conn.Close()
			ms.Handler(conn)
		}()
	}
	return
}
