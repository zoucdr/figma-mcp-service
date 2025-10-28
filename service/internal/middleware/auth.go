package middleware

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// RequireLogin 检查用户是否已登录
func RequireLogin() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user_id")
		if userID == nil {
			// 如果是API请求，返回401状态码
			if c.Request.Header.Get("X-Requested-With") == "XMLHttpRequest" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
				c.Abort()
				return
			}
			// 否则重定向到登录页面
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		c.Next()
	}
}

// CSRF 生成和验证CSRF令牌
func CSRF() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		csrfToken := session.Get("csrf_token")

		// 如果会话中没有CSRF令牌，则生成一个
		if csrfToken == nil {
			token, err := GenerateRandomString(32)
			if err != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			session.Set("csrf_token", token)
			session.Save()
			csrfToken = token
		}

		// 对于非GET请求，验证CSRF令牌
		if c.Request.Method != "GET" && c.Request.Method != "HEAD" && c.Request.Method != "OPTIONS" {
			requestToken := c.Request.Header.Get("X-CSRF-Token")
			if requestToken == "" {
				requestToken = c.PostForm("_csrf")
			}

			if requestToken != csrfToken {
				c.JSON(http.StatusForbidden, gin.H{"error": "CSRF验证失败"})
				c.Abort()
				return
			}
		}

		// 将CSRF令牌添加到响应头和模板数据中
		c.Header("X-CSRF-Token", csrfToken.(string))
		c.Set("csrf_token", csrfToken)
		c.Next()
	}
}

// GenerateRandomString 生成指定长度的随机字符串（导出供其他包使用）
func GenerateRandomString(length int) (string, error) {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// GetUserID 从会话中获取用户ID
func GetUserID(c *gin.Context) uint {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		return 0
	}
	return userID.(uint)
}
