package logging

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

const TIMEFMT = "2006-01-02T15:04:05.000"

var lvname = []string{
	"EMERG",
	"ALERT",
	"CRIT",
	"ERROR",
	"WARNING",
	"NOTICE",
	"INFO",
	"DEBUG",
}

const (
	LOG_EMERG = iota
	LOG_ALERT
	LOG_CRIT
	LOG_ERR
	LOG_WARNING
	LOG_NOTICE
	LOG_INFO
	LOG_DEBUG
)

var Default *FileLogger

func GetLevelByName(name string) (lv int, err error) {
	for k, v := range lvname {
		if v == name {
			return k, nil
		}
	}
	return -1, fmt.Errorf("unknown loglevel")
}

type Logger interface {
	Stack()
	Alert(a ...interface{})
	Alertf(format string, a ...interface{})
	Crit(a ...interface{})
	Critf(format string, a ...interface{})
	Debug(a ...interface{})
	Debugf(format string, a ...interface{})
	Emerg(a ...interface{})
	Emergf(format string, a ...interface{})
	Err(a ...interface{})
	Errf(format string, a ...interface{})
	Info(a ...interface{})
	Infof(format string, a ...interface{})
	Notice(a ...interface{})
	Noticef(format string, a ...interface{})
	Warning(a ...interface{})
	Warningf(format string, a ...interface{})
}

type FileLogger struct {
	name     string
	m        sync.Mutex
	out      io.Writer
	loglevel int
}

// logfile is empty string: use console
// buf:filename: buffered file
// filename: output to file
func NewFileLogger(logfile string, loglevel int, name string) (l *FileLogger, err error) {
	if len(logfile) == 0 {
		return &FileLogger{name: name, out: os.Stderr, loglevel: loglevel}, nil
	}

	var out io.Writer
	if logfile != "default" {
		buffed := false
		if strings.HasPrefix(logfile, "buf:") {
			logfile = logfile[4:]
			buffed = true
		}

		out, err = os.OpenFile(logfile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		if err != nil {
			return
		}

		if buffed {
			out = bufio.NewWriterSize(out, 1024)
		}
	}

	return &FileLogger{name: name, out: out, loglevel: loglevel}, nil
}

func (l *FileLogger) Output(lv int, s string) {

	loglevel := l.loglevel
	if l.loglevel == -1 {
		loglevel = Default.loglevel
	}
	if loglevel < lv {
		return
	}

	timestr := time.Now().Format(TIMEFMT)
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "???"
		line = 0
	}
	idx := strings.LastIndex(file, "/")
	if idx != -1 {
		file = file[idx+1:]
	}
	buf := fmt.Sprintf("%s (%d)[%s] %s(%s:%d): %s\n",
		timestr, os.Getpid(), lvname[lv], l.name, file, line, s)

	if l.out == nil {
		Default.m.Lock()
		defer Default.m.Unlock()
		Default.out.Write([]byte(buf))
		return
	}

	l.m.Lock()
	defer l.m.Unlock()
	l.out.Write([]byte(buf))
}

func (l *FileLogger) Stack() {
	l.Output(LOG_DEBUG, string(debug.Stack()))
}

func (l *FileLogger) Alert(a ...interface{}) {
	l.Output(LOG_ALERT, fmt.Sprint(a...))
}

func (l *FileLogger) Alertf(format string, a ...interface{}) {
	l.Output(LOG_ALERT, fmt.Sprintf(format, a...))
}

func (l *FileLogger) Crit(a ...interface{}) {
	l.Output(LOG_CRIT, fmt.Sprint(a...))
}

func (l *FileLogger) Critf(format string, a ...interface{}) {
	l.Output(LOG_ALERT, fmt.Sprintf(format, a...))
}

func (l *FileLogger) Debug(a ...interface{}) {
	l.Output(LOG_DEBUG, fmt.Sprint(a...))
}

func (l *FileLogger) Debugf(format string, a ...interface{}) {
	l.Output(LOG_DEBUG, fmt.Sprintf(format, a...))
}

func (l *FileLogger) Emerg(a ...interface{}) {
	l.Output(LOG_EMERG, fmt.Sprint(a...))
}

func (l *FileLogger) Emergf(format string, a ...interface{}) {
	l.Output(LOG_EMERG, fmt.Sprintf(format, a...))
}

func (l *FileLogger) Err(a ...interface{}) {
	l.Output(LOG_ERR, fmt.Sprint(a...))
}

