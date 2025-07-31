package nat_traversal

import (
	"auto-upnp/internal/util"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// TURNPortForwarder TURN端口转发器
type TURNPortForwarder struct {
	logger     *logrus.Logger
	turnClient *TURNClient
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup

	// 转发规则管理
	forwardRules map[string]*ForwardRule
	rulesMutex   sync.RWMutex

	// 连接池管理
	connectionPool map[string]*ConnectionInfo
	poolMutex      sync.RWMutex
}

// ForwardRule 转发规则
type ForwardRule struct {
	ID           string
	ExternalPort int    // TURN中继端口
	LocalPort    int    // 本地目标端口
	Protocol     string // tcp/udp
	Description  string
	IsActive     bool
	CreatedAt    time.Time
	LastActivity time.Time

	// 统计信息
	BytesReceived    int64
	BytesSent        int64
	ConnectionsCount int64
}

// ConnectionInfo 连接信息
type ConnectionInfo struct {
	ID           string
	RemoteAddr   *net.UDPAddr
	LocalConn    net.Conn
	CreatedAt    time.Time
	LastActivity time.Time
	IsActive     bool
}

// NewTURNPortForwarder 创建新的TURN端口转发器
func NewTURNPortForwarder(logger *logrus.Logger, turnClient *TURNClient) *TURNPortForwarder {
	ctx, cancel := context.WithCancel(context.Background())

	return &TURNPortForwarder{
		logger:         logger,
		turnClient:     turnClient,
		ctx:            ctx,
		cancel:         cancel,
		forwardRules:   make(map[string]*ForwardRule),
		connectionPool: make(map[string]*ConnectionInfo),
	}
}

// CreateForwardRule 创建转发规则
func (tpf *TURNPortForwarder) CreateForwardRule(localPort int, protocol string, description string) (*ForwardRule, error) {
	// 验证协议
	protocol = strings.ToLower(protocol)
	if protocol != "tcp" && protocol != "udp" {
		return nil, fmt.Errorf("不支持的协议: %s", protocol)
	}

	// 检查本地端口是否可达（可选检查，失败不会阻止创建）
	tpf.checkLocalPort(localPort, protocol)

	// 分配TURN中继端口
	externalPort, err := tpf.allocateExternalPort()
	if err != nil {
		return nil, fmt.Errorf("分配外部端口失败: %w", err)
	}

	rule := &ForwardRule{
		ID:           fmt.Sprintf("%s-%d-%d", protocol, localPort, externalPort),
		ExternalPort: externalPort,
		LocalPort:    localPort,
		Protocol:     protocol,
		Description:  description,
		IsActive:     true,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
	}

	tpf.rulesMutex.Lock()
	tpf.forwardRules[rule.ID] = rule
	tpf.rulesMutex.Unlock()

	// 启动转发服务
	if err := tpf.startForwarding(rule); err != nil {
		// 清理规则
		tpf.rulesMutex.Lock()
		delete(tpf.forwardRules, rule.ID)
		tpf.rulesMutex.Unlock()
		return nil, fmt.Errorf("启动转发服务失败: %w", err)
	}

	tpf.logger.WithFields(logrus.Fields{
		"rule_id":       rule.ID,
		"local_port":    localPort,
		"external_port": externalPort,
		"protocol":      protocol,
		"description":   description,
	}).Info("创建TURN端口转发规则成功")

	return rule, nil
}

// checkLocalPort 检查本地端口是否可达
func (tpf *TURNPortForwarder) checkLocalPort(port int, protocol string) {
	var status bool
	if protocol == "tcp" {
		status = util.IsTCPPortActive(port)
	} else {
		status = util.IsUDPPortActive(port)
	}
	if status {
		tpf.logger.WithFields(logrus.Fields{
			"port":     port,
			"protocol": protocol,
		}).Info("本地端口可达")
	} else {
		tpf.logger.WithFields(logrus.Fields{
			"port":     port,
			"protocol": protocol,
		}).Warn("本地端口不可达")
	}
}

// allocateExternalPort 分配外部端口（与TURN服务器协商）
func (tpf *TURNPortForwarder) allocateExternalPort() (int, error) {
	if tpf.turnClient == nil {
		return 0, fmt.Errorf("TURN客户端未初始化")
	}

	// 检查TURN客户端是否已连接
	if tpf.turnClient.client == nil {
		return 0, fmt.Errorf("TURN客户端未连接到服务器")
	}

	// 使用新的端口分配管理功能
	allocatedPort, err := tpf.turnClient.AllocatePort()
	if err != nil {
		return 0, fmt.Errorf("TURN服务器端口分配失败: %w", err)
	}

	tpf.logger.WithFields(logrus.Fields{
		"port":         allocatedPort.Port,
		"allocated_at": allocatedPort.AllocatedAt,
	}).Info("TURN服务器端口分配成功")

	return allocatedPort.Port, nil
}

// startForwarding 启动转发服务
func (tpf *TURNPortForwarder) startForwarding(rule *ForwardRule) error {
	if rule.Protocol == "tcp" {
		return tpf.startTCPForwarding(rule)
	} else {
		return tpf.startUDPForwarding(rule)
	}
}

// startTCPForwarding 启动TCP转发
func (tpf *TURNPortForwarder) startTCPForwarding(rule *ForwardRule) error {
	tpf.wg.Add(1)
	go func() {
		defer tpf.wg.Done()

		for {
			select {
			case <-tpf.ctx.Done():
				return
			default:
				// 监听TURN中继数据
				data, remoteAddr, err := tpf.turnClient.ReceiveDataFromRelay(5 * time.Second)
				if err != nil {
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						continue
					}
					tpf.logger.WithError(err).Warn("接收TURN TCP数据失败")
					continue
				}

				// 处理TCP连接
				tpf.handleTCPConnection(rule, remoteAddr, data)
			}
		}
	}()

	return nil
}

