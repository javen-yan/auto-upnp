package nathole

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"auto-upnp/internal/types"

	"github.com/sirupsen/logrus"
)

// ProxyHandler 代理处理器接口
type ProxyHandler interface {
	// OnConnectionAccepted 当连接被接受时调用
	OnConnectionAccepted(conn net.Conn, hole *NATHole)
	// OnUDPDataReceived 当UDP数据被接收时调用
	OnUDPDataReceived(remoteAddr net.Addr, data []byte, hole *NATHole)
	// IsConnectionAllowed 检查连接是否被允许
	IsConnectionAllowed(conn net.Conn) bool
	// IsUDPConnectionAllowed 检查UDP连接是否被允许
	IsUDPConnectionAllowed(remoteAddr net.Addr) bool
	// RecordConnection 记录连接
	RecordConnection(conn net.Conn)
	// RecordUDPConnection 记录UDP连接
	RecordUDPConnection(remoteAddr net.Addr)
	// GetLogger 获取日志记录器
	GetLogger() *logrus.Logger
	// GetContext 获取上下文
	GetContext() interface{}
}

// TCPProxy TCP代理
type TCPProxy struct {
	handler ProxyHandler
}

// NewTCPProxy 创建新的TCP代理
func NewTCPProxy(handler ProxyHandler) *TCPProxy {
	return &TCPProxy{
		handler: handler,
	}
}

// HandleConnections 处理TCP连接
func (p *TCPProxy) HandleConnections(listener net.Listener, hole *NATHole) {
	defer listener.Close()

	logger := p.handler.GetLogger()
	logger.WithFields(logrus.Fields{
		"external_port": hole.ExternalPort,
		"local_port":    hole.LocalPort,
		"protocol":      hole.Protocol,
	}).Info("开始监听外部TCP端口")

	ctx := p.handler.GetContext()

	for {
		select {
		case <-ctx.(interface{ Done() <-chan struct{} }).Done():
			logger.Info("停止监听外部TCP端口")
			return
		default:
			conn, err := listener.Accept()
			if err != nil {
				logger.WithError(err).Error("接受TCP连接失败")
				continue
			}

			// 检查是否允许此连接
			if !p.handler.IsConnectionAllowed(conn) {
				logger.WithFields(logrus.Fields{
					"external_port": hole.ExternalPort,
					"local_port":    hole.LocalPort,
					"remote_addr":   conn.RemoteAddr(),
					"protocol":      hole.Protocol,
				}).Warn("拒绝未授权的TCP连接")
				conn.Close()
				continue
			}

			// 更新最后活动时间
			hole.LastActivity = time.Now()

			// 记录连接
			p.handler.RecordConnection(conn)

			logger.WithFields(logrus.Fields{
				"external_port": hole.ExternalPort,
				"local_port":    hole.LocalPort,
				"remote_addr":   conn.RemoteAddr(),
				"protocol":      hole.Protocol,
			}).Info("穿透接收到外部TCP连接")

			// 处理TCP连接
			go p.HandleConnection(conn, hole)
		}
	}
}

// HandleConnection 处理单个TCP连接
func (p *TCPProxy) HandleConnection(externalConn net.Conn, hole *NATHole) {
	defer externalConn.Close()

	logger := p.handler.GetLogger()

	// 连接到本地端口
	localConn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", hole.LocalPort))
	if err != nil {
		logger.WithFields(logrus.Fields{
			"local_port":  hole.LocalPort,
			"remote_addr": externalConn.RemoteAddr(),
			"error":       err.Error(),
		}).Error("无法连接到本地TCP端口")
		return
	}
	defer localConn.Close()

	logger.WithFields(logrus.Fields{
		"external_port": hole.ExternalPort,
		"local_port":    hole.LocalPort,
		"remote_addr":   externalConn.RemoteAddr(),
		"protocol":      hole.Protocol,
	}).Debug("开始转发TCP连接")

	// 双向转发数据
	go func() {
		written, err := io.Copy(localConn, externalConn)
		if err != nil {
			logger.WithError(err).Debug("转发TCP数据到本地端口时出错")
		}
		logger.WithField("bytes_written", written).Debug("转发TCP数据到本地端口完成")
	}()

	written, err := io.Copy(externalConn, localConn)
	if err != nil {
		logger.WithError(err).Debug("转发TCP数据到外部连接时出错")
	}
	logger.WithField("bytes_written", written).Debug("转发TCP数据到外部连接完成")
}

