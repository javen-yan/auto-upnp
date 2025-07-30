package nat_traversal

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// NATTraversalConfig NAT穿透配置
type NATTraversalConfig struct {
	Enabled     bool     `mapstructure:"enabled"`
	UseSTUN     bool     `mapstructure:"use_stun"`
	STUNServers []string `mapstructure:"stun_servers"`
}

// HoleInfo 打洞信息
type HoleInfo struct {
	LocalPort    int
	Protocol     string
	Description  string
	CreatedAt    time.Time
	LastActivity time.Time
	IsActive     bool
	RemoteAddr   net.Addr
}

// NATTraversal NAT穿透管理器
type NATTraversal struct {
	config *NATTraversalConfig
	logger *logrus.Logger
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// STUN客户端
	stunClient   *STUNClient
	externalAddr *net.UDPAddr

	// 打洞信息
	holes      map[string]*HoleInfo
	holesMutex sync.RWMutex

	// 回调函数
	onHoleCreated func(port int, protocol string)
	onHoleClosed  func(port int, protocol string)
	onHoleFailed  func(port int, protocol string, error error)
}

// NewNATTraversal 创建新的NAT穿透管理器
func NewNATTraversal(config *NATTraversalConfig, logger *logrus.Logger) *NATTraversal {
	ctx, cancel := context.WithCancel(context.Background())

	nt := &NATTraversal{
		config: config,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
		holes:  make(map[string]*HoleInfo),
	}

	// 如果启用STUN，创建STUN客户端
	if config.UseSTUN {
		nt.stunClient = NewSTUNClient(logger, config.STUNServers)
	}

	return nt
}

// Start 启动NAT穿透服务
func (nt *NATTraversal) Start() error {
	if !nt.config.Enabled {
		nt.logger.Info("NAT穿透功能已禁用")
		return nil
	}

	nt.logger.Info("启动NAT穿透服务")

	// 如果启用STUN，发现外部地址
	if nt.config.UseSTUN && nt.stunClient != nil {
		nt.logger.Info("使用STUN服务器发现外部地址")

		response, err := nt.stunClient.DiscoverExternalAddress()
		if err != nil {
			nt.logger.WithError(err).Warn("STUN发现失败")
			return fmt.Errorf("STUN发现失败: %w", err)
		} else {
			nt.externalAddr = &net.UDPAddr{
				IP:   response.ExternalIP,
				Port: response.ExternalPort,
			}
			nt.logger.WithFields(logrus.Fields{
				"external_ip":   response.ExternalIP.String(),
				"external_port": response.ExternalPort,
			}).Info("STUN发现外部地址成功")
		}
	}

	nt.logger.Info("NAT穿透服务启动成功")
	return nil
}

// Stop 停止NAT穿透服务
func (nt *NATTraversal) Stop() {
	nt.logger.Info("停止NAT穿透服务")
	nt.cancel()

	if nt.stunClient != nil {
		nt.stunClient.Close()
	}

	nt.wg.Wait()
	nt.logger.Info("NAT穿透服务已停止")
}

// CreateHole 创建打洞
func (nt *NATTraversal) CreateHole(port int, protocol string, description string) error {
	if !nt.config.Enabled {
		return fmt.Errorf("NAT穿透功能已禁用")
	}

	holeKey := fmt.Sprintf("%d-%s", port, protocol)

	nt.holesMutex.Lock()
	defer nt.holesMutex.Unlock()

	// 检查是否已存在
	if _, exists := nt.holes[holeKey]; exists {
		return fmt.Errorf("打洞已存在: %s", holeKey)
	}

	hole := &HoleInfo{
		LocalPort:    port,
		Protocol:     protocol,
		Description:  description,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		IsActive:     true,
	}

	// 如果使用STUN，设置外部地址
	if nt.externalAddr != nil {
		hole.RemoteAddr = nt.externalAddr
	}

	nt.holes[holeKey] = hole

	nt.logger.WithFields(logrus.Fields{
		"port":        port,
		"protocol":    protocol,
		"description": description,
		"use_stun":    nt.config.UseSTUN,
	}).Info("创建打洞成功")

	// 触发回调
	if nt.onHoleCreated != nil {
		nt.onHoleCreated(port, protocol)
	}

	return nil
}

// CloseHole 关闭打洞
func (nt *NATTraversal) CloseHole(port int, protocol string) error {
	holeKey := fmt.Sprintf("%d-%s", port, protocol)

	nt.holesMutex.Lock()
	defer nt.holesMutex.Unlock()

	hole, exists := nt.holes[holeKey]
	if !exists {
		return fmt.Errorf("打洞不存在: %s", holeKey)
	}

	hole.IsActive = false
	delete(nt.holes, holeKey)

	nt.logger.WithFields(logrus.Fields{
		"port":     port,
		"protocol": protocol,
	}).Info("关闭打洞成功")

	// 触发回调
	if nt.onHoleClosed != nil {
		nt.onHoleClosed(port, protocol)
	}

	return nil
}

// GetHoles 获取所有打洞信息
func (nt *NATTraversal) GetHoles() map[string]*HoleInfo {
	nt.holesMutex.RLock()
	defer nt.holesMutex.RUnlock()

	result := make(map[string]*HoleInfo)
	for key, hole := range nt.holes {
		result[key] = hole
	}
	return result
}

// GetActiveHoles 获取活跃的打洞
func (nt *NATTraversal) GetActiveHoles() []*HoleInfo {
	nt.holesMutex.RLock()
	defer nt.holesMutex.RUnlock()

	var activeHoles []*HoleInfo
	for _, hole := range nt.holes {
		if hole.IsActive {
			activeHoles = append(activeHoles, hole)
		}
	}
	return activeHoles
}

// GetExternalAddress 获取外部地址信息
func (nt *NATTraversal) GetExternalAddress() *net.UDPAddr {
	return nt.externalAddr
}

// SetCallbacks 设置回调函数
func (nt *NATTraversal) SetCallbacks(
	onHoleCreated func(port int, protocol string),
	onHoleClosed func(port int, protocol string),
	onHoleFailed func(port int, protocol string, error error),
) {
	nt.onHoleCreated = onHoleCreated
	nt.onHoleClosed = onHoleClosed
	nt.onHoleFailed = onHoleFailed
}
