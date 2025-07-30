package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"auto-upnp/config"
	"auto-upnp/internal/port_mapping"
	"auto-upnp/internal/portmonitor"
	"auto-upnp/internal/util"

	"github.com/sirupsen/logrus"
)

// AutoUPnPService 自动UPnP服务
type AutoUPnPService struct {
	config             *config.Config
	logger             *logrus.Logger
	autoPortMonitor    *portmonitor.AutoPortMonitor
	portMappingManager *port_mapping.PortMappingManager
	storeService       *StoreService
	ctx                context.Context
	cancel             context.CancelFunc
	wg                 sync.WaitGroup
}

// NewAutoUPnPService 创建新的自动UPnP服务
func NewAutoUPnPService(cfg *config.Config, logger *logrus.Logger) *AutoUPnPService {
	ctx, cancel := context.WithCancel(context.Background())

	// 创建端口映射管理器
	portMappingManager := port_mapping.NewPortMappingManager(logger)

	// 创建存储服务
	storeService := NewStoreService(cfg.Admin.DataDir, logger, portMappingManager)

	return &AutoUPnPService{
		config:             cfg,
		logger:             logger,
		portMappingManager: portMappingManager,
		storeService:       storeService,
		ctx:                ctx,
		cancel:             cancel,
	}
}

// Start 启动自动UPnP服务
func (as *AutoUPnPService) Start() error {
	as.logger.Info("启动自动UPnP服务")

	// 创建并添加UPnP提供者
	upnpConfig := map[string]interface{}{
		"discovery_timeout":     as.config.UPnP.DiscoveryTimeout,
		"mapping_duration":      as.config.UPnP.MappingDuration,
		"retry_attempts":        as.config.UPnP.RetryAttempts,
		"retry_delay":           as.config.UPnP.RetryDelay,
		"health_check_interval": as.config.UPnP.HealthCheckInterval,
		"max_fail_count":        as.config.UPnP.MaxFailCount,
		"keep_alive_interval":   as.config.UPnP.KeepAliveInterval,
	}

	upnpProvider := port_mapping.NewUPnPProvider(as.logger, upnpConfig)
	as.portMappingManager.AddProvider(upnpProvider)

	// 如果启用NAT穿透，创建并添加TURN提供者
	if as.config.NATTraversal.Enabled {
		// 转换配置类型
		turnServers := make([]map[string]interface{}, 0, len(as.config.NATTraversal.TURNServers))
		for _, server := range as.config.NATTraversal.TURNServers {
			turnServers = append(turnServers, map[string]interface{}{
				"host":     server.Host,
				"port":     server.Port,
				"username": server.Username,
				"password": server.Password,
				"realm":    server.Realm,
			})
		}

		turnConfig := map[string]interface{}{
			"turn_servers": turnServers,
		}
		turnProvider := port_mapping.NewTURNProvider(as.logger, turnConfig)
		as.portMappingManager.AddProvider(turnProvider)
	}

	// 设置端口映射管理器的回调
	as.portMappingManager.SetCallbacks(
		as.onMappingCreated,
		as.onMappingRemoved,
		as.onMappingFailed,
	)

	// 启动端口映射管理器
	if err := as.portMappingManager.Start(as.config.Monitor.CheckInterval); err != nil {
		as.logger.WithError(err).Fatal("端口映射管理器启动失败")
	} else {
		// 异步启动存储服务，避免阻塞主流程
		go func() {
			if err := as.storeService.Start(); err != nil {
				as.logger.WithError(err).Error("存储服务启动失败")
			}
		}()
	}

	timeout := as.config.Monitor.CheckInterval

	// 初始化自动端口监控器
	autoPortConfig := &portmonitor.Config{
		CheckInterval: as.config.Monitor.CheckInterval,
		PortRange:     as.config.GetPortRange(),
		Timeout:       timeout,
		EnablePool:    true,
	}

	as.autoPortMonitor = portmonitor.NewAutoPortMonitor(autoPortConfig, as.logger)

	// 添加自动端口状态变化回调
	as.autoPortMonitor.AddCallback(as.onAutoPortStatusChanged)

	// 启动自动端口监控
	as.autoPortMonitor.Start()

	// 启动清理协程
	as.wg.Add(1)
	go as.cleanupRoutine()

	as.logger.Info("自动UPnP服务启动完成")
	return nil
}

