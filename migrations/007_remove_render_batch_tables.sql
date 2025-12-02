-- 移除批量渲染相关表和字段
-- 此迁移脚本用于清理批量渲染功能相关的数据库结构

-- 1. 删除队列-批次关联表
DROP TABLE IF EXISTS figma_queue_batch_relations;

-- 2. 删除渲染批次表
DROP TABLE IF EXISTS figma_render_batches;

-- 3. 移除项目表中的批次相关字段
-- 检查字段是否存在，如果存在则删除
SET @sql = (SELECT IF(
    (SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS 
     WHERE TABLE_SCHEMA = DATABASE() 
     AND TABLE_NAME = 'figma_projects' 
     AND COLUMN_NAME = 'active_render_batch_id') > 0,
    'ALTER TABLE figma_projects DROP COLUMN active_render_batch_id;',
    'SELECT "Column active_render_batch_id does not exist";'
));
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- 4. 移除项目表中的 render_id 字段（如果它是用于批次的）
-- 注意：如果 render_id 还用于其他功能，请保留此字段
-- 这里我们保留 render_id 字段，因为它可能还用于单个队列的渲染

-- 5. 清理可能存在的孤立数据
-- 由于批次功能已移除，清理项目表中可能指向已删除批次的 render_id
-- 这里我们将 render_id 重置为 0，表示没有活跃的渲染任务
UPDATE figma_projects SET render_id = 0 WHERE render_id IS NOT NULL;

-- 迁移完成日志
SELECT '批量渲染相关表和字段已成功移除' as migration_status;

