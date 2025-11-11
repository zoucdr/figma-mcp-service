package controllers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/figma-deliver/internal/middleware"
	"github.com/figma-deliver/internal/models"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// Home 首页
func Home(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	csrfToken := session.Get("csrf_token")

	// 确保CSRF令牌存在
	if csrfToken == nil {
		// 如果会话中没有CSRF令牌，则生成一个
		token, err := middleware.GenerateRandomString(32)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		session.Set("csrf_token", token)
		session.Save()
		csrfToken = token
	}

	if userID != nil {
		// 已登录，重定向到项目列表
		c.Redirect(http.StatusFound, "/projects")
		return
	}

	// 未登录，重定向到登录页面
	c.Redirect(http.StatusFound, "/login")
	return
}

// Projects 项目列表页面
func Projects(c *gin.Context) {
	userID := middleware.GetUserID(c)
	session := sessions.Default(c)
	username := session.Get("username").(string)
	csrfToken := session.Get("csrf_token")

	// 获取项目列表
	var projects []models.FigmaProject
	models.DB.Where("user_id = ?", userID).Find(&projects)

	// 格式化时间
	for i := range projects {
		projects[i].CreatedAt = projects[i].CreatedAt.Local()
	}

	// 将项目列表转换为JSON字符串
	projectsJSON, err := json.Marshal(projects)
	if err != nil {
		c.String(http.StatusInternalServerError, "无法序列化项目列表数据")
		return
	}

	c.HTML(http.StatusOK, "standalone.html", gin.H{
		"title":      "项目列表 - Figma Deliver",
		"username":   username,
		"projects":   template.JS(projectsJSON),
		"timestamp":  time.Now().Unix(),
		"csrf_token": csrfToken,
		"template":   "projects",
	})
}

// Dashboard 项目编辑页面
func Dashboard(c *gin.Context) {
	userID := middleware.GetUserID(c)
	session := sessions.Default(c)
	username := session.Get("username").(string)
	csrfToken := session.Get("csrf_token")

	// 获取用户信息，包括控件类型列表
	var user models.User
	models.DB.First(&user, userID)

	// 获取项目ID参数
	projectIDStr := c.Query("project")
	if projectIDStr == "" {
		// 如果没有项目ID，尝试获取用户的第一个项目
		var firstProject models.FigmaProject
		result := models.DB.Where("user_id = ?", userID).Order("created_at DESC").First(&firstProject)
		if result.Error != nil {
			// 如果用户没有任何项目，重定向到项目列表
			c.Redirect(http.StatusFound, "/projects")
			return
		}
		// 重定向到第一个项目的dashboard
		c.Redirect(http.StatusFound, fmt.Sprintf("/dashboard?project=%d", firstProject.ID))
		return
	}

	projectID, err := strconv.ParseUint(projectIDStr, 10, 64)
	if err != nil {
		c.Redirect(http.StatusFound, "/projects")
		return
	}

	// 获取项目信息
	var project models.FigmaProject
	result := models.DB.First(&project, projectID)
	if result.Error != nil || project.UserID != userID {
		c.Redirect(http.StatusFound, "/projects")
		return
	}

	// 获取项目节点
	var nodes []models.FigmaNode
	models.DB.Where("project_id = ?", projectID).Find(&nodes)

	// 将项目和节点转换为JSON字符串
	projectJSON, err := json.Marshal(project)
	if err != nil {
		c.String(http.StatusInternalServerError, "无法序列化项目数据")
		return
	}

	nodesJSON, err := json.Marshal(nodes)
	if err != nil {
		c.String(http.StatusInternalServerError, "无法序列化节点数据")
		return
	}

	// 渲染项目编辑页面
	c.HTML(http.StatusOK, "standalone.html", gin.H{
		"title":      "编辑项目 - Figma Deliver",
		"username":   username,
		"project":    template.JS(projectJSON),
		"nodes":      template.JS(nodesJSON),
		"compTypes":  user.CompTypes,
		"mcpToken":   user.MCPToken,
		"timestamp":  time.Now().Unix(),
		"csrf_token": csrfToken,
		"template":   "dashboard",
	})
}

// About 关于页面
func About(c *gin.Context) {
	session := sessions.Default(c)
	csrfToken := session.Get("csrf_token")
	username := session.Get("username")

	// 确保CSRF令牌存在
	if csrfToken == nil {
		// 如果会话中没有CSRF令牌，则生成一个
		token, err := middleware.GenerateRandomString(32)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		session.Set("csrf_token", token)
		session.Save()
		csrfToken = token
	}

	// 渲染关于页面
	c.HTML(http.StatusOK, "standalone.html", gin.H{
		"title":      "关于 - Figma Deliver",
		"username":   username,
		"timestamp":  time.Now().Unix(),
		"csrf_token": csrfToken,
		"template":   "about",
	})
}

// MCPLogs MCP调用记录页面
func MCPLogs(c *gin.Context) {
	userID := middleware.GetUserID(c)
	session := sessions.Default(c)
	csrfToken := session.Get("csrf_token")

	// 确保CSRF令牌存在
	if csrfToken == nil {
		// 如果会话中没有CSRF令牌，则生成一个
		token, err := middleware.GenerateRandomString(32)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		session.Set("csrf_token", token)
		session.Save()
		csrfToken = token
	}

	// 获取项目ID参数
	projectIDStr := c.Query("project")
	if projectIDStr == "" {
		c.Redirect(http.StatusFound, "/projects")
		return
	}

	projectID, err := strconv.ParseUint(projectIDStr, 10, 64)
	if err != nil {
		c.Redirect(http.StatusFound, "/projects")
		return
	}

	// 获取项目信息
	var project models.FigmaProject
	result := models.DB.First(&project, projectID)
	if result.Error != nil || project.UserID != userID {
		c.Redirect(http.StatusFound, "/projects")
		return
	}

	// 将项目转换为JSON字符串
	projectJSON, err := json.Marshal(project)
	if err != nil {
		c.String(http.StatusInternalServerError, "无法序列化项目数据")
		return
	}

	// 渲染MCP日志页面
	c.HTML(http.StatusOK, "mcp-logs.html", gin.H{
		"project":    template.JS(projectJSON),
		"csrf_token": csrfToken,
		"timestamp":  time.Now().Unix(),
	})
}
