### Makefile --- 

## Author: shell@shell-deb.shdiv.qizhitech.com
## Version: $Id: Makefile,v 0.0 2012/11/02 06:18:14 shell Exp $
## Keywords: 
## X-URL: 
LEVEL=NOTICE

all: build

clean:
	rm -rf bin

test:
	go test -i github.com/shell909090/goproxy/ipfilter
	go test -i github.com/shell909090/goproxy/msocks

build:
	mkdir -p bin
	go build -o bin/goproxy github.com/shell909090/goproxy/goproxy
	go build -o bin/glookup github.com/shell909090/goproxy/glookup

install: build
	install -d $(DESTDIR)/usr/bin/
	install -m 755 -s bin/goproxy $(DESTDIR)/usr/bin/
	install -m 755 -s bin/glookup $(DESTDIR)/usr/bin/
	install -d $(DESTDIR)/usr/share/goproxy/
	install -m 644 debian/routes.list.gz $(DESTDIR)/usr/share/goproxy/
	install -m 644 README.html $(DESTDIR)/usr/share/goproxy/
	install -d $(DESTDIR)/etc/goproxy/
	install -m 644 debian/resolv.conf $(DESTDIR)/etc/goproxy/

press-clean:
	rm -f server.log client.log httproxy.log

press: build press-clean
	bin/goproxy -loglevel=$(LEVEL) -logfile=server.log -mode server -listen=:7000 -keyfile=key -passfile=users.pwd &
	bin/goproxy -loglevel=$(LEVEL) -logfile=httproxy.log -mode http -listen=:7002 -keyfile=key -username=usr -password=pwd localhost:7000 &
# -black=/usr/share/goproxy/routes.list.gz
	sleep 1
	ab -X localhost:7002 -c 10 -n 1000 http://127.0.0.1:6060/
# curl -x http://localhost:7002 http://www.baidu.com > /dev/null
# curl -x http://localhost:7002 http://www.microsoft.com > /dev/null
# curl -x http://localhost:7002 http://mirror.steadfast.net/ubuntu-releases//precise/ubuntu-12.04.3-desktop-amd64.iso -o ubuntu-12.04.3-desktop-amd64.iso
# curl -x http://srv:8118 http://go.googlecode.com/files/go1.2rc5.src.tar.gz
	# killall goproxy

### Makefile ends here
