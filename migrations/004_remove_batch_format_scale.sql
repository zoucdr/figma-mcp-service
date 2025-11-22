-- 移除渲染批次表中的 format 和 scale 字段
-- 这些参数应该只在队列层面，批次不限制具体渲染参数

ALTER TABLE figma_render_batches DROP COLUMN format;
ALTER TABLE figma_render_batches DROP COLUMN scale;

