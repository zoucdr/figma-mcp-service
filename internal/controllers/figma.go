package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/figma-deliver/internal/middleware"
	"github.com/figma-deliver/internal/models"
	"github.com/figma-deliver/internal/services"
	"github.com/gin-gonic/gin"
)

// ParseFigmaLink 解析Figma链接
func ParseFigmaLink(c *gin.Context) {
	link := c.PostForm("link")
	name := c.PostForm("name")

	if link == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Figma链接不能为空",
		})
		return
	}

	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目名称不能为空",
		})
		return
	}

	fileKey, nodeId, err := services.ParseFigmaLink(link)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "解析Figma链接失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"fileKey": fileKey,
		"nodeId":  nodeId,
		"name":    name,
	})
}

// GetFigmaNode 获取Figma节点
func GetFigmaNode(c *gin.Context) {
	userID := middleware.GetUserID(c)
	fileKey := c.Param("file_key")
	nodeID := c.Param("node_id")

	if fileKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Figma文件Key不能为空",
		})
		return
	}

	// 获取项目名称，默认为"Figma项目 - {fileKey}"
	projectName := c.Query("name")
	if projectName == "" {
		projectName = fmt.Sprintf("Figma项目 - %s", fileKey)
	}

	// 获取Figma URL参数
	figmaURL := c.Query("figma_url")

	// 创建或获取项目，只保存到figma_projects表
	project, err := services.GetOrCreateFigmaProject(userID, fileKey, nodeID, projectName, figmaURL)
	if err != nil {
		// 检查是否是用户不存在的错误
		if strings.Contains(err.Error(), "用户不存在") {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "用户不存在，请确保您已正确登录",
			})
			return
		}

		// 检查是否是外键约束错误
		if strings.Contains(err.Error(), "foreign key constraint fails") {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "外键约束错误：用户ID不存在，请确保您已正确登录",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取Figma项目失败: " + err.Error(),
		})
		return
	}

	// 不再保存节点数据到数据库，直接返回项目信息
	c.JSON(http.StatusOK, gin.H{
		"project": project,
		"message": "项目创建成功，节点信息将在需要时通过API获取",
	})
}

// GetFigmaNodeTree 获取Figma节点树
func GetFigmaNodeTree(c *gin.Context) {
	userID := middleware.GetUserID(c)
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的项目ID",
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

	// 验证项目所有者
	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "您没有权限访问此项目",
		})
		return
	}

	// 获取用户信息
	var user models.User
	result := models.DB.First(&user, userID)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "用户不存在",
		})
		return
	}

	// 如果没有Figma Token，返回错误
	if user.FigmaToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "用户未设置Figma Token",
		})
		return
	}

	// 检查是否强制刷新
	forceRefresh := c.Query("refresh") == "true"

	// 从Figma API获取节点树
	var nodes []map[string]interface{}
	var fetchErr error

	if forceRefresh {
		// 强制从API获取新数据
		// 先删除缓存文件
		safeNodeID := strings.NewReplacer(":", "_", ";", "_", "/", "_", "\\", "_", "?", "_", "*", "_", "\"", "_", "<", "_", ">", "_", "|", "_").Replace(project.RootNodeID)
		cacheDir := filepath.Join("temp", "documents", project.FileKey)
		cacheFile := filepath.Join(cacheDir, safeNodeID+".json")

		// 如果文件存在，则删除
		if _, err := os.Stat(cacheFile); err == nil {
			os.Remove(cacheFile)
		}

		// 从API获取新数据
		nodes, fetchErr = services.GetFigmaNodes(user.FigmaToken, project.FileKey, project.RootNodeID)
	} else {
		// 使用缓存或者获取新数据
		nodes, fetchErr = services.GetFigmaNodes(user.FigmaToken, project.FileKey, project.RootNodeID)
	}

	if fetchErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取Figma节点树失败: " + fetchErr.Error(),
		})
		return
	}

	// 获取所有节点的修改信息
	var figmaNodes []models.FigmaNode
	dbResult := models.DB.Where("project_id = ?", projectID).Find(&figmaNodes)
	if dbResult.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取节点修改信息失败: " + dbResult.Error.Error(),
		})
		return
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

	// 应用修改信息到节点数据（保持扁平结构用于前端兼容）
	processedNodes := applyModificationsToFlatNodes(nodes, nodeModifys)

	c.JSON(http.StatusOK, gin.H{
		"nodes": processedNodes,
	})
}

