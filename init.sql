-- Figma Deliver 数据库初始化脚本
-- 该脚本会在MySQL容器首次启动时自动执行

-- 设置字符集
SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- 创建数据库（如果不存在）
CREATE DATABASE IF NOT EXISTS `figma_deliver` 
DEFAULT CHARACTER SET utf8mb4 
COLLATE utf8mb4_unicode_ci;

USE `figma_deliver`;

-- 用户表
CREATE TABLE IF NOT EXISTS `users` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `username` varchar(191) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `email` varchar(191) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `password_hash` varchar(191) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `figma_token` varchar(191) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `mcp_token` varchar(191) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `mcp_token_created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_users_username` (`username`),
  UNIQUE KEY `idx_users_email` (`email`),
  UNIQUE KEY `idx_users_mcp_token` (`mcp_token`),
  KEY `idx_users_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Figma项目表
CREATE TABLE IF NOT EXISTS `figma_projects` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `user_id` bigint(20) unsigned DEFAULT NULL,
  `name` varchar(191) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `file_key` varchar(191) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `node_id` varchar(191) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `figma_url` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `interface_description` text COLLATE utf8mb4_unicode_ci,
  PRIMARY KEY (`id`),
  KEY `idx_figma_projects_deleted_at` (`deleted_at`),
  KEY `idx_figma_projects_user_id` (`user_id`),
  KEY `idx_figma_projects_file_key` (`file_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 节点修改表
CREATE TABLE IF NOT EXISTS `node_modifys` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `project_id` bigint(20) unsigned DEFAULT NULL,
  `node_id` varchar(191) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `parent_id` varchar(191) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `rename` varchar(191) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `ignore` tinyint(1) DEFAULT NULL,
  `res_mode` varchar(191) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `anchor_pos` varchar(191) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `anchor_self` tinyint(1) DEFAULT NULL,
  `img_id` varchar(191) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `img_name` varchar(191) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `img_ext` varchar(191) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `components` text COLLATE utf8mb4_unicode_ci,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_node_modifys_project_node` (`project_id`,`node_id`),
  KEY `idx_node_modifys_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 依赖节点表
CREATE TABLE IF NOT EXISTS `ref_nodes` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `project_id` bigint(20) unsigned DEFAULT NULL,
  `node_id` varchar(191) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `node_name` varchar(191) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `node_type` varchar(191) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_ref_nodes_project_node` (`project_id`,`node_id`),
  KEY `idx_ref_nodes_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- MCP调用日志表
CREATE TABLE IF NOT EXISTS `mcp_call_logs` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `project_id` bigint(20) unsigned DEFAULT NULL,
  `method` varchar(191) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `params` text COLLATE utf8mb4_unicode_ci,
  `response` text COLLATE utf8mb4_unicode_ci,
  `error` text COLLATE utf8mb4_unicode_ci,
  `duration` bigint(20) DEFAULT NULL,
  `status` varchar(191) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_mcp_call_logs_deleted_at` (`deleted_at`),
  KEY `idx_mcp_call_logs_project_id` (`project_id`),
  KEY `idx_mcp_call_logs_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 提示词分享表
CREATE TABLE IF NOT EXISTS `prompt_shares` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `user_id` bigint(20) unsigned DEFAULT NULL,
  `title` varchar(191) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `content` text COLLATE utf8mb4_unicode_ci,
  `tags` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `likes` bigint(20) DEFAULT '0',
  `is_public` tinyint(1) DEFAULT '1',
  PRIMARY KEY (`id`),
  KEY `idx_prompt_shares_deleted_at` (`deleted_at`),
  KEY `idx_prompt_shares_user_id` (`user_id`),
  KEY `idx_prompt_shares_is_public` (`is_public`),
  KEY `idx_prompt_shares_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 提示词点赞表
CREATE TABLE IF NOT EXISTS `prompt_likes` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `user_id` bigint(20) unsigned DEFAULT NULL,
  `prompt_share_id` bigint(20) unsigned DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_prompt_likes_user_prompt` (`user_id`,`prompt_share_id`),
  KEY `idx_prompt_likes_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 创建默认管理员用户（密码：admin123）
INSERT IGNORE INTO `users` (`username`, `email`, `password_hash`, `created_at`, `updated_at`) 
VALUES ('admin', 'admin@figma-deliver.local', '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', NOW(), NOW());

SET FOREIGN_KEY_CHECKS = 1;
