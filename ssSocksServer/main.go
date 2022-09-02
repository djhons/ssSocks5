package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"ssSocksServer/core"
	"syscall"
)

func main() {
	var flags struct {
		ServerAddr string
	}
	flag.StringVar(&flags.ServerAddr, "saddr", ":9991", "client connect port")
	flag.Parse()
	ciph, err := core.PickCipher("aes-128-gcm", nil, "Ng&t0qawhHo45672")
	if err != nil {
		log.Fatal(err)
	}
	go Run(flags.ServerAddr, ciph.StreamConn)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}
