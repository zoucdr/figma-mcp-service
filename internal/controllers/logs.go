package controllers

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/figma-deliver/internal/services"
	"github.com/gin-gonic/gin"
)

var globalLogsOBSService *services.OBSService

// SetLogsOBSService 设置全局OBS服务实例（用于日志管理）
func SetLogsOBSService(obsService *services.OBSService) {
	globalLogsOBSService = obsService
	log.Printf("✅ [Logs] OBS服务已设置")
}

// LogsPage 日志查看页面
func LogsPage(c *gin.Context) {
	// 从URL路径中获取相对路径
	// 路径格式: /tools/logs/{relativePath}
	fullPath := c.Param("path")

	// 移除开头的斜杠
	relativePath := strings.TrimPrefix(fullPath, "/")

	c.HTML(http.StatusOK, "logs.html", gin.H{
		"title":         "日志查看 - Figma Deliver",
		"relative_path": relativePath,
	})
}

// ListLogs 列出日志文件
func ListLogs(c *gin.Context) {
	if globalLogsOBSService == nil || !globalLogsOBSService.IsEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "OBS服务未启用",
		})
		return
	}

	// 从查询参数获取路径
	relativePath := c.DefaultQuery("path", "")

	// 从查询参数获取分页参数
	pageSize := 50 // 默认每页50条
	if pageSizeStr := c.Query("page_size"); pageSizeStr != "" {
		if ps, err := fmt.Sscanf(pageSizeStr, "%d", &pageSize); err == nil && ps == 1 {
			if pageSize <= 0 {
				pageSize = 50
			}
			if pageSize > 1000 {
				pageSize = 1000
			}
		}
	}
	
	marker := c.DefaultQuery("marker", "")

	// 构建完整的OBS路径前缀
	var prefix string
	if relativePath != "" && relativePath != "/" {
		prefix = filepath.Join("logs", relativePath) + "/"
	} else {
		prefix = "logs/"
	}

	// Windows路径分隔符转换为Unix风格
	prefix = filepath.ToSlash(prefix)

	log.Printf("📂 [Logs] 列出日志文件: %s (page_size=%d, marker=%s)", prefix, pageSize, marker)

	// 列出对象（带分页）
	objects, nextMarker, hasMore, err := globalLogsOBSService.ListObjectsInPathWithPagination(prefix, pageSize, marker)
	if err != nil {
		log.Printf("❌ [Logs] 列出日志失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("列出日志失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"path":        relativePath,
		"objects":     objects,
		"next_marker": nextMarker,
		"has_more":    hasMore,
	})
}

// UploadLog 上传日志文件
func UploadLog(c *gin.Context) {
	if globalLogsOBSService == nil || !globalLogsOBSService.IsEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "OBS服务未启用",
		})
		return
	}

	// 获取相对路径（从表单或查询参数）
	relativePath := c.DefaultPostForm("path", "")
	if relativePath == "" {
		relativePath = c.DefaultQuery("path", "")
	}

	// 获取上传的文件
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		log.Printf("❌ [Logs] 获取上传文件失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "获取上传文件失败",
		})
		return
	}
	defer file.Close()

	// 读取文件内容
	fileData, err := io.ReadAll(file)
	if err != nil {
		log.Printf("❌ [Logs] 读取文件内容失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "读取文件内容失败",
		})
		return
	}

	fileName := header.Filename
	log.Printf("📤 [Logs] 上传日志文件: %s (路径: %s, 大小: %d bytes)", fileName, relativePath, len(fileData))

	// 上传到OBS
	objectKey, err := globalLogsOBSService.UploadLogFile(fileData, relativePath, fileName)
	if err != nil {
		log.Printf("❌ [Logs] 上传失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("上传失败: %v", err),
		})
		return
	}

	// 生成访问URL
	obsURL := globalLogsOBSService.GenerateOBSURL(objectKey)

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"object_key": objectKey,
		"url":        obsURL,
		"size":       len(fileData),
	})
}

// DownloadLog 下载日志文件
func DownloadLog(c *gin.Context) {
	if globalLogsOBSService == nil || !globalLogsOBSService.IsEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "OBS服务未启用",
		})
		return
	}

	// 从查询参数获取对象键
	objectKey := c.Query("key")
	if objectKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "缺少对象键参数",
		})
		return
	}

	log.Printf("📥 [Logs] 下载日志文件: %s", objectKey)

	// 从OBS下载
	data, contentType, err := globalLogsOBSService.DownloadLogFile(objectKey)
	if err != nil {
		log.Printf("❌ [Logs] 下载失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("下载失败: %v", err),
		})
		return
	}

	// 获取文件名
	fileName := filepath.Base(objectKey)

	// 返回文件
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	c.Data(http.StatusOK, contentType, data)
}

// ViewLog 在线查看日志文件内容
func ViewLog(c *gin.Context) {
	if globalLogsOBSService == nil || !globalLogsOBSService.IsEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "OBS服务未启用",
		})
		return
	}

	// 从查询参数获取对象键
	objectKey := c.Query("key")
	if objectKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "缺少对象键参数",
		})
		return
	}

	log.Printf("👁️ [Logs] 查看日志文件: %s", objectKey)

	// 从OBS下载
	data, _, err := globalLogsOBSService.DownloadLogFile(objectKey)
	if err != nil {
		log.Printf("❌ [Logs] 下载失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("下载失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"content": string(data),
	})
}

// DeleteDirectory 删除目录下的所有文件和子目录
func DeleteDirectory(c *gin.Context) {
	if globalLogsOBSService == nil || !globalLogsOBSService.IsEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "OBS服务未启用",
		})
		return
	}

	// 从查询参数获取路径
	relativePath := c.Query("path")
	if relativePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "缺少路径参数",
		})
		return
	}

	// 构建完整的OBS路径前缀
	var prefix string
	if relativePath != "" && relativePath != "/" {
		prefix = filepath.Join("logs", relativePath) + "/"
	} else {
		prefix = "logs/"
	}

	// Windows路径分隔符转换为Unix风格
	prefix = filepath.ToSlash(prefix)

	log.Printf("🗑️ [Logs] 删除目录: %s", prefix)

	// 删除目录下的所有对象
	deleteCount, err := globalLogsOBSService.DeleteDirectory(prefix)
	if err != nil {
		log.Printf("❌ [Logs] 删除失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("删除失败: %v", err),
		})
		return
	}

	log.Printf("✅ [Logs] 删除成功: %d 个对象", deleteCount)

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"deleted_count": deleteCount,
		"message":       fmt.Sprintf("成功删除 %d 个对象", deleteCount),
	})
}
