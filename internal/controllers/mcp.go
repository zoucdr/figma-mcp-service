package controllers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/figma-deliver/internal/models"
	"github.com/figma-deliver/internal/services"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// MCPGetCallLogsByProject 按项目获取MCP调用日志
func MCPGetCallLogsByProject(c *gin.Context) {
	// 验证用户登录状态
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "用户未登录",
		})
		return
	}

	projectID := c.Param("project_id")
	if projectID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID不能为空",
		})
		return
	}

	// 验证项目是否属于当前用户
	var project models.FigmaProject
	err := models.DB.Where("id = ? AND user_id = ?", projectID, userID.(uint)).First(&project).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在或无权限访问",
		})
		return
	}

	limitStr := c.DefaultQuery("limit", "100")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100 // 最多返回100条
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	// 从数据库获取MCP调用日志
	projectIDUint, err := strconv.ParseUint(projectID, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID格式错误",
		})
		return
	}

	logs, err := models.GetMCPCallLogsByProject(userID.(uint), uint(projectIDUint), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取MCP调用日志失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"logs":    logs,
		"limit":   limit,
		"offset":  offset,
		"count":   len(logs),
	})
}

// MCPClearCallLogsByProject 清理项目的MCP调用日志
func MCPClearCallLogsByProject(c *gin.Context) {
	// 验证用户登录状态
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "用户未登录",
		})
		return
	}

	projectID := c.Param("project_id")
	if projectID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID不能为空",
		})
		return
	}

	// 验证项目是否属于当前用户
	var project models.FigmaProject
	err := models.DB.Where("id = ? AND user_id = ?", projectID, userID.(uint)).First(&project).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在或无权限访问",
		})
		return
	}

	// 清理MCP调用日志
	projectIDUint, err := strconv.ParseUint(projectID, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID格式错误",
		})
		return
	}

	err = models.ClearMCPCallLogsByProject(userID.(uint), uint(projectIDUint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "清理MCP调用日志失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "MCP调用日志已清理",
	})
}

// MCPResendCall 重新发送MCP调用
func MCPResendCall(c *gin.Context) {
	// 验证用户登录状态
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "用户未登录",
		})
		return
	}

	projectID := c.Param("project_id")
	if projectID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID不能为空",
		})
		return
	}

	// 验证项目是否属于当前用户
	var project models.FigmaProject
	err := models.DB.Where("id = ? AND user_id = ?", projectID, userID.(uint)).First(&project).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在或无权限访问",
		})
		return
	}

	// 解析请求体
	var requestData struct {
		LogID       uint                   `json:"log_id" binding:"required"`
		Method      string                 `json:"method" binding:"required"`
		RequestData map[string]interface{} `json:"request_data" binding:"required"`
	}

	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数错误: " + err.Error(),
		})
		return
	}

	// 验证原始日志记录是否存在且属于当前用户
	var originalLog models.MCPCallLog
	err = models.DB.Where("id = ? AND user_id = ?", requestData.LogID, userID.(uint)).First(&originalLog).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "原始调用记录不存在或无权限访问",
		})
		return
	}

	// 获取用户的MCP token
	var user models.User
	err = models.DB.Where("id = ?", userID.(uint)).First(&user).Error
	if err != nil || user.MCPToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "用户MCP token不存在，请先生成MCP token",
		})
		return
	}

	// 模拟重新发送MCP调用
	startTime := time.Now()

	// 根据原始请求的方法类型来重新执行调用
	var response gin.H

	// 检查请求数据的结构
	// 情况1: 顶层就是MCP协议请求 {"jsonrpc": "2.0", "method": "..."}
	// 情况2: 嵌套结构 {"body": {"jsonrpc": "2.0", ...}, "method": "POST", "path": "/mcp/..."}

	var actualRequestBody map[string]interface{}

	// 检查是否有body字段（嵌套结构）
	if bodyData, hasBody := requestData.RequestData["body"].(map[string]interface{}); hasBody {
		actualRequestBody = bodyData
	} else {
		actualRequestBody = requestData.RequestData
	}

	// 检查是否是MCP协议请求
	if jsonrpc, exists := actualRequestBody["jsonrpc"]; exists && jsonrpc == "2.0" {
		// 处理MCP协议请求
		response = handleMCPProtocolRequest(userID.(uint), user.MCPToken, actualRequestBody)
	} else {
		// 处理非标准MCP请求或REST API请求
		// 从原始method中提取HTTP方法和路径
		methodParts := strings.SplitN(requestData.Method, " ", 2)
		if len(methodParts) != 2 {
			// 如果无法解析method，可能是直接的MCP调用，提供友好的错误信息
			response = gin.H{
				"error":           "重发失败",
				"message":         "无法解析原始请求方法格式",
				"hint":            "原始请求可能不是标准的HTTP请求格式",
				"original_method": requestData.Method,
			}
		} else {
			httpMethod := methodParts[0]
			path := methodParts[1]

			// 重新执行请求
			response = handleMCPRequest(c, userID.(uint), user.MCPToken, httpMethod, path, "", actualRequestBody)
		}
	}

	duration := time.Since(startTime).Milliseconds()

	// 记录新的调用日志
	status := "success"
	errorMsg := ""
	if errorResponse, exists := response["error"]; exists && errorResponse != nil {
		status = "error"
		if errMap, ok := errorResponse.(map[string]interface{}); ok {
			if msg, ok := errMap["message"].(string); ok {
				errorMsg = msg
			}
		}
	}

	// 添加重发标记到method中
	resendMethod := requestData.Method + " (重发)"
	models.LogMCPCall(userID.(uint), user.MCPToken, resendMethod, requestData.RequestData, response, status, errorMsg, duration)

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "MCP调用重发成功",
		"response": response,
		"duration": duration,
	})
}

// ============ Cursor MCP HTTP接口 ============

// validateCursorConnectionID 验证Cursor MCP connection_id
func validateCursorConnectionID(connectionID string) (uint, error) {
	// 查找对应的MCP连接
	var connection models.MCPConnection
	err := models.DB.Where("connection_id = ? AND is_active = ?", connectionID, true).First(&connection).Error
	if err != nil {
		return 0, fmt.Errorf("无效的连接ID或连接已过期")
	}

	// 更新最后活动时间
	connection.LastActivity = time.Now()
	models.DB.Save(&connection)

	return connection.UserID, nil
}

// validateCursorToken 验证Cursor MCP token (保留向后兼容)
func validateCursorToken(token string) (uint, error) {
	// 首先从用户表中查找token
	var user models.User
	err := models.DB.Where("mcp_token = ? AND mcp_token != ''", token).First(&user).Error
	if err == nil {
		// 找到用户，确保MCPConnection记录存在并设置为连接状态
		var connection models.MCPConnection
		err = models.DB.Where("user_id = ? AND connection_id = ?", user.ID, token).First(&connection).Error

		if err != nil {
			// 如果连接记录不存在，创建一个新的连接记录
			connection = models.MCPConnection{
				UserID:       user.ID,
				ConnectionID: token,
				Status:       "connected", // 有访问就设置为连接状态
				IsActive:     true,
				LastActivity: time.Now(),
			}
			models.DB.Create(&connection)
			log.Printf("MCP连接已创建: 用户ID=%d, Token=%s", user.ID, token)
		} else {
			// 更新现有连接：只要有访问就设置为连接状态
			connection.Status = "connected"
			connection.IsActive = true
			connection.LastActivity = time.Now()
			models.DB.Save(&connection)
			log.Printf("MCP连接状态已更新为连接: 用户ID=%d, Token=%s", user.ID, token)
		}

		return user.ID, nil
	}

	// 如果用户表中没有找到，尝试从MCP连接表中查找（向后兼容）
	return validateCursorConnectionID(token)
}

// CursorMCPHandler 统一处理Cursor MCP请求
func CursorMCPHandler(c *gin.Context) {
	token := c.Param("connection_id") // 实际上是token，保持参数名兼容
	method := c.Request.Method
	path := c.Request.URL.Path
	query := c.Request.URL.RawQuery

	// 通过user数据库，查找对应的userid
	var user models.User
	err := models.DB.Where("mcp_token = ? AND mcp_token != ''", token).First(&user).Error
	if err != nil {

		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "无效的连接token或用户不存在",
		})
		return
	}
	userID := user.ID

	// 解析请求体（如果有）
	var requestBody map[string]interface{}
	if method == "POST" || method == "PUT" {
		if err := c.ShouldBindJSON(&requestBody); err != nil {
			// 如果不是JSON格式，尝试解析为MCP请求
			var mcpRequest map[string]interface{}
			if err2 := c.ShouldBindJSON(&mcpRequest); err2 == nil {
				requestBody = mcpRequest
			}
		}
	}

	// 构建完整的请求数据用于日志记录
	requestData := make(map[string]interface{})
	requestData["method"] = method
	requestData["path"] = path
	if query != "" {
		requestData["query"] = query
	}
	if requestBody != nil {
		requestData["body"] = requestBody
	}

	// 处理不同的MCP请求
	startTime := time.Now()
	response := handleMCPRequest(c, userID, token, method, path, query, requestBody)
	duration := time.Since(startTime).Milliseconds()

	// 记录调用日志（包含响应数据）
	status := "success"
	errorMsg := ""
	if errorResponse, exists := response["error"]; exists && errorResponse != nil {
		status = "error"
		if errMap, ok := errorResponse.(map[string]interface{}); ok {
			if msg, ok := errMap["message"].(string); ok {
				errorMsg = msg
			}
		}
	}

	models.LogMCPCall(userID, token, method+" "+path, requestData, response, status, errorMsg, duration)

	c.JSON(http.StatusOK, response)
}

