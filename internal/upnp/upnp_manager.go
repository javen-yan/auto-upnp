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

// UPnPClientInfo UPnP客户端信息
type UPnPClientInfo struct {
	Client     *internetgateway1.WANIPConnection1
	DeviceName string
	URL        string
	LastSeen   time.Time
	IsHealthy  bool
	FailCount  int
}

// UPnPManager UPnP管理器
type UPnPManager struct {
	logger       *logrus.Logger
	clients      []*UPnPClientInfo
	mutex        sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	mappings     map[string]*PortMapping
	config       *Config
	discovered   bool
	healthTicker *time.Ticker
}

// Config UPnP配置
type Config struct {
	DiscoveryTimeout    time.Duration
	MappingDuration     time.Duration
	RetryAttempts       int
	RetryDelay          time.Duration
	HealthCheckInterval time.Duration // 健康检查间隔
	MaxFailCount        int           // 最大失败次数
	KeepAliveInterval   time.Duration // 保活间隔
}

// NewUPnPManager 创建新的UPnP管理器
func NewUPnPManager(config *Config, logger *logrus.Logger) *UPnPManager {
	ctx, cancel := context.WithCancel(context.Background())

	// 设置默认值
	if config.HealthCheckInterval == 0 {
		config.HealthCheckInterval = 2 * time.Minute
	}
	if config.MaxFailCount == 0 {
		config.MaxFailCount = 3
	}
	if config.KeepAliveInterval == 0 {
		config.KeepAliveInterval = 5 * time.Minute
	}
	um := &UPnPManager{
		logger:     logger,
		clients:    make([]*UPnPClientInfo, 0),
		ctx:        ctx,
		cancel:     cancel,
		mappings:   make(map[string]*PortMapping),
		config:     config,
		discovered: false,
	}

	// 启动健康检查协程
	go um.healthCheckRoutine()

	return um
}

// healthCheckRoutine 健康检查协程
func (um *UPnPManager) healthCheckRoutine() {
	um.healthTicker = time.NewTicker(um.config.HealthCheckInterval)
	defer um.healthTicker.Stop()

	for {
		select {
		case <-um.ctx.Done():
			return
		case <-um.healthTicker.C:
			um.performHealthCheck()
		}
	}
}

// performHealthCheck 执行健康检查
func (um *UPnPManager) performHealthCheck() {
	um.mutex.Lock()
	defer um.mutex.Unlock()

	if len(um.clients) == 0 {
		um.logger.Debug("没有UPnP客户端，跳过健康检查")
		return
	}

	um.logger.Debug("开始UPnP客户端健康检查")

	var healthyClients []*UPnPClientInfo
	var needRediscovery bool

	for _, clientInfo := range um.clients {
		if um.checkClientHealth(clientInfo) {
			healthyClients = append(healthyClients, clientInfo)
		} else {
			um.logger.WithFields(logrus.Fields{
				"device":     clientInfo.DeviceName,
				"url":        clientInfo.URL,
				"fail_count": clientInfo.FailCount,
			}).Warn("UPnP客户端健康检查失败")
			needRediscovery = true
		}
	}

	// 更新客户端列表
	um.clients = healthyClients

	// 如果没有健康的客户端，尝试重新发现
	if len(um.clients) == 0 {
		um.logger.Warn("所有UPnP客户端都不健康，尝试重新发现")
		um.discovered = false
		go um.rediscoverDevices()
	} else if needRediscovery {
		um.logger.Info("部分UPnP客户端不健康，尝试补充发现")
		go um.rediscoverDevices()
	}

	um.logger.WithField("healthy_clients", len(um.clients)).Debug("UPnP健康检查完成")
}

// checkClientHealth 检查单个客户端健康状态
func (um *UPnPManager) checkClientHealth(clientInfo *UPnPClientInfo) bool {
	// 尝试获取外部IP地址作为健康检查
	_, err := clientInfo.Client.GetExternalIPAddress()
	if err != nil {
		clientInfo.FailCount++
		clientInfo.IsHealthy = false

		if clientInfo.FailCount >= um.config.MaxFailCount {
			um.logger.WithFields(logrus.Fields{
				"device":     clientInfo.DeviceName,
				"url":        clientInfo.URL,
				"fail_count": clientInfo.FailCount,
				"error":      err,
			}).Warn("UPnP客户端失败次数过多，标记为不健康")
			return false
		}

		um.logger.WithFields(logrus.Fields{
			"device":     clientInfo.DeviceName,
			"fail_count": clientInfo.FailCount,
			"error":      err,
		}).Debug("UPnP客户端健康检查失败")
		return false
	}

	// 健康检查成功
	clientInfo.FailCount = 0
	clientInfo.IsHealthy = true
	clientInfo.LastSeen = time.Now()
	return true
}

