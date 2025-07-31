package portmapping

import (
	"time"
)

// PortMappingProvider 端口映射提供者接口
type PortMappingProvider interface {
	// Type 返回提供者类型
	Type() MappingType

	// Name 返回提供者名称
	Name() string

	// IsAvailable 检查是否可用
	IsAvailable() bool

	// CreateMapping 创建端口映射
	CreateMapping(port int, externalPort int, protocol, description string, addType MappingAddType) (*PortMapping, error)

	// RemoveMapping 移除端口映射
	RemoveMapping(port int, externalPort int, protocol string, addType MappingAddType) error

	// GetMappings 获取所有映射
	GetMappings() map[string]*PortMapping

	// GetStatus 获取提供者状态
	GetStatus() map[string]interface{}

	// Start 启动提供者
	Start(checkStatusTaskTime time.Duration) error

	// Stop 停止提供者
	Stop() error
}
