package controllers

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/figma-deliver/internal/middleware"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// TextClip 文本剪贴板模型
type TextClip struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	UpdatedAt string `json:"updated_at"`
}

// GlobalTextClips 全局文本剪贴板存储 (内存)
var (
	GlobalTextClips = []TextClip{
		{
			ID:        "default",
			Title:     "📝 主剪贴板",
			Content:   "",
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}
	textClipsLock sync.RWMutex
)

// ToolsIndexPage 渲染主页
func ToolsIndexPage(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	username := session.Get("username")
	
	data := gin.H{
		"title": "Figma Deliver - 让设计交付更高效",
	}
	
	// 如果用户已登录，添加用户信息
	if userID != nil {
		data["logged_in"] = true
		if username != nil {
			usernameStr := username.(string)
			data["username"] = usernameStr
			// 获取用户名首字母作为头像
			if len(usernameStr) > 0 {
				data["user_initial"] = string([]rune(usernameStr)[0])
			} else {
				data["user_initial"] = "U"
			}
		} else {
			data["username"] = "用户"
			data["user_initial"] = "U"
		}
	} else {
		data["logged_in"] = false
	}
	
	c.HTML(http.StatusOK, "tools_index.html", data)
}

// ShareToolsPage 渲染共享工具页面
func ShareToolsPage(c *gin.Context) {
	// 获取文件列表
	var files []string
	if globalOBSService != nil && globalOBSService.IsEnabled() {
		objects, err := globalOBSService.ListObjectsInPath("file_share/")
		if err == nil {
			for _, obj := range objects {
				if !obj.IsDir {
					files = append(files, obj.Name)
				}
			}
			// 排序
			sort.Strings(files)
		}
	}

	textClipsLock.RLock()
	clips := make([]TextClip, len(GlobalTextClips))
	copy(clips, GlobalTextClips)
	textClipsLock.RUnlock()

	// 获取服务器地址
	host := c.Request.Host
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	serverAddr := fmt.Sprintf("%s://%s", scheme, host)

	// 获取 CSRF token
	session := sessions.Default(c)
	csrfToken := session.Get("csrf_token")
	if csrfToken == nil {
		// 如果会话中没有CSRF令牌，则生成一个
		token, err := middleware.GenerateRandomString(32)
		if err == nil {
			session.Set("csrf_token", token)
			session.Save()
			csrfToken = token
		}
	}

	c.HTML(http.StatusOK, "share_tools.html", gin.H{
		"title":       "局域网共享中心",
		"text_clips":  clips,
		"files":       files,
		"server_addr": serverAddr,
		"csrf_token":  csrfToken,
	})
}

// GetTextClipsAPI 获取文本剪贴板列表 API
func GetTextClipsAPI(c *gin.Context) {
	textClipsLock.RLock()
	defer textClipsLock.RUnlock()
	c.JSON(http.StatusOK, GlobalTextClips)
}

// UpdateTextClipAPI 更新文本剪贴板 API
func UpdateTextClipAPI(c *gin.Context) {
	id := c.PostForm("id")
	content := c.PostForm("content")

	textClipsLock.Lock()
	defer textClipsLock.Unlock()

	for i := range GlobalTextClips {
		if GlobalTextClips[i].ID == id {
			GlobalTextClips[i].Content = content
			GlobalTextClips[i].UpdatedAt = time.Now().Format(time.RFC3339)
			c.JSON(http.StatusOK, gin.H{
				"status":     "success",
				"updated_at": GlobalTextClips[i].UpdatedAt,
			})
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "ID not found"})
}

// AddTextClip 添加新的文本剪贴板
func AddTextClip(c *gin.Context) {
	title := c.PostForm("title")
	if title == "" {
		title = "新剪贴板"
	}

	// 使用middleware.GenerateRandomString生成ID
	id, _ := middleware.GenerateRandomString(8)
	
	newClip := TextClip{
		ID:        id,
		Title:     title,
		Content:   "",
		UpdatedAt: time.Now().Format(time.RFC3339),
	}

	textClipsLock.Lock()
	GlobalTextClips = append(GlobalTextClips, newClip)
	textClipsLock.Unlock()

	c.Redirect(http.StatusFound, "/tools/share")
}

// DeleteTextClip 删除文本剪贴板
func DeleteTextClip(c *gin.Context) {
	id := c.Param("id")
	if id == "default" {
		c.Redirect(http.StatusFound, "/tools/share")
		return
	}

	textClipsLock.Lock()
	newClips := make([]TextClip, 0, len(GlobalTextClips))
	for _, clip := range GlobalTextClips {
		if clip.ID != id {
			newClips = append(newClips, clip)
		}
	}
	GlobalTextClips = newClips
	textClipsLock.Unlock()

	c.Redirect(http.StatusFound, "/tools/share")
}

// UploadSharedFile 上传共享文件
func UploadSharedFile(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.Redirect(http.StatusFound, "/tools/share")
		return
	}

	if globalOBSService == nil || !globalOBSService.IsEnabled() {
		c.String(http.StatusInternalServerError, "OBS服务未启用")
		return
	}

	f, err := file.Open()
	if err != nil {
		c.String(http.StatusInternalServerError, "无法读取文件")
		return
	}
	defer f.Close()

	fileData, err := io.ReadAll(f)
	if err != nil {
		c.String(http.StatusInternalServerError, "无法读取文件内容")
		return
	}

	_, err = globalOBSService.UploadSharedFile(fileData, file.Filename)
	if err != nil {
		c.String(http.StatusInternalServerError, "上传失败: "+err.Error())
		return
	}

	c.Redirect(http.StatusFound, "/tools/share")
}

