#!/bin/bash

# 简单的静态构建脚本，解决GLIBC版本问题

echo "开始静态构建..."

# 设置环境变量
export CGO_ENABLED=0
export GOOS=linux
export GOARCH=amd64

# 静态构建
go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o auto-upnp-static cmd/main.go

echo "静态构建完成: auto-upnp-static"

# 检查文件
if [ -f "auto-upnp-static" ]; then
    echo "文件大小: $(ls -lh auto-upnp-static | awk '{print $5}')"
    echo "检查依赖:"
    ldd auto-upnp-static 2>/dev/null || echo "静态链接，无外部依赖"
    echo "构建成功！"
else
    echo "构建失败！"
    exit 1
fi 