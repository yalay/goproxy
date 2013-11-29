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
	return p.pr.Read(data)
}

func (p *Pipe) Write(data []byte) (n int, err error) {
	return p.pw.Write(data)
}

func (p *Pipe) Close() (err error) {
	p.Closed = true
	p.pr.CloseWithError(io.EOF)
	p.pw.CloseWithError(io.EOF)
	return
}
