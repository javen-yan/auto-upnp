package portmonitor

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// PortStatus 端口状态
type PortStatus struct {
	Port     int
	IsActive bool
	LastSeen time.Time
}

// PortMonitor 端口监控器
type PortMonitor struct {
	config     *Config
	logger     *logrus.Logger
	portStatus map[int]*PortStatus
	mutex      sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	callbacks  []PortStatusCallback
}

// Config 端口监控配置
type Config struct {
	CheckInterval time.Duration
	PortRange     []int
	Timeout       time.Duration
}

// PortStatusCallback 端口状态变化回调函数
type PortStatusCallback func(port int, isActive bool)

// NewPortMonitor 创建新的端口监控器
func NewPortMonitor(config *Config, logger *logrus.Logger) *PortMonitor {
	ctx, cancel := context.WithCancel(context.Background())

	return &PortMonitor{
		config:     config,
		logger:     logger,
		portStatus: make(map[int]*PortStatus),
		ctx:        ctx,
		cancel:     cancel,
		callbacks:  make([]PortStatusCallback, 0),
	}
}

// AddCallback 添加端口状态变化回调
func (pm *PortMonitor) AddCallback(callback PortStatusCallback) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	pm.callbacks = append(pm.callbacks, callback)
}

// Start 启动端口监控
func (pm *PortMonitor) Start() {
	pm.logger.Info("启动端口监控器")

	// 初始化端口状态
	for _, port := range pm.config.PortRange {
		pm.portStatus[port] = &PortStatus{
			Port:     port,
			IsActive: false,
			LastSeen: time.Time{},
		}
	}

	// 启动监控协程
	go pm.monitorLoop()
}

// Stop 停止端口监控
func (pm *PortMonitor) Stop() {
	pm.logger.Info("停止端口监控器")
	pm.cancel()
}

// monitorLoop 监控循环
func (pm *PortMonitor) monitorLoop() {
	ticker := time.NewTicker(pm.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-pm.ctx.Done():
			return
		case <-ticker.C:
			pm.checkAllPorts()
		}
	}
}

// checkAllPorts 检查所有端口状态
func (pm *PortMonitor) checkAllPorts() {
	var wg sync.WaitGroup

	for _, port := range pm.config.PortRange {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			pm.checkPort(p)
		}(port)
	}

	wg.Wait()
}

// checkPort 检查单个端口状态
func (pm *PortMonitor) checkPort(port int) {
	isActive := pm.isPortActive(port)

	pm.mutex.Lock()
	status, exists := pm.portStatus[port]
	if !exists {
		status = &PortStatus{
			Port:     port,
			IsActive: false,
			LastSeen: time.Time{},
		}
		pm.portStatus[port] = status
	}

	// 检查状态是否发生变化
	statusChanged := status.IsActive != isActive

	if isActive {
		status.LastSeen = time.Now()
	}

	status.IsActive = isActive
	pm.mutex.Unlock()

	// 如果状态发生变化，触发回调
	if statusChanged {
		pm.logger.WithFields(logrus.Fields{
			"port":     port,
			"isActive": isActive,
		}).Info("端口状态发生变化")

		pm.triggerCallbacks(port, isActive)
	}
}

// isPortActive 检查端口是否活跃
func (pm *PortMonitor) isPortActive(port int) bool {
	address := fmt.Sprintf(":%d", port)

	// 尝试监听端口
	listener, err := net.Listen("tcp", address)
	if err != nil {
		// 端口被占用，说明有服务在运行
		return true
	}

	// 端口可用，关闭监听器
	listener.Close()
	return false
}

// triggerCallbacks 触发回调函数
func (pm *PortMonitor) triggerCallbacks(port int, isActive bool) {
	pm.mutex.RLock()
	callbacks := make([]PortStatusCallback, len(pm.callbacks))
	copy(callbacks, pm.callbacks)
	pm.mutex.RUnlock()

	for _, callback := range callbacks {
		go func(cb PortStatusCallback) {
			defer func() {
				if r := recover(); r != nil {
					pm.logger.WithField("error", r).Error("端口状态回调函数执行出错")
				}
			}()
			cb(port, isActive)
		}(callback)
	}
}

// GetPortStatus 获取端口状态
func (pm *PortMonitor) GetPortStatus(port int) (*PortStatus, bool) {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	status, exists := pm.portStatus[port]
	return status, exists
}

// GetAllPortStatus 获取所有端口状态
func (pm *PortMonitor) GetAllPortStatus() map[int]*PortStatus {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	result := make(map[int]*PortStatus)
	for port, status := range pm.portStatus {
		result[port] = &PortStatus{
			Port:     status.Port,
			IsActive: status.IsActive,
			LastSeen: status.LastSeen,
		}
	}

	return result
}

// GetActivePorts 获取活跃端口列表
func (pm *PortMonitor) GetActivePorts() []int {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	var activePorts []int
	for port, status := range pm.portStatus {
		if status.IsActive {
			activePorts = append(activePorts, port)
		}
	}

	return activePorts
}

// GetInactivePorts 获取非活跃端口列表
func (pm *PortMonitor) GetInactivePorts() []int {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	var inactivePorts []int
	for port, status := range pm.portStatus {
		if !status.IsActive {
			inactivePorts = append(inactivePorts, port)
		}
	}

	return inactivePorts
}
