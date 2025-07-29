package admin

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"time"

	"auto-upnp/config"
	"auto-upnp/internal/service"

	"github.com/sirupsen/logrus"
)

// AdminServer HTTP管理服务器
type AdminServer struct {
	config      *config.Config
	logger      *logrus.Logger
	autoService *service.AutoUPnPService
	server      *http.Server
	port        int
}

// NewAdminServer 创建新的管理服务器
func NewAdminServer(cfg *config.Config, logger *logrus.Logger, autoService *service.AutoUPnPService) *AdminServer {
	return &AdminServer{
		config:      cfg,
		logger:      logger,
		autoService: autoService,
	}
}

// Start 启动管理服务器
func (as *AdminServer) Start() error {
	if !as.config.Admin.Enabled {
		as.logger.Info("管理服务已禁用")
		return nil
	}

	// 找到可用的端口
	port, err := as.findAvailablePort()
	if err != nil {
		return fmt.Errorf("无法找到可用端口: %w", err)
	}
	as.port = port

	// 设置路由
	mux := http.NewServeMux()
	mux.HandleFunc("/", as.authMiddleware(as.handleIndex))
	mux.HandleFunc("/api/status", as.authMiddleware(as.handleStatus))
	mux.HandleFunc("/api/mappings", as.authMiddleware(as.handleMappings))
	mux.HandleFunc("/api/manual-mappings", as.authMiddleware(as.handleManualMappings))
	mux.HandleFunc("/api/add-mapping", as.authMiddleware(as.handleAddMapping))
	mux.HandleFunc("/api/remove-mapping", as.authMiddleware(as.handleRemoveMapping))
	mux.HandleFunc("/api/ports", as.authMiddleware(as.handlePorts))
	mux.HandleFunc("/api/upnp-status", as.authMiddleware(as.handleUPnPStatus))

	// 创建HTTP服务器
	as.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", as.config.Admin.Host, port),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	as.logger.WithFields(logrus.Fields{
		"host": as.config.Admin.Host,
		"port": port,
	}).Info("启动HTTP管理服务")

	go func() {
		if err := as.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			as.logger.WithError(err).Error("HTTP管理服务启动失败")
		}
	}()

	return nil
}

// Stop 停止管理服务器
func (as *AdminServer) Stop() error {
	if as.server != nil {
		as.logger.Info("停止HTTP管理服务")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return as.server.Shutdown(ctx)
	}
	return nil
}

// GetPort 获取实际使用的端口
func (as *AdminServer) GetPort() int {
	return as.port
}

// findAvailablePort 查找可用端口
func (as *AdminServer) findAvailablePort() (int, error) {
	startPort := as.config.PortRange.Start
	endPort := as.config.PortRange.End

	for port := startPort; port <= endPort; port += as.config.PortRange.Step {
		addr := fmt.Sprintf("%s:%d", as.config.Admin.Host, port)
		listener, err := net.Listen("tcp", addr)
		if err == nil {
			listener.Close()
			return port, nil
		}
	}

	return 0, fmt.Errorf("在端口范围 %d-%d 内没有找到可用端口", startPort, endPort)
}

// authMiddleware 认证中间件
func (as *AdminServer) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || !as.checkCredentials(username, password) {
			w.Header().Set("WWW-Authenticate", `Basic realm="Auto UPnP Admin"`)
			http.Error(w, "需要认证", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// checkCredentials 检查用户凭据
func (as *AdminServer) checkCredentials(username, password string) bool {
	expectedUsername := as.config.Admin.Username
	expectedPassword := as.config.Admin.Password

	return subtle.ConstantTimeCompare([]byte(username), []byte(expectedUsername)) == 1 &&
		subtle.ConstantTimeCompare([]byte(password), []byte(expectedPassword)) == 1
}

// handleIndex 处理首页
func (as *AdminServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	tmpl := template.Must(template.New("index").Parse(adminHTML))
	data := map[string]interface{}{
		"Title": "Auto UPnP 管理界面",
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		as.logger.WithError(err).Error("渲染首页模板失败")
		http.Error(w, "内部服务器错误", http.StatusInternalServerError)
	}
}

// handleStatus 处理状态API
func (as *AdminServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	status := as.autoService.GetStatus()

	// 添加管理服务信息
	status["admin_service"] = map[string]interface{}{
		"enabled": as.config.Admin.Enabled,
		"host":    as.config.Admin.Host,
		"port":    as.port,
		"url":     fmt.Sprintf("http://%s:%d", as.config.Admin.Host, as.port),
	}

	as.writeJSON(w, status)
}

// handleMappings 处理端口映射API
func (as *AdminServer) handleMappings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	mappings := as.autoService.GetPortMappings()

	// 转换映射数据以包含活跃状态
	response := make(map[string]interface{})
	for key, mapping := range mappings {
		response[key] = map[string]interface{}{
			"InternalPort":   mapping.InternalPort,
			"ExternalPort":   mapping.ExternalPort,
			"Protocol":       mapping.Protocol,
			"InternalClient": mapping.InternalClient,
			"Description":    mapping.Description,
			"LeaseDuration":  mapping.LeaseDuration,
			"CreatedAt":      mapping.CreatedAt,
			"Active":         true, // 如果存在映射，则认为它是活跃的
		}
	}

	as.writeJSON(w, response)
}

// handleAddMapping 处理添加映射API
func (as *AdminServer) handleAddMapping(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		as.writeJSONResponse(w, http.StatusMethodNotAllowed, "方法不允许", nil)
		return
	}

	// 设置响应头
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	// 读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		as.writeJSONResponse(w, http.StatusBadRequest, "读取请求体失败", nil)
		return
	}
	defer r.Body.Close()

	// 解析JSON请求
	var req AddMappingRequest
	if err := json.Unmarshal(body, &req); err != nil {
		as.writeJSONResponse(w, http.StatusBadRequest, "JSON格式错误", nil)
		return
	}

	// 验证必填字段
	if req.InternalPort <= 0 || req.InternalPort > 65535 {
		as.writeJSONResponse(w, http.StatusBadRequest, "内部端口格式错误", nil)
		return
	}

	// 如果InternalPort在PortRange范围内，则返回错误
	if req.InternalPort >= as.config.PortRange.Start && req.InternalPort <= as.config.PortRange.End {
		as.writeJSONResponse(w, http.StatusBadRequest, "内部端口在端口范围内,请勿重复添加", nil)
		return
	}

	if req.ExternalPort <= 0 || req.ExternalPort > 65535 {
		as.writeJSONResponse(w, http.StatusBadRequest, "外部端口格式错误", nil)
		return
	}

	// 设置默认值
	if req.Protocol == "" {
		req.Protocol = "TCP"
	}

	if req.Description == "" {
		req.Description = fmt.Sprintf("Manual %d->%d", req.InternalPort, req.ExternalPort)
	}

	// 添加映射
	if err := as.autoService.AddManualMapping(req.InternalPort, req.ExternalPort, req.Protocol, req.Description); err != nil {
		as.logger.WithError(err).Error("添加手动映射失败")
		as.writeJSONResponse(w, http.StatusInternalServerError, fmt.Sprintf("添加映射失败: %v", err), nil)
		return
	}

	as.writeJSONResponse(w, http.StatusOK, "映射添加成功", nil)
}

