package portmonitor

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// AutoPortStatus 自动端口状态
type AutoPortStatus struct {
	Port     int
	IsActive bool
	LastSeen time.Time
}

// AutoPortMonitor 自动端口监控器
type AutoPortMonitor struct {
	config     *Config
	logger     *logrus.Logger
	portStatus map[int]*AutoPortStatus
	mutex      sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	callbacks  []AutoPortStatusCallback
}

// Config 自动端口监控配置
type Config struct {
	CheckInterval time.Duration
	PortRange     []int
	Timeout       time.Duration
}

// AutoPortStatusCallback 自动端口状态变化回调函数
type AutoPortStatusCallback func(port int, isActive bool)

// NewAutoPortMonitor 创建新的自动端口监控器
func NewAutoPortMonitor(config *Config, logger *logrus.Logger) *AutoPortMonitor {
	ctx, cancel := context.WithCancel(context.Background())

	return &AutoPortMonitor{
		config:     config,
		logger:     logger,
		portStatus: make(map[int]*AutoPortStatus),
		ctx:        ctx,
		cancel:     cancel,
		callbacks:  make([]AutoPortStatusCallback, 0),
	}
}

// AddCallback 添加端口状态变化回调
func (apm *AutoPortMonitor) AddCallback(callback AutoPortStatusCallback) {
	apm.mutex.Lock()
	defer apm.mutex.Unlock()
	apm.callbacks = append(apm.callbacks, callback)
}

// Start 启动自动端口监控
func (apm *AutoPortMonitor) Start() {
	apm.logger.Info("启动自动端口监控器")

	// 初始化端口状态
	for _, port := range apm.config.PortRange {
		apm.portStatus[port] = &AutoPortStatus{
			Port:     port,
			IsActive: false,
			LastSeen: time.Time{},
		}
	}

	// 启动监控协程
	go apm.monitorLoop()
}

// Stop 停止自动端口监控
func (apm *AutoPortMonitor) Stop() {
	apm.logger.Info("停止自动端口监控器")
	apm.cancel()
}

// monitorLoop 监控循环
func (apm *AutoPortMonitor) monitorLoop() {
	ticker := time.NewTicker(apm.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-apm.ctx.Done():
			return
		case <-ticker.C:
			apm.checkAllPorts()
		}
	}
}

// checkAllPorts 检查所有端口状态
func (apm *AutoPortMonitor) checkAllPorts() {
	var wg sync.WaitGroup

	for _, port := range apm.config.PortRange {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			apm.checkPort(p)
		}(port)
	}

	wg.Wait()
}

// checkPort 检查单个端口状态
func (apm *AutoPortMonitor) checkPort(port int) {
	isActive := apm.isPortActive(port)

	apm.mutex.Lock()
	status, exists := apm.portStatus[port]
	if !exists {
		status = &AutoPortStatus{
			Port:     port,
			IsActive: false,
			LastSeen: time.Time{},
		}
		apm.portStatus[port] = status
	}

	// 检查状态是否发生变化
	statusChanged := status.IsActive != isActive

	if isActive {
		status.LastSeen = time.Now()
	}

	status.IsActive = isActive
	apm.mutex.Unlock()

	// 如果状态发生变化，触发回调
	if statusChanged {
		apm.logger.WithFields(logrus.Fields{
			"port":     port,
			"isActive": isActive,
		}).Info("自动端口状态发生变化")

		apm.triggerCallbacks(port, isActive)
	}
}

// isPortActive 检查端口是否活跃
func (apm *AutoPortMonitor) isPortActive(port int) bool {
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
func (apm *AutoPortMonitor) triggerCallbacks(port int, isActive bool) {
	apm.mutex.RLock()
	callbacks := make([]AutoPortStatusCallback, len(apm.callbacks))
	copy(callbacks, apm.callbacks)
	apm.mutex.RUnlock()

	for _, callback := range callbacks {
		go func(cb AutoPortStatusCallback) {
			defer func() {
				if r := recover(); r != nil {
					apm.logger.WithField("error", r).Error("自动端口状态回调函数执行出错")
				}
			}()
			cb(port, isActive)
		}(callback)
	}
}

// GetPortStatus 获取端口状态
func (apm *AutoPortMonitor) GetPortStatus(port int) (*AutoPortStatus, bool) {
	apm.mutex.RLock()
	defer apm.mutex.RUnlock()

	status, exists := apm.portStatus[port]
	if !exists {
		return nil, false
	}

	// 返回副本
	return &AutoPortStatus{
		Port:     status.Port,
		IsActive: status.IsActive,
		LastSeen: status.LastSeen,
	}, true
}

// GetAllPortStatus 获取所有端口状态
func (apm *AutoPortMonitor) GetAllPortStatus() map[int]*AutoPortStatus {
	apm.mutex.RLock()
	defer apm.mutex.RUnlock()

	result := make(map[int]*AutoPortStatus)
	for port, status := range apm.portStatus {
		result[port] = &AutoPortStatus{
			Port:     status.Port,
			IsActive: status.IsActive,
			LastSeen: status.LastSeen,
		}
	}

	return result
}

// GetActivePorts 获取活跃端口列表
func (apm *AutoPortMonitor) GetActivePorts() []int {
	apm.mutex.RLock()
	defer apm.mutex.RUnlock()

	var activePorts []int
	for port, status := range apm.portStatus {
		if status.IsActive {
			activePorts = append(activePorts, port)
		}
	}

	return activePorts
}

// GetInactivePorts 获取非活跃端口列表
func (apm *AutoPortMonitor) GetInactivePorts() []int {
	apm.mutex.RLock()
	defer apm.mutex.RUnlock()

	var inactivePorts []int
	for port, status := range apm.portStatus {
		if !status.IsActive {
			inactivePorts = append(inactivePorts, port)
		}
	}

	return inactivePorts
}

// GetMonitoredPorts 获取所有被监控的端口
func (apm *AutoPortMonitor) GetMonitoredPorts() []int {
	apm.mutex.RLock()
	defer apm.mutex.RUnlock()

	ports := make([]int, 0, len(apm.portStatus))
	for port := range apm.portStatus {
		ports = append(ports, port)
	}

	return ports
}
