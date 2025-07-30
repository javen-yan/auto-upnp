package nat_traversal

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/pion/turn/v2"
	"github.com/sirupsen/logrus"
)

// AllocatedPort 分配的端口信息
type AllocatedPort struct {
	Port        int
	RelayConn   net.PacketConn
	AllocatedAt time.Time
	LastUsed    time.Time
	IsActive    bool
	UsageCount  int64
}

// TURNClient TURN客户端
type TURNClient struct {
	logger      *logrus.Logger
	turnServers []TURNServer
	client      *turn.Client
	relayConn   net.PacketConn // 中继连接
	ctx         context.Context
	cancel      context.CancelFunc

	// 端口分配管理
	allocatedPorts map[int]*AllocatedPort
	portMutex      sync.RWMutex
}

// TURNServer TURN服务器信息
type TURNServer struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Realm    string `json:"realm"`
}

// TURNResponse TURN响应信息
type TURNResponse struct {
	RelayIP   net.IP
	RelayPort int
	RelayAddr *net.UDPAddr
	Username  string
	Password  string
}

// TURNConnection TURN连接信息
type TURNConnection struct {
	RelayAddr *net.UDPAddr
	Conn      net.Conn
	IsActive  bool
}

// NewTURNClient 创建新的TURN客户端
func NewTURNClient(logger *logrus.Logger, customServers []TURNServer) *TURNClient {
	ctx, cancel := context.WithCancel(context.Background())

	client := &TURNClient{
		logger:         logger,
		ctx:            ctx,
		cancel:         cancel,
		allocatedPorts: make(map[int]*AllocatedPort),
	}

	if len(customServers) > 0 {
		client.turnServers = customServers
		logger.WithField("servers", len(customServers)).Info("使用自定义TURN服务器列表")
	}

	return client
}

// ConnectToTURN 连接到TURN服务器并分配中继地址
func (tc *TURNClient) ConnectToTURN() (*TURNResponse, error) {
	tc.logger.Info("开始连接TURN服务器...")

	// 尝试连接每个TURN服务器
	for _, server := range tc.turnServers {
		if response, err := tc.connectToTURNServer(server); err == nil {
			tc.logger.WithFields(logrus.Fields{
				"server":     fmt.Sprintf("%s:%d", server.Host, server.Port),
				"relay_ip":   response.RelayIP.String(),
				"relay_port": response.RelayPort,
			}).Info("TURN服务器连接成功")
			return response, nil
		} else {
			tc.logger.WithFields(logrus.Fields{
				"server": fmt.Sprintf("%s:%d", server.Host, server.Port),
				"error":  err,
			}).Warn("TURN服务器连接失败")
		}
	}

	return nil, fmt.Errorf("所有TURN服务器连接失败")
}

// connectToTURNServer 连接到单个TURN服务器
func (tc *TURNClient) connectToTURNServer(server TURNServer) (*TURNResponse, error) {
	// 创建本地UDP连接
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{
		IP:   net.IPv4zero,
		Port: 0, // 让系统自动分配端口
	})
	if err != nil {
		return nil, fmt.Errorf("创建本地UDP连接失败: %w", err)
	}

	// 创建TURN客户端配置
	config := &turn.ClientConfig{
		// STUNServerAddr: fmt.Sprintf("%s:%d", server.Host, server.Port),
		TURNServerAddr: fmt.Sprintf("%s:%d", server.Host, server.Port),
		Username:       server.Username,
		Password:       server.Password,
		Realm:          server.Realm,
		Software:       "auto-upnp",
		Conn:           conn,
	}

	// 如果提供了TURN认证信息，则启用TURN模式
	if server.Username != "" && server.Password != "" {
		config.TURNServerAddr = fmt.Sprintf("%s:%d", server.Host, server.Port)
		tc.logger.Info("使用TURN模式（需要认证）")
	} else {
		tc.logger.Info("使用STUN模式（无需认证）")
	}

	// 创建TURN客户端
	tc.logger.WithFields(logrus.Fields{
		"server":   fmt.Sprintf("%s:%d", server.Host, server.Port),
		"username": server.Username,
		"realm":    server.Realm,
	}).Info("尝试连接TURN服务器")

	client, err := turn.NewClient(config)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("创建TURN客户端失败: %w", err)
	}

	// 启动客户端
	if err := client.Listen(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("启动TURN客户端失败: %w", err)
	}

	tc.client = client

	// 分配中继地址
	tc.logger.Info("开始分配TURN中继地址")
	relayConn, err := client.Allocate()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("分配中继地址失败: %w", err)
	}

	// 保存中继连接
	tc.relayConn = relayConn

	// 获取中继地址信息
	relayAddr, ok := relayConn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return nil, fmt.Errorf("无法获取中继地址")
	}

	return &TURNResponse{
		RelayIP:   relayAddr.IP,
		RelayPort: relayAddr.Port,
		RelayAddr: relayAddr,
		Username:  server.Username,
		Password:  server.Password,
	}, nil
}

