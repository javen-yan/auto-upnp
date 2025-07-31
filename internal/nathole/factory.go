package nathole

import (
	"auto-upnp/internal/types"
	"fmt"

	"github.com/sirupsen/logrus"
)

// CreateNATHoleProvider 根据NAT类型创建相应的NAT穿透提供者
func CreateNATHoleProvider(natType types.NATType, logger *logrus.Logger, config map[string]interface{}) (NATHoleProvider, error) {
	switch natType {
	case types.NATType1:
		return NewNAT1Provider(logger, config), nil
	case types.NATType2:
		return NewNAT2Provider(logger, config), nil
	case types.NATType3:
		return NewNAT3Provider(logger, config), nil
	case types.NATType4:
		return NewNAT4Provider(logger, config), nil
	default:
		return nil, fmt.Errorf("未知的NAT类型: %s", natType)
	}
}
