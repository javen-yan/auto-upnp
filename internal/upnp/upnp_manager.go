package upnp

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/huin/goupnp"
	"github.com/huin/goupnp/dcps/internetgateway1"
	"github.com/sirupsen/logrus"
)

// PortMapping 端口映射信息
type PortMapping struct {
	InternalPort   int
	ExternalPort   int
	Protocol       string
	InternalClient string
	Description    string
	LeaseDuration  uint32
	CreatedAt      time.Time
}

// UPnPManager UPnP管理器
type UPnPManager struct {
	logger   *logrus.Logger
	clients  []*internetgateway1.WANIPConnection1
	mutex    sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	mappings map[string]*PortMapping
	config   *Config
}

// Config UPnP配置
type Config struct {
	DiscoveryTimeout time.Duration
	MappingDuration  time.Duration
	RetryAttempts    int
	RetryDelay       time.Duration
	MaxMappings      int
}

// NewUPnPManager 创建新的UPnP管理器
func NewUPnPManager(config *Config, logger *logrus.Logger) *UPnPManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &UPnPManager{
		logger:   logger,
		clients:  make([]*internetgateway1.WANIPConnection1, 0),
		ctx:      ctx,
		cancel:   cancel,
		mappings: make(map[string]*PortMapping),
		config:   config,
	}
}

// Discover 发现UPnP设备
func (um *UPnPManager) Discover() error {
	um.logger.Info("开始发现UPnP设备")

	ctx, cancel := context.WithTimeout(um.ctx, um.config.DiscoveryTimeout)
	defer cancel()

	// 发现所有UPnP设备
	devices, err := goupnp.DiscoverDevices(ctx, "urn:schemas-upnp-org:device:InternetGatewayDevice:1")
	if err != nil {
		return fmt.Errorf("发现UPnP设备失败: %w", err)
	}

	if len(devices) == 0 {
		return fmt.Errorf("未找到UPnP设备")
	}

	um.logger.WithField("device_count", len(devices)).Info("发现UPnP设备")

	// 获取WAN IP连接客户端
	for _, device := range devices {
		client, err := internetgateway1.NewWANIPConnection1ClientsFromDevice(device, device.Root.URLBase)
		if err != nil {
			um.logger.WithField("device", device.Root.Device.FriendlyName).Warn("无法创建WAN IP连接客户端")
			continue
		}

		if len(client) > 0 {
			um.mutex.Lock()
			um.clients = append(um.clients, client[0])
			um.mutex.Unlock()

			um.logger.WithFields(logrus.Fields{
				"device": device.Root.Device.FriendlyName,
				"url":    device.Root.URLBase,
			}).Info("添加UPnP客户端")
		}
	}

	if len(um.clients) == 0 {
		return fmt.Errorf("未找到可用的WAN IP连接")
	}

	um.logger.WithField("client_count", len(um.clients)).Info("UPnP设备发现完成")
	return nil
}

// AddPortMapping 添加端口映射
func (um *UPnPManager) AddPortMapping(internalPort, externalPort int, protocol string, description string) error {
	um.mutex.Lock()
	defer um.mutex.Unlock()

	// 检查映射数量限制
	if len(um.mappings) >= um.config.MaxMappings {
		return fmt.Errorf("端口映射数量已达到上限: %d", um.config.MaxMappings)
	}

	// 检查是否已存在映射
	mappingKey := um.getMappingKey(internalPort, externalPort, protocol)
	if _, exists := um.mappings[mappingKey]; exists {
		return fmt.Errorf("端口映射已存在: %s", mappingKey)
	}

	// 获取本地IP地址
	localIP, err := um.getLocalIP()
	if err != nil {
		return fmt.Errorf("获取本地IP地址失败: %w", err)
	}

	// 尝试添加映射到所有可用的客户端
	var lastErr error
	for i, client := range um.clients {
		err := um.addPortMappingToClient(client, internalPort, externalPort, protocol, localIP, description)
		if err != nil {
			lastErr = err
			um.logger.WithFields(logrus.Fields{
				"client_index":  i,
				"internal_port": internalPort,
				"external_port": externalPort,
				"protocol":      protocol,
				"error":         err,
			}).Warn("添加端口映射失败")
			continue
		}

		// 映射成功，记录映射信息
		mapping := &PortMapping{
			InternalPort:   internalPort,
			ExternalPort:   externalPort,
			Protocol:       protocol,
			InternalClient: localIP,
			Description:    description,
			LeaseDuration:  uint32(um.config.MappingDuration.Seconds()),
			CreatedAt:      time.Now(),
		}

		um.mappings[mappingKey] = mapping

		um.logger.WithFields(logrus.Fields{
			"internal_port": internalPort,
			"external_port": externalPort,
			"protocol":      protocol,
			"local_ip":      localIP,
			"description":   description,
		}).Info("端口映射添加成功")

		return nil
	}

	return fmt.Errorf("所有UPnP客户端都添加端口映射失败: %w", lastErr)
}