// handleRemoveMapping 处理删除映射API
func (as *AdminServer) handleRemoveMapping(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		as.writeJSONResponse(w, http.StatusMethodNotAllowed, "方法不允许", nil)
		return
	}

	// 设置响应头
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	// 读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		as.writeJSONResponse(w, http.StatusBadRequest, "读取请求体失败", nil)
		return
	}
	defer r.Body.Close()

	// 解析JSON请求
	var req RemoveMappingRequest
	if err := json.Unmarshal(body, &req); err != nil {
		as.writeJSONResponse(w, http.StatusBadRequest, "JSON格式错误", nil)
		return
	}

	// 验证必填字段
	if req.InternalPort <= 0 || req.InternalPort > 65535 {
		as.writeJSONResponse(w, http.StatusBadRequest, "内部端口格式错误", nil)
		return
	}

	if req.ExternalPort <= 0 || req.ExternalPort > 65535 {
		as.writeJSONResponse(w, http.StatusBadRequest, "外部端口格式错误", nil)
		return
	}

	// 设置默认值
	if req.Protocol == "" {
		req.Protocol = "TCP"
	}

	// 删除映射
	if err := as.autoService.RemoveManualMapping(req.InternalPort, req.ExternalPort, req.Protocol); err != nil {
		as.logger.WithError(err).Error("删除手动映射失败")
		as.writeJSONResponse(w, http.StatusInternalServerError, fmt.Sprintf("删除映射失败: %v", err), nil)
		return
	}

	as.writeJSONResponse(w, http.StatusOK, "映射删除成功", nil)
}

// handlePorts 处理端口状态API
func (as *AdminServer) handlePorts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	activePorts := as.autoService.GetActivePorts()
	inactivePorts := as.autoService.GetInactivePorts()

	response := map[string]interface{}{
		"active_ports":   activePorts,
		"inactive_ports": inactivePorts,
	}

	as.writeJSON(w, response)
}

// handleManualMappings 处理手动映射API
func (as *AdminServer) handleManualMappings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	allMappings := as.autoService.GetManualMappings()
	activeMappings := as.autoService.GetActiveManualMappings()
	inactiveMappings := as.autoService.GetInactiveManualMappings()

	response := map[string]interface{}{
		"total_mappings":         len(allMappings),
		"active_mappings":        len(activeMappings),
		"inactive_mappings":      len(inactiveMappings),
		"all_mappings":           allMappings,
		"active_mappings_list":   activeMappings,
		"inactive_mappings_list": inactiveMappings,
	}

	as.writeJSON(w, response)
}

// handleUPnPStatus 处理UPnP状态API
func (as *AdminServer) handleUPnPStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		as.writeJSONResponse(w, http.StatusMethodNotAllowed, "方法不允许", nil)
		return
	}

	clientCount := as.autoService.GetUPnPClientCount()
	isAvailable := as.autoService.IsUPnPAvailable()

	status := "不可用"
	if isAvailable {
		status = "可用"
	}

	response := map[string]interface{}{
		"client_count": clientCount,
		"available":    isAvailable,
		"status":       status,
	}

	as.writeJSON(w, response)
}

// writeJSON 写入JSON响应
func (as *AdminServer) writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		as.logger.WithError(err).Error("编码JSON响应失败")
		http.Error(w, "内部服务器错误", http.StatusInternalServerError)
	}
}

// writeJSONResponse 写入标准JSON响应
func (as *AdminServer) writeJSONResponse(w http.ResponseWriter, statusCode int, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)

	response := APIResponse{
		Status:  "error",
		Message: message,
	}

	if statusCode >= 200 && statusCode < 300 {
		response.Status = "success"
	}

	if data != nil {
		response.Data = data
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		as.logger.WithError(err).Error("编码JSON响应失败")
		http.Error(w, "内部服务器错误", http.StatusInternalServerError)
	}
}