// applyModificationsToFlatNodes 将修改信息应用到扁平节点数据（用于前端兼容）
func applyModificationsToFlatNodes(nodes []map[string]interface{}, nodeModifys map[string]map[string]interface{}) []map[string]interface{} {
	processedNodes := make([]map[string]interface{}, 0)

	for _, node := range nodes {
		nodeID, ok := node["id"].(string)
		if !ok {
			continue
		}

		// 复制节点数据
		processedNode := make(map[string]interface{})
		for key, value := range node {
			processedNode[key] = value
		}

		// 保存原始名称（在应用修改信息之前）
		originalName, _ := node["name"].(string)
		if originalName == "" {
			originalName, _ = node["id"].(string)
		}
		processedNode["originalName"] = originalName

		// 应用修改信息
		if modifys, exists := nodeModifys[nodeID]; exists {
			// 处理rename - 直接替换name字段
			if rename, ok := modifys["rename"].(string); ok && rename != "" {
				processedNode["name"] = rename
			}

			// 将所有修改信息添加到modifys字段（保持前端兼容性）
			processedNode["modifys"] = modifys
		}

		processedNodes = append(processedNodes, processedNode)
	}

	return processedNodes
}

// RefreshFigmaNodeTree 刷新Figma节点树
func RefreshFigmaNodeTree(c *gin.Context) {
	// 直接调用GetFigmaNodeTree，但添加refresh=true参数
	// 在查询参数中添加refresh=true
	c.Request.URL.RawQuery = "refresh=true"
	// 调用GetFigmaNodeTree处理函数
	GetFigmaNodeTree(c)
}

// GetProjectNodeModifys 获取项目的节点修改信息
func GetProjectNodeModifys(c *gin.Context) {
	userID := middleware.GetUserID(c)
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的项目ID",
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

	// 验证项目所有者
	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "您没有权限访问此项目",
		})
		return
	}

	// 获取项目的所有节点修改信息
	var nodes []models.FigmaNode
	result := models.DB.Where("project_id = ?", projectID).Find(&nodes)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取节点修改信息失败: " + result.Error.Error(),
		})
		return
	}

	// 构建节点ID到修改信息的映射
	nodeModifys := make(map[string]interface{})
	for _, node := range nodes {
		if node.Modifys != "" {
			var modifyData interface{}
			if err := json.Unmarshal([]byte(node.Modifys), &modifyData); err == nil {
				nodeModifys[node.NodeID] = modifyData
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"modifys": nodeModifys,
	})
}

// UpdateNodeSettings 更新节点设置
func UpdateNodeSettings(c *gin.Context) {
	// 解析请求体为通用JSON
	var requestData map[string]interface{}
	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// 获取节点ID
	nodeID := c.Param("node_id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "节点ID不能为空",
		})
		return
	}

	// 获取项目ID
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID无效",
		})
		return
	}

	// 将设置转换为JSON字符串
	settingsJSON, err := json.Marshal(requestData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "处理设置数据失败: " + err.Error(),
		})
		return
	}

	// 创建或更新节点的Modifys字段
	err = models.SaveNodeModifys(uint(projectID), nodeID, string(settingsJSON))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "保存节点设置失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "节点设置更新成功",
		"settings": requestData,
	})
}

// GetNodeSettings 获取节点设置
func GetNodeSettings(c *gin.Context) {
	// 获取项目ID
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID无效",
		})
		return
	}

	nodeID := c.Param("node_id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "节点ID不能为空",
		})
		return
	}

	// 获取节点修改信息
	modifys, err := models.GetNodeModifys(uint(projectID), nodeID)
	if err != nil {
		// 如果节点不存在，返回空设置
		if err.Error() == "record not found" {
			c.JSON(http.StatusOK, gin.H{
				"settings": map[string]interface{}{},
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取节点设置失败: " + err.Error(),
		})
		return
	}

	// 解析JSON字符串为map
	var settings map[string]interface{}
	if modifys != "" {
		if err := json.Unmarshal([]byte(modifys), &settings); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "解析节点设置失败: " + err.Error(),
			})
			return
		}
	} else {
		settings = map[string]interface{}{}
	}

	c.JSON(http.StatusOK, gin.H{
		"settings": settings,
	})
}

