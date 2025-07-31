package service

import (
	"auto-upnp/internal/util"
)

type SystemService struct {
	NatDetail *util.NATDetail `json:"nat_detail"`
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
		NatDetail: natInfo.ToDetail(),
	}
	return nil
}
