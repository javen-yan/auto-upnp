package portmapping

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
)

type NATProvider struct {
	ctx    context.Context
	cancel context.CancelFunc
	logger *logrus.Logger
}

func NewNATProvider(logger *logrus.Logger, config map[string]interface{}) *NATProvider {
	ctx, cancel := context.WithCancel(context.Background())
	return &NATProvider{
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (p *NATProvider) Type() MappingType {
	return MappingTypeNAT
}

func (p *NATProvider) Name() string {
	return "NAT穿透"
}

func (p *NATProvider) IsAvailable() bool {
	return true
}

func (p *NATProvider) CreateMapping(port int, externalPort int, protocol, description string, addType MappingAddType) (*PortMapping, error) {
	return nil, nil
}

func (p *NATProvider) RemoveMapping(port int, externalPort int, protocol string, addType MappingAddType) error {
	return nil
}

func (p *NATProvider) GetMappings() map[string]*PortMapping {
	return nil
}

func (p *NATProvider) GetStatus() map[string]interface{} {
	return nil
}

func (p *NATProvider) Start(checkStatusTaskTime time.Duration) error {
	return nil
}

func (p *NATProvider) Stop() error {
	return nil
}
