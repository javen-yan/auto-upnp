package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"auto-upnp/config"
	"auto-upnp/internal/admin"
	"auto-upnp/internal/service"
	"auto-upnp/internal/util"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// 版本信息，通过编译时注入
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

// 全局变量
var (
	configFile string
	logLevel   string
	natSniffer bool
)

// rootCmd 表示没有调用子命令时的基础命令
var rootCmd = &cobra.Command{
	Use:   "auto-upnp",
	Short: "自动UPnP服务",
	Long: `自动UPnP服务是一个用于自动监控和管理UPnP端口映射的工具。

功能特性:
  1. 自动监控指定端口范围的上下线状态
  2. 自动添加和删除UPnP端口映射
  3. 支持手动端口映射管理
  4. 自动清理过期的端口映射
  5. 实时状态监控和日志记录`,
	RunE: runMain,
}

// versionCmd 显示版本信息
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "显示版本信息",
	Run: func(cmd *cobra.Command, args []string) {
		showVersionInfo()
	},
}

// natSnifferCmd NAT嗅探命令
var natSnifferCmd = &cobra.Command{
	Use:   "nat-sniffer",
	Short: "启用NAT嗅探",
	RunE: func(cmd *cobra.Command, args []string) error {
		// 启用NAT嗅探
		util.NATSnifferTry()
		return nil
	},
}

func init() {
	// 添加持久标志到 root 命令
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "config.yaml", "配置文件路径")
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "l", "info", "日志级别 (debug, info, warn, error)")
	rootCmd.PersistentFlags().BoolVar(&natSniffer, "nat-sniffer", false, "启用NAT嗅探")

	// 添加子命令
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(natSnifferCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}

func runMain(cmd *cobra.Command, args []string) error {
	// 如果启用了NAT嗅探，直接执行NAT嗅探功能
	if natSniffer {
		return natSnifferCmd.RunE(cmd, args)
	}

	// 加载配置文件
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("加载配置文件失败: %w", err)
	}

	// 使用新的日志配置系统
	logger, err := util.SetupLogger(cfg, logLevel)
	if err != nil {
		return fmt.Errorf("设置日志配置失败: %w", err)
	}

	// 创建系统服务
	if err := service.NewSystemService(cfg); err != nil {
		logger.WithError(err).Fatal("创建系统服务失败")
		os.Exit(1)
	}

	// 创建自动UPnP服务
	autoService := service.NewAutoUPnPService(cfg, logger)

	// 启动服务
	if err := autoService.Start(); err != nil {
		logger.WithError(err).Fatal("启动自动UPnP服务失败")
	}

	// 创建并启动HTTP管理服务
	adminServer := admin.NewAdminServer(cfg, logger, autoService)
	if err := adminServer.Start(); err != nil {
		logger.WithError(err).Fatal("启动HTTP管理服务失败")
	}

	// 获取日志配置信息
	logConfigInfo := util.GetLogConfigInfo(cfg, logLevel)

	// 打印启动信息
	logger.WithFields(logrus.Fields{
		"config_file":   configFile,
		"port_range":    fmt.Sprintf("%d-%d", cfg.PortRange.Start, cfg.PortRange.End),
		"admin_port":    adminServer.GetPort(),
		"nat_traversal": cfg.NATTraversal.Enabled,
		"log_config":    logConfigInfo,
	}).Info("自动UPnP服务已启动")

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 等待信号
	sig := <-sigChan
	logger.WithField("signal", sig.String()).Info("收到中断信号，开始优雅关闭")

	// 停止服务
	autoService.Stop()
	adminServer.Stop()

	logger.Info("自动UPnP服务已停止")
	return nil
}

func showVersionInfo() {
	fmt.Printf("自动UPnP服务 %s\n", version)
	fmt.Printf("提交: %s\n", commit)
	fmt.Printf("构建时间: %s\n", date)
}
