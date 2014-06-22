[![Build Status](https://travis-ci.org/shell909090/goproxy.png?branch=master)](https://travis-ci.org/shell909090/goproxy)

# goproxy

goproxy是基于go写的隧道代理服务器，主要用于翻墙。

主要分为两个部分，客户端和服务器端。客户端使用http协议向其他程序提供一个标准代理。当客户端接受请求后，会加密连接服务器端，请求服务器端连接目标。

内置了dns清洗，查询国外DNS并剔除假返回。然后把结果和IP段表对比。如果落在国内，直接代理。如果国外，多个请求复用一个加密tcp把请求转到国外vps上处理。加密是预共享密钥。

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

# 用法和配置说明

## 配置和路径 ##

系统默认使用/etc/goproxy/config.json作为配置文件，这一路径可以通过命令行参数-config来修改。

配置文件内使用json格式，其中可以指定以下内容：

* mode: 运行模式，可以为server/http/留空。留空是个特殊模式，表示不要启动。
* listen: 监听地址，一般是:port，表示监听所有interface的该端口。
* server: 服务地址，在http模式下需要，表示需要连接的目标地址。
* logfile: log文件路径，留空表示输出到stdout。在deb包中建议留空，用init脚本的机制来生成日志文件。
* loglevel: 日志级别，必须设定。支持EMERG/ALERT/CRIT/ERROR/WARNING/NOTICE/INFO/DEBUG。
* cipher: 加密算法，可以为aes/des/tripledes，默认aes。
* key: 密钥。16个随机数据base64后的结果。
* blackfile: 黑名单文件，http模式下可选。
* username: 连接用户名，http模式下需要。
* password: 连接密码，http模式下需要。
* auth: 认证用户名/密码对，server模式下需要。

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

dns是goproxy中很特殊的一个功能。由于代理经常接到连接某域名的指令，因此为了进行ip匹配，需要先进行dns查询。为了某些特殊原因，goproxy将go自带的dns做了修改。

系统会读取/etc/goproxy/resolv.conf。该文件支持一般resolv.conf的所有配置，但是额外多出一项，blackip。

如果blackip有指定，那么当dns查询结果为blackip所指定的ip时，结果丢弃，等待下一个响应包的返回。这个行为可以很大程度上抵御dns污染。

源码中附带了一个resolv.conf，一般可以直接使用。

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
		"resolvconf": "/etc/goproxy/resolv.conf",
	 
		"username": "username",
		"password": "password"
	}

# deb包解说

deb包是适用于debian/ubuntu的安装包。目前打包和测试都是在debian testing上完成，因此对此种系统的支持最完美。debian stable上可保证正常运行。ubuntu的兼容性希望得到反馈。同时希望有人做ubuntu移植，将启动模式改为upstart。

deb包中，主程序在/usr/bin下，启动项在/etc/init.d/goproxy下，配置文件在/etc/goproxy下。修改配置文件后重启服务生效。

默认black文件在/usr/share/goproxy/routes.list.gz。日志默认在/var/log/goproxy.log生成。日志的配置在init文件中修改。

在debian目录下有个默认的init脚本，负责将goproxy封装为服务。

# tar包解说

tar包内包含主程序，resolvconf和routes.list.gz示例。没有config.json示例。因此你需要自行编写一个正确的config.json，然后使用goproxy -config config.json来启动程序。

整个包不需要安装，手工启动和关闭。如果需要自动启动机制，请自行处理。

# 授权

    Copyright (C) 2012-2014 Shell Xu

    This program is free software: you can redistribute it and/or modify
    it under the terms of the GNU General Public License as published by
    the Free Software Foundation, either version 3 of the License, or
    (at your option) any later version.

    This program is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU General Public License for more details.

    You should have received a copy of the GNU General Public License
    along with this program.  If not, see <http://www.gnu.org/licenses/>.

