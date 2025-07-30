package nat_traversal

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// NATTraversalConfig NAT穿透配置
type NATTraversalConfig struct {
	Enabled     bool         `mapstructure:"enabled"`
	UseTURN     bool         `mapstructure:"use_turn"`
	TURNServers []TURNServer `mapstructure:"turn_servers"`
	// 健康检查配置
	HealthCheck HealthCheckConfig `mapstructure:"health_check"`
}

// HealthCheckConfig 健康检查配置
type HealthCheckConfig struct {
	Enabled  bool          `mapstructure:"enabled"`
	Interval time.Duration `mapstructure:"interval"`
	Timeout  time.Duration `mapstructure:"timeout"`
}

// HoleInfo 打洞信息
type HoleInfo struct {
	// 连接信息
	RemoteAddr   net.Addr
	TargetPort   int
	LocalPort    int
	Protocol     string
	Description  string
	CreatedAt    time.Time
	LastActivity time.Time
	IsActive     bool

	// TURN相关 - 每个HoleInfo独立的客户端和转发器
	TURNClient        *TURNClient
	TURNPortForwarder *TURNPortForwarder
	ForwardRuleID     string
	ExternalAddr      *net.UDPAddr

	// 数据流转统计
	BytesReceived int64
	BytesSent     int64
	Connections   int64
}

// NATTraversal NAT穿透管理器
type NATTraversal struct {
	config *NATTraversalConfig
	logger *logrus.Logger
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// 打洞信息 - 每个HoleInfo包含独立的TURN客户端和转发器
	holes      map[string]*HoleInfo
	holesMutex sync.RWMutex

	// 回调函数
	onHoleCreated func(allocatedPort int, sourcePort int, protocol string)
	onHoleClosed  func(allocatedPort int, sourcePort int, protocol string)
	onHoleFailed  func(allocatedPort int, sourcePort int, protocol string, error error)

	// 健康检查相关
	healthCheckInterval time.Duration
	healthCheckEnabled  bool
}

// NewNATTraversal 创建新的NAT穿透管理器
func NewNATTraversal(config *NATTraversalConfig, logger *logrus.Logger) *NATTraversal {
	ctx, cancel := context.WithCancel(context.Background())

	// 设置健康检查配置
	healthCheckInterval := 5 * time.Minute // 默认5分钟
	healthCheckEnabled := true

	if config.HealthCheck.Interval > 0 {
		healthCheckInterval = config.HealthCheck.Interval
	}
	if !config.HealthCheck.Enabled {
		healthCheckEnabled = false
	}

	nt := &NATTraversal{
		config:              config,
		logger:              logger,
		ctx:                 ctx,
		cancel:              cancel,
		holes:               make(map[string]*HoleInfo),
		healthCheckInterval: healthCheckInterval,
		healthCheckEnabled:  healthCheckEnabled,
	}

	return nt
}

// Start 启动NAT穿透服务
func (nt *NATTraversal) Start() error {
	if !nt.config.Enabled {
		nt.logger.Info("NAT穿透功能已禁用")
		return nil
	}

	nt.logger.Info("启动NAT穿透服务")

	// 如果启用了TURN，检测TURN服务器可用性
	if nt.config.UseTURN {
		nt.logger.Info("开始检测TURN服务器可用性...")

		// 检测TURN服务器
		availableServers, err := nt.detectTURNServers()
		if err != nil {
			nt.logger.WithError(err).Warn("TURN服务器检测失败")
			return fmt.Errorf("TURN服务器检测失败: %w", err)
		}

		if len(availableServers) == 0 {
			nt.logger.Warn("没有可用的TURN服务器")
			return fmt.Errorf("没有可用的TURN服务器")
		}

		nt.logger.WithFields(logrus.Fields{
			"total_servers":     len(nt.config.TURNServers),
			"available_servers": len(availableServers),
			"servers":           availableServers,
		}).Info("TURN服务器检测完成")

		// 启动TURN客户端健康检查任务
		if nt.healthCheckEnabled {
			nt.startTURNHealthCheck()
		}
	}

	nt.logger.Info("NAT穿透服务启动成功")
	return nil
}

