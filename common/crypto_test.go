package common

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"
)

func pipe(key []byte) (cli, srv net.Conn) {
	cA, cB := net.Pipe()
	cli, _ = WrapConn(cA, key, true)
	srv, _ = WrapConn(cB, key, false)
	return cli, srv
}

func TestCryptoRoundTrip(t *testing.T) {
	key := []byte("01234567890123456789012345678901") // 32 bytes
	cli, srv := pipe(key)
	defer cli.Close()
	defer srv.Close()

	go cli.Write([]byte("hello server"))
	cA := cli.(*CryptoConn)
	cA.Conn.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 64)
	n, err := srv.Read(buf)
	if err != nil {
		t.Fatalf("srv read: %v", err)
	}
	if string(buf[:n]) != "hello server" {
		t.Fatalf("got %q", buf[:n])
	}

	go srv.Write([]byte("hello client"))
	s := srv.(*CryptoConn)
	s.Conn.SetReadDeadline(time.Now().Add(time.Second))
	n, err = cli.Read(buf)
	if err != nil {
		t.Fatalf("cli read: %v", err)
	}
	if string(buf[:n]) != "hello client" {
		t.Fatalf("got %q", buf[:n])
	}
}

func TestCryptoLargeReadFull(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	cli, srv := pipe(key)
	defer cli.Close()
	defer srv.Close()

	payload := bytes.Repeat([]byte("ABCDEFGH"), 10000) // 80KB > 64KB single frame
	go func() { cli.Write(payload) }()

	s := srv.(*CryptoConn)
	s.Conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	got := make([]byte, len(payload))
	if _, err := io.ReadFull(srv, got); err != nil {
		t.Fatalf("read full: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("payload mismatch: got %d bytes want %d", len(got), len(payload))
	}
}
