# ssSocks5
魔改shadowsocks，实现socks5内网穿透。
## 服务端ssSocksServer

程序默认在9991端口监听客户端的请求，也可以使用使用-saddr :9991更换端口。</br>
收到客户端请求后会随机在开启一个端口作为socks5端口。</br>
例如：</br>
可使用53126端口作为socks5进入内网。
![image](https://user-images.githubusercontent.com/102639729/188045710-76cfb32d-13bb-4631-8532-30e8caa005b1.png)