// detectTURNServers 检测TURN服务器可用性
func (nt *NATTraversal) detectTURNServers() ([]string, error) {
	var availableServers []string
	var lastError error

	nt.logger.WithField("server_count", len(nt.config.TURNServers)).Info("开始检测TURN服务器")

	for i, server := range nt.config.TURNServers {
		serverAddr := fmt.Sprintf("%s:%d", server.Host, server.Port)

		nt.logger.WithFields(logrus.Fields{
			"server_index": i + 1,
			"server_addr":  serverAddr,
			"username":     server.Username,
			"realm":        server.Realm,
		}).Info("检测TURN服务器")

		// 创建测试客户端
		testClient := NewTURNClient(nt.logger, []TURNServer{server})

		// 设置超时上下文
		timeout := 10 * time.Second
		if nt.config.HealthCheck.Timeout > 0 {
			timeout = nt.config.HealthCheck.Timeout
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)

		// 在goroutine中测试连接
		errChan := make(chan error, 1)
		go func() {
			defer cancel()
			defer testClient.Close()

			_, err := testClient.ConnectToTURN()
			errChan <- err
		}()

		// 等待连接结果或超时
		select {
		case err := <-errChan:
			if err != nil {
				nt.logger.WithFields(logrus.Fields{
					"server_addr": serverAddr,
					"error":       err,
				}).Warn("TURN服务器连接失败")
				lastError = err
			} else {
				nt.logger.WithFields(logrus.Fields{
					"server_addr": serverAddr,
				}).Info("TURN服务器连接成功")
				availableServers = append(availableServers, serverAddr)
			}
		case <-ctx.Done():
			nt.logger.WithFields(logrus.Fields{
				"server_addr": serverAddr,
				"timeout":     "10s",
			}).Warn("TURN服务器连接超时")
			lastError = fmt.Errorf("连接超时")
		}
	}

	// 记录检测结果
	if len(availableServers) > 0 {
		nt.logger.WithFields(logrus.Fields{
			"available_servers": availableServers,
			"total_servers":     len(nt.config.TURNServers),
		}).Info("TURN服务器检测完成")
	} else {
		nt.logger.WithError(lastError).Error("所有TURN服务器都不可用")
		return nil, fmt.Errorf("所有TURN服务器都不可用: %w", lastError)
	}

	return availableServers, nil
}

// TriggerTURNHealthCheck 手动触发TURN客户端健康检查
func (nt *NATTraversal) TriggerTURNHealthCheck() map[string]interface{} {
	nt.logger.Info("手动触发TURN客户端健康检查")

	nt.performTURNHealthCheck()

	// 返回健康检查结果
	return nt.GetTURNHealthStatus()
}

// GetTURNHealthStatus 获取TURN客户端健康状态
func (nt *NATTraversal) GetTURNHealthStatus() map[string]interface{} {
	nt.holesMutex.RLock()
	defer nt.holesMutex.RUnlock()

	var healthyClients, totalClients int
	var clientDetails []map[string]interface{}

	for holeKey, hole := range nt.holes {
		if hole.TURNClient == nil || !hole.IsActive {
			continue
		}

		totalClients++

		// 检查TURN客户端状态
		status := hole.TURNClient.GetRelayStatus()
		if connected, ok := status["connected"].(bool); ok && connected {
			healthyClients++
		}

		// 收集客户端详细信息
		clientDetail := map[string]interface{}{
			"hole_key":      holeKey,
			"local_port":    hole.LocalPort,
			"protocol":      hole.Protocol,
			"forward_rule":  hole.ForwardRuleID,
			"external_port": hole.TargetPort,
			"connected":     status["connected"],
			"status":        status,
			"last_activity": hole.LastActivity,
		}
		clientDetails = append(clientDetails, clientDetail)
	}

	healthPercentage := 100.0
	if totalClients > 0 {
		healthPercentage = float64(healthyClients) / float64(totalClients) * 100
	}

	return map[string]interface{}{
		"total_clients":        totalClients,
		"healthy_clients":      healthyClients,
		"health_percentage":    healthPercentage,
		"check_interval":       nt.healthCheckInterval,
		"health_check_enabled": nt.healthCheckEnabled,
		"clients":              clientDetails,
	}
}

