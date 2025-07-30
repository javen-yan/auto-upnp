package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"auto-upnp/config"
	"auto-upnp/internal/portmonitor"
	"auto-upnp/internal/upnp"

	"github.com/sirupsen/logrus"
)

// AutoUPnPService 自动UPnP服务
type AutoUPnPService struct {
	config            *config.Config
	logger            *logrus.Logger
	autoPortMonitor   *portmonitor.AutoPortMonitor
	manualPortMonitor *portmonitor.ManualPortMonitor
	upnpManager       *upnp.UPnPManager
	manualManager     *ManualMappingManager
	ctx               context.Context
	cancel            context.CancelFunc
	wg                sync.WaitGroup
	activeMappings    map[int]bool
	mappingMutex      sync.RWMutex
}

// NewAutoUPnPService 创建新的自动UPnP服务
func NewAutoUPnPService(cfg *config.Config, logger *logrus.Logger) *AutoUPnPService {
	ctx, cancel := context.WithCancel(context.Background())

	// 创建手动映射管理器，使用admin.data_dir
	manualManager := NewManualMappingManager(cfg.Admin.DataDir, logger)

	return &AutoUPnPService{
		config:         cfg,
		logger:         logger,
		manualManager:  manualManager,
		ctx:            ctx,
		cancel:         cancel,
		activeMappings: make(map[int]bool),
	}
}

// Start 启动自动UPnP服务
func (as *AutoUPnPService) Start() error {
	as.logger.Info("启动自动UPnP服务")

	// 初始化UPnP管理器
	upnpConfig := &upnp.Config{
		DiscoveryTimeout:    as.config.UPnP.DiscoveryTimeout,
		MappingDuration:     as.config.UPnP.MappingDuration,
		RetryAttempts:       as.config.UPnP.RetryAttempts,
		RetryDelay:          as.config.UPnP.RetryDelay,
		MaxMappings:         as.config.Monitor.MaxMappings,
		HealthCheckInterval: as.config.UPnP.HealthCheckInterval,
		MaxFailCount:        as.config.UPnP.MaxFailCount,
		KeepAliveInterval:   as.config.UPnP.KeepAliveInterval,
	}

	as.upnpManager = upnp.NewUPnPManager(upnpConfig, as.logger)

	// 发现UPnP设备
	if err := as.upnpManager.Discover(); err != nil {
		as.logger.WithError(err).Warn("UPnP设备发现失败，将在后台继续尝试")
		// 不返回错误，继续运行服务
	}

	timeout := as.config.Monitor.CheckInterval

	// 初始化自动端口监控器
	autoPortConfig := &portmonitor.Config{
		CheckInterval: as.config.Monitor.CheckInterval,
		PortRange:     as.config.GetPortRange(),
		Timeout:       timeout,
	}

	as.autoPortMonitor = portmonitor.NewAutoPortMonitor(autoPortConfig, as.logger)

	// 添加自动端口状态变化回调
	as.autoPortMonitor.AddCallback(as.onAutoPortStatusChanged)

	// 启动自动端口监控
	as.autoPortMonitor.Start()

	// 初始化手动端口监控器
	as.manualPortMonitor = portmonitor.NewManualPortMonitor(
		as.config.Monitor.CheckInterval,
		timeout,
		as.logger,
	)

	// 添加手动端口状态变化回调
	as.manualPortMonitor.AddCallback(as.onManualPortStatusChanged)

	// 启动手动端口监控
	as.manualPortMonitor.Start()

	// 启动清理协程
	as.wg.Add(1)
	go as.cleanupRoutine()

	// 启动UPnP重试协程
	as.wg.Add(1)
	go as.upnpRetryRoutine()

	// 加载并恢复手动映射
	if err := as.restoreManualMappings(); err != nil {
		as.logger.WithError(err).Warn("恢复手动映射失败")
	}

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

	// 停止手动端口监控
	if as.manualPortMonitor != nil {
		as.manualPortMonitor.Stop()
	}

	// 取消上下文
	as.cancel()

	// 等待所有协程完成
	as.wg.Wait()

	// 关闭UPnP管理器
	if as.upnpManager != nil {
		as.upnpManager.Close()
	}

	as.logger.Info("自动UPnP服务已停止")
}

