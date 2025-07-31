package service

import (
	"auto-upnp/internal/types"
	"auto-upnp/internal/util"
)

type SystemService struct {
	NatInfo *types.NATInfo `json:"nat_info"`
}

var SystemServiceInstance *SystemService

func NewSystemService() error {
	sniffer := util.NewNATSniffer()
	defer sniffer.Close()
	natInfo, err := sniffer.DetectNATType()
	if err != nil {
		return err
	}
	SystemServiceInstance = &SystemService{
		NatInfo: natInfo,
	}
	return nil
}
