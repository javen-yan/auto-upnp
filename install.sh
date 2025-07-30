#!/bin/bash

# 自动UPnP服务安装脚本
# 支持从GitHub下载release版本并配置systemd服务

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 配置变量
GITHUB_REPO="javen-yan/auto-upnp"  # 需要替换为实际的GitHub仓库
SERVICE_NAME="auto-upnp"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
CONFIG_DIR="/etc/auto-upnp"
CONFIG_FILE="${CONFIG_DIR}/config.yaml"
BIN_DIR="/usr/local/bin"
BINARY_NAME="auto-upnp"
BINARY_PATH="${BIN_DIR}/${BINARY_NAME}"
USE_PROXY=${USE_PROXY:-true}
PROXY=${PROXY:-"https://ghfast.top"}

# 日志函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

# 检查是否为root用户
check_root() {
    if [[ $EUID -ne 0 ]]; then
        log_error "此脚本需要root权限运行"
        log_info "请使用: sudo $0"
        exit 1
    fi
}

# 显示代理配置
show_proxy_config() {
    log_step "代理配置信息..."
    log_info "USE_PROXY: ${USE_PROXY}"
    if [ "$USE_PROXY" = true ]; then
        log_info "PROXY: ${PROXY}"
    fi
    echo
}

# 检测系统平台
detect_platform() {
    # 检测操作系统类型
    case "$(uname -s)" in
        Linux)
            OS="linux"
            ;;
        Darwin)
            OS="darwin"
            ;;
        *)
            log_error "不支持的操作系统: $(uname -s)"
            exit 1
            ;;
    esac
    
    # 检测系统架构
    case "$(uname -m)" in
        x86_64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        *)
            log_error "不支持的架构: $(uname -m)"
            log_error "支持的架构: amd64, arm64"
            exit 1
            ;;
    esac
    
    log_info "检测到平台: ${OS}/${ARCH}"
}

