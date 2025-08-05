package upnp

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/huin/goupnp"
	"github.com/huin/goupnp/dcps/internetgateway1"
	"github.com/huin/goupnp/dcps/internetgateway2"
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
	Client     interface{} // 支持多种客户端类型
	DeviceName string
	URL        string
	LastSeen   time.Time
	IsHealthy  bool
	FailCount  int
	DeviceType string // 设备类型
	Location   string // 设备位置
	ParentURL  string // 父设备URL
}

// UPnPManager UPnP管理器
type UPnPManager struct {
	logger          *logrus.Logger
	clients         []*UPnPClientInfo
	mutex           sync.RWMutex
	ctx             context.Context
	cancel          context.CancelFunc
	mappings        map[string]*PortMapping
	config          *Config
	discovered      bool
	healthTicker    *time.Ticker
	keepAliveTicker *time.Ticker // 新增保活定时器
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

	// 启动保活协程
	go um.keepAliveRoutine()

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

// keepAliveRoutine 保活协程
func (um *UPnPManager) keepAliveRoutine() {
	um.keepAliveTicker = time.NewTicker(um.config.KeepAliveInterval)
	defer um.keepAliveTicker.Stop()

	for {
		select {
		case <-um.ctx.Done():
			return
		case <-um.keepAliveTicker.C:
			um.performKeepAlive()
		}
	}
}

// performKeepAlive 执行保活操作
func (um *UPnPManager) performKeepAlive() {
	um.mutex.Lock()
	defer um.mutex.Unlock()

	if len(um.mappings) == 0 {
		um.logger.Debug("没有端口映射，跳过保活检查")
		return
	}

	um.logger.Debug("开始端口映射保活检查")

	// 检查所有映射是否需要续期
	now := time.Now()
	var mappingsToRenew []*PortMapping

	for _, mapping := range um.mappings {
		// 如果映射即将过期（在保活间隔内），则续期
		expiryTime := mapping.CreatedAt.Add(um.config.MappingDuration)
		timeUntilExpiry := expiryTime.Sub(now)

		if timeUntilExpiry <= um.config.KeepAliveInterval {
			mappingsToRenew = append(mappingsToRenew, mapping)
		}
	}

	if len(mappingsToRenew) == 0 {
		um.logger.Debug("没有需要续期的端口映射")
		return
	}

	um.logger.WithField("mappings_to_renew", len(mappingsToRenew)).Info("开始续期端口映射")

	// 续期所有即将过期的映射
	for _, mapping := range mappingsToRenew {
		if err := um.renewPortMapping(mapping); err != nil {
			um.logger.WithFields(logrus.Fields{
				"internal_port": mapping.InternalPort,
				"external_port": mapping.ExternalPort,
				"protocol":      mapping.Protocol,
				"error":         err,
			}).Warn("续期端口映射失败")
		} else {
			um.logger.WithFields(logrus.Fields{
				"internal_port": mapping.InternalPort,
				"external_port": mapping.ExternalPort,
				"protocol":      mapping.Protocol,
			}).Info("端口映射续期成功")
		}
	}
}

// renewPortMapping 续期单个端口映射
func (um *UPnPManager) renewPortMapping(mapping *PortMapping) error {
	// 获取本地IP地址
	localIP, err := um.getLocalIP()
	if err != nil {
		return fmt.Errorf("获取本地IP地址失败: %w", err)
	}

	// 尝试续期到所有可用的客户端
	var lastErr error
	for i, clientInfo := range um.clients {
		if !clientInfo.IsHealthy {
			um.logger.WithFields(logrus.Fields{
				"client_index": i,
				"device":       clientInfo.DeviceName,
			}).Debug("跳过不健康的UPnP客户端进行续期")
			continue
		}

		err := um.addPortMappingToClient(clientInfo.Client, mapping.InternalPort, mapping.ExternalPort, mapping.Protocol, localIP, mapping.Description)
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
				"internal_port": mapping.InternalPort,
				"external_port": mapping.ExternalPort,
				"protocol":      mapping.Protocol,
				"error":         err,
			}).Warn("续期端口映射失败")
			continue
		}

		// 续期成功，重置失败计数并更新时间戳
		clientInfo.FailCount = 0
		clientInfo.IsHealthy = true
		clientInfo.LastSeen = time.Now()
		mapping.CreatedAt = time.Now() // 更新创建时间

		return nil
	}

	return fmt.Errorf("所有UPnP客户端都续期端口映射失败: %w", lastErr)
}

