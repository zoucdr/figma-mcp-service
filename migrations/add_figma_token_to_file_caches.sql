-- 为 figma_file_caches 表添加 figma_token 字段
-- 用途：确保每个缓存记录使用指定的 token 请求 Figma API，不允许切换到其他 token

-- 1. 添加 figma_token 字段（先设置为可空，填充数据后再设置为 NOT NULL）
ALTER TABLE `figma_file_caches` 
ADD COLUMN `figma_token` VARCHAR(255) DEFAULT NULL COMMENT '必须使用此token请求Figma API' AFTER `id`;

-- 2. 从关联的项目中填充 figma_token（通过 file_key 和 root_node_id 关联）
UPDATE `figma_file_caches` fc
INNER JOIN `figma_projects` fp ON fc.file_key = fp.file_key AND fc.root_node_id = fp.root_node_id
INNER JOIN `users` u ON fp.user_id = u.id
SET fc.figma_token = u.figma_token
WHERE fc.figma_token IS NULL AND u.figma_token IS NOT NULL AND u.figma_token != '';

-- 3. 删除无法填充 token 的记录（孤立记录）
DELETE FROM `figma_file_caches` WHERE `figma_token` IS NULL OR `figma_token` = '';

-- 4. 将 figma_token 设置为 NOT NULL
ALTER TABLE `figma_file_caches` 
MODIFY COLUMN `figma_token` VARCHAR(255) NOT NULL COMMENT '必须使用此token请求Figma API';

-- 5. 添加索引
ALTER TABLE `figma_file_caches` 
ADD INDEX `idx_figma_token` (`figma_token`);

-- 6. 重置 auto_increment（可选，整理ID）
-- ALTER TABLE `figma_file_caches` AUTO_INCREMENT = 1;

