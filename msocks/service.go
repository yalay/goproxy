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

type SessionMaker interface {
	MakeSess() (*Session, error)
}

type SessionPool struct {
	mu   sync.Mutex
	sess []*Session
	sm   SessionMaker
}

func CreateSessionPool(sm SessionMaker) (sp SessionPool) {
	sp = SessionPool{
		sess: make([]*Session, 0),
		sm:   sm,
	}
	return
}

func (sp *SessionPool) CutAll() {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	for _, s := range sp.sess {
		s.Close()
	}
	sp.sess = make([]*Session, 0)
}

func (sp *SessionPool) GetSize() int {
	return len(sp.sess)
}

func (sp *SessionPool) GetSess() (sess []*Session) {
	return sp.sess
}

func (sp *SessionPool) Add(s *Session) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.sess = append(sp.sess, s)
}

func (sp *SessionPool) Remove(s *Session) (n int, err error) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	return sp.remove(s)
}

func (sp *SessionPool) GetOrCreateSess() (sess *Session, err error) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	// 如果超时，应当直接出错
	for len(sp.sess) == 0 {
		err = sp.createSession()
		// 如果出错，也应当退出
		if err != nil {
			return err
		}
	}

	sess = sp.getLessSess()
	if sess == nil {
		panic("can't connect to host")
	}
	if sess.GetSize() > MAX_CONN_PRE_SESS || len(sp.sess) < MIN_SESS_NUM {
		go func() {
			sp.mu.Lock()
			defer sp.mu.Unlock()
			sp.createSession()
		}()
	}
	return
}

func (sp *SessionPool) createSession() (err error) {
	sess, err := sp.sm.MakeSess()
	if err != nil {
		log.Error("%s", err)
		return
	}
	sp.sess = append(sp.sess, sess)
	go sp.sessRun(sess)
	return
}

func (sp *SessionPool) getLessSess() (sess *Session) {
	size := 100000
	for _, s := range sp.sess {
		if s.GetSize() < size {
			sess = s
			size = s.GetSize()
		}
	}
	return
}

func (sp *SessionPool) remove(s *Session) (n int, err error) {
	for i, sess := range sp.sess {
		if s == sess {
			n := len(sp.sess)
			sp.sess[i], sp.sess[n-1] = sp.sess[n-1], sp.sess[i]
			sp.sess = sp.sess[:n-1]
			return len(sp.sess), nil
		}
	}
	return 0, ErrSessionNotFound
}

func (sp *SessionPool) sessRun(sess *Session) {
	defer func() {
		sp.mu.Lock()
		defer sp.mu.Unlock()
		n, err := sp.remove(sess)
		if err != nil {
			log.Error("%s", err)
			return
		}

		if n < MIN_SESS_NUM {
			if !sess.IsGameOver() {
				sp.createSession()
			}
		} else {
			if sp.getLessSess().GetSize() > MAX_CONN_PRE_SESS {
				sp.createSession()
			}
		}
	}()

	sess.Run()
	// that's mean session is dead
	log.Warning("session runtime quit, reboot from connect.")
	return
}

type Dialer struct {
	SessionPool
	sutils.Dialer
	serveraddr string
	username   string
	password   string
}

func NewDialer(dialer sutils.Dialer, serveraddr, username, password string) (d *Dialer, err error) {
	d = &Dialer{
		Dialer:     dialer,
		serveraddr: serveraddr,
		username:   username,
		password:   password,
	}
	d.SessionPool = CreateSessionPool(d)
	return
}

// Do I really need sleep? When tcp connect, syn retris will took 2 min(127s).
// I retry this retry 3 times, it will be 381s = 6mins 21s, right?
// I think that's really enough for most of time.
// REMEMBER: don't modify net.ipv4.tcp_syn_retries, unless you know what you do.
func (d *Dialer) MakeSess() (sess *Session, err error) {
	var conn net.Conn
	for i := uint(0); i < DIAL_RETRY; i++ {
		log.Notice("create connect, serveraddr: %s.", d.serveraddr)

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
	log.Notice("create session.")
	return
}

func (d *Dialer) Dial(network, address string) (conn net.Conn, err error) {
	sess, err := d.SessionPool.GetOrCreateSess()
	if err != nil {
		return
	}
	return sess.Dial(network, address)
}

func (d *Dialer) LookupIP(host string) (addrs []net.IP, err error) {
	return
}

type MsocksService struct {
	SessionPool
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
	ms = &MsocksService{
		dialer:      dialer,
		SessionPool: CreateSessionPool(nil),
	}

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
	ms.Add(sess)
	defer ms.Remove(sess)
	sess.Run()

	log.Notice("server session %d quit: %s => %s.",
		sess.LocalPort(), conn.RemoteAddr(), conn.LocalAddr())
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
