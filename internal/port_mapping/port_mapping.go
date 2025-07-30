package port_mapping

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/sirupsen/logrus"
)

// MappingType 映射类型
type MappingType string

const (
	MappingTypeUPnP MappingType = "upnp"
	MappingTypeSTUN MappingType = "stun"
)

// MappingStatus 映射状态
type MappingStatus string

const (
	MappingStatusActive   MappingStatus = "active"
	MappingStatusInactive MappingStatus = "inactive"
	MappingStatusFailed   MappingStatus = "failed"
)

// PortMapping 端口映射信息
type PortMapping struct {
	InternalPort int           `json:"internal_port"`
	Protocol     string        `json:"protocol"`
	Description  string        `json:"description"`
	Type         MappingType   `json:"type"`
	Status       MappingStatus `json:"status"`
	CreatedAt    time.Time     `json:"created_at"`
	LastActivity time.Time     `json:"last_activity"`
	ExternalAddr net.Addr      `json:"external_addr,omitempty"`
	Error        string        `json:"error,omitempty"`
}

// PortMappingProvider 端口映射提供者接口
type PortMappingProvider interface {
	// Type 返回提供者类型
	Type() MappingType

	// Name 返回提供者名称
	Name() string

	// IsAvailable 检查是否可用
	IsAvailable() bool

	// CreateMapping 创建端口映射
	CreateMapping(port int, protocol, description string) (*PortMapping, error)

	// RemoveMapping 移除端口映射
	RemoveMapping(port int, protocol string) error

	// GetMappings 获取所有映射
	GetMappings() map[string]*PortMapping

	// GetStatus 获取提供者状态
	GetStatus() map[string]interface{}

	// Start 启动提供者
	Start() error

	// Stop 停止提供者
	Stop() error
}

// PortMappingManager 端口映射管理器
type PortMappingManager struct {
	providers []PortMappingProvider
	logger    *logrus.Logger
	ctx       context.Context
	cancel    context.CancelFunc

	// 回调函数
	onMappingCreated func(port int, protocol string, providerType MappingType)
	onMappingRemoved func(port int, protocol string, providerType MappingType)
	onMappingFailed  func(port int, protocol string, providerType MappingType, error error)
}