// onAutoPortStatusChanged 自动端口状态变化回调
func (as *AutoUPnPService) onAutoPortStatusChanged(port int, isActive bool) {
	as.mappingMutex.Lock()
	defer as.mappingMutex.Unlock()

	// 处理自动映射
	if isActive {
		// 端口变为活跃状态，添加UPnP映射
		if !as.activeMappings[port] {
			as.logger.WithField("port", port).Info("检测到自动端口上线，添加UPnP映射")

			description := fmt.Sprintf("AutoUPnP-%d", port)
			err := as.upnpManager.AddPortMapping(port, port, "TCP", description)
			if err != nil {
				as.logger.WithFields(logrus.Fields{
					"port":  port,
					"error": err,
				}).Error("添加自动UPnP端口映射失败")
				return
			}

			as.activeMappings[port] = true
			as.logger.WithField("port", port).Info("自动UPnP端口映射添加成功")
		}
	} else {
		// 端口变为非活跃状态，删除UPnP映射
		if as.activeMappings[port] {
			as.logger.WithField("port", port).Info("检测到自动端口下线，删除UPnP映射")

			err := as.upnpManager.RemovePortMapping(port, port, "TCP")
			if err != nil {
				as.logger.WithFields(logrus.Fields{
					"port":  port,
					"error": err,
				}).Error("删除自动UPnP端口映射失败")
				return
			}

			delete(as.activeMappings, port)
			as.logger.WithField("port", port).Info("自动UPnP端口映射删除成功")
		}
	}
}

// onManualPortStatusChanged 手动端口状态变化回调
func (as *AutoUPnPService) onManualPortStatusChanged(port int, isActive bool, protocol string) {
	// 处理手动映射的激活状态
	as.handleManualMappingStatus(port, isActive)
}

// onPortStatusChanged 端口状态变化回调（保持兼容性）
func (as *AutoUPnPService) onPortStatusChanged(port int, isActive bool) {
	as.mappingMutex.Lock()
	defer as.mappingMutex.Unlock()

	// 处理自动映射
	if isActive {
		// 端口变为活跃状态，添加UPnP映射
		if !as.activeMappings[port] {
			as.logger.WithField("port", port).Info("检测到端口上线，添加UPnP映射")

			description := fmt.Sprintf("AutoUPnP-%d", port)
			err := as.upnpManager.AddPortMapping(port, port, "TCP", description)
			if err != nil {
				as.logger.WithFields(logrus.Fields{
					"port":  port,
					"error": err,
				}).Error("添加UPnP端口映射失败")
				return
			}

			as.activeMappings[port] = true
			as.logger.WithField("port", port).Info("UPnP端口映射添加成功")
		}
	} else {
		// 端口变为非活跃状态，删除UPnP映射
		if as.activeMappings[port] {
			as.logger.WithField("port", port).Info("检测到端口下线，删除UPnP映射")

			err := as.upnpManager.RemovePortMapping(port, port, "TCP")
			if err != nil {
				as.logger.WithFields(logrus.Fields{
					"port":  port,
					"error": err,
				}).Error("删除UPnP端口映射失败")
				return
			}

			delete(as.activeMappings, port)
			as.logger.WithField("port", port).Info("UPnP端口映射删除成功")
		}
	}

	// 处理手动映射的激活状态
	as.handleManualMappingStatus(port, isActive)
}

