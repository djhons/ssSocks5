package main

import (
	"errors"
	"fmt"
	"github.com/hashicorp/yamux"
	"io"
	"net"
	"os"
	"ssSocksServer/socks"
	"sync"
	"time"
)

func Run(config Config, shadow func(net.Conn) net.Conn) {
	l, err := net.Listen("tcp", config.BindAddr)
	if err != nil {
		fmt.Println("[-] failed to listen on ", config.BindAddr)
		return
	}
	fmt.Println("[+] listening TCP on ", config.BindAddr)
	for {
		c, err := l.Accept()
		if err != nil {
			fmt.Println("[-] failed to accept", err)
			continue
		}
		fmt.Println("[+] received a client ", c.RemoteAddr())
		go createTunnel(config, c, shadow, func(c net.Conn) (socks.Addr, error) { return socks.Handshake(c) })
	}
}

func createTunnel(config Config, c net.Conn, shadow func(net.Conn) net.Conn, getAddr func(net.Conn) (socks.Addr, error)) {
	var session *yamux.Session
	session, err := yamux.Client(c, nil)
	if err != nil {
		fmt.Println("[-] error tcp.go")
		return
	}
	cratePort := *cratePort(config.Socks5Addr)
	go bot(config, cratePort.Addr().String(), c.RemoteAddr().String(), true)
	go func() {
		for {
			if session.IsClosed() {
				fmt.Println("[-] 连接已断开", cratePort.Addr(), c.RemoteAddr())
				cratePort.Close()
				bot(config, cratePort.Addr().String(), c.RemoteAddr().String(), false)
				return
			}
		}
	}()
	for {
		if session.IsClosed() {
			return
		}
		myConn, _ := cratePort.Accept()
		go func() {
			stream, err := session.Open()
			if err != nil {
				return
			}
			tgt, err := getAddr(myConn)
			if err != nil {
				fmt.Println("[-] failed to open socksAddr ")
				return
			}
			stream = shadow(stream)
			if _, err = stream.Write(tgt); err != nil {
				return
			}

			fmt.Println("[*] proxy ", c.RemoteAddr(), " <-> <->", tgt)
			if err = relay(stream, myConn); err != nil {
				return
			}
			return
		}()
	}
}
func cratePort(socksAddr string) *net.Listener {
	attackerPort, err := net.Listen("tcp", socksAddr)
	if err != nil {
		for {
			fmt.Println("[-] "+socksAddr, "被占用")
			addr, err := net.ResolveTCPAddr("tcp", socksAddr)
			if err != nil {
				fmt.Println("Socks5Addr解析地址错误:")
				return nil
			}
			addr.Port++
			socksAddr = addr.String()
			if !isValidIpPort(socksAddr) {
				return nil
			}
			attackerPort, err = net.Listen("tcp", socksAddr)
			if err == nil {
				break
			}
		}
	}
	fmt.Println("[+] socks5 address is ", socksAddr)
	return &attackerPort
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
