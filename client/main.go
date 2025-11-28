package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/yamux"
	"io"
	"log"
	"net"
	"os"
	"socks5-tunnel/common" // 你的加密包
	"sync"
	"time"
)

// Config 增加 ClientPass 字段
type Config struct {
	ServerAddr  string `json:"server_addr"`
	ClientID    string `json:"client_id"`
	ClientPass  string `json:"client_pass"` // 新增：密码
	AesKey      string `json:"aes_key"`
	IdleTimeout int    `json:"idle_timeout"`
}

var (
	cfg     Config
	bufPool = sync.Pool{
		New: func() interface{} { return make([]byte, 32*1024) },
	}
)

const (
	DefaultIdleTimeout = 300 * time.Second
	AtypIPv4           = 0x01
	AtypDomain         = 0x03
	AtypIPv6           = 0x04
)

func main() {
	if err := loadConfig(); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	for {
		runClient()
		log.Println("Connection lost, retrying in 5 seconds...")
		time.Sleep(5 * time.Second)
	}
}

func loadConfig() error {
	file, err := os.ReadFile("client.json")
	if err != nil {
		return err
	}
	return json.Unmarshal(file, &cfg)
}

// runClient 包含完整的连接、握手(带密码)和会话建立逻辑
func runClient() {
	// 1. 建立 TCP 连接
	conn, err := net.DialTimeout("tcp", cfg.ServerAddr, 10*time.Second)
	if err != nil {
		log.Println("Dial error:", err)
		return
	}

	// 2. 加密层包装
	cryptoConn, err := common.WrapConn(conn, []byte(cfg.AesKey))
	if err != nil {
		conn.Close()
		log.Println("Crypto wrap error:", err)
		return
	}

	// 3. 发送握手信息：[ID Len][ID][Pass Len][Pass]
	// 设置写超时，防止网络卡死
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))

	idBytes := []byte(cfg.ClientID)
	passBytes := []byte(cfg.ClientPass)

	if len(idBytes) > 255 || len(passBytes) > 255 {
		log.Println("Client ID or Password too long (max 255)")
		conn.Close()
		return
	}

	// 构造 payload
	payload := make([]byte, 0, 1+len(idBytes)+1+len(passBytes))
	payload = append(payload, byte(len(idBytes)))
	payload = append(payload, idBytes...)
	payload = append(payload, byte(len(passBytes))) // 写入密码长度
	payload = append(payload, passBytes...)         // 写入密码内容

	if _, err := cryptoConn.Write(payload); err != nil {
		log.Println("Handshake write error:", err)
		conn.Close()
		return
	}

	// 清除超时
	conn.SetWriteDeadline(time.Time{})

	// 4. 初始化 Yamux 客户端 (开启 KeepAlive)
	ymConfig := yamux.DefaultConfig()
	ymConfig.EnableKeepAlive = true
	ymConfig.KeepAliveInterval = 15 * time.Second
	ymConfig.ConnectionWriteTimeout = 10 * time.Second
	ymConfig.LogOutput = io.Discard // 减少日志噪音

	session, err := yamux.Client(cryptoConn, ymConfig)
	if err != nil {
		log.Println("Yamux init error:", err)
		conn.Close()
		return
	}
	defer session.Close() // 确保退出时关闭 Session

	log.Printf("Connected to server as [%s]", cfg.ClientID)

	// 5. 循环处理服务端发来的流 (SOCKS 请求)
	for {
		stream, err := session.Accept()
		if err != nil {
			log.Println("Session accept error (server disconnected):", err)
			return
		}
		// 并发处理每个请求
		go handleSocksRequest(stream)
	}
}

// handleSocksRequest 处理具体的 SOCKS5 请求逻辑
func handleSocksRequest(stream net.Conn) {
	defer stream.Close()

	// 1. 解析请求 (使用严格模式)
	targetAddr, err := parseSocksRequest(stream)
	if err != nil {
		// 解析失败通常是协议错误或攻击，直接关闭即可
		return
	}

	// 2. 连接目标服务器
	targetConn, err := net.DialTimeout("tcp", targetAddr, 10*time.Second)
	if err != nil {
		return
	}
	defer targetConn.Close()

	// 3. 响应成功 (0x00)
	// 注意：这里简单响应 0.0.0.0:0，标准协议应返回实际绑定的地址
	if _, err := stream.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
		return
	}

	// 4. 准备超时配置
	timeout := DefaultIdleTimeout
	if cfg.IdleTimeout > 0 {
		timeout = time.Duration(cfg.IdleTimeout) * time.Second
	}

	// 5. 包装连接 (Idle Timeout)
	wrappedStream := &TimeoutConn{Conn: stream, Timeout: timeout}
	wrappedTarget := &TimeoutConn{Conn: targetConn, Timeout: timeout}

	// 6. 开始传输
	transport(wrappedStream, wrappedTarget)
}

// transport 双向零拷贝传输
func transport(local, remote net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	copyFunc := func(dst, src net.Conn) {
		defer wg.Done()
		buf := bufPool.Get().([]byte)
		defer bufPool.Put(buf)

		io.CopyBuffer(dst, src, buf)
		// 一端 EOF 或出错，关闭另一端以触发整体断开
		dst.Close()
	}

	go copyFunc(local, remote)
	go copyFunc(remote, local)

	wg.Wait()
}

// parseSocksRequest 严格解析 SOCKS5 请求头
func parseSocksRequest(reader io.Reader) (string, error) {
	// [VER, CMD, RSV, ATYP]
	header := make([]byte, 4)
	if _, err := io.ReadFull(reader, header); err != nil {
		return "", err
	}
	if header[0] != 0x05 || header[1] != 0x01 {
		return "", fmt.Errorf("invalid socks req")
	}

	var host string
	switch header[3] {
	case AtypIPv4:
		buf := make([]byte, 4)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return "", err
		}
		host = net.IP(buf).String()
	case AtypDomain:
		lenBuf := make([]byte, 1)
		if _, err := io.ReadFull(reader, lenBuf); err != nil {
			return "", err
		}
		length := int(lenBuf[0])
		buf := make([]byte, length)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return "", err
		}
		host = string(buf)
	case AtypIPv6:
		buf := make([]byte, 16)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return "", err
		}
		host = fmt.Sprintf("[%s]", net.IP(buf).String())
	default:
		return "", fmt.Errorf("unsupported atyp")
	}

	portBuf := make([]byte, 2)
	if _, err := io.ReadFull(reader, portBuf); err != nil {
		return "", err
	}
	port := binary.BigEndian.Uint16(portBuf)

	return fmt.Sprintf("%s:%d", host, port), nil
}

// TimeoutConn 自动超时装饰器
type TimeoutConn struct {
	net.Conn
	Timeout time.Duration
}

func (c *TimeoutConn) Read(b []byte) (n int, err error) {
	if c.Timeout > 0 {
		c.Conn.SetReadDeadline(time.Now().Add(c.Timeout))
	}
	return c.Conn.Read(b)
}

func (c *TimeoutConn) Write(b []byte) (n int, err error) {
	if c.Timeout > 0 {
		c.Conn.SetWriteDeadline(time.Now().Add(c.Timeout))
	}
	return c.Conn.Write(b)
}
