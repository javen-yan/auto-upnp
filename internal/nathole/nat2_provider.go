package nathole

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"auto-upnp/internal/types"

	"github.com/sirupsen/logrus"
)

// NAT2Provider NAT2提供者（受限锥形NAT）
type NAT2Provider struct {
	logger    *logrus.Logger
	ctx       context.Context
	cancel    context.CancelFunc
	holes     map[string]*NATHole
	mutex     sync.RWMutex
	available bool
	config    map[string]interface{}

	connectedHosts map[string]bool // 共享的已连接主机记录
	hostMutex      sync.RWMutex    // 共享的已连接主机记录的互斥锁
	natInfo        *types.NATInfo
	stunServers    []string
	nat2Mode       int
}

var defaultStunServers = []string{
	"stun.miwifi.com:3478",
	"stun.chat.bilibili.com:3478",
	"stun.hitv.com:3478",
	"stun.cdnbye.com:3478",
}

// NewNAT2Provider 创建新的NAT2提供者
func NewNAT2Provider(logger *logrus.Logger, config map[string]interface{}) *NAT2Provider {
	ctx, cancel := context.WithCancel(context.Background())

	p := &NAT2Provider{
		logger:         logger,
		ctx:            ctx,
		cancel:         cancel,
		holes:          make(map[string]*NATHole),
		available:      false,
		config:         config,
		connectedHosts: make(map[string]bool),
	}

	if natInfo, ok := config["nat_info"].(*types.NATInfo); ok {
		p.natInfo = natInfo
	}

	if stunServers, ok := config["stun_servers"].([]string); ok {
		p.stunServers = stunServers
	} else {
		p.stunServers = defaultStunServers
	}

	if nat2Mode, ok := config["nat2_mode"].(int); ok {
		p.nat2Mode = nat2Mode
	} else {
		p.nat2Mode = 1
	}

	return p
}

// Type 返回NAT类型
func (n *NAT2Provider) Type() types.NATType {
	return types.NATType2
}

// Name 返回提供者名称
func (n *NAT2Provider) Name() string {
	return "NAT2Provider"
}

// IsAvailable 检查是否可用
func (n *NAT2Provider) IsAvailable() bool {
	return n.available
}

// Start 启动NAT2提供者
func (n *NAT2Provider) Start() error {
	n.logger.Info("启动NAT2提供者")

	// 对于受限锥形NAT，我们需要先与外部主机建立连接
	// 然后外部主机才能连接到我们
	n.available = true

	// 启动连接收集协程
	go n.collectAvailableConnections()

	n.logger.Info("NAT2提供者启动成功")
	return nil
}

// Stop 停止NAT2提供者
func (n *NAT2Provider) Stop() error {
	n.logger.Info("停止NAT2提供者")
	n.cancel()
	n.available = false

	// 关闭所有监听器
	n.mutex.Lock()
	defer n.mutex.Unlock()

	for _, hole := range n.holes {
		if hole.Status == types.MappingStatusActive {
			hole.Status = types.MappingStatusInactive
		}
	}

	n.logger.Info("NAT2提供者已停止")
	return nil
}

// CreateHole 创建NAT穿透
func (n *NAT2Provider) CreateHole(localPort int, externalPort int, protocol string, description string) (*NATHole, error) {
	return CreateHoleCommon(
		n.logger,
		n.holes,
		&n.mutex,
		n.available,
		"NAT2",
		types.NATType2,
		localPort,
		externalPort,
		protocol,
		description,
		n.handleTCPConnections,
		n.handleUDPConnections,
		n.establishExternalConnection, // NAT2需要建立外部连接
	)
}

// RemoveHole 移除NAT穿透
func (n *NAT2Provider) RemoveHole(localPort int, externalPort int, protocol string) error {
	key := fmt.Sprintf("%d-%d-%s", localPort, externalPort, protocol)

	n.mutex.Lock()
	defer n.mutex.Unlock()

	if hole, exists := n.holes[key]; exists {
		hole.Status = types.MappingStatusInactive
		hole.LastActivity = time.Now()

		n.logger.WithFields(logrus.Fields{
			"local_port": localPort,
			"protocol":   protocol,
			"type":       "NAT2",
		}).Info("移除NAT2穿透成功")

		return nil
	}

	return fmt.Errorf("未找到指定的NAT穿透")
}

