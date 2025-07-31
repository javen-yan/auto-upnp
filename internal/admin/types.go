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

// APIResponse API响应
type APIResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