// UDPProxy UDP代理
type UDPProxy struct {
	handler ProxyHandler
}

// NewUDPProxy 创建新的UDP代理
func NewUDPProxy(handler ProxyHandler) *UDPProxy {
	return &UDPProxy{
		handler: handler,
	}
}

// HandleConnections 处理UDP连接
func (p *UDPProxy) HandleConnections(packetConn net.PacketConn, hole *NATHole) {
	defer packetConn.Close()

	logger := p.handler.GetLogger()
	logger.WithFields(logrus.Fields{
		"external_port": hole.ExternalPort,
		"local_port":    hole.LocalPort,
		"protocol":      hole.Protocol,
	}).Info("开始监听外部UDP端口")

	buffer := make([]byte, 4096)
	ctx := p.handler.GetContext()

	for {
		select {
		case <-ctx.(interface{ Done() <-chan struct{} }).Done():
			logger.Info("停止监听外部UDP端口")
			return
		default:
			bytesRead, remoteAddr, err := packetConn.ReadFrom(buffer)
			if err != nil {
				logger.WithError(err).Error("读取UDP数据失败")
				continue
			}

			// 检查是否允许此连接
			if !p.handler.IsUDPConnectionAllowed(remoteAddr) {
				logger.WithFields(logrus.Fields{
					"external_port": hole.ExternalPort,
					"local_port":    hole.LocalPort,
					"remote_addr":   remoteAddr,
					"protocol":      hole.Protocol,
				}).Warn("拒绝未授权的UDP连接")
				continue
			}

			// 更新最后活动时间
			hole.LastActivity = time.Now()

			// 记录连接
			p.handler.RecordUDPConnection(remoteAddr)

			logger.WithFields(logrus.Fields{
				"external_port": hole.ExternalPort,
				"local_port":    hole.LocalPort,
				"remote_addr":   remoteAddr,
				"protocol":      hole.Protocol,
				"data_size":     bytesRead,
			}).Info("穿透接收到外部UDP数据")

			// 处理UDP数据
			go p.HandleData(packetConn, remoteAddr, buffer[:bytesRead], hole)
		}
	}
}

// HandleData 处理UDP数据
func (p *UDPProxy) HandleData(packetConn net.PacketConn, remoteAddr net.Addr, data []byte, hole *NATHole) {
	logger := p.handler.GetLogger()

	// 连接到本地UDP端口
	localAddr := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: hole.LocalPort,
	}

	localConn, err := net.DialUDP("udp", nil, localAddr)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"local_port":  hole.LocalPort,
			"remote_addr": remoteAddr,
			"error":       err.Error(),
		}).Error("无法连接到本地UDP端口")
		return
	}
	defer localConn.Close()

	logger.WithFields(logrus.Fields{
		"external_port": hole.ExternalPort,
		"local_port":    hole.LocalPort,
		"remote_addr":   remoteAddr,
		"data_size":     len(data),
	}).Debug("开始转发UDP数据")

	// 发送数据到本地端口
	_, err = localConn.Write(data)
	if err != nil {
		logger.WithError(err).Error("发送UDP数据到本地端口失败")
		return
	}

	// 读取本地端口的响应
	responseBuffer := make([]byte, 4096)
	bytesRead, err := localConn.Read(responseBuffer)
	if err != nil {
		logger.WithError(err).Debug("读取本地UDP端口响应失败")
		return
	}

	// 发送响应回外部客户端
	_, err = packetConn.WriteTo(responseBuffer[:bytesRead], remoteAddr)
	if err != nil {
		logger.WithError(err).Error("发送UDP响应失败")
		return
	}

	logger.WithFields(logrus.Fields{
		"external_port": hole.ExternalPort,
		"local_port":    hole.LocalPort,
		"remote_addr":   remoteAddr,
		"response_size": bytesRead,
	}).Debug("UDP数据转发完成")
}

// BaseProxyHandler 基础代理处理器，提供默认实现
type BaseProxyHandler struct {
	logger         *logrus.Logger
	ctx            interface{}
	connectedHosts map[string]bool
	hostMutex      sync.RWMutex
}

// NewBaseProxyHandler 创建基础代理处理器
func NewBaseProxyHandler(logger *logrus.Logger, ctx interface{}) *BaseProxyHandler {
	return &BaseProxyHandler{
		logger:         logger,
		ctx:            ctx,
		connectedHosts: make(map[string]bool),
	}
}

