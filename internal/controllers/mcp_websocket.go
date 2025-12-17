package controllers

import (
	"archive/zip"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/figma-deliver/internal/models"
	"github.com/figma-deliver/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// 允许所有来源（开发环境）
		// 生产环境应该限制特定域名
		return true
	},
}

// MCPWebSocketHandler 处理MCP WebSocket连接
func MCPWebSocketHandler(c *gin.Context) {
	// 获取 connection_id (MCP Token)
	privateToken := c.Query("connection_id")
	if privateToken == "" {
		// 也尝试从 Header 中获取
		privateToken = c.GetHeader("X-MCP-Token")
	}

	if privateToken == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "缺少 connection_id"})
		return
	}

	// 通过 connection_id 查找用户
	user, err := models.GetUserByMCPToken(privateToken)
	if err != nil {
		log.Printf("无效的 connection_id: %s, 错误: %v", privateToken, err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的 connection_id"})
		return
	}

	// 使用 MCP Token 作为连接 ID，确保每个用户只有一个连接
	connectionID := privateToken

	log.Printf("WebSocket连接请求: 用户=%s (ID=%d), 连接ID=%s", user.Username, user.ID, connectionID)

	// 升级到WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket升级失败: %v", err)
		return
	}

	// 获取MCP服务
	mcpService := services.GetMCPService()

	// 关闭该用户的所有旧连接，确保只有一个活跃连接
	mcpService.CloseUserConnections(user.ID, connectionID)

	// 创建逻辑连接记录
	if _, err := mcpService.CreateConnection(user.ID, connectionID); err != nil {
		log.Printf("创建MCP逻辑连接失败: %v", err)
		// 继续执行，WebSocket 连接可能仍能工作，但状态查询可能会有问题
	}

	// 处理WebSocket连接
	mcpService.HandleWebSocket(conn, connectionID, user.ID)
}

// MCPWebSocketTest 测试页面
func MCPWebSocketTest(c *gin.Context) {
	c.HTML(http.StatusOK, "mcp_center.html", gin.H{
		"title": "MCP WebSocket 测试中心",
	})
}

// GetMCPConnectionStatus 获取用户的 MCP 连接状态
func GetMCPConnectionStatus(c *gin.Context) {
	// 获取当前用户 ID
	userID, exists := c.Get("user_id")
	if !exists {
		// 未登录时返回未连接状态
		c.JSON(http.StatusOK, gin.H{
			"connected": false,
			"status":    "disconnected",
		})
		return
	}

	// 获取 MCP 服务
	mcpService := services.GetMCPService()

	// 检查用户是否有活跃连接
	connection, exists := mcpService.GetUserConnection(userID.(uint))

	status := "disconnected"
	if exists && connection != nil {
		status = connection.Status
	}

	c.JSON(http.StatusOK, gin.H{
		"connected": exists && status == "connected",
		"status":    status,
		"user_id":   userID.(uint),
		"last_activity": func() interface{} {
			if connection != nil {
				return connection.LastActivity
			}
			return nil
		}(),
	})
}

// DownloadMCPPlugin 下载 MCP Figma 插件
func DownloadMCPPlugin(c *gin.Context) {
	// 获取插件目录路径
	pluginDir := "figma-plugin"

	// 检查目录是否存在
	if _, err := os.Stat(pluginDir); os.IsNotExist(err) {
		log.Printf("插件目录不存在: %s", pluginDir)
		c.JSON(http.StatusNotFound, gin.H{"error": "插件目录不存在"})
		return
	}

	// 创建临时 zip 文件
	zipFileName := "figma-deliver-plugin.zip"
	zipFile, err := os.Create(zipFileName)
	if err != nil {
		log.Printf("创建 zip 文件失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建压缩文件失败"})
		return
	}
	defer func() {
		zipFile.Close()
		// 下载完成后删除临时文件
		os.Remove(zipFileName)
	}()

	// 创建 zip writer
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// 遍历插件目录并添加文件到 zip
	err = filepath.Walk(pluginDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过目录本身
		if info.IsDir() {
			return nil
		}

		// 获取相对路径
		relPath, err := filepath.Rel(pluginDir, path)
		if err != nil {
			return err
		}

		// 创建 zip 文件条目
		writer, err := zipWriter.Create(filepath.Join("figma-plugin", relPath))
		if err != nil {
			return err
		}

		// 打开源文件
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		// 复制文件内容到 zip
		_, err = io.Copy(writer, file)
		return err
	})

	if err != nil {
		log.Printf("压缩插件文件失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "压缩文件失败"})
		return
	}

	// 关闭 zip writer 以确保所有数据都写入
	zipWriter.Close()
	zipFile.Close()

	// 重新打开文件用于下载
	zipFile, err = os.Open(zipFileName)
	if err != nil {
		log.Printf("打开 zip 文件失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "打开压缩文件失败"})
		return
	}
	defer zipFile.Close()

	// 获取文件信息
	fileInfo, err := zipFile.Stat()
	if err != nil {
		log.Printf("获取文件信息失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取文件信息失败"})
		return
	}

	// 设置响应头
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", "attachment; filename=figma-mcp-plugin.zip")
	c.Header("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))

	// 发送文件
	c.File(zipFileName)

	log.Printf("MCP 插件下载成功")
}
