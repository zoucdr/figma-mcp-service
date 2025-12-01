package services

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/figma-deliver/internal/models"
	"github.com/gorilla/websocket"
)

// MCPService MCP服务管理器
type MCPService struct {
	connections     map[string]*MCPUserConnection          // connectionID -> connection
	wsConnections   map[string]*WebSocketClient            // connectionID -> websocket client
	channels        map[string]map[string]*WebSocketClient // channelName -> (connectionID -> client)
	pendingRequests map[string]*PendingRequest             // requestID -> pending request
	mutex           sync.RWMutex
}

// PendingRequest 待处理的请求
type PendingRequest struct {
	ResponseChan chan interface{}
	ErrorChan    chan error
	Timestamp    time.Time
}

// WebSocketClient WebSocket客户端
type WebSocketClient struct {
	Conn         *websocket.Conn
	ConnectionID string
	UserID       uint
	Channel      string
	LastActivity time.Time
	Send         chan []byte
}

// WSMessage WebSocket消息结构
type WSMessage struct {
	Type    string           `json:"type"`
	Message interface{}      `json:"message,omitempty"`
	Channel string           `json:"channel,omitempty"`
	ID      string           `json:"id,omitempty"`
	Sender  string           `json:"sender,omitempty"`
	Result  interface{}      `json:"result,omitempty"`
	Error   *models.MCPError `json:"error,omitempty"`
}

// MCPUserConnection 用户MCP连接
type MCPUserConnection struct {
	UserID       uint
	ConnectionID string
	User         *models.User
	LastActivity time.Time
	Status       string
}

// 全局MCP服务实例
var mcpService *MCPService
var mcpOnce sync.Once

// GetMCPService 获取MCP服务单例
func GetMCPService() *MCPService {
	mcpOnce.Do(func() {
		mcpService = &MCPService{
			connections:     make(map[string]*MCPUserConnection),
			wsConnections:   make(map[string]*WebSocketClient),
			channels:        make(map[string]map[string]*WebSocketClient),
			pendingRequests: make(map[string]*PendingRequest),
		}
		// 清理数据库中的孤立连接（服务重启时）
		mcpService.cleanupOrphanedConnections()
		// 启动清理协程
		go mcpService.startCleanupRoutine()
		go mcpService.cleanupExpiredRequests()
	})
	return mcpService
}

// CreateConnection 创建MCP连接
func (s *MCPService) CreateConnection(userID uint, connectionID string) (*MCPUserConnection, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 获取用户信息
	var user models.User
	if err := models.DB.First(&user, userID).Error; err != nil {
		return nil, fmt.Errorf("用户不存在: %v", err)
	}

	// 创建连接
	connection := &MCPUserConnection{
		UserID:       userID,
		ConnectionID: connectionID,
		User:         &user,
		LastActivity: time.Now(),
		Status:       "connected",
	}

	s.connections[connectionID] = connection

	// 保存到数据库
	_, err := models.CreateMCPConnection(userID, connectionID)
	if err != nil {
		delete(s.connections, connectionID)
		return nil, fmt.Errorf("创建MCP连接失败: %v", err)
	}

	log.Printf("MCP连接已创建: 用户ID=%d, 连接ID=%s", userID, connectionID)
	return connection, nil
}

// GetConnection 获取MCP连接
func (s *MCPService) GetConnection(connectionID string) (*MCPUserConnection, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	connection, exists := s.connections[connectionID]
	// 注意：不要在这里更新 LastActivity，因为这是读锁
	// 如果需要更新活动时间，请使用 TouchConnection
	return connection, exists
}

// TouchConnection 更新连接的最后活动时间
func (s *MCPService) TouchConnection(connectionID string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if connection, exists := s.connections[connectionID]; exists {
		connection.LastActivity = time.Now()
	}
}

