package controllers

import (
	"net/http"

	"github.com/figma-bridge/internal/middleware"
	"github.com/figma-bridge/internal/models"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// Home 首页
func Home(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")

	if userID != nil {
		// 已登录，重定向到仪表盘
		c.Redirect(http.StatusFound, "/dashboard")
		return
	}

	// 未登录，显示首页
	c.HTML(http.StatusOK, "index.html", gin.H{
		"title": "Figma Bridge - 界面二次定义工具",
	})
}

// Dashboard 仪表盘页面
func Dashboard(c *gin.Context) {
	userID := middleware.GetUserID(c)
	session := sessions.Default(c)
	username := session.Get("username").(string)

	// 获取用户的Figma项目列表
	var projects []models.FigmaProject
	models.DB.Where("user_id = ?", userID).Find(&projects)

	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"title":    "仪表盘 - Figma Bridge",
		"username": username,
		"projects": projects,
	})
}
