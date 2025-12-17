-- =====================================================
-- 拆分 figma_file_caches 表为数据缓存和请求队列两个表
-- =====================================================

-- 1. 创建新的文件获取队列表
CREATE TABLE IF NOT EXISTS figma_file_fetch_queues (
    id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    figma_token VARCHAR(255) NOT NULL COMMENT '必须使用此token请求Figma API',
    file_key VARCHAR(100) NOT NULL,
    root_node_id VARCHAR(100) NOT NULL,
    node_ids TEXT COMMENT '逗号分隔的节点ID列表（请求参数）',
    status VARCHAR(20) NOT NULL DEFAULT 'waiting' COMMENT 'waiting, loading, loaded, error',
    error_message TEXT,
    started_at INT UNSIGNED DEFAULT 0 COMMENT 'Unix时间戳（秒）',
    completed_at INT UNSIGNED DEFAULT 0 COMMENT 'Unix时间戳（秒）',
    created_at INT UNSIGNED NOT NULL COMMENT 'Unix时间戳（秒）',
    updated_at INT UNSIGNED NOT NULL COMMENT 'Unix时间戳（秒）',
    UNIQUE KEY idx_fetch_file_root (file_key, root_node_id),
    KEY idx_figma_token (figma_token),
    KEY idx_file_key (file_key),
    KEY idx_status (status),
    KEY idx_token_status (figma_token, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='文件获取请求队列表';

-- 2. 迁移非 loaded 状态的记录到 figma_file_fetch_queues 表
INSERT INTO figma_file_fetch_queues (
    figma_token, file_key, root_node_id, node_ids, status, error_message,
    started_at, completed_at, created_at, updated_at
)
SELECT 
    figma_token,
    file_key,
    root_node_id,
    node_ids,
    status,
    error_message,
    0 as started_at,  -- 旧表没有这个字段
    0 as completed_at, -- 旧表没有这个字段
    created_at,
    updated_at
FROM figma_file_caches
WHERE status != 'loaded'
ON DUPLICATE KEY UPDATE
    status = VALUES(status),
    error_message = VALUES(error_message),
    updated_at = VALUES(updated_at);

-- 3. 备份原表（可选，建议在生产环境执行）
-- CREATE TABLE figma_file_caches_backup_20251125 AS SELECT * FROM figma_file_caches;

-- 4. 删除非 loaded 状态的记录，只保留已成功加载的缓存数据
DELETE FROM figma_file_caches WHERE status != 'loaded';

-- 5. 删除 figma_file_caches 表中的队列相关字段
ALTER TABLE figma_file_caches DROP COLUMN figma_token;
ALTER TABLE figma_file_caches DROP COLUMN status;
ALTER TABLE figma_file_caches DROP COLUMN error_message;

-- 6. 删除旧的索引（如果存在）
ALTER TABLE figma_file_caches DROP INDEX IF EXISTS idx_figma_token;
ALTER TABLE figma_file_caches DROP INDEX IF EXISTS idx_status;

-- 完成！
-- figma_file_caches 现在只包含纯数据缓存
-- figma_file_fetch_queues 负责管理文件获取请求队列