// GetUserConnection 根据用户ID获取连接
func (s *MCPService) GetUserConnection(userID uint) (*MCPUserConnection, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// 首先检查内存中的连接
	for _, connection := range s.connections {
		if connection.UserID == userID {
			connection.LastActivity = time.Now()
			return connection, true
		}
	}

	// 如果内存中没有找到，检查数据库中的活跃连接
	var dbConnection models.MCPConnection
	err := models.DB.Where("user_id = ? AND status = ?", userID, "connected").First(&dbConnection).Error
	if err == nil {
		// 数据库中有活跃连接，但内存中没有，说明服务重启了
		// 将数据库连接状态设为断开，因为实际连接已丢失
		models.DB.Model(&dbConnection).Update("status", "disconnected")
	}

	return nil, false
}

// RemoveConnection 移除MCP连接
func (s *MCPService) RemoveConnection(connectionID string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if connection, exists := s.connections[connectionID]; exists {
		// 更新数据库状态
		if dbConnection, err := models.GetMCPConnectionByUserID(connection.UserID); err == nil {
			dbConnection.UpdateStatus("disconnected")
		}

		delete(s.connections, connectionID)
		log.Printf("MCP连接已移除: 连接ID=%s", connectionID)
	}
}

// CloseUserConnections 关闭用户的所有旧连接，确保只有一个活跃连接
func (s *MCPService) CloseUserConnections(userID uint, excludeConnectionID string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 关闭内存中该用户的所有其他连接
	for connID, connection := range s.connections {
		if connection.UserID == userID && connID != excludeConnectionID {
			log.Printf("关闭用户 %d 的旧连接: %s", userID, connID)

			// 更新数据库状态
			if dbConnection, err := models.GetMCPConnectionByUserID(userID); err == nil {
				dbConnection.UpdateStatus("disconnected")
			}

			// 关闭 WebSocket 连接
			if wsClient, exists := s.wsConnections[connID]; exists {
				if wsClient.Conn != nil {
					wsClient.Conn.WriteMessage(websocket.CloseMessage,
						websocket.FormatCloseMessage(websocket.CloseNormalClosure, "新连接已建立，关闭旧连接"))
					wsClient.Conn.Close()
				}
				delete(s.wsConnections, connID)

				// 从所有频道中移除
				for _, clients := range s.channels {
					delete(clients, connID)
				}
			}

			// 从连接列表中移除
			delete(s.connections, connID)
		}
	}

	// 更新数据库中的连接状态
	models.DB.Model(&models.MCPConnection{}).
		Where("user_id = ? AND connection_id != ?", userID, excludeConnectionID).
		Update("status", "disconnected")
}

// UpdateConnectionStatus 更新连接状态
func (s *MCPService) UpdateConnectionStatus(connectionID, status string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	connection, exists := s.connections[connectionID]
	if !exists {
		return fmt.Errorf("连接不存在: %s", connectionID)
	}

	connection.Status = status
	connection.LastActivity = time.Now()

	// 更新数据库
	if dbConnection, err := models.GetMCPConnectionByUserID(connection.UserID); err == nil {
		return dbConnection.UpdateStatus(status)
	}

	return nil
}

// GetPreview 获取预览图
func (s *MCPService) GetPreview(connectionID, projectID, nodeID string) (*models.MCPResponse, error) {
	startTime := time.Now()

	connection, exists := s.GetConnection(connectionID)
	if !exists {
		return s.createErrorResponse("连接不存在", -1), nil
	}

	// 记录调用日志
	defer func() {
		duration := time.Since(startTime).Milliseconds()
		params := map[string]string{"project_id": projectID, "node_id": nodeID}
		models.LogMCPCall(connection.UserID, connectionID, "get_preview", params, nil, "success", "", duration)
	}()

	// 检查缓存（暂时跳过缓存功能）
	previewData, contentType, err := models.GetMCPPreviewCache(connection.UserID, projectID, nodeID)
	if err == nil && previewData != nil {
		log.Printf("MCP预览图缓存命中: 项目ID=%s, 节点ID=%s, 类型=%s", projectID, nodeID, contentType)
		return &models.MCPResponse{
			Result: map[string]interface{}{
				"preview_url":  "",
				"content_type": contentType,
				"cached":       true,
			},
		}, nil
	}

	// 模拟获取预览图
	log.Printf("MCP获取预览图: 用户ID=%d, 项目ID=%s, 节点ID=%s", connection.UserID, projectID, nodeID)

	// 这里应该调用实际的Figma API获取预览图
	// 目前返回模拟数据
	previewURL := fmt.Sprintf("/figma/image/%s/%s", projectID, nodeID)

	return &models.MCPResponse{
		Result: map[string]interface{}{
			"preview_url":  previewURL,
			"content_type": "image/png",
			"cached":       false,
			"message":      "预览图获取成功（模拟数据）",
		},
	}, nil
}

