package controllers

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/figma-deliver/internal/middleware"
	"github.com/figma-deliver/internal/models"
	"github.com/figma-deliver/internal/services"
	"github.com/gin-gonic/gin"
	"golang.org/x/net/proxy"
)

// formatScaleForFilename 格式化缩放等级用于文件名
// 如果是整数（如1.0, 2.0），去掉".0"，使用整数部分（如"1", "2"）
// 如果是非整数（如0.5, 1.5），保持小数点格式（如"0.5", "1.5"）
func formatScaleForFilename(scale float64) string {
	// 如果是整数（如1.0, 2.0），去掉".0"，使用整数部分
	if scale == float64(int64(scale)) {
		return fmt.Sprintf("%.0f", scale)
	}
	// 如果是非整数，保持小数点格式
	return fmt.Sprintf("%.1f", scale)
}

// APIGetUserByToken 通过MCP token反查用户ID
func APIGetUserByToken(c *gin.Context) {
	mcpToken := c.Param("mcptoken")
	if mcpToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "MCP token不能为空",
		})
		return
	}

	// 通过MCP token查找用户
	var user models.User
	err := models.DB.Where("mcp_token = ? AND mcp_token != ''", mcpToken).First(&user).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "无效的MCP token或用户不存在",
		})
		return
	}

	// 返回用户基本信息
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"user_id":    user.ID,
			"username":   user.Username,
			"created_at": user.CreatedAt,
		},
		"message": "用户信息获取成功",
	})
}

// GetProjectsByFileKey 获取相同file_key下的所有项目
func GetProjectsByFileKey(c *gin.Context) {
	// 从会话中获取用户ID
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "用户未登录",
		})
		return
	}

	fileKey := c.Query("file_key")
	if fileKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "file_key参数不能为空",
		})
		return
	}

	// 查找相同file_key下的所有项目
	var projects []models.FigmaProject
	err := models.DB.Where("user_id = ? AND file_key = ?", userID, fileKey).
		Order("created_at DESC").Find(&projects).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取项目列表失败: " + err.Error(),
		})
		return
	}

	// 格式化时间
	for i := range projects {
		projects[i].CreatedAt = projects[i].CreatedAt.Local()
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    projects,
		"message": "获取项目列表成功",
	})
}

// GetProjectsByGroup 获取相同分组下的所有项目
func GetProjectsByGroup(c *gin.Context) {
	// 从会话中获取用户ID
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "用户未登录",
		})
		return
	}

	groupName := c.Query("group_name")
	fileKey := c.Query("file_key")

	// 如果没有提供分组名称和file_key，返回错误
	if groupName == "" && fileKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "group_name或file_key参数不能同时为空",
		})
		return
	}

	var projects []models.FigmaProject
	var err error

	if groupName != "" {
		// 如果有分组名称，查找相同分组的所有项目
		err = models.DB.Where("user_id = ? AND group_name = ?", userID, groupName).
			Order("created_at DESC").Find(&projects).Error
	} else {
		// 如果没有分组名称，查找相同file_key且group_name为空的项目
		err = models.DB.Where("user_id = ? AND file_key = ? AND (group_name = '' OR group_name IS NULL)", userID, fileKey).
			Order("created_at DESC").Find(&projects).Error
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取项目列表失败: " + err.Error(),
		})
		return
	}

	// 格式化时间
	for i := range projects {
		projects[i].CreatedAt = projects[i].CreatedAt.Local()
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    projects,
		"message": "获取项目列表成功",
	})
}

// APIGetOptimizedNodes 获取优化后的节点数据（支持3档简化级别）
func APIGetOptimizedNodes(c *gin.Context) {
	mcpToken := c.Param("mcptoken")
	fileKey := c.Query("file_key")
	rootNodeID := c.Query("root_node_id")
	levelStr := c.DefaultQuery("level", "1") // 默认使用1档位

	if mcpToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "MCP token不能为空",
		})
		return
	}

	if fileKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "file_key参数不能为空",
		})
		return
	}

	if rootNodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "root_node_id参数不能为空",
		})
		return
	}

	// 解析简化级别（0、1、2）
	level, err := strconv.Atoi(levelStr)
	if err != nil || level < 0 || level > 2 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "level参数必须是0、1或2",
		})
		return
	}

	// 通过MCP token查找用户
	var user models.User
	err = models.DB.Where("mcp_token = ? AND mcp_token != ''", mcpToken).First(&user).Error
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "无效的MCP token或用户不存在",
		})
		return
	}

	// 查找或创建项目
	project, err := services.GetOrCreateFigmaProject(user.ID, fileKey, rootNodeID, "", "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取项目信息失败: " + err.Error(),
		})
		return
	}

	// 获取优化后的节点数据
	optimizedData, err := getOptimizedProjectData(&user, project, fileKey, rootNodeID, level)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取优化数据失败: " + err.Error(),
		})
		return
	}

	// 直接返回优化数据，不包装在响应对象中
	c.JSON(http.StatusOK, optimizedData)
}

// getOptimizedProjectData 获取优化后的项目数据
// level: 0=不砍任何原数据, 1=过滤空值(当前逻辑), 2=更激进的简化
func getOptimizedProjectData(user *models.User, project *models.FigmaProject, fileKey, rootNodeID string, level int) (gin.H, error) {
	// 获取节点树数据（支持 WebSocket 兜底，会自动等待响应）
	nodes, err := services.GetFigmaNodesWithUser(user.FigmaToken, fileKey, rootNodeID, user.ID)
	if err != nil {
		return nil, fmt.Errorf("获取节点树失败: %v", err)
	}

	// 获取所有节点的修改信息
	var figmaNodes []models.FigmaNode
	dbResult := models.DB.Where("project_id = ?", project.ID).Find(&figmaNodes)
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

	// 首先过滤掉需要移除的节点
	filteredNodes := filterVisibleAndNonIgnoredNodes(nodes, nodeModifys)

	// 构建优化后的节点数据
	optimizedNodes := make([]gin.H, 0)
	nodeMap := make(map[string]gin.H)

	for _, node := range filteredNodes {
		nodeID := node["id"].(string)

		// 创建优化节点，包含所有Figma原始数据
		optimizedNode := gin.H{}

		// 复制所有Figma原始属性（过滤空值，但跳过children字段）
		for key, value := range node {
			// 跳过children字段，因为我们稍后会重新构建它
			if key == "children" {
				continue
			}
			if !isEmptyValue(value) {
				optimizedNode[key] = value
			}
		}

		// 应用修改信息（如果存在）
		if modifys, exists := nodeModifys[nodeID]; exists {
			applyModificationsToNode(optimizedNode, modifys)
		}

		optimizedNodes = append(optimizedNodes, optimizedNode)
		nodeMap[nodeID] = optimizedNode
	}

	// 根据res_mode处理节点结构
	optimizedNodes = processNodesByResMode(optimizedNodes, nodeModifys)

	// 构建节点映射和子节点映射
	nodeMap = make(map[string]gin.H)
	childrenMap := make(map[string][]gin.H)

	// 先建立所有节点的映射
	for _, node := range optimizedNodes {
		nodeID := node["id"].(string)
		nodeMap[nodeID] = node
	}

	// 构建父子关系
	for _, node := range optimizedNodes {
		parentID := node["parent_id"]

		if parentID != nil && parentID != rootNodeID {
			if parentIDStr, ok := parentID.(string); ok {
				childrenMap[parentIDStr] = append(childrenMap[parentIDStr], node)
			}
		}
	}

	// 创建处理后节点的ID集合，用于快速查找
	validNodeIDs := make(map[string]bool)
	for _, node := range optimizedNodes {
		nodeID := node["id"].(string)
		validNodeIDs[nodeID] = true
	}

	// 为每个节点添加子节点信息
	for i := range optimizedNodes {
		nodeID := optimizedNodes[i]["id"].(string)

		// 清除节点原有的children字段，使用我们重新构建的children
		delete(optimizedNodes[i], "children")

		if children, hasChildren := childrenMap[nodeID]; hasChildren && len(children) > 0 {
			// 只保留仍然存在的子节点
			validChildren := make([]gin.H, 0)
			for _, child := range children {
				childID := child["id"].(string)
				if validNodeIDs[childID] {
					validChildren = append(validChildren, child)
				}
			}

			if len(validChildren) > 0 {
				// 添加有效的子节点
				optimizedNodes[i]["children"] = validChildren
			}
		}
		// 如果没有子节点或子节点为空，则不添加children字段
	}

	// 只返回根节点（从根节点开始的树形结构）
	var rootNode gin.H
	for i, node := range optimizedNodes {
		parentID := node["parent_id"]
		if parentID == nil || parentID == rootNodeID {
			// 根据简化级别处理节点
			switch level {
			case 0:
				// 0档位：不砍任何原数据，保留所有字段（包括空值）
				rootNode = optimizedNodes[i]
			case 1:
				// 1档位：过滤空值（当前逻辑）
				rootNode = filterEmptyValuesFromNode(optimizedNodes[i])
			case 2:
				// 2档位：更激进的简化，只保留核心字段
				rootNode = simplifyNodeAggressively(optimizedNodes[i])
			default:
				// 默认使用1档位
				rootNode = filterEmptyValuesFromNode(optimizedNodes[i])
			}
			break // 只取第一个根节点
		}
	}

	// 统计信息
	stats := gin.H{
		"totalNodes":     len(nodes),
		"visibleNodes":   0,
		"hiddenNodes":    0,
		"ignoredNodes":   0,
		"textNodes":      0,
		"frameNodes":     0,
		"componentNodes": 0,
	}

	for _, node := range optimizedNodes {
		nodeType, _ := node["type"].(string)

		// 安全地获取visible属性
		visible := true // 默认可见
		if visibleVal, ok := node["visible"]; ok && visibleVal != nil {
			if visibleBool, ok := visibleVal.(bool); ok {
				visible = visibleBool
			}
		}

		// ignored节点已经在过滤阶段被移除，这里统计的都是未被忽略的节点
		ignored := false

		if ignored {
			stats["ignoredNodes"] = stats["ignoredNodes"].(int) + 1
		} else if visible {
			stats["visibleNodes"] = stats["visibleNodes"].(int) + 1
		} else {
			stats["hiddenNodes"] = stats["hiddenNodes"].(int) + 1
		}

		switch nodeType {
		case "TEXT":
			stats["textNodes"] = stats["textNodes"].(int) + 1
		case "FRAME":
			stats["frameNodes"] = stats["frameNodes"].(int) + 1
		case "COMPONENT", "COMPONENT_SET":
			stats["componentNodes"] = stats["componentNodes"].(int) + 1
		}
	}

	// 返回优化后的数据结构（只包含单个根节点，保持树形结构）
	return gin.H{
		"project": gin.H{
			"id":         project.ID,
			"fileKey":    project.FileKey,
			"rootNodeId": project.RootNodeID,
			"name":       project.Name,
			"figmaUrl":   project.FigmaURL,
			"createdAt":  project.CreatedAt,
			"updatedAt":  project.UpdatedAt,
		},
		"node":       rootNode, // 单个根节点，包含完整的子树
		"statistics": stats,
		"metadata": gin.H{
			"generatedAt": "now",
			"version":     "1.0",
			"userId":      user.ID,
			"username":    user.Username,
		},
	}, nil
}