// startTURNHealthCheck 启动TURN客户端健康检查任务
func (nt *NATTraversal) startTURNHealthCheck() {
	nt.wg.Add(1)
	go func() {
		defer nt.wg.Done()

		ticker := time.NewTicker(nt.healthCheckInterval)
		defer ticker.Stop()

		nt.logger.WithFields(logrus.Fields{
			"check_interval": nt.healthCheckInterval,
		}).Info("TURN客户端健康检查任务已启动")

		for {
			select {
			case <-nt.ctx.Done():
				nt.logger.Info("TURN客户端健康检查任务已停止")
				return
			case <-ticker.C:
				nt.performTURNHealthCheck()
			}
		}
	}()
}

// performTURNHealthCheck 执行TURN客户端健康检查
func (nt *NATTraversal) performTURNHealthCheck() {
	nt.holesMutex.RLock()
	holes := make(map[string]*HoleInfo)
	for key, hole := range nt.holes {
		holes[key] = hole
	}
	nt.holesMutex.RUnlock()

	var healthyClients, totalClients int
	var unhealthyHoles []string

	for holeKey, hole := range holes {
		if hole.TURNClient == nil || !hole.IsActive {
			continue
		}

		totalClients++

		// 检查TURN客户端状态
		status := hole.TURNClient.GetRelayStatus()
		if connected, ok := status["connected"].(bool); ok && connected {
			healthyClients++
			nt.logger.WithFields(logrus.Fields{
				"hole_key": holeKey,
				"status":   "healthy",
			}).Debug("TURN客户端健康检查通过")
		} else {
			unhealthyHoles = append(unhealthyHoles, holeKey)
			nt.logger.WithFields(logrus.Fields{
				"hole_key": holeKey,
				"status":   status,
			}).Warn("TURN客户端健康检查失败")
		}
	}

	// 记录健康检查结果
	nt.logger.WithFields(logrus.Fields{
		"total_clients":   totalClients,
		"healthy_clients": healthyClients,
		"unhealthy_holes": len(unhealthyHoles),
		"health_percentage": func() float64 {
			if totalClients == 0 {
				return 100.0
			}
			return float64(healthyClients) / float64(totalClients) * 100
		}(),
	}).Info("TURN客户端健康检查完成")

	// 如果有不健康的客户端，尝试重新连接
	if len(unhealthyHoles) > 0 {
		nt.logger.WithField("unhealthy_holes", unhealthyHoles).Info("开始修复不健康的TURN客户端")
		nt.repairUnhealthyTURNClients(unhealthyHoles)
	}
}

// repairUnhealthyTURNClients 修复不健康的TURN客户端
func (nt *NATTraversal) repairUnhealthyTURNClients(unhealthyHoles []string) {
	for _, holeKey := range unhealthyHoles {
		nt.holesMutex.Lock()
		hole, exists := nt.holes[holeKey]
		if !exists {
			nt.holesMutex.Unlock()
			continue
		}
		nt.holesMutex.Unlock()

		nt.logger.WithField("hole_key", holeKey).Info("尝试修复TURN客户端")

		// 关闭旧的客户端和转发器
		if hole.TURNPortForwarder != nil {
			hole.TURNPortForwarder.Close()
		}
		if hole.TURNClient != nil {
			hole.TURNClient.Close()
		}

		// 创建新的TURN客户端
		newTurnClient := NewTURNClient(nt.logger, nt.config.TURNServers)

		// 尝试重新连接
		turnResponse, err := newTurnClient.ConnectToTURN()
		if err != nil {
			nt.logger.WithFields(logrus.Fields{
				"hole_key": holeKey,
				"error":    err,
			}).Error("TURN客户端重新连接失败")
			newTurnClient.Close()
			continue
		}

		// 创建新的端口转发器
		newTurnPortForwarder := NewTURNPortForwarder(nt.logger, newTurnClient)

		// 重新创建转发规则
		forwardRule, err := newTurnPortForwarder.CreateForwardRule(hole.LocalPort, hole.Protocol, hole.Description)
		if err != nil {
			nt.logger.WithFields(logrus.Fields{
				"hole_key": holeKey,
				"error":    err,
			}).Error("重新创建TURN转发规则失败")
			newTurnPortForwarder.Close()
			newTurnClient.Close()
			continue
		}

		// 更新HoleInfo
		nt.holesMutex.Lock()
		hole.TURNClient = newTurnClient
		hole.TURNPortForwarder = newTurnPortForwarder
		hole.ForwardRuleID = forwardRule.ID
		hole.TargetPort = forwardRule.ExternalPort
		hole.ExternalAddr = turnResponse.RelayAddr
		hole.RemoteAddr = &net.UDPAddr{
			IP:   turnResponse.RelayIP,
			Port: forwardRule.ExternalPort,
		}
		nt.holesMutex.Unlock()

		nt.logger.WithFields(logrus.Fields{
			"hole_key":      holeKey,
			"new_rule_id":   forwardRule.ID,
			"external_port": forwardRule.ExternalPort,
			"relay_ip":      turnResponse.RelayIP.String(),
		}).Info("TURN客户端修复成功")
	}
}

