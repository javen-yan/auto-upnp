package nathole

import (
	"testing"

	"auto-upnp/internal/types"

	"github.com/sirupsen/logrus"
)

func TestNAT1Provider(t *testing.T) {
	logger := logrus.New()
	config := map[string]interface{}{}

	provider := NewNAT1Provider(logger, config)

	// 测试基本信息
	if provider.Type() != types.NATType1 {
		t.Errorf("期望NAT类型为NATType1，实际为%v", provider.Type())
	}

	if provider.Name() != "NAT1提供者（完全锥形NAT）" {
		t.Errorf("期望名称为'NAT1提供者（完全锥形NAT）'，实际为%s", provider.Name())
	}

	// 测试启动
	if err := provider.Start(); err != nil {
		t.Errorf("启动NAT1提供者失败: %v", err)
	}

	if !provider.IsAvailable() {
		t.Error("NAT1提供者应该可用")
	}

	// 测试创建穿透
	hole, err := provider.CreateHole(8080, 8080, "tcp", "测试穿透")
	if err != nil {
		t.Errorf("创建NAT1穿透失败: %v", err)
	}

	if hole == nil {
		t.Error("创建的穿透不应该为nil")
	}

	if hole.LocalPort != 8080 {
		t.Errorf("期望本地端口为8080，实际为%d", hole.LocalPort)
	}

	if hole.Type != types.NATType1 {
		t.Errorf("期望NAT类型为NATType1，实际为%v", hole.Type)
	}

	// 测试获取状态
	status := provider.GetStatus()
	if status == nil {
		t.Error("状态不应该为nil")
	}

	// 测试移除穿透
	if err := provider.RemoveHole(8080, 8080, "tcp"); err != nil {
		t.Errorf("移除NAT1穿透失败: %v", err)
	}

	// 测试停止
	if err := provider.Stop(); err != nil {
		t.Errorf("停止NAT1提供者失败: %v", err)
	}
}

func TestNAT2Provider(t *testing.T) {
	logger := logrus.New()
	config := map[string]interface{}{}

	provider := NewNAT2Provider(logger, config)

	// 测试基本信息
	if provider.Type() != types.NATType2 {
		t.Errorf("期望NAT类型为NATType2，实际为%v", provider.Type())
	}

	if provider.Name() != "NAT2提供者（受限锥形NAT）" {
		t.Errorf("期望名称为'NAT2提供者（受限锥形NAT）'，实际为%s", provider.Name())
	}

	// 测试启动
	if err := provider.Start(); err != nil {
		t.Errorf("启动NAT2提供者失败: %v", err)
	}

	if !provider.IsAvailable() {
		t.Error("NAT2提供者应该可用")
	}

	// 测试创建穿透
	hole, err := provider.CreateHole(8081, 8081, "tcp", "测试穿透")
	if err != nil {
		t.Errorf("创建NAT2穿透失败: %v", err)
	}

	if hole == nil {
		t.Error("创建的穿透不应该为nil")
	}

	if hole.LocalPort != 8081 {
		t.Errorf("期望本地端口为8081，实际为%d", hole.LocalPort)
	}

	if hole.Type != types.NATType2 {
		t.Errorf("期望NAT类型为NATType2，实际为%v", hole.Type)
	}

	// 测试获取状态
	status := provider.GetStatus()
	if status == nil {
		t.Error("状态不应该为nil")
	}

	// 测试移除穿透
	if err := provider.RemoveHole(8081, 8081, "tcp"); err != nil {
		t.Errorf("移除NAT2穿透失败: %v", err)
	}

	// 测试停止
	if err := provider.Stop(); err != nil {
		t.Errorf("停止NAT2提供者失败: %v", err)
	}
}

