-- 为 figma_node_images 表添加 ignore_texts 字段
-- 迁移编号: 006
-- 创建时间: 2025-12-01
-- 说明: 添加 ignore_texts 布尔字段，用于标识图片是否忽略文本渲染

-- 1. 删除旧的唯一索引（不包含 ignore_texts）
ALTER TABLE figma_node_images DROP INDEX IF EXISTS idx_node_format_scale;

-- 2. 添加 ignore_texts 字段（默认值为 false）
ALTER TABLE figma_node_images 
ADD COLUMN ignore_texts TINYINT(1) NOT NULL DEFAULT 0 
COMMENT '是否忽略文本（不渲染文本）' 
AFTER scale;

-- 3. 创建新的唯一索引（包含 ignore_texts）
ALTER TABLE figma_node_images 
ADD UNIQUE INDEX idx_node_format_scale_ignore (file_key, node_id, format, scale, ignore_texts);

-- 4. 更新现有数据，将所有现有记录的 ignore_texts 设置为 false（如果有需要）
-- 由于默认值已经是 0（false），所以不需要额外更新

