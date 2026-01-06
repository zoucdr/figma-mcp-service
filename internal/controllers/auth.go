package controllers

import (
	"log"
	"net/http"
	"time"

	"github.com/figma-deliver/internal/models"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// LoginPage 登录页面
func LoginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{
		"title": "登录 - Figma Deliver",
	})
}

// Login 用户登录
func Login(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	// 如果用户名为空，不允许登录
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "用户名不能为空",
		})
		return
	}

	user, err := models.FindUserByUsername(username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "用户不存在",
		})
		return
	}

	// 添加密码验证
	if !user.CheckPassword(password) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "密码错误",
		})
		return
	}

	// 设置会话
	session := sessions.Default(c)
	session.Set("user_id", user.ID)
	session.Set("username", user.Username)
	err = session.Save()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "会话保存失败: " + err.Error(),
		})
		return
	}

	// 打印会话信息
	c.JSON(http.StatusOK, gin.H{
		"message": "登录成功",
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
		},
	})
}

// Register 用户注册
func Register(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")
	figmaToken := c.PostForm("figma_token") // 可选字段

	// 验证必要字段
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "用户名不能为空",
		})
		return
	}

	if password == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "密码不能为空",
		})
		return
	}

	// 检查用户名是否已存在
	_, err := models.FindUserByUsername(username)
	if err == nil {
		// 用户已存在，不允许自动登录，需要输入密码
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "用户名已存在，请使用登录功能",
		})
		return
	}

	// 创建新用户（figmaToken 可以为空）
	var user *models.User
	user, err = models.CreateUser(username, password, figmaToken)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// 设置会话
	session := sessions.Default(c)
	session.Set("user_id", user.ID)
	session.Set("username", user.Username)
	session.Save()

	c.JSON(http.StatusOK, gin.H{
		"message": "注册成功",
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
		},
	})
}

// Logout 用户登出
func Logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Save()
	c.Redirect(http.StatusFound, "/login")
}

// Profile 用户个人资料页面
func Profile(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id").(uint)
	username := session.Get("username").(string)
	csrfToken := session.Get("csrf_token")

	var user models.User
	models.DB.First(&user, userID)

	// 直接从用户表获取MCP Token
	mcpToken := user.MCPToken

	c.HTML(http.StatusOK, "standalone.html", gin.H{
		"title":        "个人资料 - Figma Deliver",
		"username":     username,
		"figmaToken":   user.FigmaToken,
		"compTypes":    user.CompTypes,
		"prompts":      user.Prompts,
		"codePrompts":  user.CodePrompts,
		"uiCreater":    user.UICreater,
		"mcpToken":     mcpToken,
		"proxyEnabled": user.ProxyEnabled,
		"proxyUrl":     user.ProxyURL,
		"timestamp":    time.Now().Unix(),
		"csrf_token":   csrfToken,
		"template":     "profile",
	})
}