// IsConnectionAllowed 检查连接是否被允许（默认允许所有连接）
func (h *BaseProxyHandler) IsConnectionAllowed(conn net.Conn) bool {
	return true
}

// IsUDPConnectionAllowed 检查UDP连接是否被允许（默认允许所有连接）
func (h *BaseProxyHandler) IsUDPConnectionAllowed(remoteAddr net.Addr) bool {
	return true
}

// RecordConnection 记录连接
func (h *BaseProxyHandler) RecordConnection(conn net.Conn) {
	// 默认实现为空
}

// RecordUDPConnection 记录UDP连接
func (h *BaseProxyHandler) RecordUDPConnection(remoteAddr net.Addr) {
	// 默认实现为空
}

// OnConnectionAccepted 当连接被接受时调用
func (h *BaseProxyHandler) OnConnectionAccepted(conn net.Conn, hole *NATHole) {
	// 默认实现为空
}

// OnUDPDataReceived 当UDP数据被接收时调用
func (h *BaseProxyHandler) OnUDPDataReceived(remoteAddr net.Addr, data []byte, hole *NATHole) {
	// 默认实现为空
}

// GetLogger 获取日志记录器
func (h *BaseProxyHandler) GetLogger() *logrus.Logger {
	return h.logger
}

// GetContext 获取上下文
func (h *BaseProxyHandler) GetContext() interface{} {
	return h.ctx
}

// NAT2ProxyHandler NAT2专用的代理处理器
type NAT2ProxyHandler struct {
	*BaseProxyHandler
}

// NewNAT2ProxyHandler 创建NAT2代理处理器
func NewNAT2ProxyHandler(logger *logrus.Logger, ctx interface{}) *NAT2ProxyHandler {
	return &NAT2ProxyHandler{
		BaseProxyHandler: NewBaseProxyHandler(logger, ctx),
	}
}

// IsConnectionAllowed 检查连接是否被允许（NAT2只允许已建立连接的主机）
func (h *NAT2ProxyHandler) IsConnectionAllowed(conn net.Conn) bool {
	remoteAddr := conn.RemoteAddr()
	if tcpAddr, ok := remoteAddr.(*net.TCPAddr); ok {
		host := tcpAddr.IP.String()

		// 允许本地回环连接（IPv4和IPv6）
		if host == "127.0.0.1" || host == "::1" || host == "localhost" {
			return true
		}

		h.hostMutex.RLock()
		defer h.hostMutex.RUnlock()

		return h.connectedHosts[host]
	}
	return false
}

// IsUDPConnectionAllowed 检查UDP连接是否被允许（NAT2只允许已建立连接的主机）
func (h *NAT2ProxyHandler) IsUDPConnectionAllowed(remoteAddr net.Addr) bool {
	if udpAddr, ok := remoteAddr.(*net.UDPAddr); ok {
		host := udpAddr.IP.String()

		// 允许本地回环连接（IPv4和IPv6）
		if host == "127.0.0.1" || host == "::1" || host == "localhost" {
			return true
		}

		h.hostMutex.RLock()
		defer h.hostMutex.RUnlock()

		return h.connectedHosts[host]
	}
	return false
}

// RecordConnection 记录连接的主机
func (h *NAT2ProxyHandler) RecordConnection(conn net.Conn) {
	remoteAddr := conn.RemoteAddr()
	if tcpAddr, ok := remoteAddr.(*net.TCPAddr); ok {
		host := tcpAddr.IP.String()

		h.hostMutex.Lock()
		defer h.hostMutex.Unlock()

		h.connectedHosts[host] = true
	}
}

// RecordUDPConnection 记录UDP连接的主机
func (h *NAT2ProxyHandler) RecordUDPConnection(remoteAddr net.Addr) {
	if udpAddr, ok := remoteAddr.(*net.UDPAddr); ok {
		host := udpAddr.IP.String()

		h.hostMutex.Lock()
		defer h.hostMutex.Unlock()

		h.connectedHosts[host] = true
	}
}

