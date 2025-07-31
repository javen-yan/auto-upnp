package portmapping

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"auto-upnp/internal/nat_traversal"
	"auto-upnp/internal/util"

	"github.com/sirupsen/logrus"
)

// TURNProvider TURN端口映射提供者
type TURNProvider struct {
	logger       *logrus.Logger
	ctx          context.Context
	cancel       context.CancelFunc
	mappings     map[string]*PortMapping
	mutex        sync.RWMutex
	available    bool
	externalAddr *net.UDPAddr

	// NAT穿透管理器
	natTraversal *nat_traversal.NATTraversal

	// TURN相关配置
	turnServers []nat_traversal.TURNServer
}

// NewTURNProvider 创建新的TURN提供者
func NewTURNProvider(logger *logrus.Logger, config map[string]interface{}) *TURNProvider {
	ctx, cancel := context.WithCancel(context.Background())

	provider := &TURNProvider{
		logger:    logger,
		ctx:       ctx,
		cancel:    cancel,
		mappings:  make(map[string]*PortMapping),
		available: false,
	}

	// 从配置中读取TURN服务器列表
	if turnServers, ok := config["turn_servers"].([]map[string]interface{}); ok {
		provider.turnServers = make([]nat_traversal.TURNServer, 0, len(turnServers))
		for _, serverMap := range turnServers {
			turnServer := nat_traversal.TURNServer{}
			if host, ok := serverMap["host"].(string); ok {
				turnServer.Host = host
			}
			if port, ok := serverMap["port"].(int); ok {
				turnServer.Port = port
			}
			if username, ok := serverMap["username"].(string); ok {
				turnServer.Username = username
			}
			if password, ok := serverMap["password"].(string); ok {
				turnServer.Password = password
			}
			if realm, ok := serverMap["realm"].(string); ok {
				turnServer.Realm = realm
			}
			provider.turnServers = append(provider.turnServers, turnServer)
		}
	} else {
		provider.logger.WithField("turn_servers_type", fmt.Sprintf("%T", config["turn_servers"])).Warn("TURN服务器配置类型不匹配")
		provider.turnServers = []nat_traversal.TURNServer{}
	}

	provider.logger.WithFields(logrus.Fields{
		"turn_servers_count": len(provider.turnServers),
		"turn_servers":       provider.turnServers,
	}).Info("TURN服务器列表")

	return provider
}

// Type 返回提供者类型
func (tp *TURNProvider) Type() MappingType {
	return MappingTypeTURN
}

// Name 返回提供者名称
func (tp *TURNProvider) Name() string {
	return "TURN端口映射"
}

// IsAvailable 检查是否可用
func (tp *TURNProvider) IsAvailable() bool {
	return tp.available && tp.natTraversal != nil
}

// Start 启动TURN提供者
func (tp *TURNProvider) Start(checkStatusTaskTime time.Duration) error {
	tp.logger.Info("启动TURN端口映射提供者")

	// 创建NAT穿透配置
	natConfig := &nat_traversal.NATTraversalConfig{
		Enabled:     true,
		UseTURN:     true,
		TURNServers: tp.turnServers,
	}

	// 创建NAT穿透管理器
	tp.natTraversal = nat_traversal.NewNATTraversal(natConfig, tp.logger)

	// 启动NAT穿透服务
	if err := tp.natTraversal.Start(); err != nil {
		tp.logger.WithError(err).Warn("NAT穿透服务启动失败")
		tp.available = false
		return fmt.Errorf("NAT穿透服务启动失败: %w", err)
	}

	// 获取外部地址
	// tp.externalAddr = tp.natTraversal.GetExternalAddress()
	// if tp.externalAddr == nil {
	// 	tp.logger.Warn("无法获取外部地址")
	// 	tp.available = false
	// 	return fmt.Errorf("无法获取外部地址")
	// }

	tp.available = true

	// tp.logger.WithFields(logrus.Fields{
	// 	"external_ip":   tp.externalAddr.IP.String(),
	// 	"external_port": tp.externalAddr.Port,
	// }).Info("TURN端口映射提供者启动成功")

	// 启动检查端口状态任务
	go tp.checkStatusTask(checkStatusTaskTime)

	return nil
}

