package admin

// AddMappingRequest 添加映射请求
type AddMappingRequest struct {
	InternalPort int    `json:"internal_port"`
	ExternalPort int    `json:"external_port"`
	Protocol     string `json:"protocol"`
	Description  string `json:"description"`
}

// RemoveMappingRequest 删除映射请求
type RemoveMappingRequest struct {
	InternalPort int    `json:"internal_port"`
	ExternalPort int    `json:"external_port"`
	Protocol     string `json:"protocol"`
}

// CreateNATHoleRequest 创建NAT穿透洞请求
type CreateNATHoleRequest struct {
	InternalPort int    `json:"internal_port"`
	Protocol     string `json:"protocol"`
	Description  string `json:"description"`
}

// CloseNATHoleRequest 关闭NAT穿透洞请求
type CloseNATHoleRequest struct {
	InternalPort int    `json:"internal_port"`
	Protocol     string `json:"protocol"`
}

// APIResponse API响应
type APIResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// StatusResponse 状态响应
type StatusResponse struct {
	ServiceStatus  map[string]interface{} `json:"service_status"`
	PortRange      map[string]interface{} `json:"port_range"`
	PortStatus     map[string]interface{} `json:"port_status"`
	UPnPMappings   map[string]interface{} `json:"upnp_mappings"`
	ManualMappings map[string]interface{} `json:"manual_mappings"`
	Config         map[string]interface{} `json:"config"`
}

// PortsResponse 端口状态响应
type PortsResponse struct {
	ActivePorts   []int `json:"active_ports"`
	InactivePorts []int `json:"inactive_ports"`
}