// GetHoles 获取所有穿透
func (n *NAT2Provider) GetHoles() map[string]*NATHole {
	n.mutex.RLock()
	defer n.mutex.RUnlock()

	result := make(map[string]*NATHole)
	for key, hole := range n.holes {
		result[key] = hole
	}

	return result
}

// GetStatus 获取提供者状态
func (n *NAT2Provider) GetStatus() map[string]interface{} {
	n.mutex.RLock()
	defer n.mutex.RUnlock()

	activeCount := 0
	inactiveCount := 0
	failedCount := 0

	for _, hole := range n.holes {
		switch hole.Status {
		case types.MappingStatusActive:
			activeCount++
		case types.MappingStatusInactive:
			inactiveCount++
		case types.MappingStatusFailed:
			failedCount++
		}
	}

	n.hostMutex.RLock()
	connectedHostsCount := len(n.connectedHosts)
	n.hostMutex.RUnlock()

	return map[string]interface{}{
		"available":       n.available,
		"total_holes":     len(n.holes),
		"active_holes":    activeCount,
		"inactive_holes":  inactiveCount,
		"failed_holes":    failedCount,
		"connected_hosts": connectedHostsCount,
		"nat_info":        n.natInfo,
	}
}

// handleTCPConnections 处理TCP连接
func (n *NAT2Provider) handleTCPConnections(listener net.Listener, hole *NATHole) {
	// 使用NAT2专用的TCP代理，共享已连接主机记录
	handler := NewNAT2ProxyHandlerWithSharedHosts(n.logger, n.ctx, n.connectedHosts, &n.hostMutex, n.nat2Mode)
	proxy := NewTCPProxy(handler)
	proxy.HandleConnections(listener, hole)
}

// establishExternalConnection 建立外部连接
func (n *NAT2Provider) establishExternalConnection(hole *NATHole) {

	// 启动自动协商协程
	go n.autoNegotiation(hole)

	n.logger.WithFields(logrus.Fields{
		"local_port":    hole.LocalPort,
		"external_port": hole.ExternalPort,
		"protocol":      hole.Protocol,
	}).Info("启动NAT2自动协商")
}

// autoNegotiation 自动协商过程
func (n *NAT2Provider) autoNegotiation(hole *NATHole) {
	// 尝试连接到多个外部服务器以建立映射
	servers := n.stunServers

	for _, server := range servers {
		select {
		case <-n.ctx.Done():
			return
		default:
			if n.tryConnectToServer(hole, server) {
				n.logger.WithFields(logrus.Fields{
					"server":        server,
					"local_port":    hole.LocalPort,
					"external_port": hole.ExternalPort,
					"protocol":      hole.Protocol,
				}).Info("成功建立NAT2映射")
				return
			}
		}
	}

	n.logger.WithFields(logrus.Fields{
		"local_port":    hole.LocalPort,
		"external_port": hole.ExternalPort,
		"protocol":      hole.Protocol,
	}).Warn("无法建立NAT2映射，所有服务器连接失败")
}

// tryConnectToServer 尝试连接到指定服务器
func (n *NAT2Provider) tryConnectToServer(hole *NATHole, server string) bool {
	// 根据协议类型选择连接方式
	var conn net.Conn
	var err error

	switch hole.Protocol {
	case "tcp":
		conn, err = net.DialTimeout("tcp", server, 5*time.Second)
	case "udp":
		conn, err = net.DialTimeout("udp", server, 5*time.Second)
	default:
		return false
	}

	if err != nil {
		n.logger.WithFields(logrus.Fields{
			"server":   server,
			"protocol": hole.Protocol,
			"error":    err.Error(),
		}).Debug("连接服务器失败")
		return false
	}
	defer conn.Close()

	// 获取远程地址（STUN服务器的IP）
	remoteAddr := conn.RemoteAddr()
	if tcpAddr, ok := remoteAddr.(*net.TCPAddr); ok {
		// 记录我们连接到的外部服务器IP（允许该IP访问我们）
		n.hostMutex.Lock()
		n.connectedHosts[tcpAddr.IP.String()] = true
		n.hostMutex.Unlock()

		n.logger.WithFields(logrus.Fields{
			"server":        server,
			"remote_addr":   remoteAddr,
			"local_addr":    conn.LocalAddr(),
			"local_port":    hole.LocalPort,
			"external_port": hole.ExternalPort,
			"protocol":      hole.Protocol,
		}).Info("成功建立NAT2连接")

		return true
	}

	return false
}

