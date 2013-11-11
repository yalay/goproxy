package cryptconn

import (
	"net"
	"sutils"
)

type Dialer struct {
	sutils.Dialer
	Wrapper func(net.Conn) (net.Conn, error)
}

func NewDialer(dialer sutils.Dialer, method string, keyfile string) (d *Dialer, err error) {
	logger.Debugf("Crypt Dialer with %s preparing.", method)
	d = &Dialer{
		Dialer: dialer,
	}

	d.Wrapper, err = New(method, keyfile)
	return
}

func (d *Dialer) Dial(network, addr string) (conn net.Conn, err error) {
	logger.Debugf("dailer connect: %s", addr)
	conn, err = d.Dialer.Dial(network, addr)
	if err != nil {
		return
	}

	return d.Wrapper(conn)
}