// Stop 停止NAT穿透服务
func (nt *NATTraversal) Stop() {
	nt.logger.Info("停止NAT穿透服务")
	nt.cancel()

	// 关闭所有HoleInfo中的TURN客户端和转发器
	nt.holesMutex.Lock()
	for _, hole := range nt.holes {
		if hole.TURNPortForwarder != nil {
			hole.TURNPortForwarder.Close()
		}
		if hole.TURNClient != nil {
			hole.TURNClient.Close()
		}
	}
	nt.holesMutex.Unlock()

	nt.wg.Wait()
	nt.logger.Info("NAT穿透服务已停止")
}

// CreateHole 创建打洞
func (nt *NATTraversal) CreateHole(port int, protocol string, description string) (net.Addr, error) {
	if !nt.config.Enabled {
		return nil, fmt.Errorf("NAT穿透功能已禁用")
	}

	holeKey := fmt.Sprintf("%d-%s", port, protocol)

	nt.holesMutex.Lock()

	// 检查是否已存在
	if _, exists := nt.holes[holeKey]; exists {
		nt.holesMutex.Unlock()
		return nil, fmt.Errorf("打洞已存在: %s", holeKey)
	}

	hole := &HoleInfo{
		LocalPort:    port,
		Protocol:     protocol,
		Description:  description,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		IsActive:     true,
	}

	// 如果启用了TURN，为每个HoleInfo创建独立的TURN客户端和转发器
	if nt.config.UseTURN {
		nt.holesMutex.Unlock() // 临时释放锁，避免死锁

		// 创建独立的TURN客户端
		turnClient := NewTURNClient(nt.logger, nt.config.TURNServers)

		// 连接到TURN服务器
		turnResponse, err := turnClient.ConnectToTURN()
		if err != nil {
			nt.logger.WithError(err).Error("TURN服务器连接失败")
			return nil, fmt.Errorf("TURN服务器连接失败: %w", err)
		}

		// 创建独立的TURN端口转发器
		turnPortForwarder := NewTURNPortForwarder(nt.logger, turnClient)

		// 创建转发规则
		forwardRule, err := turnPortForwarder.CreateForwardRule(port, protocol, description)
		if err != nil {
			nt.logger.WithError(err).Error("创建TURN转发规则失败")
			turnClient.Close()
			return nil, fmt.Errorf("创建TURN转发规则失败: %w", err)
		}

		// 重新获取锁并更新hole信息
		nt.holesMutex.Lock()
		hole.TURNClient = turnClient
		hole.TURNPortForwarder = turnPortForwarder
		hole.ForwardRuleID = forwardRule.ID
		hole.TargetPort = forwardRule.ExternalPort
		hole.ExternalAddr = turnResponse.RelayAddr
		hole.RemoteAddr = &net.UDPAddr{
			IP:   turnResponse.RelayIP,
			Port: forwardRule.ExternalPort,
		}

		nt.logger.WithFields(logrus.Fields{
			"hole_key":      holeKey,
			"forward_rule":  forwardRule.ID,
			"external_port": forwardRule.ExternalPort,
			"relay_ip":      turnResponse.RelayIP.String(),
			"relay_port":    turnResponse.RelayPort,
		}).Info("TURN转发规则创建成功（独立客户端）")
	}

	nt.holes[holeKey] = hole
	nt.holesMutex.Unlock()

	nt.logger.WithFields(logrus.Fields{
		"hole_key":     holeKey,
		"port":         port,
		"protocol":     protocol,
		"description":  description,
		"use_turn":     nt.config.UseTURN,
		"target_port":  hole.TargetPort,
		"forward_rule": hole.ForwardRuleID,
	}).Info("创建打洞成功")

	// 触发回调
	if nt.onHoleCreated != nil {
		nt.onHoleCreated(hole.LocalPort, hole.TargetPort, protocol)
	}

	return hole.RemoteAddr, nil
}

