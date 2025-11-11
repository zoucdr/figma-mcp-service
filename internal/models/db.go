package models

import (
	"log"
	"os"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// 初始化数据库连接
func InitDB() {
	// 配置GORM日志
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Info,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)

	// 从环境变量获取数据库配置
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "3306")
	dbUser := getEnv("DB_USER", "root")
	dbPass := getEnv("DB_PASS", "")
	dbName := getEnv("DB_NAME", "figma_bridge")

	// 首先尝试连接到MySQL服务器（不指定数据库）
	rootDSN := dbUser + ":" + dbPass + "@tcp(" + dbHost + ":" + dbPort + ")/"
	log.Printf("连接到MySQL服务器: %s:***@tcp(%s:%s)/", dbUser, dbHost, dbPort)

	rootDB, err := gorm.Open(mysql.Open(rootDSN), &gorm.Config{
		Logger: newLogger,
	})

	if err != nil {
		log.Printf("连接MySQL服务器失败: %v", err)
	} else {
		// 尝试创建数据库（如果不存在）
		log.Printf("尝试创建数据库: %s", dbName)
		createDBSQL := "CREATE DATABASE IF NOT EXISTS " + dbName + " CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
		err = rootDB.Exec(createDBSQL).Error
		if err != nil {
			log.Printf("创建数据库失败: %v", err)
		} else {
			log.Printf("数据库 %s 已创建或已存在", dbName)
		}
	}

	// 构建完整的DSN字符串，连接到指定数据库
	dsn := dbUser + ":" + dbPass + "@tcp(" + dbHost + ":" + dbPort + ")/" + dbName + "?charset=utf8mb4&parseTime=True&loc=Local"

	// 打印DSN字符串（隐藏密码）以便调试
	log.Printf("数据库连接DSN: %s:***@tcp(%s:%s)/%s", dbUser, dbHost, dbPort, dbName)

	// 连接到指定数据库
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: newLogger,
	})

	if err != nil {
		log.Printf("连接MySQL失败: %v", err)
		log.Printf("请检查数据库配置和MySQL服务是否正常运行")
		log.Printf("尝试使用SQLite内存数据库作为备选...")

		// 尝试使用SQLite内存数据库
		// 注意：在生产环境中，应该使用文件数据库而不是内存数据库
		DB, err = gorm.Open(mysql.New(mysql.Config{
			DriverName: "mysql",
			DSN:        ":memory:",
		}), &gorm.Config{
			Logger: newLogger,
		})

		if err != nil {
			log.Fatalf("创建内存数据库失败: %v", err)
			log.Fatalf("请确保MySQL服务正常运行，并检查配置文件中的数据库连接信息")
		} else {
			log.Printf("成功切换到内存数据库模式，注意：数据将在程序关闭后丢失")
		}
	} else {
		log.Printf("成功连接到MySQL数据库")
	}

	// 自动迁移数据库表结构
	err = DB.AutoMigrate(&User{}, &FigmaProject{}, &FigmaNode{}, &ExportJob{})
	if err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}

	log.Printf("数据库连接和迁移成功")
}

// 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