// NewPortMappingManager 创建新的端口映射管理器
func NewPortMappingManager(logger *logrus.Logger) *PortMappingManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &PortMappingManager{
		providers: make([]PortMappingProvider, 0),
		logger:    logger,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// AddProvider 添加端口映射提供者
func (pm *PortMappingManager) AddProvider(provider PortMappingProvider) {
	pm.providers = append(pm.providers, provider)
	pm.logger.WithFields(logrus.Fields{
		"type": provider.Type(),
		"name": provider.Name(),
	}).Info("添加端口映射提供者")
}

// Start 启动所有提供者
func (pm *PortMappingManager) Start() error {
	pm.logger.Info("启动端口映射管理器")

	// 按优先级启动提供者：UPnP优先，STUN备用
	var availableProviders []PortMappingProvider

	for _, provider := range pm.providers {
		if provider.IsAvailable() {
			if err := provider.Start(); err != nil {
				pm.logger.WithFields(logrus.Fields{
					"type":  provider.Type(),
					"name":  provider.Name(),
					"error": err,
				}).Warn("启动端口映射提供者失败")
				continue
			}

			availableProviders = append(availableProviders, provider)
			pm.logger.WithFields(logrus.Fields{
				"type": provider.Type(),
				"name": provider.Name(),
			}).Info("端口映射提供者启动成功")
		} else {
			pm.logger.WithFields(logrus.Fields{
				"type": provider.Type(),
				"name": provider.Name(),
			}).Info("端口映射提供者不可用")
		}
	}

	if len(availableProviders) == 0 {
		return fmt.Errorf("没有可用的端口映射提供者")
	}

	pm.logger.WithField("available_providers", len(availableProviders)).Info("端口映射管理器启动成功")
	return nil
}

// Stop 停止所有提供者
func (pm *PortMappingManager) Stop() {
	pm.logger.Info("停止端口映射管理器")
	pm.cancel()

	for _, provider := range pm.providers {
		provider.Stop()
		pm.logger.WithFields(logrus.Fields{
			"type": provider.Type(),
			"name": provider.Name(),
		}).Info("端口映射提供者已停止")
	}
}

// CreateMapping 创建端口映射（自动选择最佳提供者）
func (pm *PortMappingManager) CreateMapping(port int, protocol, description string) (*PortMapping, error) {
	// 优先尝试UPnP
	for _, provider := range pm.providers {
		if provider.Type() == MappingTypeUPnP && provider.IsAvailable() {
			mapping, err := provider.CreateMapping(port, protocol, description)
			if err == nil {
				pm.logger.WithFields(logrus.Fields{
					"port":     port,
					"protocol": protocol,
					"type":     provider.Type(),
				}).Info("使用UPnP创建端口映射成功")

				if pm.onMappingCreated != nil {
					pm.onMappingCreated(port, protocol, provider.Type())
				}
				return mapping, nil
			}

			pm.logger.WithFields(logrus.Fields{
				"port":     port,
				"protocol": protocol,
				"type":     provider.Type(),
				"error":    err,
			}).Warn("UPnP创建端口映射失败，尝试STUN")
		}
	}

	// 如果UPnP失败，尝试STUN
	for _, provider := range pm.providers {
		if provider.Type() == MappingTypeSTUN && provider.IsAvailable() {
			mapping, err := provider.CreateMapping(port, protocol, description)
			if err == nil {
				pm.logger.WithFields(logrus.Fields{
					"port":     port,
					"protocol": protocol,
					"type":     provider.Type(),
				}).Info("使用STUN创建端口映射成功")

				if pm.onMappingCreated != nil {
					pm.onMappingCreated(port, protocol, provider.Type())
				}
				return mapping, nil
			}

			pm.logger.WithFields(logrus.Fields{
				"port":     port,
				"protocol": protocol,
				"type":     provider.Type(),
				"error":    err,
			}).Error("STUN创建端口映射失败")
		}
	}

	return nil, fmt.Errorf("所有端口映射提供者都失败")
}

// RemoveMapping 移除端口映射
func (pm *PortMappingManager) RemoveMapping(port int, protocol string) error {
	// 尝试从所有提供者中移除
	var lastError error

	for _, provider := range pm.providers {
		if err := provider.RemoveMapping(port, protocol); err != nil {
			lastError = err
			pm.logger.WithFields(logrus.Fields{
				"port":     port,
				"protocol": protocol,
				"type":     provider.Type(),
				"error":    err,
			}).Warn("从提供者移除端口映射失败")
		} else {
			pm.logger.WithFields(logrus.Fields{
				"port":     port,
				"protocol": protocol,
				"type":     provider.Type(),
			}).Info("从提供者移除端口映射成功")

			if pm.onMappingRemoved != nil {
				pm.onMappingRemoved(port, protocol, provider.Type())
			}
		}
	}

	return lastError
}

// GetMappings 获取所有映射
func (pm *PortMappingManager) GetMappings() map[string]*PortMapping {
	allMappings := make(map[string]*PortMapping)

	for _, provider := range pm.providers {
		mappings := provider.GetMappings()
		for key, mapping := range mappings {
			allMappings[key] = mapping
		}
	}

	return allMappings
}

// GetStatus 获取所有提供者状态
func (pm *PortMappingManager) GetStatus() map[string]interface{} {
	status := make(map[string]interface{})

	for _, provider := range pm.providers {
		providerStatus := provider.GetStatus()
		providerStatus["type"] = provider.Type()
		providerStatus["name"] = provider.Name()
		providerStatus["available"] = provider.IsAvailable()

		status[string(provider.Type())] = providerStatus
	}

	return status
}

// SetCallbacks 设置回调函数
func (pm *PortMappingManager) SetCallbacks(
	onMappingCreated func(port int, protocol string, providerType MappingType),
	onMappingRemoved func(port int, protocol string, providerType MappingType),
	onMappingFailed func(port int, protocol string, providerType MappingType, error error),
) {
	pm.onMappingCreated = onMappingCreated
	pm.onMappingRemoved = onMappingRemoved
	pm.onMappingFailed = onMappingFailed
}
