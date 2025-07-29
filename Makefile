# 自动UPnP服务 Makefile

# 变量定义
BINARY_NAME=auto-upnp
BUILD_DIR=build
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}"
# 静态编译标志
STATIC_FLAGS=CGO_ENABLED=0 GOOS=linux GOARCH=amd64

# 默认目标
.PHONY: all
all: clean build

# 构建项目
.PHONY: build
build:
	@echo "构建 ${BINARY_NAME}..."
	@mkdir -p ${BUILD_DIR}
	go build ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME} cmd/main.go
	@echo "构建完成: ${BUILD_DIR}/${BINARY_NAME}"

# 静态构建（解决GLIBC版本问题）
.PHONY: build-static
build-static:
	@echo "静态构建 ${BINARY_NAME}..."
	@mkdir -p ${BUILD_DIR}
	${STATIC_FLAGS} go build ${LDFLAGS} -a -installsuffix cgo -o ${BUILD_DIR}/${BINARY_NAME}-static cmd/main.go
	@echo "静态构建完成: ${BUILD_DIR}/${BINARY_NAME}-static"

# 构建多个平台
.PHONY: build-all
build-all: clean
	@echo "构建多平台版本..."
	@mkdir -p ${BUILD_DIR}
	
	# Linux AMD64 (静态)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -a -installsuffix cgo -o ${BUILD_DIR}/${BINARY_NAME}-linux-amd64 cmd/main.go
	
	# Linux ARM64 (静态)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ${LDFLAGS} -a -installsuffix cgo -o ${BUILD_DIR}/${BINARY_NAME}-linux-arm64 cmd/main.go
	
	# macOS AMD64
	GOOS=darwin GOARCH=amd64 go build ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME}-darwin-amd64 cmd/main.go
	
	# macOS ARM64
	GOOS=darwin GOARCH=arm64 go build ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME}-darwin-arm64 cmd/main.go
	
	# Windows AMD64
	GOOS=windows GOARCH=amd64 go build ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME}-windows-amd64.exe cmd/main.go
	
	@echo "多平台构建完成"

# 构建兼容旧版本GLIBC的版本
.PHONY: build-compatible
build-compatible:
	@echo "构建兼容旧版本GLIBC的版本..."
	@mkdir -p ${BUILD_DIR}
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -a -installsuffix cgo -ldflags '-extldflags "-static"' -o ${BUILD_DIR}/${BINARY_NAME}-compatible cmd/main.go
	@echo "兼容版本构建完成: ${BUILD_DIR}/${BINARY_NAME}-compatible"

# 安装依赖
.PHONY: deps
deps:
	@echo "安装依赖..."
	go mod tidy
	go mod download

# 运行测试
.PHONY: test
test:
	@echo "运行测试..."
	go test -v ./...

# 运行测试并生成覆盖率报告
.PHONY: test-coverage
test-coverage:
	@echo "运行测试并生成覆盖率报告..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "覆盖率报告已生成: coverage.html"

# 代码格式化
.PHONY: fmt
fmt:
	@echo "格式化代码..."
	go fmt ./...

# 代码检查
.PHONY: lint
lint:
	@echo "检查代码..."
	golangci-lint run

# 清理构建文件
.PHONY: clean
clean:
	@echo "清理构建文件..."
	rm -rf ${BUILD_DIR}
	rm -f coverage.out coverage.html
	rm -rf release

# 运行服务
.PHONY: run
run: build
	@echo "运行服务..."
	./${BUILD_DIR}/${BINARY_NAME}

# 运行服务（调试模式）
.PHONY: run-debug
run-debug: build
	@echo "运行服务（调试模式）..."
	./${BUILD_DIR}/${BINARY_NAME} -log-level debug

# 运行静态构建的服务
.PHONY: run-static
run-static: build-static
	@echo "运行静态构建的服务..."
	./${BUILD_DIR}/${BINARY_NAME}-static

# 创建发布包
.PHONY: release
release: build-all
	@echo "创建发布包..."
	@mkdir -p release
	@cd ${BUILD_DIR} && tar -czf ../release/${BINARY_NAME}-${VERSION}-linux-amd64.tar.gz ${BINARY_NAME}-linux-amd64
	@cd ${BUILD_DIR} && tar -czf ../release/${BINARY_NAME}-${VERSION}-linux-arm64.tar.gz ${BINARY_NAME}-linux-arm64
	@cd ${BUILD_DIR} && tar -czf ../release/${BINARY_NAME}-${VERSION}-darwin-amd64.tar.gz ${BINARY_NAME}-darwin-amd64
	@cd ${BUILD_DIR} && tar -czf ../release/${BINARY_NAME}-${VERSION}-darwin-arm64.tar.gz ${BINARY_NAME}-darwin-arm64
	@cd ${BUILD_DIR} && zip ../release/${BINARY_NAME}-${VERSION}-windows-amd64.zip ${BINARY_NAME}-windows-amd64.exe
	@echo "发布包已创建: release/"

# 安装到系统
.PHONY: install
install: build-static
	@echo "安装到系统..."
	sudo cp ${BUILD_DIR}/${BINARY_NAME}-static /usr/local/bin/${BINARY_NAME}
	@echo "安装完成"

# 卸载
.PHONY: uninstall
uninstall:
	@echo "卸载..."
	sudo rm -f /usr/local/bin/${BINARY_NAME}
	@echo "卸载完成"

# 显示帮助
.PHONY: help
help:
	@echo "可用的目标:"
	@echo "  build          - 构建项目"
	@echo "  build-static   - 静态构建（解决GLIBC版本问题）"
	@echo "  build-compatible - 构建兼容旧版本GLIBC的版本"
	@echo "  build-all      - 构建多平台版本"
	@echo "  deps           - 安装依赖"
	@echo "  test           - 运行测试"
	@echo "  test-coverage  - 运行测试并生成覆盖率报告"
	@echo "  fmt            - 格式化代码"
	@echo "  lint           - 检查代码"
	@echo "  clean          - 清理构建文件"
	@echo "  run            - 运行服务"
	@echo "  run-debug      - 运行服务（调试模式）"
	@echo "  run-static     - 运行静态构建的服务"
	@echo "  release        - 创建发布包"
	@echo "  install        - 安装到系统"
	@echo "  uninstall      - 卸载"
	@echo "  help           - 显示此帮助信息" 