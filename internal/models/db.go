package models

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// CustomLogger 自定义日志记录器，截断过长的 SQL 语句
type CustomLogger struct {
	logger.Interface
	maxSQLLength int
}

// Trace 实现 logger.Interface 的 Trace 方法
func (l *CustomLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	// 获取原始 SQL 和影响行数
	sql, rows := fc()

	// 如果 SQL 超过最大长度，则截断
	if len(sql) > l.maxSQLLength {
		sql = sql[:l.maxSQLLength] + fmt.Sprintf("... (截断, 总长度: %d)", len(sql))
	}

	// 调用底层 logger 的 Trace 方法
	l.Interface.Trace(ctx, begin, func() (string, int64) {
		return sql, rows
	}, err)
}

// 初始化数据库连接
func InitDB() {
	// 配置GORM日志
	baseLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Info,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)

	// 包装为自定义 logger，最大 SQL 长度限制为 500 字符
	newLogger := &CustomLogger{
		Interface:    baseLogger,
		maxSQLLength: 500,
	}

	// 从环境变量获取数据库配置
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "3306")
	dbUser := getEnv("DB_USER", "root")
	dbPass := getEnv("DB_PASS", "")
	dbName := getEnv("DB_NAME", "figma_deliver")

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
		log.Printf("❌ 连接MySQL失败: %v", err)
		log.Printf("📋 数据库配置信息:")
		log.Printf("   - 主机: %s", dbHost)
		log.Printf("   - 端口: %s", dbPort)
		log.Printf("   - 用户: %s", dbUser)
		log.Printf("   - 数据库: %s", dbName)
		log.Printf("💡 请检查:")
		log.Printf("   1. MySQL服务是否正在运行")
		log.Printf("   2. 网络连接是否正常")
		log.Printf("   3. 数据库用户权限是否正确")
		log.Printf("   4. 防火墙是否阻止了连接")
		log.Fatalf("💥 MySQL数据库连接失败，程序无法继续运行")
	} else {
		log.Printf("✅ 成功连接到MySQL数据库")
	}

	// 自动迁移数据库表结构
	err = DB.AutoMigrate(
		&User{},
		&FigmaProject{},
		&FigmaNode{},
		&ExportJob{},
		&MCPConnection{},
		&MCPCallLog{},
		&PromptShare{},
		&PromptLike{},
		// 缓存与速率限制相关表
		&FigmaTokenCooldown{},
		&FigmaFileCache{},
		&FigmaFileFetchQueue{}, // 新增：文件获取队列表
		&FigmaRenderQueue{},
		&FigmaNodeImage{},
	)
	if err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}

	log.Printf("数据库连接和迁移成功（包含缓存表）")

	// 重置所有处理中的队列任务
	resetProcessingQueues()
}

// resetProcessingQueues 重置所有处理中的队列任务为 waiting
// 用于服务重启时恢复未完成的任务
func resetProcessingQueues() {
	now := uint32(time.Now().Unix())

	// 1. 重置文件获取队列
	resultFetch := DB.Model(&FigmaFileFetchQueue{}).
		Where("status = ?", "loading").
		Updates(map[string]interface{}{
			"status":     "waiting",
			"started_at": 0,
			"updated_at": now,
		})

	if resultFetch.Error != nil {
		log.Printf("⚠️ 重置文件获取队列失败: %v", resultFetch.Error)
	} else if resultFetch.RowsAffected > 0 {
		log.Printf("✅ 已重置 %d 个处理中的文件获取队列为 waiting 状态", resultFetch.RowsAffected)
	}

	// 2. 重置渲染队列
	resultRender := DB.Model(&FigmaRenderQueue{}).
		Where("status = ?", "processing").
		Updates(map[string]interface{}{
			"status":     "waiting",
			"started_at": 0,
			"updated_at": now,
		})

	if resultRender.Error != nil {
		log.Printf("⚠️ 重置渲染队列失败: %v", resultRender.Error)
	} else if resultRender.RowsAffected > 0 {
		log.Printf("✅ 已重置 %d 个处理中的渲染队列任务为 waiting 状态", resultRender.RowsAffected)
	}

	if resultFetch.RowsAffected == 0 && resultRender.RowsAffected == 0 {
		log.Printf("✅ 没有需要重置的队列任务")
	}
}

// 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
