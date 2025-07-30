package service

import (
	"testing"
	"time"

	"auto-upnp/config"

	"github.com/sirupsen/logrus"
)

func BenchmarkAutoUPnPService_GetStatus(b *testing.B) {
	cfg := &config.Config{}
	logger := logrus.New()

	service := NewAutoUPnPService(cfg, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.GetStatus()
	}
}

func BenchmarkAutoUPnPService_AddManualMapping(b *testing.B) {
	cfg := &config.Config{
		Admin: config.AdminConfig{
			DataDir: "benchmark_data",
		},
	}
	logger := logrus.New()

	service := NewAutoUPnPService(cfg, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		port := 8000 + i
		service.AddManualMapping(port, port, "TCP", "benchmark mapping")
	}
}

func BenchmarkPortStatusChange(b *testing.B) {
	cfg := &config.Config{
		Monitor: config.MonitorConfig{
			CheckInterval: 1 * time.Millisecond,
		},
	}
	logger := logrus.New()

	service := NewAutoUPnPService(cfg, logger)

	// 模拟端口状态变化
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		port := 8000 + (i % 100)
		service.onAutoPortStatusChanged(port, true)
		service.onAutoPortStatusChanged(port, false)
	}
}

func BenchmarkConcurrentPortMonitoring(b *testing.B) {
	cfg := &config.Config{
		Monitor: config.MonitorConfig{
			CheckInterval: 1 * time.Millisecond,
		},
	}
	logger := logrus.New()

	service := NewAutoUPnPService(cfg, logger)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			port := 8000 + (i % 100)
			service.onAutoPortStatusChanged(port, true)
			i++
		}
	})
}