// DeleteNodeSettings 删除节点设置
// 支持查询参数 subtree=true 来删除节点及其子树的设置
func DeleteNodeSettings(c *gin.Context) {
	userID := middleware.GetUserID(c)

	// 获取项目ID
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID无效",
		})
		return
	}

	nodeID := c.Param("node_id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "节点ID不能为空",
		})
		return
	}

	// 检查是否需要删除子树
	subtree := c.Query("subtree") == "true"

	// 验证项目权限
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在",
		})
		return
	}

	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "您没有权限访问此项目",
		})
		return
	}

	if subtree {
		// 获取用户信息以获取Figma Token
		user, err := models.EnsureUserExists(userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "获取用户信息失败: " + err.Error(),
			})
			return
		}

		if user.FigmaToken == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "用户未设置Figma Token",
			})
			return
		}

		// 如果节点ID为空或等于根节点ID，重置整个项目
		var subtreeNodeIDs []string
		if nodeID == "" || nodeID == project.RootNodeID {
			// 重置根节点及其子树的修改
			subtreeNodeIDs, err = services.GetNodeSubtreeIDs(user.FigmaToken, project.FileKey, project.RootNodeID, "")
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "获取根节点子树失败: " + err.Error(),
				})
				return
			}
		} else {
			// 重置指定节点及其子树的修改
			subtreeNodeIDs, err = services.GetNodeSubtreeIDs(user.FigmaToken, project.FileKey, project.RootNodeID, nodeID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "获取节点子树失败: " + err.Error(),
				})
				return
			}
		}

		// 删除节点及其子树的修改信息，并获取实际被删除的节点ID列表
		actualDeletedNodeIDs, err := models.DeleteNodeSubtreeModifysAndReturnDeleted(uint(projectID), subtreeNodeIDs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "删除节点子树设置失败: " + err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":        fmt.Sprintf("节点及其子树设置删除成功，共删除 %d 个节点的设置", len(actualDeletedNodeIDs)),
			"deleted_count":  len(actualDeletedNodeIDs),
			"subtree":        true,
			"reset_node_ids": actualDeletedNodeIDs,
		})
	} else {
		// 只删除单个节点的修改信息，先检查节点是否有修改
		actualDeletedNodeIDs, err := models.DeleteNodeSubtreeModifysAndReturnDeleted(uint(projectID), []string{nodeID})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "删除节点设置失败: " + err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":        "节点设置删除成功",
			"deleted_count":  len(actualDeletedNodeIDs),
			"subtree":        false,
			"reset_node_ids": actualDeletedNodeIDs,
		})
	}
}

// DeleteProjectNodeSettings 重置项目根节点及其子树的修改
func DeleteProjectNodeSettings(c *gin.Context) {
	userID := middleware.GetUserID(c)

	// 获取项目ID
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID无效",
		})
		return
	}

	// 验证项目是否属于当前用户
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在",
		})
		return
	}

	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "您没有权限访问此项目",
		})
		return
	}

	// 获取用户信息以获取Figma Token
	user, err := models.EnsureUserExists(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取用户信息失败: " + err.Error(),
		})
		return
	}

	if user.FigmaToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "用户未设置Figma Token",
		})
		return
	}

	// 重置根节点及其子树的修改
	subtreeNodeIDs, err := services.GetNodeSubtreeIDs(user.FigmaToken, project.FileKey, project.RootNodeID, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取根节点子树失败: " + err.Error(),
		})
		return
	}

	// 删除根节点及其子树的所有修改，并获取实际被删除的节点ID列表
	actualDeletedNodeIDs, err := models.DeleteNodeSubtreeModifysAndReturnDeleted(uint(projectID), subtreeNodeIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "重置根节点子树修改失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"message":        fmt.Sprintf("项目根节点及其子树修改已全部重置，共重置 %d 个节点", len(actualDeletedNodeIDs)),
		"deleted_count":  len(actualDeletedNodeIDs),
		"reset_node_ids": actualDeletedNodeIDs,
	})
}

// DeleteProjectModifications 删除项目的所有节点修改
func DeleteProjectModifications(c *gin.Context) {
	userID := middleware.GetUserID(c)

	// 获取项目ID
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "项目ID无效",
		})
		return
	}

	// 验证项目是否属于当前用户
	var project models.FigmaProject
	result := models.DB.First(&project, projectID)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "项目不存在",
		})
		return
	}

	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "无权限访问此项目",
		})
		return
	}

	// 删除项目的所有节点修改信息
	err = models.DeleteAllProjectNodeModifys(uint(projectID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "删除节点修改失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "项目节点修改已全部重置",
	})
}

// ExportFigmaDesign 导出Figma设计
func ExportFigmaDesign(c *gin.Context) {
	userID := middleware.GetUserID(c)
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID无效",
		})
		return
	}

	// 解析请求体中的参数
	var requestBody struct {
		Format string  `json:"format"`
		Scale  float64 `json:"scale"`
	}
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		// 如果解析失败，使用默认格式和缩放
		requestBody.Format = "png"
		requestBody.Scale = 1.0
	}

	// 从项目设置获取默认格式和缩放
	var defaultFormat string = "png"
	var defaultScale float64 = 1.0
	project, err := models.GetProjectByID(uint(projectID))
	if err == nil && project.Settings != "" {
		// 尝试将 Settings（JSON字符串）解析为 map[string]interface{}
		var settingsMap map[string]interface{}
		if err := json.Unmarshal([]byte(project.Settings), &settingsMap); err == nil {
			if v, ok := settingsMap["imageFormat"].(string); ok && v != "" {
				defaultFormat = v
			}
			if v, ok := settingsMap["imageScale"].(float64); ok && v > 0 {
				defaultScale = v
			} else if vInt, ok := settingsMap["imageScale"].(int); ok && vInt > 0 {
				defaultScale = float64(vInt)
			}
		}
	}

	// 验证格式是否有效
	format := requestBody.Format
	if format == "" {
		format = defaultFormat // 从项目设置获取默认格式
	}
	if format != "png" && format != "jpg" && format != "svg" {
		format = "png" // 如果不可用则强制为png
	}

	// 验证缩放比例是否有效
	scale := requestBody.Scale
	if scale <= 0 || scale > 4.0 {
		scale = defaultScale // 从项目设置获取默认缩放
	}

	// 先取消该项目的所有进行中的导出任务
	err = models.CancelExportJobsByProject(uint(projectID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "取消之前的导出任务失败: " + err.Error(),
		})
		return
	}

	// 创建导出任务，不需要前端提供路径，使用默认路径
	job := models.ExportJob{
		UserID:    userID,
		ProjectID: uint(projectID),
		Status:    "pending",
		Progress:  0,
		FilePath:  "", // 路径由服务端生成
		Format:    format,
		Scale:     scale,
	}

	result := models.DB.Create(&job)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "创建导出任务失败: " + result.Error.Error(),
		})
		return
	}

	// 启动导出任务（异步）
	go services.ProcessExportJob(job.ID)

	c.JSON(http.StatusOK, gin.H{
		"message": "导出任务已创建",
		"job_id":  job.ID,
	})
}

