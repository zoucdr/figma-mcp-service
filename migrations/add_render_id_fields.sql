-- 迁移脚本：添加渲染关联字段
-- 用于支持项目渲染进度跟踪功能

-- 1. 为 figma_projects 表添加 render_id 字段
ALTER TABLE figma_projects 
ADD COLUMN render_id INT UNSIGNED DEFAULT 0 COMMENT '关联的渲染队列ID';

-- 添加索引以提高查询性能
CREATE INDEX idx_render_id ON figma_projects(render_id);

-- 2. 为 figma_render_queues 表添加 project_ids 字段
ALTER TABLE figma_render_queues 
ADD COLUMN project_ids TEXT COMMENT '关联的项目ID列表，JSON格式存储';

-- 回滚脚本（如需要）
-- ALTER TABLE figma_projects DROP COLUMN render_id;
-- DROP INDEX idx_render_id ON figma_projects;
-- ALTER TABLE figma_render_queues DROP COLUMN project_ids;

