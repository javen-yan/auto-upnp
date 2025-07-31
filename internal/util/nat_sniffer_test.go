package util

import (
	"net"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestNATSniffer(t *testing.T) {
	// 创建日志器
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	// 创建NAT嗅探器
	sniffer := NewNATSniffer(logger)
	defer sniffer.Close()

	// 测试STUN服务器
	t.Run("TestSTUNServers", func(t *testing.T) {
		results := sniffer.TestAllSTUNServers()

		t.Logf("STUN服务器测试结果:")
		for server, result := range results {
			t.Logf("  %s: %v", server, result)
		}

		// 检查是否有至少一个服务器可用
		successCount := 0
		for _, result := range results {
			if result != nil && len(result.Error()) > 2 && result.Error()[:2] == "成功" {
				successCount++
			}
		}

		if successCount == 0 {
			t.Log("警告: 没有可用的STUN服务器")
		} else {
			t.Logf("成功连接到 %d 个STUN服务器", successCount)
		}
	})

	// 测试NAT类型检测
	t.Run("TestNATDetection", func(t *testing.T) {
		natInfo, err := sniffer.DetectNATType()
		if err != nil {
			t.Logf("NAT检测失败: %v", err)
			return
		}

		t.Logf("NAT检测结果:")
		t.Logf("  类型: %s", natInfo.Type.String())
		t.Logf("  描述: %s", natInfo.Description)
		t.Logf("  本地IP: %s", natInfo.LocalIP.String())
		t.Logf("  公网IP: %s:%d", natInfo.PublicIP.String(), natInfo.PublicPort)
	})

	// 测试详细NAT信息
	t.Run("TestDetailedNATInfo", func(t *testing.T) {
		detailedInfo, err := sniffer.GetDetailedNATInfo()
		if err != nil {
			t.Logf("详细NAT信息获取失败: %v", err)
			return
		}

		t.Logf("详细NAT信息:")
		t.Logf("  基本NAT类型: %s", detailedInfo.BasicInfo.Type.String())
		t.Logf("  STUN可靠性: %.2f%%", detailedInfo.Analysis.Reliability*100)
		t.Logf("  工作服务器: %d/%d", detailedInfo.Analysis.WorkingServers,
			detailedInfo.Analysis.WorkingServers+detailedInfo.Analysis.FailedServers)

		t.Logf("  建议:")
		for _, rec := range detailedInfo.Recommendations {
			t.Logf("    %s", rec)
		}
	})

	// 测试NAT友好性检查
	t.Run("TestNATFriendly", func(t *testing.T) {
		isFriendly, reason := sniffer.IsNATFriendly()
		t.Logf("NAT友好性检查:")
		t.Logf("  是否适合P2P: %v", isFriendly)
		t.Logf("  原因: %s", reason)
	})
}

func TestPrivateIPDetection(t *testing.T) {
	tests := []struct {
		ip     string
		expect bool
	}{
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"127.0.0.1", false},
	}

	for _, test := range tests {
		ip := net.ParseIP(test.ip)
		result := isPrivateIP(ip)
		if result != test.expect {
			t.Errorf("isPrivateIP(%s) = %v, 期望 %v", test.ip, result, test.expect)
		}
	}
}

func TestNATTypeString(t *testing.T) {
	tests := []struct {
		natType NATType
		expect  string
	}{
		{NATType1, "NAT1 (完全锥形NAT)"},
		{NATType2, "NAT2 (受限锥形NAT)"},
		{NATType3, "NAT3 (端口受限锥形NAT)"},
		{NATType4, "NAT4 (对称NAT)"},
		{NATTypeUnknown, "未知NAT类型"},
	}

	for _, test := range tests {
		result := test.natType.String()
		if result != test.expect {
			t.Errorf("NATType(%d).String() = %s, 期望 %s", test.natType, result, test.expect)
		}
	}
}