// GetNodeConfig 获取节点有效配置信息
func (s *MCPService) GetNodeConfig(connectionID, projectID, nodeID string) (*models.MCPResponse, error) {
	startTime := time.Now()

	connection, exists := s.GetConnection(connectionID)
	if !exists {
		return s.createErrorResponse("连接不存在", -1), nil
	}

	// 记录调用日志
	defer func() {
		duration := time.Since(startTime).Milliseconds()
		params := map[string]string{"project_id": projectID, "node_id": nodeID}
		models.LogMCPCall(connection.UserID, connectionID, "get_node_tree", params, nil, "success", "", duration)
	}()

	log.Printf("MCP获取节点配置: 用户ID=%d, 项目ID=%s, 节点ID=%s", connection.UserID, projectID, nodeID)

	// 从数据库获取配置（暂时返回默认配置）
	config, err := models.GetMCPNodeConfig(connection.UserID, projectID, nodeID, "default")
	if err != nil || len(config) == 0 {
		// 返回默认配置
		return &models.MCPResponse{
			Result: map[string]interface{}{
				"config": map[string]interface{}{
					"width":  100,
					"height": 100,
					"x":      0,
					"y":      0,
				},
				"message": "使用默认配置（模拟数据）",
			},
		}, nil
	}

	// 直接使用返回的配置数据
	return &models.MCPResponse{
		Result: map[string]interface{}{
			"config":  config,
			"message": "节点配置获取成功",
		},
	}, nil
}

// GetConfigPrompt 获取配置方案提示词
func (s *MCPService) GetConfigPrompt(connectionID, projectID, nodeID, configType string) (*models.MCPResponse, error) {
	startTime := time.Now()

	connection, exists := s.GetConnection(connectionID)
	if !exists {
		return s.createErrorResponse("连接不存在", -1), nil
	}

	// 记录调用日志
	defer func() {
		duration := time.Since(startTime).Milliseconds()
		params := map[string]interface{}{
			"project_id":  projectID,
			"node_id":     nodeID,
			"config_type": configType,
		}
		models.LogMCPCall(connection.UserID, connectionID, "get_modify_prompt", params, nil, "success", "", duration)
	}()

	log.Printf("MCP获取配置提示词: 用户ID=%d, 项目ID=%s, 节点ID=%s, 配置类型=%s",
		connection.UserID, projectID, nodeID, configType)

	// 根据配置类型返回不同的提示词
	prompts := map[string]string{
		"layout":    "请配置布局参数，包括位置、大小、对齐方式等。支持的参数：x, y, width, height, align",
		"style":     "请配置样式参数，包括颜色、字体、边框等。支持的参数：color, font-size, border-radius, background",
		"animation": "请配置动画参数，包括动画类型、持续时间、缓动函数等。支持的参数：type, duration, easing, delay",
		"default":   "请配置节点参数。支持的基础参数：width, height, x, y",
	}

	prompt, exists := prompts[configType]
	if !exists {
		prompt = prompts["default"]
	}

	return &models.MCPResponse{
		Result: map[string]interface{}{
			"prompt":      prompt,
			"config_type": configType,
			"examples": map[string]interface{}{
				"layout": map[string]interface{}{
					"x":      10,
					"y":      20,
					"width":  200,
					"height": 100,
					"align":  "center",
				},
				"style": map[string]interface{}{
					"color":         "#FF0000",
					"font-size":     "16px",
					"border-radius": "8px",
					"background":    "#FFFFFF",
				},
			},
			"message": "配置提示词获取成功（模拟数据）",
		},
	}, nil
}

