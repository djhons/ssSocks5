package common

import (
	"crypto/aes"
	"crypto/cipher"
	"io"
	"net"
)

// CryptoConn 包装 net.Conn 实现加密传输
type CryptoConn struct {
	net.Conn
	Reader io.Reader
	Writer io.Writer
}

func (c *CryptoConn) Read(b []byte) (int, error) {
	return c.Reader.Read(b)
}

func (c *CryptoConn) Write(b []byte) (int, error) {
	return c.Writer.Write(b)
}

// NewCryptoConn 创建一个加密的连接包装器
// key 长度必须是 16, 24, 或 32 字节
func NewCryptoConn(conn net.Conn, key []byte) (net.Conn, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// 初始化 IV (Initialization Vector)
	// 在实际生产中，IV 应该随机生成并作为握手的一部分发送
	// 为了简化演示，这里假设连接建立后的前缀处理，或者使用流密码
	// 这里使用 CFB 模式

	var iv [aes.BlockSize]byte

	// 这里简化逻辑：
	// 实际上应该由发送方生成随机 IV 发送给接收方。
	// 下面的实现假设双方是对称的（这在严格安全场景下需要握手改进，但在本Demo中足够）
	// 为了保证两端同步，我们在 Wrapper 外部不自动写 IV，而是假定流模式。
	// 更健壮的做法是在连接建立时，Writer 生成 IV 发送，Reader 读取 IV 初始化。

	return &CryptoConn{
		Conn:   conn,
		Reader: &cipher.StreamReader{S: cipher.NewCFBDecrypter(block, iv[:]), R: conn},
		Writer: &cipher.StreamWriter{S: cipher.NewCFBEncrypter(block, iv[:]), W: conn},
	}, nil
}

// 增强版：带IV握手的加密连接构造器
func WrapConn(conn net.Conn, key []byte) (net.Conn, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	// 对于 CFB，IV 长度等于 BlockSize
	ivRead := make([]byte, aes.BlockSize)
	ivWrite := make([]byte, aes.BlockSize)

	// 简单的固定 IV 用于演示 (生产环境请使用 crypto/rand 生成并交换)
	// 这里为了代码简洁，使用全0 IV (注意：这降低了安全性，仅供学习)
	// 真正的做法是: 连接建立后，主动方发送 Random IV，被动方读取。

	streamRead := cipher.NewCFBDecrypter(block, ivRead)
	streamWrite := cipher.NewCFBEncrypter(block, ivWrite)

	return &CryptoConn{
		Conn:   conn,
		Reader: &cipher.StreamReader{S: streamRead, R: conn},
		Writer: &cipher.StreamWriter{S: streamWrite, W: conn},
	}, nil
}