// startUDPForwarding 启动UDP转发
func (tpf *TURNPortForwarder) startUDPForwarding(rule *ForwardRule) error {
	tpf.wg.Add(1)
	go func() {
		defer tpf.wg.Done()

		for {
			select {
			case <-tpf.ctx.Done():
				return
			default:
				// 监听TURN中继数据
				data, remoteAddr, err := tpf.turnClient.ReceiveDataFromRelay(5 * time.Second)
				if err != nil {
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						continue
					}
					tpf.logger.WithError(err).Warn("接收TURN UDP数据失败")
					continue
				}

				// 处理UDP数据
				tpf.handleUDPData(rule, remoteAddr, data)
			}
		}
	}()

	return nil
}

// handleTCPConnection 处理TCP连接
func (tpf *TURNPortForwarder) handleTCPConnection(rule *ForwardRule, remoteAddr *net.UDPAddr, data []byte) {
	connectionID := fmt.Sprintf("%s-%s", rule.ID, remoteAddr.String())

	tpf.poolMutex.Lock()
	connInfo, exists := tpf.connectionPool[connectionID]
	tpf.poolMutex.Unlock()

	if !exists {
		// 创建新的TCP连接到本地服务
		localConn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", rule.LocalPort))
		if err != nil {
			tpf.logger.WithFields(logrus.Fields{
				"rule_id":    rule.ID,
				"local_port": rule.LocalPort,
				"error":      err,
			}).Error("连接本地TCP服务失败")
			return
		}

		connInfo = &ConnectionInfo{
			ID:           connectionID,
			RemoteAddr:   remoteAddr,
			LocalConn:    localConn,
			CreatedAt:    time.Now(),
			LastActivity: time.Now(),
			IsActive:     true,
		}

		tpf.poolMutex.Lock()
		tpf.connectionPool[connectionID] = connInfo
		tpf.poolMutex.Unlock()

		// 启动双向数据转发
		tpf.wg.Add(2)
		go tpf.forwardTCPData(rule, connInfo, "local-to-remote")
		go tpf.forwardTCPData(rule, connInfo, "remote-to-local")
	}

	// 更新活动时间和端口使用统计
	connInfo.LastActivity = time.Now()
	rule.LastActivity = time.Now()
	rule.BytesReceived += int64(len(data))
	rule.ConnectionsCount++

	// 更新TURN端口使用情况
	if err := tpf.turnClient.UpdatePortUsage(rule.ExternalPort); err != nil {
		tpf.logger.WithError(err).Warn("更新TURN端口使用统计失败")
	}
}

