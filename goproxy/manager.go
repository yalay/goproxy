package main

import (
	"fmt"
	"github.com/shell909090/goproxy/msocks"
	"net"
	"net/http"
	"net/http/pprof"
	"text/template"
)

type MsocksManager struct {
	sp        *msocks.SessionPool
	tmpl_sess *template.Template
}

func NewMsocksManager(sp *msocks.SessionPool) (mm *MsocksManager) {
	tmpl_sess, err := template.New("session").Parse(`
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
        <td><a href="cutoff">cutoff</a></td>
        <td><a href="lookup">lookup</a></td>
      </tr>
    </table>
    <table>
      <tr>
	<th>Sess</th><th>Id</th><th>State</th>
        <th>Recv-Q</th><th>Send-Q</th><th width="50%">Address</th>
      </tr>
      {{range $sess := .GetSess}}
	<tr>
	  <td>{{$sess.GetId}}</td>
	  <td></td>
	  <td>{{$sess.GetSize}}/{{printf "%0.2fs" $sess.GetLastPing.Seconds}}</td>
	  <td>{{$sess.GetReadSpeed}}</td>
	  <td>{{$sess.GetWriteSpeed}}</td>
	  <td>{{$sess.RemoteAddr}}</td>
	</tr>
	{{range $conn := $sess.GetPorts}}
	  <tr>
	    {{with $conn}}
	      <td></td>
	      <td>{{$conn.GetStreamId}}</td>
	      <td>{{$conn.GetStatus}}</td>
	      <td>{{$conn.GetReadBufSize}}</td>
	      <td>{{$conn.GetWriteBufSize}}</td>
	      <td>{{$conn.Address}}</td>
	    {{else}}
	      <td></td>
	      <td>half closed</td>
	    {{end}}
          </tr>
        {{end}}
      {{end}}
    </table>
  </body>
</html>
`)
	if err != nil {
		panic(err)
	}
	mm = &MsocksManager{
		sp:        sp,
		tmpl_sess: tmpl_sess,
	}
	return
}

func (mm *MsocksManager) Register(mux *http.ServeMux) {
	mux.HandleFunc("/", mm.HandlerMain)
	mux.HandleFunc("/lookup", mm.HandlerLookup)
	mux.HandleFunc("/cutoff", mm.HandlerCutoff)

	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
}

func (mm *MsocksManager) HandlerMain(w http.ResponseWriter, req *http.Request) {
	if mm.sp.GetSize() == 0 {
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
        <td><a href="cutoff">cutoff</a></td>
        <td><a href="lookup">lookup</a></td>
      </tr>
      <tr><td>no session</td></tr>
    </table>
  </body>
</html>`))
		return
	}
	err := mm.tmpl_sess.Execute(w, mm.sp)
	if err != nil {
		log.Error("%s", err)
	}
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
		addrs, err := net.LookupIP(host)
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, "error %s", err)
			return
		}

		for _, addr := range addrs {
			fmt.Fprintf(w, "%s\n", addr)
		}
	}
	return
}

func (mm *MsocksManager) HandlerCutoff(w http.ResponseWriter, req *http.Request) {
	mm.sp.CutAll()
	return
}
