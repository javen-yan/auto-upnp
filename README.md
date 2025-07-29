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

### 方法1：使用安装脚本（推荐）

```bash
# 下载并运行安装脚本
curl -fsSL https://raw.githubusercontent.com/your-username/auto-upnp/main/install.sh | sudo bash

# 或者先下载再运行
wget https://raw.githubusercontent.com/your-username/auto-upnp/main/install.sh
chmod +x install.sh
sudo ./install.sh
```

安装脚本会自动：
- 从GitHub下载最新的release版本
- 生成默认配置文件到 `/etc/auto-upnp/config.yaml`
- 创建systemd服务文件
- 设置日志目录

### 方法2：手动安装

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

### 方法1：静态编译（推荐，解决GLIBC版本问题）
```bash
# 使用构建脚本
./build-static.sh

# 或手动构建
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o auto-upnp-static cmd/main.go
```

### 方法2：使用Makefile
```bash
# 静态构建
make build-static

# 兼容版本构建
make build-compatible

# 构建所有平台
make build-all
```

### 方法3：普通构建
```bash
go build -o auto-upnp cmd/main.go
```

**注意**：如果遇到GLIBC版本问题，请使用静态编译方法。

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

### 服务管理（使用安装脚本安装后）

```bash
# 启动服务
sudo systemctl start auto-upnp

# 停止服务
sudo systemctl stop auto-upnp

# 重启服务
sudo systemctl restart auto-upnp

# 查看服务状态
sudo systemctl status auto-upnp

# 查看实时日志
sudo journalctl -u auto-upnp -f

# 开机自启动
sudo systemctl enable auto-upnp

# 禁用开机自启动
sudo systemctl disable auto-upnp
```

### 手动运行

```bash
# 使用静态编译的版本（推荐）
./auto-upnp-static

# 使用默认配置文件启动
./auto-upnp-static -config config.yaml

# 指定配置文件
./auto-upnp-static -config /path/to/config.yaml

# 设置日志级别
./auto-upnp-static -log-level debug

# 显示帮助信息
./auto-upnp-static -help
```

### 使用Makefile运行
```bash
# 构建并运行静态版本
make run-static

# 构建并运行调试版本
make run-debug
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

## 卸载

### 使用安装脚本卸载

```bash
# 卸载服务
sudo ./install.sh --uninstall

# 或使用短选项
sudo ./install.sh -u
```

卸载脚本会：
- 停止并禁用systemd服务
- 删除二进制文件
- 删除systemd服务文件
- 可选择删除配置文件目录

### 手动卸载

```bash
# 停止服务
sudo systemctl stop auto-upnp
sudo systemctl disable auto-upnp

# 删除文件
sudo rm -f /usr/local/bin/auto-upnp
sudo rm -f /etc/systemd/system/auto-upnp.service
sudo rm -rf /etc/auto-upnp

# 重新加载systemd
sudo systemctl daemon-reload
```

## 故障排除

### 常见问题

1. **GLIBC版本问题**
   ```bash
   # 错误信息：./auto-upnp: /lib/x86_64-linux-gnu/libc.so.6: version `GLIBC_2.34' not found
   
   # 解决方案：使用静态编译
   CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o auto-upnp-static cmd/main.go
   
   # 或使用构建脚本
   ./build-static.sh
   ```

2. **无法发现UPnP设备**
   - 确保路由器支持UPnP功能
   - 检查防火墙设置
   - 确认网络连接正常

3. **端口映射失败**
   - 检查路由器UPnP设置
   - 确认端口未被其他服务占用
   - 查看日志获取详细错误信息

4. **服务无法启动**
   - 检查配置文件格式
   - 确认端口范围设置正确
   - 查看系统权限

5. **构建失败**
   - 确保Go版本 >= 1.21
   - 检查网络连接（下载依赖）
   - 尝试清理并重新构建：`make clean && make build-static`

6. **UPnP设备发现失败**
   - 确保路由器支持UPnP功能
   - 检查路由器UPnP设置是否启用
   - 确保防火墙允许UPnP通信

### 调试模式

使用debug日志级别获取更详细的信息：

```bash
./auto-upnp-static -log-level debug
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