// CloseHole 关闭打洞
func (nt *NATTraversal) CloseHole(allocatedPort int, sourcePort int, protocol string) error {
	holeKey := fmt.Sprintf("%d-%d-%s", allocatedPort, sourcePort, protocol)

	nt.holesMutex.Lock()

	hole, exists := nt.holes[holeKey]
	if !exists {
		nt.holesMutex.Unlock()
		return fmt.Errorf("打洞不存在: %s", holeKey)
	}

	// 保存转发规则ID，然后释放锁
	forwardRuleID := hole.ForwardRuleID
	nt.holesMutex.Unlock()

	// 如果有TURN转发规则，先移除转发规则
	if forwardRuleID != "" && hole.TURNPortForwarder != nil {
		err := hole.TURNPortForwarder.RemoveForwardRule(forwardRuleID)
		if err != nil {
			nt.logger.WithError(err).Warn("移除TURN转发规则失败")
		} else {
			nt.logger.WithField("forward_rule", forwardRuleID).Info("TURN转发规则移除成功")
		}
	}

	// 重新获取锁进行清理
	nt.holesMutex.Lock()
	hole, exists = nt.holes[holeKey]
	if exists {
		// 关闭独立的TURN客户端和转发器
		if hole.TURNPortForwarder != nil {
			hole.TURNPortForwarder.Close()
		}
		if hole.TURNClient != nil {
			hole.TURNClient.Close()
		}

		hole.IsActive = false
		delete(nt.holes, holeKey)
	}
	nt.holesMutex.Unlock()

	nt.logger.WithFields(logrus.Fields{
		"hole_key":       holeKey,
		"allocated_port": allocatedPort,
		"source_port":    sourcePort,
		"protocol":       protocol,
		"forward_rule":   forwardRuleID,
	}).Info("关闭打洞成功")

	// 触发回调
	if nt.onHoleClosed != nil {
		nt.onHoleClosed(allocatedPort, sourcePort, protocol)
	}

	return nil
}

// GetHoles 获取所有打洞信息
func (nt *NATTraversal) GetHoles() map[string]*HoleInfo {
	nt.holesMutex.RLock()
	defer nt.holesMutex.RUnlock()

	result := make(map[string]*HoleInfo)
	for key, hole := range nt.holes {
		result[key] = hole
	}
	return result
}

// GetActiveHoles 获取活跃的打洞
func (nt *NATTraversal) GetActiveHoles() []*HoleInfo {
	nt.holesMutex.RLock()
	defer nt.holesMutex.RUnlock()

	var activeHoles []*HoleInfo
	for _, hole := range nt.holes {
		if hole.IsActive {
			activeHoles = append(activeHoles, hole)
		}
	}
	return activeHoles
}

// GetExternalAddress 获取外部地址信息
func (nt *NATTraversal) GetExternalAddress() *net.UDPAddr {
	// 返回第一个活跃HoleInfo的外部地址
	nt.holesMutex.RLock()
	defer nt.holesMutex.RUnlock()

	for _, hole := range nt.holes {
		if hole.IsActive && hole.ExternalAddr != nil {
			return hole.ExternalAddr
		}
	}
	return nil
}

