package util

import (
	"fmt"
	"net"
)

// ProtocolType represents the network protocol
type ProtocolType string

const (
	TCP ProtocolType = "tcp"
	UDP ProtocolType = "udp"
)

// PortStatus represents the result of port detection
type PortStatus struct {
	Open     bool
	Protocol ProtocolType
}

func IsPortActive(port int) PortStatus {
	// Check TCP first
	if IsTCPPortActive(port) {
		return PortStatus{
			Open:     true,
			Protocol: TCP,
		}
	} else if IsUDPPortActive(port) {
		return PortStatus{
			Open:     true,
			Protocol: UDP,
		}
	}

	return PortStatus{
		Open:     false,
		Protocol: TCP,
	}
}

func IsTCPPortActive(port int) bool {
	address := fmt.Sprintf(":%d", port)

	// Try to listen on TCP port
	listener, err := net.Listen("tcp", address)
	if err != nil {
		// Port is in use (likely by a TCP service)
		return true
	}
	listener.Close()
	return false
}

func IsUDPPortActive(port int) bool {
	address := fmt.Sprintf(":%d", port)

	// Try to listen on UDP port
	conn, err := net.ListenPacket("udp", address)
	if err != nil {
		// Port is in use (likely by a UDP service)
		return true
	}
	conn.Close()
	return false
}
