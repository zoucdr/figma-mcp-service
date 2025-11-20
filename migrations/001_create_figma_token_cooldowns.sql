-- 创建 figma_token_cooldowns 表
-- 用于管理 Figma API Token 的冷却时间

CREATE TABLE IF NOT EXISTS `figma_token_cooldowns` (
  `id` BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `figma_token` VARCHAR(255) NOT NULL,
  `last_file_request_time` INT UNSIGNED DEFAULT 0 COMMENT 'Unix时间戳（秒）',
  `last_image_request_time` INT UNSIGNED DEFAULT 0 COMMENT 'Unix时间戳（秒）',
  `created_at` INT UNSIGNED NOT NULL COMMENT 'Unix时间戳（秒）',
  `updated_at` INT UNSIGNED NOT NULL COMMENT 'Unix时间戳（秒）',
  UNIQUE KEY `idx_figma_token` (`figma_token`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Figma Token 冷却时间表';