// GetTURNStatus 获取TURN状态
func (nt *NATTraversal) GetTURNStatus() map[string]interface{} {
	nt.holesMutex.RLock()
	defer nt.holesMutex.RUnlock()

	// 统计所有活跃的TURN客户端
	var activeClients int
	var totalClients int

	for _, hole := range nt.holes {
		if hole.TURNClient != nil {
			totalClients++
			if hole.IsActive {
				activeClients++
			}
		}
	}

	if totalClients == 0 {
		return map[string]interface{}{
			"available": false,
			"message":   "没有TURN客户端",
		}
	}

	return map[string]interface{}{
		"available":      true,
		"total_clients":  totalClients,
		"active_clients": activeClients,
		"message":        fmt.Sprintf("有 %d 个活跃的TURN客户端", activeClients),
	}
}

// SetCallbacks 设置回调函数
func (nt *NATTraversal) SetCallbacks(
	onHoleCreated func(allocatedPort int, sourcePort int, protocol string),
	onHoleClosed func(allocatedPort int, sourcePort int, protocol string),
	onHoleFailed func(allocatedPort int, sourcePort int, protocol string, error error),
) {
	nt.onHoleCreated = onHoleCreated
	nt.onHoleClosed = onHoleClosed
	nt.onHoleFailed = onHoleFailed
}

// CreateTURNForwardRule 创建TURN端口转发规则
func (nt *NATTraversal) CreateTURNForwardRule(localPort int, protocol string, description string) (*ForwardRule, error) {
	// 为新的转发规则创建独立的TURN客户端和转发器
	turnClient := NewTURNClient(nt.logger, nt.config.TURNServers)

	// 连接到TURN服务器
	_, err := turnClient.ConnectToTURN()
	if err != nil {
		turnClient.Close()
		return nil, fmt.Errorf("TURN服务器连接失败: %w", err)
	}

	// 创建独立的TURN端口转发器
	turnPortForwarder := NewTURNPortForwarder(nt.logger, turnClient)

	// 创建转发规则
	forwardRule, err := turnPortForwarder.CreateForwardRule(localPort, protocol, description)
	if err != nil {
		turnPortForwarder.Close()
		turnClient.Close()
		return nil, fmt.Errorf("创建TURN转发规则失败: %w", err)
	}

	nt.logger.WithFields(logrus.Fields{
		"local_port":    localPort,
		"protocol":      protocol,
		"forward_rule":  forwardRule.ID,
		"external_port": forwardRule.ExternalPort,
	}).Info("创建独立TURN转发规则成功")

	return forwardRule, nil
}

// RemoveTURNForwardRule 移除TURN端口转发规则
func (nt *NATTraversal) RemoveTURNForwardRule(ruleID string) error {
	nt.holesMutex.RLock()
	defer nt.holesMutex.RUnlock()

	// 查找包含该转发规则的HoleInfo
	for _, hole := range nt.holes {
		if hole.ForwardRuleID == ruleID && hole.TURNPortForwarder != nil {
			return hole.TURNPortForwarder.RemoveForwardRule(ruleID)
		}
	}

	return fmt.Errorf("未找到转发规则: %s", ruleID)
}

// GetTURNForwardRules 获取所有TURN转发规则
func (nt *NATTraversal) GetTURNForwardRules() map[string]*ForwardRule {
	nt.holesMutex.RLock()
	defer nt.holesMutex.RUnlock()

	allRules := make(map[string]*ForwardRule)

	for _, hole := range nt.holes {
		if hole.TURNPortForwarder != nil && hole.IsActive {
			rules := hole.TURNPortForwarder.GetForwardRules()
			for ruleID, rule := range rules {
				allRules[ruleID] = rule
			}
		}
	}

	return allRules
}

// GetTURNActiveConnections 获取TURN活跃连接
func (nt *NATTraversal) GetTURNActiveConnections() map[string]*ConnectionInfo {
	nt.holesMutex.RLock()
	defer nt.holesMutex.RUnlock()

	allConnections := make(map[string]*ConnectionInfo)

	for _, hole := range nt.holes {
		if hole.TURNPortForwarder != nil && hole.IsActive {
			connections := hole.TURNPortForwarder.GetActiveConnections()
			for connID, conn := range connections {
				allConnections[connID] = conn
			}
		}
	}

	return allConnections
}

