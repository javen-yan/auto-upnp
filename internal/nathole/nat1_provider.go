package nathole

import (
	"context"
	"fmt"
	"io"
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
	config    map[string]interface{}
}

// NewNAT1Provider 创建新的NAT1提供者
func NewNAT1Provider(logger *logrus.Logger, config map[string]interface{}) *NAT1Provider {
	ctx, cancel := context.WithCancel(context.Background())

	return &NAT1Provider{
		logger:    logger,
		ctx:       ctx,
		cancel:    cancel,
		holes:     make(map[string]*NATHole),
		available: false,
		config:    config,
	}
}

// Type 返回NAT类型
func (n *NAT1Provider) Type() types.NATType {
	return types.NATType1
}

// Name 返回提供者名称
func (n *NAT1Provider) Name() string {
	return "NAT1提供者（完全锥形NAT）"
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
		if hole.Status == HoleStatusActive {
			hole.Status = HoleStatusInactive
		}
	}

	n.logger.Info("NAT1提供者已停止")
	return nil
}

// CreateHole 创建NAT穿透
func (n *NAT1Provider) CreateHole(localPort int, externalPort int, protocol string, description string) (*NATHole, error) {
	if !n.available {
		return nil, fmt.Errorf("NAT1提供者不可用")
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
		Type:         types.NATType1,
		Status:       HoleStatusActive,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
	}

	// 根据协议类型选择不同的监听方式
	switch protocol {
	case "tcp":
		// TCP协议使用Listener
		listener, err := net.Listen(protocol, fmt.Sprintf(":%d", externalPort))
		if err != nil {
			// 检查是否是端口冲突
			if localPort == externalPort {
				hole.Status = HoleStatusFailed
				hole.Error = fmt.Sprintf("内外端口一致(%d)且端口已被占用，无法创建NAT穿透", externalPort)
				n.holes[key] = hole
				return hole, fmt.Errorf("内外端口一致(%d)且端口已被占用，无法创建NAT穿透", externalPort)
			}
			hole.Status = HoleStatusFailed
			hole.Error = fmt.Sprintf("无法监听外部端口 %d: %v", externalPort, err)
			n.holes[key] = hole
			return hole, fmt.Errorf("无法监听外部端口 %d: %w", externalPort, err)
		}

		// 启动TCP监听协程
		go n.handleTCPConnections(listener, hole)
	case "udp":
		// UDP协议使用PacketConn
		packetConn, err := net.ListenPacket(protocol, fmt.Sprintf(":%d", externalPort))
		if err != nil {
			// 检查是否是端口冲突
			if localPort == externalPort {
				hole.Status = HoleStatusFailed
				hole.Error = fmt.Sprintf("内外端口一致(%d)且端口已被占用，无法创建NAT穿透", externalPort)
				n.holes[key] = hole
				return hole, fmt.Errorf("内外端口一致(%d)且端口已被占用，无法创建NAT穿透", externalPort)
			}
			hole.Status = HoleStatusFailed
			hole.Error = fmt.Sprintf("无法监听外部UDP端口 %d: %v", externalPort, err)
			n.holes[key] = hole
			return hole, fmt.Errorf("无法监听外部UDP端口 %d: %w", externalPort, err)
		}

		// 启动UDP监听协程
		go n.handleUDPConnections(packetConn, hole)
	default:
		hole.Status = HoleStatusFailed
		hole.Error = fmt.Sprintf("不支持的协议: %s", protocol)
		n.holes[key] = hole
		return hole, fmt.Errorf("不支持的协议: %s", protocol)
	}

	n.holes[key] = hole

	n.logger.WithFields(logrus.Fields{
		"local_port":    localPort,
		"external_port": externalPort,
		"protocol":      protocol,
		"type":          "NAT1",
	}).Info("创建NAT1穿透成功")

	return hole, nil
}

// RemoveHole 移除NAT穿透
func (n *NAT1Provider) RemoveHole(localPort int, externalPort int, protocol string) error {
	key := fmt.Sprintf("%d-%d-%s", localPort, externalPort, protocol)

	n.mutex.Lock()
	defer n.mutex.Unlock()

	if hole, exists := n.holes[key]; exists {
		hole.Status = HoleStatusInactive
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
		case HoleStatusActive:
			activeCount++
		case HoleStatusInactive:
			inactiveCount++
		case HoleStatusFailed:
			failedCount++
		}
	}

	return map[string]interface{}{
		"available":      n.available,
		"total_holes":    len(n.holes),
		"active_holes":   activeCount,
		"inactive_holes": inactiveCount,
		"failed_holes":   failedCount,
	}
}