// Stop 停止TURN提供者
func (tp *TURNProvider) Stop() error {
	tp.logger.Info("停止TURN端口映射提供者")
	tp.cancel()

	if tp.natTraversal != nil {
		tp.natTraversal.Stop()
	}

	tp.available = false
	tp.externalAddr = nil
	return nil
}

// CreateMapping 创建TURN端口映射
func (tp *TURNProvider) CreateMapping(port int, externalPort int, protocol, description string, addType MappingAddType) (*PortMapping, error) {
	if !tp.IsAvailable() {
		return nil, fmt.Errorf("TURN提供者不可用")
	}

	// 使用分配的外部端口创建映射键
	mappingKey := fmt.Sprintf("%d:%d:%s", port, externalPort, protocol)

	tp.mutex.Lock()
	defer tp.mutex.Unlock()

	// 为这个端口创建独立的TURN打洞
	externalAddr, err := tp.createTURNHole(port, protocol)
	if err != nil {
		tp.logger.WithFields(logrus.Fields{
			"port":          port,
			"external_port": externalPort,
			"protocol":      protocol,
			"error":         err,
		}).Error("TURN打洞创建失败")
		return nil, fmt.Errorf("TURN打洞创建失败: %w", err)
	}

	// 创建端口映射记录
	mapping := &PortMapping{
		InternalPort: port,
		ExternalPort: externalPort, // 使用分配的外部端口
		Protocol:     protocol,
		Description:  description,
		AddType:      addType,
		Type:         MappingTypeTURN,
		Status:       MappingStatusActive,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		ExternalAddr: externalAddr, // 使用独立的外部地址
	}

	tp.mappings[mappingKey] = mapping

	tp.logger.WithFields(logrus.Fields{
		"port":          port,
		"external_port": externalPort,
		"protocol":      protocol,
		"type":          MappingTypeTURN,
		"external_addr": externalAddr.String(),
		"mapping_key":   mappingKey,
	}).Info("TURN端口映射创建成功")

	return mapping, nil
}

// RemoveMapping 移除TURN端口映射
func (tp *TURNProvider) RemoveMapping(port int, externalPort int, protocol string, addType MappingAddType) error {

	tp.mutex.Lock()
	defer tp.mutex.Unlock()

	var matchedMapping *PortMapping

	for _, mapping := range tp.mappings {
		if mapping.InternalPort == port && mapping.Protocol == protocol {
			matchedMapping = mapping
			break
		}
	}

	if matchedMapping == nil {
		return fmt.Errorf("端口映射不存在: %d:%d:%s", port, externalPort, protocol)
	}

	mappingKey := fmt.Sprintf("%d:%d:%s", port, matchedMapping.ExternalPort, protocol)
	_, exists := tp.mappings[mappingKey]
	if !exists {
		return fmt.Errorf("端口映射不存在: %s", mappingKey)
	}

	// 使用NAT穿透关闭打洞
	err := tp.natTraversal.CloseHole(port, matchedMapping.ExternalPort, protocol)
	if err != nil {
		tp.logger.WithFields(logrus.Fields{
			"port":          port,
			"external_port": matchedMapping.ExternalPort,
			"protocol":      protocol,
			"error":         err,
		}).Error("TURN端口映射移除失败")
		return fmt.Errorf("TURN端口映射移除失败: %w", err)
	}

	delete(tp.mappings, mappingKey)

	tp.logger.WithFields(logrus.Fields{
		"port":          port,
		"external_port": matchedMapping.ExternalPort,
		"protocol":      protocol,
		"type":          MappingTypeTURN,
	}).Info("TURN端口映射移除成功")

	return nil
}

