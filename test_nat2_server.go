package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"auto-upnp/internal/nathole"

	"github.com/sirupsen/logrus"
)

func main() {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	// 创建NAT2提供者
	provider := nathole.NewNAT2Provider(logger, map[string]interface{}{})

	// 启动提供者
	err := provider.Start()
	if err != nil {
		log.Fatalf("启动NAT2提供者失败: %v", err)
	}
	defer provider.Stop()

	// 等待公网IP检测
	time.Sleep(3 * time.Second)

	// 获取公网IP
	publicIP := provider.GetPublicIP()
	if publicIP == "" {
		log.Fatal("未检测到公网IP")
	}

	fmt.Printf("公网IP: %s\n", publicIP)

	// 启动本地HTTP服务器
	go func() {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "Hello from NAT2! Time: %s\n", time.Now().Format("2006-01-02 15:04:05"))
		})

		log.Printf("启动本地HTTP服务器在端口9999")
		if err := http.ListenAndServe(":9999", nil); err != nil {
			log.Printf("HTTP服务器错误: %v", err)
		}
	}()

	// 创建NAT2穿透
	_, err = provider.CreateHole(9999, 18080, "tcp", "公网访问测试")
	if err != nil {
		log.Fatalf("创建NAT穿透失败: %v", err)
	}
	defer provider.RemoveHole(9999, 18080, "tcp")

	fmt.Printf("NAT2穿透已创建!\n")
	fmt.Printf("公网访问地址: http://%s:18080\n", publicIP)
	fmt.Printf("本地访问地址: http://localhost:9999\n")
	fmt.Printf("按Ctrl+C停止...\n")

	// 保持运行
	select {}
}
