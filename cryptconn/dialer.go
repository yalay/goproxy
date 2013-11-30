package cryptconn

import (
	"crypto/cipher"
	"github.com/shell909090/goproxy/sutils"
	"net"
)

type Dialer struct {
	sutils.Dialer
	block cipher.Block
}

func NewDialer(dialer sutils.Dialer, method string, keyfile string) (d *Dialer, err error) {
	logger.Infof("Crypt Dialer with %s preparing.", method)
	c, err := NewBlock(method, keyfile)
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
	logger.Infof("dailer connect: %s", addr)
	conn, err = d.Dialer.Dial(network, addr)
	if err != nil {
		return
	}

	return NewClient(conn, d.block)
}