// handleManualMappingStatus 处理手动映射的状态变化
func (as *AutoUPnPService) handleManualMappingStatus(port int, isActive bool) {
	// 获取所有手动映射
	manualMappings := as.manualManager.GetMappings()

	for _, mapping := range manualMappings {
		if mapping.InternalPort == port {
			// 更新映射的激活状态
			err := as.manualManager.UpdateMappingActiveStatus(
				mapping.InternalPort,
				mapping.ExternalPort,
				mapping.Protocol,
				isActive,
			)

			if err != nil {
				as.logger.WithFields(logrus.Fields{
					"port":    port,
					"mapping": mapping,
					"error":   err,
				}).Error("更新手动映射激活状态失败")
				continue
			}

			// 如果端口上线且映射之前是非激活状态，尝试重新注册UPnP映射
			if isActive && !mapping.Active {
				as.logger.WithFields(logrus.Fields{
					"internal_port": mapping.InternalPort,
					"external_port": mapping.ExternalPort,
					"protocol":      mapping.Protocol,
				}).Info("手动映射端口恢复，重新注册UPnP映射")

				err := as.upnpManager.AddPortMapping(
					mapping.InternalPort,
					mapping.ExternalPort,
					mapping.Protocol,
					mapping.Description,
				)
				if err != nil {
					as.logger.WithFields(logrus.Fields{
						"internal_port": mapping.InternalPort,
						"external_port": mapping.ExternalPort,
						"protocol":      mapping.Protocol,
						"error":         err,
					}).Error("重新注册手动映射UPnP失败")
				} else {
					as.logger.WithFields(logrus.Fields{
						"internal_port": mapping.InternalPort,
						"external_port": mapping.ExternalPort,
						"protocol":      mapping.Protocol,
					}).Info("手动映射UPnP重新注册成功")
				}
			}

			// 如果端口下线且映射之前是激活状态，取消UPnP映射
			if !isActive && mapping.Active {
				as.logger.WithFields(logrus.Fields{
					"internal_port": mapping.InternalPort,
					"external_port": mapping.ExternalPort,
					"protocol":      mapping.Protocol,
				}).Info("手动映射端口下线，取消UPnP映射")

				err := as.upnpManager.RemovePortMapping(
					mapping.InternalPort,
					mapping.ExternalPort,
					mapping.Protocol,
				)
				if err != nil {
					as.logger.WithFields(logrus.Fields{
						"internal_port": mapping.InternalPort,
						"external_port": mapping.ExternalPort,
						"protocol":      mapping.Protocol,
						"error":         err,
					}).Error("取消手动映射UPnP失败")
				} else {
					as.logger.WithFields(logrus.Fields{
						"internal_port": mapping.InternalPort,
						"external_port": mapping.ExternalPort,
						"protocol":      mapping.Protocol,
					}).Info("手动映射UPnP取消成功")
				}
			}
		}
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

	// 清理UPnP管理器中的过期映射
	as.upnpManager.CleanupExpiredMappings()

	// 检查本地记录的映射状态
	as.mappingMutex.Lock()
	defer as.mappingMutex.Unlock()

	for port := range as.activeMappings {
		// 检查端口是否仍然活跃
		status, exists := as.autoPortMonitor.GetPortStatus(port)
		if !exists || !status.IsActive {
			as.logger.WithField("port", port).Info("清理非活跃的自动端口映射记录")
			delete(as.activeMappings, port)
		}
	}
}

// upnpRetryRoutine UPnP重试协程
func (as *AutoUPnPService) upnpRetryRoutine() {
	defer as.wg.Done()

	// 每5分钟尝试重新发现UPnP设备
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-as.ctx.Done():
			return
		case <-ticker.C:
			// 检查是否有活跃的端口映射需要处理
			activePorts := as.autoPortMonitor.GetActivePorts()
			if len(activePorts) > 0 {
				as.logger.Info("检测到活跃端口，尝试重新发现UPnP设备")
				if err := as.upnpManager.Discover(); err != nil {
					as.logger.WithError(err).Debug("UPnP设备发现失败，继续等待")
				} else {
					as.logger.Info("UPnP设备重新发现成功")
				}
			}
		}
	}
}

// GetStatus 获取服务状态
func (as *AutoUPnPService) GetStatus() map[string]interface{} {
	as.mappingMutex.RLock()
	defer as.mappingMutex.RUnlock()

	// 获取端口状态
	var autoPortStatus map[int]*portmonitor.AutoPortStatus
	var activePorts []int
	var inactivePorts []int

	if as.autoPortMonitor != nil {
		autoPortStatus = as.autoPortMonitor.GetAllPortStatus()
		activePorts = as.autoPortMonitor.GetActivePorts()
		inactivePorts = as.autoPortMonitor.GetInactivePorts()
	} else {
		autoPortStatus = make(map[int]*portmonitor.AutoPortStatus)
		activePorts = []int{}
		inactivePorts = []int{}
	}

	// 获取UPnP映射状态
	var upnpMappings map[string]*upnp.PortMapping
	if as.upnpManager != nil {
		upnpMappings = as.upnpManager.GetPortMappings()
	} else {
		upnpMappings = make(map[string]*upnp.PortMapping)
	}

	// 构建活跃映射列表
	var activeMappings []int
	for port := range as.activeMappings {
		activeMappings = append(activeMappings, port)
	}

	// 获取手动映射信息
	var manualMappings []*ManualMapping
	var activeManualMappings []*ManualMapping
	var inactiveManualMappings []*ManualMapping
	if as.manualManager != nil {
		manualMappings = as.manualManager.GetMappings()
		activeManualMappings = as.manualManager.GetActiveMappings()
		inactiveManualMappings = as.manualManager.GetInactiveMappings()
	} else {
		manualMappings = []*ManualMapping{}
		activeManualMappings = []*ManualMapping{}
		inactiveManualMappings = []*ManualMapping{}
	}

	// 获取UPnP客户端数量
	var upnpClientCount int
	if as.upnpManager != nil {
		upnpClientCount = as.upnpManager.GetClientCount()
	} else {
		upnpClientCount = 0
	}

	return map[string]interface{}{
		"service_status": "running",
		"port_range": map[string]interface{}{
			"start": as.config.PortRange.Start,
			"end":   as.config.PortRange.End,
			"step":  as.config.PortRange.Step,
		},
		"port_status": map[string]interface{}{
			"total_ports":         len(autoPortStatus),
			"active_ports":        len(activePorts),
			"inactive_ports":      len(inactivePorts),
			"active_ports_list":   activePorts,
			"inactive_ports_list": inactivePorts,
		},
		"upnp_mappings": map[string]interface{}{
			"total_mappings":  len(upnpMappings),
			"active_mappings": activeMappings,
			"mappings":        upnpMappings,
		},
		"manual_mappings": map[string]interface{}{
			"total_mappings":         len(manualMappings),
			"active_mappings":        len(activeManualMappings),
			"inactive_mappings":      len(inactiveManualMappings),
			"mappings":               manualMappings,
			"active_mappings_list":   activeManualMappings,
			"inactive_mappings_list": inactiveManualMappings,
		},
		"upnp_status": map[string]interface{}{
			"client_count": upnpClientCount,
			"available":    upnpClientCount > 0,
			"discovered":   as.upnpManager != nil && len(upnpMappings) > 0,
		},
		"config": map[string]interface{}{
			"check_interval":   as.config.Monitor.CheckInterval.String(),
			"cleanup_interval": as.config.Monitor.CleanupInterval.String(),
			"mapping_duration": as.config.UPnP.MappingDuration.String(),
			"max_mappings":     as.config.Monitor.MaxMappings,
		},
	}
}

