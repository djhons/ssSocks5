# ssSocks5
魔改shadowsocks，实现socks5内网穿透。
## 服务端ssSocksServer

程序默认在9991端口监听客户端的请求，也可以使用使用-saddr :9991更换端口。</br>
收到客户端请求后会随机在开启一个端口作为socks5端口。</br>
例如：</br>
可使用53126端口作为socks5进入内网。
![image](https://user-images.githubusercontent.com/102639729/188045710-76cfb32d-13bb-4631-8532-30e8caa005b1.png)

## 客户端ssSocksClient

运行在内网服务器上，使用-s参数指定带服务器地址。</br>
例如：</br>
![image](https://user-images.githubusercontent.com/102639729/188055910-5cf9478c-d4be-44ce-badd-2cd90a6e0e17.png)
可使用-r参数指定服务器断开连接后尝试的次数，使用-t指定每次尝试的间隔（单位分钟）。默认每10分钟尝试一次，10次后退出程序。

### ssSocksServer
```
Usage of ssSocksServer.exe:
  -saddr string
        client connect port (default ":9991")
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
