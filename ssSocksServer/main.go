package main

import (
	"log"
	"ssSocksServer/core"
)

func main() {
	// 加载配置文件
	config, err := readConfig("config.ini")
	if err != nil {
		return
	}
	ciph, err := core.PickCipher(config.BlockMode, nil, config.BlockKey)
	if err != nil {
		log.Fatal(err)
	}
	Run(config, ciph.StreamConn)
}
