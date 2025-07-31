package nathole

import (
	"auto-upnp/internal/types"
	"net"
	"time"
)

// NATHoleProvider NAT穿透提供者接口
type NATHoleProvider interface {
	// Type 返回NAT类型
	Type() types.NATType

	// Name 返回提供者名称
	Name() string

	// IsAvailable 检查是否可用
	IsAvailable() bool

	// CreateHole 创建NAT穿透
	CreateHole(localPort int, externalPort int, protocol string, description string) (*NATHole, error)

	// RemoveHole 移除NAT穿透
	RemoveHole(localPort int, externalPort int, protocol string) error

	// GetHoles 获取所有穿透
	GetHoles() map[string]*NATHole

	// GetStatus 获取提供者状态
	GetStatus() map[string]interface{}

	// Start 启动提供者
	Start() error

	// Stop 停止提供者
	Stop() error
}

// NATHole NAT穿透信息
type NATHole struct {
	LocalPort    int           `json:"local_port"`
	ExternalPort int           `json:"external_port"`
	Protocol     string        `json:"protocol"`
	Description  string        `json:"description"`
	Type         types.NATType `json:"type"`
	Status       HoleStatus    `json:"status"`
	CreatedAt    time.Time     `json:"created_at"`
	LastActivity time.Time     `json:"last_activity"`
	ExternalAddr net.Addr      `json:"external_addr,omitempty"`
	Error        string        `json:"error,omitempty"`
}

// HoleStatus 穿透状态
type HoleStatus string

const (
	HoleStatusActive   HoleStatus = "active"
	HoleStatusInactive HoleStatus = "inactive"
	HoleStatusFailed   HoleStatus = "failed"
)

// HoleType 穿透类型
type HoleType string

const (
	HoleTypeAuto   HoleType = "auto"
	HoleTypeManual HoleType = "manual"
)
