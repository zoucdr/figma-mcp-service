package controllers

import (
	"net/http"
	"time"

	"github.com/figma-bridge/internal/models"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// LoginPage 登录页面
func LoginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{
		"title": "登录 - Figma Bridge",
	})
}

// Login 用户登录
func Login(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	// 如果用户名为空，允许使用任意用户名登录
	if username == "" {
		// 使用profile/update接口进行注册
		c.Redirect(http.StatusFound, "/profile/update")
		return
	}

	user, err := models.FindUserByUsername(username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "用户不存在",
		})
		return
	}

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
	session.Save()

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
	figmaToken := c.PostForm("figma_token")

	// 如果用户名不为空，检查是否已存在
	if username != "" {
		_, err := models.FindUserByUsername(username)
		if err == nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "用户名已存在",
			})
			return
		}
	}

	// 创建新用户
	user, err := models.CreateUser(username, password, figmaToken)
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

	var user models.User
	models.DB.First(&user, userID)

	c.HTML(http.StatusOK, "profile.html", gin.H{
		"title":      "个人资料 - Figma Bridge",
		"username":   username,
		"figmaToken": user.FigmaToken,
		"timestamp":  time.Now().Unix(),
	})
}

// UpdateProfile 更新用户个人资料
func UpdateProfile(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	username := c.PostForm("username")
	figmaToken := c.PostForm("figma_token")

	// 如果是新用户（从登录页空用户名跳转过来）
	if userID == nil {
		// 创建新用户，使用随机密码
		password := generateRandomPassword()
		user, err := models.CreateUser(username, password, figmaToken)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

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
	err := user.UpdateProfile(username, figmaToken)
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
