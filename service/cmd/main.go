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

	// 测试路由
	r.GET("/", func(c *gin.Context) {
		log.Println("访问首页路由")
		c.HTML(http.StatusOK, "layout.html", gin.H{
			"title":     "Figma Bridge",
			"message":   "欢迎使用Figma Bridge！应用程序已成功启动。",
			"timestamp": time.Now().Unix(),
		})
		log.Println("首页渲染成功")
	})

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
		
		c.HTML(http.StatusOK, "layout.html", gin.H{
			"title":      "登录 - Figma Bridge",
			"timestamp":  time.Now().Unix(),
			"csrf_token": csrfToken,
		})
		log.Println("登录页面渲染成功")
	})

	// 登录API
	r.POST("/login", controllers.Login)

	// 注册API
	r.POST("/register", controllers.Register)

	// 登出API
	r.GET("/logout", controllers.Logout)

	// 仪表盘页面 - 需要登录
	dashboardGroup := r.Group("/dashboard")
	dashboardGroup.Use(middleware.RequireLogin())
	dashboardGroup.GET("", func(c *gin.Context) {
		log.Println("访问仪表盘页面")
		session := sessions.Default(c)
		username := session.Get("username").(string)

		c.HTML(http.StatusOK, "layout.html", gin.H{
			"title":     "仪表盘 - Figma Bridge",
			"username":  username,
			"projects":  []interface{}{}, // 空项目列表
			"timestamp": time.Now().Unix(),
		})
		log.Println("仪表盘页面渲染成功")
	})

	// 公开的个人资料更新接口 - 用于注册
	r.POST("/profile/update", controllers.UpdateProfile)

	// 个人资料页面 - 需要登录
	profileGroup := r.Group("/profile")
	profileGroup.Use(middleware.RequireLogin())
	profileGroup.GET("", controllers.Profile)

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
