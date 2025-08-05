package portmapping

import (
	"auto-upnp/internal/types"
	"net"
	"time"
)

// PortMapping 端口映射信息
type PortMapping struct {
	InternalPort int                  `json:"internal_port"`
	ExternalPort int                  `json:"external_port"`
	Protocol     string               `json:"protocol"`
	Description  string               `json:"description"`
	AddType      types.MappingAddType `json:"add_type"`
	Type         types.MappingType    `json:"type"`
	Status       types.MappingStatus  `json:"status"`
	CreatedAt    time.Time            `json:"created_at"`
	LastActivity time.Time            `json:"last_activity"`
	ExternalAddr net.Addr             `json:"external_addr,omitempty"`
	Error        string               `json:"error,omitempty"`
}
