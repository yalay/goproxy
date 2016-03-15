package cryptconn

import (
	"crypto/cipher"
	"net"

	"github.com/shell909090/goproxy/sutils"
)

type Dialer struct {
	sutils.Dialer
	block cipher.Block
}

func NewDialer(dialer sutils.Dialer, method string, key string) (d *Dialer, err error) {
	log.Infof("Crypt Dialer with %s preparing.", method)
	c, err := NewBlock(method, key)
	if err != nil {
		return
	}

	d = &Dialer{
		Dialer: dialer,
		block:  c,
	}
	return
}

func (d *Dialer) Dial(network, addr string) (conn net.Conn, err error) {
	log.Infof("Ctypt Dailer connect %s", addr)
	conn, err = d.Dialer.Dial(network, addr)
	if err != nil {
		return
	}

	return NewClient(conn, d.block)
}
