package service

import (
	"testing"
	"time"

	"auto-upnp/config"

	"github.com/sirupsen/logrus"
)

func TestNewAutoUPnPService(t *testing.T) {
	cfg := &config.Config{}
	logger := logrus.New()

	service := NewAutoUPnPService(cfg, logger)

	if service == nil {
		t.Fatal("服务创建失败")
	}

	if service.config != cfg {
		t.Error("配置未正确设置")
	}

	if service.logger != logger {
		t.Error("日志器未正确设置")
	}
}

func TestAutoUPnPService_StartStop(t *testing.T) {
	cfg := &config.Config{
		Monitor: config.MonitorConfig{
			CheckInterval:   100 * time.Millisecond,
			CleanupInterval: 1 * time.Second,
		},
		UPnP: config.UPnPConfig{
			DiscoveryTimeout:    1 * time.Second,
			HealthCheckInterval: 1 * time.Second,
		},
	}
	logger := logrus.New()

	service := NewAutoUPnPService(cfg, logger)

	// 启动服务
	err := service.Start()
	if err != nil {
		t.Fatalf("启动服务失败: %v", err)
	}

	// 等待一段时间
	time.Sleep(100 * time.Millisecond)

	// 停止服务
	service.Stop()

	// 验证服务已停止
	time.Sleep(100 * time.Millisecond)
}

func TestAutoUPnPService_GetStatus(t *testing.T) {
	cfg := &config.Config{}
	logger := logrus.New()

	service := NewAutoUPnPService(cfg, logger)

	status := service.GetStatus()

	// 验证状态包含必要字段
	requiredFields := []string{"uptime", "active_ports", "inactive_ports", "total_mappings"}
	for _, field := range requiredFields {
		if _, exists := status[field]; !exists {
			t.Errorf("状态缺少字段: %s", field)
		}
	}
}

func TestAutoUPnPService_AddRemoveManualMapping(t *testing.T) {
	cfg := &config.Config{
		Admin: config.AdminConfig{
			DataDir: "test_data",
		},
	}
	logger := logrus.New()

	service := NewAutoUPnPService(cfg, logger)

	// 添加手动映射
	err := service.AddManualMapping(8080, 8080, "TCP", "test mapping")
	if err != nil {
		t.Fatalf("添加手动映射失败: %v", err)
	}

	// 验证映射已添加
	mappings := service.GetManualMappings()
	if len(mappings) == 0 {
		t.Error("手动映射未正确添加")
	}

	// 删除手动映射
	err = service.RemoveManualMapping(8080, 8080, "TCP")
	if err != nil {
		t.Fatalf("删除手动映射失败: %v", err)
	}

	// 验证映射已删除
	mappings = service.GetManualMappings()
	if len(mappings) > 0 {
		t.Error("手动映射未正确删除")
	}
}
