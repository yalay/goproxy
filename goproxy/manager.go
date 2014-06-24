package main

import (
	"fmt"
	"github.com/shell909090/goproxy/dns"
	"github.com/shell909090/goproxy/msocks"
	"net/http"
	"os"
	"runtime"
	"runtime/pprof"
	"text/template"
	"time"
)

type MsocksManager struct {
	ndialer   *msocks.Dialer
	tmpl_sess *template.Template
}

func NewMsocksManager(ndialer *msocks.Dialer) (mm *MsocksManager) {
	mm = &MsocksManager{
		ndialer: ndialer,
	}
	return
}

func (mm *MsocksManager) Register(mux *http.ServeMux) {
	mux.HandleFunc("/", mm.HandlerMain)
	mux.HandleFunc("/cpu", mm.HandlerCPU)
	mux.HandleFunc("/mem", mm.HandlerMemory)
	mux.HandleFunc("/stack", mm.HandlerGoroutine)
	mux.HandleFunc("/lookup", mm.HandlerLookup)
	mux.HandleFunc("/cutoff", mm.HandlerCutoff)
}

func (mm *MsocksManager) HandlerMain(w http.ResponseWriter, req *http.Request) {
	if mm.tmpl_sess == nil {
		var err error
		mm.tmpl_sess, err = template.New("session").Parse(`
<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 4.01//EN" "http://www.w3.org/TR/html4/strict.dtd">
<html>
  <head>
    <title>session list</title>
    <meta http-equiv="Content-Type" content="text/html; charset=utf-8"/>
    <meta name="author" content="Shell.Xu">
  </head>
  <body>
    <table>
      <tr>
        <td><a href="cpu">cpu</a></td>
        <td><a href="mem">mem</a></td>
        <td><a href="stack">stack</a></td>
        <td><a href="cutoff">cutoff</a></td>
        <td><a href="lookup">lookup</a></td>
        <td>LastPing: {{.GetLastPing}}</td>
      </tr>
    </table>
    <table>
      <tr>
	<th>Id</th><th>State</th><th>Recv-Q</th><th>Send-Q</th><th width="50%">address</th>
      </tr>
      {{range $conn := .GetPorts}}
      <tr>
        {{with $conn}}
          <td>{{$conn.GetId}}</td>
          <td>{{$conn.GetStatus}}</td>
          <td>{{$conn.GetReadBufSize}}</td>
          <td>{{$conn.GetWriteBufSize}}</td>
          <td>{{$conn.Address}}</td>
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

	if mm.ndialer.GetSize() == 0 {
		w.WriteHeader(200)
		w.Write([]byte(`
<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 4.01//EN" "http://www.w3.org/TR/html4/strict.dtd">
<html>
  <head>
    <title>session list</title>
    <meta http-equiv="Content-Type" content="text/html; charset=utf-8"/>
    <meta name="author" content="Shell.Xu">
  </head>
  <body>
    <table>
      <tr>
        <td><a href="cpu">cpu</a></td>
        <td><a href="mem">mem</a></td>
        <td><a href="stack">stack</a></td>
        <td><a href="cutoff">cutoff</a></td>
        <td><a href="lookup">lookup</a></td>
      </tr>
      <tr>no session</tr>
    </table>
  </body>
</html>`))
		return
	}
	err := mm.tmpl_sess.Execute(w, mm.ndialer)
	if err != nil {
		log.Error("%s", err)
	}
}

func (mm *MsocksManager) HandlerCPU(w http.ResponseWriter, req *http.Request) {
	f, err := os.Create("cpu.prof")
	if err != nil {
		log.Error("%s", err)
		w.WriteHeader(500)
		return
	}
	defer f.Close()

	pprof.StartCPUProfile(f)
	time.Sleep(10 * time.Second)
	pprof.StopCPUProfile()

	w.WriteHeader(200)
	return
}

func (mm *MsocksManager) HandlerMemory(w http.ResponseWriter, req *http.Request) {
	f, err := os.Create("mem.prof")
	if err != nil {
		log.Error("%s", err)
		w.WriteHeader(500)
		return
	}
	defer f.Close()

	pprof.WriteHeapProfile(f)

	w.WriteHeader(200)
	return
}

func (mm *MsocksManager) HandlerGoroutine(w http.ResponseWriter, req *http.Request) {
	buf := make([]byte, 20*1024*1024)
	n := runtime.Stack(buf, true)
	w.WriteHeader(200)
	w.Write(buf[:n])
	return
}

func (mm *MsocksManager) HandlerLookup(w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	hosts, ok := q["host"]
	if !ok {
		w.WriteHeader(400)
		w.Write([]byte("no domain"))
		return
	}

	for _, host := range hosts {
		addrs, err := dns.LookupIP(host)
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, "error %s", err)
			return
		}

		for _, addr := range addrs {
			fmt.Fprintf(w, "%s\n", addr)
		}
	}
	// w.WriteHeader(200)
	return
}

func (mm *MsocksManager) HandlerCutoff(w http.ResponseWriter, req *http.Request) {
	mm.ndialer.CutAll()
	return
}
