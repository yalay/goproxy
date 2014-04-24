package cryptconn

import (
	"crypto/cipher"
	"net"
)

type Listener struct {
	net.Listener
	block cipher.Block
}

func NewListener(listener net.Listener, method string, keyfile string) (l *Listener, err error) {
	log.Info("Crypt Listener with %s preparing.", method)
	c, err := NewBlock(method, keyfile)
	if err != nil {
		return
	}

	l = &Listener{
		Listener: listener,
		block:    c,
	}
	return
}

func (l *Listener) Accept() (conn net.Conn, err error) {
	conn, err = l.Listener.Accept()
	if err != nil {
		return
	}

	return NewServer(conn, l.block)
}
