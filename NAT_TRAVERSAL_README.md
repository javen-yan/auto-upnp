# NAT穿透功能说明

## 概述

NAT穿透功能通过STUN协议实现端口映射，适用于没有公网IP或UPnP不可用的NAT网络环境。通过STUN服务器发现外部地址，实现简单的端口映射功能。

## 功能特性

### 🔄 自动故障转移
- 优先尝试UPnP端口映射
- UPnP失败时自动切换到STUN穿透
- 支持TCP和UDP协议

### 🌐 STUN穿透
- **STUN协议**: 使用标准STUN协议发现外部地址
- **多服务器**: 支持多个STUN服务器，提高可靠性
- **自动重试**: 自动尝试多个STUN服务器

### ⚙️ 灵活配置
- 可配置STUN服务器列表
- 支持启用/禁用STUN功能
- 简单易用的配置

## 配置说明

### 基本配置

```yaml
# NAT穿透配置
nat_traversal:
  enabled: false            # 启用NAT穿透功能
  use_stun: true            # 启用STUN服务器
  stun_servers:             # STUN服务器列表
    - "stun.l.google.com:19302"
    - "stun1.l.google.com:19302"
    - "stun.stunprotocol.org:3478"
```

### 参数说明

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `enabled` | bool | false | 是否启用NAT穿透 |
| `use_stun` | bool | true | 是否启用STUN功能 |
| `stun_servers` | []string | 见默认值 | STUN服务器列表 |

## 使用方法

### 1. 配置客户端

```yaml
# 修改配置文件
nat_traversal:
  enabled: true
  use_stun: true
  stun_servers:
    - "stun.l.google.com:19302"
    - "stun1.l.google.com:19302"
```

### 2. 启动自动UPnP服务

```bash
# 启动服务
./auto-upnp -config config.yaml
```

## API接口

### 获取NAT穿透状态

```http
GET /api/nat-status
```

响应:
```json
{
  "available": true,
  "status": "可用",
  "use_stun": true
}
```

### 获取NAT穿透洞列表

```http
GET /api/nat-holes
```

响应:
```json
{
  "holes": {
    "8080-TCP": {
      "local_port": 8080,
      "remote_port": 8080,
      "protocol": "TCP",
      "description": "Web服务",
      "created_at": "2024-01-01T12:00:00Z",
      "last_activity": "2024-01-01T12:05:00Z",
      "is_active": true
    }
  }
}
```

### 创建NAT穿透洞

```http
POST /api/create-nat-hole
Content-Type: application/json

{
  "internal_port": 8080,
  "external_port": 8080,
  "protocol": "TCP",
  "description": "Web服务"
}
```

### 关闭NAT穿透洞

```http
POST /api/close-nat-hole
Content-Type: application/json

{
  "internal_port": 8080,
  "external_port": 8080,
  "protocol": "TCP"
}
```

## 工作原理

### 1. STUN发现
1. 客户端连接到STUN服务器
2. STUN服务器返回客户端的外网地址
3. 客户端记录外部地址信息

### 2. 端口映射
1. 客户端创建本地端口监听
2. 使用STUN发现的外部地址进行映射
3. 记录映射信息供后续使用

### 3. 数据转发
1. 外部连接通过映射的地址访问
2. 数据转发到本地服务
3. 定期检查映射状态

## 网络类型支持

| NAT类型 | STUN支持 | 说明 |
|---------|----------|------|
| 完全锥形NAT | ✅ | 完全支持 |
| 受限锥形NAT | ✅ | 完全支持 |
| 端口受限锥形NAT | ✅ | 完全支持 |
| 对称型NAT | ⚠️ | 部分支持，需要特殊处理 |

## 注意事项

1. **STUN服务器可用性**: 确保配置的STUN服务器可用
2. **网络环境**: 某些企业网络可能阻止STUN流量
3. **防火墙**: 确保防火墙允许STUN协议流量
4. **端口范围**: 建议使用常用端口范围，避免被ISP封锁

## 故障排除

### STUN发现失败
- 检查网络连接
- 尝试不同的STUN服务器
- 检查防火墙设置

### 端口映射失败
- 确认端口未被占用
- 检查本地服务是否正常运行
- 验证STUN服务器响应

### 连接不稳定
- 增加STUN服务器数量
- 检查网络质量
- 考虑使用备用方案 