// collectAvailableConnections 收集可用的连接
func (n *NAT2Provider) collectAvailableConnections() {
	// 启动时立即执行一次
	n.scanForConnections()

	// 然后每5分钟收集一次，避免过于频繁的外部连接
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-n.ctx.Done():
			return
		case <-ticker.C:
			n.scanForConnections()
		}
	}
}

// scanForConnections 扫描可用连接
func (n *NAT2Provider) scanForConnections() {
	// 对于NAT2（受限锥形NAT），我们需要主动与外部服务器建立连接
	// 来创建NAT映射，这样外部主机才能连接到我们

	// 使用更多的STUN服务器来建立连接
	servers := n.stunServers

	// 同时尝试TCP和UDP连接
	protocols := []string{"tcp", "udp"}

	for _, server := range servers {
		select {
		case <-n.ctx.Done():
			return
		default:
			for _, protocol := range protocols {
				if n.tryConnectToExternalServer(server, protocol) {
					n.logger.WithFields(logrus.Fields{
						"server":   server,
						"protocol": protocol,
					}).Debug("成功建立外部连接")
				}
			}
		}
	}

	// 尝试连接到一些常用的外部服务来建立映射
	externalServices := []string{
		"8.8.8.8:53",        // Google DNS
		"1.1.1.1:53",        // Cloudflare DNS
		"208.67.222.222:53", // OpenDNS
	}

	for _, service := range externalServices {
		select {
		case <-n.ctx.Done():
			return
		default:
			if n.tryConnectToExternalServer(service, "udp") {
				n.logger.WithField("service", service).Debug("成功建立外部服务连接")
			}
		}
	}
}

// tryConnectToExternalServer 尝试连接到外部服务器
func (n *NAT2Provider) tryConnectToExternalServer(server, protocol string) bool {
	var conn net.Conn
	var err error

	switch protocol {
	case "tcp":
		conn, err = net.DialTimeout("tcp", server, 3*time.Second)
	case "udp":
		conn, err = net.DialTimeout("udp", server, 3*time.Second)
	default:
		return false
	}

	if err != nil {
		return false
	}
	defer conn.Close()

	// 获取远程地址（外部服务器的IP）
	remoteAddr := conn.RemoteAddr()
	if tcpAddr, ok := remoteAddr.(*net.TCPAddr); ok {
		// 记录我们连接到的外部服务器IP（允许该IP访问我们）
		n.hostMutex.Lock()
		n.connectedHosts[tcpAddr.IP.String()] = true
		n.hostMutex.Unlock()

		return true
	} else if udpAddr, ok := remoteAddr.(*net.UDPAddr); ok {
		// 记录我们连接到的外部服务器IP（允许该IP访问我们）
		n.hostMutex.Lock()
		n.connectedHosts[udpAddr.IP.String()] = true
		n.hostMutex.Unlock()

		return true
	}

	return false
}

// handleUDPConnections 处理UDP连接
func (n *NAT2Provider) handleUDPConnections(packetConn net.PacketConn, hole *NATHole) {
	// 使用NAT2专用的UDP代理，共享已连接主机记录
	handler := NewNAT2ProxyHandlerWithSharedHosts(n.logger, n.ctx, n.connectedHosts, &n.hostMutex, n.nat2Mode)
	proxy := NewUDPProxy(handler)
	proxy.HandleConnections(packetConn, hole)
}
