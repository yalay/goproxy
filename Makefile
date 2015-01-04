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
	debian/rules clean

test:
	go test github.com/shell909090/goproxy/ipfilter
	go test github.com/shell909090/goproxy/msocks

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
	curl -x http://localhost:5234 http://localhost:6060/ > /dev/null
	curl -x http://localhost:5234 http://www.microsoft.com > /dev/null
	killall goproxy

### Makefile ends here
