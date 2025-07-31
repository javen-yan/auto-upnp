package util

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/pion/stun"
)

// NATSniffer 网络穿透嗅探器
type NATSniffer struct {
	ctx         context.Context
	cancel      context.CancelFunc
	stunServers []string
}

// NATType 网络类型枚举
type NATType int

const (
	NATTypeUnknown NATType = iota
	NATType1               // 完全锥形NAT (Full Cone NAT)
	NATType2               // 受限锥形NAT (Restricted Cone NAT)
	NATType3               // 端口受限锥形NAT (Port Restricted Cone NAT)
	NATType4               // 对称NAT (Symmetric NAT)
)

// NATInfo NAT信息
type NATInfo struct {
	Type        NATType
	PublicIP    net.IP
	PublicPort  int
	LocalIP     net.IP
	LocalPort   int
	Description string
}

func (n *NATInfo) ToDetail() *NATDetail {
	return &NATDetail{
		Type:        n.Type.String(),
		PublicIP:    n.PublicIP.String(),
		PublicPort:  n.PublicPort,
		LocalIP:     n.LocalIP.String(),
		LocalPort:   n.LocalPort,
		Description: n.Description,
	}
}

type NATDetail struct {
	Type        string `json:"type"`
	PublicIP    string `json:"public_ip"`
	PublicPort  int    `json:"public_port"`
	LocalIP     string `json:"local_ip"`
	LocalPort   int    `json:"local_port"`
	Description string `json:"description"`
}

// 公共STUN服务器列表
var PublicSTUNServers = []string{
	"stun.miwifi.com:3478",
	"stun.chat.bilibili.com:3478",
	"stun.hitv.com:3478",
	"stun.cdnbye.com:3478",
}

// NewNATSniffer 创建新的NAT嗅探器
func NewNATSniffer() *NATSniffer {
	ctx, cancel := context.WithCancel(context.Background())
	return &NATSniffer{
		ctx:         ctx,
		cancel:      cancel,
		stunServers: PublicSTUNServers,
	}
}

// Close 关闭NAT嗅探器
func (n *NATSniffer) Close() {
	if n.cancel != nil {
		n.cancel()
	}
}

// DetectNATType 检测NAT类型
func (n *NATSniffer) DetectNATType() (*NATInfo, error) {
	fmt.Println("开始检测NAT类型...")

	// 获取本地IP
	localIP, err := n.getLocalIP()
	if err != nil {
		return nil, fmt.Errorf("获取本地IP失败: %w", err)
	}

	// 通过STUN服务器获取公网IP
	publicIP, publicPort, err := n.getPublicIP()
	if err != nil {
		return nil, fmt.Errorf("获取公网IP失败: %w", err)
	}

	// 检测NAT类型
	natType, description, err := n.classifyNATType(localIP, publicIP, publicPort)
	if err != nil {
		return nil, fmt.Errorf("分类NAT类型失败: %w", err)
	}

	natInfo := &NATInfo{
		Type:        natType,
		PublicIP:    publicIP,
		PublicPort:  publicPort,
		LocalIP:     localIP,
		LocalPort:   0, // 本地端口在STUN测试中可能变化
		Description: description,
	}

	fmt.Printf("NAT检测完成: %s \n", description)
	return natInfo, nil
}

// getLocalIP 获取本地IP地址
func (n *NATSniffer) getLocalIP() (net.IP, error) {
	// 尝试连接到一个外部地址来获取本地IP
	conn, err := net.Dial("udp", "8.8.8.8:53")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP, nil
}

// getPublicIP 通过STUN服务器获取公网IP
func (n *NATSniffer) getPublicIP() (net.IP, int, error) {
	var lastErr error

	for _, server := range n.stunServers {
		ip, port, err := n.querySTUNServer(server)
		if err != nil {
			lastErr = err
			fmt.Printf("STUN服务器 %s 查询失败: %v \n", server, err)
			continue
		}

		fmt.Printf("通过STUN服务器 %s 获取到公网IP: %s:%d \n", server, ip.String(), port)
		return ip, port, nil
	}

	return nil, 0, fmt.Errorf("所有STUN服务器查询失败: %w", lastErr)
}

// querySTUNServer 查询单个STUN服务器
func (n *NATSniffer) querySTUNServer(server string) (net.IP, int, error) {
	// 创建UDP连接
	conn, err := net.Dial("udp", server)
	if err != nil {
		return nil, 0, err
	}
	defer conn.Close()

	// 设置超时
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	// 创建STUN消息
	message := stun.MustBuild(stun.TransactionID, stun.BindingRequest)

	// 发送STUN请求
	_, err = conn.Write(message.Raw)
	if err != nil {
		return nil, 0, err
	}

	// 读取响应
	buffer := make([]byte, 1024)
	readBytes, err := conn.Read(buffer)
	if err != nil {
		return nil, 0, err
	}

	// 解析STUN响应
	var response stun.Message
	if err := stun.Decode(buffer[:readBytes], &response); err != nil {
		return nil, 0, err
	}

	// 提取映射地址
	var xorAddr stun.XORMappedAddress
	if err := xorAddr.GetFrom(&response); err != nil {
		return nil, 0, err
	}

	return xorAddr.IP, xorAddr.Port, nil
}