// DownloadSharedFile 下载共享文件
func DownloadSharedFile(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" {
		c.String(http.StatusBadRequest, "文件名不能为空")
		return
	}

	if globalOBSService == nil || !globalOBSService.IsEnabled() {
		c.String(http.StatusInternalServerError, "OBS服务未启用")
		return
	}

	objectKey := filepath.Join("file_share", filename)
	objectKey = filepath.ToSlash(objectKey)

	data, contentType, err := globalOBSService.DownloadFile(objectKey)
	if err != nil {
		c.String(http.StatusNotFound, "文件未找到或下载失败")
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Data(http.StatusOK, contentType, data)
}

// DeleteSharedFile 删除共享文件
func DeleteSharedFile(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" {
		c.Redirect(http.StatusFound, "/tools/share")
		return
	}

	if globalOBSService == nil || !globalOBSService.IsEnabled() {
		c.String(http.StatusInternalServerError, "OBS服务未启用")
		return
	}

	objectKey := filepath.Join("file_share", filename)
	objectKey = filepath.ToSlash(objectKey)

	err := globalOBSService.DeleteObject(objectKey)
	if err != nil {
		fmt.Printf("删除文件失败: %v\n", err)
	}

	c.Redirect(http.StatusFound, "/tools/share")
}

// ClearSharedFiles 清空共享文件
func ClearSharedFiles(c *gin.Context) {
	if globalOBSService == nil || !globalOBSService.IsEnabled() {
		c.String(http.StatusInternalServerError, "OBS服务未启用")
		return
	}

	// 使用 DeleteDirectory 删除 file_share/ 目录下的所有内容
	_, err := globalOBSService.DeleteDirectory("file_share/")
	if err != nil {
		fmt.Printf("清空文件失败: %v\n", err)
	}

	c.Redirect(http.StatusFound, "/tools/share")
}

// GetSharedFilesListAPI 获取共享文件列表 API
func GetSharedFilesListAPI(c *gin.Context) {
	var files []string
	
	if globalOBSService != nil && globalOBSService.IsEnabled() {
		objects, err := globalOBSService.ListObjectsInPath("file_share/")
		if err == nil {
			for _, obj := range objects {
				if !obj.IsDir {
					files = append(files, obj.Name)
				}
			}
			// 排序
			sort.Strings(files)
		}
	}
	
	c.JSON(http.StatusOK, files)
}

