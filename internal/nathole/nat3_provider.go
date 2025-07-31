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

// NAT3Provider NAT3提供者（端口限制NAT）
type NAT3Provider struct {
	logger    *logrus.Logger
	ctx       context.Context
	cancel    context.CancelFunc
	holes     map[string]*NATHole
	mutex     sync.RWMutex
	available bool
	config    map[string]interface{}

	// 记录已连接的外部主机和端口
	connectedHosts map[string]map[int]bool
	hostMutex      sync.RWMutex
}

func NewNAT3Provider(logger *logrus.Logger, config map[string]interface{}) *NAT3Provider {
	ctx, cancel := context.WithCancel(context.Background())

	return &NAT3Provider{
		logger:         logger,
		ctx:            ctx,
		cancel:         cancel,
		holes:          make(map[string]*NATHole),
		available:      false,
		config:         config,
		connectedHosts: make(map[string]map[int]bool),
	}
}

func (n *NAT3Provider) Type() types.NATType {
	return types.NATType3
}

func (n *NAT3Provider) Name() string {
	return "NAT3提供者（端口受限锥形NAT）"
}

func (n *NAT3Provider) IsAvailable() bool {
	return n.available
}

func (n *NAT3Provider) Start() error {
	n.logger.Info("启动NAT3提供者")

	// 对于端口受限锥形NAT，我们需要记录已连接的外部主机和端口
	n.available = true

	n.logger.Info("NAT3提供者启动成功")
	return nil
}

func (n *NAT3Provider) Stop() error {
	n.logger.Info("停止NAT3提供者")
	n.cancel()
	n.available = false

	// 关闭所有监听器
	n.mutex.Lock()
	defer n.mutex.Unlock()

	for _, hole := range n.holes {
		if hole.Status == HoleStatusActive {
			hole.Status = HoleStatusInactive
		}
	}

	n.logger.Info("NAT3提供者已停止")
	return nil
}

func (n *NAT3Provider) CreateHole(localPort int, externalPort int, protocol string, description string) (*NATHole, error) {
	if !n.available {
		return nil, fmt.Errorf("NAT3提供者不可用")
	}

	key := fmt.Sprintf("%d-%d-%s", localPort, externalPort, protocol)

	n.mutex.Lock()
	defer n.mutex.Unlock()

	// 检查是否已存在
	if existing, exists := n.holes[key]; exists {
		if existing.Status == HoleStatusActive {
			return existing, nil
		}
	}

	// 创建新的穿透
	hole := &NATHole{
		LocalPort:    localPort,
		ExternalPort: externalPort,
		Protocol:     protocol,
		Description:  description,
		Type:         types.NATType3,
		Status:       HoleStatusActive,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
	}

	// 对于端口受限锥形NAT，我们需要监听本地端口
	// 并且只允许之前连接过的外部主机的特定端口连接
	listener, err := net.Listen(protocol, fmt.Sprintf(":%d", localPort))
	if err != nil {
		hole.Status = HoleStatusFailed
		hole.Error = err.Error()
		n.holes[key] = hole
		return hole, fmt.Errorf("无法监听端口 %d: %w", localPort, err)
	}

	// 启动监听协程
	go n.handleConnections(listener, hole)

	// 尝试与外部主机建立连接以建立映射
	go n.establishExternalConnection(hole)

	n.holes[key] = hole

	n.logger.WithFields(logrus.Fields{
		"local_port": localPort,
		"protocol":   protocol,
		"type":       "NAT3",
	}).Info("创建NAT3穿透成功")

	return hole, nil
}

func (n *NAT3Provider) RemoveHole(localPort int, externalPort int, protocol string) error {
	key := fmt.Sprintf("%d-%d-%s", localPort, externalPort, protocol)

	n.mutex.Lock()
	defer n.mutex.Unlock()

	if hole, exists := n.holes[key]; exists {
		hole.Status = HoleStatusInactive
		hole.LastActivity = time.Now()

		n.logger.WithFields(logrus.Fields{
			"local_port": localPort,
			"protocol":   protocol,
			"type":       "NAT3",
		}).Info("移除NAT3穿透成功")

		return nil
	}

	return fmt.Errorf("未找到指定的NAT穿透")
}