// GetExportStatus 获取导出状态
func GetExportStatus(c *gin.Context) {
	jobID, err := strconv.ParseUint(c.Param("job_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "任务ID无效",
		})
		return
	}

	// 获取任务状态
	job, err := models.GetExportJob(uint(jobID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "导出任务不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    job.Status,
		"progress":  job.Progress,
		"error":     job.Error,
		"file_path": job.FilePath,
	})
}

// GetProjectActiveExportJob 获取项目的活跃导出任务
func GetProjectActiveExportJob(c *gin.Context) {
	userID := middleware.GetUserID(c)
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID无效",
		})
		return
	}

	// 验证项目是否属于当前用户
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在",
		})
		return
	}

	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "无权访问此项目",
		})
		return
	}

	// 获取项目的活跃导出任务
	jobs, err := models.GetActiveExportJobsByProject(uint(projectID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取导出任务失败",
		})
		return
	}

	// 如果有活跃任务，返回最新的一个
	if len(jobs) > 0 {
		latestJob := jobs[len(jobs)-1] // 获取最新的任务
		c.JSON(http.StatusOK, gin.H{
			"has_active_job": true,
			"job_id":         latestJob.ID,
			"status":         latestJob.Status,
			"progress":       latestJob.Progress,
			"error":          latestJob.Error,
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"has_active_job": false,
		})
	}
}

// DownloadExport 下载导出文件
func DownloadExport(c *gin.Context) {
	jobID, err := strconv.ParseUint(c.Param("job_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "任务ID无效",
		})
		return
	}

	// 获取任务信息
	job, err := models.GetExportJob(uint(jobID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "导出任务不存在",
		})
		return
	}

	// 检查任务是否完成
	if job.Status != "completed" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "导出任务尚未完成",
		})
		return
	}

	// 检查文件是否存在
	if _, err := os.Stat(job.FilePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "导出文件不存在",
		})
		return
	}

	// 获取文件名
	fileName := filepath.Base(job.FilePath)

	// 设置下载头
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", "attachment; filename="+fileName)
	c.Header("Content-Type", "application/octet-stream")
	c.File(job.FilePath)
}

// GetFigmaImage 获取Figma图片
func GetFigmaImage(c *gin.Context) {
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID无效",
		})
		return
	}

	nodeID := c.Param("node_id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "节点ID不能为空",
		})
		return
	}

	// 获取查询参数
	format := c.DefaultQuery("format", "png")         // 默认png格式
	scaleStr := c.DefaultQuery("scale", "1.0")        // 默认1.0标准清晰度
	excludeModified := c.Query("excludeModified")     // 是否排除修改的节点（过滤预览）
	useCacheStr := c.DefaultQuery("useCache", "true") // 是否使用缓存，默认为true

	// 解析缩放比例
	scale, err := strconv.ParseFloat(scaleStr, 64)
	if err != nil || scale <= 0 || scale > 4.0 {
		scale = 1.0 // 如果解析失败或超出范围，使用默认值
		fmt.Printf("Controller: 缩放比例参数无效，使用默认值: 1.0\n")
	}

	// 验证格式
	validFormats := map[string]bool{"png": true, "jpg": true, "svg": true}
	if !validFormats[format] {
		format = "png" // 如果格式无效，使用默认值
		fmt.Printf("Controller: 图片格式参数无效，使用默认值: png\n")
	}

	// 解析是否使用缓存
	useCache := useCacheStr == "true" || useCacheStr == "1"

	fmt.Printf("Controller: 图片参数 - 格式: %s, 缩放比例: %.1f, 排除修改: %s, 使用缓存: %v\n", format, scale, excludeModified, useCache)

	// 获取项目信息
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在",
		})
		return
	}

	// 获取用户信息，确保用户存在
	user, err := models.EnsureUserExists(project.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取用户信息失败: " + err.Error(),
		})
		return
	}

	// 获取图片
	var imagePath string
	if excludeModified == "true" {
		// 过滤预览：排除修改的节点，保存到fpreviews文件夹
		fmt.Printf("Controller: 开始获取过滤后的Figma图片, project_id=%d, node_id=%s, format=%s, scale=%.1f, useCache=%v\n",
			projectID, nodeID, format, scale, useCache)
		imagePath, err = services.DownloadFilteredPreviewFigmaImage(user.FigmaToken, project.FileKey, nodeID, format, scale, uint(projectID), useCache)
	} else {
		// 普通预览：保存到previews文件夹
		fmt.Printf("Controller: 开始获取普通Figma图片, project_id=%d, node_id=%s, format=%s, scale=%.1f, useCache=%v\n",
			projectID, nodeID, format, scale, useCache)
		imagePath, err = services.DownloadPreviewFigmaImageWithOptions(user.FigmaToken, project.FileKey, nodeID, format, scale, useCache)
	}

	if err != nil {
		errMsg := fmt.Sprintf("获取图片失败: %v", err)
		fmt.Println("Controller: " + errMsg)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": errMsg,
		})
		return
	}
	fmt.Printf("Controller: 成功获取Figma图片, path=%s\n", imagePath)

	// 返回图片
	fmt.Printf("Controller: 返回图片文件: %s\n", imagePath)
	c.File(imagePath)
}

