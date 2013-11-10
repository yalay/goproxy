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

func NewCryptWrapper(method string, keyfile string) (f func(net.Conn) (net.Conn, error), err error) {
	var g func(net.Conn, []byte, []byte) (net.Conn, error)
	var key []byte
	var iv []byte

	logging.Debug("Crypt Wrapper with", method, "preparing")
	switch method {
	case "aes":
		key, iv, err = ReadKey(keyfile, 16, 16)
		g = NewAesConn
	case "des":
		key, iv, err = ReadKey(keyfile, 16, 8)
		g = NewDesConn
	case "tripledes":
		key, iv, err = ReadKey(keyfile, 16, 8)
		g = NewTripleDesConn
	case "rc4":
		key, iv, err = ReadKey(keyfile, 16, 0)
		g = NewRC4Conn
	}
	if err != nil {
		return
	}

	return func(conn net.Conn) (sc net.Conn, err error) {
		return g(conn, key, iv)
	}, nil
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
