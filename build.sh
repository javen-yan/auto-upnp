#!/bin/bash

# 自动UPnP服务构建脚本
# 解决GLIBC版本兼容性问题

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 变量定义
BINARY_NAME="auto-upnp"
BUILD_DIR="build"
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')

# 显示帮助信息
show_help() {
    echo -e "${BLUE}自动UPnP服务构建脚本${NC}"
    echo ""
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  -h, --help              显示此帮助信息"
    echo "  -s, --static            静态构建（推荐，解决GLIBC版本问题）"
    echo "  -c, --compatible        构建兼容旧版本GLIBC的版本"
    echo "  -a, --all               构建所有平台版本"
    echo "  -d, --debug             构建调试版本"
    echo "  -r, --release           构建发布版本"
    echo "  -i, --install           构建并安装到系统"
    echo ""
    echo "示例:"
    echo "  $0 -s                    # 静态构建"
    echo "  $0 -c                    # 兼容版本构建"
    echo "  $0 -a                    # 构建所有平台"
    echo "  $0 -i                    # 构建并安装"
}

# 检查依赖
check_dependencies() {
    echo -e "${BLUE}检查依赖...${NC}"
    
    if ! command -v go &> /dev/null; then
        echo -e "${RED}错误: 未找到Go编译器${NC}"
        exit 1
    fi
    
    echo -e "${GREEN}Go版本: $(go version)${NC}"
}

# 清理构建目录
clean_build() {
    echo -e "${BLUE}清理构建目录...${NC}"
    rm -rf ${BUILD_DIR}
    mkdir -p ${BUILD_DIR}
}

# 普通构建
build_normal() {
    echo -e "${BLUE}普通构建...${NC}"
    go build -ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}" \
        -o ${BUILD_DIR}/${BINARY_NAME} cmd/main.go
    echo -e "${GREEN}普通构建完成: ${BUILD_DIR}/${BINARY_NAME}${NC}"
}

# 静态构建（解决GLIBC版本问题）
build_static() {
    echo -e "${BLUE}静态构建（解决GLIBC版本问题）...${NC}"
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
        -ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -extldflags '-static'" \
        -a -installsuffix cgo \
        -o ${BUILD_DIR}/${BINARY_NAME}-static cmd/main.go
    echo -e "${GREEN}静态构建完成: ${BUILD_DIR}/${BINARY_NAME}-static${NC}"
}

# 兼容版本构建
build_compatible() {
    echo -e "${BLUE}构建兼容旧版本GLIBC的版本...${NC}"
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
        -ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -extldflags '-static'" \
        -a -installsuffix cgo \
        -o ${BUILD_DIR}/${BINARY_NAME}-compatible cmd/main.go
    echo -e "${GREEN}兼容版本构建完成: ${BUILD_DIR}/${BINARY_NAME}-compatible${NC}"
}

# 构建所有平台
build_all() {
    echo -e "${BLUE}构建所有平台版本...${NC}"
    
    # Linux AMD64 (静态)
    echo "构建 Linux AMD64..."
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
        -ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -extldflags '-static'" \
        -a -installsuffix cgo \
        -o ${BUILD_DIR}/${BINARY_NAME}-linux-amd64 cmd/main.go
    
    # Linux ARM64 (静态)
    echo "构建 Linux ARM64..."
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
        -ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -extldflags '-static'" \
        -a -installsuffix cgo \
        -o ${BUILD_DIR}/${BINARY_NAME}-linux-arm64 cmd/main.go
    
    # macOS AMD64
    echo "构建 macOS AMD64..."
    GOOS=darwin GOARCH=amd64 go build \
        -ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}" \
        -o ${BUILD_DIR}/${BINARY_NAME}-darwin-amd64 cmd/main.go
    
    # macOS ARM64
    echo "构建 macOS ARM64..."
    GOOS=darwin GOARCH=arm64 go build \
        -ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}" \
        -o ${BUILD_DIR}/${BINARY_NAME}-darwin-arm64 cmd/main.go
    
    # Windows AMD64
    echo "构建 Windows AMD64..."
    GOOS=windows GOARCH=amd64 go build \
        -ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}" \
        -o ${BUILD_DIR}/${BINARY_NAME}-windows-amd64.exe cmd/main.go
    
    echo -e "${GREEN}所有平台构建完成${NC}"
}

# 安装到系统
install_binary() {
    echo -e "${BLUE}安装到系统...${NC}"
    
    # 选择要安装的二进制文件
    if [ -f "${BUILD_DIR}/${BINARY_NAME}-static" ]; then
        BINARY_FILE="${BUILD_DIR}/${BINARY_NAME}-static"
    elif [ -f "${BUILD_DIR}/${BINARY_NAME}-compatible" ]; then
        BINARY_FILE="${BUILD_DIR}/${BINARY_NAME}-compatible"
    elif [ -f "${BUILD_DIR}/${BINARY_NAME}" ]; then
        BINARY_FILE="${BUILD_DIR}/${BINARY_NAME}"
    else
        echo -e "${RED}错误: 未找到可安装的二进制文件${NC}"
        exit 1
    fi
    
    sudo cp "${BINARY_FILE}" /usr/local/bin/${BINARY_NAME}
    sudo chmod +x /usr/local/bin/${BINARY_NAME}
    echo -e "${GREEN}安装完成: /usr/local/bin/${BINARY_NAME}${NC}"
}

# 显示构建信息
show_build_info() {
    echo -e "${BLUE}构建信息:${NC}"
    echo "  版本: ${VERSION}"
    echo "  构建时间: ${BUILD_TIME}"
    echo "  构建目录: ${BUILD_DIR}"
    
    if [ -d "${BUILD_DIR}" ]; then
        echo -e "${BLUE}构建的文件:${NC}"
        ls -la ${BUILD_DIR}/
    fi
}

# 检查GLIBC版本
check_glibc() {
    if [ -f "${BUILD_DIR}/${BINARY_NAME}" ]; then
        echo -e "${BLUE}检查GLIBC版本要求:${NC}"
        ldd "${BUILD_DIR}/${BINARY_NAME}" 2>/dev/null | grep libc || echo "静态链接，无GLIBC依赖"
    fi
}

# 主函数
main() {
    local build_type="normal"
    local install_flag=false
    
    # 解析命令行参数
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_help
                exit 0
                ;;
            -s|--static)
                build_type="static"
                shift
                ;;
            -c|--compatible)
                build_type="compatible"
                shift
                ;;
            -a|--all)
                build_type="all"
                shift
                ;;
            -d|--debug)
                build_type="debug"
                shift
                ;;
            -r|--release)
                build_type="release"
                shift
                ;;
            -i|--install)
                install_flag=true
                shift
                ;;
            *)
                echo -e "${RED}未知选项: $1${NC}"
                show_help
                exit 1
                ;;
        esac
    done
    
    # 检查依赖
    check_dependencies
    
    # 清理构建目录
    clean_build
    
    # 根据构建类型执行构建
    case $build_type in
        "normal")
            build_normal
            ;;
        "static")
            build_static
            ;;
        "compatible")
            build_compatible
            ;;
        "all")
            build_all
            ;;
        "debug")
            build_static
            ;;
        "release")
            build_compatible
            ;;
    esac
    
    # 检查GLIBC版本
    check_glibc
    
    # 显示构建信息
    show_build_info
    
    # 安装（如果需要）
    if [ "$install_flag" = true ]; then
        install_binary
    fi
    
    echo -e "${GREEN}构建完成！${NC}"
}

# 执行主函数
main "$@" 