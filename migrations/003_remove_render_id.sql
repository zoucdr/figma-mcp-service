-- 移除旧的 render_id 字段
ALTER TABLE figma_projects DROP COLUMN render_id;

-- 删除旧的索引
DROP INDEX IF EXISTS idx_render_id ON figma_projects;