// classifyNATType 分类NAT类型
func (n *NATSniffer) classifyNATType(localIP, publicIP net.IP, publicPort int) (NATType, string, error) {
	fmt.Printf("开始分类NAT类型 - 本地IP: %s, 公网IP: %s:%d \n", localIP.String(), publicIP.String(), publicPort)

	// 1. 检查是否为公网IP（无NAT）
	if localIP.Equal(publicIP) {
		return NATType1, "完全锥形NAT (Full Cone NAT) - 公网IP与本地IP相同，可能无NAT或完全锥形NAT", nil
	}

	// 2. 检查是否为私有IP
	if !isPrivateIP(localIP) {
		return NATTypeUnknown, "未知NAT类型 - 本地IP不是私有IP", nil
	}

	// 3. 检查公网IP是否为私有IP（可能是双重NAT）
	if isPrivateIP(publicIP) {
		return NATType4, "对称NAT (Symmetric NAT) - 公网IP也是私有IP，可能是双重NAT", nil
	}

	// 4. 进行更详细的NAT类型检测
	return n.performDetailedNATTest(localIP, publicIP, publicPort)
}

// performDetailedNATTest 执行详细的NAT类型测试
func (n *NATSniffer) performDetailedNATTest(localIP, publicIP net.IP, publicPort int) (NATType, string, error) {
	// 测试多个STUN服务器来检测端口映射行为
	portMappings := make(map[int]bool)
	ipMappings := make(map[string]bool)

	// 测试多个STUN服务器
	for i, server := range n.stunServers {
		if i >= 3 { // 限制测试数量
			break
		}

		ip, port, err := n.querySTUNServer(server)
		if err != nil {
			fmt.Printf("STUN服务器 %s 测试失败: %v \n", server, err)
			continue
		}

		portMappings[port] = true
		ipMappings[ip.String()] = true

		fmt.Printf("STUN服务器 %s 返回: %s:%d \n", server, ip.String(), port)
	}

	// 分析映射行为
	uniquePorts := len(portMappings)
	uniqueIPs := len(ipMappings)

	fmt.Printf("NAT映射分析 - 唯一端口数: %d, 唯一IP数: %d \n", uniquePorts, uniqueIPs)

	// 基于映射行为判断NAT类型
	if uniqueIPs == 1 && uniquePorts == 1 {
		// 所有测试返回相同的IP和端口
		return NATType1, "完全锥形NAT (Full Cone NAT) - 所有STUN服务器返回相同的映射", nil
	} else if uniqueIPs == 1 && uniquePorts > 1 {
		// 相同IP但不同端口
		return NATType2, "受限锥形NAT (Restricted Cone NAT) - 相同IP但端口映射变化", nil
	} else if uniqueIPs > 1 {
		// 不同IP映射
		return NATType4, "对称NAT (Symmetric NAT) - 不同STUN服务器返回不同IP映射", nil
	} else {
		// 默认情况，基于统计概率
		return NATType3, "端口受限锥形NAT (Port Restricted Cone NAT) - 最常见的NAT类型", nil
	}
}

// isPrivateIP 检查是否为私有IP
func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return false
	}

	// 检查私有IP范围
	privateRanges := []struct {
		start net.IP
		end   net.IP
	}{
		{net.ParseIP("10.0.0.0"), net.ParseIP("10.255.255.255")},
		{net.ParseIP("172.16.0.0"), net.ParseIP("172.31.255.255")},
		{net.ParseIP("192.168.0.0"), net.ParseIP("192.168.255.255")},
	}

	for _, r := range privateRanges {
		if inRange(ip, r.start, r.end) {
			return true
		}
	}

	return false
}

// inRange 检查IP是否在指定范围内
func inRange(ip, start, end net.IP) bool {
	return bytes2Int(ip) >= bytes2Int(start) && bytes2Int(ip) <= bytes2Int(end)
}

// bytes2Int 将IP字节转换为整数
func bytes2Int(ip net.IP) uint32 {
	ip = ip.To4()
	return uint32(ip[0])<<24 + uint32(ip[1])<<16 + uint32(ip[2])<<8 + uint32(ip[3])
}

// GetNATTypeString 获取NAT类型的字符串表示
func (n NATType) String() string {
	switch n {
	case NATType1:
		return "NAT1 (完全锥形NAT)"
	case NATType2:
		return "NAT2 (受限锥形NAT)"
	case NATType3:
		return "NAT3 (端口受限锥形NAT)"
	case NATType4:
		return "NAT4 (对称NAT)"
	default:
		return "未知NAT类型"
	}
}

