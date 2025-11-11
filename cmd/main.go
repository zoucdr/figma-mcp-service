package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/figma-bridge/internal/controllers"
	"github.com/figma-bridge/internal/middleware"
	"github.com/figma-bridge/internal/models"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// 加载环境变量，尝试多个可能的路径
	err := godotenv.Load("configs/config.env")
	if err != nil {
		// 尝试从上级目录加载
		err = godotenv.Load("../configs/config.env")
		if err != nil {
			// 尝试从当前目录加载
			err = godotenv.Load(".env")
			if err != nil {
				log.Println("未找到环境变量文件，使用默认配置")
			}
		}
	}

	// 打印当前工作目录，帮助调试
	dir, _ := os.Getwd()
	log.Printf("当前工作目录: %s", dir)

	// 初始化数据库连接
	models.InitDB()

	// 设置Gin模式
	gin.SetMode(getEnv("GIN_MODE", "debug"))

	// 创建Gin路由
	r := gin.Default()

	// 设置会话存储
	store := cookie.NewStore([]byte(getEnv("SESSION_SECRET", "figma-bridge-secret")))

	// 配置会话选项
	store.Options(sessions.Options{
		Path:     "/",   // 所有路径都可以访问
		MaxAge:   86400, // 1天有效期
		HttpOnly: true,  // 防止JavaScript访问
		Secure:   false, // 开发环境不需要HTTPS
	})

	r.Use(sessions.Sessions("figma-bridge-session", store))

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
			"title":      "登录 - Figma Bridge",
			"timestamp":  time.Now().Unix(),
			"csrf_token": csrfToken,
			"template":   "login", // 指定要使用的内容模板
		})
		log.Println("登录页面渲染成功")
	})

	// 登录API
	r.POST("/login", controllers.Login)

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

	// 公开的个人资料更新接口 - 用于注册
	r.POST("/profile/update", controllers.UpdateProfile)

	// 个人资料页面 - 需要登录
	profileGroup := r.Group("/profile")
	profileGroup.Use(middleware.RequireLogin())
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

	// 节点设置 - 旧API，保留兼容
	figmaGroup.GET("/node/:file_key/:node_id/settings", controllers.GetNodeSettings)
	figmaGroup.POST("/node/:file_key/:node_id/settings", controllers.UpdateNodeSettings)

	// 获取Figma图片
	figmaGroup.GET("/image/:project_id/:node_id", controllers.GetFigmaImage)

	// 批量获取Figma过滤预览图片（返回节点ID到图片路径的映射）
	figmaGroup.GET("/images/:project_id/:node_id", controllers.GetFigmaImages)

	// 项目管理
	figmaGroup.PUT("/project/:project_id", controllers.UpdateProject)
	figmaGroup.DELETE("/project/:project_id", controllers.DeleteProject)

	// 获取Figma节点树和节点修改信息
	figmaGroup.GET("/project/:project_id/node-tree", controllers.GetFigmaNodeTree)
	figmaGroup.GET("/project/:project_id/node-tree/refresh", controllers.RefreshFigmaNodeTree)
	figmaGroup.GET("/project/:project_id/node-modifys", controllers.GetProjectNodeModifys)

	// 清除项目图片缓存
	figmaGroup.POST("/project/:project_id/clear-cache", controllers.ClearProjectImageCache)

	// 导出功能
	figmaGroup.POST("/project/:project_id/export", controllers.ExportFigmaDesign)
	figmaGroup.GET("/export/:job_id", controllers.GetExportStatus)
	figmaGroup.GET("/export/:job_id/download", controllers.DownloadExport)

	// 启动服务器
	port := getEnv("PORT", "8080")
	log.Printf("服务器启动在 http://localhost:%s\n", port)
	r.Run(":" + port)
}

// 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