// GetTURNForwardingStatistics 获取TURN转发统计信息
func (nt *NATTraversal) GetTURNForwardingStatistics() map[string]interface{} {
	nt.holesMutex.RLock()
	defer nt.holesMutex.RUnlock()

	var totalRules, totalConnections, totalBytesReceived, totalBytesSent int
	var totalAllocatedPorts, activeTurnPorts int

	for _, hole := range nt.holes {
		if hole.TURNPortForwarder != nil && hole.IsActive {
			stats := hole.TURNPortForwarder.GetStatistics()

			if rules, ok := stats["total_rules"].(int); ok {
				totalRules += rules
			}
			if connections, ok := stats["active_connections"].(int64); ok {
				totalConnections += int(connections)
			}
			if bytesReceived, ok := stats["total_bytes_received"].(int64); ok {
				totalBytesReceived += int(bytesReceived)
			}
			if bytesSent, ok := stats["total_bytes_sent"].(int64); ok {
				totalBytesSent += int(bytesSent)
			}
			if allocatedPorts, ok := stats["total_allocated_ports"].(int); ok {
				totalAllocatedPorts += allocatedPorts
			}
			if turnPorts, ok := stats["active_turn_ports"].(int); ok {
				activeTurnPorts += turnPorts
			}
		}
	}

	if totalRules == 0 {
		return map[string]interface{}{
			"available": false,
			"message":   "没有活跃的TURN转发器",
		}
	}

	return map[string]interface{}{
		"available":             true,
		"total_rules":           totalRules,
		"active_connections":    totalConnections,
		"total_bytes_received":  totalBytesReceived,
		"total_bytes_sent":      totalBytesSent,
		"total_allocated_ports": totalAllocatedPorts,
		"active_turn_ports":     activeTurnPorts,
	}
}

// IsTURNForwardingAvailable 检查TURN端口转发是否可用
func (nt *NATTraversal) IsTURNForwardingAvailable() bool {
	nt.holesMutex.RLock()
	defer nt.holesMutex.RUnlock()

	for _, hole := range nt.holes {
		if hole.TURNPortForwarder != nil && hole.TURNClient != nil && hole.IsActive {
			return true
		}
	}
	return false
}

// GetTURNForwardingStatus 获取TURN端口转发状态
func (nt *NATTraversal) GetTURNForwardingStatus() map[string]interface{} {
	status := map[string]interface{}{
		"available": nt.IsTURNForwardingAvailable(),
	}

	if nt.IsTURNForwardingAvailable() {
		status["turn_status"] = nt.GetTURNStatus()
		status["forwarding_stats"] = nt.GetTURNForwardingStatistics()
		status["forward_rules"] = nt.GetTURNForwardRules()
		status["active_connections"] = nt.GetTURNActiveConnections()
	} else {
		status["message"] = "TURN端口转发功能未启用"
	}

	return status
}

// ==================== 数据流转统计相关方法 ====================

// GetDataFlowStatistics 获取数据流转统计信息
func (nt *NATTraversal) GetDataFlowStatistics() map[string]interface{} {
	nt.holesMutex.RLock()
	defer nt.holesMutex.RUnlock()

	var totalBytesReceived, totalBytesSent, totalConnections int64
	var activeHoles int

	for _, hole := range nt.holes {
		if hole.IsActive {
			activeHoles++
			totalBytesReceived += hole.BytesReceived
			totalBytesSent += hole.BytesSent
			totalConnections += hole.Connections
		}
	}

	return map[string]interface{}{
		"active_holes":         activeHoles,
		"total_bytes_received": totalBytesReceived,
		"total_bytes_sent":     totalBytesSent,
		"total_connections":    totalConnections,
		"holes":                nt.getHolesStatistics(),
	}
}

// getHolesStatistics 获取所有打洞的统计信息
func (nt *NATTraversal) getHolesStatistics() map[string]interface{} {
	stats := make(map[string]interface{})

	for holeKey, hole := range nt.holes {
		stats[holeKey] = map[string]interface{}{
			"local_port":     hole.LocalPort,
			"target_port":    hole.TargetPort,
			"protocol":       hole.Protocol,
			"description":    hole.Description,
			"is_active":      hole.IsActive,
			"created_at":     hole.CreatedAt,
			"last_activity":  hole.LastActivity,
			"forward_rule":   hole.ForwardRuleID,
			"bytes_received": hole.BytesReceived,
			"bytes_sent":     hole.BytesSent,
			"connections":    hole.Connections,
		}
	}

	return stats
}

