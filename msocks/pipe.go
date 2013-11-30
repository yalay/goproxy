package msocks

import (
	"io"
)

type Pipe struct {
	Closed bool
	pr     *io.PipeReader
	pw     *io.PipeWriter
}

func NewPipe() (p *Pipe) {
	pr, pw := io.Pipe()
	p = &Pipe{pr: pr, pw: pw}
	return
}

func (p *Pipe) Read(data []byte) (n int, err error) {
	n, err = p.pr.Read(data)
	if err == io.ErrClosedPipe {
		err = io.EOF
	}
	return
}

func (p *Pipe) Write(data []byte) (n int, err error) {
	n, err = p.pw.Write(data)
	if err == io.ErrClosedPipe {
		err = io.EOF
	}
	return
}

func (p *Pipe) Close() (err error) {
	p.Closed = true
	p.pr.Close()
	p.pw.Close()
	return
}