// handleTCPConnections 处理TCP连接
func (n *NAT1Provider) handleTCPConnections(listener net.Listener, hole *NATHole) {
	defer listener.Close()

	n.logger.WithFields(logrus.Fields{
		"external_port": hole.ExternalPort,
		"local_port":    hole.LocalPort,
		"protocol":      hole.Protocol,
	}).Info("开始监听外部TCP端口")

	for {
		select {
		case <-n.ctx.Done():
			n.logger.Info("停止监听外部TCP端口")
			return
		default:
			conn, err := listener.Accept()
			if err != nil {
				n.logger.WithError(err).Error("接受TCP连接失败")
				continue
			}

			// 更新最后活动时间
			hole.LastActivity = time.Now()

			n.logger.WithFields(logrus.Fields{
				"external_port": hole.ExternalPort,
				"local_port":    hole.LocalPort,
				"remote_addr":   conn.RemoteAddr(),
				"protocol":      hole.Protocol,
			}).Info("NAT1穿透接收到外部TCP连接")

			// 处理TCP连接（转发到本地端口）
			go n.handleTCPConnection(conn, hole)
		}
	}
}

// handleTCPConnection 处理单个TCP连接
func (n *NAT1Provider) handleTCPConnection(externalConn net.Conn, hole *NATHole) {
	defer externalConn.Close()

	// 连接到本地端口
	localConn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", hole.LocalPort))
	if err != nil {
		n.logger.WithFields(logrus.Fields{
			"local_port":  hole.LocalPort,
			"remote_addr": externalConn.RemoteAddr(),
			"error":       err.Error(),
		}).Error("无法连接到本地TCP端口")
		return
	}
	defer localConn.Close()

	n.logger.WithFields(logrus.Fields{
		"external_port": hole.ExternalPort,
		"local_port":    hole.LocalPort,
		"remote_addr":   externalConn.RemoteAddr(),
		"protocol":      hole.Protocol,
	}).Debug("开始转发TCP连接")

	// 双向转发数据
	go func() {
		written, err := io.Copy(localConn, externalConn)
		if err != nil {
			n.logger.WithError(err).Debug("转发TCP数据到本地端口时出错")
		}
		n.logger.WithField("bytes_written", written).Debug("转发TCP数据到本地端口完成")
	}()

	written, err := io.Copy(externalConn, localConn)
	if err != nil {
		n.logger.WithError(err).Debug("转发TCP数据到外部连接时出错")
	}
	n.logger.WithField("bytes_written", written).Debug("转发TCP数据到外部连接完成")
}

// handleUDPConnections 处理UDP连接
func (n *NAT1Provider) handleUDPConnections(packetConn net.PacketConn, hole *NATHole) {
	defer packetConn.Close()

	n.logger.WithFields(logrus.Fields{
		"external_port": hole.ExternalPort,
		"local_port":    hole.LocalPort,
		"protocol":      hole.Protocol,
	}).Info("开始监听外部UDP端口")

	buffer := make([]byte, 4096)

	for {
		select {
		case <-n.ctx.Done():
			n.logger.Info("停止监听外部UDP端口")
			return
		default:
			bytesRead, remoteAddr, err := packetConn.ReadFrom(buffer)
			if err != nil {
				n.logger.WithError(err).Error("读取UDP数据失败")
				continue
			}

			// 更新最后活动时间
			hole.LastActivity = time.Now()

			n.logger.WithFields(logrus.Fields{
				"external_port": hole.ExternalPort,
				"local_port":    hole.LocalPort,
				"remote_addr":   remoteAddr,
				"protocol":      hole.Protocol,
				"data_size":     bytesRead,
			}).Info("NAT1穿透接收到外部UDP数据")

			// 处理UDP数据（转发到本地端口）
			go n.handleUDPData(packetConn, remoteAddr, buffer[:bytesRead], hole)
		}
	}
}

// handleUDPData 处理UDP数据
func (n *NAT1Provider) handleUDPData(packetConn net.PacketConn, remoteAddr net.Addr, data []byte, hole *NATHole) {
	// 连接到本地UDP端口
	localAddr := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: hole.LocalPort,
	}

	localConn, err := net.DialUDP("udp", nil, localAddr)
	if err != nil {
		n.logger.WithFields(logrus.Fields{
			"local_port":  hole.LocalPort,
			"remote_addr": remoteAddr,
			"error":       err.Error(),
		}).Error("无法连接到本地UDP端口")
		return
	}
	defer localConn.Close()

	n.logger.WithFields(logrus.Fields{
		"external_port": hole.ExternalPort,
		"local_port":    hole.LocalPort,
		"remote_addr":   remoteAddr,
		"data_size":     len(data),
	}).Debug("开始转发UDP数据")

	// 发送数据到本地端口
	_, err = localConn.Write(data)
	if err != nil {
		n.logger.WithError(err).Error("发送UDP数据到本地端口失败")
		return
	}

	// 读取本地端口的响应
	responseBuffer := make([]byte, 4096)
	bytesRead, err := localConn.Read(responseBuffer)
	if err != nil {
		n.logger.WithError(err).Debug("读取本地UDP端口响应失败")
		return
	}

	// 发送响应回外部客户端
	_, err = packetConn.WriteTo(responseBuffer[:bytesRead], remoteAddr)
	if err != nil {
		n.logger.WithError(err).Error("发送UDP响应失败")
		return
	}

	n.logger.WithFields(logrus.Fields{
		"external_port": hole.ExternalPort,
		"local_port":    hole.LocalPort,
		"remote_addr":   remoteAddr,
		"response_size": bytesRead,
	}).Debug("UDP数据转发完成")
}
