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

// NAT1Provider NAT1提供者（完全锥形NAT）
type NAT1Provider struct {
	logger    *logrus.Logger
	ctx       context.Context
	cancel    context.CancelFunc
	holes     map[string]*NATHole
	mutex     sync.RWMutex
	available bool
	natInfo   *types.NATInfo
	config    map[string]interface{}
}

// NewNAT1Provider 创建新的NAT1提供者
func NewNAT1Provider(logger *logrus.Logger, config map[string]interface{}) *NAT1Provider {
	ctx, cancel := context.WithCancel(context.Background())

	p := &NAT1Provider{
		logger:    logger,
		ctx:       ctx,
		cancel:    cancel,
		holes:     make(map[string]*NATHole),
		available: false,
		config:    config,
	}
	if natInfo, ok := config["nat_info"].(*types.NATInfo); ok {
		p.natInfo = natInfo
	}
	return p
}

// Type 返回NAT类型
func (n *NAT1Provider) Type() types.NATType {
	return types.NATType1
}

// Name 返回提供者名称
func (n *NAT1Provider) Name() string {
	return "NAT1Provider"
}

// IsAvailable 检查是否可用
func (n *NAT1Provider) IsAvailable() bool {
	return n.available
}

// Start 启动NAT1提供者
func (n *NAT1Provider) Start() error {
	n.logger.Info("启动NAT1提供者")

	// 对于完全锥形NAT，我们可以直接监听端口
	// 因为外部主机可以连接到任何端口
	n.available = true

	n.logger.Info("NAT1提供者启动成功")
	return nil
}

// Stop 停止NAT1提供者
func (n *NAT1Provider) Stop() error {
	n.logger.Info("停止NAT1提供者")
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

	n.logger.Info("NAT1提供者已停止")
	return nil
}

// CreateHole 创建NAT穿透
func (n *NAT1Provider) CreateHole(localPort int, externalPort int, protocol string, description string) (*NATHole, error) {
	return CreateHoleCommon(
		n.logger,
		n.holes,
		&n.mutex,
		n.available,
		"NAT1",
		types.NATType1,
		localPort,
		externalPort,
		protocol,
		description,
		n.handleTCPConnections,
		n.handleUDPConnections,
		nil, // NAT1不需要建立外部连接
	)
}

// RemoveHole 移除NAT穿透
func (n *NAT1Provider) RemoveHole(localPort int, externalPort int, protocol string) error {
	key := fmt.Sprintf("%d-%d-%s", localPort, externalPort, protocol)

	n.mutex.Lock()
	defer n.mutex.Unlock()

	if hole, exists := n.holes[key]; exists {
		hole.Status = types.MappingStatusInactive
		hole.LastActivity = time.Now()

		n.logger.WithFields(logrus.Fields{
			"local_port":    localPort,
			"external_port": externalPort,
			"protocol":      protocol,
			"type":          "NAT1",
		}).Info("移除NAT1穿透成功")

		return nil
	}

	return fmt.Errorf("未找到指定的NAT穿透")
}

// GetHoles 获取所有穿透
func (n *NAT1Provider) GetHoles() map[string]*NATHole {
	n.mutex.RLock()
	defer n.mutex.RUnlock()

	result := make(map[string]*NATHole)
	for key, hole := range n.holes {
		result[key] = hole
	}

	return result
}

// GetStatus 获取提供者状态
func (n *NAT1Provider) GetStatus() map[string]interface{} {
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

	return map[string]interface{}{
		"available":      n.available,
		"total_holes":    len(n.holes),
		"active_holes":   activeCount,
		"inactive_holes": inactiveCount,
		"failed_holes":   failedCount,
		"nat_info":       n.natInfo,
	}
}

// handleTCPConnections 处理TCP连接
func (n *NAT1Provider) handleTCPConnections(listener net.Listener, hole *NATHole) {
	// 使用通用TCP代理
	handler := NewBaseProxyHandler(n.logger, n.ctx)
	proxy := NewTCPProxy(handler)
	proxy.HandleConnections(listener, hole)
}

// handleUDPConnections 处理UDP连接
func (n *NAT1Provider) handleUDPConnections(packetConn net.PacketConn, hole *NATHole) {
	// 使用通用UDP代理
	handler := NewBaseProxyHandler(n.logger, n.ctx)
	proxy := NewUDPProxy(handler)
	proxy.HandleConnections(packetConn, hole)
}
