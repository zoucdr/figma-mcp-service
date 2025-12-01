package controllers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/figma-deliver/internal/models"
	"github.com/figma-deliver/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// FigmaToolDefinition Figma工具定义
type FigmaToolDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

// GetBasicTools 获取基础工具列表（不需要WebSocket连接）
func GetBasicTools() []gin.H {
	return []gin.H{
		{
			"name":        "get_preview",
			"description": "获取Figma节点的预览图",
			"inputSchema": gin.H{
				"type": "object",
				"properties": gin.H{
					"project_id":   gin.H{"type": "number", "description": "项目ID"},
					"node_id":      gin.H{"type": "string", "description": "节点ID（可选，默认使用项目根节点）"},
					"format":       gin.H{"type": "string", "description": "图片格式", "enum": []string{"png", "jpg", "svg"}, "default": "png"},
					"scale":        gin.H{"type": "number", "description": "缩放比例 (0.1-4.0)", "minimum": 0.1, "maximum": 4.0, "default": 0.1},
					"ignore_texts": gin.H{"type": "boolean", "description": "是否忽略文本节点（不渲染文本）", "default": false},
				},
				"required": []string{"project_id"},
			},
		},
		{
			"name":        "get_node_tree",
			"description": "获取Figma节点的配置信息",
			"inputSchema": gin.H{
				"type": "object",
				"properties": gin.H{
					"project_id": gin.H{"type": "number", "description": "项目ID"},
					"node_id":    gin.H{"type": "string", "description": "节点ID（可选，默认使用项目根节点）"},
				},
				"required": []string{"project_id"},
			},
		},
		{
			"name":        "set_nodes_modify",
			"description": "批量设置Figma节点的配置信息",
			"inputSchema": gin.H{
				"type": "object",
				"properties": gin.H{
					"project_id": gin.H{"type": "number", "description": "项目ID"},
					"modifys": gin.H{
						"type":        "array",
						"description": "节点修改列表",
						"items": gin.H{
							"type": "object",
							"properties": gin.H{
								"id":         gin.H{"type": "string", "description": "节点ID"},
								"rename":     gin.H{"type": "string", "description": "自定义名称"},
								"ignore":     gin.H{"type": "boolean", "description": "是否忽略此节点"},
								"horizontal": gin.H{"type": "string", "description": "水平约束", "enum": []string{"LEFT", "RIGHT", "CENTER", "LEFT_RIGHT", "SCALE"}},
								"vertical":   gin.H{"type": "string", "description": "垂直约束", "enum": []string{"TOP", "BOTTOM", "CENTER", "TOP_BOTTOM", "SCALE"}},
								"parent_id":  gin.H{"type": "string", "description": "父级节点ID"},
								"res_mode":   gin.H{"type": "string", "description": "资源模式", "enum": []string{"attach", "sprite", "slice", "texture", "cutout"}},
								"img_name":   gin.H{"type": "string", "description": "图片名称"},
								"img_id":     gin.H{"type": "string", "description": "图片ID"},
								"img_ext":    gin.H{"type": "string", "description": "图片格式", "enum": []string{"png", "jpg", "svg"}},
								"components": gin.H{"type": "array", "items": gin.H{"type": "string"}, "description": "组件列表"},
							},
							"required": []string{"id"},
						},
					},
				},
				"required": []string{"project_id", "modifys"},
			},
		},
		{
			"name":        "get_modify_prompt",
			"description": "获取节点配置的提示词",
			"inputSchema": gin.H{
				"type": "object",
				"properties": gin.H{
					"project_id": gin.H{"type": "number", "description": "项目ID"},
					"node_id":    gin.H{"type": "string", "description": "节点ID（可选，默认使用项目根节点）"},
				},
				"required": []string{"project_id"},
			},
		},
		{
			"name":        "clear_nodes_modifys",
			"description": "清理节点修改列表",
			"inputSchema": gin.H{
				"type": "object",
				"properties": gin.H{
					"project_id": gin.H{"type": "number", "description": "项目ID"},
					"node_ids": gin.H{
						"type":        "array",
						"description": "需要清理的节点ID列表",
						"items":       gin.H{"type": "string"},
					},
				},
				"required": []string{"project_id", "node_ids"},
			},
		},
		{
			"name":        "get_ref_nodes",
			"description": "获取项目中的依赖节点ID列表",
			"inputSchema": gin.H{
				"type": "object",
				"properties": gin.H{
					"project_id": gin.H{"type": "number", "description": "项目ID"},
				},
				"required": []string{"project_id"},
			},
		},
	}
}