// GetHoleDataFlowStatistics 获取特定打洞的数据流转统计
func (nt *NATTraversal) GetHoleDataFlowStatistics(allocatedPort int, sourcePort int, protocol string) map[string]interface{} {
	holeKey := fmt.Sprintf("%d-%d-%s", allocatedPort, sourcePort, protocol)

	nt.holesMutex.RLock()
	defer nt.holesMutex.RUnlock()

	hole, exists := nt.holes[holeKey]
	if !exists {
		return map[string]interface{}{
			"error": "打洞不存在",
		}
	}

	return map[string]interface{}{
		"hole_key":       holeKey,
		"local_port":     hole.LocalPort,
		"target_port":    hole.TargetPort,
		"protocol":       hole.Protocol,
		"description":    hole.Description,
		"is_active":      hole.IsActive,
		"created_at":     hole.CreatedAt,
		"last_activity":  hole.LastActivity,
		"forward_rule":   hole.ForwardRuleID,
		"bytes_received": hole.BytesReceived,
		"bytes_sent":     hole.BytesSent,
		"connections":    hole.Connections,
	}
}

// ResetHoleStatistics 重置特定打洞的统计信息
func (nt *NATTraversal) ResetHoleStatistics(allocatedPort int, sourcePort int, protocol string) error {
	holeKey := fmt.Sprintf("%d-%d-%s", allocatedPort, sourcePort, protocol)

	nt.holesMutex.Lock()
	defer nt.holesMutex.Unlock()

	hole, exists := nt.holes[holeKey]
	if !exists {
		return fmt.Errorf("打洞不存在: %s", holeKey)
	}

	hole.BytesReceived = 0
	hole.BytesSent = 0
	hole.Connections = 0

	nt.logger.WithField("hole_key", holeKey).Info("重置打洞统计信息")
	return nil
}

// GetOverallStatus 获取整体状态信息
func (nt *NATTraversal) GetOverallStatus() map[string]interface{} {
	return map[string]interface{}{
		"nat_traversal": map[string]interface{}{
			"enabled":         nt.config.Enabled,
			"use_turn":        nt.config.UseTURN,
			"holes":           nt.GetHoles(),
			"data_flow_stats": nt.GetDataFlowStatistics(),
		},
		"turn_forwarding":      nt.GetTURNForwardingStatus(),
		"hole_forward_mapping": nt.GetHoleForwardMapping(),
	}
}

// GetHoleForwardMapping 获取打洞和转发规则的映射关系
func (nt *NATTraversal) GetHoleForwardMapping() map[string]interface{} {
	nt.holesMutex.RLock()
	defer nt.holesMutex.RUnlock()

	mapping := make(map[string]interface{})

	for holeKey, hole := range nt.holes {
		if hole.ForwardRuleID != "" {
			mapping[holeKey] = map[string]interface{}{
				"forward_rule_id": hole.ForwardRuleID,
				"local_port":      hole.LocalPort,
				"target_port":     hole.TargetPort,
				"protocol":        hole.Protocol,
				"description":     hole.Description,
				"is_active":       hole.IsActive,
			}
		}
	}

	return mapping
}

// GetHoleByForwardRule 根据转发规则ID获取对应的打洞信息
func (nt *NATTraversal) GetHoleByForwardRule(forwardRuleID string) *HoleInfo {
	nt.holesMutex.RLock()
	defer nt.holesMutex.RUnlock()

	for _, hole := range nt.holes {
		if hole.ForwardRuleID == forwardRuleID {
			return hole
		}
	}

	return nil
}

// CloseHoleByForwardRule 根据转发规则ID关闭打洞
func (nt *NATTraversal) CloseHoleByForwardRule(forwardRuleID string) error {
	hole := nt.GetHoleByForwardRule(forwardRuleID)
	if hole == nil {
		return fmt.Errorf("未找到转发规则对应的打洞: %s", forwardRuleID)
	}

	return nt.CloseHole(hole.LocalPort, hole.TargetPort, hole.Protocol)
}
