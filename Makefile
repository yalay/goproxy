### Makefile --- 

## Author: shell@shell-deb.shdiv.qizhitech.com
## Version: $Id: Makefile,v 0.0 2012/11/02 06:18:14 shell Exp $
## Keywords: 
## X-URL: 

all: build

%.html: %.md
	markdown $^ > $@

test:
	go test -i github.com/shell909090/goproxy/ipfilter
	go test -i github.com/shell909090/goproxy/msocks

build:
	mkdir -p bin
	go build -o bin/goproxy github.com/shell909090/goproxy/goproxy
	go build -o bin/glookup github.com/shell909090/goproxy/glookup

build-doc: README.html

clean:
	rm -rf bin README.html

install: build README.html
	install -d $(DESTDIR)/usr/bin/
	install -m 755 -s bin/goproxy $(DESTDIR)/usr/bin/
	install -m 755 -s bin/glookup $(DESTDIR)/usr/bin/
	install -d $(DESTDIR)/usr/share/goproxy/
	install -m 644 debian/routes.list.gz $(DESTDIR)/usr/share/goproxy/
	install -m 644 README.html $(DESTDIR)/usr/share/goproxy/
	install -d $(DESTDIR)/etc/goproxy/
	install -m 644 debian/resolv.conf $(DESTDIR)/etc/goproxy/

### Makefile ends here
