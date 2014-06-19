[![Build Status](https://travis-ci.org/shell909090/goproxy.png?branch=master)](https://travis-ci.org/shell909090/goproxy)

# goproxy #

goproxy是基于go写的隧道代理服务器。主要分为两个部分，客户端和服务器端。客户端使用http或socks5协议向其他程序提供一个标准代理。当客户端接受请求后，会加密连接服务器端，请求服务器端连接目标。双方通过预共享的密钥和加密算法通讯，通过用户名/密码验证身份。

goproxy拥有众多参数，以下为参数解释。

* config: 配置文件路径。

# server模式 #

服务器模式一般需要定义用户名/密码对以验证客户端身份。如果没有定义，则允许匿名使用。

用户名/密码对在配置文件的auth项目下定义。

# socks5模式 #

客户端模式提供socks5协议的代理，代理端口在listen中指定。启动时需要在第一个参数指定一个服务端。

## 黑名单文件 ##

黑名单文件是一个路由文件，其中列出的子网将不会由服务器端代理，而是直接连接。这通常用于部分IP不希望通过服务器端的时候。

黑名单文件使用文本格式，每个子网一行。行内以空格分割，第一段为IP地址，第二段为子网掩码。允许使用gzip压缩，后缀名必须为gz，可以直接读取。routes.list.gz为样例。

## dns配置 ##

dns是goproxy中很特殊的一个功能。由于代理经常接到连接某域名的指令，因此为了进行ip匹配，需要先进行dns查询。为了某些特殊原因，goproxy将go自带的dns做了修改。

系统会首先尝试读取本目录的resolv.conf，然后读取/etc/goproxy/resolv.conf。该文件支持一般resolv.conf的所有配置，但是额外多出一项，blackip。

如果blackip有指定，那么当dns查询结果为blackip所指定的ip时，结果丢弃，等待下一个响应包的返回。这个行为可以很大程度上抵御dns污染。

源码中附带了一个resolv.conf，一般可以直接使用。

# http模式 #

http模式提供http协议的代理。推荐使用。

黑名单，DNS配置同socks5模式。

# 启动脚本用法 #

## init脚本 ##

在debian目录下有个默认的init脚本，负责将goproxy封装为服务。

## 配置和路径 ##

系统默认使用/etc/goproxy/config.json作为配置文件，这一路径可以通过-config来修改。

配置文件内使用json格式，其中可以指定以下内容：

* mode: 运行模式，可以为server/socks5/http。stop是个特殊模式，表示不要启动。
* listen: 监听地址，一般是:port，表示监听所有interface的该端口。
* server: 服务地址，在socks5/http模式下需要，表示需要连接的目标地址。
* logfile: log文件路径，留空表示输出到stdout。
* loglevel: 日志级别。支持EMERG/ALERT/CRIT/ERROR/WARNING/NOTICE/INFO/DEBUG，默认WARNING。
* cipher: 加密算法，可以为aes/des/tripledes/rc4，默认aes。
* key: 密钥。
* blackfile: 流量分离文件，socks/http模式下需要。
* resolvconf: resolv文件的位置，默认/etc/goproxy/resolv.conf。
* username: 连接用户名，socks/http模式下需要。
* password: 连接密码，socks/http模式下需要。
* auth: 认证用户名/密码对，server模式下需要。

另外需要注意，/etc/default/goproxy下有一个RUNDAEMON。刚安装的时候，RUNDAEMON关闭，直到配置完成后改为1，goproxy才可以启动。

DAEMON_OPTS里面需要指名运行goproxy所需的参数。注意goproxy自身不算参数，不需要写在里面。

系统自带的black文件在/usr/share/goproxy/routes.list.gz。

## key的生成 ##

可以使用以下语句生成，写入两边的config即可。

    head -c 16 /dev/random | base64

## 配置样例 ##

服务器端。

	{
		"mode": "server",
		"listen": ":5233",
	 
		"logfile": "/var/log/goproxy.log",
		"loglevel": "WARNING",
	 
		"cipher": "aes",
		"keyfile": "/etc/goproxy/key",
	 
		"passwd": {
			"shell": "123"
		}
	}

客户端。

	{
		"mode": "http",
		"listen": ":5233",
		"server": "srv:5233",
	 
		"logfile": "/var/log/goproxy.log",
		"loglevel": "WARNING",
	 
		"cipher": "aes",
		"keyfile": "/etc/goproxy/key",
		"blackfile": "/usr/share/goproxy/routes.list.gz",
	 
		"username": "shell",
		"password": "123"
	}

# msocks协议 #

msocks协议最大的改进是增加了连接复用能力，这个功能允许你在一个TCP连接上代理多个http请求。由于qsocks协议非常快速的建立和释放连接，并且每次连接时必然是连接方向目标方发送大量数据，目标方再反向发送。因此有可能被流量模型发现。msocks保持这个连接，因此连接建立速度更快，没有大量的打开和关闭开销，而且流量模型很难发现。

但是目前连接复用协议中，每个chunk都是1024字节大小。下一步考虑使用随机大小，彻底打乱流量模型。同时启用内容数据压缩（考虑snappy），加快传输速度。

# 授权 #

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