// UpdateProfile 更新用户个人资料
func UpdateProfile(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	username := c.PostForm("username")
	figmaToken := c.PostForm("figma_token")
	compTypes := c.PostForm("comp_types")
	prompts := c.PostForm("prompts")
	codePrompts := c.PostForm("code_prompts")
	uiCreater := c.PostForm("ui_creater")
	proxyEnabled := c.PostForm("proxy_enabled") == "true"
	proxyURL := c.PostForm("proxy_url")

	// 如果用户名不为空，检查是否已存在
	if username != "" {
		existingUser, err := models.FindUserByUsername(username)
		if err == nil && (userID == nil || existingUser.ID != userID.(uint)) {
			// 用户已存在，直接登录
			session.Set("user_id", existingUser.ID)
			session.Set("username", existingUser.Username)
			session.Save()

			c.JSON(http.StatusOK, gin.H{
				"message": "用户已存在，已自动登录",
				"user": gin.H{
					"id":       existingUser.ID,
					"username": existingUser.Username,
				},
			})
			return
		}
	}

	// 如果是新用户（从登录页空用户名跳转过来）
	if userID == nil {
		// 验证用户名不能为空
		if username == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "用户名不能为空",
			})
			return
		}

		// 创建新用户，使用随机密码（figmaToken 可以为空）
		password := generateRandomPassword()
		user, err := models.CreateUser(username, password, figmaToken)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		// 更新控件类型列表和提示词
		user.CompTypes = compTypes
		user.Prompts = prompts
		user.CodePrompts = codePrompts
		user.UICreater = uiCreater
		user.ProxyEnabled = proxyEnabled
		user.ProxyURL = proxyURL
		models.DB.Save(&user)

		// 设置会话
		session.Set("user_id", user.ID)
		session.Set("username", user.Username)
		session.Save()

		c.JSON(http.StatusOK, gin.H{
			"message": "用户创建成功",
			"user": gin.H{
				"id":       user.ID,
				"username": user.Username,
			},
		})
		return
	}

	// 现有用户更新资料
	var user models.User
	result := models.DB.First(&user, userID)
	if result.Error != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "用户不存在",
		})
		return
	}

	// 更新用户资料
	err := user.UpdateProfileWithPrompts(username, figmaToken, compTypes, prompts, codePrompts, uiCreater, proxyEnabled, proxyURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// 更新会话中的用户名
	if username != "" {
		session.Set("username", username)
		session.Save()
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "个人资料更新成功",
	})
}

// GenerateMCPToken 生成新的MCP Token
func GenerateMCPToken(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "用户未登录",
		})
		return
	}

	// 获取用户信息
	var user models.User
	err := models.DB.First(&user, userID.(uint)).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "用户不存在",
		})
		return
	}

	// 生成新的token
	newToken, err := user.GenerateNewMCPToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Token生成失败: " + err.Error(),
		})
		return
	}

	// 检查是否已有MCP连接
	var existingConnection models.MCPConnection
	err = models.DB.Where("user_id = ? AND is_active = ?", userID.(uint), true).First(&existingConnection).Error

	if err == nil {
		// 更新现有连接的token
		existingConnection.ConnectionID = newToken
		existingConnection.LastActivity = time.Now()
		models.DB.Save(&existingConnection)
	} else {
		// 创建新的MCP连接
		connection := models.MCPConnection{
			UserID:       userID.(uint),
			ConnectionID: newToken,
			Status:       "connected",
			IsActive:     true,
			LastActivity: time.Now(),
		}
		models.DB.Create(&connection)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"token":   newToken,
		"message": "MCP Token生成成功",
	})
}

// RevokeMCPToken 撤销MCP Token
func RevokeMCPToken(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "用户未登录",
		})
		return
	}

	// 清空用户表中的MCP Token
	models.DB.Model(&models.User{}).Where("id = ?", userID.(uint)).Update("mcp_token", "")

	// 将所有该用户的MCP连接设为非活跃状态
	models.DB.Model(&models.MCPConnection{}).Where("user_id = ?", userID.(uint)).Update("is_active", false)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "MCP Token已撤销",
	})
}

// GetMCPToken 获取当前用户的 MCP Token
func GetMCPToken(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "用户未登录",
		})
		return
	}

	// 获取用户信息
	var user models.User
	err := models.DB.First(&user, userID.(uint)).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "用户不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"mcp_token": user.MCPToken,
	})
}

// GetCurrentUser 获取当前用户信息
func GetCurrentUser(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "用户未登录",
		})
		return
	}

	// 获取用户信息
	var user models.User
	err := models.DB.First(&user, userID.(uint)).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "用户不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":            user.ID,
			"username":      user.Username,
			"figma_token":   user.FigmaToken,
			"mcp_token":     user.MCPToken,
			"proxy_enabled": user.ProxyEnabled,
		},
	})
}

// ChangePassword 修改密码
func ChangePassword(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "用户未登录",
		})
		return
	}

	oldPassword := c.PostForm("old_password")
	newPassword := c.PostForm("new_password")

	// 验证必要字段
	if oldPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "旧密码不能为空",
		})
		return
	}

	if newPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "新密码不能为空",
		})
		return
	}

	// 获取用户信息
	var user models.User
	err := models.DB.First(&user, userID.(uint)).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "用户不存在",
		})
		return
	}

	// 验证旧密码
	if !user.CheckPassword(oldPassword) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "旧密码错误",
		})
		return
	}

	// 设置新密码
	err = user.SetPassword(newPassword)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// 保存到数据库
	err = models.DB.Save(&user).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "密码更新失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "密码修改成功",
	})
}

