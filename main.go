package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/figma-deliver/internal/controllers"
	"github.com/figma-deliver/internal/middleware"
	"github.com/figma-deliver/internal/models"
	"github.com/figma-deliver/internal/services"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
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
	FigmaAPI struct {
		FileAPICooldown    int `yaml:"file_api_cooldown"`
		ImagesAPICooldown  int `yaml:"images_api_cooldown"`
		MaxNodesPerRequest int `yaml:"max_nodes_per_request"`
		ImagesAPIRateLimit int `yaml:"images_api_rate_limit"` // 保留兼容
	} `yaml:"figma_api"`
	Cache struct {
		OBSExpiresDays int `yaml:"obs_expires_days"`
	} `yaml:"cache"`
	HuaweiCloud struct {
		OBS struct {
			Enabled              bool   `yaml:"enabled"`
			Endpoint             string `yaml:"endpoint"`
			AccessKeyID          string `yaml:"access_key_id"`
			SecretAccessKey      string `yaml:"secret_access_key"`
			BucketName           string `yaml:"bucket_name"`
			Region               string `yaml:"region"`
			PathPrefix           string `yaml:"path_prefix"`
			UploadConcurrency    int    `yaml:"upload_concurrency"`
			UploadTimeoutSeconds int    `yaml:"upload_timeout_seconds"`
		} `yaml:"obs"`
	} `yaml:"huawei_cloud"`
	Scheduler struct {
		RenderQueueInterval  int `yaml:"render_queue_interval"`
		QueueCleanupInterval int `yaml:"queue_cleanup_interval"`
	} `yaml:"scheduler"`
}

var appConfig Config

