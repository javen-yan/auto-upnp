package nathole

import (
	"auto-upnp/internal/types"
	"fmt"

	"github.com/sirupsen/logrus"
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

// CreateNATHoleProvider 根据NAT类型创建相应的NAT穿透提供者
func CreateNATHoleProvider(natType types.NATType, logger *logrus.Logger, config map[string]interface{}) (NATHoleProvider, error) {
	switch natType {
	case types.NATType1:
		return NewNAT1Provider(logger, config), nil
	case types.NATType2:
		return NewNAT2Provider(logger, config), nil
	default:
		return nil, fmt.Errorf("暂不支持的NAT类型: %s", natType)
	}
}