// handleMCPRequest 处理具体的MCP请求
func handleMCPRequest(_ *gin.Context, userID uint, connectionID, method, path, query string, requestBody map[string]interface{}) gin.H {
	// 解析路径参数
	pathParts := strings.Split(strings.Trim(path, "/"), "/")

	// 检查是否是MCP协议请求
	if requestBody != nil {
		if jsonrpc, exists := requestBody["jsonrpc"]; exists && jsonrpc == "2.0" {
			return handleMCPProtocolRequest(userID, connectionID, requestBody)
		}
	}

	// 处理REST API请求
	if len(pathParts) >= 2 && pathParts[0] == "mcp" {
		// 路径格式: /mcp/connection_id/action/...
		if len(pathParts) >= 3 {
			action := pathParts[2]
			return handleRESTRequest(userID, connectionID, method, action, pathParts[3:], query, requestBody)
		} else if len(pathParts) == 2 && method == "GET" {
			// 当路径为 /mcp/connection_id 且为GET请求时，返回工具列表
			return gin.H{
				"jsonrpc": "2.0",
				"result": gin.H{
					"tools": []gin.H{
						{
							"name":        "get_preview",
							"description": "获取Figma指定节点及其子树的原型图",
							"inputSchema": gin.H{
								"type": "object",
								"properties": gin.H{
									"project_id":   gin.H{"type": "number", "description": "项目ID"},
									"node_id":      gin.H{"type": "string", "description": "节点ID（可选，默认使用项目根节点）"},
									"format":       gin.H{"type": "string", "description": "图片格式", "enum": []string{"png", "jpg", "svg"}, "default": "png"},
									"scale":        gin.H{"type": "number", "description": "缩放比例 (0.1-4.0)", "minimum": 0.1, "maximum": 4.0, "default": 0.1},
									"ignore_texts": gin.H{"type": "boolean", "description": "是否忽略文本节点（不渲染文本）", "default": false},
									"ignore_nodes": gin.H{
										"type":        "array",
										"description": "要忽略的节点ID列表（这些节点将在渲染时被隐藏）",
										"items":       gin.H{"type": "string"},
										"default":     []string{},
									},
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
												"node_id":    gin.H{"type": "string", "description": "节点ID"},
												"rename":     gin.H{"type": "string", "description": "自定义名称"},
												"ignore":     gin.H{"type": "boolean", "description": "是否忽略此节点"},
												"horizontal": gin.H{"type": "string", "description": "水平约束", "enum": []string{"LEFT", "RIGHT", "CENTER", "LEFT_RIGHT", "SCALE"}},
												"vertical":   gin.H{"type": "string", "description": "垂直约束", "enum": []string{"TOP", "BOTTOM", "CENTER", "TOP_BOTTOM", "SCALE"}},
												"parent_id":  gin.H{"type": "string", "description": "父级节点ID"},
												"res_mode":   gin.H{"type": "string", "description": "资源模式", "enum": []string{"attach", "sprite", "slice", "texture", "cutout"}},
												"img_name":   gin.H{"type": "string", "description": "图片名称"},
												"img_id":     gin.H{"type": "string", "description": "图片重定向节点, 如果为空，则不重定向"},
												"img_ext":    gin.H{"type": "string", "description": "图片格式", "enum": []string{"png", "jpg", "svg"}},
												"components": gin.H{"type": "array", "items": gin.H{"type": "string"}, "description": "组件列表"},
											},
											"required": []string{"node_id"},
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
									"type":       gin.H{"type": "string", "description": "配置类型", "enum": []string{"layout", "style", "content"}},
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
					},
				},
			}
		}
	}

	// 默认响应
	return gin.H{
		"error":   "不支持的请求",
		"method":  method,
		"path":    path,
		"message": "请使用标准的MCP协议请求",
	}
}

// handleMCPProtocolRequest 处理MCP协议请求
func handleMCPProtocolRequest(userID uint, connectionID string, request map[string]interface{}) gin.H {
	method, _ := request["method"].(string)
	id := request["id"]

	switch method {
	case "initialize":
		return gin.H{
			"jsonrpc": "2.0",
			"id":      id,
			"result": gin.H{
				"protocolVersion": "2024-11-05",
				"capabilities": gin.H{
					"tools": gin.H{
						"listChanged": true,
					},
				},
				"serverInfo": gin.H{
					"name":    "figma-deliver-mcp",
					"version": "1.0.0",
				},
			},
		}

	case "tools/list":
		// 使用新的动态工具列表
		tools := GetAllTools(userID, connectionID, false)

		return gin.H{
			"jsonrpc": "2.0",
			"id":      id,
			"result": gin.H{
				"tools": tools,
			},
		}

	case "tools/call":
		params, _ := request["params"].(map[string]interface{})
		if params == nil {
			return gin.H{
				"jsonrpc": "2.0",
				"id":      id,
				"error": gin.H{
					"code":    -32602,
					"message": "Invalid params",
				},
			}
		}

		toolName, _ := params["name"].(string)
		arguments, _ := params["arguments"].(map[string]interface{})

		// 检查是否是Figma插件工具
		figmaTools := GetFigmaPluginTools()
		isFigmaTool := false
		for _, tool := range figmaTools {
			if name, ok := tool["name"].(string); ok && name == toolName {
				isFigmaTool = true
				break
			}
		}

		// 如果是Figma插件工具，使用新的处理函数
		if isFigmaTool {
			return HandleFigmaPluginToolCall(userID, connectionID, toolName, arguments, id)
		}

		// 否则使用原有的handleToolCall
		return handleToolCall(userID, connectionID, toolName, arguments, id)

	default:
		return gin.H{
			"jsonrpc": "2.0",
			"id":      id,
			"error": gin.H{
				"code":    -32601,
				"message": "Method not found",
			},
		}
	}
}

// handleRESTRequest 处理REST API请求
func handleRESTRequest(userID uint, connectionID, method, action string, pathParams []string, query string, requestBody map[string]interface{}) gin.H {
	switch action {
	case "preview":
		if len(pathParams) >= 2 {
			projectID := pathParams[0]
			nodeID := pathParams[1]
			return gin.H{
				"success": true,
				"data": gin.H{
					"project_id":  projectID,
					"node_id":     nodeID,
					"preview_url": fmt.Sprintf("/figma/image/%s/%s", projectID, nodeID),
					"width":       800,
					"height":      600,
					"format":      "png",
					"timestamp":   time.Now(),
				},
				"message": "预览图获取成功（模拟数据）",
			}
		}

	case "config":
		if len(pathParams) >= 2 {
			projectID := pathParams[0]
			nodeID := pathParams[1]

			if method == "GET" {
				return gin.H{
					"success": true,
					"data": gin.H{
						"project_id": projectID,
						"node_id":    nodeID,
						"config": gin.H{
							"horizontal": "CENTER",
							"vertical":   "CENTER",
							"res_mode":   "attach",
							"components": []string{},
						},
						"timestamp": time.Now(),
					},
					"message": "节点配置获取成功（模拟数据）",
				}
			} else if method == "POST" {
				return gin.H{
					"success": true,
					"data": gin.H{
						"project_id": projectID,
						"node_id":    nodeID,
						"updated":    true,
						"timestamp":  time.Now(),
					},
					"message": "节点配置更新成功（模拟数据）",
				}
			}
		}

	case "prompt":
		if len(pathParams) >= 2 {
			projectID := pathParams[0]
			nodeID := pathParams[1]
			return gin.H{
				"success": true,
				"data": gin.H{
					"project_id": projectID,
					"node_id":    nodeID,
					"prompt":     "这是一个示例配置提示词，用于指导如何配置此节点。",
					"timestamp":  time.Now(),
				},
				"message": "配置提示词获取成功（模拟数据）",
			}
		}
	}

	return gin.H{
		"error":   "不支持的操作",
		"action":  action,
		"method":  method,
		"message": "请检查请求路径和参数",
	}
}

// getProjectRootNodeID 获取项目的根节点ID
func getProjectRootNodeID(userID uint, projectIDInt uint) (string, error) {
	// 获取项目信息
	project, err := models.GetProjectByID(projectIDInt)
	if err != nil {
		return "", fmt.Errorf("项目不存在: %v", err)
	}

	// 验证用户权限
	if project.UserID != userID {
		return "", fmt.Errorf("无权限访问此项目")
	}

	// 如果项目有根节点ID，直接返回
	if project.RootNodeID != "" {
		return project.RootNodeID, nil
	}

	// 如果没有根节点ID，返回默认的文档根节点ID
	return "0:0", nil
}

// getProjectNodeTree 获取项目的节点树信息（仅从缓存获取，不访问 Figma API）
func getProjectNodeTree(userID uint, projectIDInt uint, nodeID string) (map[string]interface{}, error) {
	// 获取项目信息
	project, err := models.GetProjectByID(projectIDInt)
	if err != nil {
		return nil, fmt.Errorf("项目不存在: %v", err)
	}

	// 验证用户权限
	if project.UserID != userID {
		return nil, fmt.Errorf("无权限访问此项目")
	}

	// 如果没有提供节点ID，使用项目的根节点ID
	if nodeID == "" {
		if project.RootNodeID != "" {
			nodeID = project.RootNodeID
		} else {
			nodeID = "0:0"
		}
	}

	// 从数据库缓存获取节点树数据
	nodes, err := getNodesFromCache(project.FileKey, nodeID)
	if err != nil {
		return nil, fmt.Errorf("获取节点树数据失败（缓存未找到）: %v", err)
	}

	// 如果节点列表为空，返回错误
	if len(nodes) == 0 {
		return nil, fmt.Errorf("节点树数据为空（请先在 Dashboard 中刷新节点树）")
	}

	// 如果节点ID是根节点ID，并且nodes不为空，则使用nodes中的第一个节点ID
	if nodeID == project.RootNodeID && len(nodes) > 0 {
		if firstNodeID, ok := nodes[0]["id"].(string); ok {
			nodeID = firstNodeID
		}
	}

	// 获取所有节点的修改信息
	var figmaNodes []models.FigmaNode
	dbResult := models.DB.Where("project_id = ?", projectIDInt).Find(&figmaNodes)
	if dbResult.Error != nil {
		return nil, fmt.Errorf("获取节点修改信息失败: %v", dbResult.Error)
	}

	// 构建节点ID到修改信息的映射
	nodeModifys := make(map[string]map[string]interface{})
	for _, node := range figmaNodes {
		if node.Modifys != "" {
			var modifyData map[string]interface{}
			if err := json.Unmarshal([]byte(node.Modifys), &modifyData); err == nil {
				nodeModifys[node.NodeID] = modifyData
			}
		}
	}

	// 过滤并构建节点树
	filteredTree := buildFilteredNodeTree(nodes, nodeID, nodeModifys)

	return filteredTree, nil
}

// getNodesFromCache 从文件缓存和数据库缓存获取节点树（支持子树查找）
func getNodesFromCache(fileKey, nodeID string) ([]map[string]interface{}, error) {
	// 1. 先尝试从文件缓存中加载
	nodes, err := services.LoadCachedNodesFromFile(fileKey, nodeID)
	if err == nil && len(nodes) > 0 {
		log.Printf("✅ [MCP] 从文件缓存获取节点树: fileKey=%s, nodeID=%s, 节点数=%d",
			fileKey, nodeID, len(nodes))
		return nodes, nil
	}

	log.Printf("⚠️ [MCP] 文件缓存未找到或为空 (fileKey=%s, nodeID=%s)，尝试数据库缓存", fileKey, nodeID)

	// 2. 尝试从数据库直接查找该节点ID的缓存
	cache, err := models.GetFileCache(fileKey, nodeID)
	if err == nil && cache != nil && cache.FileData != "" {
		// 解析缓存的节点数据
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(cache.FileData), &result); err == nil {
			nodes := parseFigmaNodes(result, nodeID)
			if len(nodes) > 0 {
				log.Printf("✅ [MCP] 从数据库缓存获取节点树: fileKey=%s, nodeID=%s, 节点数=%d",
					fileKey, nodeID, len(nodes))
				return nodes, nil
			}
		}
	}

	// 3. 如果没有找到，尝试从根节点缓存中查找子树
	// 查找所有该 fileKey 的缓存记录
	var caches []models.FigmaFileCache
	if err := models.DB.Where("file_key = ? AND status = ?", fileKey, "loaded").Find(&caches).Error; err != nil {
		return nil, fmt.Errorf("查询缓存失败: %v", err)
	}

	// 遍历所有缓存，查找包含目标节点ID的缓存
	for _, cache := range caches {
		// 检查 node_ids 字段是否包含目标节点ID
		if cache.NodeIDs != "" && strings.Contains(cache.NodeIDs, nodeID) {
			// 解析缓存的节点数据
			var result map[string]interface{}
			if err := json.Unmarshal([]byte(cache.FileData), &result); err == nil {
				// 解析所有节点
				nodes := parseFigmaNodes(result, cache.RootNodeID)

				// 从所有节点中筛选出目标节点的子树
				filteredNodes := filterNodeSubtree(nodes, nodeID)
				if len(filteredNodes) > 0 {
					log.Printf("✅ [MCP] 从父缓存中提取子树: fileKey=%s, parentNodeID=%s, targetNodeID=%s, 节点数=%d",
						fileKey, cache.RootNodeID, nodeID, len(filteredNodes))
					return filteredNodes, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("缓存中未找到节点树数据 (fileKey=%s, nodeID=%s)", fileKey, nodeID)
}

// parseFigmaNodes 解析 Figma API 响应为节点列表（从 figma.go 复制）
func parseFigmaNodes(result map[string]interface{}, nodeID string) []map[string]interface{} {
	nodes := []map[string]interface{}{}

	// 尝试从 nodes 字段解析（单节点查询）
	if nodesObj, ok := result["nodes"].(map[string]interface{}); ok {
		if nodeData, ok := nodesObj[nodeID].(map[string]interface{}); ok {
			if document, ok := nodeData["document"].(map[string]interface{}); ok {
				flattenNode(document, nil, &nodes)
			}
		}
	}

	// 尝试从 document 字段解析（整个文件查询）
	if document, ok := result["document"].(map[string]interface{}); ok {
		flattenNode(document, nil, &nodes)
	}

	return nodes
}

// flattenNode 递归扁平化节点树（从 figma.go 复制）
func flattenNode(node map[string]interface{}, parentID interface{}, nodes *[]map[string]interface{}) {
	if node == nil {
		return
	}

	// 创建节点副本
	flatNode := make(map[string]interface{})
	for k, v := range node {
		if k != "children" {
			flatNode[k] = v
		}
	}

	// 添加父节点ID
	if parentID != nil {
		flatNode["parent_id"] = parentID
	}

	// 添加到节点列表
	*nodes = append(*nodes, flatNode)

	// 递归处理子节点
	if children, ok := node["children"].([]interface{}); ok {
		currentID := node["id"]
		for _, child := range children {
			if childMap, ok := child.(map[string]interface{}); ok {
				flattenNode(childMap, currentID, nodes)
			}
		}
	}
}

// filterNodeSubtree 从节点列表中筛选出指定节点的子树
func filterNodeSubtree(nodes []map[string]interface{}, targetNodeID string) []map[string]interface{} {
	// 构建节点ID到节点的映射
	nodeMap := make(map[string]map[string]interface{})
	for _, node := range nodes {
		if nodeID, ok := node["id"].(string); ok {
			nodeMap[nodeID] = node
		}
	}

	// 查找目标节点
	targetNode, found := nodeMap[targetNodeID]
	if !found {
		return nil
	}

	// 递归收集子树的所有节点
	subtreeNodes := []map[string]interface{}{}
	collectSubtreeNodes(targetNode, nodeMap, nodes, &subtreeNodes)

	return subtreeNodes
}

// collectSubtreeNodes 递归收集子树的所有节点
func collectSubtreeNodes(node map[string]interface{}, nodeMap map[string]map[string]interface{},
	allNodes []map[string]interface{}, result *[]map[string]interface{}) {

	nodeID, ok := node["id"].(string)
	if !ok {
		return
	}

	// 添加当前节点
	*result = append(*result, node)

	// 查找所有子节点
	for _, n := range allNodes {
		if parentID, ok := n["parent_id"].(string); ok && parentID == nodeID {
			collectSubtreeNodes(n, nodeMap, allNodes, result)
		}
	}
}

// buildFilteredNodeTree 构建过滤后的节点树
func buildFilteredNodeTree(nodes []map[string]interface{}, rootNodeID string, nodeModifys map[string]map[string]interface{}) map[string]interface{} {
	// 构建节点ID到节点的映射
	nodeMap := make(map[string]map[string]interface{})
	for _, node := range nodes {
		nodeID := node["id"].(string)
		nodeMap[nodeID] = node
	}

	// 构建父子关系映射
	childrenMap := make(map[string][]string)
	for _, node := range nodes {
		nodeID := node["id"].(string)
		if parentID, ok := node["parent_id"]; ok && parentID != nil {
			if parentIDStr, ok := parentID.(string); ok {
				childrenMap[parentIDStr] = append(childrenMap[parentIDStr], nodeID)
			}
		}
	}

	// 递归构建树结构
	return buildNodeRecursive(rootNodeID, nodeMap, childrenMap, nodeModifys)
}

// cleanModifysObject 清理 modifys 对象，移除默认值和空值
func cleanModifysObject(modifys map[string]interface{}) map[string]interface{} {
	cleaned := make(map[string]interface{})

	for key, value := range modifys {
		shouldSkip := false

		switch key {
		case "horizontal":
			// 默认值: "CENTER"
			if strVal, ok := value.(string); ok && strVal == "CENTER" {
				shouldSkip = true
			}
		case "vertical":
			// 默认值: "CENTER"
			if strVal, ok := value.(string); ok && strVal == "CENTER" {
				shouldSkip = true
			}
		case "components":
			// 默认值: 空数组
			if arrayVal, ok := value.([]interface{}); ok && len(arrayVal) == 0 {
				shouldSkip = true
			}
		case "ignore":
			// 默认值: false
			if boolVal, ok := value.(bool); ok && !boolVal {
				shouldSkip = true
			}
		case "img_ext":
			// 默认值: "png"
			if strVal, ok := value.(string); ok && strVal == "png" {
				shouldSkip = true
			}
		case "img_id", "img_name", "parent_id", "rename":
			// 默认值: 空字符串
			if strVal, ok := value.(string); ok && strVal == "" {
				shouldSkip = true
			}
		case "res_mode":
			// 默认值: "attach"
			if strVal, ok := value.(string); ok && strVal == "attach" {
				shouldSkip = true
			}
		}

		// 如果不是默认值，则添加到清理后的对象中
		if !shouldSkip {
			cleaned[key] = value
		}
	}

	return cleaned
}

// hasImageFill 检查节点是否有图片填充（fills中有imageRef）
func hasImageFill(node map[string]interface{}) bool {
	fills, ok := node["fills"]
	if !ok {
		return false
	}

	// fills 应该是一个数组
	fillsArray, ok := fills.([]interface{})
	if !ok {
		return false
	}

	// 遍历 fills 数组，检查是否有 imageRef
	for _, fill := range fillsArray {
		if fillMap, ok := fill.(map[string]interface{}); ok {
			if imageRef, exists := fillMap["imageRef"]; exists && imageRef != nil {
				return true
			}
		}
	}

	return false
}

// buildNodeRecursive 递归构建节点树
func buildNodeRecursive(nodeID string, nodeMap map[string]map[string]interface{}, childrenMap map[string][]string, nodeModifys map[string]map[string]interface{}) map[string]interface{} {
	node, exists := nodeMap[nodeID]
	if !exists {
		return nil
	}

	// 检查节点是否应该被排除
	if shouldExcludeNodeForTree(node, nodeModifys[nodeID]) {
		return nil
	}

	// 提取需要的字段 - 极简数据结构
	result := map[string]interface{}{}

	// 获取节点的修改信息
	modifys := nodeModifys[nodeID]

	// 处理name字段 - 如果有rename则使用rename，否则使用原name
	if modifys != nil && modifys["rename"] != nil {
		if rename, ok := modifys["rename"].(string); ok && rename != "" {
			result["name"] = rename
		} else if name, ok := node["name"]; ok {
			result["name"] = name
		}
	} else if name, ok := node["name"]; ok {
		result["name"] = name
	}

	if nodeType, ok := node["type"]; ok {
		result["type"] = nodeType
	}

	// 转换为 rect: [x, y, w, h]
	if bounds, ok := node["absoluteRenderBounds"].(map[string]interface{}); ok {
		var rect [4]float64
		if x, ok := bounds["x"].(float64); ok {
			rect[0] = math.Round(x*100) / 100 // 保留两位有效数字
		}
		if y, ok := bounds["y"].(float64); ok {
			rect[1] = math.Round(y*100) / 100
		}
		if w, ok := bounds["width"].(float64); ok {
			rect[2] = math.Round(w*100) / 100
		}
		if h, ok := bounds["height"].(float64); ok {
			rect[3] = math.Round(h*100) / 100
		}
		result["rect"] = []float64{rect[0], rect[1], rect[2], rect[3]}
	}

	// 将modify中的数据提取到外层
	if modifys != nil {
		// 提取horizontal到外层
		if horizontal, ok := modifys["horizontal"].(string); ok && horizontal != "" && horizontal != "CENTER" {
			result["horizontal"] = horizontal
		}

		// 提取vertical到外层
		if vertical, ok := modifys["vertical"].(string); ok && vertical != "" && vertical != "CENTER" {
			result["vertical"] = vertical
		}

		// parent_id不需要在树结构中提取到外层，因为树结构本身就表达了父子关系
		// 只有在需要重新指定父级时才在modifys中保留

		// 提取res_mode到外层
		if resMode, ok := modifys["res_mode"].(string); ok && resMode != "" && resMode != "attach" {
			result["res_mode"] = resMode
		}

		// 提取img_name到外层
		if imgName, ok := modifys["img_name"].(string); ok && imgName != "" {
			result["img_name"] = imgName
		}

		// 提取img_id到外层
		if imgID, ok := modifys["img_id"].(string); ok && imgID != "" {
			result["img_id"] = imgID
		}

		// 提取img_ext到外层
		if imgType, ok := modifys["img_ext"].(string); ok && imgType != "" && imgType != "png" {
			result["img_ext"] = imgType
		}

		// 提取components到外层
		if components, ok := modifys["components"].([]interface{}); ok && len(components) > 0 {
			result["components"] = components
		}

		// 提取ignore到外层
		if ignore, ok := modifys["ignore"].(bool); ok && ignore {
			result["ignore"] = ignore
		}
	}
	// 记录id
	result["id"] = nodeID

	// 如果res_mode是sprite、texture、slice，则无需子节点，直接返回
	if resMode, ok := result["res_mode"].(string); ok {
		if resMode == "sprite" || resMode == "texture" || resMode == "slice" {
			return result
		}
	}
	// 递归处理子节点
	var children []map[string]interface{}
	if childIDs, hasChildren := childrenMap[nodeID]; hasChildren {
		for _, childID := range childIDs {
			childNode := buildNodeRecursive(childID, nodeMap, childrenMap, nodeModifys)
			if childNode != nil {
				children = append(children, childNode)
			}
		}
	}
	if len(children) > 0 {
		result["children"] = children
	}
	return result
}

// shouldExcludeNodeForTree 判断节点是否应该在树中被排除
func shouldExcludeNodeForTree(node map[string]interface{}, modifys map[string]interface{}) bool {
	// 检查visible属性，如果为false则排除
	if visible, hasVisible := node["visible"]; hasVisible {
		if visibleBool, ok := visible.(bool); ok && !visibleBool {
			return true
		}
	}

	// 检查modifys中的ignore属性
	if modifys != nil {
		if ignore, ok := modifys["ignore"].(bool); ok && ignore {
			return true
		}
	}

	return false
}

// getMapKeys 获取map的所有键
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

// HandleBasicToolCall 处理基础工具调用（公开函数，供 HTTP API 使用）
func HandleBasicToolCall(userID uint, toolName string, arguments map[string]interface{}, requestID interface{}) gin.H {
	// 基础工具不需要 connectionID，传空字符串
	return handleToolCall(userID, "", toolName, arguments, requestID)
}

// handleToolCall 处理工具调用
func handleToolCall(userID uint, connectionID, toolName string, arguments map[string]interface{}, requestID interface{}) gin.H {
	switch toolName {
	case "get_preview":
		// 处理项目ID参数（支持int和string类型）
		var projectIDInt uint
		if projectIDFloat, ok := arguments["project_id"].(float64); ok {
			projectIDInt = uint(projectIDFloat)
		} else if projectIDStr, ok := arguments["project_id"].(string); ok {
			if parsed, err := strconv.ParseUint(projectIDStr, 10, 32); err == nil {
				projectIDInt = uint(parsed)
			}
		}

		if projectIDInt == 0 {
			return gin.H{
				"jsonrpc": "2.0",
				"id":      requestID,
				"error": gin.H{
					"code":    -32602,
					"message": "Missing or invalid required parameter: project_id",
				},
			}
		}

		// 处理节点ID参数（可选）
		nodeID, _ := arguments["node_id"].(string)
		if nodeID == "" {
			// 如果没有提供节点ID，使用项目的根节点
			var err error
			nodeID, err = getProjectRootNodeID(userID, projectIDInt)
			if err != nil {
				return gin.H{
					"jsonrpc": "2.0",
					"id":      requestID,
					"error": gin.H{
						"code":    -32603,
						"message": fmt.Sprintf("获取项目根节点失败: %v", err),
					},
				}
			}
		}

		projectID := strconv.FormatUint(uint64(projectIDInt), 10)

		// 获取可选参数
		format, _ := arguments["format"].(string)
		if format == "" {
			format = "png"
		}

		scaleInterface := arguments["scale"]
		scale := 0.1
		if scaleInterface != nil {
			if scaleFloat, ok := scaleInterface.(float64); ok {
				scale = scaleFloat
			} else if scaleStr, ok := scaleInterface.(string); ok {
				if parsedScale, err := strconv.ParseFloat(scaleStr, 64); err == nil {
					scale = parsedScale
				}
			}
		}

		// 处理 ignore_texts 参数
		ignoreTexts := false
		if ignoreTextsInterface := arguments["ignore_texts"]; ignoreTextsInterface != nil {
			if ignoreTextsBool, ok := ignoreTextsInterface.(bool); ok {
				ignoreTexts = ignoreTextsBool
			}
		}

		// 处理 ignore_nodes 参数
		var ignoreNodes []string
		if ignoreNodesInterface := arguments["ignore_nodes"]; ignoreNodesInterface != nil {
			if ignoreNodesArray, ok := ignoreNodesInterface.([]interface{}); ok {
				for _, nodeInterface := range ignoreNodesArray {
					if nodeStr, ok := nodeInterface.(string); ok {
						ignoreNodes = append(ignoreNodes, nodeStr)
					}
				}
			}
		}

		// 获取实际的预览图（优先使用 WebSocket）
		imagePath, imageBase64, err := getMCPPreviewImageWithWebSocket(userID, projectID, nodeID, format, scale, ignoreTexts, ignoreNodes)
		if err != nil {
			return gin.H{
				"jsonrpc": "2.0",
				"id":      requestID,
				"error": gin.H{
					"code":    -32603,
					"message": fmt.Sprintf("获取预览图失败: %v", err),
				},
			}
		}

		return gin.H{
			"jsonrpc": "2.0",
			"id":      requestID,
			"result": gin.H{
				"content": []gin.H{
					{
						"type": "text",
						"text": fmt.Sprintf("预览图获取成功 - 项目ID: %s, 节点ID: %s, 格式: %s, 缩放: %.1f",
							projectID, nodeID, format, scale),
					},
					{
						"type":     "image",
						"data":     imageBase64,
						"mimeType": fmt.Sprintf("image/%s", format),
					},
					{
						"type": "text",
						"text": fmt.Sprintf("图片信息: 文件路径: %s", imagePath),
					},
				},
			},
		}

	case "get_node_tree":
		// 处理项目ID参数（支持int和string类型）
		var projectIDInt uint
		if projectIDFloat, ok := arguments["project_id"].(float64); ok {
			projectIDInt = uint(projectIDFloat)
		} else if projectIDStr, ok := arguments["project_id"].(string); ok {
			if parsed, err := strconv.ParseUint(projectIDStr, 10, 32); err == nil {
				projectIDInt = uint(parsed)
			}
		}

		if projectIDInt == 0 {
			return gin.H{
				"jsonrpc": "2.0",
				"id":      requestID,
				"error": gin.H{
					"code":    -32602,
					"message": "Missing or invalid required parameter: project_id",
				},
			}
		}

		// 处理节点ID参数（可选）
		nodeID, _ := arguments["node_id"].(string)

		// 获取节点树（支持 WebSocket 兜底）
		nodeTree, err := getProjectNodeTreeWithWebSocket(userID, projectIDInt, nodeID)
		if err != nil {
			return gin.H{
				"jsonrpc": "2.0",
				"id":      requestID,
				"error": gin.H{
					"code":    -32603,
					"message": fmt.Sprintf("获取节点树失败: %v", err),
				},
			}
		}

		// 将节点树转换为JSON字符串
		nodeTreeJSON, err := json.Marshal(nodeTree)
		if err != nil {
			return gin.H{
				"jsonrpc": "2.0",
				"id":      requestID,
				"error": gin.H{
					"code":    -32603,
					"message": fmt.Sprintf("序列化节点树失败: %v", err),
				},
			}
		}

		projectID := strconv.FormatUint(uint64(projectIDInt), 10)
		displayNodeID := nodeID
		if displayNodeID == "" {
			displayNodeID = "根节点"
		}

		return gin.H{
			"jsonrpc": "2.0",
			"id":      requestID,
			"result": gin.H{
				"content": []gin.H{
					{
						"type": "text",
						"text": fmt.Sprintf("节点树获取成功 - 项目ID: %s, 节点ID: %s\n\n节点树结构:\n%s",
							projectID, displayNodeID, string(nodeTreeJSON)),
					},
				},
			},
		}

	case "set_nodes_modify":
		// 处理项目ID参数（支持int和string类型）
		var projectIDInt uint
		if projectIDFloat, ok := arguments["project_id"].(float64); ok {
			projectIDInt = uint(projectIDFloat)
		} else if projectIDStr, ok := arguments["project_id"].(string); ok {
			if parsed, err := strconv.ParseUint(projectIDStr, 10, 32); err == nil {
				projectIDInt = uint(parsed)
			}
		}

		modifys, _ := arguments["modifys"].([]interface{})

		if projectIDInt == 0 || modifys == nil || len(modifys) == 0 {
			return gin.H{
				"jsonrpc": "2.0",
				"id":      requestID,
				"error": gin.H{
					"code":    -32602,
					"message": "Missing required parameters: project_id, modifys",
				},
			}
		}

		projectID := strconv.FormatUint(uint64(projectIDInt), 10)

		// 处理批量节点修改
		var processedNodes []string
		var errors []string

		for i, modifyInterface := range modifys {
			modify, ok := modifyInterface.(map[string]interface{})
			if !ok {
				errors = append(errors, fmt.Sprintf("Invalid modify object format at index %d", i))
				continue
			}

			// 调试信息：打印modify对象的所有键
			log.Printf("MCP: modify object %d keys: %v", i, getMapKeys(modify))

			nodeID, _ := modify["node_id"].(string)
			if nodeID == "" {
				nodeID, _ = modify["id"].(string)
			}
			if nodeID == "" {
				errors = append(errors, fmt.Sprintf("Missing node_id in modify object at index %d (available keys: %v)", i, getMapKeys(modify)))
				continue
			}

			// 从 modify 对象中移除 node_id 字段，因为它不是修改信息的一部分
			modifyData := make(map[string]interface{})
			for key, value := range modify {
				if key != "node_id" && key != "id" {
					modifyData[key] = value
				}
			}

			// 保存节点修改信息到数据库（支持合并）
			err := models.SaveNodeModifysWithMerge(projectIDInt, nodeID, modifyData)
			if err != nil {
				errors = append(errors, fmt.Sprintf("保存节点 %s 失败: %v", nodeID, err))
				continue
			}

			processedNodes = append(processedNodes, nodeID)
		}

		resultText := fmt.Sprintf("批量节点配置已更新 - 项目ID: %s\n成功处理 %d 个节点: %v",
			projectID, len(processedNodes), processedNodes)

		if len(errors) > 0 {
			resultText += fmt.Sprintf("\n错误: %v", errors)
		}

		return gin.H{
			"jsonrpc": "2.0",
			"id":      requestID,
			"result": gin.H{
				"content": []gin.H{
					{
						"type": "text",
						"text": resultText,
					},
				},
			},
		}

	case "get_modify_prompt":
		// 处理项目ID参数（支持int和string类型）
		var projectIDInt uint
		if projectIDFloat, ok := arguments["project_id"].(float64); ok {
			projectIDInt = uint(projectIDFloat)
		} else if projectIDStr, ok := arguments["project_id"].(string); ok {
			if parsed, err := strconv.ParseUint(projectIDStr, 10, 32); err == nil {
				projectIDInt = uint(parsed)
			}
		}

		if projectIDInt == 0 {
			return gin.H{
				"jsonrpc": "2.0",
				"id":      requestID,
				"error": gin.H{
					"code":    -32602,
					"message": "Missing or invalid required parameter: project_id",
				},
			}
		}

		// 处理节点ID参数（可选）
		nodeID, _ := arguments["node_id"].(string)
		if nodeID == "" {
			// 如果没有提供节点ID，使用项目的根节点
			var err error
			nodeID, err = getProjectRootNodeID(userID, projectIDInt)
			if err != nil {
				return gin.H{
					"jsonrpc": "2.0",
					"id":      requestID,
					"error": gin.H{
						"code":    -32603,
						"message": fmt.Sprintf("获取项目根节点失败: %v", err),
					},
				}
			}
		}

		projectID := strconv.FormatUint(uint64(projectIDInt), 10)

		// 获取用户信息以获取自定义提示词
		user, err := models.FindUserByID(userID)
		if err != nil {
			return gin.H{
				"jsonrpc": "2.0",
				"id":      requestID,
				"error": gin.H{
					"code":    -32603,
					"message": fmt.Sprintf("获取用户信息失败: %v", err),
				},
			}
		}

		// 使用用户自定义的提示词，如果为空则使用默认提示词
		promptText := user.Prompts
		if promptText == "" {
			promptText = "1.基于mcp获取界面预览图，2.获取当前节点子树数据分析所有节点，3.修饰需要调整的所有节点,优化层级/显示/命名/布局/父级调整/是否包图/图片名称/图片id/图片格式/组件配置等等"
		}

		// 添加支持的控件信息列表
		if user.CompTypes != "" {
			promptText += "\n\n# 支持控件信息列表\n" + user.CompTypes
		}

		return gin.H{
			"jsonrpc": "2.0",
			"id":      requestID,
			"result": gin.H{
				"content": []gin.H{
					{
						"type": "text",
						"text": fmt.Sprintf("请按提示词流程操作： - 项目ID: %s, 节点ID: %s\n\n%s", projectID, nodeID, promptText),
					},
				},
			},
		}

	case "clear_nodes_modifys":
		// 处理项目ID参数（支持int和string类型）
		var projectIDInt uint
		if projectIDFloat, ok := arguments["project_id"].(float64); ok {
			projectIDInt = uint(projectIDFloat)
		} else if projectIDStr, ok := arguments["project_id"].(string); ok {
			if parsed, err := strconv.ParseUint(projectIDStr, 10, 32); err == nil {
				projectIDInt = uint(parsed)
			}
		}

		nodeIDs, _ := arguments["node_ids"].([]interface{})

		if projectIDInt == 0 || nodeIDs == nil || len(nodeIDs) == 0 {
			return gin.H{
				"jsonrpc": "2.0",
				"id":      requestID,
				"error": gin.H{
					"code":    -32602,
					"message": "Missing required parameters: project_id, node_ids",
				},
			}
		}

		// 验证项目权限
		project, err := models.GetProjectByID(projectIDInt)
		if err != nil {
			return gin.H{
				"jsonrpc": "2.0",
				"id":      requestID,
				"error": gin.H{
					"code":    -32603,
					"message": fmt.Sprintf("项目不存在: %v", err),
				},
			}
		}

		if project.UserID != userID {
			return gin.H{
				"jsonrpc": "2.0",
				"id":      requestID,
				"error": gin.H{
					"code":    -32603,
					"message": "无权限访问此项目",
				},
			}
		}

		projectID := strconv.FormatUint(uint64(projectIDInt), 10)

		// 清理节点修改信息
		var clearedNodes []string
		var errors []string

		for i, nodeIDInterface := range nodeIDs {
			nodeID, ok := nodeIDInterface.(string)
			if !ok {
				errors = append(errors, fmt.Sprintf("Invalid node_id format at index %d", i))
				continue
			}

			// 从数据库中删除节点修改记录
			err := models.DB.Where("project_id = ? AND node_id = ?", projectIDInt, nodeID).Delete(&models.FigmaNode{}).Error
			if err != nil {
				errors = append(errors, fmt.Sprintf("清理节点 %s 失败: %v", nodeID, err))
				continue
			}

			clearedNodes = append(clearedNodes, nodeID)
		}

		resultText := fmt.Sprintf("批量清理节点修改完成 - 项目ID: %s\n成功清理 %d 个节点: %v",
			projectID, len(clearedNodes), clearedNodes)

		if len(errors) > 0 {
			resultText += fmt.Sprintf("\n错误: %v", errors)
		}

		return gin.H{
			"jsonrpc": "2.0",
			"id":      requestID,
			"result": gin.H{
				"content": []gin.H{
					{
						"type": "text",
						"text": resultText,
					},
				},
			},
		}

	case "get_ref_nodes":
		// 处理项目ID参数（支持int和string类型）
		var projectIDInt uint
		if projectIDFloat, ok := arguments["project_id"].(float64); ok {
			projectIDInt = uint(projectIDFloat)
		} else if projectIDStr, ok := arguments["project_id"].(string); ok {
			if parsed, err := strconv.ParseUint(projectIDStr, 10, 32); err == nil {
				projectIDInt = uint(parsed)
			}
		}

		if projectIDInt == 0 {
			return gin.H{
				"jsonrpc": "2.0",
				"id":      requestID,
				"error": gin.H{
					"code":    -32602,
					"message": "Missing or invalid required parameter: project_id",
				},
			}
		}

		// 获取依赖节点列表
		refNodes, err := getProjectRefNodes(userID, projectIDInt)
		if err != nil {
			return gin.H{
				"jsonrpc": "2.0",
				"id":      requestID,
				"error": gin.H{
					"code":    -32603,
					"message": fmt.Sprintf("获取依赖节点列表失败: %v", err),
				},
			}
		}

		// 将依赖节点列表转换为JSON字符串
		refNodesJSON, err := json.Marshal(refNodes)
		if err != nil {
			return gin.H{
				"jsonrpc": "2.0",
				"id":      requestID,
				"error": gin.H{
					"code":    -32603,
					"message": fmt.Sprintf("序列化依赖节点列表失败: %v", err),
				},
			}
		}

		projectID := strconv.FormatUint(uint64(projectIDInt), 10)

		return gin.H{
			"jsonrpc": "2.0",
			"id":      requestID,
			"result": gin.H{
				"content": []gin.H{
					{
						"type": "text",
						"text": fmt.Sprintf("依赖节点列表获取成功 - 项目ID: %s\n\n依赖节点信息:\n%s",
							projectID, string(refNodesJSON)),
					},
				},
			},
		}

	case "get_code_prompts":
		// 获取用户信息
		user, err := models.FindUserByID(userID)
		if err != nil {
			return gin.H{
				"jsonrpc": "2.0",
				"id":      requestID,
				"error": gin.H{
					"code":    -32603,
					"message": fmt.Sprintf("获取用户信息失败: %v", err),
				},
			}
		}

		// 构建返回信息
		resultData := gin.H{
			"code_prompts": user.CodePrompts,
			"comp_types":   user.CompTypes,
		}

		// 将结果转换为JSON字符串
		resultJSON, err := json.Marshal(resultData)
		if err != nil {
			return gin.H{
				"jsonrpc": "2.0",
				"id":      requestID,
				"error": gin.H{
					"code":    -32603,
					"message": fmt.Sprintf("序列化用户配置信息失败: %v", err),
				},
			}
		}

		return gin.H{
			"jsonrpc": "2.0",
			"id":      requestID,
			"result": gin.H{
				"content": []gin.H{
					{
						"type": "text",
						"text": fmt.Sprintf("用户代码生成配置信息获取成功\n\n配置信息:\n%s",
							string(resultJSON)),
					},
				},
			},
		}

	case "get_optimaze_document":
		// 处理项目ID参数
		var projectIDInt uint
		if projectIDFloat, ok := arguments["project_id"].(float64); ok {
			projectIDInt = uint(projectIDFloat)
		} else if projectIDStr, ok := arguments["project_id"].(string); ok {
			if parsed, err := strconv.ParseUint(projectIDStr, 10, 32); err == nil {
				projectIDInt = uint(parsed)
			}
		}

		if projectIDInt == 0 {
			return gin.H{
				"jsonrpc": "2.0",
				"id":      requestID,
				"error": gin.H{
					"code":    -32602,
					"message": "Missing or invalid required parameter: project_id",
				},
			}
		}

		// 获取可选参数：format 和 scale
		format := "png"
		if formatStr, ok := arguments["format"].(string); ok && formatStr != "" {
			format = formatStr
		}

		scale := 1.0
		if scaleInterface := arguments["scale"]; scaleInterface != nil {
			if scaleFloat, ok := scaleInterface.(float64); ok {
				scale = scaleFloat
			} else if scaleStr, ok := scaleInterface.(string); ok {
				if parsedScale, err := strconv.ParseFloat(scaleStr, 64); err == nil {
					scale = parsedScale
				}
			}
		}

		// 获取优化后的文档数据
		metadata, err := services.GetOptimizedDocument(userID, projectIDInt, format, scale)
		if err != nil {
			return gin.H{
				"jsonrpc": "2.0",
				"id":      requestID,
				"error": gin.H{
					"code":    -32603,
					"message": fmt.Sprintf("获取优化文档失败: %v", err),
				},
			}
		}

		// 将结果转换为JSON字符串
		metadataJSON, err := json.MarshalIndent(metadata, "", "  ")
		if err != nil {
			return gin.H{
				"jsonrpc": "2.0",
				"id":      requestID,
				"error": gin.H{
					"code":    -32603,
					"message": fmt.Sprintf("序列化文档数据失败: %v", err),
				},
			}
		}

		projectID := strconv.FormatUint(uint64(projectIDInt), 10)
		return gin.H{
			"jsonrpc": "2.0",
			"id":      requestID,
			"result": gin.H{
				"content": []gin.H{
					{
						"type": "text",
						"text": fmt.Sprintf("优化文档获取成功 - 项目ID: %s, 格式: %s, 缩放: %.1f\n\n文档数据:\n%s",
							projectID, format, scale, string(metadataJSON)),
					},
				},
			},
		}

	default:
		return gin.H{
			"jsonrpc": "2.0",
			"id":      requestID,
			"error": gin.H{
				"code":    -32601,
				"message": "Tool not found: " + toolName,
			},
		}
	}
}

// getMCPPreviewImage 获取MCP预览图并返回base64编码
func getMCPPreviewImage(userID uint, projectIDStr, nodeID, format string, scale float64, ignoreTexts bool) (string, string, error) {
	// 解析项目ID
	projectID, err := strconv.ParseUint(projectIDStr, 10, 64)
	if err != nil {
		return "", "", fmt.Errorf("项目ID无效: %v", err)
	}

	// 获取项目信息
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		return "", "", fmt.Errorf("项目不存在: %v", err)
	}

	// 验证用户权限
	if project.UserID != userID {
		return "", "", fmt.Errorf("无权限访问此项目")
	}

	// 验证格式和缩放参数
	validFormats := map[string]bool{"png": true, "jpg": true, "svg": true}
	if !validFormats[format] {
		format = "png"
	}

	if scale <= 0 || scale > 4.0 {
		scale = 1
	}

	nodeID = strings.ReplaceAll(nodeID, "-", ":")

	// 仅从缓存获取预览图（不访问 Figma API）
	log.Printf("📷 [MCP] 从缓存获取预览图 - 项目ID: %d, 节点ID: %s, 格式: %s, 缩放: %.1f, 忽略文本: %v", projectID, nodeID, format, scale, ignoreTexts)

	imagePath, err := getImageFromCacheOnly(project.FileKey, nodeID, format, scale, ignoreTexts)
	if err != nil {
		return "", "", fmt.Errorf("获取预览图失败（缓存未找到）: %v，请先在 Dashboard 中渲染该节点", err)
	}

	// 读取图片文件并转换为base64
	imageData, err := ioutil.ReadFile(imagePath)
	if err != nil {
		return "", "", fmt.Errorf("读取图片文件失败: %v", err)
	}

	// 编码为base64
	imageBase64 := base64.StdEncoding.EncodeToString(imageData)

	log.Printf("✅ [MCP] 预览图获取成功 - 路径: %s, 大小: %d bytes", imagePath, len(imageData))

	return imagePath, imageBase64, nil
}

// getImageFromCacheOnly 仅从本地缓存、数据库、OBS、CDN 获取图片（不访问 Figma API）
func getImageFromCacheOnly(fileKey, nodeID, format string, scale float64, ignoreTexts bool) (string, error) {
	// 构建本地缓存路径
	safeNodeID := strings.NewReplacer(
		":", "_", ";", "_", "/", "_", "\\", "_",
		"*", "_", "?", "_", "\"", "_", "<", "_",
		">", "_", "|", "_", ",", "_",
	).Replace(nodeID)

	// 构建图片文件名（与渲染队列保持一致）
	scaleStr := fmt.Sprintf("%.1fx", scale)

	// 根据ignoreTexts决定文件名后缀：x表示带文字，p表示不带文字
	suffix := "x"
	if ignoreTexts {
		suffix = "p"
	}

	// 检查本地缓存目录（包括带后缀的新格式和旧格式）
	cacheDirs := []string{
		fmt.Sprintf("temp/%s/%s/%s", fileKey, format, scaleStr),
		fmt.Sprintf("temp/images/%s/%s_%s", fileKey, format, scaleStr),
		fmt.Sprintf("temp/%s/previews", fileKey),
	}

	// 1. 先检查本地文件缓存（尝试新格式：带hash和suffix）
	for _, cacheDir := range cacheDirs {
		// 列出目录中的文件，查找匹配的图片
		if files, err := ioutil.ReadDir(cacheDir); err == nil {
			for _, file := range files {
				fileName := file.Name()
				// 匹配格式: nodeID-scale[x|p]-hash.format
				if strings.HasPrefix(fileName, safeNodeID+"-"+scaleStr+suffix+"-") && strings.HasSuffix(fileName, "."+format) {
					localPath := filepath.Join(cacheDir, fileName)
					log.Printf("✅ [MCP] 从本地缓存获取图片(新格式): %s", localPath)
					return localPath, nil
				}
			}
		}

		// 回退到旧格式：nodeID-scale.format（不带后缀）
		oldFileName := fmt.Sprintf("%s-%s.%s", safeNodeID, scaleStr, format)
		oldLocalPath := filepath.Join(cacheDir, oldFileName)
		if fileInfo, err := ioutil.ReadFile(oldLocalPath); err == nil && len(fileInfo) > 0 {
			log.Printf("✅ [MCP] 从本地缓存获取图片(旧格式): %s", oldLocalPath)
			return oldLocalPath, nil
		}
	}

	// 2. 查询数据库中的图片记录
	var imageCache models.FigmaNodeImage
	err := models.DB.Where("file_key = ? AND node_id = ? AND format = ? AND scale = ?",
		fileKey, nodeID, format, scale).First(&imageCache).Error

	if err == nil && (imageCache.Status == "obs_synced" || imageCache.Status == "figma_cdn") {
		// 生成下载后的文件名（使用旧格式）
		downloadFileName := fmt.Sprintf("%s-%s.%s", safeNodeID, scaleStr, format)

		// 2.1 如果有 OBS Key，尝试从 OBS 下载
		if imageCache.OBSKey != "" && globalOBSService != nil && globalOBSService.IsEnabled() {
			log.Printf("🔄 [MCP] 尝试从 OBS 下载图片: %s", imageCache.OBSKey)
			imageData, _, err := globalOBSService.DownloadImage(imageCache.OBSKey)
			if err == nil && len(imageData) > 0 {
				// 保存到本地缓存
				localPath := filepath.Join(cacheDirs[0], downloadFileName)
				if err := os.MkdirAll(cacheDirs[0], 0755); err == nil {
					if err := ioutil.WriteFile(localPath, imageData, 0644); err == nil {
						log.Printf("✅ [MCP] 从 OBS 下载并缓存图片成功: %s", localPath)
						return localPath, nil
					}
				}
			} else {
				log.Printf("⚠️ [MCP] 从 OBS 下载图片失败: %v", err)
			}
		}

		// 2.2 如果有 Figma CDN URL，尝试从 CDN 下载
		if imageCache.FigmaCDNURL != "" {
			log.Printf("🔄 [MCP] 尝试从 Figma CDN 下载图片: %s", imageCache.FigmaCDNURL)
			resp, err := http.Get(imageCache.FigmaCDNURL)
			if err == nil && resp.StatusCode == http.StatusOK {
				defer resp.Body.Close()
				imageData, err := ioutil.ReadAll(resp.Body)
				if err == nil && len(imageData) > 0 {
					// 保存到本地缓存
					localPath := filepath.Join(cacheDirs[0], downloadFileName)
					if err := os.MkdirAll(cacheDirs[0], 0755); err == nil {
						if err := ioutil.WriteFile(localPath, imageData, 0644); err == nil {
							log.Printf("✅ [MCP] 从 Figma CDN 下载并缓存图片成功: %s", localPath)
							return localPath, nil
						}
					}
				}
			} else {
				log.Printf("⚠️ [MCP] 从 Figma CDN 下载图片失败: status=%v, err=%v",
					resp.StatusCode, err)
			}
		}
	}

	// 所有缓存都未找到
	return "", fmt.Errorf("图片缓存未找到 (fileKey=%s, nodeID=%s, format=%s, scale=%.1f, ignoreTexts=%v)",
		fileKey, nodeID, format, scale, ignoreTexts)
}

// getProjectRefNodes 获取项目中的依赖节点列表
func getProjectRefNodes(userID uint, projectIDInt uint) (map[string]interface{}, error) {
	// 获取项目信息
	project, err := models.GetProjectByID(projectIDInt)
	if err != nil {
		return nil, fmt.Errorf("项目不存在: %v", err)
	}

	// 验证用户权限
	if project.UserID != userID {
		return nil, fmt.Errorf("无权限访问此项目")
	}

	// 直接从数据库获取依赖节点列表
	refNodeIDs, err := models.GetRefNodes(projectIDInt)
	if err != nil {
		return nil, fmt.Errorf("获取依赖节点列表失败: %v", err)
	}

	// 构建返回结果
	result := map[string]interface{}{
		"ref_nodes": refNodeIDs,
		"summary": map[string]interface{}{
			"total_ref_nodes": len(refNodeIDs),
			"project_id":      projectIDInt,
			"project_name":    project.Name,
			"file_key":        project.FileKey,
		},
	}

	return result, nil
}

// getMCPPreviewImageWithWebSocket 获取MCP预览图，优先使用 WebSocket
func getMCPPreviewImageWithWebSocket(userID uint, projectIDStr, nodeID, format string, scale float64, ignoreTexts bool, ignoreNodes []string) (string, string, error) {
	// 解析项目ID
	projectID, err := strconv.ParseUint(projectIDStr, 10, 64)
	if err != nil {
		return "", "", fmt.Errorf("项目ID无效: %v", err)
	}

	// 获取项目信息
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		return "", "", fmt.Errorf("项目不存在: %v", err)
	}

	// 验证用户权限
	if project.UserID != userID {
		return "", "", fmt.Errorf("无权限访问此项目")
	}

	// 验证格式和缩放参数
	validFormats := map[string]bool{"png": true, "jpg": true, "svg": true}
	if !validFormats[format] {
		format = "png"
	}

	if scale <= 0 || scale > 4.0 {
		scale = 1
	}

	nodeID = strings.ReplaceAll(nodeID, "-", ":")

	// 1. 优先检查 WebSocket 是否可用
	log.Printf("📷 [MCP] 优先使用 WebSocket 获取预览图 - 项目ID: %d, 节点ID: %s, 格式: %s, 缩放: %.1f, 忽略文本: %v",
		projectID, nodeID, format, scale, ignoreTexts)

	mcpService := services.GetMCPService()
	if mcpService != nil {
		connection, exists := mcpService.GetUserConnection(userID)
		if exists && connection != nil && mcpService.HasActiveWebSocketConnection(connection.ConnectionID) {
			log.Printf("✅ [MCP] WebSocket 连接存在，优先使用 WebSocket 获取最新数据")

			// 构建临时目录路径
			safeNodeID := strings.NewReplacer(
				":", "_", ";", "_", "/", "_", "\\", "_",
				"*", "_", "?", "_", "\"", "_", "<", "_",
				">", "_", "|", "_", ",", "_",
			).Replace(nodeID)

			tempDir := filepath.Join("temp", project.FileKey, "previews")

			// 通过 WebSocket 获取图片
			imagePath, err := services.TryGetImageViaWebSocket(
				project.FileKey, nodeID, format, scale, userID, tempDir, safeNodeID, ignoreTexts, ignoreNodes)

			if err == nil && imagePath != "" {
				log.Printf("✅ [MCP] 成功通过 WebSocket 获取最新图片: %s", imagePath)

				// 读取图片文件并转换为base64
				imageData, err := ioutil.ReadFile(imagePath)
				if err != nil {
					return "", "", fmt.Errorf("读取图片文件失败: %v", err)
				}

				// 编码为base64
				imageBase64 := base64.StdEncoding.EncodeToString(imageData)

				return imagePath, imageBase64, nil
			}
			log.Printf("⚠️ [MCP] WebSocket 获取失败: %v，尝试降级到缓存", err)
		} else {
			log.Printf("⚠️ [MCP] WebSocket 连接不存在，使用缓存数据")
		}
	}

	// 2. WebSocket 不可用或失败，从缓存获取预览图
	log.Printf("📷 [MCP] 从缓存获取预览图 - 项目ID: %d, 节点ID: %s, 格式: %s, 缩放: %.1f, 忽略文本: %v",
		projectID, nodeID, format, scale, ignoreTexts)

	imagePath, err := getImageFromCacheOnly(project.FileKey, nodeID, format, scale, ignoreTexts)
	if err != nil {
		return "", "", fmt.Errorf("获取预览图失败（缓存未找到）: %v，请先在 Dashboard 中渲染该节点", err)
	}

	// 读取图片文件并转换为base64
	imageData, err := ioutil.ReadFile(imagePath)
	if err != nil {
		return "", "", fmt.Errorf("读取图片文件失败: %v", err)
	}

	// 编码为base64
	imageBase64 := base64.StdEncoding.EncodeToString(imageData)

	log.Printf("✅ [MCP] 预览图获取成功 - 路径: %s, 大小: %d bytes", imagePath, len(imageData))

	return imagePath, imageBase64, nil
}

// getProjectNodeTreeWithWebSocket 获取项目的节点树信息，支持 WebSocket 兜底
func getProjectNodeTreeWithWebSocket(userID uint, projectIDInt uint, nodeID string) (map[string]interface{}, error) {
	// 获取项目信息
	project, err := models.GetProjectByID(projectIDInt)
	if err != nil {
		return nil, fmt.Errorf("项目不存在: %v", err)
	}

	// 验证用户权限
	if project.UserID != userID {
		return nil, fmt.Errorf("无权限访问此项目")
	}

	// 如果没有提供节点ID，使用项目的根节点ID
	if nodeID == "" {
		if project.RootNodeID != "" {
			nodeID = project.RootNodeID
		} else {
			nodeID = "0:0"
		}
	}

	// 处理节点ID格式：将 '-' 转换为 ':'
	nodeID = strings.ReplaceAll(nodeID, "-", ":")

	// 1. 首先尝试从缓存获取节点树数据
	nodes, err := getNodesFromCache(project.FileKey, nodeID)
	if err == nil && len(nodes) > 0 {
		log.Printf("✅ [MCP] 从缓存获取节点树成功: fileKey=%s, nodeID=%s, 节点数=%d",
			project.FileKey, nodeID, len(nodes))

		// 获取所有节点的修改信息
		var figmaNodes []models.FigmaNode
		dbResult := models.DB.Where("project_id = ?", projectIDInt).Find(&figmaNodes)
		if dbResult.Error != nil {
			return nil, fmt.Errorf("获取节点修改信息失败: %v", dbResult.Error)
		}

		// 构建节点ID到修改信息的映射
		nodeModifys := make(map[string]map[string]interface{})
		for _, node := range figmaNodes {
			if node.Modifys != "" {
				var modifyData map[string]interface{}
				if err := json.Unmarshal([]byte(node.Modifys), &modifyData); err == nil {
					nodeModifys[node.NodeID] = modifyData
				}
			}
		}

		// 过滤并构建节点树
		filteredTree := buildFilteredNodeTree(nodes, nodeID, nodeModifys)

		return filteredTree, nil
	}

	log.Printf("⚠️ [MCP] 缓存未找到节点树数据，尝试 WebSocket 兜底")

	// 2. 缓存未找到，检查 WebSocket 连接状态并通过连接获取数据
	mcpService := services.GetMCPService()
	if mcpService == nil {
		return nil, fmt.Errorf("获取节点树数据失败（缓存未找到）: MCP 服务未初始化，请先在 Dashboard 中刷新节点树")
	}

	connection, exists := mcpService.GetUserConnection(userID)
	if !exists || connection == nil || !mcpService.HasActiveWebSocketConnection(connection.ConnectionID) {
		return nil, fmt.Errorf("获取节点树数据失败（缓存未找到）: 没有活跃的 WebSocket 连接，请先在 Dashboard 中刷新节点树")
	}

	log.Printf("🔌 [MCP] 发现活跃 WebSocket 连接，尝试通过插件获取节点树数据")

	// 使用 services 包中的 GetFigmaNodesWithUser 函数
	// 该函数会通过 WebSocket 请求 Figma 插件获取数据，并自动保存到缓存
	token := "" // MCP 调用不需要 Figma API Token
	nodes, err = services.GetFigmaNodesWithUser(token, project.FileKey, nodeID, userID)
	if err != nil {
		return nil, fmt.Errorf("通过 WebSocket 获取节点树失败: %v", err)
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("节点树数据为空（请检查节点ID是否正确）")
	}

	log.Printf("✅ [MCP] 通过 WebSocket 获取节点树成功: 节点数=%d", len(nodes))

	// 获取所有节点的修改信息
	var figmaNodes []models.FigmaNode
	dbResult := models.DB.Where("project_id = ?", projectIDInt).Find(&figmaNodes)
	if dbResult.Error != nil {
		return nil, fmt.Errorf("获取节点修改信息失败: %v", dbResult.Error)
	}

	// 构建节点ID到修改信息的映射
	nodeModifys := make(map[string]map[string]interface{})
	for _, node := range figmaNodes {
		if node.Modifys != "" {
			var modifyData map[string]interface{}
			if err := json.Unmarshal([]byte(node.Modifys), &modifyData); err == nil {
				nodeModifys[node.NodeID] = modifyData
			}
		}
	}

	// 过滤并构建节点树
	filteredTree := buildFilteredNodeTree(nodes, nodeID, nodeModifys)

	return filteredTree, nil
}
