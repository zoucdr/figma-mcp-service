-- 移除 figma_node_images 表中的 obs_url 字段
-- 因为可以通过 obs_key 动态生成 URL，无需存储

-- 检查字段是否存在
SELECT COLUMN_NAME 
FROM INFORMATION_SCHEMA.COLUMNS 
WHERE TABLE_SCHEMA = DATABASE() 
  AND TABLE_NAME = 'figma_node_images' 
  AND COLUMN_NAME = 'obs_url';

-- 如果存在则删除
ALTER TABLE `figma_node_images` DROP COLUMN IF EXISTS `obs_url`;

-- 验证删除结果
DESCRIBE `figma_node_images`;

