-- Figma Deliver 数据库设置脚本
-- 在MySQL中运行此脚本来创建数据库和用户

-- 创建数据库
CREATE DATABASE IF NOT EXISTS figma_deliver CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- 创建用户（如果需要）
-- CREATE USER IF NOT EXISTS 'figma_user'@'localhost' IDENTIFIED BY 'your_password';

-- 授权（如果创建了新用户）
-- GRANT ALL PRIVILEGES ON figma_deliver.* TO 'figma_user'@'localhost';
-- FLUSH PRIVILEGES;

-- 使用数据库
USE figma_deliver;

-- 显示创建结果
SELECT 'Database figma_deliver created successfully' as status;
SHOW DATABASES LIKE 'figma_deliver';
