package middleware

import (
	"crypto/rand"
	"encoding/base64"
	"log"
	"net/http"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// RequireLogin 检查用户是否已登录
func RequireLogin() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user_id")
		log.Printf("RequireLogin中间件: 路径=%s, userID=%v", c.Request.URL.Path, userID)

		if userID == nil {
			// 如果是API请求，返回401状态码
			if c.Request.Header.Get("X-Requested-With") == "XMLHttpRequest" {
				log.Println("API请求未登录，返回401")
				c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
				c.Abort()
				return
			}
			// 否则重定向到登录页面
			log.Println("页面请求未登录，重定向到/login")
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		log.Printf("用户已登录，userID=%v，继续处理请求", userID)
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

		// 跳过MCP路径的CSRF验证
		if strings.HasPrefix(c.Request.URL.Path, "/mcp/") || strings.HasPrefix(c.Request.URL.Path, "/mcp-api/") {
			c.Next()
			return
		}

		// 对于非GET请求，验证CSRF令牌
		if c.Request.Method != "GET" && c.Request.Method != "HEAD" && c.Request.Method != "OPTIONS" {
			requestToken := c.Request.Header.Get("X-CSRF-Token")
			if requestToken == "" {
				requestToken = c.PostForm("_csrf")
			}

			log.Printf("请求CSRF令牌: %s, 会话CSRF令牌: %s", requestToken, csrfToken)

			if requestToken != csrfToken {
				log.Printf("CSRF验证失败: 令牌不匹配")
				c.JSON(http.StatusForbidden, gin.H{"error": "CSRF验证失败"})
				c.Abort()
				return
			} else {
				log.Printf("CSRF验证成功")
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

// RequireLoginNoCSRF 检查用户是否已登录，但不进行CSRF验证
func RequireLoginNoCSRF() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user_id")
		log.Printf("RequireLoginNoCSRF中间件: 路径=%s, userID=%v", c.Request.URL.Path, userID)

		if userID == nil {
			// 如果是API请求，返回401状态码
			if c.Request.Header.Get("X-Requested-With") == "XMLHttpRequest" {
				log.Println("API请求未登录，返回401")
				c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
				c.Abort()
				return
			}
			// 否则返回JSON错误（因为MCP API都是AJAX请求）
			log.Println("MCP API请求未登录，返回401")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
			c.Abort()
			return
		}

		log.Printf("用户已登录，userID=%v，继续处理请求（无CSRF验证）", userID)
		c.Next()
	}
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

// GetUserIDOptional 从会话中获取用户ID（可选，不要求登录）
func GetUserIDOptional(c *gin.Context) uint {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		return 0
	}
	
	if id, ok := userID.(uint); ok {
		return id
	}
	
	return 0
}
