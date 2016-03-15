package cryptconn

import (
	"crypto/cipher"
	"net"
)

type Listener struct {
	net.Listener
	block cipher.Block
}

func NewListener(listener net.Listener, method string, key string) (l *Listener, err error) {
	log.Infof("Crypt Listener with %s preparing.", method)
	c, err := NewBlock(method, key)
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
	for {
		conn, err = l.Listener.Accept()
		if err != nil {
			return
		}

		conn, err = NewServer(conn, l.block)
		if err == nil {
			return
		}
		log.Errorf("%s", err.Error())
	}
	return
}
