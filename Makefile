### Makefile --- 

## Author: shell@shell-deb.shdiv.qizhitech.com
## Version: $Id: Makefile,v 0.0 2012/11/02 06:18:14 shell Exp $
## Keywords: 
## X-URL: 

all: build

%.html: %.md
	markdown $^ > $@

build: goproxy glookup

build-doc: README.html

clean:
	rm -f goproxy glookup README.html

install: goproxy README.html
	install -d $(DESTDIR)/usr/bin/
	install -s goproxy $(DESTDIR)/usr/bin/
	install daemonized $(DESTDIR)/usr/bin/
	install -d $(DESTDIR)/usr/share/goproxy/
	install -m 644 routes.list.gz $(DESTDIR)/usr/share/goproxy/
	install -m 644 README.html $(DESTDIR)/usr/share/goproxy/
	install -d $(DESTDIR)/etc/goproxy/
	install -m 644 resolv.conf $(DESTDIR)/etc/goproxy/

goproxy: goproxy.go
	go build -o $@ $^
	chmod 755 $@

glookup: glookup.go
	go build -o $@ $^
	chmod 755 $@

### Makefile ends here