// rediscoverDevices 重新发现设备
func (um *UPnPManager) rediscoverDevices() {
	um.logger.Info("开始重新发现UPnP设备")

	if err := um.Discover(); err != nil {
		um.logger.WithError(err).Warn("重新发现UPnP设备失败")
		return
	}

	um.logger.Info("重新发现UPnP设备成功")
}

// Discover 发现UPnP设备
func (um *UPnPManager) Discover() error {
	um.logger.Info("开始发现UPnP设备")

	// 发现所有UPnP设备
	devices, err := goupnp.DiscoverDevices("urn:schemas-upnp-org:device:InternetGatewayDevice:1")
	if err != nil {
		return fmt.Errorf("发现UPnP设备失败: %w", err)
	}

	if len(devices) == 0 {
		return fmt.Errorf("未找到UPnP设备")
	}

	um.logger.WithField("device_count", len(devices)).Info("发现UPnP设备")

	um.mutex.Lock()
	defer um.mutex.Unlock()

	// 获取WAN IP连接客户端
	for _, device := range devices {
		clients, err := internetgateway1.NewWANIPConnection1ClientsFromRootDevice(device.Root, &device.Root.URLBase)
		if err != nil {
			um.logger.WithField("device", device.Root.Device.FriendlyName).Warn("无法创建WAN IP连接客户端")
			continue
		}

		if len(clients) > 0 {
			clientInfo := &UPnPClientInfo{
				Client:     clients[0],
				DeviceName: device.Root.Device.FriendlyName,
				URL:        device.Root.URLBase.String(),
				LastSeen:   time.Now(),
				IsHealthy:  true,
				FailCount:  0,
			}

			// 检查是否已存在相同的客户端
			exists := false
			for _, existingClient := range um.clients {
				if existingClient.URL == clientInfo.URL {
					exists = true
					// 更新现有客户端信息
					existingClient.Client = clientInfo.Client
					existingClient.LastSeen = time.Now()
					existingClient.IsHealthy = true
					existingClient.FailCount = 0
					break
				}
			}

			if !exists {
				um.clients = append(um.clients, clientInfo)
			}

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
	um.discovered = true
	return nil
}

// AddPortMapping 添加端口映射
func (um *UPnPManager) AddPortMapping(internalPort, externalPort int, protocol string, description string) error {
	um.mutex.Lock()
	defer um.mutex.Unlock()

	// 检查是否已存在映射
	mappingKey := um.getMappingKey(internalPort, externalPort, protocol)
	if _, exists := um.mappings[mappingKey]; exists {
		return fmt.Errorf("端口映射已存在: %s", mappingKey)
	}

	// 如果没有发现UPnP设备，先尝试重新发现
	if !um.discovered || len(um.clients) == 0 {
		um.logger.Info("尝试重新发现UPnP设备")
		if err := um.Discover(); err != nil {
			return fmt.Errorf("无法发现UPnP设备，无法添加端口映射: %w", err)
		}
	}

	// 获取本地IP地址
	localIP, err := um.getLocalIP()
	if err != nil {
		return fmt.Errorf("获取本地IP地址失败: %w", err)
	}

	// 尝试添加映射到所有可用的客户端
	var lastErr error
	for i, clientInfo := range um.clients {
		if !clientInfo.IsHealthy {
			um.logger.WithFields(logrus.Fields{
				"client_index": i,
				"device":       clientInfo.DeviceName,
			}).Debug("跳过不健康的UPnP客户端")
			continue
		}

		err := um.addPortMappingToClient(clientInfo.Client, internalPort, externalPort, protocol, localIP, description)
		if err != nil {
			lastErr = err
			// 增加失败计数
			clientInfo.FailCount++
			if clientInfo.FailCount >= um.config.MaxFailCount {
				clientInfo.IsHealthy = false
			}

			um.logger.WithFields(logrus.Fields{
				"client_index":  i,
				"device":        clientInfo.DeviceName,
				"internal_port": internalPort,
				"external_port": externalPort,
				"protocol":      protocol,
				"error":         err,
			}).Warn("添加端口映射失败")
			continue
		}

		// 映射成功，重置失败计数
		clientInfo.FailCount = 0
		clientInfo.IsHealthy = true
		clientInfo.LastSeen = time.Now()

		// 记录映射信息
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
			"device":        clientInfo.DeviceName,
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

	// 如果没有发现UPnP设备，先尝试重新发现
	if !um.discovered || len(um.clients) == 0 {
		um.logger.Info("尝试重新发现UPnP设备")
		if err := um.Discover(); err != nil {
			return fmt.Errorf("无法发现UPnP设备，无法删除端口映射: %w", err)
		}
	}

	// 尝试从所有客户端删除映射
	var lastErr error
	for i, clientInfo := range um.clients {
		if !clientInfo.IsHealthy {
			um.logger.WithFields(logrus.Fields{
				"client_index": i,
				"device":       clientInfo.DeviceName,
			}).Debug("跳过不健康的UPnP客户端")
			continue
		}

		err := um.removePortMappingFromClient(clientInfo.Client, externalPort, protocol)
		if err != nil {
			lastErr = err
			// 增加失败计数
			clientInfo.FailCount++
			if clientInfo.FailCount >= um.config.MaxFailCount {
				clientInfo.IsHealthy = false
			}

			um.logger.WithFields(logrus.Fields{
				"client_index":  i,
				"device":        clientInfo.DeviceName,
				"external_port": externalPort,
				"protocol":      protocol,
				"error":         err,
			}).Warn("删除端口映射失败")
			continue
		}

		// 删除成功，重置失败计数
		clientInfo.FailCount = 0
		clientInfo.IsHealthy = true
		clientInfo.LastSeen = time.Now()

		// 移除映射记录
		delete(um.mappings, mappingKey)

		um.logger.WithFields(logrus.Fields{
			"internal_port": mapping.InternalPort,
			"external_port": mapping.ExternalPort,
			"protocol":      mapping.Protocol,
			"device":        clientInfo.DeviceName,
		}).Info("端口映射删除成功")

		return nil
	}

	return fmt.Errorf("所有UPnP客户端都删除端口映射失败: %w", lastErr)
}

// GetPortMappings 获取所有端口映射
func (um *UPnPManager) GetPortMappings() map[string]*PortMapping {
	um.mutex.RLock()
	defer um.mutex.RUnlock()

	mappings := make(map[string]*PortMapping)
	for key, mapping := range um.mappings {
		mappings[key] = mapping
	}
	return mappings
}

// GetClientCount 获取UPnP客户端数量
func (um *UPnPManager) GetClientCount() int {
	um.mutex.RLock()
	defer um.mutex.RUnlock()
	return len(um.clients)
}

// GetHealthyClientCount 获取健康的UPnP客户端数量
func (um *UPnPManager) GetHealthyClientCount() int {
	um.mutex.RLock()
	defer um.mutex.RUnlock()

	count := 0
	for _, client := range um.clients {
		if client.IsHealthy {
			count++
		}
	}
	return count
}

// IsUPnPAvailable 检查UPnP服务是否可用
func (um *UPnPManager) IsUPnPAvailable() bool {
	return um.GetHealthyClientCount() > 0
}

// GetClientStatus 获取客户端状态信息
func (um *UPnPManager) GetClientStatus() []map[string]interface{} {
	um.mutex.RLock()
	defer um.mutex.RUnlock()

	var status []map[string]interface{}
	for _, client := range um.clients {
		status = append(status, map[string]interface{}{
			"device_name": client.DeviceName,
			"url":         client.URL,
			"is_healthy":  client.IsHealthy,
			"fail_count":  client.FailCount,
			"last_seen":   client.LastSeen,
		})
	}
	return status
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

		// 从所有健康的客户端删除映射
		for _, clientInfo := range um.clients {
			if clientInfo.IsHealthy {
				um.removePortMappingFromClient(clientInfo.Client, mapping.ExternalPort, mapping.Protocol)
			}
		}

		delete(um.mappings, key)
	}
}

// addPortMappingToClient 向指定客户端添加端口映射
func (um *UPnPManager) addPortMappingToClient(client *internetgateway1.WANIPConnection1, internalPort, externalPort int, protocol, internalClient, description string) error {
	return client.AddPortMapping(
		"",                   // NewRemoteHost
		uint16(externalPort), // NewExternalPort
		protocol,             // NewProtocol
		uint16(internalPort), // NewInternalPort
		internalClient,       // NewInternalClient
		true,                 // NewEnabled
		description,          // NewPortMappingDescription
		uint32(um.config.MappingDuration.Seconds()), // NewLeaseDuration
	)
}

// removePortMappingFromClient 从指定客户端删除端口映射
func (um *UPnPManager) removePortMappingFromClient(client *internetgateway1.WANIPConnection1, externalPort int, protocol string) error {
	return client.DeletePortMapping(
		"",                   // NewRemoteHost
		uint16(externalPort), // NewExternalPort
		protocol,             // NewProtocol
	)
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
	if um.healthTicker != nil {
		um.healthTicker.Stop()
	}

	// 移除所有映射
	for _, mapping := range um.mappings {
		um.RemovePortMapping(mapping.InternalPort, mapping.ExternalPort, mapping.Protocol)
	}
}
