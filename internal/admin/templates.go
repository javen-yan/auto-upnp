package admin

// adminHTML 管理界面HTML模板
const adminHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            padding: 20px;
        }
        
        .container {
            max-width: 1200px;
            margin: 0 auto;
            background: white;
            border-radius: 12px;
            box-shadow: 0 20px 40px rgba(0,0,0,0.1);
            overflow: hidden;
        }
        
        .header {
            background: linear-gradient(135deg, #4facfe 0%, #00f2fe 100%);
            color: white;
            padding: 30px;
            text-align: center;
        }
        
        .header h1 {
            font-size: 2.5em;
            margin-bottom: 10px;
            font-weight: 300;
        }
        
        .header p {
            opacity: 0.9;
            font-size: 1.1em;
        }

        .add-mapping-button {
            margin-top: 20px;
            text-align: right;
        }
        
        .github-links {
            display: flex;
            gap: 15px;
            justify-content: center;
            margin-top: 15px;
            flex-wrap: wrap;
        }
        
        .github-link {
            display: inline-flex;
            align-items: center;
            gap: 8px;
            padding: 8px 16px;
            background: rgba(255, 255, 255, 0.1);
            color: white;
            text-decoration: none;
            border-radius: 6px;
            font-size: 14px;
            transition: all 0.3s ease;
            backdrop-filter: blur(10px);
            border: 1px solid rgba(255, 255, 255, 0.2);
        }
        
        .github-link:hover {
            background: rgba(255, 255, 255, 0.2);
            transform: translateY(-2px);
            text-decoration: none;
            color: white;
        }
        
        .github-link svg {
            width: 16px;
            height: 16px;
        }
        
        .content {
            padding: 30px;
        }
        
        .section {
            margin-bottom: 40px;
            background: #f8f9fa;
            border-radius: 8px;
            padding: 25px;
            border-left: 4px solid #4facfe;
        }
        
        .section h2 {
            color: #333;
            margin-bottom: 20px;
            font-size: 1.5em;
            font-weight: 600;
        }
        
        .status-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 20px;
            margin-bottom: 30px;
        }
        
        .status-card {
            background: white;
            padding: 20px;
            border-radius: 8px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
            text-align: center;
        }
        
        .status-card h3 {
            color: #666;
            font-size: 0.9em;
            text-transform: uppercase;
            letter-spacing: 1px;
            margin-bottom: 10px;
        }
        
        .status-card .value {
            font-size: 2em;
            font-weight: bold;
            color: #4facfe;
        }
        
        .mappings-table {
            width: 100%;
            border-collapse: collapse;
            background: white;
            border-radius: 8px;
            overflow: hidden;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
            font-size: 14px;
        }
        
        .mappings-table th,
        .mappings-table td {
            padding: 12px 8px;
            text-align: left;
            border-bottom: 1px solid #eee;
            vertical-align: top;
        }
        
        .mappings-table th {
            background: #4facfe;
            color: white;
            font-weight: 600;
            white-space: nowrap;
        }
        
        .mappings-table tr:hover {
            background: #f8f9fa;
        }
        
        /* 表格列宽控制 */
        .mappings-table .col-port {
            width: 80px;
            min-width: 80px;
        }
        
        .mappings-table .col-protocol {
            width: 60px;
            min-width: 60px;
        }
        
        .mappings-table .col-type {
            width: 70px;
            min-width: 70px;
        }
        
        .mappings-table .col-status {
            width: 70px;
            min-width: 70px;
        }
        
        .mappings-table .col-external {
            width: 120px;
            min-width: 120px;
            word-break: break-all;
        }
        
        .mappings-table .col-time {
            width: 140px;
            min-width: 140px;
            font-size: 12px;
        }
        
        .mappings-table .col-action {
            width: 80px;
            min-width: 80px;
        }
        
        .mappings-table .col-description {
            max-width: 200px;
            word-break: break-word;
        }
        
        .manual-mapping-stats {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
            gap: 15px;
            margin-bottom: 20px;
        }
        
        .stat-item {
            background: white;
            padding: 15px;
            border-radius: 6px;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
            text-align: center;
        }
        
        .stat-item h4 {
            color: #666;
            font-size: 0.8em;
            text-transform: uppercase;
            letter-spacing: 1px;
            margin-bottom: 8px;
        }
        
        .stat-item .value {
            font-size: 1.5em;
            font-weight: bold;
        }
        
        .stat-item.active .value {
            color: #4caf50;
        }
        
        .stat-item.inactive .value {
            color: #f44336;
        }
        
        .status-badge {
            display: inline-block;
            padding: 4px 8px;
            border-radius: 12px;
            font-size: 0.8em;
            font-weight: 500;
            text-transform: uppercase;
        }
        
        .status-badge.active {
            background: #e8f5e8;
            color: #2e7d32;
        }
        
        .status-badge.inactive {
            background: #ffebee;
            color: #c62828;
        }
        
        .status-badge {
            background: #e3f2fd;
            color: #1976d2;
        }
        
        .btn {
            background: #4facfe;
            color: white;
            border: none;
            padding: 10px 10px;
            border-radius: 6px;
            cursor: pointer;
            font-size: 14px;
            transition: all 0.3s ease;
        }
        
        .btn:hover {
            background: #3a8bfe;
            transform: translateY(-2px);
        }
        
        .btn-danger {
            background: #ff6b6b;
        }
        
        .btn-danger:hover {
            background: #ff5252;
        }
        
        .btn-success {
            background: #4caf50;
        }
        
        .btn-success:hover {
            background: #45a049;
        }
        
        .btn-warning {
            background: #ff9800;
        }
        
        .btn-warning:hover {
            background: #f57c00;
        }
        
        .form-group {
            margin-bottom: 20px;
        }
        
        .form-group label {
            display: block;
            margin-bottom: 8px;
            font-weight: 600;
            color: #333;
        }
        
        .form-group input,
        .form-group select {
            width: 100%;
            padding: 12px;
            border: 2px solid #e1e5e9;
            border-radius: 6px;
            font-size: 14px;
            transition: border-color 0.3s ease;
        }
        
        .form-group input:focus,
        .form-group select:focus {
            outline: none;
            border-color: #4facfe;
        }
        
        .form-row {
            display: grid;
            grid-template-columns: 1fr 1fr 1fr 1fr;
            gap: 15px;
            align-items: end;
        }
        
        /* 弹窗内的表单样式 */
        .modal-body .form-row {
            grid-template-columns: 1fr 1fr;
            gap: 20px;
        }
        
        .modal-body .form-group {
            margin-bottom: 20px;
        }
        
        .modal-body .form-group input,
        .modal-body .form-group select {
            width: 100%;
            padding: 12px 15px;
            border: 2px solid #e1e5e9;
            border-radius: 8px;
            font-size: 14px;
            transition: border-color 0.3s ease;
            box-sizing: border-box;
        }
        
        .modal-body .form-group input:focus,
        .modal-body .form-group select:focus {
            outline: none;
            border-color: #4facfe;
            box-shadow: 0 0 0 3px rgba(79, 172, 254, 0.1);
        }
        
        .ports-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(80px, 1fr));
            gap: 10px;
            margin-top: 15px;
        }
        
        .port-item {
            background: white;
            padding: 10px;
            border-radius: 6px;
            text-align: center;
            border: 2px solid #e1e5e9;
            cursor: pointer;
            transition: all 0.3s ease;
        }
        
        .port-item.active {
            background: #4facfe;
            color: white;
            border-color: #4facfe;
        }
        
        .port-item.inactive {
            background: #f8f9fa;
            color: #666;
        }
        
        .loading {
            text-align: center;
            padding: 20px;
            color: #666;
        }
        
        .error {
            background: #ffebee;
            color: #c62828;
            padding: 15px;
            border-radius: 6px;
            margin-bottom: 20px;
            border-left: 4px solid #f44336;
        }
        
        .success {
            background: #e8f5e8;
            color: #2e7d32;
            padding: 15px;
            border-radius: 6px;
            margin-bottom: 20px;
            border-left: 4px solid #4caf50;
        }
        
        .message {
            padding: 15px;
            border-radius: 6px;
            margin-bottom: 20px;
            border-left: 4px solid;
            font-weight: 500;
        }
        
        .message.error {
            background: #ffebee;
            color: #c62828;
            border-left-color: #f44336;
        }
        
        .message.success {
            background: #e8f5e8;
            color: #2e7d32;
            border-left-color: #4caf50;
        }
        
        .message.warning {
            background: #fff3e0;
            color: #ef6c00;
            border-left-color: #ff9800;
        }
        
        .nat-status {
            display: flex;
            align-items: center;
            gap: 10px;
            margin-bottom: 15px;
        }
        
        .nat-indicator {
            width: 12px;
            height: 12px;
            border-radius: 50%;
            background: #ccc;
        }
        
        .nat-indicator.enabled {
            background: #4caf50;
        }
        
        .nat-indicator.disabled {
            background: #f44336;
        }
        
        .tab-container {
            margin-bottom: 20px;
        }
        
        .tab-buttons {
            display: flex;
            border-bottom: 2px solid #e1e5e9;
            margin-bottom: 20px;
        }
        
        .tab-button {
            background: none;
            border: none;
            padding: 12px 24px;
            cursor: pointer;
            font-size: 14px;
            font-weight: 500;
            color: #666;
            border-bottom: 2px solid transparent;
            transition: all 0.3s ease;
        }
        
        .tab-button.active {
            color: #4facfe;
            border-bottom-color: #4facfe;
        }
        
        .tab-content {
            display: none;
        }
        
        .tab-content.active {
            display: block;
        }
        
        @media (max-width: 768px) {
            .form-row {
                grid-template-columns: 1fr;
            }
            
            .status-grid {
                grid-template-columns: 1fr;
            }
            
            .ports-grid {
                grid-template-columns: repeat(auto-fill, minmax(60px, 1fr));
            }
            
            .tab-buttons {
                flex-direction: column;
            }
            
            .mappings-table {
                font-size: 12px;
            }
            
            .mappings-table th,
            .mappings-table td {
                padding: 8px 4px;
            }
            
            .mappings-table .col-time {
                display: none;
            }
            
            .mappings-table .col-external {
                max-width: 100px;
                word-break: break-all;
            }
            
            .mappings-table .col-description {
                max-width: 120px;
            }
            
            .github-links {
                flex-direction: column;
                gap: 10px;
            }
            
            .github-link {
                justify-content: center;
                padding: 10px 16px;
            }
        }
        
        @media (max-width: 480px) {
            .mappings-table .col-external,
            .mappings-table .col-description {
                display: none;
            }
            
            .mappings-table .col-type {
                display: none;
            }
        }
        
        /* 弹窗样式 */
        .modal {
            display: none;
            position: fixed;
            z-index: 1000;
            left: 0;
            top: 0;
            width: 100%;
            height: 100%;
            background-color: rgba(0,0,0,0.5);
            backdrop-filter: blur(5px);
        }
        
        .modal-content {
            background-color: white;
            margin: 5% auto;
            border-radius: 12px;
            width: 90%;
            max-width: 600px;
            box-shadow: 0 20px 40px rgba(0,0,0,0.2);
            animation: modalSlideIn 0.3s ease-out;
        }
        
        @keyframes modalSlideIn {
            from {
                opacity: 0;
                transform: translateY(-50px);
            }
            to {
                opacity: 1;
                transform: translateY(0);
            }
        }
        
        .modal-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 20px 25px;
            border-bottom: 1px solid #e1e5e9;
            background: linear-gradient(135deg, #4facfe 0%, #00f2fe 100%);
            color: white;
            border-radius: 12px 12px 0 0;
        }
        
        .modal-header h3 {
            margin: 0;
            font-size: 1.3em;
            font-weight: 600;
        }
        
        .close {
            color: white;
            font-size: 28px;
            font-weight: bold;
            cursor: pointer;
            line-height: 1;
            transition: opacity 0.3s ease;
        }
        
        .close:hover {
            opacity: 0.7;
        }
        
        .modal-body {
            padding: 25px;
        }
        
        .modal-footer {
            display: flex;
            justify-content: flex-end;
            gap: 15px;
            padding: 20px 25px;
            border-top: 1px solid #e1e5e9;
            background: #f8f9fa;
            border-radius: 0 0 12px 12px;
        }
        
        .btn-secondary {
            background: #6c757d;
        }
        
        .btn-secondary:hover {
            background: #5a6268;
        }
        
        /* 弹窗响应式设计 */
        @media (max-width: 768px) {
            .modal-content {
                margin: 10% auto;
                width: 95%;
                max-width: none;
            }
            
            .modal-body {
                padding: 20px;
            }
            
            .modal-body .form-row {
                grid-template-columns: 1fr;
                gap: 15px;
            }
            
            .modal-footer {
                padding: 15px 20px;
                flex-direction: column;
            }
            
            .modal-footer .btn {
                width: 100%;
                margin-bottom: 10px;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Auto UPnP 管理界面</h1>
            <p>自动端口映射管理服务 UPnP + TURN</p>

            <!-- GitHub 链接 -->
            <div class="github-links">
                <a href="https://github.com/javen-yan/auto-upnp" target="_blank" class="github-link">
                    <svg viewBox="0 0 24 24" fill="currentColor">
                        <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
                    </svg>
                    GitHub Repo
                </a>
                <a href="https://javen-yan.github.io/auto-upnp-doc/" target="_blank" class="github-link">
                    <svg viewBox="0 0 24 24" fill="currentColor">
                        <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z"/>
                    </svg>
                    Documentation
                </a>
            </div>

            <!-- 添加映射按钮 -->
            <div class="add-mapping-button">
                <button type="button" class="btn" onclick="openAddMappingModal()">
                    <span style="margin-right: 8px;">+</span>添加映射
                </button>
            </div>
        </div>
        
        <div class="content">
            <!-- 服务状态 -->
            <div class="section">
                <h2>服务状态</h2>
                <div class="status-grid" id="statusGrid">
                    <div class="loading">加载中...</div>
                </div>
            </div>
            
            <!-- 映射管理标签页 -->
            <div class="section">
                <h2>映射管理</h2>
                <div class="tab-container">
                    <div class="tab-buttons">
                        <button class="tab-button active" onclick="switchTab('auto')">自动映射</button>
                        <button class="tab-button" onclick="switchTab('manual')">手动映射</button>
                    </div>
                    <!-- 自动映射标签页 -->
                    <div id="autoTab" class="tab-content active">
                        <div id="mappingsTable">
                            <div class="loading">加载中...</div>
                        </div>
                    </div>
                    
                    <!-- 手动映射标签页 -->
                    <div id="manualTab" class="tab-content">
                        <div id="manualMappingsTable">
                            <div class="loading">加载中...</div>
                        </div>
                    </div>
                </div>
            </div>
            
            <!-- 端口状态 -->
            <div class="section">
                <h2>活跃端口监控</h2>
                <div id="portsStatus">
                    <div class="loading">加载中...</div>
                </div>
            </div>
        </div>
    </div>

    <!-- 添加映射弹窗 -->
    <div id="addMappingModal" class="modal">
        <div class="modal-content">
            <div class="modal-header">
                <h3>添加端口映射</h3>
                <span class="close" onclick="closeAddMappingModal()">&times;</span>
            </div>
            <form id="addMappingForm">
                <div class="modal-body">
                    <div class="form-row">
                        <div class="form-group">
                            <label for="internalPort">内部端口</label>
                            <input type="number" id="internalPort" name="internal_port" min="1" max="65535" placeholder="例如: 8080" required>
                        </div>
                        <div class="form-group">
                            <label for="externalPort">外部端口</label>
                            <input type="number" id="externalPort" name="external_port" min="1" max="65535" placeholder="例如: 8080" required>
                        </div>
                    </div>
                    <div class="form-row">
                        <div class="form-group">
                            <label for="protocol">协议</label>
                            <select id="protocol" name="protocol">
                                <option value="TCP">TCP</option>
                                <option value="UDP">UDP</option>
                            </select>
                        </div>
                        <div class="form-group">
                            <label for="description">描述</label>
                            <input type="text" id="description" name="description" placeholder="例如: Web服务器端口">
                        </div>
                    </div>
                </div>
                <div class="modal-footer">
                    <button type="button" class="btn btn-secondary" onclick="closeAddMappingModal()">取消</button>
                    <button type="submit" class="btn">添加映射</button>
                </div>
            </form>
        </div>
    </div>
        </div>
    </div>

    <script>
        // 全局变量
        let refreshInterval;
        let currentTab = 'auto';
        
        // 页面加载完成后初始化
        document.addEventListener('DOMContentLoaded', function() {
            loadStatus();
            loadMappings();
            loadManualMappings();
            loadPorts();
            
            // 设置定时刷新
            refreshInterval = setInterval(function() {
                loadStatus();
                if (currentTab === 'auto') {
                    loadMappings();
                } else if (currentTab === 'manual') {
                    loadManualMappings();
                }
                loadPorts();
            }, 5000); // 每5秒刷新一次
            
            // 绑定表单提交事件
            document.getElementById('addMappingForm').addEventListener('submit', handleAddMapping);
            
            // 绑定弹窗关闭事件
            window.addEventListener('click', function(event) {
                const modal = document.getElementById('addMappingModal');
                if (event.target === modal) {
                    closeAddMappingModal();
                }
            });
            
            // 绑定ESC键关闭弹窗
            document.addEventListener('keydown', function(event) {
                if (event.key === 'Escape') {
                    closeAddMappingModal();
                }
            });
        });
        
        // 切换标签页
        function switchTab(tabName) {
            // 更新按钮状态
            document.querySelectorAll('.tab-button').forEach(btn => {
                btn.classList.remove('active');
            });
            event.target.classList.add('active');
            
            // 更新内容显示
            document.querySelectorAll('.tab-content').forEach(content => {
                content.classList.remove('active');
            });
            document.getElementById(tabName + 'Tab').classList.add('active');
            
            currentTab = tabName;
            
            // 加载对应数据
            if (tabName === 'manual') {
                loadManualMappings();
            } else if (tabName === 'auto') {
                loadMappings();
            }
        }
        
        // 打开添加映射弹窗
        function openAddMappingModal() {
            const modal = document.getElementById('addMappingModal');
            modal.style.display = 'block';
            document.body.style.overflow = 'hidden'; // 防止背景滚动
            
            // 聚焦到第一个输入框
            setTimeout(() => {
                document.getElementById('internalPort').focus();
            }, 100);
        }
        
        // 关闭添加映射弹窗
        function closeAddMappingModal() {
            const modal = document.getElementById('addMappingModal');
            modal.style.display = 'none';
            document.body.style.overflow = ''; // 恢复背景滚动
            
            // 重置表单
            document.getElementById('addMappingForm').reset();
        }
        
        // 加载服务状态
        async function loadStatus() {
            try {
                const response = await fetch('/api/status');
                
                if (!response.ok) {
                    if (response.status === 401) {
                        showMessage('认证失败，请检查用户名和密码', 'error');
                        return;
                    }
                    throw new Error('HTTP ' + response.status + ': ' + response.statusText);
                }
                
                const data = await response.json();
                
                const statusGrid = document.getElementById('statusGrid');
                statusGrid.innerHTML = 
                    '<div class="status-card">' +
                        '<h3>活跃端口</h3>' +
                        '<div class="value">' + (data.port_status?.active_ports || 0) + '</div>' +
                    '</div>' +
                    '<div class="status-card">' +
                        '<h3>总映射数</h3>' +
                        '<div class="value">' + (data.upnp_mappings?.total_mappings || data.total_mappings || 0) + '</div>' +
                    '</div>' +
                    '<div class="status-card">' +
                        '<h3>手动映射</h3>' +
                        '<div class="value">' + (data.manual_mappings?.total_mappings || 0) + '</div>' +
                    '</div>' +
                    '<div class="status-card">' +
                        '<h3>UPnP状态</h3>' +
                        '<div class="value">' + (data.port_mapping_status?.upnp?.available ? '可用' : '不可用') + '</div>' +
                    '</div>' +
                    '<div class="status-card">' +
                        '<h3>NAT穿透</h3>' +
                        '<div class="value">' + (data.port_mapping_status?.turn?.available ? '可用' : '不可用') + '</div>' +
                        (data.port_mapping_status?.turn?.external_address ? 
                            '<div style="font-size: 0.8em; margin-top: 5px; color: #666;">' +
                                (data.port_mapping_status.turn.external_address.ip || data.port_mapping_status.turn.external_address.IP) + ':' + 
                                (data.port_mapping_status.turn.external_address.port || data.port_mapping_status.turn.external_address.Port) +
                            '</div>' : '') +
                    '</div>';
            } catch (error) {
                console.error('加载状态失败:', error);
                const statusGrid = document.getElementById('statusGrid');
                statusGrid.innerHTML = '<div class="error">加载状态失败: ' + error.message + '</div>';
                showMessage('加载状态失败: ' + error.message, 'error');
            }
        }
        
        // 加载手动映射
        async function loadManualMappings() {
            try {
                const response = await fetch('/api/mappings?addType=manual');
                
                if (!response.ok) {
                    if (response.status === 401) {
                        showMessage('认证失败，请检查用户名和密码', 'error');
                        return;
                    }
                    throw new Error('HTTP ' + response.status + ': ' + response.statusText);
                }
                
                const data = await response.json();
                // 更新映射表格
                const mappingsTable = document.getElementById('manualMappingsTable');
                
                // 检查数据是否为数组格式
                const mappings = Array.isArray(data) ? data : [];
                
                if (mappings.length === 0) {
                    mappingsTable.innerHTML = '<p>暂无端口映射</p>';
                    return;
                }
                
                let tableHTML = 
                    '<table class="mappings-table">' +
                        '<thead>' +
                            '<tr>' +
                                '<th class="col-port">内部端口</th>' +
                                '<th class="col-port">外部端口</th>' +
                                '<th class="col-protocol">协议</th>' +
                                '<th class="col-description">描述</th>' +
                                '<th class="col-type">类型</th>' +
                                '<th class="col-status">状态</th>' +
                                '<th class="col-time">创建时间</th>' +
                                '<th class="col-action">操作</th>' +
                            '</tr>' +
                        '</thead>' +
                        '<tbody>';
                
                mappings.forEach(mapping => {
                    const statusClass = mapping.status === 'active' ? 'active' : 'inactive';
                    const statusText = mapping.status === 'active' ? '活跃' : '非活跃';
                    const typeText = mapping.type || '未知';
                    const isTurn = typeText.toLowerCase() === 'turn';
                    
                    let externalPort = mapping.external_port || '-';
                    let showPort = mapping.external_port || '-';
                    
                    if (isTurn && mapping.external_addr) {
                        showPort = mapping.external_addr.IP + ':' + mapping.external_addr.Port;
                    }
                    
                    // 格式化创建时间
                    const createdAt = mapping.created_at ? 
                        new Date(mapping.created_at).toLocaleString('zh-CN') : '-';
                    
                    tableHTML += 
                        '<tr>' +
                            '<td class="col-port">' + (mapping.internal_port || '-') + '</td>' +
                            '<td class="col-port">' + showPort + '</td>' +
                            '<td class="col-protocol">' + (mapping.protocol ? mapping.protocol.toUpperCase() : '-') + '</td>' +
                            '<td class="col-description">' + (mapping.description || '-') + '</td>' +
                            '<td class="col-type"><span class="status-badge">' + typeText + '</span></td>' +
                            '<td class="col-status"><span class="status-badge ' + statusClass + '">' + statusText + '</span></td>' +
                            '<td class="col-time">' + createdAt + '</td>' +
                            '<td class="col-action">' +
                                '<button class="btn btn-danger" onclick="removeMapping(' + (mapping.internal_port || 0) + ', ' + externalPort + ', \'' + (mapping.protocol || 'TCP') + '\')">' +
                                    '删除' +
                                '</button>' +
                            '</td>' +
                        '</tr>';
                });
                
                tableHTML += '</tbody></table>';
                mappingsTable.innerHTML = tableHTML;
            } catch (error) {
                console.error('加载手动映射失败:', error);
                const mappingsTable = document.getElementById('manualMappingsTable');
                mappingsTable.innerHTML = '<div class="error">加载手动映射失败: ' + error.message + '</div>';
                showMessage('加载手动映射失败: ' + error.message, 'error');
            }
        }
        
        // 加载端口映射
        async function loadMappings() {
            try {
                const response = await fetch('/api/mappings?addType=auto');
                
                if (!response.ok) {
                    if (response.status === 401) {
                        showMessage('认证失败，请检查用户名和密码', 'error');
                        return;
                    }
                    throw new Error('HTTP ' + response.status + ': ' + response.statusText);
                }
                
                const data = await response.json();
                
                const mappingsTable = document.getElementById('mappingsTable');
                
                // 检查数据是否为数组格式
                const mappings = Array.isArray(data) ? data : [];
                
                if (mappings.length === 0) {
                    mappingsTable.innerHTML = '<p>暂无端口映射</p>';
                    return;
                }
                
                let tableHTML = 
                    '<table class="mappings-table">' +
                        '<thead>' +
                            '<tr>' +
                                '<th class="col-port">内部端口</th>' +
                                '<th class="col-port">外部端口</th>' +
                                '<th class="col-protocol">协议</th>' +
                                '<th class="col-description">描述</th>' +
                                '<th class="col-type">类型</th>' +
                                '<th class="col-status">状态</th>' +
                                '<th class="col-time">创建时间</th>' +
                                '<th class="col-action">操作</th>' +
                            '</tr>' +
                        '</thead>' +
                        '<tbody>';
                
                mappings.forEach(mapping => {
                    const statusClass = mapping.status === 'active' ? 'active' : 'inactive';
                    const statusText = mapping.status === 'active' ? '活跃' : '非活跃';
                    const typeText = mapping.type || '未知';
                    const isTurn = typeText.toLowerCase() === 'turn';
                    
                    let showPort = mapping.external_port || '-';
                    let externalPort =  mapping.external_port || '-';
                    
                    if (isTurn && mapping.external_addr) {
                        showPort = mapping.external_addr.IP + ':' + mapping.external_addr.Port;
                    } 
                    
                    // 格式化创建时间
                    const createdAt = mapping.created_at ? 
                        new Date(mapping.created_at).toLocaleString('zh-CN') : '-';
                    
                    tableHTML += 
                        '<tr>' +
                            '<td class="col-port">' + (mapping.internal_port || '-') + '</td>' +
                            '<td class="col-port">' + showPort + '</td>' +
                            '<td class="col-protocol">' + (mapping.protocol ? mapping.protocol.toUpperCase() : '-') + '</td>' +
                            '<td class="col-description">' + (mapping.description || '-') + '</td>' +
                            '<td class="col-type"><span class="status-badge">' + typeText + '</span></td>' +
                            '<td class="col-status"><span class="status-badge ' + statusClass + '">' + statusText + '</span></td>' +
                            '<td class="col-time">' + createdAt + '</td>' +
                            '<td class="col-action">' +
                                '<button class="btn btn-danger" onclick="removeMapping(' + (mapping.internal_port || 0) + ', ' + externalPort + ', \'' + (mapping.protocol || 'TCP') + '\')">' +
                                    '删除' +
                                '</button>' +
                            '</td>' +
                        '</tr>';
                });
                
                tableHTML += '</tbody></table>';
                mappingsTable.innerHTML = tableHTML;
            } catch (error) {
                console.error('加载映射失败:', error);
                const mappingsTable = document.getElementById('mappingsTable');
                mappingsTable.innerHTML = '<div class="error">加载映射失败: ' + error.message + '</div>';
                showMessage('加载映射失败: ' + error.message, 'error');
            }
        }
        
        // 加载端口状态
        async function loadPorts() {
            try {
                const response = await fetch('/api/ports');
                
                if (!response.ok) {
                    if (response.status === 401) {
                        showMessage('认证失败，请检查用户名和密码', 'error');
                        return;
                    }
                    throw new Error('HTTP ' + response.status + ': ' + response.statusText);
                }
                
                const data = await response.json();
                
                const portsStatus = document.getElementById('portsStatus');
                
                // 确保数据是数组类型，只获取活跃端口
                const activePorts = Array.isArray(data.active_ports) ? data.active_ports : [];
                
                if (activePorts.length === 0) {
                    portsStatus.innerHTML = '<p>暂无活跃端口</p>';
                    return;
                }
                
                let portsHTML = '<div class="ports-grid">';
                
                // 只显示活跃端口
                activePorts.sort((a, b) => a - b).forEach(port => {
                    portsHTML += '<div class="port-item active">' + port + '</div>';
                });
                
                portsHTML += '</div>';
                portsStatus.innerHTML = portsHTML;
            } catch (error) {
                console.error('加载端口状态失败:', error);
                const portsStatus = document.getElementById('portsStatus');
                portsStatus.innerHTML = '<div class="error">加载端口状态失败: ' + error.message + '</div>';
                showMessage('加载端口状态失败: ' + error.message, 'error');
            }
        }
        
        // 处理添加映射
        async function handleAddMapping(event) {
            event.preventDefault();
            
            const formData = new FormData(event.target);
            const requestData = {
                internal_port: parseInt(formData.get('internal_port')),
                external_port: parseInt(formData.get('external_port')),
                protocol: formData.get('protocol') || 'TCP',
                description: formData.get('description') || ''
            };
            
            // 验证输入
            if (!requestData.internal_port || requestData.internal_port < 1 || requestData.internal_port > 65535) {
                showMessage('内部端口必须是1-65535之间的数字', 'error');
                return;
            }
            
            if (!requestData.external_port || requestData.external_port < 1 || requestData.external_port > 65535) {
                showMessage('外部端口必须是1-65535之间的数字', 'error');
                return;
            }
            
            try {
                const response = await fetch('/api/add-mapping', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify(requestData)
                });
                
                const result = await response.json();
                
                if (response.ok) {
                    showMessage('映射添加成功', 'success');
                    closeAddMappingModal(); // 关闭弹窗
                    loadMappings();
                    loadManualMappings();
                    loadStatus();
                } else {
                    // 处理不同的错误状态
                    let errorMessage = result.message || '添加映射失败';
                    
                    if (response.status === 401) {
                        errorMessage = '认证失败，请检查用户名和密码';
                    } else if (response.status === 400) {
                        errorMessage = result.message || '请求参数错误';
                    } else if (response.status === 500) {
                        errorMessage = result.message || '服务器内部错误';
                    }
                    
                    showMessage(errorMessage, 'error');
                }
            } catch (error) {
                console.error('添加映射失败:', error);
                showMessage('网络错误: ' + error.message, 'error');
            }
        }
        
        // 删除映射
        async function removeMapping(internalPort, externalPort, protocol) {
            if (!confirm('确定要删除这个端口映射吗？')) {
                return;
            }
            
            const requestData = {
                internal_port: parseInt(internalPort),
                external_port: parseInt(externalPort),
                protocol: protocol || 'TCP'
            };
            
            try {
                const response = await fetch('/api/remove-mapping', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify(requestData)
                });
                
                const result = await response.json();
                
                if (response.ok) {
                    showMessage('映射删除成功', 'success');
                    loadMappings();
                    loadManualMappings();
                    loadStatus();
                } else {
                    // 处理不同的错误状态
                    let errorMessage = result.message || '删除映射失败';
                    
                    if (response.status === 401) {
                        errorMessage = '认证失败，请检查用户名和密码';
                    } else if (response.status === 400) {
                        errorMessage = result.message || '请求参数错误';
                    } else if (response.status === 500) {
                        errorMessage = result.message || '服务器内部错误';
                    }
                    
                    showMessage(errorMessage, 'error');
                }
            } catch (error) {
                console.error('删除映射失败:', error);
                showMessage('网络错误: ' + error.message, 'error');
            }
        }
        
        // 显示消息
        function showMessage(message, type) {
            // 移除现有的消息
            const existingMessages = document.querySelectorAll('.message');
            existingMessages.forEach(msg => msg.remove());
            
            const messageDiv = document.createElement('div');
            messageDiv.className = 'message ' + type;
            messageDiv.textContent = message;
            
            const content = document.querySelector('.content');
            content.insertBefore(messageDiv, content.firstChild);
            
            // 自动移除消息
            setTimeout(() => {
                if (messageDiv.parentNode) {
                    messageDiv.remove();
                }
            }, 5000);
        }
        
        // 页面卸载时清理定时器
        window.addEventListener('beforeunload', function() {
            if (refreshInterval) {
                clearInterval(refreshInterval);
            }
        });
    </script>
</body>
</html>`