// restoreManualMappings 恢复手动映射
func (as *AutoUPnPService) restoreManualMappings() error {
	// 加载手动映射文件
	if err := as.manualManager.LoadMappings(); err != nil {
		return fmt.Errorf("加载手动映射失败: %w", err)
	}

	// 获取所有手动映射
	mappings := as.manualManager.GetMappings()
	if len(mappings) == 0 {
		as.logger.Info("没有需要恢复的手动映射")
		return nil
	}

	as.logger.Infof("开始恢复 %d 个手动映射", len(mappings))

	// 恢复每个映射
	for _, mapping := range mappings {
		// 检查端口当前状态
		var isPortActive bool
		if as.manualPortMonitor != nil {
			status, exists := as.manualPortMonitor.GetPortStatus(mapping.InternalPort)
			isPortActive = exists && status.IsActive
		}

		// 更新映射的激活状态
		if err := as.manualManager.UpdateMappingActiveStatus(
			mapping.InternalPort,
			mapping.ExternalPort,
			mapping.Protocol,
			isPortActive,
		); err != nil {
			as.logger.WithError(err).WithFields(logrus.Fields{
				"internal_port": mapping.InternalPort,
				"external_port": mapping.ExternalPort,
				"protocol":      mapping.Protocol,
			}).Warn("更新手动映射激活状态失败")
		}

		// 添加到手动端口监控器
		if as.manualPortMonitor != nil {
			as.manualPortMonitor.AddPort(mapping.InternalPort, mapping.Protocol)
		}

		// 只有当端口活跃时才注册UPnP映射
		if isPortActive {
			if err := as.upnpManager.AddPortMapping(
				mapping.InternalPort,
				mapping.ExternalPort,
				mapping.Protocol,
				mapping.Description,
			); err != nil {
				as.logger.WithError(err).WithFields(logrus.Fields{
					"internal_port": mapping.InternalPort,
					"external_port": mapping.ExternalPort,
					"protocol":      mapping.Protocol,
				}).Warn("恢复手动映射UPnP失败")
			} else {
				as.logger.WithFields(logrus.Fields{
					"internal_port": mapping.InternalPort,
					"external_port": mapping.ExternalPort,
					"protocol":      mapping.Protocol,
					"active":        isPortActive,
				}).Info("成功恢复手动映射")
			}
		} else {
			as.logger.WithFields(logrus.Fields{
				"internal_port": mapping.InternalPort,
				"external_port": mapping.ExternalPort,
				"protocol":      mapping.Protocol,
				"active":        isPortActive,
			}).Info("手动映射端口非活跃，等待端口上线")
		}
	}

	return nil
}