// GetFigmaImages 批量获取Figma过滤预览图片（返回节点ID到图片路径的映射）
func GetFigmaImages(c *gin.Context) {
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID无效",
		})
		return
	}

	nodeID := c.Param("node_id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "节点ID不能为空",
		})
		return
	}

	// 获取查询参数
	format := c.DefaultQuery("format", "png")  // 默认png格式
	scaleStr := c.DefaultQuery("scale", "1.0") // 默认1.0标准清晰度

	// 解析缩放比例
	scale, err := strconv.ParseFloat(scaleStr, 64)
	if err != nil || scale <= 0 || scale > 4.0 {
		scale = 1.0 // 如果解析失败或超出范围，使用默认值
		fmt.Printf("Controller: 缩放比例参数无效，使用默认值: 1.0\n")
	}

	// 验证格式
	validFormats := map[string]bool{"png": true, "jpg": true, "svg": true}
	if !validFormats[format] {
		format = "png" // 如果格式无效，使用默认值
		fmt.Printf("Controller: 图片格式参数无效，使用默认值: png\n")
	}

	fmt.Printf("Controller: 批量图片参数 - 格式: %s, 缩放比例: %.1f\n", format, scale)

	// 获取项目信息
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在",
		})
		return
	}

	// 获取用户信息，确保用户存在
	user, err := models.EnsureUserExists(project.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取用户信息失败: " + err.Error(),
		})
		return
	}

	// 批量获取过滤预览图片
	fmt.Printf("Controller: 开始批量获取过滤预览图片 - projectID=%d, nodeID=%s, format=%s, scale=%.1f\n",
		projectID, nodeID, format, scale)

	imagePathMap, err := services.DownloadFilteredPreviewFigmaImages(user.FigmaToken, project.FileKey, nodeID, format, scale, uint(projectID))
	if err != nil {
		errMsg := fmt.Sprintf("批量获取图片失败: %v", err)
		fmt.Println("Controller: " + errMsg)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": errMsg,
		})
		return
	}

	fmt.Printf("Controller: 成功批量获取Figma图片，共 %d 个图片\n", len(imagePathMap))

	// 返回节点ID到图片路径的映射
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"images":  imagePathMap,
		"count":   len(imagePathMap),
	})
}

// GetFigmaNodeInfo 获取Figma节点信息（直接从API获取）
func GetFigmaNodeInfo(c *gin.Context) {
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID无效",
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

	// 获取用户信息，确保用户存在
	user, err := models.EnsureUserExists(project.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取用户信息失败: " + err.Error(),
		})
		return
	}

	// 从API获取节点信息，使用项目的RootNodeID
	nodeID := project.RootNodeID
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目没有设置根节点ID",
		})
		return
	}

	// 从Figma API获取节点数据
	nodes, err := services.GetFigmaNodes(user.FigmaToken, project.FileKey, nodeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取Figma节点数据失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"project": project,
		"nodes":   nodes,
	})
}

// UpdateProject 更新项目
func UpdateProject(c *gin.Context) {
	userID := middleware.GetUserID(c)
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID无效",
		})
		return
	}

	// 检查项目是否属于当前用户
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在",
		})
		return
	}

	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "无权更新该项目",
		})
		return
	}

	// 获取新的项目名称
	name := c.PostForm("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目名称不能为空",
		})
		return
	}

	// 获取分组名称（可选）
	groupName := c.PostForm("group_name")

	// 获取Figma URL（可选）
	figmaURL := c.PostForm("figma_url")

	// 更新项目信息
	project.Name = name
	project.GroupName = groupName
	project.FigmaURL = figmaURL
	if err := models.DB.Save(&project).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "更新项目失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "项目更新成功",
		"project": project,
	})
}

