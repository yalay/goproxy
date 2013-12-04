package main

import (
	"github.com/shell909090/goproxy/logging"
	"github.com/shell909090/goproxy/msocks"
	"github.com/shell909090/goproxy/sutils"
	"net/http"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"text/template"
)

var hopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te", // canonicalized version of "TE"
	"Trailers",
	"Transfer-Encoding",
	"Upgrade",
}

var httplogger logging.Logger

func init() {
	var err error
	httplogger, err = logging.NewFileLogger("default", -1, "httproxy")
	if err != nil {
		panic(err)
	}

}

type Proxy struct {
	transport http.Transport
	dialer    sutils.Dialer
	mux       *http.ServeMux
	ndialer   *msocks.Dialer
	tmpl_sess *template.Template
}

func NewProxy(dialer sutils.Dialer, ndialer *msocks.Dialer) (p *Proxy) {
	p = &Proxy{
		dialer:    dialer,
		transport: http.Transport{Dial: dialer.Dial},
		mux:       http.NewServeMux(),
		ndialer:   ndialer,
	}
	p.mux.HandleFunc("/mem", p.HandlerMemory)
	p.mux.HandleFunc("/stack", p.HandlerGoroutine)
	p.mux.HandleFunc("/sess", p.HandlerSession)
	return
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	httplogger.Infof("%s: %s", req.Method, req.URL)

	if req.Method == "CONNECT" {
		p.Connect(w, req)
		return
	}

	if req.URL.Host == "" {
		p.mux.ServeHTTP(w, req)
		return
	}

	req.RequestURI = ""
	for _, h := range hopHeaders {
		if req.Header.Get(h) != "" {
			req.Header.Del(h)
		}
	}

	resp, err := p.transport.RoundTrip(req)
	if err != nil {
		httplogger.Err(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	_, err = sutils.CoreCopy(w, resp.Body)
	if err != nil {
		httplogger.Err(err)
		return
	}
	return
}

func (p *Proxy) Connect(w http.ResponseWriter, r *http.Request) {
	hij, ok := w.(http.Hijacker)
	if !ok {
		httplogger.Err("httpserver does not support hijacking")
		return
	}
	srcconn, _, err := hij.Hijack()
	if err != nil {
		httplogger.Err("Cannot hijack connection ", err)
		return
	}
	defer srcconn.Close()

	host := r.URL.Host
	if !strings.Contains(host, ":") {
		host += ":80"
	}
	dstconn, err := p.dialer.Dial("tcp", host)
	if err != nil {
		httplogger.Err("dial failed:", err)
		srcconn.Write([]byte("HTTP/1.0 502 OK\r\n\r\n"))
		return
	}
	srcconn.Write([]byte("HTTP/1.0 200 OK\r\n\r\n"))

	sutils.CopyLink(srcconn, dstconn)
	return
}

func (p *Proxy) HandlerMemory(w http.ResponseWriter, req *http.Request) {
	f, err := os.Create("mem.prof")
	if err != nil {
		logger.Err(err)
		w.WriteHeader(500)
		return
	}
	defer f.Close()

	pprof.WriteHeapProfile(f)

	w.WriteHeader(200)
	return
}

func (p *Proxy) HandlerGoroutine(w http.ResponseWriter, req *http.Request) {
	buf := make([]byte, 20*1024*1024)
	n := runtime.Stack(buf, true)
	w.WriteHeader(200)
	w.Write(buf[:n])
	return
}

func (p *Proxy) HandlerSession(w http.ResponseWriter, req *http.Request) {
	if p.tmpl_sess == nil {
		var err error
		p.tmpl_sess, err = template.New("session").Parse(`
<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 4.01//EN" "http://www.w3.org/TR/html4/strict.dtd">
<html>
  <head>
    <title>session list</title>
    <meta http-equiv="Content-Type" content="text/html; charset=utf-8"/>
    <meta name="author" content="Shell.Xu">
  </head>
  <body>
    LastPing: {{.GetLastPing}}
    <table>
      <tr>
	<th>index</th><th>address</th><th>status</th><th>recvlen</th><th>window</th>
      </tr>
      {{range $index, $conn := .GetPorts}}
      <tr>
        {{with $conn}}
          <td>{{$index}}</td><td>{{$conn.Address}}</td><td>{{$conn.GetStatus}}</td><td>{{$conn.ChanFrameSender.Len}}</td><td>{{$conn.GetWindowSize}}</td>
        {{else}}
          <td>{{$index}}</td><td>half closed</td>
        {{end}}
      </tr>
      {{end}}
    </table>
  </body>
</html>
`)
		if err != nil {
			panic(err)
		}
	}

	sess := p.ndialer.GetSess(false)
	if sess == nil {
		w.Write([]byte("no session"))
		return
	}
	err := p.tmpl_sess.Execute(w, sess)
	if err != nil {
		logger.Err(err)
	}
}
