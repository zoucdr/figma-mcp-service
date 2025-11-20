-- Figma API 缓存与速率限制系统 - 数据库迁移脚本
-- 版本: v1.1
-- 创建日期: 2025-11-19

-- 1. Token 冷却管理表
CREATE TABLE IF NOT EXISTS `figma_token_cooldowns` (
  `id` BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `figma_token` VARCHAR(255) NOT NULL COMMENT 'Figma Token',
  `last_file_request_time` INT UNSIGNED DEFAULT 0 COMMENT '最后一次文件树请求时间（Unix时间戳，秒）',
  `last_image_request_time` INT UNSIGNED DEFAULT 0 COMMENT '最后一次渲染图请求时间（Unix时间戳，秒）',
  `created_at` INT UNSIGNED NOT NULL COMMENT '创建时间（Unix时间戳，秒）',
  `updated_at` INT UNSIGNED NOT NULL COMMENT '更新时间（Unix时间戳，秒）',
  UNIQUE INDEX `idx_figma_token` (`figma_token`),
  INDEX `idx_last_file_request` (`last_file_request_time`),
  INDEX `idx_last_image_request` (`last_image_request_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Figma Token 冷却管理表 - 确保30秒API调用间隔';

-- 2. 节点树缓存表
CREATE TABLE IF NOT EXISTS `figma_file_caches` (
  `id` BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `file_key` VARCHAR(100) NOT NULL COMMENT 'Figma 文件 Key',
  `root_node_id` VARCHAR(100) NOT NULL COMMENT '根节点 ID（空字符串表示整个文件）',
  `node_ids` TEXT COMMENT '请求的节点 ID 列表，逗号分隔',
  `file_data` MEDIUMTEXT NOT NULL COMMENT '节点树 JSON 数据（最大 16MB）',
  `file_version` VARCHAR(100) COMMENT 'Figma 文件版本号',
  `status` ENUM('waiting', 'loading', 'loaded', 'error') NOT NULL DEFAULT 'waiting' COMMENT '状态',
  `error_message` TEXT COMMENT '错误信息',
  `next_available_time` INT UNSIGNED DEFAULT 0 COMMENT '下次可请求时间（Unix时间戳，秒）',
  `hit_count` INT UNSIGNED DEFAULT 0 COMMENT '缓存命中次数',
  `created_at` INT UNSIGNED NOT NULL COMMENT '创建时间（Unix时间戳，秒）',
  `updated_at` INT UNSIGNED NOT NULL COMMENT '更新时间（Unix时间戳，秒）',
  UNIQUE INDEX `idx_file_root` (`file_key`, `root_node_id`),
  INDEX `idx_status` (`status`),
  INDEX `idx_next_available_time` (`next_available_time`),
  INDEX `idx_file_key` (`file_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Figma 文件节点树缓存表';

-- 3. 渲染队列表
CREATE TABLE IF NOT EXISTS `figma_render_queues` (
  `id` BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `figma_token` VARCHAR(255) NOT NULL COMMENT 'Figma Token',
  `file_key` VARCHAR(100) NOT NULL COMMENT 'Figma 文件 Key',
  `node_ids` TEXT NOT NULL COMMENT '节点 ID 列表，逗号分隔（最多1000个）',
  `format` VARCHAR(10) NOT NULL DEFAULT 'png' COMMENT '图片格式: png, jpg, svg',
  `scale` DECIMAL(3,1) NOT NULL DEFAULT 1.0 COMMENT '图片缩放比例',
  `status` ENUM('waiting', 'processing', 'completed', 'error', 'cancelled') NOT NULL DEFAULT 'waiting' COMMENT '队列状态',
  `progress` INT UNSIGNED DEFAULT 0 COMMENT '完成进度 0-100',
  `total_nodes` INT UNSIGNED DEFAULT 0 COMMENT '总节点数',
  `processed_nodes` INT UNSIGNED DEFAULT 0 COMMENT '已处理节点数',
  `failed_nodes` INT UNSIGNED DEFAULT 0 COMMENT '失败节点数',
  `error_message` TEXT COMMENT '错误信息',
  `next_available_time` INT UNSIGNED DEFAULT 0 COMMENT '下次可请求时间（Unix时间戳，秒）',
  `started_at` INT UNSIGNED DEFAULT 0 COMMENT '开始处理时间（Unix时间戳，秒）',
  `completed_at` INT UNSIGNED DEFAULT 0 COMMENT '完成时间（Unix时间戳，秒）',
  `created_at` INT UNSIGNED NOT NULL COMMENT '创建时间（Unix时间戳，秒）',
  `updated_at` INT UNSIGNED NOT NULL COMMENT '更新时间（Unix时间戳，秒）',
  INDEX `idx_figma_token` (`figma_token`),
  INDEX `idx_status` (`status`),
  INDEX `idx_file_key` (`file_key`),
  INDEX `idx_next_available_time` (`next_available_time`),
  INDEX `idx_token_status` (`figma_token`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Figma 渲染队列表 - 按Token排队处理';

-- 4. 节点图片缓存表
CREATE TABLE IF NOT EXISTS `figma_node_images` (
  `id` BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `file_key` VARCHAR(100) NOT NULL COMMENT 'Figma 文件 Key',
  `node_id` VARCHAR(100) NOT NULL COMMENT '节点 ID',
  `format` VARCHAR(10) NOT NULL COMMENT '图片格式',
  `scale` DECIMAL(3,1) NOT NULL COMMENT '缩放比例',
  `figma_cdn_url` VARCHAR(1000) COMMENT 'Figma CDN URL（临时，用于下载）',
  `obs_url` VARCHAR(1000) COMMENT '华为云 OBS URL',
  `obs_key` VARCHAR(500) COMMENT '华为云 OBS 存储 Key',
  `obs_expires_at` INT UNSIGNED DEFAULT 0 COMMENT 'OBS 文件过期时间（Unix时间戳，秒，1个月）',
  `file_size` BIGINT UNSIGNED COMMENT '文件大小（字节）',
  `width` INT UNSIGNED COMMENT '图片宽度',
  `height` INT UNSIGNED COMMENT '图片高度',
  `status` ENUM('pending', 'figma_cdn', 'obs_synced', 'error') NOT NULL DEFAULT 'pending' COMMENT '状态',
  `error_message` TEXT COMMENT '错误信息',
  `hit_count` INT UNSIGNED DEFAULT 0 COMMENT '访问次数',
  `last_accessed_at` INT UNSIGNED DEFAULT 0 COMMENT '最后访问时间（Unix时间戳，秒）',
  `created_at` INT UNSIGNED NOT NULL COMMENT '创建时间（Unix时间戳，秒）',
  `updated_at` INT UNSIGNED NOT NULL COMMENT '更新时间（Unix时间戳，秒）',
  UNIQUE INDEX `idx_node_format_scale` (`file_key`, `node_id`, `format`, `scale`),
  INDEX `idx_status` (`status`),
  INDEX `idx_obs_key` (`obs_key`),
  INDEX `idx_obs_expires` (`obs_expires_at`),
  INDEX `idx_last_accessed` (`last_accessed_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Figma 节点图片缓存表';

-- 完成提示
SELECT 'Figma Cache & Rate Limit Tables Created Successfully!' AS message;

