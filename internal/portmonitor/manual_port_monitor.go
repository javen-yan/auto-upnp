package portmonitor

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// ManualPortStatus 手动端口状态
type ManualPortStatus struct {
	Port     int
	IsActive bool
	LastSeen time.Time
	Protocol string
}

// ManualPortMonitor 手动端口监控器
type ManualPortMonitor struct {
	logger        *logrus.Logger
	portStatus    map[int]*ManualPortStatus
	mutex         sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	callbacks     []ManualPortStatusCallback
	checkInterval time.Duration
	timeout       time.Duration
}

// ManualPortStatusCallback 手动端口状态变化回调函数
type ManualPortStatusCallback func(port int, isActive bool, protocol string)

// NewManualPortMonitor 创建新的手动端口监控器
func NewManualPortMonitor(checkInterval, timeout time.Duration, logger *logrus.Logger) *ManualPortMonitor {
	ctx, cancel := context.WithCancel(context.Background())

	return &ManualPortMonitor{
		logger:        logger,
		portStatus:    make(map[int]*ManualPortStatus),
		ctx:           ctx,
		cancel:        cancel,
		callbacks:     make([]ManualPortStatusCallback, 0),
		checkInterval: checkInterval,
		timeout:       timeout,
	}
}

// AddCallback 添加端口状态变化回调
func (mpm *ManualPortMonitor) AddCallback(callback ManualPortStatusCallback) {
	mpm.mutex.Lock()
	defer mpm.mutex.Unlock()
	mpm.callbacks = append(mpm.callbacks, callback)
}

// AddPort 添加要监控的端口
func (mpm *ManualPortMonitor) AddPort(port int, protocol string) {
	mpm.mutex.Lock()
	defer mpm.mutex.Unlock()

	if _, exists := mpm.portStatus[port]; !exists {
		mpm.portStatus[port] = &ManualPortStatus{
			Port:     port,
			IsActive: false,
			LastSeen: time.Time{},
			Protocol: protocol,
		}
		mpm.logger.WithFields(logrus.Fields{
			"port":     port,
			"protocol": protocol,
		}).Info("添加手动端口监控")
	}
}

// RemovePort 移除端口监控
func (mpm *ManualPortMonitor) RemovePort(port int) {
	mpm.mutex.Lock()
	defer mpm.mutex.Unlock()

	if _, exists := mpm.portStatus[port]; exists {
		delete(mpm.portStatus, port)
		mpm.logger.WithField("port", port).Info("移除手动端口监控")
	}
}

// Start 启动手动端口监控
func (mpm *ManualPortMonitor) Start() {
	mpm.logger.Info("启动手动端口监控器")
	go mpm.monitorLoop()
}

// Stop 停止手动端口监控
func (mpm *ManualPortMonitor) Stop() {
	mpm.logger.Info("停止手动端口监控器")
	mpm.cancel()
}

// monitorLoop 监控循环
func (mpm *ManualPortMonitor) monitorLoop() {
	ticker := time.NewTicker(mpm.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-mpm.ctx.Done():
			return
		case <-ticker.C:
			mpm.checkAllManualPorts()
		}
	}
}

// checkAllManualPorts 检查所有手动监控的端口状态
func (mpm *ManualPortMonitor) checkAllManualPorts() {
	mpm.mutex.RLock()
	ports := make([]int, 0, len(mpm.portStatus))
	for port := range mpm.portStatus {
		ports = append(ports, port)
	}
	mpm.mutex.RUnlock()

	var wg sync.WaitGroup
	for _, port := range ports {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			mpm.checkManualPort(p)
		}(port)
	}

	wg.Wait()
}

// checkManualPort 检查单个手动端口状态
func (mpm *ManualPortMonitor) checkManualPort(port int) {
	mpm.mutex.RLock()
	status, exists := mpm.portStatus[port]
	if !exists {
		mpm.mutex.RUnlock()
		return
	}
	protocol := status.Protocol
	mpm.mutex.RUnlock()

	isActive := mpm.isManualPortActive(port, protocol)

	mpm.mutex.Lock()
	status, exists = mpm.portStatus[port]
	if !exists {
		mpm.mutex.Unlock()
		return
	}

	// 检查状态是否发生变化
	statusChanged := status.IsActive != isActive

	if isActive {
		status.LastSeen = time.Now()
	}

	status.IsActive = isActive
	mpm.mutex.Unlock()

	// 如果状态发生变化，触发回调
	if statusChanged {
		mpm.logger.WithFields(logrus.Fields{
			"port":     port,
			"protocol": protocol,
			"isActive": isActive,
		}).Info("手动端口状态发生变化")

		mpm.triggerCallbacks(port, isActive, protocol)
	}
}

