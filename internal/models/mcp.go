package models

import (
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// MCPConnection MCP连接信息
type MCPConnection struct {
	ID           uint           `json:"id" gorm:"primaryKey"`
	UserID       uint           `json:"user_id" gorm:"not null;index"`
	User         User           `json:"user" gorm:"foreignKey:UserID"`
	ConnectionID string         `json:"connection_id" gorm:"type:varchar(255);uniqueIndex;not null"` // 唯一连接标识
	Status       string         `json:"status" gorm:"default:'disconnected'"`                        // connected, disconnected, error
	IsActive     bool           `json:"is_active" gorm:"default:true"`                               // 连接是否活跃
	LastActivity time.Time      `json:"last_activity"`                                               // 最后活动时间
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

// MCPRequest MCP请求结构
type MCPRequest struct {
	Method     string      `json:"method"`
	Parameters interface{} `json:"parameters,omitempty"`
	ID         string      `json:"id,omitempty"`
}

// MCPResponse MCP响应结构
type MCPResponse struct {
	Result interface{} `json:"result,omitempty"`
	Error  *MCPError   `json:"error,omitempty"`
	ID     string      `json:"id,omitempty"`
}

// MCPError MCP错误结构
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCPCallLog MCP调用日志记录
type MCPCallLog struct {
	ID           uint           `json:"id" gorm:"primaryKey"`
	UserID       uint           `json:"user_id" gorm:"not null;index"`
	User         User           `json:"user" gorm:"foreignKey:UserID"`
	ProjectID    *uint          `json:"project_id" gorm:"index"`                          // 关联的项目ID（可选）
	Project      *FigmaProject  `json:"project" gorm:"foreignKey:ProjectID"`              // 关联的项目
	ConnectionID string         `json:"connection_id" gorm:"type:varchar(255)"`           // MCP连接ID/Token
	Method       string         `json:"method" gorm:"type:varchar(100);not null"`         // 调用方法
	RequestData  string         `json:"request_data" gorm:"type:longtext"`                // 请求数据JSON
	ResponseData string         `json:"response_data" gorm:"type:longtext"`               // 响应数据JSON
	Status       string         `json:"status" gorm:"type:varchar(20);default:'success'"` // success, error
	ErrorMessage string         `json:"error_message" gorm:"type:text"`                   // 错误信息
	Duration     int64          `json:"duration" gorm:"default:0"`                        // 执行耗时（毫秒）
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

// GetMCPConnectionByUserID 根据用户ID获取MCP连接
func GetMCPConnectionByUserID(userID uint) (*MCPConnection, error) {
	var connection MCPConnection
	err := DB.Where("user_id = ?", userID).First(&connection).Error
	if err != nil {
		return nil, err
	}
	return &connection, nil
}

// CreateOrUpdateMCPConnection 创建或更新MCP连接
func CreateOrUpdateMCPConnection(userID uint, connectionID string) (*MCPConnection, error) {
	// 首先尝试查找现有连接
	var existingConnection MCPConnection
	err := DB.Where("connection_id = ?", connectionID).First(&existingConnection).Error

	if err == nil {
		// 连接已存在，更新现有连接
		existingConnection.Status = "connected"
		existingConnection.IsActive = true
		existingConnection.LastActivity = time.Now()
		err = DB.Save(&existingConnection).Error
		if err != nil {
			return nil, err
		}
		log.Printf("MCP连接已更新: 用户ID=%d, 连接ID=%s", userID, connectionID)
		return &existingConnection, nil
	}

	// 连接不存在，创建新连接
	connection := &MCPConnection{
		UserID:       userID,
		ConnectionID: connectionID,
		Status:       "connected",
		IsActive:     true,
		LastActivity: time.Now(),
	}

	err = DB.Create(connection).Error
	if err != nil {
		return nil, err
	}

	log.Printf("MCP连接已创建: 用户ID=%d, 连接ID=%s", userID, connectionID)
	return connection, nil
}

// CreateMCPConnection 创建MCP连接 (保留向后兼容)
func CreateMCPConnection(userID uint, connectionID string) (*MCPConnection, error) {
	return CreateOrUpdateMCPConnection(userID, connectionID)
}

// UpdateMCPConnectionStatus 更新MCP连接状态
func (c *MCPConnection) UpdateStatus(status string) error {
	c.Status = status
	c.LastActivity = time.Now()
	return DB.Save(c).Error
}

// LogMCPCall 记录MCP调用日志
func LogMCPCall(userID uint, connectionID, method string, parameters interface{}, response interface{}, status, errorMsg string, duration int64) error {
	var parametersJSON string
	var responseJSON string

	if parameters != nil {
		if paramBytes, err := json.Marshal(parameters); err == nil {
			parametersJSON = string(paramBytes)
		}
	}

	if response != nil {
		if respBytes, err := json.Marshal(response); err == nil {
			responseJSON = string(respBytes)
		}
	}

	// 尝试从method中提取项目ID（如果存在）
	var projectID *uint
	if strings.Contains(method, "/") {
		parts := strings.Split(method, "/")
		for i, part := range parts {
			if part == "project" && i+1 < len(parts) {
				if id, err := strconv.ParseUint(parts[i+1], 10, 32); err == nil {
					pid := uint(id)
					projectID = &pid
				}
				break
			}
		}
	}

	// 创建日志记录
	logEntry := &MCPCallLog{
		UserID:       userID,
		ProjectID:    projectID,
		ConnectionID: connectionID,
		Method:       method,
		RequestData:  parametersJSON,
		ResponseData: responseJSON,
		Status:       status,
		ErrorMessage: errorMsg,
		Duration:     duration,
	}

	// 保存到数据库
	err := DB.Create(logEntry).Error
	if err != nil {
		log.Printf("保存MCP调用日志失败: %v", err)
		// 即使数据库保存失败，也要打印日志
	}

	// 同时打印日志到控制台（用于调试）
	log.Printf("MCP调用日志 - 用户ID: %d, 连接ID: %s, 方法: %s, 状态: %s, 耗时: %dms",
		userID, connectionID, method, status, duration)

	if errorMsg != "" {
		log.Printf("MCP调用错误 - %s", errorMsg)
	}

	return err
}

// GetMCPCallLogs 获取MCP调用日志
func GetMCPCallLogs(userID uint, limit int, offset int) ([]MCPCallLog, error) {
	var logs []MCPCallLog

	query := DB.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset)

	err := query.Find(&logs).Error
	if err != nil {
		log.Printf("查询MCP调用日志失败: %v", err)
		return nil, err
	}

	log.Printf("获取MCP调用日志成功 - 用户ID: %d, 返回: %d条", userID, len(logs))
	return logs, nil
}

// GetMCPCallLogsByProject 按项目获取MCP调用日志
func GetMCPCallLogsByProject(userID uint, projectID uint, limit int, offset int) ([]MCPCallLog, error) {
	var logs []MCPCallLog

	query := DB.Where("user_id = ? AND (project_id = ? OR project_id IS NULL)", userID, projectID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset)

	err := query.Find(&logs).Error
	if err != nil {
		log.Printf("按项目查询MCP调用日志失败: %v", err)
		return nil, err
	}

	log.Printf("按项目获取MCP调用日志成功 - 用户ID: %d, 项目ID: %d, 返回: %d条", userID, projectID, len(logs))
	return logs, nil
}

// SaveMCPNodeConfig 保存MCP节点配置（暂时只打印日志）
func SaveMCPNodeConfig(userID uint, projectID, nodeID, configData, configType string) error {
	log.Printf("保存MCP节点配置 - 用户ID: %d, 项目ID: %s, 节点ID: %s, 配置类型: %s",
		userID, projectID, nodeID, configType)
	log.Printf("配置数据: %s", configData)
	return nil
}

// GetMCPNodeConfig 获取MCP节点配置（暂时只打印日志）
func GetMCPNodeConfig(userID uint, projectID, nodeID, configType string) (map[string]interface{}, error) {
	log.Printf("获取MCP节点配置 - 用户ID: %d, 项目ID: %s, 节点ID: %s, 配置类型: %s",
		userID, projectID, nodeID, configType)
	// 暂时返回空配置
	return map[string]interface{}{}, nil
}

// SaveMCPPreviewCache 保存MCP预览图缓存（暂时只打印日志）
func SaveMCPPreviewCache(userID uint, projectID, nodeID, previewURL string, previewData []byte, contentType string, expiresAt time.Time) error {
	log.Printf("保存MCP预览图缓存 - 用户ID: %d, 项目ID: %s, 节点ID: %s, URL: %s, 类型: %s",
		userID, projectID, nodeID, previewURL, contentType)
	return nil
}

// GetMCPPreviewCache 获取MCP预览图缓存（暂时只打印日志）
func GetMCPPreviewCache(userID uint, projectID, nodeID string) ([]byte, string, error) {
	log.Printf("获取MCP预览图缓存 - 用户ID: %d, 项目ID: %s, 节点ID: %s",
		userID, projectID, nodeID)
	// 暂时返回空数据
	return nil, "", nil
}

// ClearMCPCallLogsByProject 清理项目的MCP调用日志
func ClearMCPCallLogsByProject(userID uint, projectID uint) error {
	result := DB.Where("user_id = ? AND (project_id = ? OR project_id IS NULL)", userID, projectID).Delete(&MCPCallLog{})
	if result.Error != nil {
		log.Printf("清理项目MCP调用日志失败: %v", result.Error)
		return result.Error
	}

	log.Printf("清理项目MCP调用日志成功 - 用户ID: %d, 项目ID: %d, 删除: %d条", userID, projectID, result.RowsAffected)
	return nil
}
