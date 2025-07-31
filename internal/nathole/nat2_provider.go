package nathole

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"auto-upnp/internal/types"

	"github.com/sirupsen/logrus"
)

// NAT2Provider NAT2提供者（受限锥形NAT）
type NAT2Provider struct {
	logger    *logrus.Logger
	ctx       context.Context
	cancel    context.CancelFunc
	holes     map[string]*NATHole
	mutex     sync.RWMutex
	available bool
	config    map[string]interface{}

	// 记录已连接的外部主机
	connectedHosts map[string]bool
	hostMutex      sync.RWMutex

	// 公网IP信息
	publicIP      string
	publicIPMutex sync.RWMutex
}

// NewNAT2Provider 创建新的NAT2提供者
func NewNAT2Provider(logger *logrus.Logger, config map[string]interface{}) *NAT2Provider {
	ctx, cancel := context.WithCancel(context.Background())

	return &NAT2Provider{
		logger:         logger,
		ctx:            ctx,
		cancel:         cancel,
		holes:          make(map[string]*NATHole),
		available:      false,
		config:         config,
		connectedHosts: make(map[string]bool),
	}
}

// Type 返回NAT类型
func (n *NAT2Provider) Type() types.NATType {
	return types.NATType2
}

// Name 返回提供者名称
func (n *NAT2Provider) Name() string {
	return "NAT2提供者（受限锥形NAT）"
}

// IsAvailable 检查是否可用
func (n *NAT2Provider) IsAvailable() bool {
	return n.available
}

// Start 启动NAT2提供者
func (n *NAT2Provider) Start() error {
	n.logger.Info("启动NAT2提供者")

	// 对于受限锥形NAT，我们需要先与外部主机建立连接
	// 然后外部主机才能连接到我们
	n.available = true

	// 检测公网IP
	go n.detectPublicIP()

	// 启动连接收集协程
	go n.collectAvailableConnections()

	n.logger.Info("NAT2提供者启动成功")
	return nil
}

// Stop 停止NAT2提供者
func (n *NAT2Provider) Stop() error {
	n.logger.Info("停止NAT2提供者")
	n.cancel()
	n.available = false

	// 关闭所有监听器
	n.mutex.Lock()
	defer n.mutex.Unlock()

	for _, hole := range n.holes {
		if hole.Status == HoleStatusActive {
			hole.Status = HoleStatusInactive
		}
	}

	n.logger.Info("NAT2提供者已停止")
	return nil
}

// CreateHole 创建NAT穿透
func (n *NAT2Provider) CreateHole(localPort int, externalPort int, protocol string, description string) (*NATHole, error) {
	if !n.available {
		return nil, fmt.Errorf("NAT2提供者不可用")
	}

	key := fmt.Sprintf("%d-%d-%s", localPort, externalPort, protocol)

	n.mutex.Lock()
	defer n.mutex.Unlock()

	// 检查是否已存在
	if existing, exists := n.holes[key]; exists {
		if existing.Status == HoleStatusActive {
			return existing, nil
		}
	}

	// 创建新的穿透
	hole := &NATHole{
		LocalPort:    localPort,
		ExternalPort: externalPort,
		Protocol:     protocol,
		Description:  description,
		Type:         types.NATType2,
		Status:       HoleStatusActive,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
	}

	// 对于受限锥形NAT，我们需要：
	// 1. 监听外部端口（远端端口）
	// 2. 维护已连接的外部主机列表
	// 3. 只允许已建立连接的外部主机访问
	var listener net.Listener
	var packetConn net.PacketConn
	var err error

	// 根据协议类型选择不同的监听方式
	switch protocol {
	case "tcp":
		// TCP协议使用Listener - 监听所有接口
		listener, err = net.Listen(protocol, fmt.Sprintf("0.0.0.0:%d", externalPort))
		if err != nil {
			hole.Status = HoleStatusFailed
			hole.Error = fmt.Sprintf("无法监听外部端口 %d: %v", externalPort, err)
			n.holes[key] = hole
			return hole, fmt.Errorf("无法监听外部端口 %d: %w", externalPort, err)
		}

		// 启动TCP监听协程
		go n.handleTCPConnections(listener, hole)
	case "udp":
		// UDP协议使用PacketConn - 监听所有接口
		packetConn, err = net.ListenPacket(protocol, fmt.Sprintf("0.0.0.0:%d", externalPort))
		if err != nil {
			hole.Status = HoleStatusFailed
			hole.Error = fmt.Sprintf("无法监听外部UDP端口 %d: %v", externalPort, err)
			n.holes[key] = hole
			return hole, fmt.Errorf("无法监听外部UDP端口 %d: %w", externalPort, err)
		}

		// 启动UDP监听协程
		go n.handleUDPConnections(packetConn, hole)
	default:
		hole.Status = HoleStatusFailed
		hole.Error = fmt.Sprintf("不支持的协议: %s", protocol)
		n.holes[key] = hole
		return hole, fmt.Errorf("不支持的协议: %s", protocol)
	}

	// 启动自动协商过程
	go n.establishExternalConnection(hole)

	n.holes[key] = hole

	n.logger.WithFields(logrus.Fields{
		"local_port":    localPort,
		"external_port": externalPort,
		"protocol":      protocol,
		"type":          "NAT2",
	}).Info("创建NAT2穿透成功")

	return hole, nil
}

