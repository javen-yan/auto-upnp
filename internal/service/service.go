package service

import (
	"auto-upnp/internal/portmapping"
)

// Service 服务接口
type Service interface {
	// Start 启动服务
	Start() error

	// Stop 停止服务
	Stop()

	// GetStatus 获取服务状态
	GetStatus() map[string]interface{}

	// AddManualMapping 添加手动映射
	AddManualMapping(internalPort, externalPort int, protocol, description string) error

	// RemoveManualMapping 删除手动映射
	RemoveManualMapping(internalPort, externalPort int, protocol string) error

	// GetPortMappings 获取所有端口映射
	GetPortMappings(addType string) []portmapping.PortMapping

	// GetActivePorts 获取活跃端口列表
	GetActivePorts() []int
}

// ServiceMappingType 服务映射类型
type ServiceMappingType string

const (
	// AutoMapping 自动映射（仅内存存储）
	AutoMapping ServiceMappingType = "auto"

	// ManualMappingType 手动映射（内存+持久化）
	ManualMappingType ServiceMappingType = "manual"
)

// MappingInfo 映射信息
type MappingInfo struct {
	Type         ServiceMappingType `json:"type"`
	InternalPort int                `json:"internal_port"`
	ExternalPort int                `json:"external_port"`
	Protocol     string             `json:"protocol"`
	Description  string             `json:"description"`
	Active       bool               `json:"active"`
	Provider     string             `json:"provider,omitempty"`
}