// RemovePortMapping 删除端口映射
func (um *UPnPManager) RemovePortMapping(internalPort, externalPort int, protocol string) error {
	um.mutex.Lock()
	defer um.mutex.Unlock()

	mappingKey := um.getMappingKey(internalPort, externalPort, protocol)
	mapping, exists := um.mappings[mappingKey]
	if !exists {
		return fmt.Errorf("端口映射不存在: %s", mappingKey)
	}

	// 尝试从所有客户端删除映射
	var lastErr error
	for i, client := range um.clients {
		err := um.removePortMappingFromClient(client, externalPort, protocol)
		if err != nil {
			lastErr = err
			um.logger.WithFields(logrus.Fields{
				"client_index":  i,
				"external_port": externalPort,
				"protocol":      protocol,
				"error":         err,
			}).Warn("删除端口映射失败")
			continue
		}

		// 删除成功，移除映射记录
		delete(um.mappings, mappingKey)

		um.logger.WithFields(logrus.Fields{
			"internal_port": mapping.InternalPort,
			"external_port": mapping.ExternalPort,
			"protocol":      mapping.Protocol,
		}).Info("端口映射删除成功")

		return nil
	}

	return fmt.Errorf("所有UPnP客户端都删除端口映射失败: %w", lastErr)
}

// GetPortMappings 获取所有端口映射
func (um *UPnPManager) GetPortMappings() map[string]*PortMapping {
	um.mutex.RLock()
	defer um.mutex.RUnlock()

	result := make(map[string]*PortMapping)
	for key, mapping := range um.mappings {
		result[key] = &PortMapping{
			InternalPort:   mapping.InternalPort,
			ExternalPort:   mapping.ExternalPort,
			Protocol:       mapping.Protocol,
			InternalClient: mapping.InternalClient,
			Description:    mapping.Description,
			LeaseDuration:  mapping.LeaseDuration,
			CreatedAt:      mapping.CreatedAt,
		}
	}

	return result
}

// CleanupExpiredMappings 清理过期的端口映射
func (um *UPnPManager) CleanupExpiredMappings() {
	um.mutex.Lock()
	defer um.mutex.Unlock()

	now := time.Now()
	var expiredKeys []string

	for key, mapping := range um.mappings {
		if um.config.MappingDuration > 0 {
			expiredTime := mapping.CreatedAt.Add(um.config.MappingDuration)
			if now.After(expiredTime) {
				expiredKeys = append(expiredKeys, key)
			}
		}
	}

	for _, key := range expiredKeys {
		mapping := um.mappings[key]
		um.logger.WithFields(logrus.Fields{
			"internal_port": mapping.InternalPort,
			"external_port": mapping.ExternalPort,
			"protocol":      mapping.Protocol,
		}).Info("清理过期的端口映射")

		// 从所有客户端删除映射
		for _, client := range um.clients {
			um.removePortMappingFromClient(client, mapping.ExternalPort, mapping.Protocol)
		}

		delete(um.mappings, key)
	}
}

// addPortMappingToClient 向指定客户端添加端口映射
func (um *UPnPManager) addPortMappingToClient(client *internetgateway1.WANIPConnection1, internalPort, externalPort int, protocol, internalClient, description string) error {
	ctx, cancel := context.WithTimeout(um.ctx, 10*time.Second)
	defer cancel()

	return client.AddPortMapping(ctx, &internetgateway1.AddPortMappingArgs{
		NewRemoteHost:             "",
		NewExternalPort:           uint16(externalPort),
		NewProtocol:               protocol,
		NewInternalPort:           uint16(internalPort),
		NewInternalClient:         internalClient,
		NewEnabled:                1,
		NewPortMappingDescription: description,
		NewLeaseDuration:          uint32(um.config.MappingDuration.Seconds()),
	})
}

// removePortMappingFromClient 从指定客户端删除端口映射
func (um *UPnPManager) removePortMappingFromClient(client *internetgateway1.WANIPConnection1, externalPort int, protocol string) error {
	ctx, cancel := context.WithTimeout(um.ctx, 10*time.Second)
	defer cancel()

	return client.DeletePortMapping(ctx, &internetgateway1.DeletePortMappingArgs{
		NewRemoteHost:   "",
		NewExternalPort: uint16(externalPort),
		NewProtocol:     protocol,
	})
}

// getMappingKey 获取映射键
func (um *UPnPManager) getMappingKey(internalPort, externalPort int, protocol string) string {
	return fmt.Sprintf("%d:%d:%s", internalPort, externalPort, protocol)
}

// getLocalIP 获取本地IP地址
func (um *UPnPManager) getLocalIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}

// Close 关闭UPnP管理器
func (um *UPnPManager) Close() {
	um.logger.Info("关闭UPnP管理器")
	um.cancel()
}