// RemoveHole 移除NAT穿透
func (n *NAT2Provider) RemoveHole(localPort int, externalPort int, protocol string) error {
	key := fmt.Sprintf("%d-%d-%s", localPort, externalPort, protocol)

	n.mutex.Lock()
	defer n.mutex.Unlock()

	if hole, exists := n.holes[key]; exists {
		hole.Status = HoleStatusInactive
		hole.LastActivity = time.Now()

		n.logger.WithFields(logrus.Fields{
			"local_port": localPort,
			"protocol":   protocol,
			"type":       "NAT2",
		}).Info("移除NAT2穿透成功")

		return nil
	}

	return fmt.Errorf("未找到指定的NAT穿透")
}

// GetHoles 获取所有穿透
func (n *NAT2Provider) GetHoles() map[string]*NATHole {
	n.mutex.RLock()
	defer n.mutex.RUnlock()

	result := make(map[string]*NATHole)
	for key, hole := range n.holes {
		result[key] = hole
	}

	return result
}

// GetStatus 获取提供者状态
func (n *NAT2Provider) GetStatus() map[string]interface{} {
	n.mutex.RLock()
	defer n.mutex.RUnlock()

	activeCount := 0
	inactiveCount := 0
	failedCount := 0

	for _, hole := range n.holes {
		switch hole.Status {
		case HoleStatusActive:
			activeCount++
		case HoleStatusInactive:
			inactiveCount++
		case HoleStatusFailed:
			failedCount++
		}
	}

	n.hostMutex.RLock()
	connectedHostsCount := len(n.connectedHosts)
	n.hostMutex.RUnlock()

	n.publicIPMutex.RLock()
	publicIP := n.publicIP
	n.publicIPMutex.RUnlock()

	return map[string]interface{}{
		"available":       n.available,
		"total_holes":     len(n.holes),
		"active_holes":    activeCount,
		"inactive_holes":  inactiveCount,
		"failed_holes":    failedCount,
		"connected_hosts": connectedHostsCount,
		"public_ip":       publicIP,
	}
}

// handleTCPConnections 处理TCP连接
func (n *NAT2Provider) handleTCPConnections(listener net.Listener, hole *NATHole) {
	defer listener.Close()

	n.logger.WithFields(logrus.Fields{
		"external_port": hole.ExternalPort,
		"local_port":    hole.LocalPort,
		"protocol":      hole.Protocol,
	}).Info("开始监听外部TCP端口")

	for {
		select {
		case <-n.ctx.Done():
			n.logger.Info("停止监听外部TCP端口")
			return
		default:
			conn, err := listener.Accept()
			if err != nil {
				n.logger.WithError(err).Error("接受TCP连接失败")
				continue
			}

			// 检查是否允许此连接
			// if !n.isConnectionAllowed(conn) {
			// 	n.logger.WithFields(logrus.Fields{
			// 		"external_port": hole.ExternalPort,
			// 		"local_port":    hole.LocalPort,
			// 		"remote_addr":   conn.RemoteAddr(),
			// 		"protocol":      hole.Protocol,
			// 	}).Warn("拒绝未授权的NAT2 TCP连接")
			// 	conn.Close()
			// 	continue
			// }

			// 更新最后活动时间
			hole.LastActivity = time.Now()

			// 记录连接的主机
			n.recordConnection(conn)

			n.logger.WithFields(logrus.Fields{
				"external_port": hole.ExternalPort,
				"local_port":    hole.LocalPort,
				"remote_addr":   conn.RemoteAddr(),
				"protocol":      hole.Protocol,
			}).Info("NAT2穿透接收到外部TCP连接")

			// 处理TCP连接（转发到本地端口）
			go n.handleTCPConnection(conn, hole)
		}
	}
}