// UpdateProjectSettings 更新项目设置
func UpdateProjectSettings(c *gin.Context) {
	userID := middleware.GetUserID(c)
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID无效",
		})
		return
	}

	// 检查项目是否属于当前用户
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在",
		})
		return
	}

	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "无权更新该项目",
		})
		return
	}

	// 解析请求体为JSON
	var settings map[string]interface{}
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的JSON数据: " + err.Error(),
		})
		return
	}

	// 将设置转换为JSON字符串
	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "处理设置数据失败: " + err.Error(),
		})
		return
	}

	// 更新项目设置
	project.Settings = string(settingsJSON)
	if err := models.DB.Save(&project).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "更新项目设置失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "项目设置更新成功",
		"settings": settings,
	})
}

// DeleteProject 删除项目
func DeleteProject(c *gin.Context) {
	userID := middleware.GetUserID(c)
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID无效",
		})
		return
	}

	// 检查项目是否属于当前用户
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在",
		})
		return
	}

	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "无权删除该项目",
		})
		return
	}

	// 删除项目
	err = models.DeleteProject(uint(projectID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "删除项目失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "项目删除成功",
	})
}

// ClearProjectImageCache 清除项目的图片缓存
func ClearProjectImageCache(c *gin.Context) {
	userID := middleware.GetUserID(c)
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID无效",
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

	// 检查项目是否属于当前用户
	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "无权清除该项目的缓存",
		})
		return
	}

	// 检查是否需要按节点ID清理（通过查询参数smart=true启用）
	smartClear := c.Query("smart") == "true"

	var clearedCount int

	if smartClear {
		// 智能清理：根据项目的节点数据进行精确清理
		fmt.Printf("启用智能缓存清理模式\n")

		// 获取用户信息
		user, err := models.EnsureUserExists(userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "获取用户信息失败: " + err.Error(),
			})
			return
		}

		// 获取项目的优化JSON数据以提取所有节点ID
		optimizedData, err := getOptimizedProjectDataForCache(user, project)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "获取项目数据失败: " + err.Error(),
			})
			return
		}

		// 从JSON数据中提取所有节点ID
		nodeIDs, err := extractAllNodeIDsFromData(optimizedData)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "提取节点ID失败: " + err.Error(),
			})
			return
		}

		// 按节点ID清除缓存
		err = services.ClearProjectImageCacheByNodeIDs(project.FileKey, nodeIDs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "智能清除缓存失败: " + err.Error(),
			})
			return
		}

		// 清除项目导出目录
		exportCleared, err := services.ClearProjectExportCache(uint(projectID))
		if err != nil {
			fmt.Printf("清除项目导出缓存失败: %v\n", err)
		}

		clearedCount = len(nodeIDs)

		c.JSON(http.StatusOK, gin.H{
			"message":      fmt.Sprintf("已成功智能清除项目图片缓存，涉及 %d 个节点，清除 %d 个导出文件", clearedCount, exportCleared),
			"smart_clear":  true,
			"node_count":   clearedCount,
			"export_count": exportCleared,
		})
	} else {
		// 传统清理：清除所有缓存文件
		err = services.ClearProjectImageCache(project.FileKey)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "清除缓存失败: " + err.Error(),
			})
			return
		}

		// 清除项目导出目录
		exportCleared, err := services.ClearProjectExportCache(uint(projectID))
		if err != nil {
			fmt.Printf("清除项目导出缓存失败: %v\n", err)
		}

		c.JSON(http.StatusOK, gin.H{
			"message":      fmt.Sprintf("已成功清除项目图片缓存，清除 %d 个导出文件", exportCleared),
			"smart_clear":  false,
			"export_count": exportCleared,
		})
	}
}

