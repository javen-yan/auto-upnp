package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"auto-upnp/internal/port_mapping"

	"github.com/sirupsen/logrus"
)

// StoreService 存储服务
type StoreService struct {
	filePath           string
	logger             *logrus.Logger
	portMappingManager *port_mapping.PortMappingManager
	mutex              sync.RWMutex
}

// StoredMapping 存储的映射信息
type StoredMapping struct {
	InternalPort int    `json:"internal_port"`
	ExternalPort int    `json:"external_port"`
	Protocol     string `json:"protocol"`
	Description  string `json:"description"`
}

// NewStoreService 创建新的存储服务
func NewStoreService(dataDir string, logger *logrus.Logger, portMappingManager *port_mapping.PortMappingManager) *StoreService {
	if dataDir == "" {
		dataDir = "."
	}

	// 检查目录权限并尝试创建
	if err := ensureDataDir(dataDir, logger); err != nil {
		logger.WithError(err).Warnf("无法使用配置的数据目录 %s，将使用备用目录", dataDir)
		dataDir = "/tmp"
		// 再次尝试创建备用目录
		if err := ensureDataDir(dataDir, logger); err != nil {
			logger.WithError(err).Error("无法创建任何数据目录")
			os.Exit(1)
		}
	}

	filePath := filepath.Join(dataDir, "mappings.json")

	return &StoreService{
		filePath:           filePath,
		logger:             logger,
		portMappingManager: portMappingManager,
	}
}

// ensureDataDir 确保数据目录存在且有写权限
func ensureDataDir(dataDir string, logger *logrus.Logger) error {
	// 创建目录
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 测试写权限
	testFile := filepath.Join(dataDir, ".test_write")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return fmt.Errorf("目录无写权限: %w", err)
	}

	// 清理测试文件
	os.Remove(testFile)

	logger.Infof("使用数据目录: %s", dataDir)
	return nil
}

func (ss *StoreService) Start() error {
	ss.logger.Info("启动存储服务")
	return ss.Recover()
}

func (ss *StoreService) Stop() {
	ss.logger.Info("停止存储服务")
}

func (ss *StoreService) Recover() error {
	ss.logger.Info("恢复存储服务")

	ss.mutex.Lock()
	defer ss.mutex.Unlock()

	mappings := ss.loadFromFile()
	if mappings == nil {
		ss.logger.Info("没有找到存储的映射文件")
		return nil // 没有存储的映射不是错误
	}

	ss.logger.Infof("找到 %d 个存储的映射，开始恢复", len(mappings))

	successCount := 0
	failCount := 0

	for i, m := range mappings {
		ss.logger.Debugf("恢复映射 %d/%d: %d->%d %s", i+1, len(mappings), m.InternalPort, m.ExternalPort, m.Protocol)

		_, err := ss.portMappingManager.CreateMapping(
			m.InternalPort,
			m.ExternalPort,
			m.Protocol,
			m.Description,
			port_mapping.MappingAddTypeManual,
		)
		if err != nil {
			ss.logger.WithError(err).Warnf("恢复映射失败: %d->%d %s", m.InternalPort, m.ExternalPort, m.Protocol)
			failCount++
			continue
		}

		successCount++
		ss.logger.Debugf("映射恢复成功: %d->%d %s", m.InternalPort, m.ExternalPort, m.Protocol)
	}

	ss.logger.Infof("存储服务恢复完成: 成功 %d 个, 失败 %d 个", successCount, failCount)
	return nil
}

func (ss *StoreService) Add(internalPort int, externalPort int, protocol string, description string) error {
	ss.logger.Infof("添加存储服务 %d %d %s %s", internalPort, externalPort, protocol, description)

	ss.mutex.Lock()
	defer ss.mutex.Unlock()

	mappings := ss.loadFromFile()
	if mappings == nil {
		mappings = make([]*StoredMapping, 0)
	}

	// 检查是否已存在
	for _, m := range mappings {
		if m.InternalPort == internalPort && m.ExternalPort == externalPort && m.Protocol == protocol {
			return nil
		}
	}

	// 添加新映射
	mappings = append(mappings, &StoredMapping{
		InternalPort: internalPort,
		ExternalPort: externalPort,
		Protocol:     protocol,
		Description:  description,
	})

	return ss.saveToFile(mappings)
}

func (ss *StoreService) Remove(internalPort int, externalPort int, protocol string) error {
	ss.logger.Infof("删除存储服务 %d %d %s", internalPort, externalPort, protocol)

	ss.mutex.Lock()
	defer ss.mutex.Unlock()

	mappings := ss.loadFromFile()
	if mappings == nil {
		return nil
	}

	// 查找并删除匹配的映射
	newMappings := make([]*StoredMapping, 0, len(mappings))
	for _, m := range mappings {
		if !(m.InternalPort == internalPort && m.ExternalPort == externalPort && m.Protocol == protocol) {
			newMappings = append(newMappings, m)
		}
	}

	return ss.saveToFile(newMappings)
}

func (ss *StoreService) loadFromFile() []*StoredMapping {
	ss.logger.Debugf("从文件加载映射: %s", ss.filePath)

	// 检查文件是否存在
	if _, err := os.Stat(ss.filePath); os.IsNotExist(err) {
		return nil
	}

	file, err := os.Open(ss.filePath)
	if err != nil {
		ss.logger.WithError(err).Error("无法打开文件")
		return nil
	}
	defer file.Close()

	var mappings []*StoredMapping
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&mappings); err != nil {
		ss.logger.WithError(err).Error("无法解析文件")
		return nil
	}

	return mappings
}

func (ss *StoreService) saveToFile(mappings []*StoredMapping) error {
	ss.logger.Debugf("保存映射到文件: %s", ss.filePath)

	file, err := os.Create(ss.filePath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(mappings); err != nil {
		return fmt.Errorf("编码JSON失败: %w", err)
	}

	return nil
}