func (l *FileLogger) Errf(format string, a ...interface{}) {
	l.Output(LOG_ERR, fmt.Sprintf(format, a...))
}

func (l *FileLogger) Info(a ...interface{}) {
	l.Output(LOG_INFO, fmt.Sprint(a...))
}

func (l *FileLogger) Infof(format string, a ...interface{}) {
	l.Output(LOG_INFO, fmt.Sprintf(format, a...))
}

func (l *FileLogger) Notice(a ...interface{}) {
	l.Output(LOG_NOTICE, fmt.Sprint(a...))
}

func (l *FileLogger) Noticef(format string, a ...interface{}) {
	l.Output(LOG_NOTICE, fmt.Sprintf(format, a...))
}

func (l *FileLogger) Warning(a ...interface{}) {
	l.Output(LOG_WARNING, fmt.Sprint(a...))
}

func (l *FileLogger) Warningf(format string, a ...interface{}) {
	l.Output(LOG_WARNING, fmt.Sprintf(format, a...))
}

type SysLogger struct {
	FileLogger
	facility int
	hostname string
}

// udp address: syslog format, use f as facility
func NewSysLogger(logfile string, loglevel int, name string, facility int) (l *SysLogger, err error) {
	addr, e := net.ResolveUDPAddr("udp", logfile)
	if e != nil {
		return
	}
	out, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return
	}
	hostname, err := os.Hostname()
	return &SysLogger{
		FileLogger: FileLogger{
			name:     name,
			out:      out,
			loglevel: loglevel,
		},
		facility: facility,
		hostname: hostname,
	}, nil
}

func (l *SysLogger) Output(lv int, s string) {
	if l.loglevel < lv {
		return
	}

	// <facility * 8 + pri>version timestamp hostname app-name procid msgid
	// <facility * 8 + pri>timestamp hostname procid msgid
	timestr := time.Now().Format(TIMEFMT)
	buf := fmt.Sprintf("<%d>%s %s %d %s[]: %s\n", l.facility*8+lv,
		timestr, l.hostname, os.Getpid(), l.name, s)

	l.m.Lock()
	defer l.m.Unlock()
	l.out.Write([]byte(buf))
}

func Stack() {
	Default.Output(LOG_DEBUG, string(debug.Stack()))
}

func Alert(a ...interface{}) {
	Default.Output(LOG_ALERT, fmt.Sprint(a...))
}

func Alertf(format string, a ...interface{}) {
	Default.Output(LOG_ALERT, fmt.Sprintf(format, a...))
}

func Crit(a ...interface{}) {
	Default.Output(LOG_CRIT, fmt.Sprint(a...))
}

func Critf(format string, a ...interface{}) {
	Default.Output(LOG_ALERT, fmt.Sprintf(format, a...))
}

func Debug(a ...interface{}) {
	Default.Output(LOG_DEBUG, fmt.Sprint(a...))
}

func Debugf(format string, a ...interface{}) {
	Default.Output(LOG_DEBUG, fmt.Sprintf(format, a...))
}

func Emerg(a ...interface{}) {
	Default.Output(LOG_EMERG, fmt.Sprint(a...))
}

func Emergf(format string, a ...interface{}) {
	Default.Output(LOG_EMERG, fmt.Sprintf(format, a...))
}

func Err(a ...interface{}) {
	Default.Output(LOG_ERR, fmt.Sprint(a...))
}

func Errf(format string, a ...interface{}) {
	Default.Output(LOG_ERR, fmt.Sprintf(format, a...))
}

func Info(a ...interface{}) {
	Default.Output(LOG_INFO, fmt.Sprint(a...))
}

func Infof(format string, a ...interface{}) {
	Default.Output(LOG_INFO, fmt.Sprintf(format, a...))
}

func Notice(a ...interface{}) {
	Default.Output(LOG_NOTICE, fmt.Sprint(a...))
}

func Noticef(format string, a ...interface{}) {
	Default.Output(LOG_NOTICE, fmt.Sprintf(format, a...))
}

func Warning(a ...interface{}) {
	Default.Output(LOG_WARNING, fmt.Sprint(a...))
}

func Warningf(format string, a ...interface{}) {
	Default.Output(LOG_WARNING, fmt.Sprintf(format, a...))
}

func SetupDefault(logfile string, loglevel int) (err error) {
	Default, err = NewFileLogger(logfile, loglevel, "")
	return
}
