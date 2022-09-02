package main

import (
	"flag"
	"log"
	"ssSocksClient/core"
	"time"
)

func main() {
	var flags struct {
		ConnectAddr string
		Retry       int
		RetryTime   int
	}
	flag.StringVar(&flags.ConnectAddr, "s", "", "connect server addr")
	flag.IntVar(&flags.Retry, "r", 10, "Retry count, default 10")
	flag.IntVar(&flags.RetryTime, "t", 10, "Time of each retry, 10 minutes by default")
	flag.Parse()
	ciph, err := core.PickCipher("aes-128-gcm", nil, "Ng&t0qawhHo45672")
	if err != nil {
		log.Fatal(err)
	}
	retryed := 0
	for {
		err := tcpRemote(flags.ConnectAddr, ciph.StreamConn, &retryed)
		if err != nil {
			retryed++
			time.Sleep(time.Duration(flags.RetryTime) * time.Minute)
		}
		if retryed > flags.Retry {
			break
		}
	}

}