// handleUDPData 处理UDP数据
func (tpf *TURNPortForwarder) handleUDPData(rule *ForwardRule, remoteAddr *net.UDPAddr, data []byte) {
	// 连接到本地UDP服务
	localAddr := &net.UDPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: rule.LocalPort,
	}

	localConn, err := net.DialUDP("udp", nil, localAddr)
	if err != nil {
		tpf.logger.WithFields(logrus.Fields{
			"rule_id":    rule.ID,
			"local_port": rule.LocalPort,
			"error":      err,
		}).Error("连接本地UDP服务失败")
		return
	}
	defer localConn.Close()

	// 发送数据到本地服务
	_, err = localConn.Write(data)
	if err != nil {
		tpf.logger.WithFields(logrus.Fields{
			"rule_id":    rule.ID,
			"local_port": rule.LocalPort,
			"error":      err,
		}).Error("发送数据到本地UDP服务失败")
		return
	}

	// 读取响应
	responseBuffer := make([]byte, 4096)
	localConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := localConn.Read(responseBuffer)
	if err != nil {
		tpf.logger.WithFields(logrus.Fields{
			"rule_id":    rule.ID,
			"local_port": rule.LocalPort,
			"error":      err,
		}).Warn("读取本地UDP服务响应失败")
		return
	}

	// 发送响应回远程客户端
	err = tpf.turnClient.SendDataViaRelay(remoteAddr, responseBuffer[:n])
	if err != nil {
		tpf.logger.WithFields(logrus.Fields{
			"rule_id":     rule.ID,
			"remote_addr": remoteAddr.String(),
			"error":       err,
		}).Error("发送UDP响应失败")
		return
	}

	// 更新统计信息
	rule.LastActivity = time.Now()
	rule.BytesReceived += int64(len(data))
	rule.BytesSent += int64(n)
	rule.ConnectionsCount++

	// 更新TURN端口使用情况
	if err := tpf.turnClient.UpdatePortUsage(rule.ExternalPort); err != nil {
		tpf.logger.WithError(err).Warn("更新TURN端口使用统计失败")
	}

	tpf.logger.WithFields(logrus.Fields{
		"rule_id":       rule.ID,
		"remote_addr":   remoteAddr.String(),
		"data_size":     len(data),
		"response_size": n,
	}).Debug("UDP数据转发完成")
}

// forwardTCPData TCP数据转发
func (tpf *TURNPortForwarder) forwardTCPData(rule *ForwardRule, connInfo *ConnectionInfo, direction string) {
	defer tpf.wg.Done()

	var src, dst net.Conn
	if direction == "local-to-remote" {
		src = connInfo.LocalConn
		dst = nil // TURN客户端没有直接的连接对象
	} else {
		src = nil
		dst = connInfo.LocalConn
	}

	buffer := make([]byte, 4096)
	for {
		select {
		case <-tpf.ctx.Done():
			return
		default:
			if direction == "local-to-remote" {
				// 从本地读取数据发送到远程
				n, err := src.Read(buffer)
				if err != nil {
					if err != io.EOF {
						tpf.logger.WithError(err).Warn("读取本地TCP数据失败")
					}
					return
				}

				err = tpf.turnClient.SendDataViaRelay(connInfo.RemoteAddr, buffer[:n])
				if err != nil {
					tpf.logger.WithError(err).Error("发送TCP数据到远程失败")
					return
				}

				rule.BytesSent += int64(n)
			} else {
				// 从TURN接收数据发送到本地
				data, _, err := tpf.turnClient.ReceiveDataFromRelay(5 * time.Second)
				if err != nil {
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						continue
					}
					tpf.logger.WithError(err).Warn("接收TURN TCP数据失败")
					return
				}

				_, err = dst.Write(data)
				if err != nil {
					tpf.logger.WithError(err).Error("发送TCP数据到本地失败")
					return
				}

				rule.BytesReceived += int64(len(data))
			}

			connInfo.LastActivity = time.Now()
			rule.LastActivity = time.Now()
		}
	}
}

// RemoveForwardRule 移除转发规则
func (tpf *TURNPortForwarder) RemoveForwardRule(ruleID string) error {
	tpf.rulesMutex.Lock()
	rule, exists := tpf.forwardRules[ruleID]
	if !exists {
		tpf.rulesMutex.Unlock()
		return fmt.Errorf("转发规则不存在: %s", ruleID)
	}

	rule.IsActive = false
	delete(tpf.forwardRules, ruleID)
	tpf.rulesMutex.Unlock()

	// 释放TURN端口
	if err := tpf.turnClient.ReleasePort(rule.ExternalPort); err != nil {
		tpf.logger.WithFields(logrus.Fields{
			"rule_id":       ruleID,
			"external_port": rule.ExternalPort,
			"error":         err,
		}).Warn("释放TURN端口失败")
	}

	// 清理相关连接
	tpf.cleanupConnections(ruleID)

	tpf.logger.WithFields(logrus.Fields{
		"rule_id":       ruleID,
		"external_port": rule.ExternalPort,
		"local_port":    rule.LocalPort,
		"protocol":      rule.Protocol,
	}).Info("移除TURN端口转发规则成功")
	return nil
}

// cleanupConnections 清理连接
func (tpf *TURNPortForwarder) cleanupConnections(ruleID string) {
	tpf.poolMutex.Lock()
	defer tpf.poolMutex.Unlock()

	for connID, connInfo := range tpf.connectionPool {
		if len(connID) >= len(ruleID) && connID[:len(ruleID)] == ruleID {
			if connInfo.LocalConn != nil {
				connInfo.LocalConn.Close()
			}
			connInfo.IsActive = false
			delete(tpf.connectionPool, connID)
		}
	}
}

