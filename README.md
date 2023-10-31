# ssSocks5
魔改shadowsocks，实现socks5内网穿透。
## 服务端ssSocksServer

配置文件固定为同目录的config.ini
配置文件中的BindAddr用于接收客户端的链接。</br>
收到客户端请求后会以Socks5Addr为socks5端口，若收到多个客户端请求便会在Socks5Addr端口的基础上+1，遇到被占用的端口会跳过。</br>
![image](https://github.com/djhons/ssSocks5/assets/102639729/74d7542a-df6c-46a4-8105-c032c1878f5d)
![image](https://github.com/djhons/ssSocks5/assets/102639729/4c58d741-e9a6-4e9c-ac5a-88a99967e4ae)

添加企业微信通知机器人
```
代理掉辣，兄弟们别急。
socks5:8.8.8.8:1445          
client:127.0.0.1:57018
```
```
新上线了一个socks5代理，兄弟们快冲。
socks5:8.8.8.8:1445          
client:127.0.0.1:57018
```

## 客户端ssSocksClient

运行在内网服务器上，使用-s参数指定带服务器地址。</br>
例如：</br>
![image](https://user-images.githubusercontent.com/102639729/188055910-5cf9478c-d4be-44ce-badd-2cd90a6e0e17.png)
可使用-r参数指定服务器断开连接后尝试的次数，使用-t指定每次尝试的间隔（单位分钟）。默认每10分钟尝试一次，10次后退出程序。

### ssSocksServer
```
ssSocksServer.exe:
```
### ssSocksClient
```
Usage of ssSocksClient.exe:
  -r int
        Retry count, default 10 (default 10)
  -s string
        connect server addr
  -t int
        Time of each retry, 10 minutes by default (default 10)
```
