-- 移除 figma_file_caches 表的 next_available_time 字段
-- 因为已经有统一的 figma_token_cooldowns 表来管理冷却时间

ALTER TABLE figma_file_caches 
DROP COLUMN IF EXISTS next_available_time;

-- 移除 figma_render_queues 表的 next_available_time 字段
-- 使用 figma_token_cooldowns 表来统一管理 Token 冷却

ALTER TABLE figma_render_queues 
DROP COLUMN IF EXISTS next_available_time;

-- 移除相关索引（如果存在）
DROP INDEX IF EXISTS idx_next_available_time ON figma_file_caches;
DROP INDEX IF EXISTS idx_next_available_time ON figma_render_queues;

