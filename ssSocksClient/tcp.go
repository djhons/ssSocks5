package main

import (
	"errors"
	"github.com/hashicorp/yamux"
	"io"
	"io/ioutil"
	"net"
	"os"
	"ssSocksClient/socks"
	"sync"
	"time"
)

var session *yamux.Session

func tcpRemote(addr string, shadow func(net.Conn) net.Conn, retryed *int) error {
	l, err := net.Dial("tcp", addr)
	if err != nil {
		logf("failed to connect on %s: %v", addr, err)
		return err
	}
	logf("connected")
	*retryed = 0
	session, err = yamux.Server(l, nil)
	defer session.Close()
	defer l.Close()
	for {
		c, err := session.Accept()
		if err != nil {
			logf("[X]  Disconnected and reconnecting %s", err.Error())
			return err
		}
		go func() {
			defer c.Close()
			sc := shadow(c)
			tgt, err := socks.ReadAddr(sc)
			if err != nil {
				_, _ = io.Copy(ioutil.Discard, c)
				return
			}
			rc, err := net.Dial("tcp", tgt.String())
			if err != nil {
				return
			}
			defer rc.Close()
			if err = relay(sc, rc); err != nil {
			}
		}()
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