// getOptimizedProjectDataForCache 获取项目的优化数据用于缓存清理
func getOptimizedProjectDataForCache(user *models.User, project *models.FigmaProject) (gin.H, error) {
	// 获取节点树数据
	nodes, err := services.GetFigmaNodes(user.FigmaToken, project.FileKey, project.RootNodeID)
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

	// 构建优化后的节点数据
	optimizedNodes := make([]gin.H, 0)

	for _, node := range nodes {
		nodeID := node["id"].(string)
		nodeName, _ := node["name"].(string)
		nodeType, _ := node["type"].(string)

		// 创建优化节点，包含所有Figma原始数据
		optimizedNode := gin.H{}

		// 复制所有Figma原始属性
		for key, value := range node {
			optimizedNode[key] = value
		}

		// 添加几何信息的简化版本（保持向后兼容）
		if absoluteBoundingBox, ok := node["absoluteBoundingBox"].(map[string]interface{}); ok {
			optimizedNode["bounds"] = gin.H{
				"x":      absoluteBoundingBox["x"],
				"y":      absoluteBoundingBox["y"],
				"width":  absoluteBoundingBox["width"],
				"height": absoluteBoundingBox["height"],
			}
		}

		// 确保可见性信息存在
		if _, ok := node["visible"]; !ok {
			optimizedNode["visible"] = true // 默认可见
		}

		// 添加修改信息（如果存在）
		if modifys, exists := nodeModifys[nodeID]; exists {
			optimizedNode["modifys"] = modifys

			// 如果有重命名，使用重命名后的名称
			if rename, ok := modifys["rename"].(string); ok && rename != "" {
				optimizedNode["displayName"] = rename
			} else {
				optimizedNode["displayName"] = nodeName
			}

			// 添加资源模式信息
			if resMode, ok := modifys["res_mode"].(string); ok {
				optimizedNode["resourceMode"] = resMode
			}

			// 添加忽略标记
			if ignore, ok := modifys["ignore"].(bool); ok {
				optimizedNode["ignored"] = ignore
			}
		} else {
			optimizedNode["displayName"] = nodeName
			optimizedNode["resourceMode"] = "default"
			optimizedNode["ignored"] = false
		}

		// 添加额外的便利属性（基于原始数据）
		if nodeType == "TEXT" {
			if characters, ok := node["characters"].(string); ok {
				optimizedNode["text"] = characters // 便利属性
			}
		}

		optimizedNodes = append(optimizedNodes, optimizedNode)
	}

	// 构建节点映射和子节点映射
	nodeMap := make(map[string]gin.H)
	childrenMap := make(map[string][]gin.H)

	// 先建立所有节点的映射
	for _, node := range optimizedNodes {
		nodeID := node["id"].(string)
		nodeMap[nodeID] = node
	}

	// 构建父子关系
	for _, node := range optimizedNodes {
		parentID := node["parent_id"]

		if parentID != nil && parentID != project.RootNodeID {
			if parentIDStr, ok := parentID.(string); ok {
				childrenMap[parentIDStr] = append(childrenMap[parentIDStr], node)
			}
		}
	}

	// 为每个节点添加子节点信息
	for i := range optimizedNodes {
		nodeID := optimizedNodes[i]["id"].(string)
		if children, hasChildren := childrenMap[nodeID]; hasChildren {
			optimizedNodes[i]["children"] = children
		} else {
			optimizedNodes[i]["children"] = []gin.H{}
		}
	}

	// 只返回根节点（从根节点开始的树形结构）
	treeNodes := make([]gin.H, 0)
	for _, node := range optimizedNodes {
		parentID := node["parent_id"]
		if parentID == nil || parentID == project.RootNodeID {
			treeNodes = append(treeNodes, node)
		}
	}

	// 返回优化后的数据结构（只包含nodes，保持树形结构）
	return gin.H{
		"nodes": treeNodes, // 只返回根节点，每个节点包含完整的子树
	}, nil
}

// extractAllNodeIDsFromData 从优化的JSON数据中提取所有节点ID
func extractAllNodeIDsFromData(optimizedData gin.H) ([]string, error) {
	var nodeIDs []string

	// 从nodes字段中提取节点ID
	if nodes, ok := optimizedData["nodes"].([]gin.H); ok {
		nodeIDs = extractNodeIDsRecursiveFromData(nodes, nodeIDs)
	} else {
		return nil, fmt.Errorf("无法解析nodes数据")
	}

	fmt.Printf("从项目数据中提取到 %d 个节点ID\n", len(nodeIDs))
	return nodeIDs, nil
}

// extractNodeIDsRecursiveFromData 递归提取节点ID（包括子节点）
func extractNodeIDsRecursiveFromData(nodes []gin.H, nodeIDs []string) []string {
	for _, node := range nodes {
		// 提取当前节点的ID
		if id, ok := node["id"].(string); ok {
			nodeIDs = append(nodeIDs, id)
		}

		// 递归处理子节点
		if children, ok := node["children"].([]gin.H); ok && len(children) > 0 {
			nodeIDs = extractNodeIDsRecursiveFromData(children, nodeIDs)
		}
	}
	return nodeIDs
}

// AddRefNode 添加依赖节点
func AddRefNode(c *gin.Context) {
	userID := middleware.GetUserID(c)
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的项目ID",
		})
		return
	}

	// 获取节点ID
	nodeID := c.PostForm("node_id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "节点ID不能为空",
		})
		return
	}

	nodeID = strings.ReplaceAll(nodeID, "-", ":")
	// 获取项目信息并验证权限
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在",
		})
		return
	}

	// 验证项目所有者
	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "您没有权限访问此项目",
		})
		return
	}

	// 添加依赖节点
	err = models.AddRefNode(uint(projectID), nodeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "依赖节点添加成功",
		"data": gin.H{
			"project_id": projectID,
			"node_id":    nodeID,
		},
	})
}

