package cryptconn

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/des"
	"crypto/rc4"
	"io"
	"logging"
	"net"
	"os"
)

var logger logging.Logger

func init() {
	var err error
	logger, err = logging.NewFileLogger("default", -1, "crypt")
	if err != nil {
		panic(err)
	}
}

func ReadKey(keyfile string, keysize int, ivsize int) (key []byte, iv []byte, err error) {
	file, err := os.Open(keyfile)
	if err != nil {
		return
	}
	defer file.Close()

	key = make([]byte, keysize)
	_, err = io.ReadFull(file, key)
	if err != nil {
		return
	}

	iv = make([]byte, ivsize)
	_, err = io.ReadFull(file, iv)
	return
}

func NewAesConn(conn net.Conn, key []byte, iv []byte) (sc net.Conn, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return
	}
	in := cipher.NewCFBDecrypter(block, iv)
	out := cipher.NewCFBEncrypter(block, iv)
	return CryptConn{conn, in, out}, nil
}

func NewDesConn(conn net.Conn, key []byte, iv []byte) (sc net.Conn, err error) {
	block, err := des.NewCipher(key)
	if err != nil {
		return
	}
	in := cipher.NewCFBDecrypter(block, iv)
	out := cipher.NewCFBEncrypter(block, iv)
	return CryptConn{conn, in, out}, nil
}

func NewTripleDesConn(conn net.Conn, key []byte, iv []byte) (sc net.Conn, err error) {
	block, err := des.NewTripleDESCipher(key)
	if err != nil {
		return
	}
	in := cipher.NewCFBDecrypter(block, iv)
	out := cipher.NewCFBEncrypter(block, iv)
	return CryptConn{conn, in, out}, nil
}

func NewRC4Conn(conn net.Conn, key []byte, iv []byte) (sc net.Conn, err error) {
	in, err := rc4.NewCipher(key)
	if err != nil {
		return
	}
	out, err := rc4.NewCipher(key)
	if err != nil {
		return
	}
	return CryptConn{conn, in, out}, nil
}

func New(method string, keyfile string) (
	wrapper func(net.Conn) (net.Conn, error), err error) {
	logger.Debugf("Crypt Wrapper with %s preparing.", method)

	var key, iv []byte
	var g func(net.Conn, []byte, []byte) (net.Conn, error)
	switch method {
	case "aes":
		g = NewAesConn
		key, iv, err = ReadKey(keyfile, 16, 16)
	case "des":
		g = NewDesConn
		key, iv, err = ReadKey(keyfile, 16, 8)
	case "tripledes":
		g = NewTripleDesConn
		key, iv, err = ReadKey(keyfile, 16, 8)
	case "rc4":
		g = NewRC4Conn
		key, iv, err = ReadKey(keyfile, 16, 0)
	}
	if err != nil {
		return
	}

	return func(c net.Conn) (net.Conn, error) {
		return g(c, key, iv)
	}, nil
}