// createTURNHole 为指定端口创建独立的TURN打洞
func (tp *TURNProvider) createTURNHole(port int, protocol string) (net.Addr, error) {
	// 使用TURN客户端为这个端口创建独立的打洞
	if tp.natTraversal != nil {
		// 创建打洞记录，在外部端口上监听，转发到本地端口
		externalAddr, err := tp.natTraversal.CreateHole(port, protocol, fmt.Sprintf("TURN-%d", port))
		if err != nil {
			return nil, fmt.Errorf("创建TURN打洞失败: %w", err)
		}

		return externalAddr, nil
	}

	return nil, fmt.Errorf("TURN客户端不可用")
}

// GetMappings 获取所有TURN映射
func (tp *TURNProvider) GetMappings() map[string]*PortMapping {
	tp.mutex.RLock()
	defer tp.mutex.RUnlock()

	result := make(map[string]*PortMapping)
	for key, mapping := range tp.mappings {
		result[key] = mapping
	}
	return result
}

// GetStatus 获取TURN提供者状态
func (tp *TURNProvider) GetStatus() map[string]interface{} {
	tp.mutex.RLock()
	defer tp.mutex.RUnlock()

	activeCount := 0
	for _, mapping := range tp.mappings {
		if mapping.Status == MappingStatusActive {
			activeCount++
		}
	}

	status := map[string]interface{}{
		"available":       tp.IsAvailable(),
		"total_mappings":  len(tp.mappings),
		"active_mappings": activeCount,
		"turn_servers":    tp.turnServers,
	}

	if tp.externalAddr != nil {
		status["external_address"] = map[string]interface{}{
			"ip":   tp.externalAddr.IP.String(),
			"port": tp.externalAddr.Port,
		}
	}

	// 如果NAT穿透管理器可用，添加其状态信息
	if tp.natTraversal != nil {
		holes := tp.natTraversal.GetHoles()
		activeHoles := tp.natTraversal.GetActiveHoles()

		status["nat_traversal"] = map[string]interface{}{
			"total_holes":   len(holes),
			"active_holes":  len(activeHoles),
			"external_addr": tp.natTraversal.GetExternalAddress(),
		}
	}

	return status
}

func (tp *TURNProvider) checkStatusTask(tickerTime time.Duration) {
	tp.logger.Info("检查端口状态任务启动")

	if tickerTime == 0 {
		tickerTime = 5 * time.Second
	}

	ticker := time.NewTicker(tickerTime)
	defer ticker.Stop()

	for {
		select {
		case <-tp.ctx.Done():
			tp.logger.Info("检查端口状态任务停止")
			return
		case <-ticker.C:
			tp.checkPortStatus()
		}
	}
}

func (tp *TURNProvider) checkPortStatus() {
	tp.mutex.RLock()
	allManualMappings := make([]*PortMapping, 0)
	for _, mapping := range tp.mappings {
		if mapping.AddType == MappingAddTypeManual {
			allManualMappings = append(allManualMappings, mapping)
		}
	}
	tp.mutex.RUnlock()

	for _, mapping := range allManualMappings {
		oldMapStatus := mapping.Status
		portStatus := util.IsPortActive(mapping.InternalPort)
		newMapStatus := MappingStatusInactive
		if portStatus.Open {
			newMapStatus = MappingStatusActive
		}
		if oldMapStatus != newMapStatus {
			tp.updateMappingStatus(mapping, newMapStatus)
		}
	}
}

func (tp *TURNProvider) updateMappingStatus(mapping *PortMapping, status MappingStatus) {
	tp.mutex.Lock()
	defer tp.mutex.Unlock()

	if mapping.Status == status {
		return
	}
	tp.logger.WithFields(logrus.Fields{
		"port":          mapping.InternalPort,
		"external_port": mapping.ExternalPort,
		"protocol":      mapping.Protocol,
	}).Info("端口状态发生变化")
	mapping.Status = status
}