// RemoveRefNode 移除依赖节点
func RemoveRefNode(c *gin.Context) {
	userID := middleware.GetUserID(c)
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的项目ID",
		})
		return
	}

	// 获取节点ID
	nodeID := c.Param("node_id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "节点ID不能为空",
		})
		return
	}

	// 获取项目信息并验证权限
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在",
		})
		return
	}

	// 验证项目所有者
	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "您没有权限访问此项目",
		})
		return
	}

	// 移除依赖节点
	err = models.RemoveRefNode(uint(projectID), nodeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "依赖节点移除成功",
		"data": gin.H{
			"project_id": projectID,
			"node_id":    nodeID,
		},
	})
}

// GetRefNodes 获取依赖节点列表
func GetRefNodes(c *gin.Context) {
	userID := middleware.GetUserID(c)
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的项目ID",
		})
		return
	}

	// 获取项目信息并验证权限
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在",
		})
		return
	}

	// 验证项目所有者
	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "您没有权限访问此项目",
		})
		return
	}

	// 获取依赖节点列表
	refNodes, err := models.GetRefNodes(uint(projectID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取依赖节点失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"project_id": projectID,
			"ref_nodes":  refNodes,
		},
		"message": "获取依赖节点列表成功",
	})
}

// UpdateInterfaceDescription 更新项目界面描述
func UpdateInterfaceDescription(c *gin.Context) {
	userID := middleware.GetUserID(c)
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID无效",
		})
		return
	}

	// 获取请求体中的界面描述
	var requestBody struct {
		InterfaceDescription string `json:"interface_description"`
	}
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求数据格式错误: " + err.Error(),
		})
		return
	}

	// 检查项目是否属于当前用户
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在",
		})
		return
	}

	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "无权更新该项目",
		})
		return
	}

	// 更新界面描述
	project.InterfaceDescription = requestBody.InterfaceDescription
	if err := models.DB.Save(&project).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "更新界面描述失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "界面描述更新成功",
		"data": gin.H{
			"project_id":            projectID,
			"interface_description": requestBody.InterfaceDescription,
		},
	})
}

// GetInterfaceDescription 获取项目界面描述
func GetInterfaceDescription(c *gin.Context) {
	userID := middleware.GetUserID(c)
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID无效",
		})
		return
	}

	// 检查项目是否属于当前用户
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在",
		})
		return
	}

	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "无权访问该项目",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"project_id":            projectID,
			"interface_description": project.InterfaceDescription,
		},
	})
}

// GetRefNodeDetails 获取指定依赖节点的详细信息
func GetRefNodeDetails(c *gin.Context) {
	userID := middleware.GetUserID(c)
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的项目ID",
		})
		return
	}

	// 获取节点ID列表（通过查询参数传递，逗号分隔）
	nodeIDsParam := c.Query("node_ids")
	if nodeIDsParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "节点ID列表不能为空",
		})
		return
	}

	nodeIDs := strings.Split(nodeIDsParam, ",")

	// 获取项目信息并验证权限
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在",
		})
		return
	}

	// 验证项目所有者
	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "您没有权限访问此项目",
		})
		return
	}

	// 获取用户信息
	var user models.User
	result := models.DB.First(&user, userID)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "用户不存在",
		})
		return
	}

	// 如果没有Figma Token，返回错误
	if user.FigmaToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "用户未设置Figma Token",
		})
		return
	}

	// 为每个节点ID获取具体的节点数据
	nodeDetails := make(map[string]interface{})

	for _, nodeID := range nodeIDs {
		nodeID = strings.TrimSpace(nodeID)
		if nodeID == "" {
			continue
		}

		// 使用指定的节点ID获取节点树数据（只获取该节点及其子节点）
		nodeData, err := services.GetFigmaNodes(user.FigmaToken, project.FileKey, nodeID)
		if err != nil {
			// 如果获取失败，记录错误但继续处理其他节点
			fmt.Printf("获取节点 %s 数据失败: %v\n", nodeID, err)
			nodeDetails[nodeID] = gin.H{
				"id":           nodeID,
				"name":         fmt.Sprintf("节点 %s", nodeID),
				"type":         "REFERENCE",
				"error":        true,
				"errorMessage": err.Error(),
			}
			continue
		}

		// 查找根节点（应该是请求的节点ID）
		var rootNode map[string]interface{}
		for _, node := range nodeData {
			if nodeIDStr, ok := node["id"].(string); ok && nodeIDStr == nodeID {
				rootNode = node
				break
			}
		}

		if rootNode != nil {
			// 返回根节点信息和完整的树结构数据
			nodeDetails[nodeID] = gin.H{
				"rootNode": rootNode,
				"treeData": nodeData, // 包含完整的节点树数据
			}
		} else {
			// 如果没有找到对应的节点，返回占位符
			nodeDetails[nodeID] = gin.H{
				"id":       nodeID,
				"name":     fmt.Sprintf("节点 %s", nodeID),
				"type":     "REFERENCE",
				"notFound": true,
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"project_id":   projectID,
			"file_key":     project.FileKey,
			"node_details": nodeDetails,
		},
		"message": "获取依赖节点详细信息成功",
	})
}