// GetFigmaPluginTools 获取Figma插件相关工具（需要WebSocket连接）
func GetFigmaPluginTools() []gin.H {
	return []gin.H{
		{
			"name":        "read_my_design",
			"description": "获取指定节点的详细设计信息，包括所有子节点",
			"inputSchema": gin.H{
				"type": "object",
				"properties": gin.H{
					"nodeId": gin.H{"type": "string", "description": "节点ID"},
				},
				"required": []string{"nodeId"},
			},
		},
		{
			"name":        "export_node_as_image",
			"description": "导出节点为图片",
			"inputSchema": gin.H{
				"type": "object",
				"properties": gin.H{
					"nodeId":      gin.H{"type": "string", "description": "节点ID"},
					"format":      gin.H{"type": "string", "enum": []string{"PNG", "JPG", "SVG", "PDF"}, "description": "导出格式", "default": "PNG"},
					"scale":       gin.H{"type": "number", "description": "缩放比例", "default": 1},
					"ignore_text": gin.H{"type": "boolean", "description": "是否忽略文本节点（不渲染文本）", "default": false},
				},
				"required": []string{"nodeId"},
			},
		},
		{
			"name":        "export_nodes_as_images",
			"description": "批量导出多个节点为图片，返回所有图片的base64数据字典",
			"inputSchema": gin.H{
				"type": "object",
				"properties": gin.H{
					"nodeIds":     gin.H{"type": "string", "description": "节点ID列表，使用逗号分隔（例如：'1-123,1-456,1-789'）"},
					"scale":       gin.H{"type": "number", "description": "缩放比例", "default": 1},
					"ignore_text": gin.H{"type": "boolean", "description": "是否忽略文本节点（不渲染文本）", "default": false},
				},
				"required": []string{"nodeIds"},
			},
		},
	}
}

// GetAllTools 获取所有工具（基于WebSocket连接状态）
func GetAllTools(userID uint, connectionID string, include_figma_plugin bool) []gin.H {
	tools := GetBasicTools()

	// 检查用户是否有活跃的WebSocket连接
	mcpService := services.GetMCPService()
	if mcpService.HasActiveWebSocketConnection(connectionID) && include_figma_plugin {
		// 如果有WebSocket连接，添加Figma插件工具
		tools = append(tools, GetFigmaPluginTools()...)
	}

	return tools
}

// ForwardToFigmaPlugin 将MCP工具调用转发到Figma插件（通过WebSocket）
func ForwardToFigmaPlugin(userID uint, connectionID, toolName string, arguments map[string]interface{}) (interface{}, error) {
	mcpService := services.GetMCPService()

	// 检查WebSocket连接
	if !mcpService.HasActiveWebSocketConnection(connectionID) {
		return nil, fmt.Errorf("没有活跃的WebSocket连接，请先确保Figma插件已连接")
	}

	// 生成请求ID
	requestID := uuid.New().String()

	// 构建发送到Figma插件的消息
	message := map[string]interface{}{
		"id":      requestID,
		"command": toolName,
		"params":  arguments,
	}

	// 创建响应通道
	responseChan := make(chan interface{}, 1)
	errorChan := make(chan error, 1)

	// 注册请求等待响应
	mcpService.RegisterPendingRequest(requestID, responseChan, errorChan)
	defer mcpService.UnregisterPendingRequest(requestID)

	// 获取当前用户的频道
	channel := mcpService.GetUserChannel(connectionID)
	if channel == "" {
		// 如果没有加入频道，默认使用 figma-bridge
		channel = "figma-bridge"
		// 自动加入默认频道
		_ = mcpService.JoinChannel(connectionID, channel)
	}

	// 广播消息到频道
	err := mcpService.BroadcastToChannel(connectionID, channel, message)
	if err != nil {
		return nil, fmt.Errorf("发送消息到Figma插件失败: %v", err)
	}

	// 等待响应（30秒超时）
	select {
	case response := <-responseChan:
		return response, nil
	case err := <-errorChan:
		return nil, err
	case <-time.After(60 * time.Second):
		return nil, fmt.Errorf("请求超时：Figma插件未在60秒内响应")
	}
}

