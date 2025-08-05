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

	// 记录已连接的外部主机
	connectedHosts map[string]bool
	hostMutex      sync.RWMutex
	natInfo        *types.NATInfo
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
	// 使用NAT2专用的TCP代理
	handler := NewNAT2ProxyHandler(n.logger, n.ctx)
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
	servers := []string{
		"stun.miwifi.com:3478",
		"stun.chat.bilibili.com:3478",
		"stun.hitv.com:3478",
		"stun.cdnbye.com:3478",
	}

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

	// 获取本地地址
	localAddr := conn.LocalAddr()
	if tcpAddr, ok := localAddr.(*net.TCPAddr); ok {
		// 记录连接的主机（允许该主机访问）
		n.hostMutex.Lock()
		n.connectedHosts[tcpAddr.IP.String()] = true
		n.hostMutex.Unlock()

		n.logger.WithFields(logrus.Fields{
			"server":        server,
			"local_addr":    localAddr,
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
	ticker := time.NewTicker(30 * time.Second) // 每30秒收集一次
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
	// 扫描常见的端口和服务
	ports := []int{80, 443, 8080, 8443, 22, 21, 25, 53}

	for _, port := range ports {
		select {
		case <-n.ctx.Done():
			return
		default:
			// 尝试连接到本地端口以建立映射
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 1*time.Second)
			if err == nil {
				conn.Close()

				// 记录本地连接
				n.hostMutex.Lock()
				n.connectedHosts["127.0.0.1"] = true
				n.hostMutex.Unlock()

				n.logger.WithField("port", port).Debug("发现本地可用连接")
			}
		}
	}
}

// handleUDPConnections 处理UDP连接
func (n *NAT2Provider) handleUDPConnections(packetConn net.PacketConn, hole *NATHole) {
	// 使用NAT2专用的UDP代理
	handler := NewNAT2ProxyHandler(n.logger, n.ctx)
	proxy := NewUDPProxy(handler)
	proxy.HandleConnections(packetConn, hole)
}