func TestNAT3Provider(t *testing.T) {
	logger := logrus.New()
	config := map[string]interface{}{}

	provider := NewNAT3Provider(logger, config)

	// 测试基本信息
	if provider.Type() != types.NATType3 {
		t.Errorf("期望NAT类型为NATType3，实际为%v", provider.Type())
	}

	if provider.Name() != "NAT3提供者（端口受限锥形NAT）" {
		t.Errorf("期望名称为'NAT3提供者（端口受限锥形NAT）'，实际为%s", provider.Name())
	}

	// 测试启动
	if err := provider.Start(); err != nil {
		t.Errorf("启动NAT3提供者失败: %v", err)
	}

	if !provider.IsAvailable() {
		t.Error("NAT3提供者应该可用")
	}

	// 测试创建穿透
	hole, err := provider.CreateHole(8082, 8082, "tcp", "测试穿透")
	if err != nil {
		t.Errorf("创建NAT3穿透失败: %v", err)
	}

	if hole == nil {
		t.Error("创建的穿透不应该为nil")
	}

	if hole.LocalPort != 8082 {
		t.Errorf("期望本地端口为8082，实际为%d", hole.LocalPort)
	}

	if hole.Type != types.NATType3 {
		t.Errorf("期望NAT类型为NATType3，实际为%v", hole.Type)
	}

	// 测试获取状态
	status := provider.GetStatus()
	if status == nil {
		t.Error("状态不应该为nil")
	}

	// 测试移除穿透
	if err := provider.RemoveHole(8082, 8082, "tcp"); err != nil {
		t.Errorf("移除NAT3穿透失败: %v", err)
	}

	// 测试停止
	if err := provider.Stop(); err != nil {
		t.Errorf("停止NAT3提供者失败: %v", err)
	}
}

func TestNATHolePunching(t *testing.T) {
	logger := logrus.New()
	natInfo := &types.NATInfo{
		Type:        types.NATType1,
		Description: "测试NAT信息",
	}

	punching := NewNATHolePunching(logger, natInfo)
	if punching == nil {
		t.Fatal("NAT穿透管理器不应该为nil")
	}

	// 测试启动
	if err := punching.Start(); err != nil {
		t.Errorf("启动NAT穿透管理器失败: %v", err)
	}

	// 测试创建穿透
	hole, err := punching.CreateHole(8083, 8083, "tcp", "测试穿透")
	if err != nil {
		t.Errorf("创建NAT穿透失败: %v", err)
	}

	if hole == nil {
		t.Error("创建的穿透不应该为nil")
	}

	// 测试获取状态
	status := punching.GetStatus()
	if status == nil {
		t.Error("状态不应该为nil")
	}

	// 测试移除穿透
	if err := punching.RemoveHole(8083, 8083, "tcp"); err != nil {
		t.Errorf("移除NAT穿透失败: %v", err)
	}

	// 测试停止
	if err := punching.Stop(); err != nil {
		t.Errorf("停止NAT穿透管理器失败: %v", err)
	}
}

func TestFactory(t *testing.T) {
	logger := logrus.New()
	config := map[string]interface{}{}

	// 测试创建NAT1提供者
	provider1, err := CreateNATHoleProvider(types.NATType1, logger, config)
	if err != nil {
		t.Errorf("创建NAT1提供者失败: %v", err)
	}
	if provider1.Type() != types.NATType1 {
		t.Errorf("期望NAT类型为NATType1，实际为%v", provider1.Type())
	}

	// 测试创建NAT2提供者
	provider2, err := CreateNATHoleProvider(types.NATType2, logger, config)
	if err != nil {
		t.Errorf("创建NAT2提供者失败: %v", err)
	}
	if provider2.Type() != types.NATType2 {
		t.Errorf("期望NAT类型为NATType2，实际为%v", provider2.Type())
	}

	// 测试创建NAT3提供者
	provider3, err := CreateNATHoleProvider(types.NATType3, logger, config)
	if err != nil {
		t.Errorf("创建NAT3提供者失败: %v", err)
	}
	if provider3.Type() != types.NATType3 {
		t.Errorf("期望NAT类型为NATType3，实际为%v", provider3.Type())
	}

	// 测试未知NAT类型
	_, err = CreateNATHoleProvider(types.NATTypeUnknown, logger, config)
	if err == nil {
		t.Error("应该返回未知NAT类型的错误")
	}
} 