func (n *NAT3Provider) GetHoles() map[string]*NATHole {
	n.mutex.RLock()
	defer n.mutex.RUnlock()

	result := make(map[string]*NATHole)
	for key, hole := range n.holes {
		result[key] = hole
	}

	return result
}

func (n *NAT3Provider) GetStatus() map[string]interface{} {
	n.mutex.RLock()
	defer n.mutex.RUnlock()

	activeCount := 0
	inactiveCount := 0
	failedCount := 0

	for _, hole := range n.holes {
		switch hole.Status {
		case HoleStatusActive:
			activeCount++
		case HoleStatusInactive:
			inactiveCount++
		case HoleStatusFailed:
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
	}
}

// handleConnections 处理连接
func (n *NAT3Provider) handleConnections(listener net.Listener, hole *NATHole) {
	defer listener.Close()

	for {
		select {
		case <-n.ctx.Done():
			return
		default:
			conn, err := listener.Accept()
			if err != nil {
				n.logger.WithError(err).Error("接受连接失败")
				continue
			}

			// 检查是否允许此连接
			if !n.isConnectionAllowed(conn) {
				n.logger.WithFields(logrus.Fields{
					"local_port":  hole.LocalPort,
					"remote_addr": conn.RemoteAddr(),
					"protocol":    hole.Protocol,
				}).Warn("拒绝未授权的NAT3连接")
				conn.Close()
				continue
			}

			// 更新最后活动时间
			hole.LastActivity = time.Now()

			// 记录连接的主机和端口
			n.recordConnection(conn)

			n.logger.WithFields(logrus.Fields{
				"local_port":  hole.LocalPort,
				"remote_addr": conn.RemoteAddr(),
				"protocol":    hole.Protocol,
			}).Info("NAT3穿透接收到连接")

			// 处理连接
			go n.handleConnection(conn, hole)
		}
	}
}

// isConnectionAllowed 检查连接是否被允许
func (n *NAT3Provider) isConnectionAllowed(conn net.Conn) bool {
	remoteAddr := conn.RemoteAddr()
	if tcpAddr, ok := remoteAddr.(*net.TCPAddr); ok {
		host := tcpAddr.IP.String()
		port := tcpAddr.Port

		n.hostMutex.RLock()
		defer n.hostMutex.RUnlock()

		if ports, exists := n.connectedHosts[host]; exists {
			return ports[port]
		}
	}
	return false
}

// recordConnection 记录连接的主机和端口
func (n *NAT3Provider) recordConnection(conn net.Conn) {
	remoteAddr := conn.RemoteAddr()
	if tcpAddr, ok := remoteAddr.(*net.TCPAddr); ok {
		host := tcpAddr.IP.String()
		port := tcpAddr.Port

		n.hostMutex.Lock()
		defer n.hostMutex.Unlock()

		if ports, exists := n.connectedHosts[host]; exists {
			ports[port] = true
		} else {
			n.connectedHosts[host] = map[int]bool{port: true}
		}
	}
}

// establishExternalConnection 建立外部连接
func (n *NAT3Provider) establishExternalConnection(hole *NATHole) {
	// 对于端口受限锥形NAT，我们需要与外部主机建立连接
	// 这样外部主机的特定端口才能连接到我们

	n.logger.WithFields(logrus.Fields{
		"local_port": hole.LocalPort,
		"protocol":   hole.Protocol,
	}).Debug("尝试建立外部连接以建立NAT3映射")
}

// handleConnection 处理单个连接
func (n *NAT3Provider) handleConnection(conn net.Conn, hole *NATHole) {
	defer conn.Close()

	// 这里可以实现具体的连接处理逻辑
	// 例如转发到本地服务或处理数据

	n.logger.WithFields(logrus.Fields{
		"local_port":  hole.LocalPort,
		"remote_addr": conn.RemoteAddr(),
	}).Debug("处理NAT3穿透连接")
}
