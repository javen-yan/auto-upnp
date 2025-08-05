package service

import (
	"auto-upnp/config"
	"auto-upnp/internal/types"
	"auto-upnp/internal/util"
)

type SystemService struct {
	NatInfo *types.NATInfo `json:"nat_info"`
}

var SystemServiceInstance *SystemService

func NewSystemService(cfg *config.Config) error {
	sniffer := util.NewNATSniffer(cfg.NATTraversal.STUNServers)
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
