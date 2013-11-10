package logging

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
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

var Default *Logger

func GetLevelByName(name string) (lv int, err error) {
	for k, v := range lvname {
		if v == name {
			return k, nil
		}
	}
	return -1, fmt.Errorf("unknown loglevel")
}

type Logger struct {
	name     string
	m        sync.Mutex
	out      io.Writer
	loglevel int
}

// logfile is empty string: use console
// buf:filename: buffered file
// filename: output to file
func NewLogger(logfile string, loglevel int, name string) (l *Logger, err error) {
	if len(logfile) == 0 {
		return &Logger{name: name, out: os.Stderr, loglevel: loglevel}, nil
	}

	if loglevel < 0 {
		loglevel = Default.loglevel
	}

	var out io.Writer
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

	return &Logger{name: name, out: out, loglevel: loglevel}, nil
}

func (l *Logger) Output(lv int, s string) {
	if l.loglevel < lv {
		return
	}

	timestr := time.Now().Format(TIMEFMT)
	buf := fmt.Sprintf("%s %s[%s]: %s\n", timestr, l.name, lvname[lv], s)

	l.m.Lock()
	defer l.m.Unlock()
	l.out.Write([]byte(buf))
}

func (l *Logger) Alert(a ...interface{}) {
	l.Output(LOG_ALERT, fmt.Sprintln(a))
}

func (l *Logger) Alertf(format string, a ...interface{}) {
	l.Output(LOG_ALERT, fmt.Sprintf(format, a))
}

func (l *Logger) Crit(a ...interface{}) {
	l.Output(LOG_CRIT, fmt.Sprintln(a))
}

func (l *Logger) Critf(format string, a ...interface{}) {
	l.Output(LOG_ALERT, fmt.Sprintf(format, a))
}

func (l *Logger) Debug(a ...interface{}) {
	l.Output(LOG_DEBUG, fmt.Sprintln(a))
}

func (l *Logger) Debugf(format string, a ...interface{}) {
	l.Output(LOG_DEBUG, fmt.Sprintf(format, a))
}

func (l *Logger) Emerg(a ...interface{}) {
	l.Output(LOG_EMERG, fmt.Sprintln(a))
}

func (l *Logger) Emergf(format string, a ...interface{}) {
	l.Output(LOG_EMERG, fmt.Sprintf(format, a))
}

func (l *Logger) Err(a ...interface{}) {
	l.Output(LOG_ERR, fmt.Sprintln(a))
}

func (l *Logger) Errf(format string, a ...interface{}) {
	l.Output(LOG_ERR, fmt.Sprintf(format, a))
}

func (l *Logger) Info(a ...interface{}) {
	l.Output(LOG_INFO, fmt.Sprintln(a))
}

func (l *Logger) Infof(format string, a ...interface{}) {
	l.Output(LOG_INFO, fmt.Sprintf(format, a))
}

func (l *Logger) Notice(a ...interface{}) {
	l.Output(LOG_NOTICE, fmt.Sprintln(a))
}

func (l *Logger) Noticef(format string, a ...interface{}) {
	l.Output(LOG_NOTICE, fmt.Sprintf(format, a))
}

func (l *Logger) Warning(a ...interface{}) {
	l.Output(LOG_WARNING, fmt.Sprintln(a))
}

func (l *Logger) Warningf(format string, a ...interface{}) {
	l.Output(LOG_WARNING, fmt.Sprintf(format, a))
}

type SysLogger struct {
	Logger
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
		Logger: Logger{
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

func Alert(a ...interface{}) {
	Default.Output(LOG_ALERT, fmt.Sprintln(a))
}

func Alertf(format string, a ...interface{}) {
	Default.Output(LOG_ALERT, fmt.Sprintf(format, a))
}

func Crit(a ...interface{}) {
	Default.Output(LOG_CRIT, fmt.Sprintln(a))
}

func Critf(format string, a ...interface{}) {
	Default.Output(LOG_ALERT, fmt.Sprintf(format, a))
}

func Debug(a ...interface{}) {
	Default.Output(LOG_DEBUG, fmt.Sprintln(a))
}

func Debugf(format string, a ...interface{}) {
	Default.Output(LOG_DEBUG, fmt.Sprintf(format, a))
}

func Emerg(a ...interface{}) {
	Default.Output(LOG_EMERG, fmt.Sprintln(a))
}

func Emergf(format string, a ...interface{}) {
	Default.Output(LOG_EMERG, fmt.Sprintf(format, a))
}

func Err(a ...interface{}) {
	Default.Output(LOG_ERR, fmt.Sprintln(a))
}

func Errf(format string, a ...interface{}) {
	Default.Output(LOG_ERR, fmt.Sprintf(format, a))
}

func Info(a ...interface{}) {
	Default.Output(LOG_INFO, fmt.Sprintln(a))
}

func Infof(format string, a ...interface{}) {
	Default.Output(LOG_INFO, fmt.Sprintf(format, a))
}

func Notice(a ...interface{}) {
	Default.Output(LOG_NOTICE, fmt.Sprintln(a))
}

func Noticef(format string, a ...interface{}) {
	Default.Output(LOG_NOTICE, fmt.Sprintf(format, a))
}

func Warning(a ...interface{}) {
	Default.Output(LOG_WARNING, fmt.Sprintln(a))
}

func Warningf(format string, a ...interface{}) {
	Default.Output(LOG_WARNING, fmt.Sprintf(format, a))
}

func SetupDefault(logfile string, loglevel int) (err error) {
	Default, err = NewLogger(logfile, loglevel, "")
	return
}