// CreateRelayConnection 创建中继连接
func (tc *TURNClient) CreateRelayConnection(targetAddr *net.UDPAddr) (*TURNConnection, error) {
	if tc.client == nil {
		return nil, fmt.Errorf("TURN客户端未连接")
	}

	// 创建中继权限
	err := tc.client.CreatePermission(targetAddr)
	if err != nil {
		return nil, fmt.Errorf("创建中继权限失败: %w", err)
	}

	return &TURNConnection{
		RelayAddr: targetAddr,
		Conn:      nil, // TURN客户端不直接返回连接对象
		IsActive:  true,
	}, nil
}

// SendDataViaRelay 通过中继发送数据
func (tc *TURNClient) SendDataViaRelay(targetAddr *net.UDPAddr, data []byte) error {
	if tc.client == nil {
		return fmt.Errorf("TURN客户端未连接")
	}

	// 发送数据到目标地址
	_, err := tc.client.WriteTo(data, targetAddr)
	return err
}

// ReceiveDataFromRelay 从中继接收数据
func (tc *TURNClient) ReceiveDataFromRelay(timeout time.Duration) ([]byte, *net.UDPAddr, error) {
	if tc.relayConn == nil {
		return nil, nil, fmt.Errorf("TURN中继连接未建立")
	}

	// 设置读取超时
	if timeout > 0 {
		tc.relayConn.SetReadDeadline(time.Now().Add(timeout))
	}

	// 创建缓冲区
	buffer := make([]byte, 4096)

	// 从中继连接读取数据
	n, remoteAddr, err := tc.relayConn.ReadFrom(buffer)
	if err != nil {
		return nil, nil, err
	}

	// 转换地址类型
	udpAddr, ok := remoteAddr.(*net.UDPAddr)
	if !ok {
		return nil, nil, fmt.Errorf("无法转换地址类型")
	}

	return buffer[:n], udpAddr, nil
}

// StartTURNDataForwarding 启动TURN数据转发
func (tc *TURNClient) StartTURNDataForwarding(targetPort int, onDataReceived func([]byte, *net.UDPAddr)) error {
	if tc.relayConn == nil {
		return fmt.Errorf("TURN中继连接未建立")
	}

	// 启动监听
	go func() {
		for {
			select {
			case <-tc.ctx.Done():
				return
			default:
				// 接收数据
				data, remoteAddr, err := tc.ReceiveDataFromRelay(5 * time.Second)
				if err != nil {
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						continue // 超时继续
					}
					tc.logger.WithError(err).Warn("接收TURN数据失败")
					continue
				}

				// 转发数据到本地目标端口
				go tc.forwardTURNDataToLocal(targetPort, data, remoteAddr)

				// 调用回调函数处理数据
				if onDataReceived != nil {
					onDataReceived(data, remoteAddr)
				}
			}
		}
	}()

	tc.logger.WithFields(logrus.Fields{
		"target_port": targetPort,
	}).Info("TURN数据转发已启动")
	return nil
}

// forwardTURNDataToLocal 转发TURN数据到本地端口
func (tc *TURNClient) forwardTURNDataToLocal(targetPort int, data []byte, remoteAddr *net.UDPAddr) {
	// 连接到本地目标端口
	targetAddr := &net.UDPAddr{
		IP:   net.IPv4(127, 0, 0, 1), // localhost
		Port: targetPort,
	}

	conn, err := net.DialUDP("udp", nil, targetAddr)
	if err != nil {
		tc.logger.WithFields(logrus.Fields{
			"target_port": targetPort,
			"error":       err,
		}).Error("连接本地UDP目标端口失败")
		return
	}
	defer conn.Close()

	// 发送数据到本地目标端口
	_, err = conn.Write(data)
	if err != nil {
		tc.logger.WithFields(logrus.Fields{
			"target_port": targetPort,
			"error":       err,
		}).Error("发送数据到本地UDP目标端口失败")
		return
	}

	// 读取响应数据
	responseBuffer := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := conn.Read(responseBuffer)
	if err != nil {
		tc.logger.WithFields(logrus.Fields{
			"target_port": targetPort,
			"error":       err,
		}).Warn("读取本地UDP目标端口响应失败")
		return
	}

	// 将响应数据发送回远程客户端
	err = tc.SendDataViaRelay(remoteAddr, responseBuffer[:n])
	if err != nil {
		tc.logger.WithFields(logrus.Fields{
			"target_port": targetPort,
			"remote_addr": remoteAddr.String(),
			"error":       err,
		}).Error("发送TURN响应数据失败")
		return
	}

	tc.logger.WithFields(logrus.Fields{
		"target_port":   targetPort,
		"remote_addr":   remoteAddr.String(),
		"data_size":     len(data),
		"response_size": n,
	}).Info("TURN数据转发完成")
}

