#!/bin/bash

LEVEL=INFO

rm server.log client.log httproxy.log

make
bin/goproxy -loglevel=$LEVEL -logfile=server.log -mode server -listen=:7000 -keyfile=key -passfile=users.pwd &
bin/goproxy -loglevel=$LEVEL -logfile=client.log -mode client -listen=:7001 -keyfile=key -username=usr -password=pwd -black=routes.list.gz localhost:7000 &
bin/goproxy -loglevel=$LEVEL -logfile=httproxy.log -mode httproxy -listen=:7002 -keyfile=key -username=usr -password=pwd -black=/usr/share/goproxy/routes.list.gz localhost:7000 &

sleep 1

# curl -x socks5://localhost:7001 http://www.baidu.com > /dev/null
# curl -x socks5://localhost:7001 http://www.microsoft.com > /dev/null
# curl -x http://localhost:7002 http://www.baidu.com > /dev/null
# curl -x http://localhost:7002 http://www.microsoft.com > /dev/null

ab -X localhost:7002 -c 450 -n 450000 http://127.0.0.1:6060/

# sleep 6

killall goproxy