// Stop 停止自动UPnP服务
func (as *AutoUPnPService) Stop() {
	as.logger.Info("停止自动UPnP服务")

	// 停止自动端口监控
	if as.autoPortMonitor != nil {
		as.autoPortMonitor.Stop()
	}

	// 停止端口映射管理器
	if as.portMappingManager != nil {
		as.portMappingManager.Stop()
	}

	if as.storeService != nil {
		as.storeService.Stop()
	}

	// 取消上下文
	as.cancel()

	// 等待所有协程完成
	as.wg.Wait()

	as.logger.Info("自动UPnP服务已停止")
}

// onAutoPortStatusChanged 自动端口状态变化回调
func (as *AutoUPnPService) onAutoPortStatusChanged(port int, isActive bool, protocol util.ProtocolType) {
	if isActive {
		// 端口变为活跃状态，创建自动映射
		as.logger.WithField("port", port).Info("检测到自动端口上线，创建端口映射")

		description := fmt.Sprintf("AutoUPnP-%d", port)
		_, err := as.portMappingManager.CreateMapping(port, port, string(protocol), description, port_mapping.MappingAddTypeAuto)
		if err != nil {
			as.logger.WithFields(logrus.Fields{
				"port":  port,
				"error": err,
			}).Error("自动端口映射创建失败")
			return
		}

		as.logger.WithField("port", port).Info("自动端口映射创建成功")
	} else {
		// 端口变为非活跃状态，删除自动映射
		as.logger.WithField("port", port).Info("检测到自动端口下线，删除映射")

		if err := as.portMappingManager.RemoveMapping(port, port, string(protocol), port_mapping.MappingAddTypeAuto); err != nil {
			as.logger.WithFields(logrus.Fields{
				"port":  port,
				"error": err,
			}).Warn("删除自动端口映射失败")
		} else {
			as.logger.WithField("port", port).Info("自动端口映射删除完成")
		}
	}
}

// onMappingCreated 映射创建回调
func (as *AutoUPnPService) onMappingCreated(port int, externalPort int, protocol string, providerType port_mapping.MappingType, addType port_mapping.MappingAddType) {
	as.logger.WithFields(logrus.Fields{
		"port":          port,
		"external_port": externalPort,
		"protocol":      protocol,
		"provider":      providerType,
	}).Info("端口映射创建成功")

	if addType == port_mapping.MappingAddTypeManual {
		// 异步保存到存储服务，避免阻塞主流程
		go func() {
			if err := as.storeService.Add(port, externalPort, protocol, fmt.Sprintf("Manual-%d", port)); err != nil {
				as.logger.WithError(err).Error("保存手动映射到存储服务失败")
			}
		}()
	}
}

// onMappingRemoved 映射删除回调
func (as *AutoUPnPService) onMappingRemoved(port int, externalPort int, protocol string, providerType port_mapping.MappingType, addType port_mapping.MappingAddType) {
	as.logger.WithFields(logrus.Fields{
		"port":          port,
		"external_port": externalPort,
		"protocol":      protocol,
		"provider":      providerType,
	}).Info("端口映射删除成功")

	if addType == port_mapping.MappingAddTypeManual {
		// 异步从存储服务删除，避免阻塞主流程
		go func() {
			if err := as.storeService.Remove(port, externalPort, protocol); err != nil {
				as.logger.WithError(err).Error("从存储服务删除手动映射失败")
			}
		}()
	}
}

// onMappingFailed 映射失败回调
func (as *AutoUPnPService) onMappingFailed(port int, externalPort int, protocol string, providerType port_mapping.MappingType, addType port_mapping.MappingAddType, err error) {
	as.logger.WithFields(logrus.Fields{
		"port":          port,
		"external_port": externalPort,
		"protocol":      protocol,
		"provider":      providerType,
		"error":         err,
	}).Error("端口映射操作失败")

	if addType == port_mapping.MappingAddTypeManual {
		// 异步从存储服务删除，避免阻塞主流程
		go func() {
			if err := as.storeService.Remove(port, externalPort, protocol); err != nil {
				as.logger.WithError(err).Error("从存储服务删除失败映射失败")
			}
		}()
	}
}

// cleanupRoutine 清理协程
func (as *AutoUPnPService) cleanupRoutine() {
	defer as.wg.Done()

	ticker := time.NewTicker(as.config.Monitor.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-as.ctx.Done():
			return
		case <-ticker.C:
			as.cleanupExpiredMappings()
		}
	}
}

