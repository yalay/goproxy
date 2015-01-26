package msocks

import (
	"net"

	"github.com/shell909090/goproxy/sutils"
)

type Dialer struct {
	*SessionPool
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
	log.Notice("create session.")
	return
}

// Maybe we should take care of timeout.
func (d *Dialer) Dial(network, address string) (conn net.Conn, err error) {
	sess, err := d.SessionPool.GetOrCreateSess()
	if err != nil {
		return
	}
	return sess.Dial(network, address)
}