# 获取最新版本
get_latest_version() {
    log_step "获取最新版本信息..."
    
    # 使用GitHub API获取最新版本
    LATEST_VERSION=$(curl -s "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    
    if [[ -z "$LATEST_VERSION" ]]; then
        log_error "无法获取最新版本信息"
        log_info "请检查网络连接或手动指定版本"
        exit 1
    fi
    
    log_info "最新版本: ${LATEST_VERSION}"
}

# 下载二进制文件
download_binary() {
    log_step "下载二进制文件..."
    
    # 构建下载URL
    if [ "$USE_PROXY" = true ]; then
        DOWNLOAD_URL="${PROXY}/https://github.com/${GITHUB_REPO}/releases/download/${LATEST_VERSION}/auto-upnp-${OS}-${ARCH}"
        log_info "使用代理下载: ${DOWNLOAD_URL}"
    else
        DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/download/${LATEST_VERSION}/auto-upnp-${OS}-${ARCH}"
        log_info "直接下载: ${DOWNLOAD_URL}"
    fi
    
    # 创建临时目录
    TEMP_DIR=$(mktemp -d)
    cd "$TEMP_DIR"
    
    # 下载文件
    if ! curl -L -o "${BINARY_NAME}" "$DOWNLOAD_URL"; then
        log_error "下载失败"
        rm -rf "$TEMP_DIR"
        exit 1
    fi
    
    # 检查文件是否下载成功
    if [[ ! -f "${BINARY_NAME}" ]]; then
        log_error "下载的文件不存在"
        rm -rf "$TEMP_DIR"
        exit 1
    fi
    
    # 设置执行权限
    chmod +x "${BINARY_NAME}"
    
    log_info "下载完成"
}

# 安装二进制文件
install_binary() {
    local skip_backup=${1:-false}
    
    log_step "安装二进制文件..."
    
    # 创建目标目录
    mkdir -p "$BIN_DIR"
    
    # 备份现有文件（除非跳过备份）
    if [[ -f "$BINARY_PATH" ]] && [[ "$skip_backup" != "true" ]]; then
        log_warn "发现现有二进制文件，创建备份"
        mv "$BINARY_PATH" "${BINARY_PATH}.backup.$(date +%Y%m%d_%H%M%S)"
    fi
    
    # 移动文件
    mv "${TEMP_DIR}/${BINARY_NAME}" "$BINARY_PATH"
    
    # 清理临时目录
    rm -rf "$TEMP_DIR"
    
    log_info "二进制文件安装完成: ${BINARY_PATH}"
}

# 生成默认配置文件
generate_config() {
    log_step "生成默认配置文件..."
    
    # 创建配置目录
    mkdir -p "$CONFIG_DIR"
    
    # 备份现有配置文件
    if [[ -f "$CONFIG_FILE" ]]; then
        log_warn "发现现有配置文件，创建备份"
        mv "$CONFIG_FILE" "${CONFIG_FILE}.backup.$(date +%Y%m%d_%H%M%S)"
    fi
    
    # 生成默认配置文件
    cat > "$CONFIG_FILE" << 'EOF'
# 自动UPnP服务配置文件

# 端口监听范围配置
port_range:
  start: 18000      # 起始端口
  end: 19000        # 结束端口
  step: 1          # 端口间隔

# UPnP配置
upnp:
  discovery_timeout: 10s    # 设备发现超时时间
  mapping_duration: 1h      # 端口映射持续时间，0表示永久
  retry_attempts: 3         # 重试次数
  retry_delay: 5s           # 重试延迟
  health_check_interval: 1m # 健康检查间隔
  max_fail_count: 3         # 最大失败次数
  keep_alive_interval: 2m   # 保活间隔
  max_cache_size: 10        # 最大缓存大小
  cache_ttl: 10m            # 缓存TTL
  enable_retry: true        # 启用重试机制
  retry_max_attempts: 5     # 最大重试次数
  retry_backoff_factor: 2.0 # 重试退避因子

# 网络接口配置
network:
  preferred_interfaces: ["eth0", "wlan0"]  # 优先使用的网络接口
  exclude_interfaces: ["lo", "docker"]     # 排除的网络接口

# 日志配置
log:
  level: "info"
  format: "json"
  file: "/var/log/auto-upnp.log"
  max_size: 10485760  # 10MB
  backup_count: 5

# 监控配置
monitor:
  check_interval: 10s       # 端口状态检查间隔
  cleanup_interval: 5m      # 清理无效映射间隔
  max_mappings: 100         # 最大端口映射数量

# 管理员配置
admin:
  enabled: true
  username: "admin"
  password: "admin"
  host: "0.0.0.0"
  data_dir: "/var/lib/auto-upnp"
EOF

    # 设置配置文件权限
    chmod 644 "$CONFIG_FILE"
    
    log_info "配置文件生成完成: ${CONFIG_FILE}"
}

# 生成systemd服务文件
generate_service() {
    log_step "生成systemd服务文件..."
    
    # 备份现有服务文件
    if [[ -f "$SERVICE_FILE" ]]; then
        log_warn "发现现有服务文件，创建备份"
        mv "$SERVICE_FILE" "${SERVICE_FILE}.backup.$(date +%Y%m%d_%H%M%S)"
    fi
    
    # 生成服务文件
    cat > "$SERVICE_FILE" << EOF
[Unit]
Description=Auto UPnP Service
Documentation=https://github.com/${GITHUB_REPO}
After=network.target
Wants=network.target

[Service]
Type=simple
User=root
Group=root
ExecStart=${BINARY_PATH} -config ${CONFIG_FILE}
ExecReload=/bin/kill -HUP \$MAINPID
Restart=always
RestartSec=10
StandardOutput=null
StandardError=null
SyslogIdentifier=auto-upnp

# 安全设置
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=${CONFIG_DIR} /var/log /var/lib/auto-upnp

[Install]
WantedBy=multi-user.target
EOF

    # 重新加载systemd
    systemctl daemon-reload
    
    log_info "服务文件生成完成: ${SERVICE_FILE}"
}

# 创建日志目录
setup_logging() {
    log_step "设置日志目录..."
    
    # 创建日志目录
    mkdir -p /var/log
    touch /var/log/auto-upnp.log
    chmod 666 /var/log/auto-upnp.log
    
    # 确保日志文件可以被服务写入
    chown root:root /var/log/auto-upnp.log
    
    log_info "日志目录设置完成"
}

# 创建数据目录
setup_data_dir() {
    log_step "创建数据目录..."
    mkdir -p /var/lib/auto-upnp
    chmod 755 /var/lib/auto-upnp
    chown root:root /var/lib/auto-upnp
}

# 显示安装完成信息
show_completion_info() {
    echo
    log_info "=== 安装完成 ==="
    echo
    log_info "二进制文件: ${BINARY_PATH}"
    log_info "配置文件: ${CONFIG_FILE}"
    log_info "服务文件: ${SERVICE_FILE}"
    log_info "日志文件: /var/log/auto-upnp.log"
    log_info "数据目录: /var/lib/auto-upnp"
    echo
    log_warn "请编辑配置文件: ${CONFIG_FILE}"
    log_info "配置文件说明:"
    echo "  - port_range: 设置要监控的端口范围"
    echo "  - upnp: UPnP相关配置"
    echo "  - network: 网络接口配置"
    echo "  - log: 日志配置"
    echo "  - monitor: 监控配置"
    echo "  - admin: 管理员配置"
    echo
    log_info "服务管理命令:"
    echo "  启动服务: systemctl start ${SERVICE_NAME}"
    echo "  停止服务: systemctl stop ${SERVICE_NAME}"
    echo "  重启服务: systemctl restart ${SERVICE_NAME}"
    echo "  查看状态: systemctl status ${SERVICE_NAME}"
    echo "  查看日志: journalctl -u ${SERVICE_NAME} -f"
    echo "  查看文件日志: tail -f /var/log/auto-upnp.log"
    echo "  开机自启: systemctl enable ${SERVICE_NAME}"
    echo "  禁用自启: systemctl disable ${SERVICE_NAME}"
    echo
    log_info "测试命令:"
    echo "  测试配置: ${BINARY_PATH} -config ${CONFIG_FILE} -test"
    echo "  查看帮助: ${BINARY_PATH} -help"
    echo
}

# 主安装流程
main() {
    echo "=========================================="
    echo "        自动UPnP服务安装脚本"
    echo "=========================================="
    echo
    
    # 检查root权限
    check_root
    
    # 显示代理配置
    show_proxy_config
    
    # 检测系统平台
    detect_platform
    
    # 获取最新版本
    get_latest_version
    
    # 下载二进制文件
    download_binary
    
    # 安装二进制文件
    install_binary
    
    # 生成配置文件
    generate_config
    
    # 生成服务文件
    generate_service
    
    # 设置日志
    setup_logging

    # 创建数据目录
    setup_data_dir
    
    # 显示完成信息
    show_completion_info
    
    log_info "安装完成！"
}

# 卸载函数
uninstall() {
    log_step "开始卸载自动UPnP服务..."
    
    # 停止并禁用服务
    if systemctl is-active --quiet "${SERVICE_NAME}"; then
        log_info "停止服务..."
        systemctl stop "${SERVICE_NAME}"
    fi
    
    if systemctl is-enabled --quiet "${SERVICE_NAME}"; then
        log_info "禁用服务..."
        systemctl disable "${SERVICE_NAME}"
    fi
    
    # 删除服务文件
    if [[ -f "$SERVICE_FILE" ]]; then
        log_info "删除服务文件..."
        rm -f "$SERVICE_FILE"
        systemctl daemon-reload
    fi
    
    # 删除二进制文件
    if [[ -f "$BINARY_PATH" ]]; then
        log_info "删除二进制文件..."
        rm -f "$BINARY_PATH"
    fi
    
    # 删除配置文件（可选）
    if [[ -d "$CONFIG_DIR" ]]; then
        log_warn "配置文件目录: ${CONFIG_DIR}"
        read -p "是否删除配置文件目录？(y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            rm -rf "$CONFIG_DIR"
            log_info "配置文件目录已删除"
        fi
    fi

    # 删除数据目录
    if [[ -d "/var/lib/auto-upnp" ]]; then
        log_warn "数据目录: /var/lib/auto-upnp"
        read -p "是否删除数据目录？(y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            rm -rf "/var/lib/auto-upnp"
            log_info "数据目录已删除"
        fi
    fi

    log_info "卸载完成！"
}

# 升级函数
upgrade() {
    log_step "开始升级自动UPnP服务..."
    
    # 检查二进制文件是否存在
    if [[ ! -f "$BINARY_PATH" ]]; then
        log_error "未找到现有安装的二进制文件: ${BINARY_PATH}"
        log_info "请先运行完整安装: $0"
        exit 1
    fi
    
    # 获取当前版本
    log_info "检查当前版本..."
    CURRENT_VERSION=$("$BINARY_PATH" -version 2>/dev/null || echo "未知")
    log_info "当前版本: ${CURRENT_VERSION}"
    
    # 检测系统平台
    detect_platform
    
    # 获取最新版本
    get_latest_version
    
    # 检查是否需要升级
    if [[ "$CURRENT_VERSION" == "$LATEST_VERSION" ]]; then
        log_info "当前已是最新版本，无需升级"
        exit 0
    fi
    
    # 检查服务是否正在运行
    if systemctl is-active --quiet "${SERVICE_NAME}"; then
        log_warn "服务正在运行，将停止服务进行升级..."
        systemctl stop "${SERVICE_NAME}"
        SERVICE_WAS_RUNNING=true
    else
        SERVICE_WAS_RUNNING=false
    fi
    
    # 备份当前二进制文件
    log_info "备份当前二进制文件..."
    BACKUP_PATH="${BINARY_PATH}.backup.$(date +%Y%m%d_%H%M%S)"
    cp "$BINARY_PATH" "$BACKUP_PATH"
    log_info "备份文件: ${BACKUP_PATH}"
    
    # 下载新的二进制文件
    download_binary
    
    # 安装新的二进制文件（跳过备份，因为已经手动备份了）
    install_binary true
    
    # 验证新二进制文件
    log_step "验证新二进制文件..."
    if ! "$BINARY_PATH" -version >/dev/null 2>&1; then
        log_error "新二进制文件验证失败，恢复备份..."
        mv "$BACKUP_PATH" "$BINARY_PATH"
        if [[ "$SERVICE_WAS_RUNNING" == true ]]; then
            log_info "重新启动服务..."
            systemctl start "${SERVICE_NAME}"
        fi
        exit 1
    fi
    
    # 删除备份文件
    rm -f "$BACKUP_PATH"
    log_info "备份文件已删除"
    
    # 如果服务之前在运行，重新启动服务
    if [[ "$SERVICE_WAS_RUNNING" == true ]]; then
        log_info "重新启动服务..."
        systemctl start "${SERVICE_NAME}"
    fi
    
    # 显示升级完成信息
    echo
    log_info "=== 升级完成 ==="
    echo
    log_info "升级前版本: ${CURRENT_VERSION}"
    log_info "升级后版本: ${LATEST_VERSION}"
    log_info "新二进制文件: ${BINARY_PATH}"
    log_info "服务状态: $(systemctl is-active ${SERVICE_NAME} 2>/dev/null || echo 'stopped')"
    echo
    log_info "升级命令:"
    echo "  查看服务状态: systemctl status ${SERVICE_NAME}"
    echo "  查看服务日志: journalctl -u ${SERVICE_NAME} -f"
    echo "  查看文件日志: tail -f /var/log/auto-upnp.log"
    echo
    log_info "升级完成！"
}

# 检查更新函数
check_update() {
    log_step "检查自动UPnP服务更新..."
    
    # 检查二进制文件是否存在
    if [[ ! -f "$BINARY_PATH" ]]; then
        log_error "未找到现有安装的二进制文件: ${BINARY_PATH}"
        log_info "请先运行完整安装: $0"
        exit 1
    fi
    
    # 获取当前版本
    log_info "检查当前版本..."
    CURRENT_VERSION=$("$BINARY_PATH" -version 2>/dev/null || echo "未知")
    log_info "当前版本: ${CURRENT_VERSION}"
    
    # 检测系统平台
    detect_platform
    
    # 获取最新版本
    get_latest_version
    
    # 比较版本
    if [[ "$CURRENT_VERSION" == "$LATEST_VERSION" ]]; then
        log_info "当前已是最新版本: ${LATEST_VERSION}"
    else
        log_warn "发现新版本: ${LATEST_VERSION}"
        log_info "当前版本: ${CURRENT_VERSION}"
        log_info "运行以下命令进行升级:"
        echo "  sudo $0 --upgrade"
        if [[ "$USE_PROXY" == "true" ]]; then
            echo "  sudo USE_PROXY=true $0 --upgrade"
        fi
    fi
}

# 检查参数
case "${1:-}" in
    --uninstall|-u)
        check_root
        uninstall
        ;;
    --upgrade|-U)
        check_root
        upgrade
        ;;
    --check-update|-c)
        check_update
        ;;
    --help|-h)
        echo "用法: $0 [选项]"
        echo "选项:"
        echo "  --uninstall, -u    卸载服务"
        echo "  --upgrade, -U      升级二进制文件"
        echo "  --check-update, -c 检查更新"
        echo "  --help, -h         显示帮助信息"
        echo
        echo "环境变量:"
        echo "  USE_PROXY          是否使用代理 (true/false, 默认: false)"
        echo "  PROXY              代理服务器地址 (默认: https://ghfast.top)"
        echo
        echo "示例:"
        echo "  sudo $0                                    # 安装服务"
        echo "  sudo USE_PROXY=true $0                     # 使用代理安装"
        echo "  sudo USE_PROXY=true PROXY=https://ghproxy.com $0  # 使用自定义代理"
        echo "  sudo $0 --check-update                     # 检查更新"
        echo "  sudo $0 --upgrade                          # 升级二进制文件"
        echo "  sudo USE_PROXY=true $0 --upgrade           # 使用代理升级"
        echo "  sudo $0 --uninstall                        # 卸载服务"
        ;;
    "")
        main
        ;;
    *)
        log_error "未知参数: $1"
        echo "使用 --help 查看帮助信息"
        exit 1
        ;;
esac 