package portmonitor

import (
	"context"
	"sync"
	"time"

	"auto-upnp/internal/util"

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

	// 添加对象池
	statusPool sync.Pool
}

// Config 自动端口监控配置
type Config struct {
	CheckInterval time.Duration
	PortRange     []int
	Timeout       time.Duration
	EnablePool    bool // 是否启用对象池
}

// AutoPortStatusCallback 自动端口状态变化回调函数
type AutoPortStatusCallback func(port int, isActive bool, protocol util.ProtocolType)

// NewAutoPortMonitor 创建新的自动端口监控器
func NewAutoPortMonitor(config *Config, logger *logrus.Logger) *AutoPortMonitor {
	ctx, cancel := context.WithCancel(context.Background())

	apm := &AutoPortMonitor{
		config:     config,
		logger:     logger,
		portStatus: make(map[int]*AutoPortStatus),
		ctx:        ctx,
		cancel:     cancel,
		callbacks:  make([]AutoPortStatusCallback, 0),
	}

	// 初始化对象池
	if config.EnablePool {
		apm.statusPool = sync.Pool{
			New: func() interface{} {
				return &AutoPortStatus{}
			},
		}
	}

	return apm
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
		apm.portStatus[port] = apm.getStatusFromPool()
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
	portStatus := util.IsPortActive(port)

	apm.mutex.Lock()
	status, exists := apm.portStatus[port]
	if !exists {
		status = apm.getStatusFromPool()
		apm.portStatus[port] = status
	}

	// 检查状态是否发生变化
	statusChanged := status.IsActive != portStatus.Open

	if portStatus.Open {
		status.LastSeen = time.Now()
	}

	status.IsActive = portStatus.Open
	apm.mutex.Unlock()

	// 如果状态发生变化，触发回调
	if statusChanged {
		apm.logger.WithFields(logrus.Fields{
			"port":     port,
			"protocol": portStatus.Protocol,
			"isActive": portStatus.Open,
		}).Info("自动端口状态发生变化")

		apm.triggerCallbacks(port, portStatus.Open, portStatus.Protocol)
	}
}

// triggerCallbacks 触发回调函数
func (apm *AutoPortMonitor) triggerCallbacks(port int, isActive bool, protocol util.ProtocolType) {
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
			cb(port, isActive, protocol)
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

// getStatusFromPool 从对象池获取状态对象
func (apm *AutoPortMonitor) getStatusFromPool() *AutoPortStatus {
	if apm.config.EnablePool {
		return apm.statusPool.Get().(*AutoPortStatus)
	}
	return &AutoPortStatus{}
}