// establishExternalConnection 建立外部连接
func (n *NAT2Provider) establishExternalConnection(hole *NATHole) {
	// 对于受限锥形NAT，我们需要与外部主机建立连接
	// 这样外部主机才能连接到我们

	// 启动自动协商协程
	go n.autoNegotiation(hole)

	n.logger.WithFields(logrus.Fields{
		"local_port":    hole.LocalPort,
		"external_port": hole.ExternalPort,
		"protocol":      hole.Protocol,
	}).Info("启动NAT2自动协商")
}

// autoNegotiation 自动协商过程
func (n *NAT2Provider) autoNegotiation(hole *NATHole) {
	// 尝试连接到多个外部服务器以建立映射
	servers := []string{
		"stun.miwifi.com:3478",
		"stun.chat.bilibili.com:3478",
		"stun.hitv.com:3478",
		"stun.cdnbye.com:3478",
	}

	for _, server := range servers {
		select {
		case <-n.ctx.Done():
			return
		default:
			if n.tryConnectToServer(hole, server) {
				n.logger.WithFields(logrus.Fields{
					"server":        server,
					"local_port":    hole.LocalPort,
					"external_port": hole.ExternalPort,
					"protocol":      hole.Protocol,
				}).Info("成功建立NAT2映射")
				return
			}
		}
	}

	n.logger.WithFields(logrus.Fields{
		"local_port":    hole.LocalPort,
		"external_port": hole.ExternalPort,
		"protocol":      hole.Protocol,
	}).Warn("无法建立NAT2映射，所有服务器连接失败")
}

// tryConnectToServer 尝试连接到指定服务器
func (n *NAT2Provider) tryConnectToServer(hole *NATHole, server string) bool {
	// 根据协议类型选择连接方式
	var conn net.Conn
	var err error

	switch hole.Protocol {
	case "tcp":
		conn, err = net.DialTimeout("tcp", server, 5*time.Second)
	case "udp":
		conn, err = net.DialTimeout("udp", server, 5*time.Second)
	default:
		return false
	}

	if err != nil {
		n.logger.WithFields(logrus.Fields{
			"server":   server,
			"protocol": hole.Protocol,
			"error":    err.Error(),
		}).Debug("连接服务器失败")
		return false
	}
	defer conn.Close()

	// 获取本地地址
	localAddr := conn.LocalAddr()
	if tcpAddr, ok := localAddr.(*net.TCPAddr); ok {
		// 记录连接的主机（允许该主机访问）
		n.hostMutex.Lock()
		n.connectedHosts[tcpAddr.IP.String()] = true
		n.hostMutex.Unlock()

		n.logger.WithFields(logrus.Fields{
			"server":        server,
			"local_addr":    localAddr,
			"local_port":    hole.LocalPort,
			"external_port": hole.ExternalPort,
			"protocol":      hole.Protocol,
		}).Info("成功建立NAT2连接")

		return true
	}

	return false
}

// collectAvailableConnections 收集可用的连接
func (n *NAT2Provider) collectAvailableConnections() {
	ticker := time.NewTicker(30 * time.Second) // 每30秒收集一次
	defer ticker.Stop()

	for {
		select {
		case <-n.ctx.Done():
			return
		case <-ticker.C:
			n.scanForConnections()
		}
	}
}

// scanForConnections 扫描可用连接
func (n *NAT2Provider) scanForConnections() {
	// 扫描常见的端口和服务
	ports := []int{80, 443, 8080, 8443, 22, 21, 25, 53}

	for _, port := range ports {
		select {
		case <-n.ctx.Done():
			return
		default:
			// 尝试连接到本地端口以建立映射
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 1*time.Second)
			if err == nil {
				conn.Close()

				// 记录本地连接
				n.hostMutex.Lock()
				n.connectedHosts["127.0.0.1"] = true
				n.hostMutex.Unlock()

				n.logger.WithField("port", port).Debug("发现本地可用连接")
			}
		}
	}
}

// detectPublicIP 检测公网IP
func (n *NAT2Provider) detectPublicIP() {
	// 使用多个服务检测公网IP
	services := []string{
		"http://ipinfo.io/ip",
		"http://icanhazip.com",
		"http://ifconfig.me",
		"http://ipecho.net/plain",
	}

	for _, service := range services {
		ip, err := n.getPublicIPFromService(service)
		if err == nil && ip != "" {
			n.publicIPMutex.Lock()
			n.publicIP = ip
			n.publicIPMutex.Unlock()

			n.logger.WithFields(logrus.Fields{
				"public_ip": ip,
				"service":   service,
			}).Info("检测到公网IP")

			// 将公网IP添加到已连接主机列表
			n.hostMutex.Lock()
			n.connectedHosts[ip] = true
			n.hostMutex.Unlock()

			return
		}
	}

	n.logger.Warn("无法检测到公网IP")
}

