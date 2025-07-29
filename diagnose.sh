#!/bin/bash

# UPnP环境诊断脚本

echo "=== UPnP环境诊断 ==="
echo ""

# 检查网络接口
echo "1. 网络接口信息："
ip addr show | grep -E "inet.*global" | head -5
echo ""

# 检查默认网关
echo "2. 默认网关："
ip route show default
echo ""

# 检查UPnP端口是否开放
echo "3. 检查UPnP相关端口："
echo "   SSDP (1900):"
netstat -tuln | grep :1900 || echo "   端口1900未开放"
echo "   HTTP (80):"
netstat -tuln | grep :80 || echo "   端口80未开放"
echo ""

# 检查防火墙状态
echo "4. 防火墙状态："
if command -v ufw &> /dev/null; then
    ufw status
elif command -v firewall-cmd &> /dev/null; then
    firewall-cmd --state
else
    echo "未检测到常见防火墙"
fi
echo ""

# 检查是否有其他UPnP工具
echo "5. 检查UPnP工具："
if command -v upnpc &> /dev/null; then
    echo "发现upnpc工具"
    echo "尝试发现UPnP设备："
    timeout 10 upnpc -l 2>/dev/null || echo "upnpc未发现设备"
else
    echo "未安装upnpc工具"
    echo "建议安装：sudo apt-get install miniupnpc"
fi
echo ""

# 检查网络连接
echo "6. 网络连接测试："
echo "   测试到8.8.8.8的连接："
ping -c 3 8.8.8.8 >/dev/null && echo "   网络连接正常" || echo "   网络连接异常"
echo ""

# 检查本地端口监听
echo "7. 本地端口监听状态："
netstat -tuln | grep LISTEN | head -10
echo ""

echo "=== 诊断完成 ==="
echo ""
echo "建议："
echo "1. 确保路由器支持UPnP功能"
echo "2. 检查路由器UPnP设置是否启用"
echo "3. 确保防火墙允许UPnP通信"
echo "4. 尝试在路由器管理界面手动添加端口映射" 