// APIGetProjectNodes 获取项目节点列表
func APIGetProjectNodes(c *gin.Context) {
	mcpToken := c.Param("mcptoken")
	projectIDStr := c.Query("project_id")

	if mcpToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "MCP token不能为空",
		})
		return
	}

	if projectIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "project_id参数不能为空",
		})
		return
	}

	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "project_id参数格式错误",
		})
		return
	}

	// 通过MCP token查找用户
	var user models.User
	err = models.DB.Where("mcp_token = ? AND mcp_token != ''", mcpToken).First(&user).Error
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "无效的MCP token或用户不存在",
		})
		return
	}

	// 获取项目信息
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在",
		})
		return
	}

	// 检查项目是否属于该用户
	if project.UserID != user.ID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "无权访问该项目",
		})
		return
	}

	// 获取项目的所有节点修改信息
	var figmaNodes []models.FigmaNode
	dbResult := models.DB.Where("project_id = ?", projectID).Find(&figmaNodes)
	if dbResult.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取节点信息失败: " + dbResult.Error.Error(),
		})
		return
	}

	// 构建节点列表
	nodeList := make([]gin.H, 0)
	for _, node := range figmaNodes {
		nodeInfo := gin.H{
			"id":        node.ID,
			"nodeId":    node.NodeID,
			"projectId": node.ProjectID,
			"createdAt": node.CreatedAt,
			"updatedAt": node.UpdatedAt,
		}

		// 解析修改信息
		if node.Modifys != "" {
			var modifyData map[string]interface{}
			if err := json.Unmarshal([]byte(node.Modifys), &modifyData); err == nil {
				nodeInfo["modifys"] = modifyData
			}
		}

		nodeList = append(nodeList, nodeInfo)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"project": gin.H{
				"id":         project.ID,
				"fileKey":    project.FileKey,
				"rootNodeId": project.RootNodeID,
				"name":       project.Name,
				"figmaUrl":   project.FigmaURL,
			},
			"nodes": nodeList,
			"count": len(nodeList),
		},
		"message": "节点列表获取成功",
	})
}

// APIDownloadFigmaImage 下载Figma图片并直接返回给客户端
func APIDownloadFigmaImage(c *gin.Context) {
	mcpToken := c.Param("mcptoken")
	fileKey := c.Query("file_key")
	nodeIDs := c.Query("node_ids")

	// 参数验证
	if mcpToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "MCP token不能为空",
		})
		return
	}

	if fileKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "file_key参数不能为空",
		})
		return
	}

	if nodeIDs == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "node_ids参数不能为空",
		})
		return
	}

	// 通过MCP token查找用户
	var user models.User
	err := models.DB.Where("mcp_token = ? AND mcp_token != ''", mcpToken).First(&user).Error
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "无效的MCP token或用户不存在",
		})
		return
	}

	// 检查用户是否有Figma token
	if user.FigmaToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请先在个人资料页面配置 Figma Token",
		})
		return
	}

	// 获取可选参数
	format := c.DefaultQuery("format", "png")
	scaleStr := c.DefaultQuery("scale", "1.0")
	ignoreTextsStr := c.DefaultQuery("ignore_texts", "false")
	ignoreNodesStr := c.DefaultQuery("ignore_nodes", "")

	// 解析缩放比例
	scale, err := strconv.ParseFloat(scaleStr, 64)
	if err != nil || scale <= 0 || scale > 4.0 {
		scale = 1.0
	}

	// 解析是否忽略文字
	ignoreTexts := false
	if ignoreTextsStr == "true" || ignoreTextsStr == "1" {
		ignoreTexts = true
	}

	// 解析要忽略的节点列表
	var ignoreNodes []string
	if ignoreNodesStr != "" {
		// 支持逗号分隔的节点ID列表
		ignoreNodes = strings.Split(ignoreNodesStr, ",")
		// 去除空白字符
		for i, nodeID := range ignoreNodes {
			ignoreNodes[i] = strings.TrimSpace(nodeID)
		}
		// 过滤掉空字符串
		var validIgnoreNodes []string
		for _, nodeID := range ignoreNodes {
			if nodeID != "" {
				validIgnoreNodes = append(validIgnoreNodes, nodeID)
			}
		}
		ignoreNodes = validIgnoreNodes
	}

	// 验证格式
	validFormats := map[string]bool{"png": true, "jpg": true, "svg": true}
	if !validFormats[format] {
		format = "png"
	}

	// 使用缓存机制下载图片
	imagePath, err := downloadFigmaImageWithCache(user.ID, user.FigmaToken, fileKey, nodeIDs, format, scale, ignoreTexts, ignoreNodes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "下载图片失败: " + err.Error(),
		})
		return
	}

	// 读取缓存的图片文件
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "读取图片文件失败: " + err.Error(),
		})
		return
	}

	// 根据文件扩展名确定Content-Type
	contentType := getContentTypeByExtension(filepath.Ext(imagePath))

	// 设置响应头
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"figma_%s_%s.%s\"", fileKey, strings.ReplaceAll(nodeIDs, ",", "_"), format))
	c.Header("Cache-Control", "public, max-age=3600") // 缓存1小时

	// 直接返回图片数据
	c.Data(http.StatusOK, contentType, imageData)
}

