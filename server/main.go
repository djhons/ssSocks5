package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/yamux"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"socks5-tunnel/common"
	"strings" // 新增引入
	"sync"
	"time"
)

type Config struct {
	TunnelPort   string `json:"tunnel_port"`
	SocksPort    string `json:"socks_port"`
	AesKey       string `json:"aes_key"`
	WeComWebhook string `json:"wecom_webhook"`
	IdleTimeout  int    `json:"idle_timeout"`
}

type ClientSession struct {
	ID       string
	Password string
	Session  *yamux.Session
}

var (
	clients = make(map[string]*ClientSession)
	mu      sync.RWMutex
	cfg     Config
	bufPool = sync.Pool{
		New: func() interface{} { return make([]byte, 32*1024) },
	}
	httpClient = &http.Client{Timeout: 5 * time.Second}
	serverIP   string // 新增：用于存储服务器公网IP
)

const DefaultIdleTimeout = 300 * time.Second

func main() {
	if err := loadConfig(); err != nil {
		log.Fatalf("Config error: %v", err)
	}

	// === 新增：启动时获取公网 IP ===
	serverIP = getPublicIP()
	log.Printf("Server Public IP: %s", serverIP)

	go startTunnelServer()
	startSocksServer()
}

// === 新增：获取公网 IP 的辅助函数 ===
func getPublicIP() string {
	resp, err := httpClient.Get("https://4.ipw.cn/")
	if err != nil {
		log.Printf("Failed to get public IP: %v", err)
		return "127.0.0.1" // 获取失败时的默认回退
	}
	defer resp.Body.Close()
	ip, err := io.ReadAll(resp.Body)
	if err != nil {
		return "127.0.0.1"
	}
	return strings.TrimSpace(string(ip))
}

func loadConfig() error {
	file, err := os.ReadFile("server.json")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(file, &cfg); err != nil {
		return err
	}
	if len(cfg.AesKey) != 32 {
		return fmt.Errorf("AES Key must be 32 bytes")
	}
	return nil
}

// --- Tunnel Server Logic ---

func startTunnelServer() {
	ln, err := net.Listen("tcp", ":"+cfg.TunnelPort)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Tunnel server listening on %s", cfg.TunnelPort)

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go handleClientHandshake(conn)
	}
}

func handleClientHandshake(conn net.Conn) {
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	cryptoConn, err := common.WrapConn(conn, []byte(cfg.AesKey))
	if err != nil {
		conn.Close()
		return
	}

	// 1. 读取 ID
	lenBuf := make([]byte, 1)
	if _, err := io.ReadFull(cryptoConn, lenBuf); err != nil {
		conn.Close()
		return
	}
	idLen := int(lenBuf[0])
	idBuf := make([]byte, idLen)
	if _, err := io.ReadFull(cryptoConn, idBuf); err != nil {
		conn.Close()
		return
	}
	clientID := string(idBuf)

	// 2. 读取 Password
	if _, err := io.ReadFull(cryptoConn, lenBuf); err != nil {
		conn.Close()
		return
	}
	passLen := int(lenBuf[0])
	passBuf := make([]byte, passLen)
	if _, err := io.ReadFull(cryptoConn, passBuf); err != nil {
		conn.Close()
		return
	}
	clientPass := string(passBuf)

	conn.SetDeadline(time.Time{})

	// 3. 注册 Session
	mu.Lock()
	if old, exists := clients[clientID]; exists {
		old.Session.Close()
		delete(clients, clientID)
	}
	mu.Unlock()

	ymConfig := yamux.DefaultConfig()
	ymConfig.EnableKeepAlive = true
	ymConfig.KeepAliveInterval = 15 * time.Second
	ymConfig.ConnectionWriteTimeout = 10 * time.Second

	session, err := yamux.Server(cryptoConn, ymConfig)
	if err != nil {
		conn.Close()
		return
	}

	mu.Lock()
	clients[clientID] = &ClientSession{
		ID:       clientID,
		Password: clientPass,
		Session:  session,
	}
	mu.Unlock()

	log.Printf("Client [%s] connected (Password set)", clientID)

	// === 修改核心：格式化通知消息 ===
	// 格式: client_id 上线\n socks5 ip 端口 socks5用户名 socks5密码
	notifyMsg := fmt.Sprintf("%s 上线\n socks5 %s %s %s %s",
		clientID,      // Client ID
		serverIP,      // 公网 IP
		cfg.SocksPort, // SOCKS5 端口
		clientID,      // 用户名 (通常SOCKS5用户名和ClientID一致)
		clientPass,    // 密码
	)
	go sendWeComNotify(notifyMsg)

	go func() {
		<-session.CloseChan()
		mu.Lock()
		if current, ok := clients[clientID]; ok && current.Session == session {
			delete(clients, clientID)
			log.Printf("Client [%s] disconnected", clientID)
			go sendWeComNotify(fmt.Sprintf("Client [%s] 掉线了", clientID))
		}
		mu.Unlock()
	}()
}

