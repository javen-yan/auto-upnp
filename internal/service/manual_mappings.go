package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// ManualMapping 手动端口映射记录
type ManualMapping struct {
	InternalPort int    `json:"internal_port"`
	ExternalPort int    `json:"external_port"`
	Protocol     string `json:"protocol"`
	Description  string `json:"description"`
	CreatedAt    string `json:"created_at"`
}

// ManualMappingManager 手动映射管理器
type ManualMappingManager struct {
	filePath string
	logger   *logrus.Logger
	mutex    sync.RWMutex
	mappings map[string]*ManualMapping // key: "internalPort:externalPort:protocol"
}

// NewManualMappingManager 创建手动映射管理器
func NewManualMappingManager(dataDir string, logger *logrus.Logger) *ManualMappingManager {
	if dataDir == "" {
		dataDir = "."
	}

	filePath := filepath.Join(dataDir, "manual_mappings.json")

	return &ManualMappingManager{
		filePath: filePath,
		logger:   logger,
		mappings: make(map[string]*ManualMapping),
	}
}

// LoadMappings 从文件加载手动映射
func (mm *ManualMappingManager) LoadMappings() error {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	// 确保目录存在
	dir := filepath.Dir(mm.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 检查文件是否存在
	if _, err := os.Stat(mm.filePath); os.IsNotExist(err) {
		mm.logger.Info("手动映射文件不存在，将创建新文件")
		return nil
	}

	// 读取文件
	data, err := os.ReadFile(mm.filePath)
	if err != nil {
		return fmt.Errorf("读取手动映射文件失败: %w", err)
	}

	// 解析JSON
	var mappings []*ManualMapping
	if err := json.Unmarshal(data, &mappings); err != nil {
		return fmt.Errorf("解析手动映射文件失败: %w", err)
	}

	// 加载到内存
	mm.mappings = make(map[string]*ManualMapping)
	for _, mapping := range mappings {
		key := mm.getMappingKey(mapping.InternalPort, mapping.ExternalPort, mapping.Protocol)
		mm.mappings[key] = mapping
	}

	mm.logger.Infof("成功加载 %d 个手动映射", len(mappings))
	return nil
}

// SaveMappings 保存手动映射到文件
func (mm *ManualMappingManager) SaveMappings() error {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()

	// 转换为切片
	mappings := make([]*ManualMapping, 0, len(mm.mappings))
	for _, mapping := range mm.mappings {
		mappings = append(mappings, mapping)
	}

	// 序列化为JSON
	data, err := json.MarshalIndent(mappings, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化手动映射失败: %w", err)
	}

	// 确保目录存在
	dir := filepath.Dir(mm.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(mm.filePath, data, 0644); err != nil {
		return fmt.Errorf("写入手动映射文件失败: %w", err)
	}

	mm.logger.Infof("成功保存 %d 个手动映射到文件", len(mappings))
	return nil
}

// AddMapping 添加手动映射
func (mm *ManualMappingManager) AddMapping(internalPort, externalPort int, protocol, description string) error {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	key := mm.getMappingKey(internalPort, externalPort, protocol)

	mapping := &ManualMapping{
		InternalPort: internalPort,
		ExternalPort: externalPort,
		Protocol:     protocol,
		Description:  description,
		CreatedAt:    time.Now().Format(time.RFC3339),
	}

	mm.mappings[key] = mapping

	// 保存到文件
	return mm.saveMappingsUnsafe()
}

// RemoveMapping 删除手动映射
func (mm *ManualMappingManager) RemoveMapping(internalPort, externalPort int, protocol string) error {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	key := mm.getMappingKey(internalPort, externalPort, protocol)

	if _, exists := mm.mappings[key]; !exists {
		return fmt.Errorf("手动映射不存在: %s", key)
	}

	delete(mm.mappings, key)

	// 保存到文件
	return mm.saveMappingsUnsafe()
}

// GetMappings 获取所有手动映射
func (mm *ManualMappingManager) GetMappings() []*ManualMapping {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()

	mappings := make([]*ManualMapping, 0, len(mm.mappings))
	for _, mapping := range mm.mappings {
		mappings = append(mappings, mapping)
	}
	return mappings
}

// GetMapping 获取指定映射
func (mm *ManualMappingManager) GetMapping(internalPort, externalPort int, protocol string) (*ManualMapping, bool) {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()

	key := mm.getMappingKey(internalPort, externalPort, protocol)
	mapping, exists := mm.mappings[key]
	return mapping, exists
}

// getMappingKey 生成映射键
func (mm *ManualMappingManager) getMappingKey(internalPort, externalPort int, protocol string) string {
	return fmt.Sprintf("%d:%d:%s", internalPort, externalPort, protocol)
}

// saveMappingsUnsafe 不安全保存（调用者需要持有锁）
func (mm *ManualMappingManager) saveMappingsUnsafe() error {
	// 转换为切片
	mappings := make([]*ManualMapping, 0, len(mm.mappings))
	for _, mapping := range mm.mappings {
		mappings = append(mappings, mapping)
	}

	// 序列化为JSON
	data, err := json.MarshalIndent(mappings, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化手动映射失败: %w", err)
	}

	// 确保目录存在
	dir := filepath.Dir(mm.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(mm.filePath, data, 0644); err != nil {
		return fmt.Errorf("写入手动映射文件失败: %w", err)
	}

	return nil
}
