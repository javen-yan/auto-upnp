package nat_traversal

import (
	"fmt"
	"sync"

	"auto-upnp/internal/util"
)

// PortAllocator 端口分配器
type PortAllocator struct {
	allocatedPorts map[int]bool
	mutex          sync.RWMutex
	startPort      int
	endPort        int
}

// NewPortAllocator 创建新的端口分配器
func NewPortAllocator(startPort, endPort int) *PortAllocator {
	return &PortAllocator{
		allocatedPorts: make(map[int]bool),
		startPort:      startPort,
		endPort:        endPort,
	}
}

// AllocatePort 分配一个可用端口
func (pa *PortAllocator) AllocatePort() (int, error) {
	pa.mutex.Lock()
	defer pa.mutex.Unlock()

	for port := pa.startPort; port <= pa.endPort; port++ {
		status := util.IsPortActive(port)
		if !status.Open {
			pa.allocatedPorts[port] = true
			return port, nil
		}
	}

	return 0, fmt.Errorf("没有可用的端口")
}

// ReleasePort 释放端口
func (pa *PortAllocator) ReleasePort(port int) {
	pa.mutex.Lock()
	defer pa.mutex.Unlock()
	delete(pa.allocatedPorts, port)
}
