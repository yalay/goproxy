### Makefile --- 

## Author: shell@shell-deb.shdiv.qizhitech.com
## Version: $Id: Makefile,v 0.0 2012/11/02 06:18:14 shell Exp $
## Keywords: 
## X-URL: 
LEVEL=NOTICE

all: build

buildtar: build
	strip bin/goproxy
	tar cJf ../goproxy-`uname -m`.tar.xz bin/goproxy debian/config.json debian/resolv.conf debian/routes.list.gz

clean:
	rm -rf bin

test:
	go test -i github.com/shell909090/goproxy/ipfilter
	go test -i github.com/shell909090/goproxy/msocks

build:
	mkdir -p bin
	go build -o bin/goproxy github.com/shell909090/goproxy/goproxy

install: build
	install -d $(DESTDIR)/usr/bin/
	install -m 755 -s bin/goproxy $(DESTDIR)/usr/bin/
	install -d $(DESTDIR)/usr/share/goproxy/
	install -m 644 debian/routes.list.gz $(DESTDIR)/usr/share/goproxy/
	install -m 644 README.html $(DESTDIR)/usr/share/goproxy/
	install -d $(DESTDIR)/etc/goproxy/
	install -m 644 debian/resolv.conf $(DESTDIR)/etc/goproxy/
	install -m 644 debian/config.json $(DESTDIR)/etc/goproxy/

press-clean:
	rm -f server.log client.log httproxy.log

press: build press-clean
	bin/goproxy -config=server.json &
	bin/goproxy -config=client.json &
	sleep 1
# ab -X localhost:5234 -c 100 -n 10000 http://127.0.0.1:6060/
	curl -x http://localhost:5234 http://localhost:6060/ > /dev/null
	curl -x http://localhost:5234 http://www.microsoft.com > /dev/null
	curl -x http://localhost:5234 http://web/shell/goproxy_2.0.7_amd64.deb > /dev/null
# curl -x http://localhost:5234 http://202.141.176.110/ubuntu-releases/14.04/ubuntu-14.04-server-amd64.iso -o ubuntu-14.04-server-amd64.iso
# curl -x http://localhost:5234 http://go.googlecode.com/files/go1.2rc5.src.tar.gz
	killall goproxy

### Makefile ends here
