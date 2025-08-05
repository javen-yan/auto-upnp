package portmapping

import (
	"context"
	"fmt"
	"sync"
	"time"

	"auto-upnp/internal/nathole"
	"auto-upnp/internal/types"
	"auto-upnp/internal/util"

	"github.com/sirupsen/logrus"
)

// NATProvider NAT端口映射提供者
type NATProvider struct {
	logger          *logrus.Logger
	ctx             context.Context
	cancel          context.CancelFunc
	mappings        map[string]*PortMapping
	mutex           sync.RWMutex
	available       bool
	natInfo         *types.NATInfo
	natHolePunching *nathole.NATHolePunching
	stunServers     []string
}

// NewNATProvider 创建新的NAT提供者
func NewNATProvider(logger *logrus.Logger, config map[string]interface{}) *NATProvider {
	ctx, cancel := context.WithCancel(context.Background())

	provider := &NATProvider{
		logger:    logger,
		ctx:       ctx,
		cancel:    cancel,
		mappings:  make(map[string]*PortMapping),
		available: false,
	}

	// 从配置中读取NAT服务器列表
	if stunServers, ok := config["stun_servers"].([]string); ok {
		provider.stunServers = stunServers
	}

	// 从配置中读取系统NAT信息
	if systemNatInfo, ok := config["system_natinfo"].(*types.NATInfo); ok {
		provider.natInfo = systemNatInfo
	}

	provider.logger.WithFields(logrus.Fields{
		"stun_servers_count": len(provider.stunServers),
		"stun_servers":       provider.stunServers,
	}).Info("STUN服务器列表")

	return provider
}

// Type 返回提供者类型
func (tp *NATProvider) Type() types.MappingType {
	return types.MappingTypeNAT
}

// Name 返回提供者名称
func (tp *NATProvider) Name() string {
	return "NAT端口映射"
}

// IsAvailable 检查是否可用
func (tp *NATProvider) IsAvailable() bool {
	return tp.available && tp.natHolePunching != nil
}

// Start 启动NAT提供者
func (tp *NATProvider) Start(checkStatusTaskTime time.Duration) error {
	tp.logger.Info("启动NAT端口映射提供者")

	// 创建NAT穿透管理器
	config := make(map[string]interface{})
	config["stun_servers"] = tp.stunServers
	config["nat_info"] = tp.natInfo
	tp.natHolePunching = nathole.NewNATHolePunching(tp.logger, tp.natInfo, config)

	// 启动NAT穿透服务
	if err := tp.natHolePunching.Start(); err != nil {
		tp.logger.WithError(err).Warn("NAT穿透服务启动失败")
		tp.available = false
		return fmt.Errorf("NAT穿透服务启动失败: %w", err)
	}
	tp.available = true

	// 启动检查端口状态任务
	go tp.checkStatusTask(checkStatusTaskTime)

	return nil
}

// Stop 停止NAT提供者
func (tp *NATProvider) Stop() error {
	tp.logger.Info("停止NAT端口映射提供者")
	tp.cancel()

	if tp.natHolePunching != nil {
		tp.natHolePunching.Stop()
	}

	tp.available = false
	tp.natInfo = nil
	return nil
}

// CreateMapping 创建NAT端口映射
func (tp *NATProvider) CreateMapping(port int, externalPort int, protocol, description string, addType types.MappingAddType) (*PortMapping, error) {
	if !tp.IsAvailable() {
		return nil, fmt.Errorf("NAT提供者不可用")
	}

	// 使用分配的外部端口创建映射键
	mappingKey := fmt.Sprintf("%d:%d:%s", port, externalPort, protocol)

	tp.mutex.Lock()
	defer tp.mutex.Unlock()

	// 为这个端口创建独立的NAT打洞
	natHole, err := tp.createNATHole(port, protocol)
	if err != nil {
		tp.logger.WithFields(logrus.Fields{
			"port":          port,
			"external_port": externalPort,
			"protocol":      protocol,
			"error":         err,
		}).Error("NAT打洞创建失败")
		return nil, fmt.Errorf("NAT打洞创建失败: %w", err)
	}

	// 创建端口映射记录
	mapping := &PortMapping{
		InternalPort: port,
		ExternalPort: natHole.ExternalPort, // 使用分配的外部端口
		Protocol:     protocol,
		Description:  description,
		AddType:      addType,
		Type:         types.MappingTypeNAT,
		Status:       types.MappingStatusActive,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		ExternalAddr: natHole.ExternalAddr, // 使用独立的外部地址
	}

	tp.mappings[mappingKey] = mapping

	// 安全地获取外部地址字符串
	externalAddrStr := "unknown"
	if natHole.ExternalAddr != nil {
		externalAddrStr = natHole.ExternalAddr.String()
	}

	tp.logger.WithFields(logrus.Fields{
		"port":          port,
		"external_port": externalPort,
		"protocol":      protocol,
		"type":          types.MappingTypeNAT,
		"external_addr": externalAddrStr,
		"mapping_key":   mappingKey,
	}).Info("NAT端口映射创建成功")

	return mapping, nil
}