// APIClearNodeCache 根据JSON数据中的节点ID清除对应的缓存文件
func APIClearNodeCache(c *gin.Context) {
	mcpToken := c.Param("mcptoken")
	fileKey := c.Query("file_key")
	rootNodeID := c.Query("root_node_id")

	// 参数验证
	if mcpToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "MCP token不能为空",
		})
		return
	}

	if fileKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "file_key参数不能为空",
		})
		return
	}

	if rootNodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "root_node_id参数不能为空",
		})
		return
	}

	// 通过MCP token查找用户
	var user models.User
	err := models.DB.Where("mcp_token = ? AND mcp_token != ''", mcpToken).First(&user).Error
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "无效的MCP token或用户不存在",
		})
		return
	}

	// 获取项目的优化JSON数据以提取所有节点ID
	project, err := services.GetOrCreateFigmaProject(user.ID, fileKey, rootNodeID, "", "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取项目信息失败: " + err.Error(),
		})
		return
	}

	// 获取优化后的JSON数据（使用默认1档位）
	optimizedData, err := getOptimizedProjectData(&user, project, fileKey, rootNodeID, 1)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取项目数据失败: " + err.Error(),
		})
		return
	}

	// 从JSON数据中提取所有节点ID
	nodeIDs, err := extractAllNodeIDs(optimizedData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "提取节点ID失败: " + err.Error(),
		})
		return
	}

	// 清除指定节点ID的缓存文件
	clearedCount, err := clearCacheByNodeIDs(fileKey, nodeIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "清除缓存失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("成功清除 %d 个缓存文件", clearedCount),
		"data": gin.H{
			"cleared_count": clearedCount,
			"node_count":    len(nodeIDs),
			"file_key":      fileKey,
		},
	})
}

// downloadFigmaImageWithCache 使用缓存机制下载Figma图片
func downloadFigmaImageWithCache(userID uint, _ /*token*/, fileKey, nodeIDs, format string, scale float64, ignoreTexts bool, ignoreNodes []string) (string, error) {
	fmt.Printf("开始下载Figma图片（带缓存）: fileKey=%s, nodeIDs=%s, format=%s, scale=%.1f, ignoreTexts=%v, ignoreNodes=%v\n",
		fileKey, nodeIDs, format, scale, ignoreTexts, ignoreNodes)

	// 创建临时文件夹
	tempDir := filepath.Join("temp", fileKey, "previews")
	err := os.MkdirAll(tempDir, os.ModePerm)
	if err != nil {
		fmt.Printf("创建临时文件夹失败: %v\n", err)
		return "", fmt.Errorf("创建临时文件夹失败: %v", err)
	}

	// 生成安全的文件名 - 替换特殊字符为下划线
	safeNodeIDs := strings.NewReplacer(":", "_", ";", "_", "/", "_", "\\", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_", ",", "_").Replace(nodeIDs)

	// 步骤1：优先检查 WebSocket 是否可用，如果可用则使用最新数据
	fmt.Printf("步骤1：检查 WebSocket 连接状态 (userID=%d)\n", userID)
	mcpService := services.GetMCPService()
	if mcpService != nil {
		connection, exists := mcpService.GetUserConnection(userID)
		if exists && connection != nil && mcpService.HasActiveWebSocketConnection(connection.ConnectionID) {
			fmt.Printf("✅ WebSocket 连接存在，优先使用 WebSocket 获取最新图片数据 (ignoreTexts=%v)\n", ignoreTexts)

			// 通过WebSocket调用Figma插件导出图片
			imagePath, err := services.TryGetImageViaWebSocket(fileKey, nodeIDs, format, scale, userID, tempDir, safeNodeIDs, ignoreTexts, ignoreNodes)
			if err == nil {
				fmt.Printf("✅ 成功通过 WebSocket 获取最新图片: %s\n", imagePath)
				// WebSocket 成功获取，handleImageDataFromPlugin 已经自动更新了 OBS、数据库和本地文件
				return imagePath, nil
			}
			fmt.Printf("⚠️ WebSocket 获取失败: %v，尝试降级到缓存\n", err)
		} else {
			fmt.Printf("⚠️ WebSocket 连接不存在，使用缓存数据\n")
		}
	}

	// 步骤2：WebSocket 不可用或失败，检查本地缓存文件
	existingFile := findLatestNodeImageWithIgnoreTextsAndNodes(tempDir, safeNodeIDs, scale, ignoreTexts, ignoreNodes)
	if existingFile != "" {
		fmt.Printf("✅ 找到节点 %s 的本地缓存图片: %s\n", nodeIDs, existingFile)
		return existingFile, nil
	}
	fmt.Printf("⚠️ 本地缓存未找到\n")

	// 步骤3：从数据库查找华为云OBS URL
	// 注意：当前数据库模型不支持ignore_nodes字段，所以如果有ignore_nodes参数，跳过数据库缓存
	fmt.Printf("步骤3：从数据库查找华为云OBS URL\n")
	var nodeImage models.FigmaNodeImage
	if len(ignoreNodes) > 0 {
		// 如果有忽略节点参数，跳过数据库缓存查询，因为数据库模型暂不支持ignore_nodes
		fmt.Printf("⚠️ 检测到ignore_nodes参数，跳过数据库缓存查询\n")
		err = fmt.Errorf("ignore_nodes参数不支持数据库缓存")
	} else {
		err = models.DB.Where("file_key = ? AND node_id = ? AND format = ? AND scale = ? AND ignore_texts = ? AND status IN ('obs_synced', 'figma_cdn')",
			fileKey, nodeIDs, format, scale, ignoreTexts).First(&nodeImage).Error
	}

	if err == nil && nodeImage.OBSKey != "" {
		// 找到了OBS记录，尝试从OBS下载
		fmt.Printf("📦 找到OBS记录: obsKey=%s, status=%s\n", nodeImage.OBSKey, nodeImage.Status)

		// 尝试从OBS下载（使用全局OBS服务）
		imagePath, err := services.DownloadImageFromOBS(nodeImage.OBSKey, tempDir, safeNodeIDs, format, scale, ignoreTexts)
		if err == nil {
			fmt.Printf("✅ 从OBS下载成功: %s\n", imagePath)
			return imagePath, nil
		}
		fmt.Printf("⚠️ 从OBS下载失败: %v\n", err)
	} else if err != nil {
		fmt.Printf("⚠️ 数据库未找到图片记录: %v\n", err)
	}

	// 步骤4：所有途径都失败
	fmt.Printf("❌ 所有获取途径都失败\n")
	return "", fmt.Errorf("无法获取图片：WebSocket不可用，本地缓存不存在，数据库无OBS记录")
}

// getFigmaImageURL 通过Figma API获取图片URL
func getFigmaImageURL(token, fileKey, nodeIDs, format string, scale float64) (string, error) {
	// 构建API URL
	apiURL := fmt.Sprintf("https://api.figma.com/v1/images/%s?ids=%s&format=%s&scale=%.1f",
		fileKey, nodeIDs, format, scale)

	// 创建请求
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建API请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("X-Figma-Token", token)

	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送API请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API请求失败: %s", resp.Status)
	}

	// 解析响应
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取API响应失败: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return "", fmt.Errorf("解析API响应JSON失败: %v", err)
	}

	// 获取图片URL
	images, ok := result["images"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("解析图片信息失败，未找到images字段")
	}

	// 获取第一个可用的图片URL
	for _, url := range images {
		if urlStr, ok := url.(string); ok && urlStr != "" {
			return urlStr, nil
		}
	}

	return "", fmt.Errorf("未找到有效的图片URL")
}

