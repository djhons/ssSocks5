package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

func bot(config Config, socks5 string, remoteAddr string, status bool) {
	socks5 = publicIp() + socks5[4:]
	switch config.BotType {
	case "wchatWork":
		fmt.Println("[+] send bot", socks5, remoteAddr)
		wchatWork(config.BotKey, socks5, remoteAddr, status)
	default:
		return
	}
}
func publicIp() string {
	client := http.Client{
		Timeout: 3 * time.Second,
	}

	resp, err := client.Get("https://api.ipify.org?format=text")
	if err != nil {
		return "0.0.0.0"
	}
	defer resp.Body.Close()

	ip, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "0.0.0.0"
	}
	return string(ip)
}
func wchatWork(key string, socks5 string, remoteAddr string, status bool) bool {
	url := "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=" + key
	data := ""
	if status {
		data = "{     \"msgtype\": \"markdown\",     \"markdown\": {         \"content\": \"新上线了一个socks5代理，兄弟们快冲。\\n          >socks5:<font color=\\\"comment\\\">" + socks5 + "</font>          \\n>client:<font color=\\\"comment\\\">" + remoteAddr + "</font>\"     } }"
	} else {
		data = "{     \"msgtype\": \"markdown\",     \"markdown\": {         \"content\": \"<font color=\\\"warning\\\">代理掉辣，兄弟们别急。</font>\\n          >socks5:<font color=\\\"comment\\\">" + socks5 + "</font>          \\n>client:<font color=\\\"comment\\\">" + remoteAddr + "</font>\"     } }"
	}
	return postApi(url, data, "application/json")
}

func postApi(url string, data string, contentType string) bool {
	// 将JSON数据转换为字节数组
	jsonData := []byte(data)
	// 创建一个带有超时设置的HTTP客户端
	client := &http.Client{
		Timeout: 10 * time.Second, // 设置超时时间为10秒
	}

	// 创建POST请求
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Println("Failed to create request:", err)
		return false
	}
	req.Header.Set("Content-Type", contentType)

	// 发送请求并获取响应
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("HTTP request failed:", err)
		return false
	}
	defer resp.Body.Close()

	// 处理响应
	if resp.StatusCode == http.StatusOK {
		return true
		// 在这里处理成功的响应
	} else {
		fmt.Println("POST request failed with status code:", resp.StatusCode)
		return false
		// 在这里处理失败的响应
	}
}
