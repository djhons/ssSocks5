# SOCKS5 反向隧道/内网穿透工具

这是一个基于 Go 语言开发的高性能 SOCKS5 反向隧道工具。它允许你将位于内网（无公网 IP）的机器作为出口，通过具有公网 IP 的服务器进行代理访问。

**核心原理**：客户端（内网）主动连接服务端（公网），建立加密隧道。用户连接服务端的 SOCKS5 端口时，流量会被转发至客户端，并由客户端访问目标网络。

## 主要特性

- **反向代理**: 即使客户端处于 NAT 或防火墙后，也能提供代理服务（内网穿透）。
- **多客户端支持**: 服务端通过 `client_id` 区分不同的客户端连接，支持多个节点同时在线。
- **安全性**:
    - 隧道通信使用 **AES-256-GCM** 加密，防止流量被识别或篡改。
    - SOCKS5 代理强制开启认证，用户名密码即为客户端配置的 ID 和密码。
- **JSON 配置**: 全面采用 JSON 格式配置文件，结构清晰。
- **智能通知**: 客户端上线/掉线时，服务端会自动通过 **企业微信 Webhook** 推送通知，通知内容包含连接所需的 IP、端口、账号和密码。
- **断线重连**: 客户端具备自动检测连接状态并重连的机制。
- **多路复用**: 使用 `yamux` 实现单 TCP 连接上的多路复用，降低延迟并提高稳定性。

## 目录结构

```text
socks5-tunnel/
├── client/
│   ├── main.go          # 客户端主程序
│   └── client.json      # 客户端配置文件（需手动创建）
├── server/
│   ├── main.go          # 服务端主程序
│   └── server.json      # 服务端配置文件（需手动创建）
└── common/
    └── crypto.go        # 加密/解密共享库
```

## 编译指南

请确保您的环境已安装 Go 1.18 或更高版本。

### 1. 初始化模块

如果是首次运行项目，请先初始化 Go module：

```bash
go mod init socks5-tunnel
go mod tidy
```

### 2. 编译

```bash
# 编译服务端
go build -o server_bin ./server

# 编译客户端
go build -o client_bin ./client
```

## 快速开始

### 1. 服务端配置 (部署在公网服务器)

在 `server_bin` 同级目录下创建 `server.json`：

```json
{
  "tunnel_port": "8080",
  "socks_port": "1080",
  "aes_key": "your-32-byte-secret-key-must-match",
  "wecom_webhook": "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=YOUR_KEY",
  "idle_timeout": 300
}
```

* `tunnel_port`: 供内网客户端连接的隧道通信端口。
* `socks_port`: 供用户使用的 SOCKS5 代理端口（浏览器或软件填这个）。
* `aes_key`: 通信加密密钥，**必须严格等于 32 个字符**。
* `wecom_webhook`: (可选) 企业微信群机器人地址，用于接收上线通知。

**运行服务端：**
```bash
./server_bin
```
> 服务端启动后，会自动检测并打印本机公网 IP，用于通知内容拼接。

### 2. 客户端配置 (部署在内网/被控端)

在 `client_bin` 同级目录下创建 `client.json`：

```json
{
  "server_addr": "x.x.x.x:8080",
  "client_id": "user1",
  "client_pass": "secure_password_123",
  "aes_key": "your-32-byte-secret-key-must-match",
  "idle_timeout": 300
}
```

* `server_addr`: 服务端的 IP 和 `tunnel_port`。
* `client_id`: 客户端唯一标识，将作为 SOCKS5 的**用户名**。
* `client_pass`: 客户端密码，将作为 SOCKS5 的**密码**。
* `aes_key`: 必须与服务端配置完全一致。

**运行客户端：**
```bash
./client_bin
```

### 3. 连接与使用

当客户端成功连接服务端后：

1.  **接收通知**：服务端会向企业微信发送一条格式如下的通知，方便直接复制使用：
    ```text
    user1 上线
    socks5 <服务端公网IP> 1080 user1 secure_password_123
    ```

2.  **配置代理**：
    在你的浏览器（推荐 SwitchyOmega 插件）、Telegram 或其他支持 SOCKS5 的软件中配置：
    * **协议**: SOCKS5
    * **地址**: 服务端公网 IP
    * **端口**: `socks_port` (例如 1080)
    * **认证**: 必须开启
        * **账号**: `client_id` (例如 user1)
        * **密码**: `client_pass` (例如 secure_password_123)

## 注意事项

1.  **AES Key**: `aes_key` 必须正好是 32 字节长度，否则程序会报错退出。
2.  **防火墙**: 请确保云服务器的安全组放行了 `tunnel_port` 和 `socks_port`。
3.  **多用户模式**: 你可以部署多个客户端连接同一个服务端，只要他们的 `client_id` 不同。服务端会根据连接时的用户名自动将流量路由到对应的内网客户端。