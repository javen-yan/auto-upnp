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
        }
        
        .mappings-table th,
        .mappings-table td {
            padding: 15px;
            text-align: left;
            border-bottom: 1px solid #eee;
        }
        
        .mappings-table th {
            background: #4facfe;
            color: white;
            font-weight: 600;
        }
        
        .mappings-table tr:hover {
            background: #f8f9fa;
        }
        
        .btn {
            background: #4facfe;
            color: white;
            border: none;
            padding: 10px 20px;
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
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Auto UPnP 管理界面</h1>
            <p>自动端口映射管理服务</p>
        </div>
        
        <div class="content">
            <!-- 服务状态 -->
            <div class="section">
                <h2>服务状态</h2>
                <div class="status-grid" id="statusGrid">
                    <div class="loading">加载中...</div>
                </div>
            </div>
            
            <!-- 端口映射管理 -->
            <div class="section">
                <h2>端口映射管理</h2>
                <div id="mappingsTable">
                    <div class="loading">加载中...</div>
                </div>
            </div>
            
            <!-- 端口状态 -->
            <div class="section">
                <h2>活跃端口监控</h2>
                <div id="portsStatus">
                    <div class="loading">加载中...</div>
                </div>
            </div>

            <!-- 添加映射 -->
            <div class="section">
                <h2>添加端口映射</h2>
                <form id="addMappingForm">
                    <div class="form-row">
                        <div class="form-group">
                            <label for="internalPort">内部端口</label>
                            <input type="number" id="internalPort" name="internal_port" min="1" max="65535" required>
                        </div>
                        <div class="form-group">
                            <label for="externalPort">外部端口</label>
                            <input type="number" id="externalPort" name="external_port" min="1" max="65535" required>
                        </div>
                        <div class="form-group">
                            <label for="protocol">协议</label>
                            <select id="protocol" name="protocol">
                                <option value="TCP">TCP</option>
                                <option value="UDP">UDP</option>
                            </select>
                        </div>
                        <div class="form-group">
                            <label for="description">描述</label>
                            <input type="text" id="description" name="description" placeholder="可选">
                        </div>
                    </div>
                    <button type="submit" class="btn">添加映射</button>
                </form>
            </div>
        </div>
    </div>

    <script>
        // 全局变量
        let refreshInterval;
        
        // 页面加载完成后初始化
        document.addEventListener('DOMContentLoaded', function() {
            loadStatus();
            loadMappings();
            loadPorts();
            
            // 设置定时刷新
            refreshInterval = setInterval(function() {
                loadStatus();
                loadMappings();
                loadPorts();
            }, 5000); // 每5秒刷新一次
            
            // 绑定表单提交事件
            document.getElementById('addMappingForm').addEventListener('submit', handleAddMapping);
        });
        
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
                        '<div class="value">' + (data.upnp_mappings?.total_mappings || 0) + '</div>' +
                    '</div>' +
                    '<div class="status-card">' +
                        '<h3>手动映射</h3>' +
                        '<div class="value">' + (data.manual_mappings?.total_mappings || 0) + '</div>' +
                    '</div>' +
                    '<div class="status-card">' +
                        '<h3>UPnP状态</h3>' +
                        '<div class="value">' + (data.upnp_status?.available ? '可用' : '不可用') + '</div>' +
                    '</div>' +
                    '<div class="status-card">' +
                        '<h3>UPnP客户端</h3>' +
                        '<div class="value">' + (data.upnp_status?.client_count || 0) + '</div>' +
                    '</div>';
            } catch (error) {
                console.error('加载状态失败:', error);
                const statusGrid = document.getElementById('statusGrid');
                statusGrid.innerHTML = '<div class="error">加载状态失败: ' + error.message + '</div>';
                showMessage('加载状态失败: ' + error.message, 'error');
            }
        }
        
        // 加载端口映射
        async function loadMappings() {
            try {
                const response = await fetch('/api/mappings');
                
                if (!response.ok) {
                    if (response.status === 401) {
                        showMessage('认证失败，请检查用户名和密码', 'error');
                        return;
                    }
                    throw new Error('HTTP ' + response.status + ': ' + response.statusText);
                }
                
                const mappings = await response.json();
                
                const mappingsTable = document.getElementById('mappingsTable');
                
                if (!mappings || Object.keys(mappings).length === 0) {
                    mappingsTable.innerHTML = '<p>暂无端口映射</p>';
                    return;
                }
                
                let tableHTML = 
                    '<table class="mappings-table">' +
                        '<thead>' +
                            '<tr>' +
                                '<th>内部端口</th>' +
                                '<th>外部端口</th>' +
                                '<th>协议</th>' +
                                '<th>描述</th>' +
                                '<th>状态</th>' +
                                '<th>操作</th>' +
                            '</tr>' +
                        '</thead>' +
                        '<tbody>';
                
                for (const [key, mapping] of Object.entries(mappings)) {
                    if (mapping && typeof mapping === 'object') {
                        tableHTML += 
                            '<tr>' +
                                '<td>' + (mapping.InternalPort || '-') + '</td>' +
                                '<td>' + (mapping.ExternalPort || '-') + '</td>' +
                                '<td>' + (mapping.Protocol || '-') + '</td>' +
                                '<td>' + (mapping.Description || '-') + '</td>' +
                                '<td>' + (mapping.Active ? '活跃' : '非活跃') + '</td>' +
                                '<td>' +
                                    '<button class="btn btn-danger" onclick="removeMapping(' + (mapping.InternalPort || 0) + ', ' + (mapping.ExternalPort || 0) + ', \'' + (mapping.Protocol || 'TCP') + '\')">' +
                                        '删除' +
                                    '</button>' +
                                '</td>' +
                            '</tr>';
                    }
                }
                
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
                    event.target.reset();
                    loadMappings();
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