// downloadAndCacheImage 下载图片并缓存到本地
func downloadAndCacheImage(imageURL, tempDir, safeNodeIDs, format string, scale float64) (string, error) {
	fmt.Printf("开始下载图片: %s\n", imageURL)

	// 创建带超时的HTTP客户端
	client := &http.Client{Timeout: 30 * time.Second}

	// 创建请求
	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建图片下载请求失败: %v", err)
	}

	// 添加请求头
	req.Header.Set("User-Agent", "Mozilla/5.0 FigmaDeliver/1.0")

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载图片请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载图片失败: %s", resp.Status)
	}

	// 检查Content-Type并确定文件扩展名
	contentType := resp.Header.Get("Content-Type")
	fileExt := getFileExtensionByContentType(contentType, format)

	// 临时文件名
	tempFileName := fmt.Sprintf("%s-temp%s", safeNodeIDs, fileExt)
	tempFilePath := filepath.Join(tempDir, tempFileName)

	// 创建临时文件
	downloadTempPath := tempFilePath + ".tmp"
	file, err := os.Create(downloadTempPath)
	if err != nil {
		return "", fmt.Errorf("创建临时文件失败: %v", err)
	}

	// 创建一个TeeReader，同时写入文件和计算哈希值
	hasher := md5.New()
	reader := io.TeeReader(resp.Body, hasher)

	bytes, err := io.Copy(file, reader)
	if err != nil {
		file.Close()
		os.Remove(downloadTempPath)
		return "", fmt.Errorf("写入临时文件失败: %v", err)
	}

	// 关闭文件
	if err := file.Close(); err != nil {
		os.Remove(downloadTempPath)
		return "", fmt.Errorf("关闭临时文件失败: %v", err)
	}

	// 检查文件大小
	if bytes == 0 {
		os.Remove(downloadTempPath)
		return "", fmt.Errorf("下载的图片文件为空")
	}

	// 计算文件哈希值
	hashSum := fmt.Sprintf("%x", hasher.Sum(nil))
	fmt.Printf("图片文件哈希值: %s, 大小: %d bytes\n", hashSum, bytes)

	// 生成最终文件名（包含缩放等级和哈希值）
	// 格式：节点-缩放等级x-hash.ext，例如：节点-1x-hash.png 或 节点-0.5x-hash.png
	// 注意：这里默认使用x后缀（带文字），如需支持p后缀需要传入ignoreTexts参数
	scaleStr := formatScaleForFilename(scale)
	finalFileName := fmt.Sprintf("%s-%sx-%s%s", safeNodeIDs, scaleStr, hashSum[:8], fileExt)
	finalFilePath := filepath.Join(tempDir, finalFileName)

	// 检查是否已经存在相同哈希的文件
	if _, err := os.Stat(finalFilePath); err == nil {
		// 文件已存在，删除临时文件并返回现有文件路径
		os.Remove(downloadTempPath)
		fmt.Printf("相同内容的图片文件已存在，使用现有文件: %s\n", finalFilePath)
		return finalFilePath, nil
	}

	// 将临时文件重命名为最终文件名
	if err := os.Rename(downloadTempPath, finalFilePath); err != nil {
		os.Remove(downloadTempPath)
		return "", fmt.Errorf("重命名文件失败: %v", err)
	}

	fmt.Printf("图片下载并缓存成功: %s\n", finalFilePath)

	// 清理同一节点和缩放等级的过多旧文件
	cleanupOldNodeImages(tempDir, safeNodeIDs, scale, 10)

	return finalFilePath, nil
}

// findLatestNodeImage 查找节点的最新图片文件（根据缩放等级）
func findLatestNodeImage(tempDir, safeNodeID string, scale float64) string {
	return findLatestNodeImageWithIgnoreTexts(tempDir, safeNodeID, scale, false)
}

// findLatestNodeImageWithIgnoreTexts 查找节点的最新图片文件（根据缩放等级和忽略文字标志）
func findLatestNodeImageWithIgnoreTexts(tempDir, safeNodeID string, scale float64, ignoreTexts bool) string {
	return findLatestNodeImageWithIgnoreTextsAndNodes(tempDir, safeNodeID, scale, ignoreTexts, []string{})
}

// findLatestNodeImageWithIgnoreTextsAndNodes 查找节点的最新图片文件（根据缩放等级、忽略文字标志和忽略节点列表）
func findLatestNodeImageWithIgnoreTextsAndNodes(tempDir, safeNodeID string, scale float64, ignoreTexts bool, ignoreNodes []string) string {
	// 构建缩放等级字符串
	scaleStr := formatScaleForFilename(scale)

	// 根据ignoreTexts决定文件名后缀：x表示带文字，p表示不带文字
	suffix := "x"
	if ignoreTexts {
		suffix = "p"
	}

	// 如果有忽略节点，添加到后缀中
	if len(ignoreNodes) > 0 {
		// 对忽略节点列表进行排序以确保一致性
		sortedIgnoreNodes := make([]string, len(ignoreNodes))
		copy(sortedIgnoreNodes, ignoreNodes)
		sort.Strings(sortedIgnoreNodes)

		// 生成忽略节点的哈希值作为文件名的一部分
		ignoreNodesStr := strings.Join(sortedIgnoreNodes, ",")
		hash := md5.Sum([]byte(ignoreNodesStr))
		ignoreNodesHash := fmt.Sprintf("%x", hash)[:8] // 使用前8位哈希值
		suffix = suffix + "i" + ignoreNodesHash
	}

	// 查找同一节点和缩放等级的所有图片文件（支持多种格式，不包括临时文件）
	supportedExts := []string{".png", ".svg", ".jpg", ".jpeg"}
	var allFiles []string

	for _, ext := range supportedExts {
		// 匹配格式：节点-缩放等级x/p[i哈希值]-*.*
		// 例如：1_1223-1.0x-abc123.png 或 1_1223-1.0pi12345678-abc123.png
		pattern := filepath.Join(tempDir, safeNodeID+"-"+scaleStr+suffix+"-*"+ext)
		files, err := filepath.Glob(pattern)
		if err == nil && len(files) > 0 {
			allFiles = append(allFiles, files...)
		}
	}

	if len(allFiles) == 0 {
		return ""
	}

	// 过滤掉临时文件
	var validFiles []string
	for _, f := range allFiles {
		// 检查是否是临时文件（包含-temp.扩展名）
		if !strings.Contains(f, "-temp.") && !strings.Contains(f, ".tmp") {
			validFiles = append(validFiles, f)
		}
	}

	if len(validFiles) == 0 {
		return ""
	}

	// 按修改时间排序
	type fileInfo struct {
		path    string
		modTime time.Time
	}
	fileInfos := make([]fileInfo, 0, len(validFiles))

	for _, f := range validFiles {
		if info, err := os.Stat(f); err == nil {
			fileInfos = append(fileInfos, fileInfo{path: f, modTime: info.ModTime()})
		}
	}

	// 按修改时间降序排序
	sort.Slice(fileInfos, func(i, j int) bool {
		return fileInfos[i].modTime.After(fileInfos[j].modTime)
	})

	// 返回最新的文件
	if len(fileInfos) > 0 {
		return fileInfos[0].path
	}

	return ""
}

// cleanupOldNodeImages 清理节点的旧图片文件（根据缩放等级）
func cleanupOldNodeImages(tempDir, safeNodeID string, scale float64, keepCount int) {
	// 构建缩放等级字符串
	scaleStr := formatScaleForFilename(scale)

	// 查找同一节点和缩放等级的所有图片文件（支持多种格式，不包括临时文件）
	// 同时支持x后缀（带文字）和p后缀（不带文字）
	supportedExts := []string{".png", ".svg", ".jpg", ".jpeg"}
	var allFiles []string

	for _, ext := range supportedExts {
		// 匹配格式：节点-缩放等级x-*.* 和 节点-缩放等级p-*.*
		patternX := filepath.Join(tempDir, safeNodeID+"-"+scaleStr+"x-*"+ext)
		filesX, err := filepath.Glob(patternX)
		if err == nil && len(filesX) > 0 {
			allFiles = append(allFiles, filesX...)
		}

		patternP := filepath.Join(tempDir, safeNodeID+"-"+scaleStr+"p-*"+ext)
		filesP, err := filepath.Glob(patternP)
		if err == nil && len(filesP) > 0 {
			allFiles = append(allFiles, filesP...)
		}
	}

	// 过滤掉临时文件
	var validFiles []string
	for _, f := range allFiles {
		if !strings.Contains(f, "-temp.") && !strings.Contains(f, ".tmp") {
			validFiles = append(validFiles, f)
		}
	}

	if len(validFiles) <= keepCount {
		return // 不需要清理
	}

	// 按修改时间排序
	type fileInfo struct {
		path    string
		modTime time.Time
	}
	fileInfos := make([]fileInfo, 0, len(validFiles))

	for _, f := range validFiles {
		if info, err := os.Stat(f); err == nil {
			fileInfos = append(fileInfos, fileInfo{path: f, modTime: info.ModTime()})
		}
	}

	// 按修改时间降序排序
	sort.Slice(fileInfos, func(i, j int) bool {
		return fileInfos[i].modTime.After(fileInfos[j].modTime)
	})

	// 删除多余的旧文件
	for i := keepCount; i < len(fileInfos); i++ {
		if err := os.Remove(fileInfos[i].path); err != nil {
			fmt.Printf("删除旧图片文件失败: %s, 错误: %v\n", fileInfos[i].path, err)
		} else {
			fmt.Printf("已删除旧图片文件: %s\n", fileInfos[i].path)
		}
	}
}

