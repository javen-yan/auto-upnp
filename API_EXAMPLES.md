# API 使用示例

## 认证

所有API都需要Basic认证，用户名和密码在配置文件中设置。

```bash
# 使用curl进行认证
curl -u admin:admin http://localhost:8080/api/status
```

## API 端点

### 1. 获取服务状态

```bash
GET /api/status
```

**响应示例：**
```json
{
  "service_status": "running",
  "port_range": {
    "start": 18000,
    "end": 19000,
    "step": 1
  },
  "port_status": {
    "total_ports": 1001,
    "active_ports": 5,
    "inactive_ports": 996,
    "active_ports_list": [8080, 9000, 9090, 9200, 9300],
    "inactive_ports_list": [18000, 18001, ...]
  },
  "upnp_mappings": {
    "total_mappings": 3,
    "active_mappings": [8080, 9000],
    "mappings": {
      "8080:8080:TCP": {
        "internal_port": 8080,
        "external_port": 8080,
        "protocol": "TCP",
        "description": "手动映射 8080->8080"
      }
    }
  },
  "manual_mappings": {
    "total_mappings": 2,
    "active_mappings": 1,
    "inactive_mappings": 1,
    "mappings": [
      {
        "internal_port": 8080,
        "external_port": 8080,
        "protocol": "TCP",
        "description": "手动映射 8080->8080",
        "created_at": "2024-01-15T10:30:00Z",
        "active": true
      },
      {
        "internal_port": 9090,
        "external_port": 9090,
        "protocol": "TCP",
        "description": "手动映射 9090->9090",
        "created_at": "2024-01-15T11:00:00Z",
        "active": false
      }
    ],
    "active_mappings_list": [
      {
        "internal_port": 8080,
        "external_port": 8080,
        "protocol": "TCP",
        "description": "手动映射 8080->8080",
        "created_at": "2024-01-15T10:30:00Z",
        "active": true
      }
    ],
    "inactive_mappings_list": [
      {
        "internal_port": 9090,
        "external_port": 9090,
        "protocol": "TCP",
        "description": "手动映射 9090->9090",
        "created_at": "2024-01-15T11:00:00Z",
        "active": false
      }
    ]
  },
  "config": {
    "check_interval": "30s",
    "cleanup_interval": "5m",
    "mapping_duration": "1h"
  },
  "admin_service": {
    "enabled": true,
    "host": "127.0.0.1",
    "port": 8080,
    "url": "http://127.0.0.1:8080"
  },
  "system_service": {
    "nat_detail": {
      "type": "Symmetric NAT",
      "public_ip": "203.0.113.1",
      "public_port": 54321,
      "local_ip": "192.168.1.100",
      "local_port": 0,
      "description": "对称型NAT，端口映射受限"
    }
  }
}
```

**新增字段说明：**
- `admin_service`: 管理服务信息
  - `enabled`: 管理服务是否启用
  - `host`: 管理服务主机地址
  - `port`: 管理服务端口
  - `url`: 管理服务完整URL
- `system_service`: 系统服务信息
  - `nat_detail`: NAT详细信息
    - `type`: NAT类型（如"Symmetric NAT"、"Full Cone NAT"等）
    - `public_ip`: 公网IP地址
    - `public_port`: 公网端口
    - `local_ip`: 本地IP地址
    - `local_port`: 本地端口
    - `description`: NAT类型描述

### 2. 获取端口映射列表

```bash
GET /api/mappings
```

**查询参数：**
- `addType` (可选): 过滤映射类型，支持 "auto" 或 "manual"

**响应示例：**
```json
{
  "8080:8080:TCP": {
    "internal_port": 8080,
    "external_port": 8080,
    "protocol": "TCP",
    "description": "手动映射 8080->8080",
    "active": true
  },
  "9000:9000:UDP": {
    "internal_port": 9000,
    "external_port": 9000,
    "protocol": "UDP",
    "description": "手动映射 9000->9000",
    "active": false
  }
}
```

### 3. 添加端口映射

```bash
POST /api/add-mapping
Content-Type: application/json
```

**请求体：**
```json
{
  "internal_port": 8080,
  "external_port": 8080,
  "protocol": "TCP",
  "description": "Web服务器端口"
}
```