// 生成随机密码
func generateRandomPassword() string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()_+"
	result := make([]byte, 12) // 12位密码

	for i := 0; i < 12; i++ {
		result[i] = chars[time.Now().UnixNano()%int64(len(chars))]
		time.Sleep(time.Nanosecond)
	}

	return string(result)
}

// GetTokenCooldownInfo 获取Token冷却信息
// GET /profile/api/cooldown-info
func GetTokenCooldownInfo(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	// 获取用户信息
	user, err := models.FindUserByID(userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户信息失败"})
		return
	}

	if user.FigmaToken == "" {
		c.JSON(http.StatusOK, gin.H{
			"success":                  true,
			"last_file_request_time":   0,
			"last_image_request_time":  0,
			"file_cooldown_remaining":  0,
			"image_cooldown_remaining": 0,
		})
		return
	}

	// 查询或创建冷却记录
	cooldown, err := models.GetOrCreateTokenCooldown(user.FigmaToken)
	if err != nil {
		// 查询失败，返回默认值
		c.JSON(http.StatusOK, gin.H{
			"success":                  true,
			"last_file_request_time":   0,
			"last_image_request_time":  0,
			"file_cooldown_remaining":  0,
			"image_cooldown_remaining": 0,
		})
		return
	}

	// 计算剩余冷却时间（默认冷却30秒）
	now := uint32(time.Now().Unix())
	fileRemaining := int64(0)
	imageRemaining := int64(0)
	cooldownSeconds := uint32(30) // 默认冷却30秒

	if cooldown.LastFileRequestTime > now {
		// 未来时间（来自Retry-After）
		fileRemaining = int64(cooldown.LastFileRequestTime - now)
	} else if now-cooldown.LastFileRequestTime < cooldownSeconds {
		// 正常冷却中
		fileRemaining = int64(cooldownSeconds) - int64(now-cooldown.LastFileRequestTime)
	}

	if cooldown.LastImageRequestTime > now {
		// 未来时间（来自Retry-After）
		imageRemaining = int64(cooldown.LastImageRequestTime - now)
	} else if now-cooldown.LastImageRequestTime < cooldownSeconds {
		// 正常冷却中
		imageRemaining = int64(cooldownSeconds) - int64(now-cooldown.LastImageRequestTime)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":                  true,
		"last_file_request_time":   cooldown.LastFileRequestTime,
		"last_image_request_time":  cooldown.LastImageRequestTime,
		"file_cooldown_remaining":  fileRemaining,
		"image_cooldown_remaining": imageRemaining,
	})
}

// ResetTokenCooldown 重置Token冷却时间
// POST /profile/api/reset-cooldown
func ResetTokenCooldown(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	// 获取用户信息
	user, err := models.FindUserByID(userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户信息失败"})
		return
	}

	if user.FigmaToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请先在个人资料页面配置 Figma Token"})
		return
	}

	// 重置冷却时间（将时间设置为0，表示很久之前，确保下次请求不受冷却限制）
	// 查询或创建冷却记录
	cooldown, err := models.GetOrCreateTokenCooldown(user.FigmaToken)
	if err != nil {
		log.Printf("❌ [ResetTokenCooldown] 获取冷却记录失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "重置失败"})
		return
	}

	// 更新冷却记录（设置为0，表示很久之前）
	cooldown.LastFileRequestTime = 0
	cooldown.LastImageRequestTime = 0
	if err := models.DB.Save(cooldown).Error; err != nil {
		log.Printf("❌ [ResetTokenCooldown] 更新冷却记录失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "重置失败"})
		return
	}

	log.Printf("✅ [ResetTokenCooldown] Token冷却已重置: %s...", user.FigmaToken[:10])

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "冷却时间已重置",
	})
}
