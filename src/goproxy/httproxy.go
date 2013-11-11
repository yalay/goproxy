package main

import (
	"logging"
	"net/http"
	"strings"
	"sutils"
)

var httplogger logging.Logger

func init() {
	var err error
	httplogger, err = logging.NewFileLogger("default", -1, "httproxy")
	if err != nil {
		panic(err)
	}
}

type Proxy struct {
	tspt   http.Transport
	dialer sutils.Dialer
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	httplogger.Infof("%s: %s", r.Method, r.URL)

	if r.Method == "CONNECT" {
		p.Connect(w, r)
		return
	}

	r.RequestURI = ""
	r.Header.Del("Accept-Encoding")
	r.Header.Del("Proxy-Connection")
	r.Header.Del("Connection")

	resp, err := p.tspt.RoundTrip(r)
	if err != nil {
		httplogger.Err(err)
		return
	}
	defer resp.Body.Close()

	resp.Header.Del("Content-Length")
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
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
		httplogger.Err(err)
		srcconn.Write([]byte("HTTP/1.0 502 OK\r\n\r\n"))
		return
	}
	defer dstconn.Close()
	srcconn.Write([]byte("HTTP/1.0 200 OK\r\n\r\n"))

	sutils.CopyLink(srcconn, dstconn)
	return
}
