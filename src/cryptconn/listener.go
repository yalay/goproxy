package cryptconn

import (
	"net"
)

type Listener struct {
	net.Listener
	key     []byte
	iv      []byte
	Wrapper func(net.Conn) (net.Conn, error)
}

func NewListener(listener net.Listener, method string, keyfile string) (l *Listener, err error) {
	logger.Debugf("Crypt Listener with %s preparing.", method)
	l = &Listener{
		Listener: listener,
	}

	l.Wrapper, err = New(method, keyfile)
	return
}

func (l *Listener) Accept() (conn net.Conn, err error) {
	conn, err = l.Listener.Accept()
	if err != nil {
		return
	}
	return l.Wrapper(conn)
}