// getContentTypeByExtension 根据文件扩展名获取Content-Type
func getContentTypeByExtension(ext string) string {
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".svg":
		return "image/svg+xml"
	default:
		return "application/octet-stream"
	}
}

// getFileExtensionByContentType 根据Content-Type和请求格式确定文件扩展名
func getFileExtensionByContentType(contentType, requestFormat string) string {
	// 优先使用Content-Type判断
	if strings.HasPrefix(contentType, "image/png") {
		return ".png"
	} else if strings.HasPrefix(contentType, "image/jpeg") {
		return ".jpg"
	} else if strings.HasPrefix(contentType, "image/svg") {
		return ".svg"
	}

	// 如果Content-Type无法判断，使用请求的格式
	switch strings.ToLower(requestFormat) {
	case "png":
		return ".png"
	case "jpg", "jpeg":
		return ".jpg"
	case "svg":
		return ".svg"
	default:
		return ".png" // 默认为PNG
	}
}

// extractAllNodeIDs 从优化的JSON数据中提取所有节点ID
func extractAllNodeIDs(optimizedData gin.H) ([]string, error) {
	var nodeIDs []string

	// 从node字段中提取节点ID（现在是单个对象而不是数组）
	if node, ok := optimizedData["node"].(gin.H); ok {
		nodeIDs = extractNodeIDsRecursiveFromSingle(node, nodeIDs)
	} else {
		return nil, fmt.Errorf("无法解析node数据")
	}

	fmt.Printf("从JSON数据中提取到 %d 个节点ID\n", len(nodeIDs))
	return nodeIDs, nil
}

// extractNodeIDsRecursiveFromSingle 从单个节点递归提取节点ID（包括子节点）
func extractNodeIDsRecursiveFromSingle(node gin.H, nodeIDs []string) []string {
	// 提取当前节点的ID
	if id, ok := node["id"].(string); ok {
		nodeIDs = append(nodeIDs, id)
	}

	// 递归处理子节点
	if children, ok := node["children"].([]gin.H); ok && len(children) > 0 {
		nodeIDs = extractNodeIDsRecursive(children, nodeIDs)
	}

	return nodeIDs
}

// extractNodeIDsRecursive 递归提取节点ID（包括子节点）
func extractNodeIDsRecursive(nodes []gin.H, nodeIDs []string) []string {
	for _, node := range nodes {
		// 提取当前节点的ID
		if id, ok := node["id"].(string); ok {
			nodeIDs = append(nodeIDs, id)
		}

		// 递归处理子节点
		if children, ok := node["children"].([]gin.H); ok && len(children) > 0 {
			nodeIDs = extractNodeIDsRecursive(children, nodeIDs)
		}
	}
	return nodeIDs
}

// clearCacheByNodeIDs 根据节点ID列表清除对应的缓存文件
func clearCacheByNodeIDs(fileKey string, nodeIDs []string) (int, error) {
	// 需要清理的目录列表
	cacheDirs := []string{
		filepath.Join("temp", fileKey, "previews"),
		filepath.Join("temp", fileKey, "images"),
		filepath.Join("temp", fileKey, "fpreviews"),
		filepath.Join("temp", fileKey, "documents"),
	}

	totalCleared := 0
	var allErrors []string

	// 将节点ID转换为安全的文件名前缀
	safeNodeIDs := make([]string, len(nodeIDs))
	for i, nodeID := range nodeIDs {
		safeNodeIDs[i] = strings.NewReplacer(
			":", "_", ";", "_", "/", "_", "\\", "_",
			"*", "_", "?", "_", "\"", "_", "<", "_",
			">", "_", "|", "_", ",", "_",
		).Replace(nodeID)
	}

	fmt.Printf("开始清除 %d 个节点的缓存文件\n", len(nodeIDs))

	for _, tempDir := range cacheDirs {
		fmt.Printf("检查缓存目录: %s\n", tempDir)

		// 检查目录是否存在
		if _, err := os.Stat(tempDir); os.IsNotExist(err) {
			fmt.Printf("目录不存在，跳过: %s\n", tempDir)
			continue
		}

		dirCleared := 0

		// 首先清理所有 .tmp 文件
		tmpPattern := filepath.Join(tempDir, "*.tmp")
		tmpFiles, err := filepath.Glob(tmpPattern)
		if err != nil {
			errMsg := fmt.Sprintf("查找临时文件失败: %v", err)
			fmt.Println(errMsg)
			allErrors = append(allErrors, errMsg)
		} else {
			for _, tmpFile := range tmpFiles {
				fmt.Printf("删除临时文件: %s\n", tmpFile)
				if err := os.Remove(tmpFile); err != nil {
					errMsg := fmt.Sprintf("删除临时文件失败: %s, 错误: %v", tmpFile, err)
					fmt.Println(errMsg)
					allErrors = append(allErrors, errMsg)
				} else {
					dirCleared++
					totalCleared++
				}
			}
		}

		// 如果是 documents 目录，清理 JSON 缓存文件
		if strings.Contains(tempDir, "documents") {
			jsonPattern := filepath.Join(tempDir, "*.json")
			jsonFiles, err := filepath.Glob(jsonPattern)
			if err != nil {
				errMsg := fmt.Sprintf("查找JSON缓存文件失败: %v", err)
				fmt.Println(errMsg)
				allErrors = append(allErrors, errMsg)
			} else {
				for _, jsonFile := range jsonFiles {
					fmt.Printf("删除JSON缓存文件: %s\n", jsonFile)
					if err := os.Remove(jsonFile); err != nil {
						errMsg := fmt.Sprintf("删除JSON缓存文件失败: %s, 错误: %v", jsonFile, err)
						fmt.Println(errMsg)
						allErrors = append(allErrors, errMsg)
					} else {
						dirCleared++
						totalCleared++
					}
				}
			}
		} else {
			// 为其他目录（图片目录），按节点ID查找并删除对应的缓存文件
			for i, safeNodeID := range safeNodeIDs {
				originalNodeID := nodeIDs[i]

				// 支持多种图片格式
				supportedExts := []string{".png", ".jpg", ".svg"}

				for _, ext := range supportedExts {
					// 查找以节点ID为前缀的文件
					pattern := filepath.Join(tempDir, safeNodeID+"-*"+ext)
					files, err := filepath.Glob(pattern)
					if err != nil {
						errMsg := fmt.Sprintf("查找节点 %s 的缓存文件失败: %v", originalNodeID, err)
						fmt.Println(errMsg)
						allErrors = append(allErrors, errMsg)
						continue
					}

					// 删除找到的文件
					for _, file := range files {
						fmt.Printf("删除节点 %s 的缓存文件: %s\n", originalNodeID, file)
						if err := os.Remove(file); err != nil {
							errMsg := fmt.Sprintf("删除文件失败: %s, 错误: %v", file, err)
							fmt.Println(errMsg)
							allErrors = append(allErrors, errMsg)
						} else {
							dirCleared++
							totalCleared++
						}
					}
				}
			}
		}

		fmt.Printf("目录 %s 中清除了 %d 个文件\n", tempDir, dirCleared)
	}

	fmt.Printf("总共清除了 %d 个缓存文件\n", totalCleared)

	// 如果有错误，返回合并的错误信息
	if len(allErrors) > 0 {
		return totalCleared, fmt.Errorf("清理过程中发生错误: %s", strings.Join(allErrors, "; "))
	}

	return totalCleared, nil
}