// cleanupExpiredMappings 清理过期的端口映射
func (as *AutoUPnPService) cleanupExpiredMappings() {
	as.logger.Debug("开始清理过期的端口映射")

	// 从portMappingManager获取所有自动映射
	allMappings := as.portMappingManager.GetMappings()

	for _, mapping := range allMappings {
		if mapping.AddType == port_mapping.MappingAddTypeAuto {
			// 检查端口是否仍然活跃
			status, exists := as.autoPortMonitor.GetPortStatus(mapping.InternalPort)
			if !exists || !status.IsActive {
				as.logger.WithField("port", mapping.InternalPort).Info("清理非活跃的自动端口映射")
				// 删除映射
				as.portMappingManager.RemoveMapping(mapping.InternalPort, mapping.ExternalPort, mapping.Protocol, port_mapping.MappingAddTypeAuto)
			}
		}
	}
}

// GetStatus 获取服务状态
func (as *AutoUPnPService) GetStatus() map[string]interface{} {
	// 获取端口状态
	var activePorts []int

	if as.autoPortMonitor != nil {
		activePorts = as.autoPortMonitor.GetActivePorts()
	} else {
		activePorts = []int{}
	}

	portMappings := as.portMappingManager.GetMappings()
	activeAutoMappings := []int{}
	for _, mapping := range portMappings {
		if mapping.AddType == port_mapping.MappingAddTypeAuto {
			activeAutoMappings = append(activeAutoMappings, mapping.InternalPort)
		}
	}
	manualMappings := make([]*port_mapping.PortMapping, 0)
	for _, mapping := range portMappings {
		if mapping.AddType == port_mapping.MappingAddTypeManual {
			manualMappings = append(manualMappings, mapping)
		}
	}

	portMappingStatus := as.portMappingManager.GetStatus()
	return map[string]interface{}{
		"port_status": map[string]interface{}{
			"active_ports": len(activePorts),
		},
		"upnp_mappings": map[string]interface{}{
			"total_mappings": len(activeAutoMappings),
		},
		"manual_mappings": map[string]interface{}{
			"total_mappings": len(manualMappings),
		},
		"port_mapping_status": portMappingStatus,
	}
}

// AddManualMapping 手动添加端口映射
func (as *AutoUPnPService) AddManualMapping(internalPort, externalPort int, protocol, description string) error {
	if description == "" {
		description = fmt.Sprintf("Manual-%d", internalPort)
	}

	// 直接创建端口映射，不依赖端口监控
	_, err := as.portMappingManager.CreateMapping(internalPort, externalPort, protocol, description, port_mapping.MappingAddTypeManual)
	if err != nil {
		as.logger.WithError(err).Error("创建手动端口映射失败")
		return err
	}

	as.logger.WithFields(logrus.Fields{
		"internal_port": internalPort,
		"external_port": externalPort,
		"protocol":      protocol,
	}).Info("成功添加手动映射")

	return nil
}

// RemoveManualMapping 手动删除端口映射
func (as *AutoUPnPService) RemoveManualMapping(internalPort, externalPort int, protocol string) error {
	// 从端口映射管理器中删除（如果存在）
	if err := as.portMappingManager.RemoveMapping(internalPort, externalPort, protocol, port_mapping.MappingAddTypeManual); err != nil {
		as.logger.WithError(err).Warn("删除端口映射失败")
	}

	as.logger.WithFields(logrus.Fields{
		"internal_port": internalPort,
		"external_port": externalPort,
		"protocol":      protocol,
	}).Info("成功删除手动映射")

	return nil
}

// GetPortMappings 获取所有端口映射
func (as *AutoUPnPService) GetPortMappings(addType string) []port_mapping.PortMapping {
	if as.portMappingManager == nil {
		return []port_mapping.PortMapping{}
	}
	allMappings := as.portMappingManager.GetMappings()
	allMappingsList := []port_mapping.PortMapping{}
	for _, mapping := range allMappings {
		allMappingsList = append(allMappingsList, *mapping)
	}
	switch addType {
	case "":
		return allMappingsList
	case "auto":
		autoMappings := []port_mapping.PortMapping{}
		for _, mapping := range allMappings {
			if mapping.AddType == port_mapping.MappingAddTypeAuto {
				autoMappings = append(autoMappings, *mapping)
			}
		}
		return autoMappings
	case "manual":
		manualMappings := []port_mapping.PortMapping{}
		for _, mapping := range allMappings {
			if mapping.AddType == port_mapping.MappingAddTypeManual {
				manualMappings = append(manualMappings, *mapping)
			}
		}
		return manualMappings
	default:
		return []port_mapping.PortMapping{}
	}
}

// GetActivePorts 获取活跃端口列表
func (as *AutoUPnPService) GetActivePorts() []int {
	if as.autoPortMonitor == nil {
		return []int{}
	}
	return as.autoPortMonitor.GetActivePorts()
}
