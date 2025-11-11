package controllers

import (
	"net/http"
	"os"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/figma-bridge/internal/models"
	"github.com/gin-gonic/gin"
)

// PluginLogin 处理Figma插件的登录请求
func PluginLogin(c *gin.Context) {
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

	// 创建JWT令牌
	token, err := generateJWTToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "生成令牌失败: " + err.Error(),
		})
		return
	}

	// 返回令牌和用户信息
	c.JSON(http.StatusOK, gin.H{
		"message": "登录成功",
		"token":   token,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
		},
	})
}

// 生成JWT令牌
func generateJWTToken(user *models.User) (string, error) {
	// 获取JWT密钥，如果环境变量中没有设置，则使用默认值
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "figma-deliver-secret-key"
	}

	// 创建JWT声明
	claims := jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"exp":      time.Now().Add(time.Hour * 24 * 7).Unix(), // 令牌有效期7天
	}

	// 创建令牌
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// 签名令牌
	return token.SignedString([]byte(jwtSecret))
}

// ValidateToken 验证JWT令牌
func ValidateToken(tokenString string) (*jwt.Token, error) {
	// 获取JWT密钥，如果环境变量中没有设置，则使用默认值
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "figma-deliver-secret-key"
	}

	// 解析令牌
	return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(jwtSecret), nil
	})
}
