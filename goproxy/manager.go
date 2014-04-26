package main

import (
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
	mux.HandleFunc("/cpu", mm.HandlerCPU)
	mux.HandleFunc("/mem", mm.HandlerMemory)
	mux.HandleFunc("/stack", mm.HandlerGoroutine)
	mux.HandleFunc("/sess", mm.HandlerSession)
	mux.HandleFunc("/cutoff", mm.HandlerCutoff)
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

func (mm *MsocksManager) HandlerSession(w http.ResponseWriter, req *http.Request) {
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
    LastPing: {{.GetLastPing}}
    <table>
      <tr>
	<th>index</th><th>State</th><th>Recv-Q</th><th>Send-Q</th><th>address</th>
      </tr>
      {{range $index, $conn := .GetPorts}}
      <tr>
        {{with $conn}}
          <td>{{$index}}</td>
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

	sess := mm.ndialer.GetSess(false)
	if sess == nil {
		w.Write([]byte("no session"))
		return
	}
	err := mm.tmpl_sess.Execute(w, sess)
	if err != nil {
		log.Error("%s", err)
	}
}

func (mm *MsocksManager) HandlerCutoff(w http.ResponseWriter, req *http.Request) {
	mm.ndialer.Cutoff()
	return
}