// APIClearTempFiles 清理临时文件
func APIClearTempFiles(c *gin.Context) {
	mcpToken := c.Param("mcptoken")
	fileKey := c.Query("file_key")

	// 参数验证
	if mcpToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "MCP token不能为空",
		})
		return
	}

	if fileKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "file_key参数不能为空",
		})
		return
	}

	// 通过MCP token查找用户
	var user models.User
	err := models.DB.Where("mcp_token = ? AND mcp_token != ''", mcpToken).First(&user).Error
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "无效的MCP token或用户不存在",
		})
		return
	}

	// 需要清理的目录列表
	cacheDirs := []string{
		filepath.Join("temp", fileKey, "previews"),
		filepath.Join("temp", fileKey, "images"),
		filepath.Join("temp", fileKey, "fpreviews"),
	}

	totalCleared := 0
	var allErrors []string

	for _, tempDir := range cacheDirs {
		fmt.Printf("清理目录中的临时文件: %s\n", tempDir)

		// 检查目录是否存在
		if _, err := os.Stat(tempDir); os.IsNotExist(err) {
			fmt.Printf("目录不存在，跳过: %s\n", tempDir)
			continue
		}

		// 清理临时文件
		err := services.CleanupAllTempFiles(tempDir)
		if err != nil {
			errMsg := fmt.Sprintf("清理目录 %s 的临时文件失败: %v", tempDir, err)
			fmt.Println(errMsg)
			allErrors = append(allErrors, errMsg)
		} else {
			// 计算清理的文件数量
			tmpPattern := filepath.Join(tempDir, "*.tmp")
			tmpFiles, _ := filepath.Glob(tmpPattern)
			tempPattern := filepath.Join(tempDir, "*-temp.*")
			tempFiles, _ := filepath.Glob(tempPattern)

			// 过滤掉 .tmp 文件（避免重复计算）
			var validTempFiles []string
			for _, f := range tempFiles {
				if !strings.HasSuffix(f, ".tmp") {
					validTempFiles = append(validTempFiles, f)
				}
			}

			totalCleared += len(tmpFiles) + len(validTempFiles)
		}
	}

	if len(allErrors) > 0 {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "清理临时文件过程中发生错误: " + strings.Join(allErrors, "; "),
			"cleared": totalCleared,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("已成功清理 %d 个临时文件", totalCleared),
		"cleared": totalCleared,
	})
}

// isEmptyValue 判断值是否为空
func isEmptyValue(value interface{}) bool {
	if value == nil {
		return true
	}

	switch v := value.(type) {
	case string:
		return v == ""
	case []interface{}:
		return len(v) == 0
	case map[string]interface{}:
		return len(v) == 0
	case []gin.H:
		return len(v) == 0
	case gin.H:
		// 节点对象不应该被认为是空值，即使字段数为0
		return false
	case []string:
		return len(v) == 0
	case []int:
		return len(v) == 0
	case []float64:
		return len(v) == 0
	default:
		// 对于其他类型（如bool、int、float64等），不认为是空值
		return false
	}
}

// applyModificationsToNode 将修改信息应用到节点中
func applyModificationsToNode(node gin.H, modifys map[string]interface{}) {
	// 1. rename不为空时，替换节点name
	if rename, ok := modifys["rename"].(string); ok && !isEmptyValue(rename) {
		node["name"] = rename
	}

	// 2. 设置constraints约束（支持horizontal和vertical）
	var hasConstraints bool
	constraints := gin.H{}

	if horizontal, ok := modifys["horizontal"].(string); ok && !isEmptyValue(horizontal) {
		constraints["horizontal"] = horizontal
		hasConstraints = true
	}

	if vertical, ok := modifys["vertical"].(string); ok && !isEmptyValue(vertical) {
		constraints["vertical"] = vertical
		hasConstraints = true
	}

	if hasConstraints {
		node["constraints"] = constraints
	}

	// 3. components不为空时，增加字段
	if components, ok := modifys["components"]; ok && !isEmptyValue(components) {
		node["components"] = components
	}

	// 4. img_ext不为空时，增加字段
	if img_ext, ok := modifys["img_ext"].(string); ok && !isEmptyValue(img_ext) {
		node["img_ext"] = img_ext
	}

	// 5. img_name不为空时，增加字段
	if imgName, ok := modifys["img_name"].(string); ok && !isEmptyValue(imgName) {
		node["img_name"] = imgName
	}

	// 6. img_id不为空时，增加字段
	if imgID, ok := modifys["img_id"].(string); ok && !isEmptyValue(imgID) {
		node["img_id"] = imgID
	}

	// 7. res_mode不为空时，增加字段
	if resMode, ok := modifys["res_mode"].(string); ok && !isEmptyValue(resMode) {
		node["res_mode"] = resMode
	}

	// 8. parent_id不为空时，切换子树的父节点到parent_id
	if parentID, ok := modifys["parent_id"].(string); ok && !isEmptyValue(parentID) {
		node["parent_id"] = parentID
	}

	// 注意：ignore为true的节点已经在过滤阶段被移除，这里不需要处理
}

// filterEmptyValuesFromNode 递归过滤节点中的空值
func filterEmptyValuesFromNode(node gin.H) gin.H {
	filteredNode := gin.H{}

	for key, value := range node {
		if !isEmptyValue(value) {
			// 如果是子节点数组，递归过滤
			if key == "children" {
				// 处理不同类型的children数组
				var childrenArray []gin.H
				var hasValidChildren bool

				// 尝试[]gin.H类型
				if ginHChildren, ok := value.([]gin.H); ok {
					childrenArray = ginHChildren
					hasValidChildren = len(childrenArray) > 0
				} else if interfaceChildren, ok := value.([]interface{}); ok {
					// 尝试[]interface{}类型，转换为[]gin.H
					childrenArray = make([]gin.H, 0, len(interfaceChildren))
					for _, item := range interfaceChildren {
						if childNode, ok := item.(map[string]interface{}); ok {
							childrenArray = append(childrenArray, gin.H(childNode))
						}
					}
					hasValidChildren = len(childrenArray) > 0
				}

				if hasValidChildren {
					filteredChildren := make([]gin.H, 0)
					for _, child := range childrenArray {
						filteredChild := filterEmptyValuesFromNode(child)
						filteredChildren = append(filteredChildren, filteredChild)
					}
					if len(filteredChildren) > 0 {
						filteredNode[key] = filteredChildren
					}
				}
			} else {
				filteredNode[key] = value
			}
		}
	}

	return filteredNode
}

// simplifyNodeAggressively 激进简化节点，只保留核心字段（2档位）
func simplifyNodeAggressively(node gin.H) gin.H {
	simplifiedNode := gin.H{}

	// 核心字段列表 - 这些字段总是保留
	coreFields := map[string]bool{
		"id":        true,
		"name":      true,
		"type":      true,
		"parent_id": true,
		"children":  true,
		// 修改相关的字段
		"horizontal": true,
		"vertical":   true,
		"components": true,
		"img_ext":    true,
		"img_name":   true,
		"res_mode":   true,
		// 布局相关的重要字段
		"absoluteBoundingBox":  true,
		"absoluteRenderBounds": true,
		"x":                    true,
		"y":                    true,
		"width":                true,
		"height":               true,
		// 样式相关的重要字段
		"fills":        true,
		"strokeAlign":  true,
		"strokeWeight": true,
		"strokes":      true,
		"opacity":      true,
		"blendMode":    true,
		"effects":      true,
		// 文字相关
		"characters": true,
		"style":      true,
		"fontName":   true,
		"fontSize":   true,
		// 组件相关
		"componentId":    true,
		"componentSetId": true,
	}

	// 根据节点类型保留特定字段
	nodeType, _ := node["type"].(string)
	typeSpecificFields := map[string][]string{
		"TEXT": {
			"characters", "style", "fontName", "fontSize", "textAlignHorizontal", "textAlignVertical",
			"textAutoResize", "textCase", "textDecoration", "lineHeight", "letterSpacing",
		},
		"FRAME": {
			"layoutMode", "primaryAxisAlignItems", "counterAxisAlignItems", "paddingLeft",
			"paddingRight", "paddingTop", "paddingBottom", "itemSpacing",
		},
		"COMPONENT": {
			"componentId", "componentSetId",
		},
		"INSTANCE": {
			"componentId", "componentProperties",
		},
	}

	// 添加节点类型特定的字段到核心字段列表
	if fields, ok := typeSpecificFields[nodeType]; ok {
		for _, field := range fields {
			coreFields[field] = true
		}
	}

	// 遍历节点，只保留核心字段
	for key, value := range node {
		if coreFields[key] {
			if key == "children" {
				// 递归处理子节点
				var childrenArray []gin.H
				if ginHChildren, ok := value.([]gin.H); ok {
					childrenArray = ginHChildren
				} else if interfaceChildren, ok := value.([]interface{}); ok {
					childrenArray = make([]gin.H, 0, len(interfaceChildren))
					for _, item := range interfaceChildren {
						if childNode, ok := item.(map[string]interface{}); ok {
							childrenArray = append(childrenArray, gin.H(childNode))
						}
					}
				}

				if len(childrenArray) > 0 {
					simplifiedChildren := make([]gin.H, 0)
					for _, child := range childrenArray {
						simplifiedChild := simplifyNodeAggressively(child)
						// 只添加非空的简化子节点
						if len(simplifiedChild) > 0 {
							simplifiedChildren = append(simplifiedChildren, simplifiedChild)
						}
					}
					if len(simplifiedChildren) > 0 {
						simplifiedNode[key] = simplifiedChildren
					}
				}
			} else if !isEmptyValue(value) {
				simplifiedNode[key] = value
			}
		}
	}

	return simplifiedNode
}

