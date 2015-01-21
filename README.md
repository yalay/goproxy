[![Build Status](https://travis-ci.org/shell909090/goproxy.png?branch=master)](https://travis-ci.org/shell909090/goproxy)

# goproxy

goproxy是基于go写的隧道代理服务器，主要用于翻墙。

主要分为两个部分，客户端和服务器端。客户端使用http协议向其他程序提供一个标准代理。当客户端接受请求后，会加密连接服务器端，请求服务器端连接目标。

具体工作细节是。首先查询国外DNS并获得正确结果(未污染结果)，然后把结果和IP段表对比。如果落在国内，直接代理。如果国外，多个请求复用一个加密tcp把请求转到国外vps上处理。加密是预共享密钥。

注意，新版本不再内置dns清洗，建议采用其他dns清理方案。但是预订会增加dns over msocks功能，做为以防万一的手段。

## msocks协议

msocks协议最大的改进是增加了连接复用能力，这个功能允许你在一个TCP连接上封装多个tcp连接。由于qsocks协议非常快速的建立和释放连接，并且每次连接时必然是连接方向目标方发送大量数据，目标方再反向发送。因此有可能被流量模型发现。msocks保持这个连接，因此连接建立速度更快，没有大量的打开和关闭开销，而且流量模型很难发现。

但是由于多个tcp复用封装到一个tcp内，导致单tcp过慢时所有请求的速度都受到压制。因此记得调优tcp配置，增强LFN下的网络效率。而且注意，当高速下载境外资源时，其他翻墙访问会受到影响。

	net.ipv4.tcp_congestion_control = htcp
	net.core.rmem_default = 2621440
	net.core.rmem_max = 16777216
	net.core.wmem_default = 655360
	net.core.wmem_max = 16777216
	net.ipv4.tcp_rmem = 4096        2621440 16777216
	net.ipv4.tcp_wmem = 4096        655360  16777216

## 连接池规则

在msocks的客户端，一次会主动发起两个连接。当连接数低于2个时会主动补充，除非连接被ping-pong game关闭。

在连接时，会寻找承载tcp最少的一根去用。如果所有连接中，承载tcp最小的连接数大于一定值(目前是16)，那么会在后台再增加一根tcp。

当msocks连接断开时，在上面承载的tcp不会主动迁移到其他msocks上，而是会跟着断开。如果连接池满足一定规则(如上所述)，那么断开的连接会重新发起。

连接池不会主动释放链接。但是在断开时不满足规则的链接不会被重建。这使得连接池可以借助链接的主动断开回收msocks连接。

总体来说，连接池使得每个tcp承载的最大连接数保持在15-20左右。避免大量连接堵塞在一个tcp上，同时也尽力避免频繁的tcp连接握手和释放。

# 用法和配置说明

## 配置和路径 ##

系统默认使用/etc/goproxy/config.json作为配置文件，这一路径可以通过命令行参数-config来修改。

配置文件内使用json格式，其中可以指定以下内容：

* mode: 运行模式，可以为server/http/留空。留空是个特殊模式，表示不要启动。
* listen: 监听地址，一般是:port，表示监听所有interface的该端口。
* server: 服务地址，在http模式下需要，表示需要连接的目标地址。
* logfile: log文件路径，留空表示输出到stdout。在deb包中建议留空，用init脚本的机制来生成日志文件。
* loglevel: 日志级别，必须设定。支持EMERG/ALERT/CRIT/ERROR/WARNING/NOTICE/INFO/DEBUG。
* adminiface: 服务器端的控制端口，可以看到服务器端有多少个连接，分别是谁。
* dnsaddr: dns查询的目标地址。如不定义则采用系统自带的dns系统，会读取默认配置并使用。
* dnsnet: dns的网络模式，设定为tcp可以采用tcp模式。
* cipher: 加密算法，可以为aes/des/tripledes，默认aes。
* key: 密钥。16个随机数据base64后的结果。
* blackfile: 黑名单文件，http模式下可选。
* username: 连接用户名，http模式下需要。
* password: 连接密码，http模式下需要。
* auth: 认证用户名/密码对，server模式下需要。
* portmaps: 端口映射配置，将本地端口映射到远程任意一个端口。

## server模式

服务器模式运行在境外机器上，监听某个端口提供服务。客户端可以连接服务器端，通过他连接目标tcp。中间所用的协议是tcp级的。

服务器模式一般需要定义用户名/密码对以验证客户端身份。用户名/密码对在配置文件的auth项目下定义。

## http模式

http模式运行在本地，需要一个境外的server服务器做支撑，对内提供http代理。

连接到http模式的服务上，可以按照http代理协议访问某个目标。服务首先会对目标做DNS查询，然后根据IP地址区分，如果在国内，直接连接。如果国外，使用server去连接。区分的基础是IP段分配列表。

地址黑名单（直接访问的项）在配置文件blackfile下定义。

## 黑名单文件

黑名单文件是一个路由文件，其中列出的子网将不会由服务器端代理，而是直接连接。这通常用于部分IP不希望通过服务器端的时候。

黑名单文件使用文本格式，每个子网一行。行内以空格分割，第一段为IP地址，第二段为子网掩码。允许使用gzip压缩，后缀名必须为gz，可以直接读取。routes.list.gz为样例。

## dns配置

dns是goproxy中很特殊的一个功能。由于代理经常接到连接某域名的指令，因此为了进行ip匹配，需要先进行dns查询。

在老版本goproxy中，使用的是修改过的golang内置的dns系统。由于过滤系统依赖于污染返回地址仅限于特定地址的假定，所以当污染系统升级后，这个方案就不再可行。在新版goproxy中，建议使用独立方案解决dns清洗问题。

具体来说，我们推荐非标准端口转发的方案。这个方案分为两个步骤。

## 远程dns转发

首先远程服务器的非53端口打开一个dnsmasq，并指向正确的upstream。具体来说，请找到/etc/dnsmasq.conf文件，并修改其中的这行。将端口改为合适的值(非53)，并打开防火墙，保证可以从客户端正常连接。

    #port=5353

## 本地dns转发

基于两个因素，我们建议本地配置第二个dnsmasq。首先，很多系统并不支持在resolv.conf中增加端口。其次，反复查询远程服务器性能并不好。这两个问题可以通过配置本地dnsmasq来加以解决。

注意，和远程不同，本地的dns需要被配置为监听在53端口。其余主要配置如下内容:

    server=remoteip#remoteport
	server=secondarydns

第一行指明了远程服务器的地址和端口(上一节的非标准端口)，第二行指定了无法连接时的第二dns服务器。一般第二dns服务器为当前环境中的dns服务器或本地可以快速访问的服务器。

## dns的应用

一般是修改/etc/resolv.conf为nameserver 127.0.0.1。windows下也可类似设定。

## key的生成

可以使用以下语句生成，写入两边的config即可。

    head -c 16 /dev/random | base64

## 服务器端配置样例

	{
		"mode": "server",
		"listen": ":5233",
	 
		"logfile": "",
		"loglevel": "WARNING",
	 
		"cipher": "aes",
		"key": "[your key]",
	 
		"passwd": {
			"username": "password"
		}
	}

## 客户端配置样例

	{
		"mode": "http",
		"listen": ":5233",
		"server": "srv:5233",
	 
		"logfile": "",
		"loglevel": "WARNING",
	 
		"cipher": "aes",
		"key": "[your key]",
		"blackfile": "/usr/share/goproxy/routes.list.gz",
	 
		"username": "username",
		"password": "password"
	}

# admin界面

在http模式下，直接访问代理端口，可以看到当前工作中的所有msocks链接和其中承载的tcp链接。

## 状态解说

* sess: 显示msocks链接的id，其实为本地端口号。
* id: 显示连接在msocks中的编号，随着时间递增而增加。
* state: 显示链接状态。msocks显示承载了多少tcp(下面的行数)，和lastping。
* Recv-Q: 接收后尚未读取的字节数，如果长时间不为0应该是bug。如果是msocks，则显示粗略的每秒接收字节数。
* Send-Q: 发送后未确认的字节数。如果长时间只增长可能是对方没有回应(例如链接断开)。如果是msocks，则显示粗略的每秒发送字节数。
* Address: 远程的地址。msocks行是服务器/客户端地址。连接行是这个链接所链接到的目标。

## last ping

goproxy利用自定的ping-pong规则来检查和保持tcp的活跃。从一端开始发出ping包，对方收到后间隔一段时间后回复。如果超时不回复，则主动断开连接。如果在多次ping-pong中都没有数据收发，则主动断开。

这个机制的保活效果比tcp keepalive更加激进一些，可以在秒级检查连接通畅。但是相应的，更容易受到网络抖动影响而误判为失去连接。lastping上面显示的是最后一次ping的时间间隔。如果超过一定值(目前设定值为30s)，则断开连接。

## cut off

切断所有连接。一般用于所有链接都处于断开状态。大多数情况用不到。

# deb包解说

deb包是适用于debian/ubuntu的安装包。目前打包和测试都是在debian testing上完成，因此对此种系统的支持最完美。debian stable上可保证正常运行。ubuntu的兼容性希望得到反馈。同时希望有人做ubuntu移植，将启动模式改为upstart。

deb包中，主程序在/usr/bin下，启动项在/etc/init.d/goproxy下，配置文件在/etc/goproxy下。修改配置文件后重启服务生效。

默认black文件在/usr/share/goproxy/routes.list.gz。日志默认在/var/log/goproxy.log生成。日志的配置在init文件中修改。

在debian目录下有个默认的init脚本，负责将goproxy封装为服务。

# tar包解说

tar包内包含主程序，routes.list.gz示例。没有config.json示例。因此你需要自行编写一个正确的config.json，然后使用goproxy -config config.json来启动程序。

整个包不需要安装，手工启动和关闭。如果需要自动启动机制，请自行处理。

# 鸣谢

* dns污染后IP地址来源来自[ChinaDNS](https://github.com/clowwindy/ChinaDNS)，MIT license.
* dns库来自golang1.2，BSD license.

# 授权

	Copyright (C) 2012-2014 Shell Xu

	This program is free software; you can redistribute it and/or
	modify it under the terms of the GNU General Public License
	as published by the Free Software Foundation; either version 2
	of the License, or (at your option) any later version.

	This program is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
	GNU General Public License for more details.

	You should have received a copy of the GNU General Public License
	along with this program; if not, write to the Free Software
	Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, MA  02110-1301, USA.

	http://www.gnu.org/licenses/gpl-2.0.html