// AddManualMapping 手动添加端口映射
func (as *AutoUPnPService) AddManualMapping(internalPort, externalPort int, protocol, description string) error {
	if description == "" {
		description = fmt.Sprintf("Manual-%d", internalPort)
	}

	// 检查端口当前状态
	var isPortActive bool
	if as.manualPortMonitor != nil {
		status, exists := as.manualPortMonitor.GetPortStatus(internalPort)
		isPortActive = exists && status.IsActive
	}

	// 保存到手动映射管理器（包含激活状态）
	if err := as.manualManager.AddMapping(internalPort, externalPort, protocol, description); err != nil {
		return err
	}

	// 更新激活状态
	if err := as.manualManager.UpdateMappingActiveStatus(internalPort, externalPort, protocol, isPortActive); err != nil {
		as.logger.WithError(err).Warn("更新手动映射激活状态失败")
	}

	// 添加到手动端口监控器
	if as.manualPortMonitor != nil {
		as.manualPortMonitor.AddPort(internalPort, protocol)
	}

	// 只有当端口活跃时才添加到UPnP管理器
	if isPortActive {
		if err := as.upnpManager.AddPortMapping(internalPort, externalPort, protocol, description); err != nil {
			as.logger.WithError(err).Warn("添加UPnP映射失败，但已保存手动映射")
			return err
		}
		as.logger.WithFields(logrus.Fields{
			"internal_port": internalPort,
			"external_port": externalPort,
			"protocol":      protocol,
			"active":        isPortActive,
		}).Info("成功添加手动映射并注册UPnP")
	} else {
		as.logger.WithFields(logrus.Fields{
			"internal_port": internalPort,
			"external_port": externalPort,
			"protocol":      protocol,
			"active":        isPortActive,
		}).Info("添加手动映射，等待端口上线")
	}

	return nil
}

// RemoveManualMapping 手动删除端口映射
func (as *AutoUPnPService) RemoveManualMapping(internalPort, externalPort int, protocol string) error {
	// 从UPnP管理器中删除（如果存在）
	if err := as.upnpManager.RemovePortMapping(internalPort, externalPort, protocol); err != nil {
		as.logger.WithError(err).Warn("删除UPnP映射失败，但继续删除手动映射")
	}

	// 从手动映射管理器中删除
	if err := as.manualManager.RemoveMapping(internalPort, externalPort, protocol); err != nil {
		return err
	}

	// 从手动端口监控器中移除
	if as.manualPortMonitor != nil {
		as.manualPortMonitor.RemovePort(internalPort)
	}

	as.logger.WithFields(logrus.Fields{
		"internal_port": internalPort,
		"external_port": externalPort,
		"protocol":      protocol,
	}).Info("成功删除手动映射")

	return nil
}

// GetPortMappings 获取所有端口映射
func (as *AutoUPnPService) GetPortMappings() map[string]*upnp.PortMapping {
	return as.upnpManager.GetPortMappings()
}

// GetActivePorts 获取活跃端口列表
func (as *AutoUPnPService) GetActivePorts() []int {
	if as.autoPortMonitor == nil {
		return []int{}
	}
	return as.autoPortMonitor.GetActivePorts()
}

// GetInactivePorts 获取非活跃端口列表
func (as *AutoUPnPService) GetInactivePorts() []int {
	if as.autoPortMonitor == nil {
		return []int{}
	}
	return as.autoPortMonitor.GetInactivePorts()
}

// GetManualMappings 获取手动映射列表
func (as *AutoUPnPService) GetManualMappings() []*ManualMapping {
	if as.manualManager == nil {
		return []*ManualMapping{}
	}
	return as.manualManager.GetMappings()
}

// GetActiveManualMappings 获取激活的手动映射列表
func (as *AutoUPnPService) GetActiveManualMappings() []*ManualMapping {
	if as.manualManager == nil {
		return []*ManualMapping{}
	}
	return as.manualManager.GetActiveMappings()
}

// GetInactiveManualMappings 获取非激活的手动映射列表
func (as *AutoUPnPService) GetInactiveManualMappings() []*ManualMapping {
	if as.manualManager == nil {
		return []*ManualMapping{}
	}
	return as.manualManager.GetInactiveMappings()
}

// GetUPnPClientCount 获取UPnP客户端数量
func (as *AutoUPnPService) GetUPnPClientCount() int {
	if as.upnpManager == nil {
		return 0
	}
	return as.upnpManager.GetClientCount()
}

// IsUPnPAvailable 检查UPnP服务是否可用
func (as *AutoUPnPService) IsUPnPAvailable() bool {
	return as.GetUPnPClientCount() > 0
}
