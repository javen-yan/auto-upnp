# 端口监控器分离架构

## 概述

为了支持手动映射端口监控，我们将原来的单一端口监控器分离为两个专门的监控器：

1. **自动端口监控器** (`AutoPortMonitor`)：监控配置的端口范围
2. **手动端口监控器** (`ManualPortMonitor`)：监控手动添加的端口

## 架构设计

### 自动端口监控器 (AutoPortMonitor)

**职责**：
- 监控配置文件中指定的端口范围
- 处理自动检测的端口状态变化
- 管理自动UPnP映射的生命周期

**特点**：
- 固定监控配置的端口范围
- 使用TCP协议检测端口状态
- 自动处理端口上线/下线事件

**文件位置**：`internal/portmonitor/auto_port_monitor.go`

### 手动端口监控器 (ManualPortMonitor)

**职责**：
- 监控手动添加的端口映射
- 支持TCP和UDP协议检测
- 处理手动映射的激活状态管理

**特点**：
- 动态添加/移除监控端口
- 支持多种协议类型
- 专门处理手动映射的状态变化

**文件位置**：`internal/portmonitor/manual_port_monitor.go`

## 核心功能对比

| 功能 | 自动端口监控器 | 手动端口监控器 |
|------|----------------|----------------|
| 监控范围 | 配置的端口范围 | 手动添加的端口 |
| 协议支持 | TCP | TCP/UDP |
| 端口管理 | 固定范围 | 动态添加/移除 |
| 回调参数 | `(port, isActive)` | `(port, isActive, protocol)` |
| 状态结构 | `AutoPortStatus` | `ManualPortStatus` |

## 数据结构

### AutoPortStatus
```go
type AutoPortStatus struct {
    Port     int
    IsActive bool
    LastSeen time.Time
}
```

### ManualPortStatus
```go
type ManualPortStatus struct {
    Port     int
    IsActive bool
    LastSeen time.Time
    Protocol string  // 新增协议字段
}
```

## 服务集成

### AutoUPnPService 更新

服务现在包含两个监控器实例：

```go
type AutoUPnPService struct {
    // ... 其他字段
    autoPortMonitor   *portmonitor.AutoPortMonitor
    manualPortMonitor *portmonitor.ManualPortMonitor
    // ... 其他字段
}
```

### 回调处理

1. **自动端口回调** (`onAutoPortStatusChanged`)：
   - 处理配置范围内的端口状态变化
   - 自动添加/删除UPnP映射

2. **手动端口回调** (`onManualPortStatusChanged`)：
   - 处理手动映射端口的状态变化
   - 更新手动映射的激活状态
   - 管理UPnP映射的注册/取消

## 工作流程

### 自动端口监控流程
1. 启动时初始化配置的端口范围
2. 定期检查每个端口的活跃状态
3. 端口状态变化时触发回调
4. 自动管理UPnP映射

### 手动端口监控流程
1. 添加手动映射时，将端口添加到监控器
2. 定期检查手动端口的活跃状态
3. 端口状态变化时更新映射的active字段
4. 根据状态注册/取消UPnP映射
5. 删除手动映射时，从监控器中移除端口

## 优势

### 1. 职责分离
- 自动监控器专注于配置范围内的端口
- 手动监控器专注于用户添加的端口
- 避免功能混淆和冲突

### 2. 灵活性
- 手动监控器支持动态端口管理
- 支持多种协议类型
- 可以监控任意端口，不受配置限制

### 3. 可维护性
- 代码结构更清晰
- 功能模块化，便于测试和维护
- 独立的错误处理和日志记录

### 4. 扩展性
- 可以为不同类型的监控器添加特定功能
- 支持不同的检测策略
- 便于添加新的监控类型

## 配置示例

### 自动端口监控配置
```yaml
port_range:
  start: 18000
  end: 19000
  step: 1

monitor:
  check_interval: 30s
  cleanup_interval: 5m
```

### 手动端口监控
- 无需配置，动态管理
- 使用与自动监控相同的检查间隔
- 支持TCP和UDP协议

## 使用场景

### 自动端口监控
- 监控已知的服务端口范围
- 自动发现和映射服务
- 适合标准化的部署环境

### 手动端口监控
- 监控用户自定义的服务端口
- 支持临时或特殊的端口映射
- 适合开发和测试环境

## 注意事项

1. **性能考虑**：两个监控器独立运行，避免相互影响
2. **资源管理**：手动监控器会动态添加端口，需要及时清理
3. **错误处理**：每个监控器有独立的错误处理机制
4. **日志记录**：不同类型的监控器使用不同的日志标识 