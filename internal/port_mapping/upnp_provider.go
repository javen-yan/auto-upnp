package port_mapping

import (
	"context"
	"fmt"
	"sync"
	"time"

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

	// UPnP相关配置
	discoveryTimeout time.Duration
	mappingDuration  time.Duration
	retryAttempts    int
	retryDelay       time.Duration
}

// NewUPnPProvider 创建新的UPnP提供者
func NewUPnPProvider(logger *logrus.Logger, config map[string]interface{}) *UPnPProvider {
	ctx, cancel := context.WithCancel(context.Background())

	provider := &UPnPProvider{
		logger:    logger,
		ctx:       ctx,
		cancel:    cancel,
		mappings:  make(map[string]*PortMapping),
		available: false,
	}

	// 从配置中读取参数
	if discoveryTimeout, ok := config["discovery_timeout"].(time.Duration); ok {
		provider.discoveryTimeout = discoveryTimeout
	} else {
		provider.discoveryTimeout = 10 * time.Second
	}

	if mappingDuration, ok := config["mapping_duration"].(time.Duration); ok {
		provider.mappingDuration = mappingDuration
	} else {
		provider.mappingDuration = 1 * time.Hour
	}

	if retryAttempts, ok := config["retry_attempts"].(int); ok {
		provider.retryAttempts = retryAttempts
	} else {
		provider.retryAttempts = 3
	}

	if retryDelay, ok := config["retry_delay"].(time.Duration); ok {
		provider.retryDelay = retryDelay
	} else {
		provider.retryDelay = 5 * time.Second
	}

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
	return up.available
}

// Start 启动UPnP提供者
func (up *UPnPProvider) Start() error {
	up.logger.Info("启动UPnP端口映射提供者")

	// 这里应该实现UPnP设备发现和连接逻辑
	// 由于这是一个示例，我们模拟UPnP可用性检查
	up.available = up.checkUPnPAvailability()

	if up.available {
		up.logger.Info("UPnP端口映射提供者启动成功")
	} else {
		up.logger.Warn("UPnP端口映射提供者不可用")
	}

	return nil
}

// Stop 停止UPnP提供者
func (up *UPnPProvider) Stop() {
	up.logger.Info("停止UPnP端口映射提供者")
	up.cancel()
	up.available = false
}

// CreateMapping 创建UPnP端口映射
func (up *UPnPProvider) CreateMapping(port int, protocol, description string) (*PortMapping, error) {
	if !up.available {
		return nil, fmt.Errorf("UPnP提供者不可用")
	}

	mappingKey := fmt.Sprintf("%d-%s", port, protocol)

	up.mutex.Lock()
	defer up.mutex.Unlock()

	// 检查是否已存在
	if _, exists := up.mappings[mappingKey]; exists {
		return nil, fmt.Errorf("端口映射已存在: %s", mappingKey)
	}

	// 创建端口映射
	mapping := &PortMapping{
		InternalPort: port,
		Protocol:     protocol,
		Description:  description,
		Type:         MappingTypeUPnP,
		Status:       MappingStatusActive,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
	}

	// 这里应该调用实际的UPnP API来创建端口映射
	// 由于这是一个示例，我们模拟成功
	up.mappings[mappingKey] = mapping

	up.logger.WithFields(logrus.Fields{
		"port":     port,
		"protocol": protocol,
		"type":     MappingTypeUPnP,
	}).Info("UPnP端口映射创建成功")

	return mapping, nil
}

// RemoveMapping 移除UPnP端口映射
func (up *UPnPProvider) RemoveMapping(port int, protocol string) error {
	mappingKey := fmt.Sprintf("%d-%s", port, protocol)

	up.mutex.Lock()
	defer up.mutex.Unlock()

	_, exists := up.mappings[mappingKey]
	if !exists {
		return fmt.Errorf("端口映射不存在: %s", mappingKey)
	}

	// 这里应该调用实际的UPnP API来移除端口映射
	// 由于这是一个示例，我们模拟成功
	delete(up.mappings, mappingKey)

	up.logger.WithFields(logrus.Fields{
		"port":     port,
		"protocol": protocol,
		"type":     MappingTypeUPnP,
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

	return map[string]interface{}{
		"available":         up.available,
		"total_mappings":    len(up.mappings),
		"active_mappings":   activeCount,
		"discovery_timeout": up.discoveryTimeout.String(),
		"mapping_duration":  up.mappingDuration.String(),
	}
}

// checkUPnPAvailability 检查UPnP可用性
func (up *UPnPProvider) checkUPnPAvailability() bool {
	// 这里应该实现实际的UPnP设备发现逻辑
	// 由于这是一个示例，我们返回false表示UPnP不可用
	// 在实际实现中，应该：
	// 1. 发送SSDP发现消息
	// 2. 查找IGD设备
	// 3. 测试端口映射功能

	up.logger.Info("检查UPnP可用性")
	return false // 模拟UPnP不可用
}
