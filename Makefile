### Makefile --- 

## Author: shell@shell-deb.shdiv.qizhitech.com
## Version: $Id: Makefile,v 0.0 2012/11/02 06:18:14 shell Exp $
## Keywords: 
## X-URL: 

all: build

%.html: %.md
	markdown $^ > $@

build:
	export GOPATH=$$GOPATH:$(shell pwd); go install ./...

build-doc: README.html

clean:
	rm -rf bin pkg README.html

install: build README.html
	install -d $(DESTDIR)/usr/bin/
	install -s bin/goproxy $(DESTDIR)/usr/bin/
	install -s bin/glookup $(DESTDIR)/usr/bin/
	install -d $(DESTDIR)/usr/share/goproxy/
	install -m 644 routes.list.gz $(DESTDIR)/usr/share/goproxy/
	install -m 644 README.html $(DESTDIR)/usr/share/goproxy/
	install -d $(DESTDIR)/etc/goproxy/
	install -m 644 resolv.conf $(DESTDIR)/etc/goproxy/

### Makefile ends here
