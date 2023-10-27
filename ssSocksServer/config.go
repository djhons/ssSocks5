package main

import (
	"errors"
	"fmt"
	"gopkg.in/ini.v1"
	"os"
	"regexp"
)

type Config struct {
	BindAddr   string
	BlockMode  string
	BlockKey   string
	Socks5Addr string
	DeleteMe   bool
}

func readConfig(configFile string) (config Config, err error) {
	cfg, err := ini.Load(configFile)
	if err != nil {
		fmt.Println("无法加载配置文件:", err)
		return config, err
	}

	// 解析配置文件中的数据
	config = Config{}
	err = cfg.Section("").MapTo(&config)
	if config.DeleteMe {
		os.Remove(configFile)
	}
	if err != nil {
		fmt.Println("无法解析配置文件:", err)
		return config, err
	}
	// 校验地址是否ip+端口的形式
	if !isValidIpPort(config.BindAddr) {
		fmt.Println("BindAddr不符合要求，格式为ip加端口")
		return config, errors.New("BindAddr不符合要求")
	}
	if !isValidIpPort(config.Socks5Addr) {
		fmt.Println("Socks5Addr不符合要求，格式为ip加端口")
		return config, errors.New("Socks5Addr不符合要求")
	}
	// 检查ApiKey长度是否符合要求
	if len(config.BlockKey) != 16 {
		fmt.Println("BlockKey长度不符合要求，只能16位")
		return config, errors.New("BlockKey长度不符合要求")
	}
	return config, nil
}

// 校验地址是否ip+端口的形式
func isValidIpPort(address string) bool {

	// 使用正则表达式验证字符串格式
	pattern := `^((25[0-5]|2[0-4]\d|1\d{2}|[1-9]\d|\d)\.){3}(25[0-5]|2[0-4]\d|1\d{2}|[1-9]\d|\d):(\d{0,4}|[1-5]\d{4}|6[0-5]{2}[0-3][0-5])$`
	match, _ := regexp.MatchString(pattern, address)
	return match
}