func main() {
	// 优先尝试加载 YAML 配置文件
	if err := loadConfig(); err != nil {
		log.Printf("YAML配置加载失败，尝试使用 .env 文件: %v", err)

		// 回退到 .env 文件加载
		err := godotenv.Load("configs/config.env")
		if err != nil {
			// 尝试从上级目录加载
			err = godotenv.Load("../configs/config.env")
			if err != nil {
				// 尝试从当前目录加载
				err = godotenv.Load(".env")
				if err != nil {
					log.Println("未找到配置文件，使用默认配置")
				}
			}
		}

		// 使用环境变量填充配置结构（兼容模式）
		appConfig.App.Port = getEnv("PORT", "8080")
		appConfig.App.GinMode = getEnv("GIN_MODE", "debug")
		appConfig.App.SessionSecret = getEnv("SESSION_SECRET", "figma-deliver-secret")
		appConfig.Database.Host = getEnv("DB_HOST", "")
		appConfig.Database.Port = getEnv("DB_PORT", "")
		appConfig.Database.User = getEnv("DB_USER", "")
		appConfig.Database.Password = getEnv("DB_PASS", "")
		appConfig.Database.Name = getEnv("DB_NAME", "")
		appConfig.Storage.ExportDir = getEnv("EXPORT_DIR", "./exports")
		appConfig.Storage.TempDir = getEnv("TEMP_DIR", "./temp")

		// Figma API 配置默认值
		rateLimitStr := getEnv("FIGMA_API_RATE_LIMIT", "10")
		if rateLimit, err := strconv.Atoi(rateLimitStr); err == nil && rateLimit > 0 {
			appConfig.FigmaAPI.ImagesAPIRateLimit = rateLimit
		} else {
			appConfig.FigmaAPI.ImagesAPIRateLimit = 10 // 默认10秒
		}
	}

	// 打印当前工作目录，帮助调试
	dir, _ := os.Getwd()
	log.Printf("当前工作目录: %s", dir)
	log.Printf("已加载配置: %s 模式", appConfig.App.GinMode)
	log.Printf("Figma API 配置:")
	log.Printf("  - File API 冷却: %d 秒", appConfig.FigmaAPI.FileAPICooldown)
	log.Printf("  - Images API 冷却: %d 秒", appConfig.FigmaAPI.ImagesAPICooldown)
	log.Printf("  - 最大节点数: %d", appConfig.FigmaAPI.MaxNodesPerRequest)
	log.Printf("缓存配置:")
	log.Printf("  - OBS 过期时间: %d 天", appConfig.Cache.OBSExpiresDays)
	log.Printf("华为云 OBS:")
	log.Printf("  - 启用状态: %v", appConfig.HuaweiCloud.OBS.Enabled)
	if appConfig.HuaweiCloud.OBS.Enabled {
		log.Printf("  - Endpoint: %s", appConfig.HuaweiCloud.OBS.Endpoint)
		log.Printf("  - Bucket: %s", appConfig.HuaweiCloud.OBS.BucketName)
	}
	log.Printf("调度器配置:")
	log.Printf("  - 渲染队列处理间隔: %d 秒", appConfig.Scheduler.RenderQueueInterval)
	log.Printf("  - 队列清理间隔: %d 小时", appConfig.Scheduler.QueueCleanupInterval)

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

	// 初始化 services 配置
	initServicesConfig()

	// 初始化OBS服务
	obsService, err := services.NewOBSService(services.OBSConfig{
		Enabled:              appConfig.HuaweiCloud.OBS.Enabled,
		Endpoint:             appConfig.HuaweiCloud.OBS.Endpoint,
		AccessKeyID:          appConfig.HuaweiCloud.OBS.AccessKeyID,
		SecretAccessKey:      appConfig.HuaweiCloud.OBS.SecretAccessKey,
		BucketName:           appConfig.HuaweiCloud.OBS.BucketName,
		Region:               appConfig.HuaweiCloud.OBS.Region,
		PathPrefix:           appConfig.HuaweiCloud.OBS.PathPrefix,
		UploadConcurrency:    appConfig.HuaweiCloud.OBS.UploadConcurrency,
		UploadTimeoutSeconds: appConfig.HuaweiCloud.OBS.UploadTimeoutSeconds,
	})
	if err != nil {
		log.Fatalf("❌ OBS服务初始化失败: %v", err)
	}
	defer obsService.Close()

	// 设置全局 OBS 服务实例（供图片控制器和服务使用）
	controllers.SetGlobalOBSService(obsService)
	services.SetGlobalOBSService(obsService)

	// 设置日志管理OBS服务实例
	controllers.SetLogsOBSService(obsService)

	// 初始化缓存和队列服务
	cacheService := services.NewCacheService(
		uint32(appConfig.FigmaAPI.FileAPICooldown),
		uint32(appConfig.FigmaAPI.ImagesAPICooldown),
		uint32(appConfig.Cache.OBSExpiresDays),
	)
	queueService := services.NewQueueService(
		uint(appConfig.FigmaAPI.MaxNodesPerRequest),
		cacheService,
	)

	log.Printf("✅ 缓存和队列服务初始化完成")

	// 初始化调度服务
	schedulerService := services.NewSchedulerService(
		cacheService,
		queueService,
		obsService,
		appConfig.Scheduler.RenderQueueInterval,
		appConfig.Scheduler.QueueCleanupInterval,
	)
	log.Printf("✅ 调度服务初始化完成")

	// 启动调度器
	schedulerService.Start()

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

	// 设置模板函数
	r.SetFuncMap(template.FuncMap{
		"len": func(v interface{}) int {
			if v == nil {
				return 0
			}
			switch val := v.(type) {
			case map[string]interface{}:
				return len(val)
			case []interface{}:
				return len(val)
			case string:
				return len(val)
			default:
				return 0
			}
		},
		"toJSON": func(v interface{}) template.JS {
			// 将值转换为JSON字符串，保留原始换行符
			b, err := json.Marshal(v)
			if err != nil {
				return template.JS("{}")
			}
			return template.JS(b)
		},
	})

	// 加载所有模板文件，包括布局模板
	r.LoadHTMLGlob("./web/templates/*")
	log.Println("已加载模板文件")

	// 添加时间戳中间件，用于缓存控制
	r.Use(func(c *gin.Context) {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		c.Set("timestamp", timestamp)
		c.Next()
	})

	// 首页路由 - 工具中心
	r.GET("/", controllers.ToolsIndexPage)

	// 旧的主页逻辑（已登录跳转到 /projects，未登录跳转到 /login）
	r.GET("/home", controllers.Home)

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
	// r.GET("/mcp-logs", middleware.RequireLogin(), controllers.MCPLogs)

	// MCP WebSocket 连接 - 不需要登录（可选）
	r.GET("/mcp-ws", controllers.MCPWebSocketHandler)

	// MCP WebSocket 测试页面 - 需要登录
	// r.GET("/mcp-ws-test", middleware.RequireLogin(), controllers.MCPWebSocketTest)

	// MCP 插件下载 - 不需要登录
	r.GET("/api/mcp/download-plugin", controllers.DownloadMCPPlugin)

	// 公开的个人资料更新接口 - 用于注册
	r.POST("/profile/update", controllers.UpdateProfile)

	// Token冷却信息和重置接口
	r.GET("/profile/api/cooldown-info", controllers.GetTokenCooldownInfo)
	r.POST("/profile/api/reset-cooldown", controllers.ResetTokenCooldown)

	// 测试代理连接接口
	r.POST("/api/test-proxy", controllers.TestProxy)

	// 用户 API
	r.GET("/api/user/current", middleware.RequireLogin(), controllers.GetCurrentUser)

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

	// 日志管理工具 - 无需登录
	toolsGroup := r.Group("/tools")

	// 工具索引页面
	toolsGroup.GET("", controllers.ToolsIndexPage)

	// 日志查看页面（支持路径参数）
	toolsGroup.GET("/logs", controllers.LogsPage)
	toolsGroup.GET("/logs/*path", controllers.LogsPage)

	// 局域网共享中心
	toolsGroup.GET("/share", controllers.ShareToolsPage)
	toolsGroup.GET("/share/api/text/list", controllers.GetTextClipsAPI)
	toolsGroup.POST("/share/api/text/update", controllers.UpdateTextClipAPI)
	toolsGroup.GET("/share/api/files/list", controllers.GetSharedFilesListAPI)
	toolsGroup.POST("/share/clipboard/add", controllers.AddTextClip)
	toolsGroup.GET("/share/clipboard/delete/:id", controllers.DeleteTextClip)
	toolsGroup.POST("/share/upload", controllers.UploadSharedFile)
	toolsGroup.GET("/share/files/:filename", controllers.DownloadSharedFile)
	toolsGroup.GET("/share/delete/:filename", controllers.DeleteSharedFile)
	toolsGroup.POST("/share/clear", controllers.ClearSharedFiles)

	// 日志管理 API - 无需登录
	logsAPIGroup := toolsGroup.Group("/api/logs")
	logsAPIGroup.GET("/list", controllers.ListLogs)                // 列出日志文件
	logsAPIGroup.POST("/upload", controllers.UploadLog)            // 上传日志文件
	logsAPIGroup.GET("/view", controllers.ViewLog)                 // 查看日志文件内容
	logsAPIGroup.GET("/download", controllers.DownloadLog)         // 下载日志文件
	logsAPIGroup.DELETE("/directory", controllers.DeleteDirectory) // 删除目录

	// 个人资料页面 - 需要登录
	profileGroup := r.Group("/profile")
	profileGroup.Use(middleware.RequireLogin())
	profileGroup.POST("/generate-mcp-token", controllers.GenerateMCPToken)
	profileGroup.POST("/revoke-mcp-token", controllers.RevokeMCPToken)
	profileGroup.POST("/change-password", controllers.ChangePassword)
	profileGroup.GET("/api/mcp-token", controllers.GetMCPToken) // 获取当前用户的 MCP Token
	profileGroup.GET("/api/info", controllers.GetCurrentUser)   // 获取当前用户的详细信息
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

	// 新API - 获取项目渲染节点（包含依赖节点，排除ignore节点）
	figmaGroup.GET("/project/:project_id/render_nodes", controllers.GetFigmaRenderNodeInfo)

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

	// 获取相同分组下的所有项目
	figmaGroup.GET("/projects-by-group", controllers.GetProjectsByGroup)

	// 获取Figma节点树和节点修改信息
	// 注意：更具体的路由应该先注册
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

	// 缓存和队列管理（新增）
	cacheController := controllers.NewCacheController(cacheService, queueService, obsService)
	figmaGroup.GET("/project/:project_id/node-tree/refresh", cacheController.RefreshProjectNodeTree) // 节点树刷新（带冷却检查）
	figmaGroup.POST("/file/refresh", cacheController.RefreshFile)                                    // 节点树刷新
	figmaGroup.POST("/batch-refresh-node-tree", cacheController.BatchRefreshNodeTree)                // 批量刷新节点树（支持多个root节点）
	figmaGroup.GET("/file/cache/status/:cache_id", cacheController.GetFileCacheStatus)               // 获取节点树状态
	// figmaGroup.POST("/project/:project_id/render", cacheController.RenderProject)                    // 项目渲染 [已弃用，使用 ManualRender 替代]
	figmaGroup.POST("/manual/render", cacheController.ManualRender)                                  // 手动渲染（统一接口）
	figmaGroup.GET("/render/queue", cacheController.GetRenderQueue)                                  // 获取队列信息
	figmaGroup.GET("/render/queue/stats", cacheController.GetQueueStatistics)                        // 获取队列统计
	figmaGroup.GET("/project/:project_id/render/progress", cacheController.GetProjectRenderProgress) // 获取项目渲染进度（批次）
	figmaGroup.POST("/project/:project_id/render/cancel", cacheController.CancelProjectRender)       // 取消项目渲染

	// 导出功能
	figmaGroup.POST("/project/:project_id/export", controllers.ExportFigmaDesign)
	figmaGroup.GET("/project/:project_id/export/active", controllers.GetProjectActiveExportJob)
	figmaGroup.GET("/export/:job_id", controllers.GetExportStatus)
	figmaGroup.GET("/export/status/:job_id", controllers.GetExportStatus)
	figmaGroup.GET("/export/:job_id/download", controllers.DownloadExport)

	// 自定义代码导出功能（替代旧的平台特定导出）
	figmaGroup.POST("/project/:project_id/export/custom", controllers.ExportCustomCode)
	figmaGroup.POST("/project/:project_id/custom/preview", controllers.GenerateCustomCodePreview)
	figmaGroup.GET("/project/:project_id/custom/code-preview", controllers.ShowCustomCodePreview)

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
	// MCP 连接状态查询
	mcpAPIGroup.GET("/connection-status", controllers.GetMCPConnectionStatus)
	// MCP 模拟工具调用
	mcpAPIGroup.POST("/tool-call", controllers.SimulateMCPToolCall)
	// MCP 获取工具列表
	mcpAPIGroup.GET("/tools", controllers.GetToolsList)
	// MCP 页面
	mcpAPIGroup.GET("/mcp-logs", controllers.MCPLogs)
	mcpAPIGroup.GET("/mcp-ws-test", controllers.MCPWebSocketTest)

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
	// 尝试多个可能的配置文件路径（优先本地开发配置）
	configPaths := []string{
		"configs/config.yaml", // 生产配置
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

	// Figma API 配置默认值
	if appConfig.FigmaAPI.FileAPICooldown <= 0 {
		appConfig.FigmaAPI.FileAPICooldown = 30 // 默认30秒
	}
	if appConfig.FigmaAPI.ImagesAPICooldown <= 0 {
		appConfig.FigmaAPI.ImagesAPICooldown = 30 // 默认30秒
	}
	if appConfig.FigmaAPI.MaxNodesPerRequest <= 0 {
		appConfig.FigmaAPI.MaxNodesPerRequest = 1000 // 默认1000个节点
	}
	if appConfig.FigmaAPI.ImagesAPIRateLimit <= 0 {
		appConfig.FigmaAPI.ImagesAPIRateLimit = 10 // 保留兼容，默认10秒
	}

	// 缓存配置默认值
	if appConfig.Cache.OBSExpiresDays <= 0 {
		appConfig.Cache.OBSExpiresDays = 30 // 默认30天
	}

	// 华为云OBS配置默认值
	if appConfig.HuaweiCloud.OBS.PathPrefix == "" {
		appConfig.HuaweiCloud.OBS.PathPrefix = "figma_images/"
	}
	if appConfig.HuaweiCloud.OBS.UploadConcurrency <= 0 {
		appConfig.HuaweiCloud.OBS.UploadConcurrency = 5
	}
	if appConfig.HuaweiCloud.OBS.UploadTimeoutSeconds <= 0 {
		appConfig.HuaweiCloud.OBS.UploadTimeoutSeconds = 300
	}

	// 调度器配置默认值
	if appConfig.Scheduler.RenderQueueInterval <= 0 {
		appConfig.Scheduler.RenderQueueInterval = 5 // 默认5秒
	}
	if appConfig.Scheduler.QueueCleanupInterval <= 0 {
		appConfig.Scheduler.QueueCleanupInterval = 1 // 默认1小时
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

// 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// initServicesConfig 初始化 services 包的配置
func initServicesConfig() {
	// 设置 Figma API 速率限制配置到环境变量
	// 优先使用新的 images_api_cooldown 配置，如果不存在则使用旧的 images_api_rate_limit
	cooldown := appConfig.FigmaAPI.ImagesAPICooldown
	if cooldown == 0 {
		cooldown = appConfig.FigmaAPI.ImagesAPIRateLimit
	}
	if cooldown == 0 {
		cooldown = 30 // 默认30秒
	}

	os.Setenv("FIGMA_API_RATE_LIMIT", strconv.Itoa(cooldown))

	log.Printf("Services 配置已初始化: Figma Images API 速率限制=%d秒", cooldown)
}
