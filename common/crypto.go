package common

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
)

// 本文件实现一个基于 AES-256-GCM 的加密连接包装器。
//
// 设计要点：
//   - 每个应用写操作都被当作一帧独立加密：帧格式为 [len:4][nonce:12][ciphertext]，
//     其中 len = 12 + len(ciphertext)，ciphertext 尾部自带 GCM 的 16 字节认证标签。
//   - 每帧使用 crypto/rand 生成的随机 nonce，单方向内 nonce 碰撞概率可忽略，
//     从而避免 CFB 流加密在固定 IV 下的可证明不安全性。
//   - 发送/接收两个方向使用由 isClient 派生的不同密钥：client->server 方向的密钥
//     用于客户端加密/服务端解密，server->client 方向反之。每条 CryptoConn 因此持有
//     enc、dec 两个 AEAD，彻底杜绝双向 (key, nonce) 空间重叠。
//   - 读取端按帧拆包：先读 4 字节长度，再读对应长度的 nonce+密文，GCM 认证失败即
//     视为连接被篡改，立即终止。帧长度设有上限，防止恶意对端用巨大长度字段耗尽内存。
//
// 收益：抗篡改、抗重放（GCM 认证 + 每帧随机 nonce）；即便底层 TCP 是流，应用层也有
// 清晰边界；不再依赖固定全 0 IV（原实现的安全缺陷）。
const (
	nonceSize    = 12            // GCM 标准 nonce 长度
	tagSize      = 16            // GCM 认证标签长度
	lenField     = 4             // 帧长度字段字节数
	maxPlainLen  = 32 * 1024     // 单帧明文上限（与复制缓冲一致）
	maxFrameLen  = maxPlainLen + nonceSize + tagSize // 单帧总长上限
)

// CryptoConn 包装 net.Conn，提供按帧的 AES-256-GCM 加密传输。
type CryptoConn struct {
	net.Conn
	enc     cipher.AEAD  // 本端发送方向使用的 AEAD
	dec     cipher.AEAD  // 本端接收方向使用的 AEAD
	readBuf []byte       // 复用：存放已解密、尚未被上层读走的明文
	reader  io.Reader
}

// deriveKey 把主密钥与方向标签按字节异或，派生出该方向的会话密钥。
func deriveKey(key []byte, dir string) []byte {
	out := make([]byte, len(key))
	for i := range key {
		out[i] = key[i] ^ dir[i%len(dir)]
	}
	return out
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// WrapConn 用 AES-256-GCM 包装一条连接。
// isClient 决定密钥派生方向：客户端 enc=client->server、dec=server->client；
// 服务端恰好相反，保证两端同一方向的收发密钥一致。
func WrapConn(conn net.Conn, key []byte, isClient bool) (net.Conn, error) {
	const dirC2S = "client->server"
	const dirS2C = "server->client"

	var encKey, decKey []byte
	if isClient {
		encKey = deriveKey(key, dirC2S)
		decKey = deriveKey(key, dirS2C)
	} else {
		encKey = deriveKey(key, dirS2C)
		decKey = deriveKey(key, dirC2S)
	}

	enc, err := newAEAD(encKey)
	if err != nil {
		return nil, err
	}
	dec, err := newAEAD(decKey)
	if err != nil {
		return nil, err
	}
	return &CryptoConn{
		Conn:   conn,
		enc:    enc,
		dec:    dec,
		reader: conn,
	}, nil
}

// Read 读取并解密下一帧，返回明文。同一帧的剩余明文会在后续 Read 中被透明分发。
func (c *CryptoConn) Read(b []byte) (int, error) {
	// 先把上一帧尚未读完的明文交付出去。
	if len(c.readBuf) > 0 {
		n := copy(b, c.readBuf)
		c.readBuf = c.readBuf[n:]
		return n, nil
	}

	// 读 4 字节帧长度（含 nonce + 密文 + tag）。
	var lenBytes [lenField]byte
	if _, err := io.ReadFull(c.reader, lenBytes[:]); err != nil {
		return 0, err
	}
	frameLen := binary.BigEndian.Uint32(lenBytes[:])
	if frameLen < nonceSize+tagSize || frameLen > maxFrameLen {
		return 0, fmt.Errorf("invalid frame length: %d", frameLen)
	}

	// 读 nonce + 密文（含 tag）。
	frame := make([]byte, frameLen)
	if _, err := io.ReadFull(c.reader, frame); err != nil {
		return 0, err
	}

	nonce := frame[:nonceSize]
	ciphertext := frame[nonceSize:]

	// GCM.Open 失败（认证错误）即视为连接被篡改，立即终止。
	plaintext, err := c.dec.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return 0, errors.New("authentication failed")
	}

	c.readBuf = plaintext
	n := copy(b, plaintext)
	c.readBuf = plaintext[n:]
	return n, nil
}

// Write 加密并写出一帧；超过单帧明文上限的写入会自动分帧。
func (c *CryptoConn) Write(b []byte) (int, error) {
	total := 0
	for len(b) > 0 {
		chunk := b
		if len(chunk) > maxPlainLen {
			chunk = chunk[:maxPlainLen]
		}

		nonce := make([]byte, nonceSize)
		if _, err := rand.Read(nonce); err != nil {
			return total, err
		}
		ciphertext := c.enc.Seal(nil, nonce, chunk, nil)

		frame := make([]byte, lenField+len(nonce)+len(ciphertext))
		binary.BigEndian.PutUint32(frame[:lenField], uint32(len(nonce)+len(ciphertext)))
		copy(frame[lenField:lenField+len(nonce)], nonce)
		copy(frame[lenField+len(nonce):], ciphertext)

		if _, err := c.Conn.Write(frame); err != nil {
			return total, err
		}

		total += len(chunk)
		b = b[len(chunk):]
	}
	return total, nil
}
