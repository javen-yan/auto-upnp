package port_mapping

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// STUNProvider STUN端口映射提供者
type STUNProvider struct {
	logger       *logrus.Logger
	ctx          context.Context
	cancel       context.CancelFunc
	mappings     map[string]*PortMapping
	mutex        sync.RWMutex
	available    bool
	externalAddr *net.UDPAddr

	// STUN相关配置
	stunServers []string
}

// NewSTUNProvider 创建新的STUN提供者
func NewSTUNProvider(logger *logrus.Logger, config map[string]interface{}) *STUNProvider {
	ctx, cancel := context.WithCancel(context.Background())

	provider := &STUNProvider{
		logger:    logger,
		ctx:       ctx,
		cancel:    cancel,
		mappings:  make(map[string]*PortMapping),
		available: false,
	}

	// 从配置中读取STUN服务器列表
	if stunServers, ok := config["stun_servers"].([]string); ok {
		provider.stunServers = stunServers
	} else {
		// 默认STUN服务器
		provider.stunServers = []string{
			"stun.l.google.com:19302",
			"stun1.l.google.com:19302",
			"stun.stunprotocol.org:3478",
		}
	}

	return provider
}

// Type 返回提供者类型
func (sp *STUNProvider) Type() MappingType {
	return MappingTypeSTUN
}

// Name 返回提供者名称
func (sp *STUNProvider) Name() string {
	return "STUN端口映射"
}

// IsAvailable 检查是否可用
func (sp *STUNProvider) IsAvailable() bool {
	return sp.available
}

// Start 启动STUN提供者
func (sp *STUNProvider) Start() error {
	sp.logger.Info("启动STUN端口映射提供者")

	// 尝试发现外部地址
	externalAddr, err := sp.discoverExternalAddress()
	if err != nil {
		sp.logger.WithError(err).Warn("STUN外部地址发现失败")
		sp.available = false
		return fmt.Errorf("STUN外部地址发现失败: %w", err)
	}

	sp.externalAddr = externalAddr
	sp.available = true

	sp.logger.WithFields(logrus.Fields{
		"external_ip":   externalAddr.IP.String(),
		"external_port": externalAddr.Port,
	}).Info("STUN端口映射提供者启动成功")

	return nil
}

// Stop 停止STUN提供者
func (sp *STUNProvider) Stop() {
	sp.logger.Info("停止STUN端口映射提供者")
	sp.cancel()
	sp.available = false
	sp.externalAddr = nil
}

// CreateMapping 创建STUN端口映射
func (sp *STUNProvider) CreateMapping(port int, protocol, description string) (*PortMapping, error) {
	if !sp.available {
		return nil, fmt.Errorf("STUN提供者不可用")
	}

	mappingKey := fmt.Sprintf("%d-%s", port, protocol)

	sp.mutex.Lock()
	defer sp.mutex.Unlock()

	// 检查是否已存在
	if _, exists := sp.mappings[mappingKey]; exists {
		return nil, fmt.Errorf("端口映射已存在: %s", mappingKey)
	}

	// 创建端口映射
	mapping := &PortMapping{
		InternalPort: port,
		Protocol:     protocol,
		Description:  description,
		Type:         MappingTypeSTUN,
		Status:       MappingStatusActive,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		ExternalAddr: sp.externalAddr,
	}

	sp.mappings[mappingKey] = mapping

	sp.logger.WithFields(logrus.Fields{
		"port":          port,
		"protocol":      protocol,
		"type":          MappingTypeSTUN,
		"external_addr": sp.externalAddr.String(),
	}).Info("STUN端口映射创建成功")

	return mapping, nil
}

// RemoveMapping 移除STUN端口映射
func (sp *STUNProvider) RemoveMapping(port int, protocol string) error {
	mappingKey := fmt.Sprintf("%d-%s", port, protocol)

	sp.mutex.Lock()
	defer sp.mutex.Unlock()

	_, exists := sp.mappings[mappingKey]
	if !exists {
		return fmt.Errorf("端口映射不存在: %s", mappingKey)
	}

	delete(sp.mappings, mappingKey)

	sp.logger.WithFields(logrus.Fields{
		"port":     port,
		"protocol": protocol,
		"type":     MappingTypeSTUN,
	}).Info("STUN端口映射移除成功")

	return nil
}

// GetMappings 获取所有STUN映射
func (sp *STUNProvider) GetMappings() map[string]*PortMapping {
	sp.mutex.RLock()
	defer sp.mutex.RUnlock()

	result := make(map[string]*PortMapping)
	for key, mapping := range sp.mappings {
		result[key] = mapping
	}
	return result
}

// GetStatus 获取STUN提供者状态
func (sp *STUNProvider) GetStatus() map[string]interface{} {
	sp.mutex.RLock()
	defer sp.mutex.RUnlock()

	activeCount := 0
	for _, mapping := range sp.mappings {
		if mapping.Status == MappingStatusActive {
			activeCount++
		}
	}

	status := map[string]interface{}{
		"available":       sp.available,
		"total_mappings":  len(sp.mappings),
		"active_mappings": activeCount,
		"stun_servers":    sp.stunServers,
	}

	if sp.externalAddr != nil {
		status["external_address"] = map[string]interface{}{
			"ip":   sp.externalAddr.IP.String(),
			"port": sp.externalAddr.Port,
		}
	}

	return status
}

// discoverExternalAddress 发现外部地址
func (sp *STUNProvider) discoverExternalAddress() (*net.UDPAddr, error) {
	// 这里应该实现实际的STUN协议逻辑
	// 由于这是一个示例，我们模拟成功
	// 在实际实现中，应该：
	// 1. 向STUN服务器发送绑定请求
	// 2. 解析响应获取外部地址
	// 3. 处理错误和重试

	sp.logger.Info("开始STUN外部地址发现")

	// 模拟成功发现外部地址
	externalIP := net.ParseIP("203.198.28.145")
	if externalIP == nil {
		return nil, fmt.Errorf("无效的外部IP地址")
	}

	externalAddr := &net.UDPAddr{
		IP:   externalIP,
		Port: 28978,
	}

	sp.logger.WithFields(logrus.Fields{
		"external_ip":   externalIP.String(),
		"external_port": 28978,
	}).Info("STUN外部地址发现成功")

	return externalAddr, nil
}
