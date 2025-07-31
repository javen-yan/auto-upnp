package nathole

import (
	"fmt"

	"auto-upnp/internal/types"

	"github.com/sirupsen/logrus"
)

// NATHolePunching NAT穿透管理器
type NATHolePunching struct {
	logger   *logrus.Logger
	natInfo  *types.NATInfo
	provider NATHoleProvider

	// 回调函数
	onHoleCreated func(localPort int, externalPort int, protocol string, natType types.NATType)
	onHoleRemoved func(localPort int, externalPort int, protocol string, natType types.NATType)
	onHoleFailed  func(localPort int, externalPort int, protocol string, natType types.NATType, error error)
}

// NewNATHolePunching 创建新的NAT穿透管理器
func NewNATHolePunching(logger *logrus.Logger, natInfo *types.NATInfo) *NATHolePunching {
	provider, err := CreateNATHoleProvider(natInfo.Type, logger, nil)
	if err != nil {
		logger.WithField("nat_type", natInfo.Type).Error("创建NAT穿透提供者失败")
		return nil
	}

	return &NATHolePunching{
		logger:   logger,
		natInfo:  natInfo,
		provider: provider,
	}
}

// Start 启动NAT穿透管理器
func (n *NATHolePunching) Start() error {
	n.logger.Info("启动NAT穿透管理器")

	if n.provider != nil {
		if err := n.provider.Start(); err != nil {
			n.logger.WithFields(logrus.Fields{
				"type":  n.provider.Type(),
				"name":  n.provider.Name(),
				"error": err,
			}).Warn("启动NAT穿透提供者失败")
			return err
		}
	}

	n.logger.WithFields(logrus.Fields{
		"type": n.provider.Type(),
		"name": n.provider.Name(),
	}).Info("NAT穿透提供者启动成功")

	return nil
}

// Stop 停止NAT穿透管理器
func (n *NATHolePunching) Stop() error {
	n.logger.Info("停止NAT穿透管理器")

	if n.provider != nil {
		n.provider.Stop()
	}
	n.logger.WithFields(logrus.Fields{
		"type": n.provider.Type(),
		"name": n.provider.Name(),
	}).Info("NAT穿透提供者已停止")
	return nil
}

// CreateHole 创建NAT穿透
func (n *NATHolePunching) CreateHole(localPort int, externalPort int, protocol, description string) (*NATHole, error) {
	// 根据NAT类型选择相应的提供者
	if n.provider != nil && n.provider.IsAvailable() {
		hole, err := n.provider.CreateHole(localPort, externalPort, protocol, description)
		if err == nil {
			n.logger.WithFields(logrus.Fields{
				"local_port": localPort,
				"protocol":   protocol,
				"type":       n.provider.Type(),
			}).Info("创建NAT穿透成功")

			if n.onHoleCreated != nil {
				n.onHoleCreated(localPort, hole.ExternalPort, protocol, n.provider.Type())
			}
			return hole, nil
		}
		n.logger.WithFields(logrus.Fields{
			"local_port": localPort,
			"protocol":   protocol,
			"type":       n.provider.Type(),
			"error":      err,
		}).Error("创建NAT穿透失败")
	}
	return nil, fmt.Errorf("未找到可用的NAT穿透提供者")
}

// RemoveHole 移除NAT穿透
func (n *NATHolePunching) RemoveHole(localPort int, externalPort int, protocol string) error {
	if n.provider != nil && n.provider.IsAvailable() {
		if err := n.provider.RemoveHole(localPort, externalPort, protocol); err != nil {
			n.logger.WithFields(logrus.Fields{
				"local_port": localPort,
				"protocol":   protocol,
				"type":       n.provider.Type(),
				"error":      err,
			}).Warn("从提供者移除NAT穿透失败")
		} else {
			n.logger.WithFields(logrus.Fields{
				"local_port": localPort,
				"protocol":   protocol,
				"type":       n.provider.Type(),
			}).Info("从提供者移除NAT穿透成功")

			if n.onHoleRemoved != nil {
				n.onHoleRemoved(localPort, externalPort, protocol, n.provider.Type())
			}
		}
	}

	return nil
}

// GetHoles 获取所有穿透
func (n *NATHolePunching) GetHoles() map[string]*NATHole {
	allHoles := make(map[string]*NATHole)

	if n.provider != nil {
		holes := n.provider.GetHoles()
		for key, hole := range holes {
			allHoles[key] = hole
		}
	}

	return allHoles
}

// GetStatus 获取所有提供者状态
func (n *NATHolePunching) GetStatus() map[string]interface{} {
	if n.provider != nil {
		providerStatus := n.provider.GetStatus()
		providerStatus["type"] = n.provider.Type()
		providerStatus["name"] = n.provider.Name()
		providerStatus["available"] = n.provider.IsAvailable()
		return providerStatus
	}
	return nil
}

// SetCallbacks 设置回调函数
func (n *NATHolePunching) SetCallbacks(
	onHoleCreated func(localPort int, externalPort int, protocol string, natType types.NATType),
	onHoleRemoved func(localPort int, externalPort int, protocol string, natType types.NATType),
	onHoleFailed func(localPort int, externalPort int, protocol string, natType types.NATType, error error),
) {
	n.onHoleCreated = onHoleCreated
	n.onHoleRemoved = onHoleRemoved
	n.onHoleFailed = onHoleFailed
}