// GetRelayStatus 获取中继状态
func (tc *TURNClient) GetRelayStatus() map[string]interface{} {
	if tc.client == nil {
		return map[string]interface{}{
			"connected": false,
			"message":   "TURN客户端未连接",
		}
	}

	relayAddr := "未知"
	if tc.relayConn != nil {
		if addr := tc.relayConn.LocalAddr(); addr != nil {
			relayAddr = addr.String()
		}
	}

	return map[string]interface{}{
		"connected":  true,
		"server":     tc.getCurrentServer(),
		"relay_addr": relayAddr,
	}
}

// getCurrentServer 获取当前连接的服务器信息
func (tc *TURNClient) getCurrentServer() string {
	if tc.client == nil {
		return "未连接"
	}

	// 这里可以根据实际需要返回服务器信息
	return "TURN服务器"
}

// AllocatePort 分配TURN端口
func (tc *TURNClient) AllocatePort() (*AllocatedPort, error) {
	if tc.client == nil {
		return nil, fmt.Errorf("TURN客户端未连接")
	}

	tc.logger.Info("开始分配TURN端口...")

	// 检查是否已经有中继连接
	if tc.relayConn != nil {
		// 如果已经有中继连接，直接使用它
		relayAddr, ok := tc.relayConn.LocalAddr().(*net.UDPAddr)
		if !ok {
			return nil, fmt.Errorf("无法获取现有TURN中继地址")
		}

		// 检查端口是否已经被分配
		tc.portMutex.RLock()
		if existingPort, exists := tc.allocatedPorts[relayAddr.Port]; exists && existingPort.IsActive {
			tc.portMutex.RUnlock()
			tc.logger.WithField("port", relayAddr.Port).Info("端口已分配，返回现有端口")
			return existingPort, nil
		}
		tc.portMutex.RUnlock()

		// 创建新的分配记录
		allocatedPort := &AllocatedPort{
			Port:        relayAddr.Port,
			RelayConn:   tc.relayConn,
			AllocatedAt: time.Now(),
			LastUsed:    time.Now(),
			IsActive:    true,
			UsageCount:  0,
		}

		// 保存分配的端口
		tc.portMutex.Lock()
		tc.allocatedPorts[relayAddr.Port] = allocatedPort
		tc.portMutex.Unlock()

		tc.logger.WithFields(logrus.Fields{
			"port":         relayAddr.Port,
			"relay_ip":     relayAddr.IP.String(),
			"allocated_at": allocatedPort.AllocatedAt,
		}).Info("TURN端口分配成功（复用现有连接）")

		return allocatedPort, nil
	}

	// 如果没有中继连接，创建新的
	relayConn, err := tc.client.Allocate()
	if err != nil {
		return nil, fmt.Errorf("TURN端口分配失败: %w", err)
	}

	// 获取分配的中继地址
	relayAddr, ok := relayConn.LocalAddr().(*net.UDPAddr)
	if !ok {
		relayConn.Close()
		return nil, fmt.Errorf("无法获取TURN中继地址")
	}

	// 保存中继连接
	tc.relayConn = relayConn

	allocatedPort := &AllocatedPort{
		Port:        relayAddr.Port,
		RelayConn:   relayConn,
		AllocatedAt: time.Now(),
		LastUsed:    time.Now(),
		IsActive:    true,
		UsageCount:  0,
	}

	// 保存分配的端口
	tc.portMutex.Lock()
	tc.allocatedPorts[relayAddr.Port] = allocatedPort
	tc.portMutex.Unlock()

	tc.logger.WithFields(logrus.Fields{
		"port":         relayAddr.Port,
		"relay_ip":     relayAddr.IP.String(),
		"allocated_at": allocatedPort.AllocatedAt,
	}).Info("TURN端口分配成功（新建连接）")

	return allocatedPort, nil
}

