package types

import (
	"encoding/json"
	"net"
)

type NATType int

const (
	NATTypeUnknown NATType = iota
	NATType1               // 完全锥形NAT (Full Cone NAT)
	NATType2               // 受限锥形NAT (Restricted Cone NAT)
	NATType3               // 端口受限锥形NAT (Port Restricted Cone NAT)
	NATType4               // 对称NAT (Symmetric NAT)
)

func (n NATType) String() string {
	switch n {
	case NATType1:
		return "NAT1 (完全锥形NAT)"
	case NATType2:
		return "NAT2 (受限锥形NAT)"
	case NATType3:
		return "NAT3 (端口受限锥形NAT)"
	case NATType4:
		return "NAT4 (对称NAT)"
	default:
		return "未知NAT类型"
	}
}

// NATInfo NAT信息
type NATInfo struct {
	Type        NATType `json:"type"`
	PublicIP    net.IP  `json:"public_ip"`
	PublicPort  int     `json:"public_port"`
	LocalIP     net.IP  `json:"local_ip"`
	LocalPort   int     `json:"local_port"`
	Description string  `json:"description"`
}

func (n *NATInfo) String() string {
	temp := struct {
		Type        string `json:"type"`
		PublicIP    net.IP `json:"public_ip"`
		PublicPort  int    `json:"public_port"`
		LocalIP     net.IP `json:"local_ip"`
		LocalPort   int    `json:"local_port"`
		Description string `json:"description"`
	}{
		Type:        n.Type.String(),
		PublicIP:    n.PublicIP,
		PublicPort:  n.PublicPort,
		LocalIP:     n.LocalIP,
		LocalPort:   n.LocalPort,
		Description: n.Description,
	}

	json, err := json.Marshal(temp)
	if err != nil {
		return ""
	}
	return string(json)
}
