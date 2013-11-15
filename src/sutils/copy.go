package sutils

import (
	"io"
)

func CoreCopy(dst io.Writer, src io.Reader) (written int64, err error) {
	buf := Klb.Get()
	defer Klb.Free(buf)

	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er == io.EOF {
			break
		}
		if er != nil {
			err = er
			break
		}
	}
	return written, err
}

func CopyLink(src, dst io.ReadWriteCloser) {
	defer src.Close()
	defer dst.Close()
	go func() {
		defer src.Close()
		defer dst.Close()
		CoreCopy(src, dst)
	}()
	CoreCopy(dst, src)
}
