package controllers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/figma-bridge/internal/middleware"
	"github.com/figma-bridge/internal/models"
	"github.com/figma-bridge/internal/services"
	"github.com/gin-gonic/gin"
)

// ParseFigmaLink 解析Figma链接
func ParseFigmaLink(c *gin.Context) {
	link := c.PostForm("link")
	if link == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请提供Figma链接",
		})
		return
	}

	fileKey, nodeId, err := services.ParseFigmaLink(link)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"fileKey": fileKey,
		"nodeId":  nodeId,
	})
}

// GetFigmaNode 获取Figma节点数据
func GetFigmaNode(c *gin.Context) {
	userID := middleware.GetUserID(c)
	fileKey := c.Param("fileKey")
	nodeId := c.Param("nodeId")

	// 获取用户的Figma Token
	var user models.User
	result := models.DB.First(&user, userID)
	if result.Error != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "用户不存在",
		})
		return
	}

	if user.FigmaToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "未设置Figma Token，请在个人资料页面设置",
		})
		return
	}

	// 创建Figma服务
	figmaService := services.NewFigmaService(user.FigmaToken)

	// 获取节点数据
	response, err := figmaService.GetFigmaNode(fileKey, nodeId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// 创建或更新项目
	project, err := models.CreateOrUpdateProject(userID, fileKey, response.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "保存项目失败: " + err.Error(),
		})
		return
	}

	// 处理节点数据
	var document map[string]interface{}
	if nodeId == "" {
		// 如果没有指定节点ID，使用整个文档
		document = response.Document
	} else {
		// 如果指定了节点ID，从响应中获取该节点
		nodes, ok := response.Document["nodes"].(map[string]interface{})
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "无效的节点数据",
			})
			return
		}

		nodeData, ok := nodes[nodeId].(map[string]interface{})
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "节点不存在",
			})
			return
		}

		document = nodeData["document"].(map[string]interface{})
	}

	// 处理节点树
	var parentID *string
	_, err = figmaService.ProcessNode(project.ID, document, parentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "处理节点失败: " + err.Error(),
		})
		return
	}

	// 获取节点树
	var nodes []models.FigmaNode
	models.DB.Where("project_id = ?", project.ID).Find(&nodes)

	c.JSON(http.StatusOK, gin.H{
		"project": project,
		"nodes":   nodes,
	})
}

// UpdateNodeSettings 更新节点设置
func UpdateNodeSettings(c *gin.Context) {
	var settings models.NodeSetting
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	if settings.NodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "节点ID不能为空",
		})
		return
	}

	err := models.SaveNodeSettings(&settings)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "保存节点设置失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "节点设置更新成功",
		"settings": settings,
	})
}

// ExportFigmaDesign 导出Figma设计
func ExportFigmaDesign(c *gin.Context) {
	userID := middleware.GetUserID(c)
	projectID, err := strconv.ParseUint(c.PostForm("project_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的项目ID",
		})
		return
	}

	// 创建导出任务
	job, err := models.CreateExportJob(userID, uint(projectID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "创建导出任务失败: " + err.Error(),
		})
		return
	}

	// 获取用户的Figma Token
	var user models.User
	result := models.DB.First(&user, userID)
	if result.Error != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "用户不存在",
		})
		return
	}

	if user.FigmaToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "未设置Figma Token，请在个人资料页面设置",
		})
		return
	}

	// 创建输出目录
	outputDir := filepath.Join(os.TempDir(), fmt.Sprintf("figma-export-%d", job.ID))
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "创建输出目录失败: " + err.Error(),
		})
		return
	}

	// 创建Figma服务
	figmaService := services.NewFigmaService(user.FigmaToken)

	// 在后台执行导出任务
	go func() {
		err := figmaService.ExportDesign(job, outputDir)
		if err != nil {
			models.UpdateExportJobStatus(job.ID, "failed", 0, "", err.Error())
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"message": "导出任务已启动",
		"job_id":  job.ID,
	})
}

// GetExportStatus 获取导出任务状态
func GetExportStatus(c *gin.Context) {
	exportID, err := strconv.ParseUint(c.Param("exportId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的导出任务ID",
		})
		return
	}

	job, err := models.GetExportJob(uint(exportID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "导出任务不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   job.Status,
		"progress": job.Progress,
		"error":    job.Error,
	})
}

// DownloadExport 下载导出文件
func DownloadExport(c *gin.Context) {
	exportID, err := strconv.ParseUint(c.Param("exportId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的导出任务ID",
		})
		return
	}

	job, err := models.GetExportJob(uint(exportID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "导出任务不存在",
		})
		return
	}

	if job.Status != "completed" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "导出任务尚未完成",
		})
		return
	}

	if job.FilePath == "" {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "导出文件路径为空",
		})
		return
	}

	// 检查文件是否存在
	if _, err := os.Stat(job.FilePath); os.IsNotExist(err) {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "导出文件不存在",
		})
		return
	}

	// 设置文件名
	fileName := fmt.Sprintf("figma-export-%d.zip", job.ID)
	c.FileAttachment(job.FilePath, fileName)
}