// CreateHoleCommon 公共的创建穿透方法
func CreateHoleCommon(
	logger *logrus.Logger,
	holes map[string]*NATHole,
	mutex *sync.RWMutex,
	available bool,
	providerName string,
	natType types.NATType,
	localPort int,
	externalPort int,
	protocol string,
	description string,
	handleTCP func(net.Listener, *NATHole),
	handleUDP func(net.PacketConn, *NATHole),
	establishExternal func(*NATHole),
) (*NATHole, error) {
	if !available {
		return nil, fmt.Errorf("%s不可用", providerName)
	}

	key := fmt.Sprintf("%d-%d-%s", localPort, externalPort, protocol)

	mutex.Lock()
	defer mutex.Unlock()

	// 检查是否已存在
	if existing, exists := holes[key]; exists {
		if existing.Status == types.MappingStatusActive {
			return existing, nil
		}
	}

	// 创建新的穿透
	hole := &NATHole{
		LocalPort:    localPort,
		ExternalPort: externalPort,
		Protocol:     protocol,
		Description:  description,
		Type:         natType,
		Status:       types.MappingStatusActive,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
	}

	// 确保外部端口与本地端口不同，避免端口冲突
	listenPort := externalPort
	useRandomPort := false

	// 如果外部端口与本地端口相同，则使用随机端口
	if listenPort == localPort {
		logger.WithFields(logrus.Fields{
			"local_port":    localPort,
			"external_port": externalPort,
		}).Warn("外部端口与本地端口相同，使用随机端口避免冲突")
		listenPort = 0
		useRandomPort = true
	}

	// 根据协议类型选择不同的监听方式
	switch protocol {
	case "tcp":
		// 首先尝试使用目标端口
		listener, err := net.Listen(protocol, fmt.Sprintf(":%d", listenPort))
		if err != nil {
			// 如果目标端口被占用，尝试使用随机端口
			logger.WithFields(logrus.Fields{
				"target_port": externalPort,
				"error":       err,
			}).Warn("目标端口被占用，尝试使用随机端口")

			listenPort = 0 // 使用随机端口
			listener, err = net.Listen(protocol, fmt.Sprintf(":%d", listenPort))
			useRandomPort = true
		}

		if err != nil {
			hole.Status = types.MappingStatusFailed
			hole.Error = fmt.Sprintf("无法监听TCP端口: %v", err)
			holes[key] = hole
			return hole, fmt.Errorf("无法监听TCP端口: %w", err)
		}

		// 获取实际监听的端口
		actualPort := listener.Addr().(*net.TCPAddr).Port
		hole.ExternalPort = actualPort      // 更新为实际监听的端口
		hole.ExternalAddr = listener.Addr() // 设置外部地址

		// 启动TCP监听协程
		go handleTCP(listener, hole)

	case "udp":
		// 首先尝试使用目标端口（如果与本地端口不同）
		packetConn, err := net.ListenPacket(protocol, fmt.Sprintf(":%d", listenPort))
		if err != nil {
			// 如果目标端口被占用，尝试使用随机端口
			logger.WithFields(logrus.Fields{
				"target_port": externalPort,
				"error":       err,
			}).Warn("目标端口被占用，尝试使用随机端口")

			listenPort = 0 // 使用随机端口
			packetConn, err = net.ListenPacket(protocol, fmt.Sprintf(":%d", listenPort))
			useRandomPort = true
		}

		if err != nil {
			hole.Status = types.MappingStatusFailed
			hole.Error = fmt.Sprintf("无法监听UDP端口: %v", err)
			holes[key] = hole
			return hole, fmt.Errorf("无法监听UDP端口: %w", err)
		}

		// 获取实际监听的端口
		actualPort := packetConn.LocalAddr().(*net.UDPAddr).Port
		hole.ExternalPort = actualPort             // 更新为实际监听的端口
		hole.ExternalAddr = packetConn.LocalAddr() // 设置外部地址

		// 启动UDP监听协程
		go handleUDP(packetConn, hole)

	default:
		hole.Status = types.MappingStatusFailed
		hole.Error = fmt.Sprintf("不支持的协议: %s", protocol)
		holes[key] = hole
		return hole, fmt.Errorf("不支持的协议: %s", protocol)
	}

	// 启动外部连接建立过程（如果有的话）
	if establishExternal != nil {
		go establishExternal(hole)
	}

	holes[key] = hole

	portType := "目标端口"
	if useRandomPort {
		portType = "随机端口"
	}

	logger.WithFields(logrus.Fields{
		"local_port":    localPort,
		"external_port": hole.ExternalPort, // 使用实际监听的端口
		"protocol":      protocol,
		"type":          providerName,
		"port_type":     portType,
	}).Info("创建穿透成功")

	return hole, nil
}
