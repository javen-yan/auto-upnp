package portmapping

import (
	"net"
	"time"
)

// MappingType 映射类型
type MappingType string

const (
	MappingTypeUPnP MappingType = "upnp"
	MappingTypeTURN MappingType = "turn"
	MappingTypeNAT  MappingType = "nat"
)

// MappingStatus 映射状态
type MappingStatus string

const (
	MappingStatusActive   MappingStatus = "active"
	MappingStatusInactive MappingStatus = "inactive"
	MappingStatusFailed   MappingStatus = "failed"
)

// MappingAddType 映射添加类型
type MappingAddType string

const (
	MappingAddTypeAuto   MappingAddType = "auto"
	MappingAddTypeManual MappingAddType = "manual"
)

// PortMapping 端口映射信息
type PortMapping struct {
	InternalPort int            `json:"internal_port"`
	ExternalPort int            `json:"external_port"`
	Protocol     string         `json:"protocol"`
	Description  string         `json:"description"`
	AddType      MappingAddType `json:"add_type"`
	Type         MappingType    `json:"type"`
	Status       MappingStatus  `json:"status"`
	CreatedAt    time.Time      `json:"created_at"`
	LastActivity time.Time      `json:"last_activity"`
	ExternalAddr net.Addr       `json:"external_addr,omitempty"`
	Error        string         `json:"error,omitempty"`
}
