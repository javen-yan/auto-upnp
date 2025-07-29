# 自动UPnP服务

一个用Golang编写的自动UPnP端口映射服务，能够自动监控端口状态并管理UPnP端口映射。

## 功能特性

- 🔍 **自动端口监控**: 监控指定端口范围的上下线状态
- 🔄 **自动UPnP映射**: 根据端口状态自动添加/删除UPnP端口映射
- 🛠️ **手动映射管理**: 支持手动添加和删除端口映射
- 🧹 **自动清理**: 自动清理过期的端口映射
- 📊 **状态监控**: 实时监控服务状态和端口映射情况
- 📝 **详细日志**: 完整的日志记录和错误处理
- ⚙️ **灵活配置**: 支持YAML配置文件自定义各种参数

## 系统要求

- Go 1.21 或更高版本
- 支持UPnP的路由器
- Linux/macOS/Windows

## 安装

1. 克隆项目：
```bash
git clone <repository-url>
cd auto-upnp
```

2. 安装依赖：
```bash
go mod tidy
```

3. 编译项目：
```bash
go build -o auto-upnp cmd/main.go
```

## 配置

创建配置文件 `config.yaml`：

```yaml
# 端口监听范围配置
port_range:
  start: 8000      # 起始端口
  end: 9000        # 结束端口
  step: 1          # 端口间隔

# UPnP配置
upnp:
  discovery_timeout: 10s    # 设备发现超时时间
  mapping_duration: 1h      # 端口映射持续时间，0表示永久
  retry_attempts: 3         # 重试次数
  retry_delay: 5s           # 重试延迟

# 网络接口配置
network:
  preferred_interfaces: ["eth0", "wlan0"]  # 优先使用的网络接口
  exclude_interfaces: ["lo", "docker"]     # 排除的网络接口

# 日志配置
log:
  level: "info"
  format: "json"
  file: "auto_upnp.log"
  max_size: 10485760  # 10MB
  backup_count: 5

# 监控配置
monitor:
  check_interval: 30s       # 端口状态检查间隔
  cleanup_interval: 5m      # 清理无效映射间隔
  max_mappings: 100         # 最大端口映射数量
```

## 使用方法

### 基本使用

```bash
# 使用默认配置文件启动
./auto-upnp

# 指定配置文件
./auto-upnp -config /path/to/config.yaml

# 设置日志级别
./auto-upnp -log-level debug

# 显示帮助信息
./auto-upnp -help
```

### 命令行选项

- `-config`: 配置文件路径 (默认: config.yaml)
- `-log-level`: 日志级别 (debug, info, warn, error) (默认: info)
- `-help`: 显示帮助信息

## 工作原理

1. **设备发现**: 启动时自动发现网络中的UPnP设备
2. **端口监控**: 定期检查配置的端口范围，检测端口状态变化
3. **自动映射**: 
   - 当检测到端口上线时，自动添加UPnP端口映射
   - 当检测到端口下线时，自动删除UPnP端口映射
4. **状态管理**: 维护端口映射状态，避免重复操作
5. **定期清理**: 定期清理过期的端口映射

## 项目结构

```
auto-upnp/
├── cmd/
│   └── main.go              # 主程序入口
├── config/
│   └── config.go            # 配置管理
├── internal/
│   ├── portmonitor/
│   │   └── port_monitor.go  # 端口监控器
│   ├── service/
│   │   └── auto_upnp_service.go  # 自动UPnP服务
│   └── upnp/
│       └── upnp_manager.go  # UPnP管理器
├── config.yaml              # 配置文件
├── go.mod                   # Go模块文件
├── go.sum                   # 依赖校验文件
└── README.md               # 项目说明
```

## 日志说明

服务会输出详细的JSON格式日志，包括：

- 服务启动/停止信息
- UPnP设备发现过程
- 端口状态变化
- 端口映射添加/删除操作
- 错误和警告信息
- 定期状态报告

## 故障排除

### 常见问题

1. **无法发现UPnP设备**
   - 确保路由器支持UPnP功能
   - 检查防火墙设置
   - 确认网络连接正常

2. **端口映射失败**
   - 检查路由器UPnP设置
   - 确认端口未被其他服务占用
   - 查看日志获取详细错误信息

3. **服务无法启动**
   - 检查配置文件格式
   - 确认端口范围设置正确
   - 查看系统权限

### 调试模式

使用debug日志级别获取更详细的信息：

```bash
./auto-upnp -log-level debug
```

## 开发

### 添加新功能

1. 在相应的包中添加新功能
2. 更新配置文件结构（如需要）
3. 添加测试用例
4. 更新文档

### 运行测试

```bash
go test ./...
```

## 许可证

MIT License

## 贡献

欢迎提交Issue和Pull Request！

## 更新日志

### v1.0.0
- 初始版本发布
- 支持自动端口监控和UPnP映射
- 完整的配置管理和日志系统 