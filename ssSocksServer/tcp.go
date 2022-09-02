package main

import (
	"errors"
	"github.com/hashicorp/yamux"
	"io"
	"math/rand"
	"net"
	"os"
	"ssSocksServer/socks"
	"strconv"
	"sync"
	"time"
)

func Run(addr string, shadow func(net.Conn) net.Conn) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		logf("failed to listen on %s: %v", addr, err)
		return
	}
	logf("[+] listening TCP on %s", addr)
	for {
		c, err := l.Accept()
		if err != nil {
			logf("[-] failed to accept: %v", err)
			continue
		}
		logf("[*] received a client %s", c.RemoteAddr())
		go createTunnel(c, shadow, func(c net.Conn) (socks.Addr, error) { return socks.Handshake(c) })
	}
}
func cratePort() *net.Listener {
	randPort := rand.Intn(65535)
	attackerPort, err := net.Listen("tcp", ":"+strconv.Itoa(randPort))
	if err != nil {
		logf("[-] greate listen port error %d", randPort)
		logf("[-] greate port error is %s", err)
		return cratePort()
	}
	logf("[+++] The randomly generated ports are %d", randPort)
	return &attackerPort
}
func createTunnel(c net.Conn, shadow func(net.Conn) net.Conn, getAddr func(net.Conn) (socks.Addr, error)) {
	var session *yamux.Session
	session, err := yamux.Client(c, nil)
	if err != nil {
		return
	}
	cratePort := *cratePort()
	for {
		myConn, _ := cratePort.Accept()
		go func() {
			defer myConn.Close()
			if session == nil {
				logf("[*] Error target not connect")
				return
			}
			stream, err := session.Open()
			if err != nil {
				return
			}
			tgt, err := getAddr(myConn)
			if err != nil {
				logf("[-] failed to get target address: %v", err)
				return
			}
			stream = shadow(stream)
			if _, err = stream.Write(tgt); err != nil {
				logf("[*] failed to send target address: %v", err)
				return
			}

			logf("proxy %s <-> <-> %s", c.RemoteAddr(), tgt)
			if err = relay(stream, myConn); err != nil {
				logf("relay error: %v", err)
			}
			return
		}()
		if session.IsClosed() { //不加这句，客户端断开会死循环
			cratePort.Close()
			logf("[*] Remote link disconnected,%s Port closed", cratePort.Addr())
			break
		}
	}
}

func relay(left, right net.Conn) error {
	var err, err1 error
	var wg sync.WaitGroup
	var wait = 5 * time.Second
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err1 = io.Copy(right, left)
		right.SetReadDeadline(time.Now().Add(wait)) // unblock read on right
	}()
	_, err = io.Copy(left, right)
	left.SetReadDeadline(time.Now().Add(wait)) // unblock read on left
	wg.Wait()
	if err1 != nil && !errors.Is(err1, os.ErrDeadlineExceeded) { // requires Go 1.15+
		return err1
	}
	if err != nil && !errors.Is(err, os.ErrDeadlineExceeded) {
		return err
	}
	return nil
}