// checkClientHealth 检查单个客户端健康状态
func (um *UPnPManager) checkClientHealth(clientInfo *UPnPClientInfo) bool {
	// 尝试获取外部IP地址作为健康检查
	switch client := clientInfo.Client.(type) {
	case *internetgateway1.WANIPConnection1:
		_, err := client.GetExternalIPAddress()
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
	case *internetgateway2.WANIPConnection2:
		_, err := client.GetExternalIPAddress()
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
	default:
		um.logger.WithField("device_type", clientInfo.DeviceType).Warn("不支持的客户端类型进行健康检查")
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

// Discover 发现UPnP设备（简化版）
func (um *UPnPManager) Discover() error {
	um.logger.Info("开始UPnP设备发现")

	// 使用基础发现方法
	allDevices, err := um.discoverDevicesBasic()
	if err != nil {
		return fmt.Errorf("发现UPnP设备失败: %w", err)
	}

	if len(allDevices) == 0 {
		return fmt.Errorf("未找到UPnP设备")
	}

	um.logger.WithField("device_count", len(allDevices)).Info("发现UPnP设备")

	um.mutex.Lock()
	defer um.mutex.Unlock()

	// 处理发现的设备
	for _, device := range allDevices {
		if err := um.processDiscoveredDevice(device); err != nil {
			um.logger.WithError(err).Warn("处理发现的设备失败")
			continue
		}
	}

	if len(um.clients) == 0 {
		return fmt.Errorf("未找到可用的WAN连接服务")
	}

	um.logger.WithField("client_count", len(um.clients)).Info("UPnP设备发现完成")
	um.discovered = true
	return nil
}

// discoverDevicesBasic 基础设备发现（保持向后兼容）
func (um *UPnPManager) discoverDevicesBasic() ([]goupnp.MaybeRootDevice, error) {
	return goupnp.DiscoverDevices("urn:schemas-upnp-org:device:InternetGatewayDevice:1")
}

// processDiscoveredDevice 处理发现的设备
func (um *UPnPManager) processDiscoveredDevice(device goupnp.MaybeRootDevice) error {
	if device.Root == nil {
		return fmt.Errorf("设备根信息为空")
	}

	// 尝试创建多种类型的客户端
	clients, err := um.createClientsFromDevice(device)
	if err != nil {
		um.logger.WithFields(logrus.Fields{
			"device": device.Root.Device.FriendlyName,
			"error":  err,
		}).Debug("无法从设备创建客户端")
		return err
	}

	// 添加客户端到管理器
	for _, client := range clients {
		um.addClient(client)
	}

	return nil
}

// createClientsFromDevice 从设备创建客户端
func (um *UPnPManager) createClientsFromDevice(device goupnp.MaybeRootDevice) ([]*UPnPClientInfo, error) {
	var clients []*UPnPClientInfo

	// 尝试创建WANIPConnection1客户端
	wanIP1Clients, err := internetgateway1.NewWANIPConnection1ClientsFromRootDevice(device.Root, &device.Root.URLBase)
	if err == nil && len(wanIP1Clients) > 0 {
		for _, client := range wanIP1Clients {
			clientInfo := &UPnPClientInfo{
				Client:     client,
				DeviceName: device.Root.Device.FriendlyName,
				URL:        device.Root.URLBase.String(),
				LastSeen:   time.Now(),
				IsHealthy:  true,
				FailCount:  0,
				DeviceType: "WANIPConnection1",
				Location:   device.Root.URLBase.String(),
			}
			clients = append(clients, clientInfo)
		}
	}

	// 尝试创建WANIPConnection2客户端
	wanIP2Clients, err := internetgateway2.NewWANIPConnection2ClientsFromRootDevice(device.Root, &device.Root.URLBase)
	if err == nil && len(wanIP2Clients) > 0 {
		for _, client := range wanIP2Clients {
			clientInfo := &UPnPClientInfo{
				Client:     client,
				DeviceName: device.Root.Device.FriendlyName,
				URL:        device.Root.URLBase.String(),
				LastSeen:   time.Now(),
				IsHealthy:  true,
				FailCount:  0,
				DeviceType: "WANIPConnection2",
				Location:   device.Root.URLBase.String(),
			}
			clients = append(clients, clientInfo)
		}
	}

	return clients, nil
}

// addClient 添加客户端到管理器
func (um *UPnPManager) addClient(clientInfo *UPnPClientInfo) {
	// 检查是否已存在相同的客户端
	exists := false
	for _, existingClient := range um.clients {
		if existingClient.URL == clientInfo.URL && existingClient.DeviceType == clientInfo.DeviceType {
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
		um.logger.WithFields(logrus.Fields{
			"device":      clientInfo.DeviceName,
			"device_type": clientInfo.DeviceType,
			"url":         clientInfo.URL,
		}).Info("添加UPnP客户端")
	}
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
func (um *UPnPManager) addPortMappingToClient(client interface{}, internalPort, externalPort int, protocol, internalClient, description string) error {
	// 根据客户端类型选择不同的添加方法
	switch c := client.(type) {
	case *internetgateway1.WANIPConnection1:
		return c.AddPortMapping(
			"",                   // NewRemoteHost
			uint16(externalPort), // NewExternalPort
			protocol,             // NewProtocol
			uint16(internalPort), // NewInternalPort
			internalClient,       // NewInternalClient
			true,                 // NewEnabled
			description,          // NewPortMappingDescription
			uint32(um.config.MappingDuration.Seconds()), // NewLeaseDuration
		)
	case *internetgateway2.WANIPConnection2:
		return c.AddPortMapping(
			"",                   // NewRemoteHost
			uint16(externalPort), // NewExternalPort
			protocol,             // NewProtocol
			uint16(internalPort), // NewInternalPort
			internalClient,       // NewInternalClient
			true,                 // NewEnabled
			description,          // NewPortMappingDescription
			uint32(um.config.MappingDuration.Seconds()), // NewLeaseDuration
		)
	default:
		return fmt.Errorf("不支持的客户端类型: %T", client)
	}
}

// removePortMappingFromClient 从指定客户端删除端口映射
func (um *UPnPManager) removePortMappingFromClient(client interface{}, externalPort int, protocol string) error {
	// 根据客户端类型选择不同的删除方法
	switch c := client.(type) {
	case *internetgateway1.WANIPConnection1:
		return c.DeletePortMapping(
			"",                   // NewRemoteHost
			uint16(externalPort), // NewExternalPort
			protocol,             // NewProtocol
		)
	case *internetgateway2.WANIPConnection2:
		return c.DeletePortMapping(
			"",                   // NewRemoteHost
			uint16(externalPort), // NewExternalPort
			protocol,             // NewProtocol
		)
	default:
		return fmt.Errorf("不支持的客户端类型: %T", client)
	}
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

	if um.keepAliveTicker != nil {
		um.keepAliveTicker.Stop()
	}

	// 移除所有映射
	for _, mapping := range um.mappings {
		um.RemovePortMapping(mapping.InternalPort, mapping.ExternalPort, mapping.Protocol)
	}
}
