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
	config         *config.Config
	logger         *logrus.Logger
	portMonitor    *portmonitor.PortMonitor
	upnpManager    *upnp.UPnPManager
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	activeMappings map[int]bool
	mappingMutex   sync.RWMutex
}

// NewAutoUPnPService 创建新的自动UPnP服务
func NewAutoUPnPService(cfg *config.Config, logger *logrus.Logger) *AutoUPnPService {
	ctx, cancel := context.WithCancel(context.Background())

	return &AutoUPnPService{
		config:         cfg,
		logger:         logger,
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
		DiscoveryTimeout: as.config.UPnP.DiscoveryTimeout,
		MappingDuration:  as.config.UPnP.MappingDuration,
		RetryAttempts:    as.config.UPnP.RetryAttempts,
		RetryDelay:       as.config.UPnP.RetryDelay,
		MaxMappings:      as.config.Monitor.MaxMappings,
	}

	as.upnpManager = upnp.NewUPnPManager(upnpConfig, as.logger)

	// 发现UPnP设备
	if err := as.upnpManager.Discover(); err != nil {
		return fmt.Errorf("UPnP设备发现失败: %w", err)
	}

	// 初始化端口监控器
	portConfig := &portmonitor.Config{
		CheckInterval: as.config.Monitor.CheckInterval,
		PortRange:     as.config.GetPortRange(),
		Timeout:       5 * time.Second,
	}

	as.portMonitor = portmonitor.NewPortMonitor(portConfig, as.logger)

	// 添加端口状态变化回调
	as.portMonitor.AddCallback(as.onPortStatusChanged)

	// 启动端口监控
	as.portMonitor.Start()

	// 启动清理协程
	as.wg.Add(1)
	go as.cleanupRoutine()

	as.logger.Info("自动UPnP服务启动完成")
	return nil
}

// Stop 停止自动UPnP服务
func (as *AutoUPnPService) Stop() {
	as.logger.Info("停止自动UPnP服务")

	// 停止端口监控
	if as.portMonitor != nil {
		as.portMonitor.Stop()
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

// onPortStatusChanged 端口状态变化回调
func (as *AutoUPnPService) onPortStatusChanged(port int, isActive bool) {
	as.mappingMutex.Lock()
	defer as.mappingMutex.Unlock()

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
		status, exists := as.portMonitor.GetPortStatus(port)
		if !exists || !status.IsActive {
			as.logger.WithField("port", port).Info("清理非活跃的端口映射记录")
			delete(as.activeMappings, port)
		}
	}
}

// GetStatus 获取服务状态
func (as *AutoUPnPService) GetStatus() map[string]interface{} {
	as.mappingMutex.RLock()
	defer as.mappingMutex.RUnlock()

	// 获取端口状态
	portStatus := as.portMonitor.GetAllPortStatus()
	activePorts := as.portMonitor.GetActivePorts()
	inactivePorts := as.portMonitor.GetInactivePorts()

	// 获取UPnP映射状态
	upnpMappings := as.upnpManager.GetPortMappings()

	// 构建活跃映射列表
	var activeMappings []int
	for port := range as.activeMappings {
		activeMappings = append(activeMappings, port)
	}

	return map[string]interface{}{
		"service_status": "running",
		"port_range": map[string]interface{}{
			"start": as.config.PortRange.Start,
			"end":   as.config.PortRange.End,
			"step":  as.config.PortRange.Step,
		},
		"port_status": map[string]interface{}{
			"total_ports":         len(portStatus),
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
		"config": map[string]interface{}{
			"check_interval":   as.config.Monitor.CheckInterval.String(),
			"cleanup_interval": as.config.Monitor.CleanupInterval.String(),
			"mapping_duration": as.config.UPnP.MappingDuration.String(),
			"max_mappings":     as.config.Monitor.MaxMappings,
		},
	}
}

// AddManualMapping 手动添加端口映射
func (as *AutoUPnPService) AddManualMapping(internalPort, externalPort int, protocol, description string) error {
	if description == "" {
		description = fmt.Sprintf("Manual-%d", internalPort)
	}

	return as.upnpManager.AddPortMapping(internalPort, externalPort, protocol, description)
}

// RemoveManualMapping 手动删除端口映射
func (as *AutoUPnPService) RemoveManualMapping(internalPort, externalPort int, protocol string) error {
	return as.upnpManager.RemovePortMapping(internalPort, externalPort, protocol)
}

// GetPortMappings 获取所有端口映射
func (as *AutoUPnPService) GetPortMappings() map[string]*upnp.PortMapping {
	return as.upnpManager.GetPortMappings()
}

// GetActivePorts 获取活跃端口列表
func (as *AutoUPnPService) GetActivePorts() []int {
	return as.portMonitor.GetActivePorts()
}

// GetInactivePorts 获取非活跃端口列表
func (as *AutoUPnPService) GetInactivePorts() []int {
	return as.portMonitor.GetInactivePorts()
}