// --- SOCKS5 Server Logic ---
// (此部分代码保持不变，与原代码一致)
func startSocksServer() {
	ln, err := net.Listen("tcp", ":"+cfg.SocksPort)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("SOCKS5 server listening on %s", cfg.SocksPort)

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go handleSocks5(conn)
	}
}

func handleSocks5(conn net.Conn) {
	defer conn.Close()
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return
	}
	nMethods := int(header[1])
	methods := make([]byte, nMethods)
	if _, err := io.ReadFull(conn, methods); err != nil {
		return
	}
	conn.Write([]byte{0x05, 0x02})

	authHeader := make([]byte, 2)
	if _, err := io.ReadFull(conn, authHeader); err != nil {
		return
	}
	if authHeader[0] != 0x01 {
		return
	}
	uLen := int(authHeader[1])
	userBuf := make([]byte, uLen)
	if _, err := io.ReadFull(conn, userBuf); err != nil {
		return
	}
	username := string(userBuf)
	pLenBuf := make([]byte, 1)
	if _, err := io.ReadFull(conn, pLenBuf); err != nil {
		return
	}
	pLen := int(pLenBuf[0])
	passBuf := make([]byte, pLen)
	if _, err := io.ReadFull(conn, passBuf); err != nil {
		return
	}
	providedPassword := string(passBuf)

	mu.RLock()
	clientSession, ok := clients[username]
	mu.RUnlock()

	if !ok || clientSession.Password != providedPassword {
		log.Printf("Auth failed for user: %s", username)
		conn.Write([]byte{0x01, 0x01})
		return
	}
	conn.Write([]byte{0x01, 0x00})

	stream, err := clientSession.Session.Open()
	if err != nil {
		return
	}
	defer stream.Close()

	timeout := DefaultIdleTimeout
	if cfg.IdleTimeout > 0 {
		timeout = time.Duration(cfg.IdleTimeout) * time.Second
	}
	transport(
		&TimeoutConn{Conn: conn, Timeout: timeout},
		&TimeoutConn{Conn: stream, Timeout: timeout},
	)
}

func transport(user, tunnel net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	copyFunc := func(dst, src net.Conn) {
		defer wg.Done()
		buf := bufPool.Get().([]byte)
		defer bufPool.Put(buf)
		io.CopyBuffer(dst, src, buf)
		dst.Close()
	}
	go copyFunc(user, tunnel)
	go copyFunc(tunnel, user)
	wg.Wait()
}

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

func sendWeComNotify(msg string) {
	if cfg.WeComWebhook == "" {
		return
	}
	defer func() { recover() }()
	payload := map[string]interface{}{"msgtype": "text", "text": map[string]string{"content": msg}}
	body, _ := json.Marshal(payload)
	resp, err := httpClient.Post(cfg.WeComWebhook, "application/json", bytes.NewBuffer(body))
	if err == nil {
		resp.Body.Close()
	}
}
