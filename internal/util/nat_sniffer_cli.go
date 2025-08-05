package util

import (
	"fmt"
	"log"
	"strings"
	"time"

	"auto-upnp/internal/types"
)

func NATSnifferTry() {
	fmt.Println("🔍 NATSniffer - 网络穿透嗅探器演示")
	fmt.Println(strings.Repeat("=", 50))

	// 创建NAT嗅探器
	sniffer := NewNATSniffer(nil)
	defer sniffer.Close()

	// 1. 基本NAT检测
	fmt.Println("\n📡 1. 基本NAT类型检测")
	fmt.Println(strings.Repeat("-", 30))

	startTime := time.Now()
	natInfo, err := sniffer.DetectNATType()
	if err != nil {
		log.Fatalf("❌ NAT检测失败: %v", err)
	}
	duration := time.Since(startTime)

	fmt.Printf("⏱️  检测耗时: %v\n", duration)
	fmt.Printf("🏠 本地IP: %s\n", natInfo.LocalIP.String())
	fmt.Printf("🌐 公网IP: %s:%d\n", natInfo.PublicIP.String(), natInfo.PublicPort)
	fmt.Printf("📊 NAT类型: %s\n", natInfo.Type.String())
	fmt.Printf("📝 描述: %s\n", natInfo.Description)

	// 2. STUN服务器测试
	fmt.Println("\n🌍 2. STUN服务器连接性测试")
	fmt.Println(strings.Repeat("-", 30))

	results := sniffer.TestAllSTUNServers()
	successCount := 0
	for server, result := range results {
		if result != nil && len(result.Error()) > 2 && strings.Contains(result.Error(), "成功") {
			fmt.Printf("✅ %s: %s\n", server, result.Error())
			successCount++
		} else {
			fmt.Printf("❌ %s: %v\n", server, result)
		}
	}
	fmt.Printf("\n📈 成功率: %d/%d (%.1f%%)\n",
		successCount, len(results), float64(successCount)/float64(len(results))*100)

	// 3. 详细NAT信息
	fmt.Println("\n🔬 3. 详细NAT分析")
	fmt.Println(strings.Repeat("-", 30))

	detailedInfo, err := sniffer.GetDetailedNATInfo()
	if err != nil {
		log.Printf("⚠️  详细分析失败: %v", err)
	} else {
		fmt.Printf("📊 STUN可靠性: %.1f%%\n", detailedInfo.Analysis.Reliability*100)
		fmt.Printf("🖥️  工作服务器: %d/%d\n",
			detailedInfo.Analysis.WorkingServers,
			detailedInfo.Analysis.WorkingServers+detailedInfo.Analysis.FailedServers)
	}

	// 4. NAT友好性检查
	fmt.Println("\n🤝 4. NAT友好性检查")
	fmt.Println(strings.Repeat("-", 30))

	isFriendly, reason := sniffer.IsNATFriendly()
	if isFriendly {
		fmt.Printf("✅ %s\n", reason)
	} else {
		fmt.Printf("⚠️  %s\n", reason)
	}

	// 5. 建议和推荐
	fmt.Println("\n💡 5. 网络优化建议")
	fmt.Println(strings.Repeat("-", 30))

	if detailedInfo != nil {
		for i, rec := range detailedInfo.Recommendations {
			fmt.Printf("%d. %s\n", i+1, rec)
		}
	}

	// 6. 网络环境评估
	fmt.Println("\n📋 6. 网络环境评估")
	fmt.Println(strings.Repeat("-", 30))

	assessment := getNetworkAssessment(natInfo.Type, successCount, len(results))
	fmt.Printf("🎯 总体评分: %s\n", assessment.Score)
	fmt.Printf("📊 评估结果: %s\n", assessment.Result)
	fmt.Printf("🔧 建议措施: %s\n", assessment.Action)

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("🎉 NAT检测完成！")
}

// NetworkAssessment 网络环境评估
type NetworkAssessment struct {
	Score  string
	Result string
	Action string
}

// getNetworkAssessment 获取网络环境评估
func getNetworkAssessment(natType types.NATType, successCount, totalServers int) NetworkAssessment {
	successRate := float64(successCount) / float64(totalServers)

	var score, result, action string

	// 基于NAT类型和STUN成功率评估
	switch natType {
	case types.NATType1:
		if successRate >= 0.5 {
			score = "A+ (优秀)"
			result = "网络环境非常适合P2P连接"
			action = "可以直接使用UPnP和STUN进行NAT穿透"
		} else {
			score = "B+ (良好)"
			result = "网络环境适合P2P连接，但STUN服务器连接不稳定"
			action = "建议配置更多STUN服务器或使用TURN备用方案"
		}
	case types.NATType2:
		if successRate >= 0.5 {
			score = "A (良好)"
			result = "网络环境适合P2P连接"
			action = "建议使用ICE协议和UPnP端口映射"
		} else {
			score = "B (一般)"
			result = "网络环境基本适合P2P，但需要优化"
			action = "建议使用TURN服务器作为备用方案"
		}
	case types.NATType3:
		if successRate >= 0.5 {
			score = "C+ (可接受)"
			result = "网络环境需要特殊处理"
			action = "建议使用TURN服务器进行中继"
		} else {
			score = "C (困难)"
			result = "网络环境较难穿透"
			action = "强烈建议使用TURN服务器或VPN"
		}
	case types.NATType4:
		score = "D (困难)"
		result = "网络环境最难穿透"
		action = "必须使用TURN服务器或考虑VPN方案"
	default:
		score = "F (未知)"
		result = "无法确定网络环境"
		action = "建议进行手动网络测试"
	}

	return NetworkAssessment{
		Score:  score,
		Result: result,
		Action: action,
	}
}