// ReleasePort 释放TURN端口
func (tc *TURNClient) ReleasePort(port int) error {
	tc.portMutex.Lock()
	defer tc.portMutex.Unlock()

	allocatedPort, exists := tc.allocatedPorts[port]
	if !exists {
		return fmt.Errorf("端口 %d 未分配", port)
	}

	// 检查是否是共享的中继连接
	if allocatedPort.RelayConn == tc.relayConn {
		// 如果是共享连接，只标记为非活跃，不关闭连接
		tc.logger.WithField("port", port).Info("释放共享TURN端口（不关闭连接）")
	} else {
		// 如果是独立连接，关闭它
		if allocatedPort.RelayConn != nil {
			allocatedPort.RelayConn.Close()
		}
	}

	// 标记为非活跃
	allocatedPort.IsActive = false

	// 从管理列表中移除
	delete(tc.allocatedPorts, port)

	tc.logger.WithFields(logrus.Fields{
		"port":           port,
		"allocated_time": time.Since(allocatedPort.AllocatedAt),
		"usage_count":    allocatedPort.UsageCount,
	}).Info("TURN端口释放成功")

	return nil
}

// GetPortStatus 获取端口状态
func (tc *TURNClient) GetPortStatus(port int) (*AllocatedPort, error) {
	tc.portMutex.RLock()
	defer tc.portMutex.RUnlock()

	allocatedPort, exists := tc.allocatedPorts[port]
	if !exists {
		return nil, fmt.Errorf("端口 %d 未分配", port)
	}

	return allocatedPort, nil
}

// GetAllocatedPorts 获取所有分配的端口
func (tc *TURNClient) GetAllocatedPorts() map[int]*AllocatedPort {
	tc.portMutex.RLock()
	defer tc.portMutex.RUnlock()

	result := make(map[int]*AllocatedPort)
	for port, allocatedPort := range tc.allocatedPorts {
		result[port] = allocatedPort
	}
	return result
}

// UpdatePortUsage 更新端口使用情况
func (tc *TURNClient) UpdatePortUsage(port int) error {
	tc.portMutex.Lock()
	defer tc.portMutex.Unlock()

	allocatedPort, exists := tc.allocatedPorts[port]
	if !exists {
		return fmt.Errorf("端口 %d 未分配", port)
	}

	allocatedPort.LastUsed = time.Now()
	allocatedPort.UsageCount++

	return nil
}

// CleanupInactivePorts 清理非活跃端口
func (tc *TURNClient) CleanupInactivePorts(maxIdleTime time.Duration) {
	tc.portMutex.Lock()
	defer tc.portMutex.Unlock()

	now := time.Now()
	var portsToRemove []int

	for port, allocatedPort := range tc.allocatedPorts {
		if !allocatedPort.IsActive || now.Sub(allocatedPort.LastUsed) > maxIdleTime {
			portsToRemove = append(portsToRemove, port)
		}
	}

	for _, port := range portsToRemove {
		allocatedPort := tc.allocatedPorts[port]

		// 检查是否是共享的中继连接
		if allocatedPort.RelayConn == tc.relayConn {
			// 如果是共享连接，只标记为非活跃，不关闭连接
			tc.logger.WithFields(logrus.Fields{
				"port":           port,
				"idle_time":      now.Sub(allocatedPort.LastUsed),
				"allocated_time": now.Sub(allocatedPort.AllocatedAt),
			}).Info("清理非活跃共享TURN端口（不关闭连接）")
		} else {
			// 如果是独立连接，关闭它
			if allocatedPort.RelayConn != nil {
				allocatedPort.RelayConn.Close()
			}
			tc.logger.WithFields(logrus.Fields{
				"port":           port,
				"idle_time":      now.Sub(allocatedPort.LastUsed),
				"allocated_time": now.Sub(allocatedPort.AllocatedAt),
			}).Info("清理非活跃TURN端口")
		}

		delete(tc.allocatedPorts, port)
	}
}

// Close 关闭TURN客户端
func (tc *TURNClient) Close() {
	tc.logger.Info("关闭TURN客户端")
	tc.cancel()

	// 清理所有分配的端口
	tc.portMutex.Lock()
	for port, allocatedPort := range tc.allocatedPorts {
		// 检查是否是共享的中继连接
		if allocatedPort.RelayConn == tc.relayConn {
			tc.logger.WithField("port", port).Debug("标记共享TURN端口为非活跃")
		} else {
			if allocatedPort.RelayConn != nil {
				allocatedPort.RelayConn.Close()
			}
			tc.logger.WithField("port", port).Debug("关闭独立TURN端口连接")
		}
	}
	tc.allocatedPorts = make(map[int]*AllocatedPort)
	tc.portMutex.Unlock()

	if tc.relayConn != nil {
		tc.relayConn.Close()
	}

	if tc.client != nil {
		tc.client.Close()
	}
}
