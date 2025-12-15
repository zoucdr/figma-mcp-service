-- 添加ui_creater字段到users表
-- 用于存储用户自定义的UI代码生成脚本（最大1024KB）

ALTER TABLE users ADD COLUMN IF NOT EXISTS ui_creater TEXT;

-- 添加注释
COMMENT ON COLUMN users.ui_creater IS '用户自定义的UI代码生成JS脚本，最大1024KB';