**字段说明：**
- `internal_port` (必需): 内部端口号 (1-65535)
- `external_port` (必需): 外部端口号 (1-65535)
- `protocol` (可选): 协议类型，支持 "TCP" 或 "UDP"，默认为 "TCP"
- `description` (可选): 映射描述，如果不提供会自动生成

**响应示例：**
```json
{
  "status": "success",
  "message": "映射添加成功"
}
```

**错误响应示例：**
```json
{
  "status": "error",
  "message": "内部端口格式错误"
}
```

**可能的错误信息：**
- "内部端口格式错误": 端口号不在1-65535范围内
- "外部端口格式错误": 端口号不在1-65535范围内
- "内部端口在端口范围内,请勿重复添加": 端口在自动管理范围内
- "JSON格式错误": 请求体JSON格式不正确
- "读取请求体失败": 无法读取请求体

### 4. 删除端口映射

```bash
POST /api/remove-mapping
Content-Type: application/json
```

**请求体：**
```json
{
  "internal_port": 8080,
  "external_port": 8080,
  "protocol": "TCP"
}
```

**字段说明：**
- `internal_port` (必需): 内部端口号 (1-65535)
- `external_port` (必需): 外部端口号 (1-65535)
- `protocol` (可选): 协议类型，支持 "TCP" 或 "UDP"，默认为 "TCP"

**响应示例：**
```json
{
  "status": "success",
  "message": "映射删除成功"
}
```

**错误响应示例：**
```json
{
  "status": "error",
  "message": "删除映射失败: 映射不存在"
}
```

### 5. 获取端口状态

```bash
GET /api/ports
```

**响应示例：**
```json
{
  "active_ports": [8080, 9000, 9090]
}
```

**字段说明：**
- `active_ports`: 当前活跃的端口列表

## 使用curl示例

### 添加映射
```bash
curl -X POST 'http://localhost:8080/api/add-mapping' \
  -H 'Content-Type: application/json' \
  -u admin:admin \
  -d '{
    "internal_port": 8080,
    "external_port": 8080,
    "protocol": "TCP",
    "description": "Web服务器端口"
  }'
```

### 删除映射
```bash
curl -X POST 'http://localhost:8080/api/remove-mapping' \
  -H 'Content-Type: application/json' \
  -u admin:admin \
  -d '{
    "internal_port": 8080,
    "external_port": 8080,
    "protocol": "TCP"
  }'
```

### 获取状态
```bash
curl -u admin:admin 'http://localhost:8080/api/status'
```

### 获取端口映射列表
```bash
# 获取所有映射
curl -u admin:admin 'http://localhost:8080/api/mappings'

# 获取自动映射
curl -u admin:admin 'http://localhost:8080/api/mappings?addType=auto'

# 获取手动映射
curl -u admin:admin 'http://localhost:8080/api/mappings?addType=manual'
```

### 获取端口状态
```bash
curl -u admin:admin 'http://localhost:8080/api/ports'
```

## 错误码说明

- `200 OK`: 请求成功
- `400 Bad Request`: 请求参数错误
- `401 Unauthorized`: 认证失败
- `405 Method Not Allowed`: 请求方法不允许
- `500 Internal Server Error`: 服务器内部错误

## NAT类型说明

系统会自动检测NAT类型，常见的NAT类型包括：

- **Full Cone NAT**: 完全锥形NAT，最宽松的类型
- **Restricted Cone NAT**: 受限锥形NAT
- **Port Restricted Cone NAT**: 端口受限锥形NAT
- **Symmetric NAT**: 对称型NAT，最严格的类型

NAT类型会影响端口映射的成功率和稳定性。

## 注意事项

1. 所有POST请求必须使用JSON格式
2. 端口号必须在1-65535范围内
3. 协议支持TCP和UDP，默认为TCP
4. 描述字段为可选，如果不提供会自动生成
5. 手动映射会自动持久化保存到文件
6. 手动映射的`active`字段会根据端口状态自动更新：
   - `true`: 端口在线，UPnP映射已注册
   - `false`: 端口离线，UPnP映射已取消
7. 系统会自动监控手动映射端口的上下线状态
8. 端口恢复时会自动重新注册UPnP映射
9. 内部端口不能与自动管理的端口范围重叠
10. 系统会自动检测NAT类型并提供相关信息 