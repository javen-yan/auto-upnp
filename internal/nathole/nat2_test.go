package nathole

import (
	"net"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestNAT2Provider_CreateHole(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	provider := NewNAT2Provider(logger, map[string]interface{}{})

	// 启动提供者
	err := provider.Start()
	if err != nil {
		t.Fatalf("启动NAT2提供者失败: %v", err)
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

		// 等待自动协商
		time.Sleep(2 * time.Second)

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

		// 等待自动协商
		time.Sleep(2 * time.Second)

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

func TestNAT2Provider_ConnectionRestriction(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	provider := NewNAT2Provider(logger, map[string]interface{}{})

	// 启动提供者
	err := provider.Start()
	if err != nil {
		t.Fatalf("启动NAT2提供者失败: %v", err)
	}
	defer provider.Stop()

	// 创建NAT穿透
	_, err = provider.CreateHole(8080, 18080, "tcp", "测试连接限制")
	if err != nil {
		t.Fatalf("创建NAT穿透失败: %v", err)
	}
	defer provider.RemoveHole(8080, 18080, "tcp")

	// 等待一下让监听器启动
	time.Sleep(100 * time.Millisecond)

	// 尝试连接到外部端口（应该被拒绝，因为没有预先建立连接）
	conn, err := net.Dial("tcp", "127.0.0.1:18080")
	if err != nil {
		t.Logf("连接被拒绝（符合预期）: %v", err)
		return
	}
	defer conn.Close()

	// 等待一下让连接处理完成
	time.Sleep(100 * time.Millisecond)

	// 尝试发送数据来验证是否真的被拒绝
	_, err = conn.Write([]byte("test"))
	if err != nil {
		t.Logf("连接被拒绝（符合预期）: %v", err)
		return
	}

	// 尝试读取响应来验证连接是否真的被拒绝
	buffer := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	_, err = conn.Read(buffer)
	if err != nil {
		t.Logf("连接被拒绝（符合预期）: %v", err)
		return
	}

	// 如果连接成功且能发送和接收数据，说明没有实现连接限制
	t.Error("连接应该被拒绝，但连接成功了")
}

func TestNAT2Provider_AutoNegotiation(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	provider := NewNAT2Provider(logger, map[string]interface{}{})

	// 启动提供者
	err := provider.Start()
	if err != nil {
		t.Fatalf("启动NAT2提供者失败: %v", err)
	}
	defer provider.Stop()

	// 创建NAT穿透
	hole, err := provider.CreateHole(8080, 18080, "tcp", "测试自动协商")
	if err != nil {
		t.Fatalf("创建NAT穿透失败: %v", err)
	}
	defer provider.RemoveHole(8080, 18080, "tcp")

	// 等待自动协商过程
	time.Sleep(5 * time.Second)

	// 检查状态
	status := provider.GetStatus()
	t.Logf("NAT2提供者状态: %+v", status)

	// 验证自动协商是否启动
	if hole.Status != HoleStatusActive {
		t.Errorf("期望状态为Active，实际为: %s", hole.Status)
	}

	t.Log("NAT2自动协商测试完成")

	select {}
}

func TestNAT2Provider_PublicIPDetection(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	provider := NewNAT2Provider(logger, map[string]interface{}{})

	// 启动提供者
	err := provider.Start()
	if err != nil {
		t.Fatalf("启动NAT2提供者失败: %v", err)
	}
	defer provider.Stop()

	// 等待公网IP检测
	time.Sleep(3 * time.Second)

	// 获取公网IP
	publicIP := provider.GetPublicIP()
	if publicIP != "" {
		t.Logf("检测到公网IP: %s", publicIP)
	} else {
		t.Log("未检测到公网IP（可能是网络问题）")
	}

	// 检查状态
	status := provider.GetStatus()
	t.Logf("NAT2提供者状态: %+v", status)

	t.Log("NAT2公网IP检测测试完成")
}

func TestNAT2Provider_PublicAccess(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	provider := NewNAT2Provider(logger, map[string]interface{}{})

	// 启动提供者
	err := provider.Start()
	if err != nil {
		t.Fatalf("启动NAT2提供者失败: %v", err)
	}
	defer provider.Stop()

	// 等待公网IP检测
	time.Sleep(3 * time.Second)

	// 获取公网IP
	publicIP := provider.GetPublicIP()
	if publicIP == "" {
		t.Skip("未检测到公网IP，跳过公网访问测试")
	}

	t.Logf("公网IP: %s", publicIP)

	// 检查端口可访问性
	accessiblePorts := provider.GetAccessiblePorts()
	t.Logf("可访问的端口: %+v", accessiblePorts)

	// 创建NAT穿透
	_, err = provider.CreateHole(8080, 18080, "tcp", "公网访问测试")
	if err != nil {
		t.Fatalf("创建NAT穿透失败: %v", err)
	}
	defer provider.RemoveHole(8080, 18080, "tcp")

	// 等待监听器启动
	time.Sleep(1 * time.Second)

	// 检查状态
	status := provider.GetStatus()
	t.Logf("NAT2提供者状态: %+v", status)

	t.Logf("NAT2穿透已创建，可以通过以下方式访问:")
	t.Logf("公网IP: %s", publicIP)
	t.Logf("端口: 18080")
	t.Logf("协议: TCP")
	t.Logf("本地端口: 8080")

	t.Log("NAT2公网访问测试完成")
}