// SetNodeConfig 配置节点参数
func (s *MCPService) SetNodeConfig(connectionID, projectID, nodeID string, config map[string]interface{}) (*models.MCPResponse, error) {
	startTime := time.Now()

	connection, exists := s.GetConnection(connectionID)
	if !exists {
		return s.createErrorResponse("连接不存在", -1), nil
	}

	// 记录调用日志
	defer func() {
		duration := time.Since(startTime).Milliseconds()
		params := map[string]interface{}{
			"project_id": projectID,
			"node_id":    nodeID,
			"config":     config,
		}
		models.LogMCPCall(connection.UserID, connectionID, "set_nodes_modify", params, nil, "success", "", duration)
	}()

	log.Printf("MCP配置节点参数: 用户ID=%d, 项目ID=%s, 节点ID=%s", connection.UserID, projectID, nodeID)

	// 将配置保存到数据库
	configJSON, err := json.Marshal(config)
	if err != nil {
		return s.createErrorResponse("配置数据序列化失败", -2), nil
	}

	err = models.SaveMCPNodeConfig(connection.UserID, projectID, nodeID, string(configJSON), "default")
	if err != nil {
		return s.createErrorResponse("配置保存失败", -3), nil
	}

	return &models.MCPResponse{
		Result: map[string]interface{}{
			"success": true,
			"config":  config,
			"message": "节点配置保存成功",
		},
	}, nil
}

// GetConnectionStatus 获取连接状态
func (s *MCPService) GetConnectionStatus(connectionID string) (*models.MCPResponse, error) {
	connection, exists := s.GetConnection(connectionID)
	if !exists {
		return &models.MCPResponse{
			Result: map[string]interface{}{
				"status":    "disconnected",
				"connected": false,
				"message":   "连接不存在",
			},
		}, nil
	}

	return &models.MCPResponse{
		Result: map[string]interface{}{
			"status":        connection.Status,
			"connected":     true,
			"user_id":       connection.UserID,
			"connection_id": connection.ConnectionID,
			"last_activity": connection.LastActivity,
			"message":       "连接状态正常",
		},
	}, nil
}

// ListConnections 列出所有活跃连接
func (s *MCPService) ListConnections() map[string]*MCPUserConnection {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	result := make(map[string]*MCPUserConnection)
	for k, v := range s.connections {
		result[k] = v
	}
	return result
}

// createErrorResponse 创建错误响应
func (s *MCPService) createErrorResponse(message string, code int) *models.MCPResponse {
	return &models.MCPResponse{
		Error: &models.MCPError{
			Code:    code,
			Message: message,
		},
	}
}

// startCleanupRoutine 启动清理协程，定期清理过期连接
func (s *MCPService) startCleanupRoutine() {
	ticker := time.NewTicker(5 * time.Minute) // 每5分钟清理一次
	defer ticker.Stop()

	for range ticker.C {
		s.cleanupExpiredConnections()
	}
}

// cleanupExpiredConnections 清理过期连接
func (s *MCPService) cleanupExpiredConnections() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	now := time.Now()
	expiredConnections := make([]string, 0)

	for connectionID, connection := range s.connections {
		// 如果连接超过30分钟没有活动，则认为过期
		if now.Sub(connection.LastActivity) > 30*time.Minute {
			expiredConnections = append(expiredConnections, connectionID)
		}
	}

	for _, connectionID := range expiredConnections {
		if connection, exists := s.connections[connectionID]; exists {
			// 更新数据库状态
			if dbConnection, err := models.GetMCPConnectionByUserID(connection.UserID); err == nil {
				dbConnection.UpdateStatus("expired")
			}
			delete(s.connections, connectionID)
			log.Printf("MCP连接已过期并清理: 连接ID=%s", connectionID)
		}
	}

	if len(expiredConnections) > 0 {
		log.Printf("清理了 %d 个过期的MCP连接", len(expiredConnections))
	}
}

