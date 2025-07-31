package portmapping

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"auto-upnp/config"
	"auto-upnp/internal/upnp"
	"auto-upnp/internal/util"

	"github.com/sirupsen/logrus"
)

// UPnPProvider UPnP端口映射提供者
type UPnPProvider struct {
	logger    *logrus.Logger
	ctx       context.Context
	cancel    context.CancelFunc
	mappings  map[string]*PortMapping
	mutex     sync.RWMutex
	available bool

	// UPnP管理器
	upnpManager *upnp.UPnPManager

	config.UPnPConfig
}

// NewUPnPProvider 创建新的UPnP提供者
func NewUPnPProvider(logger *logrus.Logger, configMap map[string]interface{}) *UPnPProvider {
	ctx, cancel := context.WithCancel(context.Background())

	provider := &UPnPProvider{
		logger:    logger,
		ctx:       ctx,
		cancel:    cancel,
		mappings:  make(map[string]*PortMapping),
		available: false,
	}

	var cfg config.UPnPConfig

	body, _ := json.Marshal(configMap)
	err := json.Unmarshal(body, &cfg)
	if err != nil {
		provider.logger.WithError(err).Error("解析UPnP配置失败")
		return nil
	}
	provider.UPnPConfig = cfg
	return provider
}

// Type 返回提供者类型
func (up *UPnPProvider) Type() MappingType {
	return MappingTypeUPnP
}

// Name 返回提供者名称
func (up *UPnPProvider) Name() string {
	return "UPnP端口映射"
}

// IsAvailable 检查是否可用
func (up *UPnPProvider) IsAvailable() bool {
	return up.available && up.upnpManager != nil && up.upnpManager.IsUPnPAvailable()
}

// Start 启动UPnP提供者
func (up *UPnPProvider) Start(checkStatusTaskTime time.Duration) error {
	up.logger.Info("启动UPnP端口映射提供者")

	upnpConfig := upnp.Config{
		DiscoveryTimeout:    up.DiscoveryTimeout,
		MappingDuration:     up.MappingDuration,
		RetryAttempts:       up.RetryAttempts,
		RetryDelay:          up.RetryDelay,
		HealthCheckInterval: up.HealthCheckInterval,
		MaxFailCount:        up.MaxFailCount,
		KeepAliveInterval:   up.KeepAliveInterval,
	}

	// 创建UPnP管理器
	up.upnpManager = upnp.NewUPnPManager(&upnpConfig, up.logger)

	// 发现UPnP设备
	if err := up.upnpManager.Discover(); err != nil {
		up.logger.WithError(err).Warn("UPnP设备发现失败")
		up.available = false
		return fmt.Errorf("UPnP设备发现失败: %w", err)
	}

	up.available = true

	up.logger.WithFields(logrus.Fields{
		"client_count":    up.upnpManager.GetClientCount(),
		"healthy_clients": up.upnpManager.GetHealthyClientCount(),
	}).Info("UPnP端口映射提供者启动成功")

	// 启动检查端口状态任务
	go up.checkStatusTask(checkStatusTaskTime)

	return nil
}

// Stop 停止UPnP提供者
func (up *UPnPProvider) Stop() error {
	up.logger.Info("停止UPnP端口映射提供者")
	up.cancel()

	if up.upnpManager != nil {
		up.upnpManager.Close()
	}

	up.available = false
	return nil
}

// CreateMapping 创建UPnP端口映射
func (up *UPnPProvider) CreateMapping(port int, externalPort int, protocol, description string, addType MappingAddType) (*PortMapping, error) {
	if !up.IsAvailable() {
		return nil, fmt.Errorf("UPnP提供者不可用")
	}

	mappingKey := fmt.Sprintf("%d:%d:%s", port, externalPort, protocol)

	up.mutex.Lock()
	defer up.mutex.Unlock()

	// 检查是否已存在
	if _, exists := up.mappings[mappingKey]; exists {
		return nil, fmt.Errorf("端口映射已存在: %s", mappingKey)
	}

	// 使用UPnP管理器添加端口映射
	err := up.upnpManager.AddPortMapping(port, externalPort, protocol, description)
	if err != nil {
		up.logger.WithFields(logrus.Fields{
			"port":          port,
			"external_port": externalPort,
			"protocol":      protocol,
			"error":         err,
		}).Error("UPnP端口映射创建失败")
		return nil, fmt.Errorf("UPnP端口映射创建失败: %w", err)
	}

	// 创建端口映射记录
	mapping := &PortMapping{
		InternalPort: port,
		ExternalPort: externalPort,
		Protocol:     protocol,
		Description:  description,
		AddType:      addType,
		Type:         MappingTypeUPnP,
		Status:       MappingStatusActive,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
	}

	up.mappings[mappingKey] = mapping

	up.logger.WithFields(logrus.Fields{
		"port":          port,
		"external_port": externalPort,
		"protocol":      protocol,
		"type":          MappingTypeUPnP,
	}).Info("UPnP端口映射创建成功")

	return mapping, nil
}

