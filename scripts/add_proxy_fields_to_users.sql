-- 添加代理配置字段到 users 表
-- 迁移日期: 2025-11-18
-- 功能: 为每个用户添加代理配置支持

-- 添加 proxy_enabled 字段（是否启用代理）
ALTER TABLE users ADD COLUMN IF NOT EXISTS proxy_enabled BOOLEAN DEFAULT FALSE COMMENT '是否启用代理';

-- 添加 proxy_url 字段（代理地址）
ALTER TABLE users ADD COLUMN IF NOT EXISTS proxy_url VARCHAR(500) DEFAULT '' COMMENT '代理地址，如 http://127.0.0.1:7890';

-- 添加索引以提高查询效率
CREATE INDEX IF NOT EXISTS idx_users_proxy_enabled ON users(proxy_enabled);

-- 查看迁移结果
SELECT 'Migration completed successfully!' AS status;
SELECT COLUMN_NAME, DATA_TYPE, COLUMN_DEFAULT, IS_NULLABLE, COLUMN_COMMENT 
FROM INFORMATION_SCHEMA.COLUMNS 
WHERE TABLE_NAME = 'users' 
  AND COLUMN_NAME IN ('proxy_enabled', 'proxy_url');

