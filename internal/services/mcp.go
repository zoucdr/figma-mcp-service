package services

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/figma-bridge/internal/models"
)

// MCPService MCP服务管理器
type MCPService struct {
	connections map[string]*MCPUserConnection // connectionID -> connection
	mutex       sync.RWMutex
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
			connections: make(map[string]*MCPUserConnection),
		}
		// 清理数据库中的孤立连接（服务重启时）
		mcpService.cleanupOrphanedConnections()
		// 启动清理协程
		go mcpService.startCleanupRoutine()
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
	if exists {
		connection.LastActivity = time.Now()
	}
	return connection, exists
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