// RemoveMapping 移除UPnP端口映射
func (up *UPnPProvider) RemoveMapping(port int, externalPort int, protocol string, addType MappingAddType) error {
	mappingKey := fmt.Sprintf("%d:%d:%s", port, externalPort, protocol)

	up.mutex.Lock()
	defer up.mutex.Unlock()

	_, exists := up.mappings[mappingKey]
	if !exists {
		return fmt.Errorf("端口映射不存在: %s", mappingKey)
	}

	// 使用UPnP管理器删除端口映射
	err := up.upnpManager.RemovePortMapping(port, externalPort, protocol)
	if err != nil {
		up.logger.WithFields(logrus.Fields{
			"port":          port,
			"external_port": externalPort,
			"protocol":      protocol,
			"error":         err,
		}).Error("UPnP端口映射移除失败")
		return fmt.Errorf("UPnP端口映射移除失败: %w", err)
	}

	delete(up.mappings, mappingKey)

	up.logger.WithFields(logrus.Fields{
		"port":          port,
		"external_port": externalPort,
		"protocol":      protocol,
		"type":          MappingTypeUPnP,
	}).Info("UPnP端口映射移除成功")

	return nil
}

// GetMappings 获取所有UPnP映射
func (up *UPnPProvider) GetMappings() map[string]*PortMapping {
	up.mutex.RLock()
	defer up.mutex.RUnlock()

	result := make(map[string]*PortMapping)
	for key, mapping := range up.mappings {
		result[key] = mapping
	}
	return result
}

// GetStatus 获取UPnP提供者状态
func (up *UPnPProvider) GetStatus() map[string]interface{} {
	up.mutex.RLock()
	defer up.mutex.RUnlock()

	activeCount := 0
	for _, mapping := range up.mappings {
		if mapping.Status == MappingStatusActive {
			activeCount++
		}
	}

	status := map[string]interface{}{
		"available":         up.IsAvailable(),
		"total_mappings":    len(up.mappings),
		"active_mappings":   activeCount,
		"discovery_timeout": up.DiscoveryTimeout.String(),
		"mapping_duration":  up.MappingDuration.String(),
		"retry_attempts":    up.RetryAttempts,
		"retry_delay":       up.RetryDelay.String(),
	}

	// 如果UPnP管理器可用，添加其状态信息
	if up.upnpManager != nil {
		status["upnp_manager"] = map[string]interface{}{
			"client_count":         up.upnpManager.GetClientCount(),
			"healthy_client_count": up.upnpManager.GetHealthyClientCount(),
			"is_available":         up.upnpManager.IsUPnPAvailable(),
			"client_status":        up.upnpManager.GetClientStatus(),
		}
	}

	return status
}

func (sp *UPnPProvider) checkStatusTask(tickerTime time.Duration) {
	sp.logger.Info("检查端口状态任务启动")

	if tickerTime == 0 {
		tickerTime = 5 * time.Second
	}

	ticker := time.NewTicker(tickerTime)
	defer ticker.Stop()

	for {
		select {
		case <-sp.ctx.Done():
			sp.logger.Info("检查端口状态任务停止")
			return
		case <-ticker.C:
			sp.checkPortStatus()
		}
	}
}

func (sp *UPnPProvider) checkPortStatus() {
	sp.mutex.RLock()
	allManualMappings := make([]*PortMapping, 0)
	for _, mapping := range sp.mappings {
		if mapping.AddType == MappingAddTypeManual {
			allManualMappings = append(allManualMappings, mapping)
		}
	}
	sp.mutex.RUnlock()

	for _, mapping := range allManualMappings {
		oldMapStatus := mapping.Status
		portStatus := util.IsPortActive(mapping.InternalPort)
		newMapStatus := MappingStatusInactive
		if portStatus.Open {
			newMapStatus = MappingStatusActive
		}
		if oldMapStatus != newMapStatus {
			sp.updateMappingStatus(mapping, newMapStatus)
		}
	}
}

func (sp *UPnPProvider) updateMappingStatus(mapping *PortMapping, status MappingStatus) {
	sp.mutex.Lock()
	defer sp.mutex.Unlock()

	if mapping.Status == status {
		return
	}
	sp.logger.WithFields(logrus.Fields{
		"port":          mapping.InternalPort,
		"external_port": mapping.ExternalPort,
		"protocol":      mapping.Protocol,
	}).Info("端口状态发生变化")
	mapping.Status = status
}
