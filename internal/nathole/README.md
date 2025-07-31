# NAT穿透模块

本模块实现了NAT1-3类型的NAT穿透功能，支持完全锥形NAT、受限锥形NAT和端口受限锥形NAT。

## NAT类型说明

### NAT1 (完全锥形NAT)
- **特点**: 外部主机可以连接到任何端口
- **实现**: 直接监听本地端口，任何外部主机都可以连接
- **适用场景**: 最宽松的NAT类型，穿透成功率最高

### NAT2 (受限锥形NAT)
- **特点**: 只有之前连接过的外部主机才能连接
- **实现**: 记录已连接的外部主机IP，只允许这些主机连接
- **适用场景**: 中等限制的NAT类型

### NAT3 (端口受限锥形NAT)
- **特点**: 只有之前连接过的外部主机的特定端口才能连接
- **实现**: 记录已连接的外部主机IP和端口，只允许这些主机和端口连接
- **适用场景**: 较严格的NAT类型

## 核心组件

### NATHoleProvider 接口
定义了NAT穿透提供者的标准接口：

```go
type NATHoleProvider interface {
    Type() types.NATType
    Name() string
    IsAvailable() bool
    CreateHole(localPort int, externalPort int, protocol string, description string) (*NATHole, error)
    RemoveHole(localPort int, externalPort int, protocol string) error
    GetHoles() map[string]*NATHole
    GetStatus() map[string]interface{}
    Start() error
    Stop() error
}
```

### NATHole 结构
表示一个NAT穿透：

```go
type NATHole struct {
    LocalPort    int           `json:"local_port"`
    ExternalPort int           `json:"external_port"`
    Protocol     string        `json:"protocol"`
    Description  string        `json:"description"`
    Type         types.NATType `json:"type"`
    Status       HoleStatus    `json:"status"`
    CreatedAt    time.Time     `json:"created_at"`
    LastActivity time.Time     `json:"last_activity"`
    ExternalAddr net.Addr      `json:"external_addr,omitempty"`
    Error        string        `json:"error,omitempty"`
}
```

### NATHolePunching 管理器
统一管理NAT穿透的创建、移除和状态监控：

```go
type NATHolePunching struct {
    logger   *logrus.Logger
    ctx      context.Context
    cancel   context.CancelFunc
    natInfo  *types.NATInfo
    provider NATHoleProvider
    
    // 回调函数
    onHoleCreated func(localPort int, externalPort int, protocol string, natType types.NATType)
    onHoleRemoved func(localPort int, externalPort int, protocol string, natType types.NATType)
    onHoleFailed  func(localPort int, externalPort int, protocol string, natType types.NATType, error error)
}
```

## 使用方法

### 1. 直接使用提供者

```go
// 创建NAT1提供者
logger := logrus.New()
config := map[string]interface{}{}
provider := NewNAT1Provider(logger, config)

// 启动提供者
if err := provider.Start(); err != nil {
    log.Fatal(err)
}

// 创建穿透
hole, err := provider.CreateHole(8080, 8080, "tcp", "Web服务")
if err != nil {
    log.Fatal(err)
}

// 获取状态
status := provider.GetStatus()
fmt.Printf("状态: %+v\n", status)

// 清理资源
provider.RemoveHole(8080, 8080, "tcp")
provider.Stop()
```

### 2. 使用工厂模式

```go
// 根据NAT类型创建提供者
provider, err := CreateNATHoleProvider(types.NATType1, logger, config)
if err != nil {
    log.Fatal(err)
}

// 使用提供者...
```

### 3. 使用管理器

```go
// 创建NAT信息
natInfo := &types.NATInfo{
    Type:        types.NATType1,
    Description: "示例NAT",
}

// 创建管理器
punching := NewNATHolePunching(logger, natInfo)

// 设置回调
punching.SetCallbacks(
    func(localPort, externalPort int, protocol string, natType types.NATType) {
        fmt.Printf("穿透创建: %d:%d %s\n", localPort, externalPort, protocol)
    },
    func(localPort, externalPort int, protocol string, natType types.NATType) {
        fmt.Printf("穿透移除: %d:%d %s\n", localPort, externalPort, protocol)
    },
    func(localPort, externalPort int, protocol string, natType types.NATType, err error) {
        fmt.Printf("穿透失败: %d:%d %s - %v\n", localPort, externalPort, protocol, err)
    },
)

// 启动管理器
if err := punching.Start(); err != nil {
    log.Fatal(err)
}

// 创建穿透
hole, err := punching.CreateHole(8080, 8080, "tcp", "服务")
if err != nil {
    log.Fatal(err)
}

// 清理
punching.Stop()
```

## 测试

运行测试：

```bash
go test ./internal/nathole -v
```

测试覆盖了：
- NAT1、NAT2、NAT3提供者的基本功能
- 工厂模式的正确性
- 管理器的完整流程
- 错误处理

## 注意事项

1. **端口冲突**: 确保要监听的端口没有被其他程序占用
2. **权限问题**: 在某些系统上，监听低端口号可能需要管理员权限
3. **防火墙**: 确保防火墙允许相应的端口通信
4. **资源清理**: 使用完毕后记得调用Stop()方法清理资源

## 扩展

如需添加新的NAT类型（如NAT4对称NAT），可以：

1. 在`types.NATType`中添加新的类型
2. 实现新的提供者结构体
3. 在工厂函数中添加对应的创建逻辑
4. 添加相应的测试用例 