// processNodesByResMode 根据res_mode处理节点结构
func processNodesByResMode(nodes []gin.H, nodeModifys map[string]map[string]interface{}) []gin.H {
	// 构建节点ID到节点的映射
	nodeMap := make(map[string]*gin.H)
	for i := range nodes {
		nodeID := nodes[i]["id"].(string)
		nodeMap[nodeID] = &nodes[i]
	}

	// 构建父子关系映射
	childrenMap := make(map[string][]*gin.H)
	for i := range nodes {
		node := &nodes[i]
		parentID := (*node)["parent_id"]
		if parentID != nil {
			if parentIDStr, ok := parentID.(string); ok {
				childrenMap[parentIDStr] = append(childrenMap[parentIDStr], node)
			}
		}
	}

	// 标记需要移除的节点
	nodesToRemove := make(map[string]bool)
	nodesToPromote := make(map[string][]*gin.H) // 需要提升的子节点

	// 递归处理所有节点的资源模式
	processNodeResMode(nodes, childrenMap, nodeModifys, nodesToRemove, nodesToPromote)

	// 构建新的节点列表，排除被标记移除的节点
	var result []gin.H
	for _, node := range nodes {
		nodeID := node["id"].(string)
		if !nodesToRemove[nodeID] {
			result = append(result, node)
		}
	}

	return result
}

// processNodeResMode 递归处理所有节点的资源模式
func processNodeResMode(nodes []gin.H, childrenMap map[string][]*gin.H, nodeModifys map[string]map[string]interface{}, nodesToRemove map[string]bool, nodesToPromote map[string][]*gin.H) {
	for i := range nodes {
		node := &nodes[i]
		nodeID := (*node)["id"].(string)

		// 如果节点已经被标记为删除，跳过处理
		if nodesToRemove[nodeID] {
			continue
		}

		// 检查是否有res_mode修改
		if modifys, exists := nodeModifys[nodeID]; exists {
			if resMode, ok := modifys["res_mode"].(string); ok && resMode != "" {
				switch resMode {
				case "sprite", "slice", "texture":
					// 图片相关模式：实现父子res_mode共存机制
					// 当父节点设置了图片资源模式时，只保留以下**直接子节点**：
					// 1. 子节点设置了res_mode（会单独渲染）
					// 2. 子节点为TEXT类型（保留为动态文本）
					// 3. 子节点的后代中包含设置了res_mode或TEXT类型的节点
					// 清除其他装饰性子节点
					if children, hasChildren := childrenMap[nodeID]; hasChildren {
						for _, child := range children {
							childID := (*child)["id"].(string)
							childType := ""
							if t, ok := (*child)["type"].(string); ok {
								childType = t
							}

							// 检查子节点是否应该保留
							shouldKeep := false

							// 1. 子节点本身设置了res_mode - 保留（会单独渲染）
							if childModifys, exists := nodeModifys[childID]; exists {
								if childResMode, ok := childModifys["res_mode"].(string); ok && childResMode != "" {
									shouldKeep = true
									fmt.Printf("保留子节点 %s (设置了res_mode=%s)\n", childID, childResMode)
								}
							}

							// 2. 子节点为TEXT类型 - 保留（动态文本）
							if childType == "TEXT" {
								shouldKeep = true
								fmt.Printf("保留子节点 %s (TEXT类型)\n", childID)
							}

							// 3. 子节点的后代中包含图片或文字节点 - 保留此子节点
							// 这样可以保持层级结构，让后代的图片/文字节点能够正确渲染
							if !shouldKeep && hasResNodeOrTextInDescendants(childID, childrenMap, nodeModifys) {
								shouldKeep = true
								fmt.Printf("保留子节点 %s (后代中包含图片或文字节点)\n", childID)
							}

							// 如果不需要保留，则标记删除
							if !shouldKeep {
								fmt.Printf("删除装饰性子节点 %s (父节点=%s为图片资源)\n", childID, nodeID)
								markNodeAndDescendantsForRemoval(childID, childrenMap, nodesToRemove)
							}
						}
					}
				case "cutout":
					// 层级剔除模式：移除当前节点，子节点上移
					nodesToRemove[nodeID] = true
					if children, hasChildren := childrenMap[nodeID]; hasChildren {
						currentParentID := (*node)["parent_id"]
						if currentParentID != nil {
							if parentIDStr, ok := currentParentID.(string); ok {
								// 将子节点的父节点设置为当前节点的父节点
								for _, child := range children {
									(*child)["parent_id"] = parentIDStr
								}
								// 记录需要提升的子节点
								nodesToPromote[parentIDStr] = append(nodesToPromote[parentIDStr], children...)
							}
						} else {
							// 如果当前节点没有父节点，子节点也设为根节点
							for _, child := range children {
								(*child)["parent_id"] = nil
							}
						}
					}
				}
			}
		}

		// 递归处理子节点（只处理未被删除的子节点）
		if children, hasChildren := childrenMap[nodeID]; hasChildren {
			for _, child := range children {
				childID := (*child)["id"].(string)
				if !nodesToRemove[childID] {
					// 递归处理单个子节点
					processNodeResModeSingle(*child, childrenMap, nodeModifys, nodesToRemove, nodesToPromote)
				}
			}
		}
	}
}

// shouldKeepChildNode 检查子节点是否应该被保留
func shouldKeepChildNode(nodeID string, nodeModifys map[string]map[string]interface{}) bool {
	// 检查节点是否有修改信息
	modifys, exists := nodeModifys[nodeID]
	if !exists {
		// 没有修改信息，不保留
		return false
	}

	// 检查res_mode
	if resMode, ok := modifys["res_mode"].(string); ok && resMode != "" {
		// 如果res_mode是attach或cutout，不保留
		if resMode == "attach" || resMode == "cutout" {
			return false
		}
		// 其他res_mode（sprite, slice, texture等）都保留
		return true
	}

	// 有修改信息但没有res_mode，也不保留
	return false
}

