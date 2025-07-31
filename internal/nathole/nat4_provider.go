package nathole

import (
	"auto-upnp/internal/types"
	"context"

	"github.com/sirupsen/logrus"
)

// NAT4Provider NAT4提供者（对称NAT）
type NAT4Provider struct {
	logger *logrus.Logger
	ctx    context.Context
	cancel context.CancelFunc
	holes  map[string]*NATHole
	// mutex     sync.RWMutex
	available bool
	config    map[string]interface{}
}

func NewNAT4Provider(logger *logrus.Logger, config map[string]interface{}) *NAT4Provider {
	return &NAT4Provider{
		logger:    logger,
		ctx:       context.Background(),
		cancel:    func() {},
		holes:     make(map[string]*NATHole),
		available: false,
		config:    config,
	}
}

func (n *NAT4Provider) Type() types.NATType {
	return types.NATType4
}

func (n *NAT4Provider) Name() string {
	return "NAT4Provider"
}

func (n *NAT4Provider) IsAvailable() bool {
	return true
}

func (n *NAT4Provider) Start() error {
	return nil
}

func (n *NAT4Provider) Stop() error {
	return nil
}

func (n *NAT4Provider) CreateHole(localPort int, externalPort int, protocol string, description string) (*NATHole, error) {
	return nil, nil
}

func (n *NAT4Provider) RemoveHole(localPort int, externalPort int, protocol string) error {
	return nil
}

func (n *NAT4Provider) GetHoles() map[string]*NATHole {
	return nil
}

func (n *NAT4Provider) GetStatus() map[string]interface{} {
	return nil
}
