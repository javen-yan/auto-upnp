# Auto UPnP HTTP管理服务

## 功能概述

HTTP管理服务为Auto UPnP提供了Web界面，方便用户通过浏览器管理端口映射和服务状态。

## 主要功能

### 1. 服务状态监控
- 实时显示活跃端口数量
- 显示总映射数、自动映射数、手动映射数
- 监控服务运行状态

### 2. 端口映射管理
- 查看所有端口映射列表
- 显示映射的详细信息（内部端口、外部端口、协议、描述等）
- 支持删除现有映射

### 3. 手动端口映射
- 添加新的端口映射
- 支持TCP和UDP协议
- 可自定义映射描述

### 4. 端口状态监控
- 可视化显示所有监控端口的活跃状态
- 实时更新端口状态

## 配置说明

在`config.yaml`文件中添加以下配置：

```yaml
# 管理服务配置
admin:
  enabled: true             # 是否启用管理服务
  host: "0.0.0.0"          # 监听地址
  port: 8080               # 监听端口（如果被占用会自动选择下一个可用端口）
  username: "admin"         # 用户名
  password: "admin"      # 密码
```

## 使用方法

### 1. 启动服务
```bash
./auto-upnp -config config.yaml
```

### 2. 访问管理界面
在浏览器中访问：`http://localhost:8080`（或实际使用的端口）

### 3. 登录认证
使用配置文件中设置的用户名和密码登录：
- 用户名：admin
- 密码：admin

## API接口

### 1. 获取服务状态
```
GET /api/status
```

### 2. 获取端口映射列表
```
GET /api/mappings
```

### 3. 添加端口映射
```
POST /api/add-mapping
Content-Type: application/x-www-form-urlencoded

internal_port=8080&external_port=8080&protocol=TCP&description=Web服务
```

### 4. 删除端口映射
```
POST /api/remove-mapping
Content-Type: application/x-www-form-urlencoded

internal_port=8080&external_port=8080&protocol=TCP
```

### 5. 获取端口状态
```
GET /api/ports
```

## 安全特性

1. **基本认证**：所有API接口都需要用户名和密码认证
2. **HTTPS支持**：可以配置SSL证书以支持HTTPS访问
3. **访问控制**：可以限制管理界面的访问IP地址

## 界面特性

1. **响应式设计**：支持桌面和移动设备访问
2. **实时更新**：每5秒自动刷新数据
3. **现代化UI**：使用现代化的CSS样式和交互效果
4. **中文界面**：完全中文化的用户界面

## 故障排除

### 1. 端口被占用
如果配置的端口被占用，服务会自动选择下一个可用端口。查看日志获取实际使用的端口号。

### 2. 认证失败
确保使用正确的用户名和密码，检查配置文件中的设置。

### 3. 无法访问界面
- 检查防火墙设置
- 确认服务正在运行
- 验证访问地址和端口

## 开发说明

### 文件结构
```
internal/admin/
├── admin.go      # HTTP服务器主逻辑
└── templates.go  # HTML模板
```

### 扩展功能
可以通过修改`admin.go`文件添加新的API接口，或修改`templates.go`文件自定义界面样式。

## 注意事项

1. 管理服务默认监听所有网络接口（0.0.0.0），生产环境建议限制访问IP
2. 密码以明文形式存储在配置文件中，生产环境建议使用环境变量或加密存储
3. 管理界面会显示敏感信息，请确保网络安全
4. 建议在生产环境中启用HTTPS 