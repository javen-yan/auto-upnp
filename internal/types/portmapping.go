package types

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
