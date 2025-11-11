package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/figma-bridge/internal/middleware"
	"github.com/figma-bridge/internal/models"
	"github.com/figma-bridge/internal/services"
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

	c.JSON(http.StatusOK, gin.H{
		"nodes": nodes,
	})
}

// RefreshFigmaNodeTree 刷新Figma节点树
func RefreshFigmaNodeTree(c *gin.Context) {
	// 重定向到GetFigmaNodeTree，但添加refresh=true参数
	projectID := c.Param("project_id")
	c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("/figma/project/%s/nodes?refresh=true", projectID))
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
func DeleteNodeSettings(c *gin.Context) {
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

	// 删除节点修改信息
	err = models.DeleteNodeModifys(uint(projectID), nodeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "删除节点设置失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "节点设置删除成功",
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
		Format string `json:"format"`
	}
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		// 如果解析失败，使用默认格式
		requestBody.Format = "png"
	}

	// 验证格式是否有效
	format := requestBody.Format
	if format == "" || (format != "png" && format != "jpg" && format != "svg" && format != "pdf") {
		format = "png" // 默认使用png格式
	}

	// 创建导出任务，不需要前端提供路径，使用默认路径
	job := models.ExportJob{
		UserID:    userID,
		ProjectID: uint(projectID),
		Status:    "pending",
		Progress:  0,
		FilePath:  "", // 路径由服务端生成
		Format:    format,
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
	format := c.DefaultQuery("format", "png")     // 默认png格式
	scaleStr := c.DefaultQuery("scale", "1.0")    // 默认1.0标准清晰度
	excludeModified := c.Query("excludeModified") // 是否排除修改的节点（过滤预览）

	// 解析缩放比例
	scale, err := strconv.ParseFloat(scaleStr, 64)
	if err != nil || scale <= 0 || scale > 4.0 {
		scale = 1.0 // 如果解析失败或超出范围，使用默认值
		fmt.Printf("Controller: 缩放比例参数无效，使用默认值: 1.0\n")
	}

	// 验证格式
	validFormats := map[string]bool{"png": true, "jpg": true, "svg": true, "pdf": true}
	if !validFormats[format] {
		format = "png" // 如果格式无效，使用默认值
		fmt.Printf("Controller: 图片格式参数无效，使用默认值: png\n")
	}

	fmt.Printf("Controller: 图片参数 - 格式: %s, 缩放比例: %.1f, 排除修改: %s\n", format, scale, excludeModified)

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
		fmt.Printf("Controller: 开始获取过滤后的Figma图片, project_id=%d, node_id=%s, format=%s, scale=%.1f\n",
			projectID, nodeID, format, scale)
		imagePath, err = services.DownloadFilteredPreviewFigmaImage(user.FigmaToken, project.FileKey, nodeID, format, scale, uint(projectID))
	} else {
		// 普通预览：保存到previews文件夹
		fmt.Printf("Controller: 开始获取普通Figma图片, project_id=%d, node_id=%s, format=%s, scale=%.1f\n",
			projectID, nodeID, format, scale)
		imagePath, err = services.DownloadPreviewFigmaImageWithOptions(user.FigmaToken, project.FileKey, nodeID, format, scale)
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
	validFormats := map[string]bool{"png": true, "jpg": true, "svg": true, "pdf": true}
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

	// 获取Figma URL（可选）
	figmaURL := c.PostForm("figma_url")

	// 更新项目信息
	project.Name = name
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

	// 清除项目的图片缓存
	err = services.ClearProjectImageCache(project.FileKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "清除缓存失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "已成功清除项目图片缓存",
	})
}
