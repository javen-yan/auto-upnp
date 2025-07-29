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
    "mappings": [
      {
        "internal_port": 8080,
        "external_port": 8080,
        "protocol": "TCP",
        "description": "手动映射 8080->8080",
        "created_at": "2024-01-15T10:30:00Z"
      }
    ]
  },
  "config": {
    "check_interval": "30s",
    "cleanup_interval": "5m",
    "mapping_duration": "1h",
    "max_mappings": 100
  }
}
```

### 2. 获取端口映射列表

```bash
GET /api/mappings
```

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

**响应示例：**
```json
{
  "status": "success",
  "message": "映射删除成功"
}
```

### 5. 获取端口状态

```bash
GET /api/ports
```

**响应示例：**
```json
{
  "active_ports": [8080, 9000, 9090],
  "inactive_ports": [18000, 18001, 18002]
}
```

### 6. 获取UPnP状态

```bash
GET /api/upnp-status
```

**响应示例：**
```json
{
  "client_count": 2,
  "available": true,
  "status": "可用"
}
```

**响应字段说明：**
- `client_count`: UPnP客户端数量
- `available`: UPnP服务是否可用（client_count > 0）
- `status`: 状态描述（"可用" 或 "不可用"）

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

## 错误码说明

- `200 OK`: 请求成功
- `400 Bad Request`: 请求参数错误
- `401 Unauthorized`: 认证失败
- `405 Method Not Allowed`: 请求方法不允许
- `500 Internal Server Error`: 服务器内部错误

## 注意事项

1. 所有POST请求必须使用JSON格式
2. 端口号必须在1-65535范围内
3. 协议支持TCP和UDP，默认为TCP
4. 描述字段为可选，如果不提供会自动生成
5. 手动映射会自动持久化保存到文件 