// cleanupOrphanedConnections 清理数据库中的孤立连接
func (s *MCPService) cleanupOrphanedConnections() {
	// 将所有数据库中状态为"connected"的连接设为"disconnected"
	// 因为服务重启后，内存中的连接都丢失了
	result := models.DB.Model(&models.MCPConnection{}).Where("status = ?", "connected").Update("status", "disconnected")
	if result.Error == nil && result.RowsAffected > 0 {
		log.Printf("服务启动时清理了 %d 个孤立的MCP连接", result.RowsAffected)
	}
}

// ========== WebSocket 相关方法 ==========

// HandleWebSocket 处理WebSocket连接
func (s *MCPService) HandleWebSocket(conn *websocket.Conn, connectionID string, userID uint) {
	client := &WebSocketClient{
		Conn:         conn,
		ConnectionID: connectionID,
		UserID:       userID,
		LastActivity: time.Now(),
		Send:         make(chan []byte, 256),
	}

	s.mutex.Lock()
	s.wsConnections[connectionID] = client
	s.mutex.Unlock()

	log.Printf("WebSocket连接已建立: 用户ID=%d, 连接ID=%s", userID, connectionID)

	// 发送欢迎消息
	welcomeMsg := WSMessage{
		Type:    "system",
		Message: "WebSocket连接成功，请加入频道开始通信",
	}
	client.SendJSON(welcomeMsg)

	// 启动读写协程
	go client.writePump()
	go client.readPump(s)
}

// JoinChannel 加入频道
func (s *MCPService) JoinChannel(connectionID, channelName string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	client, exists := s.wsConnections[connectionID]
	if !exists {
		return fmt.Errorf("WebSocket连接不存在")
	}

	// 如果已经在其他频道，先离开
	if client.Channel != "" {
		s.leaveChannelLocked(connectionID)
	}

	// 创建频道（如果不存在）
	if _, exists := s.channels[channelName]; !exists {
		s.channels[channelName] = make(map[string]*WebSocketClient)
	}

	// 加入频道
	s.channels[channelName][connectionID] = client
	client.Channel = channelName

	log.Printf("客户端加入频道: 连接ID=%s, 频道=%s, 频道人数=%d",
		connectionID, channelName, len(s.channels[channelName]))

	return nil
}

// leaveChannelLocked 离开频道（内部方法，需要已持有锁）
func (s *MCPService) leaveChannelLocked(connectionID string) {
	client, exists := s.wsConnections[connectionID]
	if !exists || client.Channel == "" {
		return
	}

	channelName := client.Channel
	if channelClients, exists := s.channels[channelName]; exists {
		delete(channelClients, connectionID)

		// 通知频道内其他客户端
		msg := WSMessage{
			Type:    "system",
			Message: "有用户离开了频道",
			Channel: channelName,
		}
		for _, c := range channelClients {
			c.SendJSON(msg)
		}

		// 如果频道为空，删除频道
		if len(channelClients) == 0 {
			delete(s.channels, channelName)
			log.Printf("频道已清空并删除: %s", channelName)
		}
	}

	client.Channel = ""
}

