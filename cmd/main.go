package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"auto-upnp/config"
	"auto-upnp/internal/service"

	"github.com/sirupsen/logrus"
)

// 版本信息，通过编译时注入
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

var (
	configFile  = flag.String("config", "config.yaml", "配置文件路径")
	logLevel    = flag.String("log-level", "info", "日志级别 (debug, info, warn, error)")
	showHelp    = flag.Bool("help", false, "显示帮助信息")
	showVersion = flag.Bool("version", false, "显示版本信息")
)

func main() {
	flag.Parse()

	if *showHelp {
		showUsage()
		return
	}

	if *showVersion {
		showVersionInfo()
		return
	}

	// 设置日志级别
	level, err := logrus.ParseLevel(*logLevel)
	if err != nil {
		fmt.Printf("无效的日志级别: %s\n", *logLevel)
		os.Exit(1)
	}

	// 配置日志
	logger := logrus.New()
	logger.SetLevel(level)
	logger.SetFormatter(&logrus.JSONFormatter{})

	// 加载配置文件
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		logger.WithError(err).Fatal("加载配置文件失败")
	}

	// 创建自动UPnP服务
	autoService := service.NewAutoUPnPService(cfg, logger)

	// 启动服务
	if err := autoService.Start(); err != nil {
		logger.WithError(err).Fatal("启动自动UPnP服务失败")
	}

	// 打印启动信息
	logger.WithFields(logrus.Fields{
		"config_file": *configFile,
		"log_level":   *logLevel,
		"port_range":  fmt.Sprintf("%d-%d", cfg.PortRange.Start, cfg.PortRange.End),
	}).Info("自动UPnP服务已启动")

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 创建上下文用于优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动状态监控协程
	go func() {
		ticker := time.NewTicker(60 * time.Second) // 每分钟输出一次状态
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				status := autoService.GetStatus()
				logger.WithFields(logrus.Fields{
					"active_ports":   status["port_status"].(map[string]interface{})["active_ports"],
					"total_mappings": status["upnp_mappings"].(map[string]interface{})["total_mappings"],
				}).Info("服务状态")
			}
		}
	}()

	// 等待信号
	sig := <-sigChan
	logger.WithField("signal", sig.String()).Info("收到中断信号，开始优雅关闭")

	// 停止服务
	autoService.Stop()

	logger.Info("自动UPnP服务已停止")
}

func showUsage() {
	fmt.Println("自动UPnP服务")
	fmt.Println()
	fmt.Println("用法:")
	fmt.Printf("  %s [选项]\n", os.Args[0])
	fmt.Println()
	fmt.Println("选项:")
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("示例:")
	fmt.Printf("  %s -config config.yaml -log-level debug\n", os.Args[0])
	fmt.Printf("  %s -config /path/to/config.yaml\n", os.Args[0])
	fmt.Println()
	fmt.Println("功能:")
	fmt.Println("  1. 自动监控指定端口范围的上下线状态")
	fmt.Println("  2. 自动添加和删除UPnP端口映射")
	fmt.Println("  3. 支持手动端口映射管理")
	fmt.Println("  4. 自动清理过期的端口映射")
	fmt.Println("  5. 实时状态监控和日志记录")
}

func showVersionInfo() {
	fmt.Printf("自动UPnP服务 v%s\n", version)
	fmt.Printf("提交: %s\n", commit)
	fmt.Printf("构建时间: %s\n", date)
}