// isManualPortActive 检查手动端口是否活跃
func (mpm *ManualPortMonitor) isManualPortActive(port int, protocol string) bool {
	address := fmt.Sprintf(":%d", port)

	// 根据协议类型检查端口
	switch protocol {
	case "TCP":
		return mpm.isTCPPortActive(address)
	case "UDP":
		return mpm.isUDPPortActive(address)
	default:
		// 默认检查TCP
		return mpm.isTCPPortActive(address)
	}
}

// isTCPPortActive 检查TCP端口是否活跃
func (mpm *ManualPortMonitor) isTCPPortActive(address string) bool {
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

// isUDPPortActive 检查UDP端口是否活跃
func (mpm *ManualPortMonitor) isUDPPortActive(address string) bool {
	// 尝试监听UDP端口
	conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: 0})
	if err != nil {
		// 无法创建UDP连接，可能是权限问题
		return false
	}
	defer conn.Close()

	// 尝试连接到目标端口
	remoteAddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return false
	}

	// 设置超时
	conn.SetDeadline(time.Now().Add(mpm.timeout))

	// 尝试发送数据包
	_, err = conn.WriteToUDP([]byte("ping"), remoteAddr)
	if err != nil {
		// 无法发送，可能端口不可用
		return false
	}

	// 尝试接收响应
	buffer := make([]byte, 1024)
	_, _, err = conn.ReadFromUDP(buffer)
	if err != nil {
		// 没有响应，但端口可能仍然活跃（UDP是无连接的）
		// 这里我们假设如果无法连接，端口就是不活跃的
		return false
	}

	return true
}

// triggerCallbacks 触发回调函数
func (mpm *ManualPortMonitor) triggerCallbacks(port int, isActive bool, protocol string) {
	mpm.mutex.RLock()
	callbacks := make([]ManualPortStatusCallback, len(mpm.callbacks))
	copy(callbacks, mpm.callbacks)
	mpm.mutex.RUnlock()

	for _, callback := range callbacks {
		go func(cb ManualPortStatusCallback) {
			defer func() {
				if r := recover(); r != nil {
					mpm.logger.WithField("error", r).Error("手动端口状态回调函数执行出错")
				}
			}()
			cb(port, isActive, protocol)
		}(callback)
	}
}

// GetPortStatus 获取端口状态
func (mpm *ManualPortMonitor) GetPortStatus(port int) (*ManualPortStatus, bool) {
	mpm.mutex.RLock()
	defer mpm.mutex.RUnlock()

	status, exists := mpm.portStatus[port]
	if !exists {
		return nil, false
	}

	// 返回副本
	return &ManualPortStatus{
		Port:     status.Port,
		IsActive: status.IsActive,
		LastSeen: status.LastSeen,
		Protocol: status.Protocol,
	}, true
}

// GetAllPortStatus 获取所有端口状态
func (mpm *ManualPortMonitor) GetAllPortStatus() map[int]*ManualPortStatus {
	mpm.mutex.RLock()
	defer mpm.mutex.RUnlock()

	result := make(map[int]*ManualPortStatus)
	for port, status := range mpm.portStatus {
		result[port] = &ManualPortStatus{
			Port:     status.Port,
			IsActive: status.IsActive,
			LastSeen: status.LastSeen,
			Protocol: status.Protocol,
		}
	}

	return result
}

// GetActivePorts 获取活跃端口列表
func (mpm *ManualPortMonitor) GetActivePorts() []int {
	mpm.mutex.RLock()
	defer mpm.mutex.RUnlock()

	var activePorts []int
	for port, status := range mpm.portStatus {
		if status.IsActive {
			activePorts = append(activePorts, port)
		}
	}

	return activePorts
}

// GetInactivePorts 获取非活跃端口列表
func (mpm *ManualPortMonitor) GetInactivePorts() []int {
	mpm.mutex.RLock()
	defer mpm.mutex.RUnlock()

	var inactivePorts []int
	for port, status := range mpm.portStatus {
		if !status.IsActive {
			inactivePorts = append(inactivePorts, port)
		}
	}

	return inactivePorts
}

// GetMonitoredPorts 获取所有被监控的端口
func (mpm *ManualPortMonitor) GetMonitoredPorts() []int {
	mpm.mutex.RLock()
	defer mpm.mutex.RUnlock()

	ports := make([]int, 0, len(mpm.portStatus))
	for port := range mpm.portStatus {
		ports = append(ports, port)
	}

	return ports
}