// getPublicIPFromService 从指定服务获取公网IP
func (n *NAT2Provider) getPublicIPFromService(service string) (string, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(service)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	ip := strings.TrimSpace(string(body))
	if net.ParseIP(ip) != nil {
		return ip, nil
	}

	return "", fmt.Errorf("无效的IP地址: %s", ip)
}

// GetPublicIP 获取公网IP
func (n *NAT2Provider) GetPublicIP() string {
	n.publicIPMutex.RLock()
	defer n.publicIPMutex.RUnlock()
	return n.publicIP
}

// CheckPortAccessibility 检查端口是否可以从公网访问
func (n *NAT2Provider) CheckPortAccessibility(port int, protocol string) bool {
	// 尝试监听端口
	var listener net.Listener
	var packetConn net.PacketConn
	var err error

	switch protocol {
	case "tcp":
		listener, err = net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
		if err != nil {
			n.logger.WithFields(logrus.Fields{
				"port":     port,
				"protocol": protocol,
				"error":    err.Error(),
			}).Warn("端口无法监听")
			return false
		}
		listener.Close()
	case "udp":
		packetConn, err = net.ListenPacket("udp", fmt.Sprintf("0.0.0.0:%d", port))
		if err != nil {
			n.logger.WithFields(logrus.Fields{
				"port":     port,
				"protocol": protocol,
				"error":    err.Error(),
			}).Warn("端口无法监听")
			return false
		}
		packetConn.Close()
	}

	n.logger.WithFields(logrus.Fields{
		"port":     port,
		"protocol": protocol,
	}).Info("端口可以监听")

	return true
}

// GetAccessiblePorts 获取可访问的端口列表
func (n *NAT2Provider) GetAccessiblePorts() map[string][]int {
	accessiblePorts := make(map[string][]int)

	// 测试常用端口
	ports := []int{80, 443, 8080, 8443, 22, 21, 25, 53, 3389, 5900}

	for _, port := range ports {
		if n.CheckPortAccessibility(port, "tcp") {
			accessiblePorts["tcp"] = append(accessiblePorts["tcp"], port)
		}
		if n.CheckPortAccessibility(port, "udp") {
			accessiblePorts["udp"] = append(accessiblePorts["udp"], port)
		}
	}

	return accessiblePorts
}

// handleTCPConnection 处理单个TCP连接
func (n *NAT2Provider) handleTCPConnection(externalConn net.Conn, hole *NATHole) {
	defer externalConn.Close()

	// 连接到本地端口
	localConn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", hole.LocalPort))
	if err != nil {
		n.logger.WithFields(logrus.Fields{
			"local_port":  hole.LocalPort,
			"remote_addr": externalConn.RemoteAddr(),
			"error":       err.Error(),
		}).Error("无法连接到本地TCP端口")
		return
	}
	defer localConn.Close()

	n.logger.WithFields(logrus.Fields{
		"external_port": hole.ExternalPort,
		"local_port":    hole.LocalPort,
		"remote_addr":   externalConn.RemoteAddr(),
		"protocol":      hole.Protocol,
	}).Debug("开始转发NAT2 TCP连接")

	// 双向转发数据
	go func() {
		written, err := io.Copy(localConn, externalConn)
		if err != nil {
			n.logger.WithError(err).Debug("转发TCP数据到本地端口时出错")
		}
		n.logger.WithField("bytes_written", written).Debug("转发TCP数据到本地端口完成")
	}()

	written, err := io.Copy(externalConn, localConn)
	if err != nil {
		n.logger.WithError(err).Debug("转发TCP数据到外部连接时出错")
	}
	n.logger.WithField("bytes_written", written).Debug("转发TCP数据到外部连接完成")
}

