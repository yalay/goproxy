package cryptconn

import (
	"crypto/cipher"
	"encoding/hex"
	"net"
)

const DEBUGOUTPUT bool = false

type CryptConn struct {
	net.Conn
	in  cipher.Stream
	out cipher.Stream
}

func (sc CryptConn) Read(b []byte) (n int, err error) {
	n, err = sc.Conn.Read(b)
	if err != nil {
		return
	}
	sc.in.XORKeyStream(b[:n], b[:n])
	if DEBUGOUTPUT {
		logger.Debug("recv\n", hex.Dump(b[:n]))
	}
	return
}

func (sc CryptConn) Write(b []byte) (n int, err error) {
	if DEBUGOUTPUT {
		logger.Debug("send\n", hex.Dump(b))
	}
	sc.out.XORKeyStream(b[:], b[:])
	return sc.Conn.Write(b)
}