// HandleFigmaPluginToolCall 处理Figma插件工具调用
func HandleFigmaPluginToolCall(userID uint, connectionID, toolName string, arguments map[string]interface{}, requestID interface{}) gin.H {
	// 其他工具调用转发到Figma插件
	startTime := time.Now()
	result, err := ForwardToFigmaPlugin(userID, connectionID, toolName, arguments)
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		// 记录失败日志
		models.LogMCPCall(userID, connectionID, toolName, arguments, nil, "error", err.Error(), duration)

		return gin.H{
			"jsonrpc": "2.0",
			"id":      requestID,
			"error": gin.H{
				"code":    -32603,
				"message": err.Error(),
			},
		}
	}

	// 特殊处理 export_nodes_as_images：上传到华为云OBS
	if toolName == "export_nodes_as_images" {
		result = handleBatchImageUpload(result, arguments)
	}

	// 记录成功日志
	models.LogMCPCall(userID, connectionID, toolName, arguments, result, "success", "", duration)

	// 格式化返回结果
	return gin.H{
		"jsonrpc": "2.0",
		"id":      requestID,
		"result": gin.H{
			"content": []gin.H{
				{
					"type": "text",
					"text": formatToolResult(result),
				},
			},
		},
	}
}

// SimulateMCPToolCall 模拟MCP工具调用（基础工具直接处理，插件工具通过WebSocket转发）
func SimulateMCPToolCall(c *gin.Context) {
	// Get User ID from context (set by middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	var req struct {
		Command string                 `json:"command" binding:"required"`
		Params  map[string]interface{} `json:"params"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 定义基础工具列表（不需要 Figma 插件连接，直接调用后端处理）
	basicTools := map[string]bool{
		"get_preview":         true,
		"get_node_tree":       true,
		"set_nodes_modify":    true,
		"get_modify_prompt":   true,
		"clear_nodes_modifys": true,
		"get_ref_nodes":       true,
	}

	// Create a request ID
	requestID := "sim_" + strconv.FormatInt(time.Now().UnixNano(), 10)

	// 检查是否为基础工具
	if basicTools[req.Command] {
		// 基础工具：直接调用 handleToolCall（在 mcp.go 中定义）
		// 注意：基础工具不需要 connectionID
		response := HandleBasicToolCall(userID.(uint), req.Command, req.Params, requestID)
		c.JSON(http.StatusOK, response)
		return
	}

	// 插件工具：需要 WebSocket 连接，转发到 Figma 插件
	mcpService := services.GetMCPService()
	conn, exists := mcpService.GetUserConnection(userID.(uint))
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到活跃的 Figma 连接，请先打开 Figma 插件"})
		return
	}

	response := HandleFigmaPluginToolCall(userID.(uint), conn.ConnectionID, req.Command, req.Params, requestID)
	c.JSON(http.StatusOK, response)
}

// GetToolsList 获取所有可用的 MCP 工具列表
func GetToolsList(c *gin.Context) {
	// Get User ID from context (set by middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	// Get MCP Service
	mcpService := services.GetMCPService()

	// Find active connection for user to check status
	conn, exists := mcpService.GetUserConnection(userID.(uint))
	connectionID := ""
	if exists {
		connectionID = conn.ConnectionID
	}

	// Get all tools based on connection status
	tools := GetAllTools(userID.(uint), connectionID, true)

	c.JSON(http.StatusOK, gin.H{
		"tools": tools,
	})
}

// handleBatchImageUpload 处理批量图片上传到华为云OBS
func handleBatchImageUpload(result interface{}, arguments map[string]interface{}) interface{} {
	// 解析结果
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return result
	}

	// 检查是否包含 images 字段
	images, ok := resultMap["images"].(map[string]interface{})
	if !ok || len(images) == 0 {
		return result
	}

	// 获取 OBS 服务（使用 figma.go 中的全局变量）
	if globalOBSService == nil || !globalOBSService.IsEnabled() {
		// OBS 未启用，返回原结果
		resultMap["obs_enabled"] = false
		resultMap["obs_message"] = "OBS服务未启用，图片未上传到云端"
		return resultMap
	}

	// 从参数中获取相关信息
	scale := 1.0
	if s, ok := arguments["scale"].(float64); ok {
		scale = s
	}

	// 准备上传结果
	obsURLs := make(map[string]string)
	obsKeys := make(map[string]string)
	uploadErrors := make(map[string]string)

	// 批量上传到 OBS
	for nodeID, base64Data := range images {
		base64Str, ok := base64Data.(string)
		if !ok {
			uploadErrors[nodeID] = "图片数据格式错误"
			continue
		}

		// 解码 base64
		imageBytes, err := decodeBase64(base64Str)
		if err != nil {
			uploadErrors[nodeID] = fmt.Sprintf("解码base64失败: %v", err)
			continue
		}

		// 生成文件key（使用nodeID作为唯一标识）
		fileKey := fmt.Sprintf("batch_%d", time.Now().UnixNano())
		format := "png" // 默认PNG格式
		contentType := "image/png"

		// 上传到 OBS
		obsURL, obsKey, fileSize, err := globalOBSService.UploadImageFromBytes(
			imageBytes,
			fileKey,
			nodeID,
			format,
			scale,
			contentType,
		)

		if err != nil {
			uploadErrors[nodeID] = fmt.Sprintf("上传失败: %v", err)
			continue
		}

		obsURLs[nodeID] = obsURL
		obsKeys[nodeID] = obsKey

		// 异步保存到数据库
		go saveImageToCache(fileKey, nodeID, format, scale, obsKey, obsURL, fileSize, false) // 默认不忽略文本
	}

	// 添加 OBS 信息到结果
	resultMap["obs_enabled"] = true
	resultMap["obs_urls"] = obsURLs
	resultMap["obs_keys"] = obsKeys
	if len(uploadErrors) > 0 {
		resultMap["obs_errors"] = uploadErrors
	}
	resultMap["obs_uploaded"] = len(obsURLs)
	resultMap["obs_failed"] = len(uploadErrors)

	return resultMap
}

// decodeBase64 解码base64字符串
func decodeBase64(base64Str string) ([]byte, error) {
	// 移除可能的数据URI前缀
	if idx := strings.Index(base64Str, ","); idx != -1 {
		base64Str = base64Str[idx+1:]
	}

	// 移除可能的空白字符
	base64Str = strings.TrimSpace(base64Str)

	// 解码 base64
	return base64.StdEncoding.DecodeString(base64Str)
}

// saveImageToCache 保存图片信息到数据库缓存
func saveImageToCache(fileKey, nodeID, format string, scale float64, obsKey, obsURL string, fileSize int64, ignoreTexts bool) {
	now := uint32(time.Now().Unix())
	expiresAt := now + (7 * 24 * 3600) // 7天过期

	// 检查是否已存在
	existingImage, err := models.GetNodeImage(fileKey, nodeID, format, scale, ignoreTexts)
	if err == nil && existingImage != nil {
		// 更新现有记录
		existingImage.OBSKey = obsKey
		existingImage.OBSExpiresAt = expiresAt
		existingImage.FileSize = uint64(fileSize)
		existingImage.Status = "obs_synced"
		existingImage.UpdatedAt = now

		if err := models.UpdateNodeImage(existingImage); err != nil {
			fmt.Printf("⚠️ 更新数据库缓存失败: %v\n", err)
		}
	} else {
		// 创建新记录
		newImage := &models.FigmaNodeImage{
			FileKey:      fileKey,
			NodeID:       nodeID,
			Format:       format,
			Scale:        scale,
			IgnoreTexts:  ignoreTexts,
			FigmaCDNURL:  obsURL,
			OBSKey:       obsKey,
			OBSExpiresAt: expiresAt,
			FileSize:     uint64(fileSize),
			Status:       "obs_synced",
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		if err := models.CreateNodeImage(newImage); err != nil {
			fmt.Printf("⚠️ 创建数据库缓存失败: %v\n", err)
		}
	}
}

// formatToolResult 格式化工具调用结果
func formatToolResult(result interface{}) string {
	// 如果结果已经是字符串，直接返回
	if str, ok := result.(string); ok {
		return str
	}

	// 否则转为JSON字符串
	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", result)
	}
	return string(jsonBytes)
}