// BroadcastToChannel 向频道内其他客户端广播消息
func (s *MCPService) BroadcastToChannel(connectionID, channelName string, message interface{}) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	_, exists := s.wsConnections[connectionID]
	if !exists {
		return fmt.Errorf("WebSocket连接不存在")
	}

	channelClients, exists := s.channels[channelName]
	if !exists || !channelClientExists(channelClients, connectionID) {
		return fmt.Errorf("必须先加入频道")
	}

	// 广播给频道内所有客户端
	// 修改原因：MCP工具调用本身可能不是一个WebSocket连接，而是HTTP请求。
	// 当Figma插件连接到WS时，它加入了频道。
	// MCP工具（HTTP）通过 BroadcastToChannel 发送消息给 Figma 插件。
	// 之前的逻辑排除了 connectionID（发送者），但这在 MCP 场景下是不必要的，
	// 因为 HTTP 请求的 connectionID 实际上是用来标识"用户"的，不对应一个 WS 连接（或者是一个临时的）。
	// 关键是找到 Figma 插件的 WS 连接并发送。
	// 简单起见，我们向频道内所有连接广播，客户端自己决定是否处理。
	broadcastCount := 0
	msg := WSMessage{
		Type:    "broadcast",
		Message: message,
		Sender:  "peer",
		Channel: channelName,
	}

	for _, c := range channelClients {
		// 这里不排除发送者，因为 MCP 工具通常是通过 HTTP 触发的，connectionID 可能复用
		// 即使 connectionID 相同，如果 socket 不同（虽然这里用 connectionID 做 key），
		// 主要问题是 HTTP 调用者并没有 WS 连接，或者我们希望 Figma 插件（即使 ID 相同）能收到。
		// 但根据 map 结构，一个 connectionID 对应一个 client。
		// 如果 Figma 插件使用了 connectionID="my-token"，而 MCP 工具也用 connectionID="my-token"，
		// 那么它们在 map 中会冲突（后来的覆盖前者）。
		// 通常 Figma 插件保持长连接，而 MCP 工具是短 HTTP 请求（无 WS）。
		// MCP 工具调用 ForwardToFigmaPlugin 时，它查找的是 *Figma 插件* 的连接吗？
		// 不，ForwardToFigmaPlugin 逻辑是：
		// 1. 检查 connectionID 是否有活跃 WS 连接。如果有，说明这个 ID 是 Figma 插件的。
		// 2. 它调用 BroadcastToChannel(connectionID, channel, msg)。
		// 3. 如果排除 connectionID，那 Figma 插件自己就收不到消息了！
		//
		// 所以，必须允许发给自己（如果 connectionID 就是接收者）。

		c.SendJSON(msg)
		broadcastCount++
	}

	if broadcastCount == 0 {
		log.Printf("⚠️ 频道 \"%s\" 中没有其他客户端接收消息", channelName)
	} else {
		log.Printf("✓ 向频道 \"%s\" 中的 %d 个客户端广播消息", channelName, broadcastCount)
	}

	return nil
}

// SendToConnection 向指定连接发送消息
func (s *MCPService) SendToConnection(connectionID string, message interface{}) error {
	s.mutex.RLock()
	client, exists := s.wsConnections[connectionID]
	s.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("WebSocket连接不存在")
	}

	return client.SendJSON(message)
}

// SendAndWaitForResponse 发送请求并等待响应（带超时）
func (s *MCPService) SendAndWaitForResponse(connectionID string, message WSMessage, timeout time.Duration) (interface{}, error) {
	// 创建响应通道
	responseChan := make(chan interface{}, 1)
	errorChan := make(chan error, 1)

	// 注册待处理请求
	pendingReq := &PendingRequest{
		ResponseChan: responseChan,
		ErrorChan:    errorChan,
		Timestamp:    time.Now(),
	}

	s.mutex.Lock()
	s.pendingRequests[message.ID] = pendingReq
	s.mutex.Unlock()

	// 清理函数
	defer func() {
		s.mutex.Lock()
		delete(s.pendingRequests, message.ID)
		s.mutex.Unlock()
		close(responseChan)
		close(errorChan)
	}()

	// 发送消息
	if err := s.SendToConnection(connectionID, message); err != nil {
		return nil, fmt.Errorf("发送消息失败: %v", err)
	}

	log.Printf("✅ [SendAndWaitForResponse] 已发送请求 (ID=%s)，等待响应...", message.ID)

	// 等待响应或超时
	select {
	case response := <-responseChan:
		log.Printf("✅ [SendAndWaitForResponse] 收到响应 (ID=%s)", message.ID)
		return response, nil
	case err := <-errorChan:
		log.Printf("❌ [SendAndWaitForResponse] 收到错误响应 (ID=%s): %v", message.ID, err)
		return nil, err
	case <-time.After(timeout):
		log.Printf("⏰ [SendAndWaitForResponse] 请求超时 (ID=%s, timeout=%v)", message.ID, timeout)
		return nil, fmt.Errorf("请求超时，未在 %v 内收到插件响应", timeout)
	}
}

