#!/bin/bash

LEVEL=NOTICE

rm -f server.log client.log httproxy.log

make
bin/goproxy -loglevel=$LEVEL -logfile=server.log -mode server -listen=:7000 -keyfile=key -passfile=users.pwd &
bin/goproxy -loglevel=$LEVEL -logfile=httproxy.log -mode http -listen=:7002 -keyfile=key -username=usr -password=pwd -black=/usr/share/goproxy/routes.list.gz localhost:7000 &

sleep 1

ab -X localhost:7002 -c 100 -n 10000 http://127.0.0.1:6060/

# curl -x http://localhost:7002 http://www.baidu.com > /dev/null
# curl -x http://localhost:7002 http://www.microsoft.com > /dev/null
# curl -x http://localhost:7002 http://mirrors.ustc.edu.cn/debian-cd/7.2.0/amd64/iso-cd/debian-7.2.0-amd64-netinst.iso debian-7.2.0-amd64-netinst.iso

killall goproxy
