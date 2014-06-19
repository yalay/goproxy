package cryptconn

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/des"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"github.com/op/go-logging"
	"io"
	"net"
)

var log = logging.MustGetLogger("")

const (
	KEYSIZE     = 16
	DEBUGOUTPUT = false
)

func NewBlock(method string, key string) (c cipher.Block, err error) {
	log.Debug("Crypt Wrapper with %s preparing.", method)
	byteKey, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return
	}

	switch method {
	case "aes":
		c, err = aes.NewCipher(byteKey)
	case "des":
		c, err = des.NewCipher(byteKey)
	case "tripledes":
		c, err = des.NewTripleDESCipher(byteKey)
	}
	return
}

type CryptConn struct {
	net.Conn
	block cipher.Block
	in    cipher.Stream
	out   cipher.Stream
}

func NewClient(conn net.Conn, block cipher.Block) (sc CryptConn, err error) {
	iv := make([]byte, block.BlockSize())
	_, err = rand.Read(iv)
	if err != nil {
		return
	}

	n, err := conn.Write(iv)
	if err != nil {
		return
	}
	if n != len(iv) {
		err = io.ErrShortWrite
		return
	}

	sc = CryptConn{
		Conn:  conn,
		block: block,
		in:    cipher.NewCFBDecrypter(block, iv),
		out:   cipher.NewCFBEncrypter(block, iv),
	}
	return
}

func NewServer(conn net.Conn, block cipher.Block) (sc *CryptConn, err error) {
	iv := make([]byte, block.BlockSize())
	_, err = io.ReadFull(conn, iv)
	if err != nil {
		return
	}

	sc = &CryptConn{
		Conn:  conn,
		block: block,
		in:    cipher.NewCFBDecrypter(block, iv),
		out:   cipher.NewCFBEncrypter(block, iv),
	}
	return
}

func (sc CryptConn) Read(b []byte) (n int, err error) {
	n, err = sc.Conn.Read(b)
	if err != nil {
		return
	}
	sc.in.XORKeyStream(b[:n], b[:n])
	if DEBUGOUTPUT {
		log.Debug("recv\n", hex.Dump(b[:n]))
	}
	return
}

func (sc CryptConn) Write(b []byte) (n int, err error) {
	if DEBUGOUTPUT {
		log.Debug("send\n", hex.Dump(b))
	}
	sc.out.XORKeyStream(b[:], b[:])
	return sc.Conn.Write(b)
}