// RemoveWebSocketConnection 移除WebSocket连接
func (s *MCPService) RemoveWebSocketConnection(connectionID string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	client, exists := s.wsConnections[connectionID]
	if !exists {
		return
	}

	// 离开频道
	if client.Channel != "" {
		s.leaveChannelLocked(connectionID)
	}

	// 关闭发送通道
	close(client.Send)

	// 删除连接
	delete(s.wsConnections, connectionID)

	log.Printf("WebSocket连接已移除: 连接ID=%s", connectionID)
}

// channelClientExists 检查客户端是否在频道中
func channelClientExists(clients map[string]*WebSocketClient, connectionID string) bool {
	_, exists := clients[connectionID]
	return exists
}

// ========== WebSocketClient 方法 ==========

// SendJSON 发送JSON消息
func (c *WebSocketClient) SendJSON(msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	select {
	case c.Send <- data:
		return nil
	default:
		return fmt.Errorf("发送通道已满")
	}
}

// readPump 读取消息
func (c *WebSocketClient) readPump(service *MCPService) {
	defer func() {
		service.RemoveWebSocketConnection(c.ConnectionID)
		c.Conn.Close()
	}()

	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket读取错误: %v", err)
			}
			break
		}

		c.LastActivity = time.Now()
		// 同时更新逻辑连接的活动时间
		service.TouchConnection(c.ConnectionID)

		var msg WSMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("消息解析失败: %v", err)
			continue
		}

		log.Printf("\n=== 收到客户端消息 ===")
		log.Printf("类型: %s, 频道: %s", msg.Type, msg.Channel)
		
		// 限制日志输出长度，避免过长的消息
		msgStr := string(message)
		maxLogLength := 500
		if len(msgStr) > maxLogLength {
			log.Printf("消息内容 (前%d字符): %s...", maxLogLength, msgStr[:maxLogLength])
			log.Printf("消息总长度: %d 字节", len(msgStr))
		} else {
			log.Printf("完整消息: %s", msgStr)
		}

		// 处理不同类型的消息
		switch msg.Type {
		case "join":
			if msg.Channel == "" {
				c.SendJSON(WSMessage{
					Type:    "error",
					Message: "频道名称不能为空",
				})
				continue
			}

			err := service.JoinChannel(c.ConnectionID, msg.Channel)
			if err != nil {
				c.SendJSON(WSMessage{
					Type:    "error",
					Message: err.Error(),
				})
				continue
			}

			// 通知加入成功
			c.SendJSON(WSMessage{
				Type:    "system",
				Message: fmt.Sprintf("已加入频道: %s", msg.Channel),
				Channel: msg.Channel,
			})

			c.SendJSON(WSMessage{
				Type: "system",
				Message: map[string]interface{}{
					"id":     msg.ID,
					"result": "Connected to channel: " + msg.Channel,
				},
				Channel: msg.Channel,
			})

			// 通知频道内其他客户端
			service.mutex.RLock()
			if channelClients, exists := service.channels[msg.Channel]; exists {
				for cid, client := range channelClients {
					if cid != c.ConnectionID {
						client.SendJSON(WSMessage{
							Type:    "system",
							Message: "有新用户加入频道",
							Channel: msg.Channel,
						})
					}
				}
			}
			service.mutex.RUnlock()

		case "message":
			if msg.Channel == "" {
				c.SendJSON(WSMessage{
					Type:    "error",
					Message: "频道名称不能为空",
				})
				continue
			}

			// 检查这是否是一个响应消息
			// 如果消息包含 id 且包含 result 或 error，则尝试作为响应处理
			// 这是因为插件返回的消息类型是 "message" 而不是 "broadcast"
			if msgMap, ok := msg.Message.(map[string]interface{}); ok {
				if _, hasID := msgMap["id"]; hasID {
					if _, hasResult := msgMap["result"]; hasResult {
						// 尝试处理为响应
						// 注意：我们需要重新序列化消息以匹配 HandleFigmaPluginResponse 的输入
						// 这里直接调用处理逻辑可能更高效，但为了复用 HandleFigmaPluginResponse，我们传递原始消息
						if err := service.HandleFigmaPluginResponse(message); err == nil {
							// 成功处理为响应，说明是回复给 MCP 工具的（PendingRequest），
							// 此时不应再广播给频道内其他客户端，因为这是单向的请求-响应流程。
							continue
						}
					}
				}
			}

			// 只有非响应消息（即真正的广播消息或新请求）才广播
			err := service.BroadcastToChannel(c.ConnectionID, msg.Channel, msg.Message)
			if err != nil {
				c.SendJSON(WSMessage{
					Type:    "error",
					Message: err.Error(),
				})
			}

		case "broadcast":
			// 处理来自Figma插件的响应
			if err := service.HandleFigmaPluginResponse(message); err != nil {
				log.Printf("处理Figma响应失败: %v", err)
			}

		case "progress_update":
			// 处理进度更新
			if err := service.HandleFigmaPluginResponse(message); err != nil {
				log.Printf("处理进度更新失败: %v", err)
			}

		default:
			log.Printf("未知消息类型: %s", msg.Type)
		}
	}
}

