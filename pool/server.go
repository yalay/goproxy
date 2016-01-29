package pool

import (
	"errors"
	"net"

	"github.com/shell909090/goproxy/msocks"
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
		dialer: dialer,
	}

	if auth != nil {
		ms.userpass = auth
	}
	return
}

func (ms *MsocksServer) Handler(conn net.Conn) {
	sess, err := msocks.ServerInital(conn, ms.userpass, ms.dialer)
	if err != nil {
		log.Error("%s", err.Error())
		return
	}

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