// TestAllSTUNServers 测试所有STUN服务器
func (n *NATSniffer) TestAllSTUNServers() map[string]error {
	results := make(map[string]error)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, server := range n.stunServers {
		wg.Add(1)
		go func(srv string) {
			defer wg.Done()

			ip, port, err := n.querySTUNServer(srv)
			mu.Lock()
			if err != nil {
				results[srv] = err
			} else {
				results[srv] = fmt.Errorf("成功: %s:%d", ip.String(), port)
			}
			mu.Unlock()
		}(server)
	}

	wg.Wait()
	return results
}

// GetDetailedNATInfo 获取详细的NAT信息
func (n *NATSniffer) GetDetailedNATInfo() (*DetailedNATInfo, error) {
	natInfo, err := n.DetectNATType()
	if err != nil {
		return nil, err
	}

	// 测试所有STUN服务器
	stunResults := n.TestAllSTUNServers()

	// 分析STUN结果
	analysis := n.analyzeSTUNResults(stunResults)

	return &DetailedNATInfo{
		BasicInfo:       natInfo,
		STUNResults:     stunResults,
		Analysis:        analysis,
		Recommendations: n.getRecommendations(natInfo.Type),
	}, nil
}

// DetailedNATInfo 详细的NAT信息
type DetailedNATInfo struct {
	BasicInfo       *NATInfo
	STUNResults     map[string]error
	Analysis        *STUNAnalysis
	Recommendations []string
}

// STUNAnalysis STUN分析结果
type STUNAnalysis struct {
	WorkingServers      int
	FailedServers       int
	UniqueIPs           int
	UniquePorts         int
	AverageResponseTime time.Duration
	Reliability         float64 // 0.0 - 1.0
}

// analyzeSTUNResults 分析STUN测试结果
func (n *NATSniffer) analyzeSTUNResults(results map[string]error) *STUNAnalysis {
	workingServers := 0
	failedServers := 0
	uniqueIPs := make(map[string]bool)
	uniquePorts := make(map[int]bool)

	for _, result := range results {
		if result != nil && len(result.Error()) > 2 && result.Error()[:2] == "成功" {
			workingServers++
			// 这里可以解析IP和端口，但为了简化，我们只统计数量
		} else {
			failedServers++
		}
	}

	totalServers := len(results)
	reliability := 0.0
	if totalServers > 0 {
		reliability = float64(workingServers) / float64(totalServers)
	}

	return &STUNAnalysis{
		WorkingServers: workingServers,
		FailedServers:  failedServers,
		UniqueIPs:      len(uniqueIPs),
		UniquePorts:    len(uniquePorts),
		Reliability:    reliability,
	}
}

// getRecommendations 根据NAT类型获取建议
func (n *NATSniffer) getRecommendations(natType NATType) []string {
	var recommendations []string

	switch natType {
	case NATType1:
		recommendations = []string{
			"✓ 您的网络环境非常适合P2P连接",
			"✓ 可以直接使用UPnP进行端口映射",
			"✓ 支持所有类型的NAT穿透技术",
			"✓ 建议启用UPnP自动端口映射",
		}
	case NATType2:
		recommendations = []string{
			"✓ 您的网络环境适合P2P连接",
			"✓ 需要先与目标主机通信才能建立连接",
			"✓ 建议使用ICE协议进行连接",
			"✓ 可以尝试UPnP端口映射",
		}
	case NATType3:
		recommendations = []string{
			"⚠ 您的网络环境需要特殊处理",
			"⚠ 建议使用TURN服务器进行中继",
			"⚠ 可以尝试UPnP但可能不成功",
			"⚠ 考虑使用STUN+TURN组合方案",
		}
	case NATType4:
		recommendations = []string{
			"✗ 您的网络环境最难穿透",
			"✗ 强烈建议使用TURN服务器",
			"✗ UPnP通常无效",
			"✗ 考虑使用VPN或代理服务",
		}
	default:
		recommendations = []string{
			"? 无法确定NAT类型",
			"? 建议进行手动网络测试",
			"? 可以尝试多种穿透技术",
		}
	}

	return recommendations
}

// IsNATFriendly 检查NAT是否适合P2P连接
func (n *NATSniffer) IsNATFriendly() (bool, string) {
	natInfo, err := n.DetectNATType()
	if err != nil {
		return false, "无法检测NAT类型"
	}

	switch natInfo.Type {
	case NATType1, NATType2:
		return true, "NAT类型适合P2P连接"
	case NATType3:
		return false, "NAT类型需要特殊处理"
	case NATType4:
		return false, "NAT类型不适合P2P连接"
	default:
		return false, "未知NAT类型"
	}
}