// RemoveMapping 移除NAT端口映射
func (tp *NATProvider) RemoveMapping(port int, externalPort int, protocol string, addType types.MappingAddType) error {

	tp.mutex.Lock()
	defer tp.mutex.Unlock()

	var matchedMapping *PortMapping

	for _, mapping := range tp.mappings {
		if mapping.InternalPort == port && mapping.Protocol == protocol {
			matchedMapping = mapping
			break
		}
	}

	if matchedMapping == nil {
		return fmt.Errorf("端口映射不存在: %d:%d:%s", port, externalPort, protocol)
	}

	mappingKey := fmt.Sprintf("%d:%d:%s", port, matchedMapping.ExternalPort, protocol)
	_, exists := tp.mappings[mappingKey]
	if !exists {
		return fmt.Errorf("端口映射不存在: %s", mappingKey)
	}

	// 使用NAT穿透关闭打洞
	err := tp.natHolePunching.RemoveHole(port, matchedMapping.ExternalPort, protocol)
	if err != nil {
		tp.logger.WithFields(logrus.Fields{
			"port":          port,
			"external_port": matchedMapping.ExternalPort,
			"protocol":      protocol,
			"error":         err,
		}).Error("NAT端口映射移除失败")
		return fmt.Errorf("NAT端口映射移除失败: %w", err)
	}

	delete(tp.mappings, mappingKey)

	tp.logger.WithFields(logrus.Fields{
		"port":          port,
		"external_port": matchedMapping.ExternalPort,
		"protocol":      protocol,
		"type":          types.MappingTypeNAT,
	}).Info("NAT端口映射移除成功")

	return nil
}

// createNATHole 为指定端口创建独立的NAT打洞
func (tp *NATProvider) createNATHole(port int, protocol string) (*nathole.NATHole, error) {
	// 使用NAT客户端为这个端口创建独立的打洞
	if tp.natHolePunching != nil {
		// 创建打洞记录，在外部端口上监听，转发到本地端口
		externalAddr, err := tp.natHolePunching.CreateHole(port, port, protocol, fmt.Sprintf("NAT-%d", port))
		if err != nil {
			return nil, fmt.Errorf("创建NAT打洞失败: %w", err)
		}
		return externalAddr, nil
	}

	return nil, fmt.Errorf("NAT客户端不可用")
}

// GetMappings 获取所有NAT映射
func (tp *NATProvider) GetMappings() map[string]*PortMapping {
	tp.mutex.RLock()
	defer tp.mutex.RUnlock()

	result := make(map[string]*PortMapping)
	for key, mapping := range tp.mappings {
		result[key] = mapping
	}
	return result
}

// GetStatus 获取NAT提供者状态
func (tp *NATProvider) GetStatus() map[string]interface{} {
	tp.mutex.RLock()
	defer tp.mutex.RUnlock()

	activeCount := 0
	for _, mapping := range tp.mappings {
		if mapping.Status == types.MappingStatusActive {
			activeCount++
		}
	}

	status := map[string]interface{}{
		"available":       tp.IsAvailable(),
		"total_mappings":  len(tp.mappings),
		"active_mappings": activeCount,
		"stun_servers":    tp.stunServers,
	}

	if tp.natInfo != nil {
		status["external_address"] = map[string]interface{}{
			"ip":   tp.natInfo.PublicIP.String(),
			"port": tp.natInfo.PublicPort,
		}
	}

	// 如果NAT穿透管理器可用，添加其状态信息
	if tp.natHolePunching != nil {
		holes := tp.natHolePunching.GetHoles()
		activeHoles := tp.natHolePunching.GetActiveHoles()

		status["nat_traversal"] = map[string]interface{}{
			"total_holes":   len(holes),
			"active_holes":  len(activeHoles),
			"external_addr": tp.natHolePunching.GetExternalAddress(),
		}
	}

	return status
}

func (tp *NATProvider) checkStatusTask(tickerTime time.Duration) {
	tp.logger.Info("检查端口状态任务启动")

	if tickerTime == 0 {
		tickerTime = 5 * time.Second
	}

	ticker := time.NewTicker(tickerTime)
	defer ticker.Stop()

	for {
		select {
		case <-tp.ctx.Done():
			tp.logger.Info("检查端口状态任务停止")
			return
		case <-ticker.C:
			tp.checkPortStatus()
		}
	}
}

func (tp *NATProvider) checkPortStatus() {
	tp.mutex.RLock()
	allManualMappings := make([]*PortMapping, 0)
	for _, mapping := range tp.mappings {
		if mapping.AddType == types.MappingAddTypeManual {
			allManualMappings = append(allManualMappings, mapping)
		}
	}
	tp.mutex.RUnlock()

	for _, mapping := range allManualMappings {
		oldMapStatus := mapping.Status
		portStatus := util.IsPortActive(mapping.InternalPort)
		newMapStatus := types.MappingStatusInactive
		if portStatus.Open {
			newMapStatus = types.MappingStatusActive
		}
		if oldMapStatus != newMapStatus {
			tp.updateMappingStatus(mapping, newMapStatus)
		}
	}
}

func (tp *NATProvider) updateMappingStatus(mapping *PortMapping, status types.MappingStatus) {
	tp.mutex.Lock()
	defer tp.mutex.Unlock()

	if mapping.Status == status {
		return
	}
	tp.logger.WithFields(logrus.Fields{
		"port":          mapping.InternalPort,
		"external_port": mapping.ExternalPort,
		"protocol":      mapping.Protocol,
	}).Info("端口状态发生变化")
	mapping.Status = status
}