// writePump 发送消息
func (c *WebSocketClient) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// RegisterPendingRequest 注册待处理请求
func (s *MCPService) RegisterPendingRequest(requestID string, responseChan chan interface{}, errorChan chan error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.pendingRequests[requestID] = &PendingRequest{
		ResponseChan: responseChan,
		ErrorChan:    errorChan,
		Timestamp:    time.Now(),
	}
}

// UnregisterPendingRequest 取消注册待处理请求
func (s *MCPService) UnregisterPendingRequest(requestID string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	delete(s.pendingRequests, requestID)
}

// RespondToPendingRequest 响应待处理请求
func (s *MCPService) RespondToPendingRequest(requestID string, response interface{}, err error) bool {
	s.mutex.RLock()
	pending, exists := s.pendingRequests[requestID]
	s.mutex.RUnlock()

	if !exists {
		return false
	}

	if err != nil {
		select {
		case pending.ErrorChan <- err:
		default:
		}
	} else {
		select {
		case pending.ResponseChan <- response:
		default:
		}
	}

	return true
}

// HasActiveWebSocketConnection 检查用户是否有活跃的WebSocket连接
func (s *MCPService) HasActiveWebSocketConnection(connectionID string) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	client, exists := s.wsConnections[connectionID]
	if !exists {
		return false
	}

	// 检查连接是否仍然活跃（5分钟内有活动）
	return time.Since(client.LastActivity) < 5*time.Minute
}

// GetUserChannel 获取用户当前所在频道
func (s *MCPService) GetUserChannel(connectionID string) string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if client, exists := s.wsConnections[connectionID]; exists {
		return client.Channel
	}
	return ""
}

// cleanupExpiredRequests 清理过期的请求
func (s *MCPService) cleanupExpiredRequests() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mutex.Lock()
		now := time.Now()
		for requestID, pending := range s.pendingRequests {
			// 清理超过5分钟的请求
			if now.Sub(pending.Timestamp) > 5*time.Minute {
				select {
				case pending.ErrorChan <- fmt.Errorf("请求已过期"):
				default:
				}
				delete(s.pendingRequests, requestID)
			}
		}
		s.mutex.Unlock()
	}
}