// processNodeResModeSingle 递归处理单个节点的资源模式
func processNodeResModeSingle(node gin.H, childrenMap map[string][]*gin.H, nodeModifys map[string]map[string]interface{}, nodesToRemove map[string]bool, nodesToPromote map[string][]*gin.H) {
	nodeID := node["id"].(string)

	// 如果节点已经被标记为删除，跳过处理
	if nodesToRemove[nodeID] {
		return
	}

	// 检查是否有res_mode修改
	if modifys, exists := nodeModifys[nodeID]; exists {
		if resMode, ok := modifys["res_mode"].(string); ok && resMode != "" {
			switch resMode {
			case "sprite", "slice", "texture":
				// 图片相关模式：实现父子res_mode共存机制
				// 当父节点设置了图片资源模式时，只保留以下**直接子节点**：
				// 1. 子节点设置了res_mode（会单独渲染）
				// 2. 子节点为TEXT类型（保留为动态文本）
				// 3. 子节点的后代中包含设置了res_mode或TEXT类型的节点
				if children, hasChildren := childrenMap[nodeID]; hasChildren {
					for _, child := range children {
						childID := (*child)["id"].(string)
						childType := ""
						if t, ok := (*child)["type"].(string); ok {
							childType = t
						}

						// 检查子节点是否应该保留
						shouldKeep := false

						// 1. 子节点本身设置了res_mode - 保留
						if childModifys, exists := nodeModifys[childID]; exists {
							if childResMode, ok := childModifys["res_mode"].(string); ok && childResMode != "" {
								shouldKeep = true
							}
						}

						// 2. 子节点为TEXT类型 - 保留
						if childType == "TEXT" {
							shouldKeep = true
						}

						// 3. 子节点的后代中包含图片或文字节点 - 保留此子节点
						if !shouldKeep && hasResNodeOrTextInDescendants(childID, childrenMap, nodeModifys) {
							shouldKeep = true
						}

						// 如果不需要保留，则标记删除
						if !shouldKeep {
							markNodeAndDescendantsForRemoval(childID, childrenMap, nodesToRemove)
						}
					}
				}
			case "cutout":
				// 层级剔除模式：移除当前节点，子节点上移
				nodesToRemove[nodeID] = true
				if children, hasChildren := childrenMap[nodeID]; hasChildren {
					currentParentID := node["parent_id"]
					if currentParentID != nil {
						if parentIDStr, ok := currentParentID.(string); ok {
							// 将子节点的父节点设置为当前节点的父节点
							for _, child := range children {
								(*child)["parent_id"] = parentIDStr
							}
							// 记录需要提升的子节点
							nodesToPromote[parentIDStr] = append(nodesToPromote[parentIDStr], children...)
						}
					} else {
						// 如果当前节点没有父节点，子节点也设为根节点
						for _, child := range children {
							(*child)["parent_id"] = nil
						}
					}
				}
			}
		}
	}

	// 递归处理子节点（只处理未被删除的子节点）
	if children, hasChildren := childrenMap[nodeID]; hasChildren {
		for _, child := range children {
			childID := (*child)["id"].(string)
			if !nodesToRemove[childID] {
				// 递归处理单个子节点
				processNodeResModeSingle(*child, childrenMap, nodeModifys, nodesToRemove, nodesToPromote)
			}
		}
	}
}

// hasResNodeOrTextInDescendants 检查节点的后代中是否包含设置了res_mode的节点或TEXT类型节点
// 用于判断子节点是否需要保留（父子res_mode共存机制）
// 注意：只检查后代节点，不检查节点本身
func hasResNodeOrTextInDescendants(nodeID string, childrenMap map[string][]*gin.H, nodeModifys map[string]map[string]interface{}) bool {
	// 递归检查所有后代节点
	if children, hasChildren := childrenMap[nodeID]; hasChildren {
		for _, child := range children {
			childID := (*child)["id"].(string)
			childType := ""
			if t, ok := (*child)["type"].(string); ok {
				childType = t
			}

			// 检查子节点是否为TEXT类型
			if childType == "TEXT" {
				return true
			}

			// 检查子节点是否设置了有效的res_mode
			if modifys, exists := nodeModifys[childID]; exists {
				if resMode, ok := modifys["res_mode"].(string); ok && resMode != "" {
					// 如果res_mode是attach或cutout，不算作有效的资源节点
					if resMode != "attach" && resMode != "cutout" {
						return true
					}
				}
			}

			// 递归检查孙子节点
			if hasResNodeOrTextInDescendants(childID, childrenMap, nodeModifys) {
				return true
			}
		}
	}

	return false
}

// markNodeAndDescendantsForRemoval 递归标记节点及其所有后代节点为移除状态
func markNodeAndDescendantsForRemoval(nodeID string, childrenMap map[string][]*gin.H, nodesToRemove map[string]bool) {
	nodesToRemove[nodeID] = true

	// 递归标记所有子节点
	if children, hasChildren := childrenMap[nodeID]; hasChildren {
		for _, child := range children {
			childID := (*child)["id"].(string)
			markNodeAndDescendantsForRemoval(childID, childrenMap, nodesToRemove)
		}
	}
}

// filterVisibleAndNonIgnoredNodes 过滤掉不可见和被忽略的节点及其子树
func filterVisibleAndNonIgnoredNodes(nodes []map[string]interface{}, nodeModifys map[string]map[string]interface{}) []map[string]interface{} {
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

	// 收集需要排除的节点ID（包括其子树）
	excludedNodeIDs := make(map[string]bool)

	for _, node := range nodes {
		nodeID := node["id"].(string)

		// 检查是否应该排除此节点
		if shouldExcludeNodeAndSubtree(node, nodeModifys[nodeID]) {
			// 标记此节点及其所有子节点为排除
			markNodeAndChildrenAsExcluded(nodeID, childrenMap, excludedNodeIDs)
		}
	}

	// 构建过滤后的节点列表
	var filteredNodes []map[string]interface{}
	for _, node := range nodes {
		nodeID := node["id"].(string)
		if !excludedNodeIDs[nodeID] {
			filteredNodes = append(filteredNodes, node)
		}
	}

	fmt.Printf("原始节点数: %d, 过滤后节点数: %d, 排除节点数: %d\n",
		len(nodes), len(filteredNodes), len(excludedNodeIDs))
	// 打印所有被忽略的节点
	fmt.Printf("被忽略的节点ID: [")
	first := true
	for nodeID := range excludedNodeIDs {
		if first {
			fmt.Printf("%q", nodeID)
			first = false
		} else {
			fmt.Printf(", %q", nodeID)
		}
	}
	fmt.Println("]")

	return filteredNodes
}

// shouldExcludeNodeAndSubtree 判断节点是否应该被排除（包括其子树）
func shouldExcludeNodeAndSubtree(node map[string]interface{}, modifys map[string]interface{}) bool {
	nodeID := node["id"].(string)

	// 1. 检查visible属性，如果为false则排除整个子树
	if visible, hasVisible := node["visible"]; hasVisible {
		if visibleBool, ok := visible.(bool); ok && !visibleBool {
			fmt.Printf("排除节点 %s 及其子树，因为其visible为false\n", nodeID)
			return true
		}
	}

	// 2. 检查modifys中的ignore标记，如果为true则排除整个子树
	if modifys != nil {
		if ignore, ok := modifys["ignore"].(bool); ok && ignore {
			fmt.Printf("排除节点 %s 及其子树，因为其被标记为忽略\n", nodeID)
			return true
		}
	}

	return false
}

// markNodeAndChildrenAsExcluded 递归标记节点及其所有子节点为排除
func markNodeAndChildrenAsExcluded(nodeID string, childrenMap map[string][]string, excludedNodeIDs map[string]bool) {
	// 标记当前节点
	excludedNodeIDs[nodeID] = true

	// 递归标记所有子节点
	if children, hasChildren := childrenMap[nodeID]; hasChildren {
		for _, childID := range children {
			markNodeAndChildrenAsExcluded(childID, childrenMap, excludedNodeIDs)
		}
	}
}

// TestProxy 测试代理连接
func TestProxy(c *gin.Context) {
	var req struct {
		ProxyURL string `json:"proxy_url"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数错误",
		})
		return
	}

	if req.ProxyURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "代理地址不能为空",
		})
		return
	}

	// 解析代理URL
	parsedURL, err := url.Parse(req.ProxyURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("代理URL解析失败: %v", err),
		})
		return
	}

	// 创建带代理的HTTP客户端（支持HTTP/HTTPS/SOCKS5）
	transport := &http.Transport{}

	if parsedURL.Scheme == "socks5" {
		// SOCKS5代理
		dialer, dialErr := proxy.SOCKS5("tcp", parsedURL.Host, nil, proxy.Direct)
		if dialErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("SOCKS5代理配置失败: %v", dialErr),
			})
			return
		}
		// 使用 DialContext 支持 context 和超时控制
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		}
	} else if parsedURL.Scheme == "http" || parsedURL.Scheme == "https" {
		// HTTP/HTTPS代理
		transport.Proxy = http.ProxyURL(parsedURL)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("不支持的代理类型: %s，仅支持 http/https/socks5", parsedURL.Scheme),
		})
		return
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	// 测试请求Figma API
	testURL := "https://api.figma.com/"
	httpReq, err := http.NewRequest("GET", testURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "创建测试请求失败",
		})
		return
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("代理连接失败: %v", err),
		})
		return
	}
	defer resp.Body.Close()

	// 测试成功
	c.JSON(http.StatusOK, gin.H{
		"message": "代理连接测试成功",
		"status":  resp.StatusCode,
	})
}
