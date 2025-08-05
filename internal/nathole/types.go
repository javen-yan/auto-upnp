package nathole

import (
	"auto-upnp/internal/types"
	"net"
	"time"
)

// NATHole NAT穿透信息
type NATHole struct {
	LocalPort    int                 `json:"local_port"`
	ExternalPort int                 `json:"external_port"`
	Protocol     string              `json:"protocol"`
	Description  string              `json:"description"`
	Type         types.NATType       `json:"type"`
	Status       types.MappingStatus `json:"status"`
	CreatedAt    time.Time           `json:"created_at"`
	LastActivity time.Time           `json:"last_activity"`
	ExternalAddr net.Addr            `json:"external_addr,omitempty"`
	Error        string              `json:"error,omitempty"`
}
