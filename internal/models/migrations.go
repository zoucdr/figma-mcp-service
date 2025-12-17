package models

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/gorm"
)

// RunMigrations 执行数据库迁移
// 会检查并执行 migrations 目录下的 SQL 脚本
func RunMigrations(db *gorm.DB) error {
	log.Println("🔄 开始检查数据库迁移...")

	// 创建迁移记录表
	if err := createMigrationsTable(db); err != nil {
		return fmt.Errorf("创建迁移记录表失败: %v", err)
	}

	// 需要执行的迁移脚本列表（按顺序）
	migrations := []string{
		"005_split_file_cache_and_fetch_queue.sql",
		"006_add_ignore_texts_to_figma_node_images.sql",
	}

	migrationsDir := "migrations"
	executedCount := 0

	for _, migrationFile := range migrations {
		// 检查是否已执行
		if isMigrationExecuted(db, migrationFile) {
			log.Printf("⏭️  跳过已执行的迁移: %s", migrationFile)
			continue
		}

		// 读取并执行迁移脚本
		migrationPath := filepath.Join(migrationsDir, migrationFile)
		if err := executeMigrationFile(db, migrationPath); err != nil {
			return fmt.Errorf("执行迁移 %s 失败: %v", migrationFile, err)
		}

		// 记录已执行的迁移
		if err := recordMigration(db, migrationFile); err != nil {
			return fmt.Errorf("记录迁移 %s 失败: %v", migrationFile, err)
		}

		log.Printf("✅ 成功执行迁移: %s", migrationFile)
		executedCount++
	}

	if executedCount > 0 {
		log.Printf("✨ 数据库迁移完成，共执行 %d 个迁移脚本", executedCount)
	} else {
		log.Println("✅ 数据库结构已是最新，无需迁移")
	}

	return nil
}

// createMigrationsTable 创建迁移记录表
func createMigrationsTable(db *gorm.DB) error {
	sql := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			migration VARCHAR(255) NOT NULL UNIQUE,
			executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_migration (migration)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='数据库迁移记录表';
	`
	return db.Exec(sql).Error
}

// isMigrationExecuted 检查迁移是否已执行
func isMigrationExecuted(db *gorm.DB, migration string) bool {
	var count int64
	db.Raw("SELECT COUNT(*) FROM schema_migrations WHERE migration = ?", migration).Scan(&count)
	return count > 0
}

// recordMigration 记录已执行的迁移
func recordMigration(db *gorm.DB, migration string) error {
	return db.Exec("INSERT INTO schema_migrations (migration) VALUES (?)", migration).Error
}

// executeMigrationFile 执行迁移文件
func executeMigrationFile(db *gorm.DB, filePath string) error {
	// 读取 SQL 文件
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("读取迁移文件失败: %v", err)
	}

	// 分割 SQL 语句（按分号分隔）
	sqlStatements := splitSQLStatements(string(content))

	// 在事务中执行所有语句
	return db.Transaction(func(tx *gorm.DB) error {
		for i, stmt := range sqlStatements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" || strings.HasPrefix(stmt, "--") {
				continue
			}

			log.Printf("  执行 SQL 语句 %d/%d...", i+1, len(sqlStatements))
			if err := tx.Exec(stmt).Error; err != nil {
				// 如果是 "表已存在" 或 "列不存在" 等可以忽略的错误，记录警告但继续
				if strings.Contains(err.Error(), "Duplicate column name") ||
					strings.Contains(err.Error(), "already exists") ||
					strings.Contains(err.Error(), "Unknown column") ||
					strings.Contains(err.Error(), "Can't DROP") {
					log.Printf("  ⚠️ 忽略错误: %v", err)
					continue
				}
				return fmt.Errorf("执行 SQL 失败: %v\n语句: %s", err, stmt)
			}
		}
		return nil
	})
}

// splitSQLStatements 分割 SQL 语句
func splitSQLStatements(content string) []string {
	var statements []string
	var current strings.Builder
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 跳过注释行
		if strings.HasPrefix(trimmed, "--") {
			continue
		}

		// 累积当前语句
		current.WriteString(line)
		current.WriteString("\n")

		// 如果遇到分号，说明语句结束
		if strings.HasSuffix(trimmed, ";") {
			stmt := strings.TrimSpace(current.String())
			if stmt != "" {
				statements = append(statements, stmt)
			}
			current.Reset()
		}
	}

	// 处理最后一个语句（如果没有分号结尾）
	if current.Len() > 0 {
		stmt := strings.TrimSpace(current.String())
		if stmt != "" {
			statements = append(statements, stmt)
		}
	}

	return statements
}

// FixFigmaFileCacheTable 修复 figma_file_caches 表结构
// 如果表中有 figma_token 字段，则移除它
func FixFigmaFileCacheTable(db *gorm.DB) error {
	log.Println("🔧 检查 figma_file_caches 表结构...")

	// 检查是否存在 figma_token 字段
	var columnExists bool
	err := db.Raw(`
		SELECT COUNT(*) > 0 
		FROM information_schema.COLUMNS 
		WHERE TABLE_SCHEMA = DATABASE() 
		AND TABLE_NAME = 'figma_file_caches' 
		AND COLUMN_NAME = 'figma_token'
	`).Scan(&columnExists).Error

	if err != nil {
		return fmt.Errorf("检查表结构失败: %v", err)
	}

	if columnExists {
		log.Println("⚠️ 检测到 figma_file_caches 表中存在 figma_token 字段，需要执行迁移")

		// 执行表拆分迁移
		if err := RunMigrations(db); err != nil {
			return fmt.Errorf("执行迁移失败: %v", err)
		}
	} else {
		log.Println("✅ figma_file_caches 表结构正确，无需修复")
	}

	return nil
}
