package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/figma-deliver/internal/controllers"
	"github.com/figma-deliver/internal/middleware"
	"github.com/figma-deliver/internal/models"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

// Config 应用配置结构
type Config struct {
	App struct {
		Port          string `yaml:"port"`
		GinMode       string `yaml:"gin_mode"`
		SessionSecret string `yaml:"session_secret"`
	} `yaml:"app"`
	Database struct {
		Host     string `yaml:"host"`
		Port     string `yaml:"port"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		Name     string `yaml:"name"`
	} `yaml:"database"`
	Storage struct {
		ExportDir string `yaml:"export_dir"`
		TempDir   string `yaml:"temp_dir"`
	} `yaml:"storage"`
}

var appConfig Config

func main() {
	// 加载 YAML 配置文件
	if err := loadConfig(); err != nil {
		log.Fatalf("加载配置文件失败: %v", err)
	}

	// 打印当前工作目录，帮助调试
	dir, _ := os.Getwd()
	log.Printf("当前工作目录: %s", dir)
	log.Printf("已加载配置: %s 模式", appConfig.App.GinMode)

	// 设置环境变量以供其他模块使用
	os.Setenv("DB_HOST", appConfig.Database.Host)
	os.Setenv("DB_PORT", appConfig.Database.Port)
	os.Setenv("DB_USER", appConfig.Database.User)
	os.Setenv("DB_PASS", appConfig.Database.Password)
	os.Setenv("DB_NAME", appConfig.Database.Name)
	os.Setenv("PORT", appConfig.App.Port)
	os.Setenv("GIN_MODE", appConfig.App.GinMode)
	os.Setenv("EXPORT_DIR", appConfig.Storage.ExportDir)
	os.Setenv("TEMP_DIR", appConfig.Storage.TempDir)

	// 初始化数据库连接
	models.InitDB()

	// 设置Gin模式
	gin.SetMode(appConfig.App.GinMode)

	// 创建Gin路由
	r := gin.Default()

	// 添加CORS中间件，支持Figma插件
	r.Use(middleware.CORSMiddleware())

	// 设置会话存储
	store := cookie.NewStore([]byte(appConfig.App.SessionSecret))

	// 配置会话选项
	store.Options(sessions.Options{
		Path:     "/",   // 所有路径都可以访问
		MaxAge:   86400, // 1天有效期
		HttpOnly: true,  // 防止JavaScript访问
		Secure:   false, // 开发环境不需要HTTPS
	})

	r.Use(sessions.Sessions("figma-deliver-session", store))

	// 添加CSRF中间件
	r.Use(middleware.CSRF())

	// 设置静态文件目录
	r.Static("/static", "./web/static")

	// 加载所有模板文件，包括布局模板
	r.LoadHTMLGlob("./web/templates/*")
	log.Println("已加载模板文件")

	// 添加时间戳中间件，用于缓存控制
	r.Use(func(c *gin.Context) {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		c.Set("timestamp", timestamp)
		c.Next()
	})

	// 首页路由
	r.GET("/", controllers.Home)

	// 健康检查路由 - 无需验证
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"timestamp": time.Now().Unix(),
			"service":   "figma-deliver",
		})
	})

	// 关于页面 - 公开访问
	r.GET("/about", controllers.About)

	// 登录页面
	r.GET("/login", func(c *gin.Context) {
		log.Println("访问登录页面")
		// 确保CSRF令牌存在
		session := sessions.Default(c)
		csrfToken := session.Get("csrf_token")
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
			log.Printf("生成新的CSRF令牌: %s", csrfToken)
		} else {
			log.Printf("使用现有CSRF令牌: %s", csrfToken)
		}

		// 检查用户是否已登录
		userID := session.Get("user_id")
		if userID != nil {
			// 已登录，重定向到项目列表
			c.Redirect(http.StatusFound, "/projects")
			return
		}

		c.HTML(http.StatusOK, "standalone.html", gin.H{
			"title":      "登录 - Figma Deliver",
			"timestamp":  time.Now().Unix(),
			"csrf_token": csrfToken,
			"template":   "login", // 指定要使用的内容模板
		})
		log.Println("登录页面渲染成功")
	})

	// 登录API
	r.POST("/login", controllers.Login)

	// Figma插件登录API - 返回JWT令牌
	r.POST("/plugin/login", controllers.PluginLogin)

	// 注册API
	r.POST("/register", controllers.Register)

	// 登出API
	r.GET("/logout", controllers.Logout)

	// 项目列表页面 - 需要登录
	projectsGroup := r.Group("/projects")
	projectsGroup.Use(middleware.RequireLogin())
	projectsGroup.GET("", controllers.Projects)

	// 项目编辑页面（仪表盘）- 需要登录
	dashboardGroup := r.Group("/dashboard")
	dashboardGroup.Use(middleware.RequireLogin())
	dashboardGroup.GET("", controllers.Dashboard)

	// MCP调用记录页面 - 需要登录
	r.GET("/mcp-logs", middleware.RequireLogin(), controllers.MCPLogs)

	// 公开的个人资料更新接口 - 用于注册
	r.POST("/profile/update", controllers.UpdateProfile)

	// 提示词分享页面 - 公开访问
	shareGroup := r.Group("/share")
	shareGroup.GET("", controllers.SharePage)

	// 提示词分享 API
	shareAPIGroup := shareGroup.Group("/api")
	// 公开接口 - 不需要登录
	shareAPIGroup.GET("/shares", controllers.GetPromptShares)

	// 需要登录的接口
	shareAPIGroup.POST("/shares", middleware.RequireLogin(), controllers.CreatePromptShare)
	shareAPIGroup.DELETE("/shares/:id", middleware.RequireLogin(), controllers.DeletePromptShare)
	shareAPIGroup.POST("/shares/:id/like", middleware.RequireLogin(), controllers.LikePromptShare)
	shareAPIGroup.POST("/shares/:id/unlike", middleware.RequireLogin(), controllers.UnlikePromptShare)
	shareAPIGroup.GET("/shares/:id/like-status", middleware.RequireLogin(), controllers.CheckLikeStatus)
	shareAPIGroup.GET("/my-shares", middleware.RequireLogin(), controllers.GetMyShares)

	// 个人资料页面 - 需要登录
	profileGroup := r.Group("/profile")
	profileGroup.Use(middleware.RequireLogin())
	profileGroup.POST("/generate-mcp-token", controllers.GenerateMCPToken)
	profileGroup.POST("/revoke-mcp-token", controllers.RevokeMCPToken)
	profileGroup.GET("", func(c *gin.Context) {
		// 确保CSRF令牌存在
		session := sessions.Default(c)
		csrfToken := session.Get("csrf_token")
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
			log.Printf("个人资料页面 - 生成新的CSRF令牌: %s", csrfToken)
		} else {
			log.Printf("个人资料页面 - 使用现有CSRF令牌: %s", csrfToken)
		}

		controllers.Profile(c)
	})

	// Figma API路由
	figmaGroup := r.Group("/figma")
	figmaGroup.Use(middleware.RequireLogin())

	// 解析Figma链接
	figmaGroup.POST("/parse", controllers.ParseFigmaLink)

	// 获取Figma节点 - 旧API，保留兼容
	figmaGroup.GET("/node/:file_key/:node_id", controllers.GetFigmaNode)
	figmaGroup.GET("/node/:file_key", controllers.GetFigmaNode)

	// 新API - 通过项目ID和节点ID获取节点信息
	figmaGroup.GET("/project/:project_id/nodes", controllers.GetFigmaNodeInfo)

	// 节点设置 - 新API
	figmaGroup.GET("/project/:project_id/node/:node_id/settings", controllers.GetNodeSettings)
	figmaGroup.POST("/project/:project_id/node/:node_id/settings", controllers.UpdateNodeSettings)
	figmaGroup.DELETE("/project/:project_id/node/:node_id/settings", controllers.DeleteNodeSettings)

	// 删除项目的所有节点修改（支持子树重置）
	figmaGroup.DELETE("/project/:project_id/modifications", controllers.DeleteProjectModifications)

	// 重置项目根节点及其子树的修改（新增）
	figmaGroup.DELETE("/project/:project_id/settings", controllers.DeleteProjectNodeSettings)

	// 节点设置 - 旧API，保留兼容
	figmaGroup.GET("/node/:file_key/:node_id/settings", controllers.GetNodeSettings)
	figmaGroup.POST("/node/:file_key/:node_id/settings", controllers.UpdateNodeSettings)

	// 获取Figma图片
	figmaGroup.GET("/image/:project_id/:node_id", controllers.GetFigmaImage)

	// 批量获取Figma过滤预览图片（返回节点ID到图片路径的映射）
	figmaGroup.GET("/images/:project_id/:node_id", controllers.GetFigmaImages)

	// 项目管理
	figmaGroup.PUT("/project/:project_id", controllers.UpdateProject)
	figmaGroup.PUT("/project/:project_id/settings", controllers.UpdateProjectSettings)
	figmaGroup.DELETE("/project/:project_id", controllers.DeleteProject)

	// 获取相同file_key下的所有项目
	figmaGroup.GET("/projects-by-filekey", controllers.GetProjectsByFileKey)

	// 获取Figma节点树和节点修改信息
	// 注意：更具体的路由应该先注册
	figmaGroup.GET("/project/:project_id/node-tree/refresh", controllers.RefreshFigmaNodeTree)
	figmaGroup.GET("/project/:project_id/node-tree", controllers.GetFigmaNodeTree)
	figmaGroup.GET("/project/:project_id/node-modifys", controllers.GetProjectNodeModifys)

	// 依赖节点管理
	figmaGroup.GET("/project/:project_id/ref-nodes", controllers.GetRefNodes)
	figmaGroup.POST("/project/:project_id/ref-nodes", controllers.AddRefNode)
	figmaGroup.DELETE("/project/:project_id/ref-nodes/:node_id", controllers.RemoveRefNode)
	figmaGroup.GET("/project/:project_id/ref-nodes/details", controllers.GetRefNodeDetails)

	// 界面描述管理
	figmaGroup.GET("/project/:project_id/interface-description", controllers.GetInterfaceDescription)
	figmaGroup.PUT("/project/:project_id/interface-description", controllers.UpdateInterfaceDescription)

	// 清除项目图片缓存
	figmaGroup.POST("/project/:project_id/clear-cache", controllers.ClearProjectImageCache)

	// 导出功能
	figmaGroup.POST("/project/:project_id/export", controllers.ExportFigmaDesign)
	figmaGroup.GET("/export/:job_id", controllers.GetExportStatus)
	figmaGroup.GET("/export/:job_id/download", controllers.DownloadExport)

	// Cursor MCP HTTP接口 - 无需认证，通过connection_id验证
	// 支持 /mcp/connection_id 格式的URL访问
	cursorMCPGroup := r.Group("/mcp")

	// Cursor MCP核心功能 - 使用connection_id认证
	cursorMCPGroup.GET("/:connection_id", controllers.CursorMCPHandler)
	cursorMCPGroup.POST("/:connection_id", controllers.CursorMCPHandler)
	cursorMCPGroup.PUT("/:connection_id", controllers.CursorMCPHandler)
	cursorMCPGroup.DELETE("/:connection_id", controllers.CursorMCPHandler)

	// MCP API路由 - 保留用于Web界面查看调用记录
	mcpAPIGroup := r.Group("/mcp-api")
	mcpAPIGroup.Use(middleware.RequireLogin()) // 需要登录验证
	// 只保留调用记录查询接口，供Web界面使用
	mcpAPIGroup.GET("/logs/:project_id", controllers.MCPGetCallLogsByProject)
	mcpAPIGroup.DELETE("/logs/:project_id/clear", controllers.MCPClearCallLogsByProject)
	mcpAPIGroup.POST("/resend/:project_id", controllers.MCPResendCall)

	// API服务模块 - 无需认证，通过MCP token验证
	apiGroup := r.Group("/api")
	// 通过MCP token反查用户ID
	apiGroup.GET("/:mcptoken", controllers.APIGetUserByToken)
	// 获取优化后的节点数据（支持3档简化级别）
	apiGroup.GET("/:mcptoken/optimized_nodes", controllers.APIGetOptimizedNodes)
	// 获取项目节点列表
	apiGroup.GET("/:mcptoken/project_nodes", controllers.APIGetProjectNodes)
	// 清除节点缓存
	apiGroup.POST("/:mcptoken/clear_cache", controllers.APIClearNodeCache)
	// 清除临时文件
	apiGroup.POST("/:mcptoken/clear_temp_files", controllers.APIClearTempFiles)
	// 下载Figma图片
	apiGroup.GET("/:mcptoken/download_image", controllers.APIDownloadFigmaImage)

	// 启动服务器
	log.Printf("服务器启动在 http://localhost:%s\n", appConfig.App.Port)
	r.Run(":" + appConfig.App.Port)
}

// loadConfig 加载YAML配置文件
func loadConfig() error {
	// 尝试多个可能的配置文件路径
	configPaths := []string{
		"configs/config.yaml",
		"./configs/config.yaml",
		"config.yaml",
	}

	var configFile string
	var err error

	// 查找存在的配置文件
	for _, path := range configPaths {
		if _, err := os.Stat(path); err == nil {
			configFile = path
			break
		}
	}

	if configFile == "" {
		return fmt.Errorf("未找到配置文件，请确保 config.yaml 存在于 configs 目录")
	}

	log.Printf("正在加载配置文件: %s", configFile)

	// 读取配置文件
	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 解析 YAML
	if err := yaml.Unmarshal(data, &appConfig); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 设置默认值
	if appConfig.App.Port == "" {
		appConfig.App.Port = "8080"
	}
	if appConfig.App.GinMode == "" {
		appConfig.App.GinMode = "debug"
	}
	if appConfig.App.SessionSecret == "" {
		appConfig.App.SessionSecret = "figma-deliver-secret"
	}
	if appConfig.Storage.ExportDir == "" {
		appConfig.Storage.ExportDir = "./exports"
	}
	if appConfig.Storage.TempDir == "" {
		appConfig.Storage.TempDir = "./temp"
	}

	// 环境变量覆盖（支持 K8s ConfigMap/Secret）
	if port := os.Getenv("PORT"); port != "" {
		appConfig.App.Port = port
	}
	if ginMode := os.Getenv("GIN_MODE"); ginMode != "" {
		appConfig.App.GinMode = ginMode
	}
	if sessionSecret := os.Getenv("SESSION_SECRET"); sessionSecret != "" {
		appConfig.App.SessionSecret = sessionSecret
	}
	if dbHost := os.Getenv("DB_HOST"); dbHost != "" {
		appConfig.Database.Host = dbHost
	}
	if dbPort := os.Getenv("DB_PORT"); dbPort != "" {
		appConfig.Database.Port = dbPort
	}
	if dbUser := os.Getenv("DB_USER"); dbUser != "" {
		appConfig.Database.User = dbUser
	}
	if dbPass := os.Getenv("DB_PASS"); dbPass != "" {
		appConfig.Database.Password = dbPass
	}
	if dbName := os.Getenv("DB_NAME"); dbName != "" {
		appConfig.Database.Name = dbName
	}
	if exportDir := os.Getenv("EXPORT_DIR"); exportDir != "" {
		appConfig.Storage.ExportDir = exportDir
	}
	if tempDir := os.Getenv("TEMP_DIR"); tempDir != "" {
		appConfig.Storage.TempDir = tempDir
	}

	return nil
}
