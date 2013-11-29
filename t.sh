#!/bin/bash

LEVEL=NOTICE

rm -f server.log client.log httproxy.log

make
bin/goproxy -loglevel=$LEVEL -logfile=server.log -mode server -listen=:7000 -keyfile=key -passfile=users.pwd &
bin/goproxy -loglevel=$LEVEL -logfile=httproxy.log -mode http -listen=:7002 -keyfile=key -username=usr -password=pwd localhost:7000 &
# -black=/usr/share/goproxy/routes.list.gz

sleep 1

ab -X localhost:7002 -c 100 -n 10000 http://127.0.0.1:6060/

# curl -x http://localhost:7002 http://www.baidu.com > /dev/null
# curl -x http://localhost:7002 http://www.microsoft.com > /dev/null
# curl -x http://localhost:7002 http://mirror.steadfast.net/ubuntu-releases//precise/ubuntu-12.04.3-desktop-amd64.iso -o ubuntu-12.04.3-desktop-amd64.iso
# curl -x http://srv:8118 http://go.googlecode.com/files/go1.2rc5.src.tar.gz

killall goproxy
