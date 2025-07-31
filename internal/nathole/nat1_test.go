package nathole

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestNAT1Provider_CreateHole(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	provider := NewNAT1Provider(logger, map[string]interface{}{})

	// 启动提供者
	err := provider.Start()
	if err != nil {
		t.Fatalf("启动NAT1提供者失败: %v", err)
	}
	defer provider.Stop()

	// 测试TCP穿透
	t.Run("TCP穿透", func(t *testing.T) {
		hole, err := provider.CreateHole(8080, 8080, "tcp", "测试TCP穿透")
		if err != nil {
			t.Fatalf("创建TCP穿透失败: %v", err)
		}

		if hole.Status != HoleStatusActive {
			t.Errorf("期望状态为Active，实际为: %s", hole.Status)
		}

		if hole.LocalPort != 8080 || hole.ExternalPort != 8080 {
			t.Errorf("端口配置错误: local=%d, external=%d", hole.LocalPort, hole.ExternalPort)
		}

		// 清理
		err = provider.RemoveHole(8080, 8080, "tcp")
		if err != nil {
			t.Errorf("移除TCP穿透失败: %v", err)
		}
	})

	// 测试UDP穿透
	t.Run("UDP穿透", func(t *testing.T) {
		hole, err := provider.CreateHole(8080, 18080, "udp", "测试UDP穿透")
		if err != nil {
			t.Fatalf("创建UDP穿透失败: %v", err)
		}

		if hole.Status != HoleStatusActive {
			t.Errorf("期望状态为Active，实际为: %s", hole.Status)
		}

		if hole.LocalPort != 8080 || hole.ExternalPort != 18080 {
			t.Errorf("端口配置错误: local=%d, external=%d", hole.LocalPort, hole.ExternalPort)
		}

		// 清理
		err = provider.RemoveHole(8080, 18080, "udp")
		if err != nil {
			t.Errorf("移除UDP穿透失败: %v", err)
		}
	})

	// 测试不支持的协议
	t.Run("不支持的协议", func(t *testing.T) {
		hole, err := provider.CreateHole(8080, 8080, "icmp", "测试不支持的协议")
		if err == nil {
			t.Error("期望创建失败，但成功了")
		}

		if hole != nil && hole.Status != HoleStatusFailed {
			t.Errorf("期望状态为Failed，实际为: %s", hole.Status)
		}
	})
}

func TestNAT1Provider_ConnectionForwarding(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	provider := NewNAT1Provider(logger, map[string]interface{}{})

	// 启动提供者
	err := provider.Start()
	if err != nil {
		t.Fatalf("启动NAT1提供者失败: %v", err)
	}
	defer provider.Stop()

	// 启动一个本地TCP服务器
	localServer, err := net.Listen("tcp", ":8080")
	if err != nil {
		t.Fatalf("启动本地服务器失败: %v", err)
	}
	defer localServer.Close()

	// 处理本地服务器连接
	go func() {
		for {
			conn, err := localServer.Accept()
			if err != nil {
				return
			}
			defer conn.Close()

			// 简单的echo服务器
			buffer := make([]byte, 1024)
			n, err := conn.Read(buffer)
			if err != nil {
				continue
			}
			conn.Write(buffer[:n])
		}
	}()

	// 创建NAT穿透
	_, err = provider.CreateHole(8080, 18080, "tcp", "测试连接转发")
	if err != nil {
		t.Fatalf("创建NAT穿透失败: %v", err)
	}
	defer provider.RemoveHole(8080, 18080, "tcp")

	// 等待一下让监听器启动
	time.Sleep(100 * time.Millisecond)

	// 连接到外部端口
	conn, err := net.Dial("tcp", "127.0.0.1:18080")
	if err != nil {
		t.Fatalf("连接到外部端口失败: %v", err)
	}
	defer conn.Close()

	// 发送测试数据
	testData := "Hello, NAT1!"
	_, err = conn.Write([]byte(testData))
	if err != nil {
		t.Fatalf("发送数据失败: %v", err)
	}

	// 读取响应
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		t.Fatalf("读取响应失败: %v", err)
	}

	response := string(buffer[:n])
	if response != testData {
		t.Errorf("期望响应: %s, 实际响应: %s", testData, response)
	}

	fmt.Printf("NAT1穿透测试成功: 外部端口18080 -> 本地端口8080\n")
}

func TestNAT1Provider_PortConflict(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	provider := NewNAT1Provider(logger, map[string]interface{}{})

	// 启动提供者
	err := provider.Start()
	if err != nil {
		t.Fatalf("启动NAT1提供者失败: %v", err)
	}
	defer provider.Stop()

	// 先启动一个本地服务器占用8080端口
	localServer, err := net.Listen("tcp", ":8080")
	if err != nil {
		t.Fatalf("启动本地服务器失败: %v", err)
	}
	defer localServer.Close()

	// 尝试创建内外端口一致的NAT穿透
	hole, err := provider.CreateHole(8080, 8080, "tcp", "测试端口冲突")
	if err == nil {
		t.Error("期望创建失败，但成功了")
		provider.RemoveHole(8080, 8080, "tcp")
	} else {
		t.Logf("端口冲突测试通过，错误信息: %v", err)
	}

	if hole != nil && hole.Status != HoleStatusFailed {
		t.Errorf("期望状态为Failed，实际为: %s", hole.Status)
	}
}

func TestNAT1Provider_SamePortSuccess(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	provider := NewNAT1Provider(logger, map[string]interface{}{})

	// 启动提供者
	err := provider.Start()
	if err != nil {
		t.Fatalf("启动NAT1提供者失败: %v", err)
	}
	defer provider.Stop()

	// 测试内外端口一致但本地端口未被占用的情况
	// 使用一个不太常用的端口
	hole, err := provider.CreateHole(9999, 9999, "tcp", "测试内外端口一致")
	if err != nil {
		t.Fatalf("创建内外端口一致的NAT穿透失败: %v", err)
	}

	if hole.Status != HoleStatusActive {
		t.Errorf("期望状态为Active，实际为: %s", hole.Status)
	}

	if hole.LocalPort != 9999 || hole.ExternalPort != 9999 {
		t.Errorf("端口配置错误: local=%d, external=%d", hole.LocalPort, hole.ExternalPort)
	}

	// 清理
	err = provider.RemoveHole(9999, 9999, "tcp")
	if err != nil {
		t.Errorf("移除NAT穿透失败: %v", err)
	}

	t.Log("内外端口一致的NAT穿透创建成功")
}