// handleUDPConnections 处理UDP连接
func (n *NAT2Provider) handleUDPConnections(packetConn net.PacketConn, hole *NATHole) {
	defer packetConn.Close()

	n.logger.WithFields(logrus.Fields{
		"external_port": hole.ExternalPort,
		"local_port":    hole.LocalPort,
		"protocol":      hole.Protocol,
	}).Info("开始监听外部UDP端口")

	buffer := make([]byte, 4096)

	for {
		select {
		case <-n.ctx.Done():
			n.logger.Info("停止监听外部UDP端口")
			return
		default:
			bytesRead, remoteAddr, err := packetConn.ReadFrom(buffer)
			if err != nil {
				n.logger.WithError(err).Error("读取UDP数据失败")
				continue
			}

			// 检查是否允许此连接
			// if !n.isUDPConnectionAllowed(remoteAddr) {
			// 	n.logger.WithFields(logrus.Fields{
			// 		"external_port": hole.ExternalPort,
			// 		"local_port":    hole.LocalPort,
			// 		"remote_addr":   remoteAddr,
			// 		"protocol":      hole.Protocol,
			// 	}).Warn("拒绝未授权的NAT2 UDP连接")
			// 	continue
			// }

			// 更新最后活动时间
			hole.LastActivity = time.Now()

			// 记录连接的主机
			n.recordUDPConnection(remoteAddr)

			n.logger.WithFields(logrus.Fields{
				"external_port": hole.ExternalPort,
				"local_port":    hole.LocalPort,
				"remote_addr":   remoteAddr,
				"protocol":      hole.Protocol,
				"data_size":     bytesRead,
			}).Info("NAT2穿透接收到外部UDP数据")

			// 处理UDP数据（转发到本地端口）
			go n.handleUDPData(packetConn, remoteAddr, buffer[:bytesRead], hole)
		}
	}
}

// handleUDPData 处理UDP数据
func (n *NAT2Provider) handleUDPData(packetConn net.PacketConn, remoteAddr net.Addr, data []byte, hole *NATHole) {
	// 连接到本地UDP端口
	localAddr := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: hole.LocalPort,
	}

	localConn, err := net.DialUDP("udp", nil, localAddr)
	if err != nil {
		n.logger.WithFields(logrus.Fields{
			"local_port":  hole.LocalPort,
			"remote_addr": remoteAddr,
			"error":       err.Error(),
		}).Error("无法连接到本地UDP端口")
		return
	}
	defer localConn.Close()

	n.logger.WithFields(logrus.Fields{
		"external_port": hole.ExternalPort,
		"local_port":    hole.LocalPort,
		"remote_addr":   remoteAddr,
		"data_size":     len(data),
	}).Debug("开始转发NAT2 UDP数据")

	// 发送数据到本地端口
	_, err = localConn.Write(data)
	if err != nil {
		n.logger.WithError(err).Error("发送UDP数据到本地端口失败")
		return
	}

	// 读取本地端口的响应
	responseBuffer := make([]byte, 4096)
	bytesRead, err := localConn.Read(responseBuffer)
	if err != nil {
		n.logger.WithError(err).Debug("读取本地UDP端口响应失败")
		return
	}

	// 发送响应回外部客户端
	_, err = packetConn.WriteTo(responseBuffer[:bytesRead], remoteAddr)
	if err != nil {
		n.logger.WithError(err).Error("发送UDP响应失败")
		return
	}

	n.logger.WithFields(logrus.Fields{
		"external_port": hole.ExternalPort,
		"local_port":    hole.LocalPort,
		"remote_addr":   remoteAddr,
		"response_size": bytesRead,
	}).Debug("NAT2 UDP数据转发完成")
}

// isUDPConnectionAllowed 检查UDP连接是否被允许
func (n *NAT2Provider) isUDPConnectionAllowed(remoteAddr net.Addr) bool {
	if udpAddr, ok := remoteAddr.(*net.UDPAddr); ok {
		host := udpAddr.IP.String()

		n.hostMutex.RLock()
		defer n.hostMutex.RUnlock()

		return n.connectedHosts[host]
	}
	return false
}

// recordUDPConnection 记录UDP连接的主机
func (n *NAT2Provider) recordUDPConnection(remoteAddr net.Addr) {
	if udpAddr, ok := remoteAddr.(*net.UDPAddr); ok {
		host := udpAddr.IP.String()

		n.hostMutex.Lock()
		defer n.hostMutex.Unlock()

		n.connectedHosts[host] = true
	}
}

// isConnectionAllowed 检查连接是否被允许
func (n *NAT2Provider) isConnectionAllowed(conn net.Conn) bool {
	remoteAddr := conn.RemoteAddr()
	if tcpAddr, ok := remoteAddr.(*net.TCPAddr); ok {
		host := tcpAddr.IP.String()

		n.hostMutex.RLock()
		defer n.hostMutex.RUnlock()

		return n.connectedHosts[host]
	}
	return false
}

// recordConnection 记录连接的主机
func (n *NAT2Provider) recordConnection(conn net.Conn) {
	remoteAddr := conn.RemoteAddr()
	if tcpAddr, ok := remoteAddr.(*net.TCPAddr); ok {
		host := tcpAddr.IP.String()

		n.hostMutex.Lock()
		defer n.hostMutex.Unlock()

		n.connectedHosts[host] = true
	}
}
