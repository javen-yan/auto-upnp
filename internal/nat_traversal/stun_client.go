package nat_traversal

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/sirupsen/logrus"
)

// STUNClient STUN客户端
type STUNClient struct {
	logger      *logrus.Logger
	ctx         context.Context
	cancel      context.CancelFunc
	stunServers []STUNServer
}

// STUNServer STUN服务器信息
type STUNServer struct {
	Host string
	Port int
}

// STUNResponse STUN响应信息
type STUNResponse struct {
	ExternalIP    net.IP
	ExternalPort  int
	MappedAddr    *net.UDPAddr
	ReflexiveAddr *net.UDPAddr
}

// 公共STUN服务器列表
var PublicSTUNServers = []STUNServer{
	{"stun.miwifi.com", 3478},
	{"stun.chat.bilibili.com", 3478},
	{"stun.hitv.com", 3478},
	{"stun.cdnbye.com", 3478},
}

// parseSTUNServers 解析STUN服务器字符串列表
func parseSTUNServers(serverStrings []string) []STUNServer {
	var servers []STUNServer

	for _, serverStr := range serverStrings {
		host, port := parseServerAddress(serverStr)
		if host != "" {
			servers = append(servers, STUNServer{
				Host: host,
				Port: port,
			})
		}
	}

	return servers
}

// parseServerAddress 解析服务器地址字符串
func parseServerAddress(serverStr string) (string, int) {
	// 默认端口
	defaultPort := 3478

	// 检查是否包含端口
	if host, port, err := net.SplitHostPort(serverStr); err == nil {
		// 有端口号
		if portNum, err := net.LookupPort("udp", port); err == nil {
			return host, portNum
		}
		return host, defaultPort
	}

	// 没有端口号，使用默认端口
	return serverStr, defaultPort
}

// NewSTUNClient 创建新的STUN客户端
func NewSTUNClient(logger *logrus.Logger, customServers []string) *STUNClient {
	ctx, cancel := context.WithCancel(context.Background())

	client := &STUNClient{
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}

	// 如果提供了自定义服务器列表，使用自定义列表
	if len(customServers) > 0 {
		client.stunServers = parseSTUNServers(customServers)
		logger.WithField("servers", customServers).Info("使用自定义STUN服务器列表")
	} else {
		client.stunServers = PublicSTUNServers
	}

	return client
}

// DiscoverExternalAddress 发现外部地址
func (sc *STUNClient) DiscoverExternalAddress() (*STUNResponse, error) {
	// 尝试多个STUN服务器
	for _, server := range sc.stunServers {
		if response, err := sc.querySTUNServer(server); err == nil {
			sc.logger.WithFields(logrus.Fields{
				"server":        fmt.Sprintf("%s:%d", server.Host, server.Port),
				"external_ip":   response.ExternalIP.String(),
				"external_port": response.ExternalPort,
			}).Info("STUN服务器响应成功")
			return response, nil
		} else {
			sc.logger.WithFields(logrus.Fields{
				"server": fmt.Sprintf("%s:%d", server.Host, server.Port),
				"error":  err,
			}).Warn("STUN服务器查询失败")
		}
	}

	return nil, fmt.Errorf("所有STUN服务器查询失败")
}

// querySTUNServer 查询单个STUN服务器
func (sc *STUNClient) querySTUNServer(server STUNServer) (*STUNResponse, error) {
	// 创建UDP连接
	conn, err := net.DialTimeout("udp", fmt.Sprintf("%s:%d", server.Host, server.Port), 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("连接STUN服务器失败: %w", err)
	}
	defer conn.Close()

	// 发送STUN绑定请求
	request := sc.createSTUNBindingRequest()
	_, err = conn.Write(request)
	if err != nil {
		return nil, fmt.Errorf("发送STUN请求失败: %w", err)
	}

	// 设置读取超时
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	// 读取响应
	response := make([]byte, 1024)
	n, err := conn.Read(response)
	if err != nil {
		return nil, fmt.Errorf("读取STUN响应失败: %w", err)
	}

	// 解析STUN响应
	return sc.parseSTUNResponse(response[:n])
}

// createSTUNBindingRequest 创建STUN绑定请求
func (sc *STUNClient) createSTUNBindingRequest() []byte {
	// STUN消息头 (20字节)
	// Message Type: Binding Request (0x0001)
	// Message Length: 0
	// Magic Cookie: 0x2112A442
	// Transaction ID: 随机生成

	request := make([]byte, 20)

	// Message Type: Binding Request
	request[0] = 0x00
	request[1] = 0x01

	// Message Length: 0
	request[2] = 0x00
	request[3] = 0x00

	// Magic Cookie: 0x2112A442
	request[4] = 0x21
	request[5] = 0x12
	request[6] = 0xA4
	request[7] = 0x42

	// Transaction ID: 随机生成 (12字节)
	for i := 8; i < 20; i++ {
		request[i] = byte(time.Now().UnixNano() % 256)
	}

	return request
}

// parseSTUNResponse 解析STUN响应
func (sc *STUNClient) parseSTUNResponse(data []byte) (*STUNResponse, error) {
	if len(data) < 20 {
		return nil, fmt.Errorf("STUN响应数据太短")
	}

	// 检查Magic Cookie
	if data[4] != 0x21 || data[5] != 0x12 || data[6] != 0xA4 || data[7] != 0x42 {
		return nil, fmt.Errorf("无效的STUN响应")
	}

	// 检查消息类型 (应该是Binding Success Response: 0x0101)
	messageType := uint16(data[0])<<8 | uint16(data[1])
	if messageType != 0x0101 {
		return nil, fmt.Errorf("非绑定成功响应: %04x", messageType)
	}

	response := &STUNResponse{}

	// 解析属性
	offset := 20
	for offset < len(data) {
		if offset+4 > len(data) {
			break
		}

		attrType := uint16(data[offset])<<8 | uint16(data[offset+1])
		attrLength := uint16(data[offset+2])<<8 | uint16(data[offset+3])

		if offset+4+int(attrLength) > len(data) {
			break
		}

		attrData := data[offset+4 : offset+4+int(attrLength)]

		switch attrType {
		case 0x0001: // MAPPED-ADDRESS
			if len(attrData) >= 8 {
				response.ExternalIP = net.IP(attrData[4:8])
				response.ExternalPort = int(attrData[2])<<8 | int(attrData[3])
			}
		case 0x0020: // XOR-MAPPED-ADDRESS
			if len(attrData) >= 8 {
				// XOR with Magic Cookie
				xorIP := make([]byte, 4)
				for i := 0; i < 4; i++ {
					xorIP[i] = attrData[4+i] ^ data[4+i]
				}
				response.ExternalIP = net.IP(xorIP)
				response.ExternalPort = int(attrData[2])<<8 | int(attrData[3]) ^ int(data[4])<<8 | int(data[5])
			}
		}

		offset += 4 + int(attrLength)
		// 4字节对齐
		if attrLength%4 != 0 {
			offset += 4 - int(attrLength%4)
		}
	}

	if response.ExternalIP == nil {
		return nil, fmt.Errorf("未找到外部地址信息")
	}

	return response, nil
}

// GetLocalAddress 获取本地地址
func (sc *STUNClient) GetLocalAddress() (*net.UDPAddr, error) {
	// 创建临时UDP连接来获取本地地址
	conn, err := net.Dial("udp", "8.8.8.8:53")
	if err != nil {
		return nil, fmt.Errorf("获取本地地址失败: %w", err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr, nil
}

// Close 关闭STUN客户端
func (sc *STUNClient) Close() {
	sc.cancel()
}