// GetForwardRules 获取所有转发规则
func (tpf *TURNPortForwarder) GetForwardRules() map[string]*ForwardRule {
	tpf.rulesMutex.RLock()
	defer tpf.rulesMutex.RUnlock()

	result := make(map[string]*ForwardRule)
	for id, rule := range tpf.forwardRules {
		result[id] = rule
	}
	return result
}

// GetActiveConnections 获取活跃连接
func (tpf *TURNPortForwarder) GetActiveConnections() map[string]*ConnectionInfo {
	tpf.poolMutex.RLock()
	defer tpf.poolMutex.RUnlock()

	result := make(map[string]*ConnectionInfo)
	for id, conn := range tpf.connectionPool {
		if conn.IsActive {
			result[id] = conn
		}
	}
	return result
}

// GetStatistics 获取统计信息
func (tpf *TURNPortForwarder) GetStatistics() map[string]interface{} {
	rules := tpf.GetForwardRules()
	connections := tpf.GetActiveConnections()
	allocatedPorts := tpf.turnClient.GetAllocatedPorts()

	var totalBytesReceived, totalBytesSent int64
	var totalConnections int64

	for _, rule := range rules {
		totalBytesReceived += rule.BytesReceived
		totalBytesSent += rule.BytesSent
	}

	totalConnections = int64(len(connections))

	// 统计TURN端口信息
	var totalAllocatedPorts int
	var activePorts int
	for _, port := range allocatedPorts {
		totalAllocatedPorts++
		if port.IsActive {
			activePorts++
		}
	}

	return map[string]interface{}{
		"total_rules":           len(rules),
		"active_connections":    totalConnections,
		"total_bytes_received":  totalBytesReceived,
		"total_bytes_sent":      totalBytesSent,
		"total_allocated_ports": totalAllocatedPorts,
		"active_turn_ports":     activePorts,
		"rules":                 rules,
		"connections":           connections,
		"allocated_ports":       allocatedPorts,
	}
}

// StartPortCleanup 启动端口清理任务
func (tpf *TURNPortForwarder) StartPortCleanup(cleanupInterval time.Duration, maxIdleTime time.Duration) {
	tpf.wg.Add(1)
	go func() {
		defer tpf.wg.Done()
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-tpf.ctx.Done():
				return
			case <-ticker.C:
				tpf.cleanupInactivePorts(maxIdleTime)
			}
		}
	}()

	tpf.logger.WithFields(logrus.Fields{
		"cleanup_interval": cleanupInterval,
		"max_idle_time":    maxIdleTime,
	}).Info("TURN端口清理任务已启动")
}

// cleanupInactivePorts 清理非活跃端口
func (tpf *TURNPortForwarder) cleanupInactivePorts(maxIdleTime time.Duration) {
	// 清理TURN客户端的非活跃端口
	tpf.turnClient.CleanupInactivePorts(maxIdleTime)

	// 清理转发规则中的非活跃连接
	tpf.rulesMutex.RLock()
	rules := make(map[string]*ForwardRule)
	for id, rule := range tpf.forwardRules {
		rules[id] = rule
	}
	tpf.rulesMutex.RUnlock()

	now := time.Now()
	for ruleID, rule := range rules {
		if !rule.IsActive {
			continue
		}

		// 检查规则是否长时间未活动
		if now.Sub(rule.LastActivity) > maxIdleTime {
			tpf.logger.WithFields(logrus.Fields{
				"rule_id":       ruleID,
				"last_activity": rule.LastActivity,
				"idle_time":     now.Sub(rule.LastActivity),
			}).Info("清理非活跃转发规则")

			// 移除非活跃规则
			if err := tpf.RemoveForwardRule(ruleID); err != nil {
				tpf.logger.WithError(err).Error("清理非活跃转发规则失败")
			}
		}
	}
}

// Close 关闭端口转发器
func (tpf *TURNPortForwarder) Close() {
	tpf.logger.Info("关闭TURN端口转发器")
	tpf.cancel()

	// 清理所有连接
	tpf.poolMutex.Lock()
	for _, connInfo := range tpf.connectionPool {
		if connInfo.LocalConn != nil {
			connInfo.LocalConn.Close()
		}
	}
	tpf.connectionPool = make(map[string]*ConnectionInfo)
	tpf.poolMutex.Unlock()

	// 等待所有goroutine结束
	tpf.wg.Wait()
}
