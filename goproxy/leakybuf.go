package main

var freelist = make(chan []byte, 3)

func allocbuf() (buf []byte) {
	select {
	case buf = <-freelist:
	default:
		buf = make([]byte, 8192)
	}
	return
}

func freebuf(buf []byte) {
	select {
	case freelist <- buf:
	default:
	}
	return
}
