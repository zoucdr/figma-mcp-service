# 数据库迁移指南 - 缓存与速率限制系统

## 概述

本文档说明如何将缓存与速率限制系统的数据库表应用到现有系统中。

## 迁移脚本

数据库迁移脚本位于：`scripts/migrations/001_create_cache_tables.sql`

## 自动迁移（推荐）

系统使用 GORM 的 AutoMigrate 功能，启动时会自动创建/更新表结构。

### 步骤

1. **更新配置文件** (`configs/config.yaml`)
   - 已添加 Figma API、缓存、华为云 OBS 和调度器配置

2. **启动服务**
   ```bash
   go run main.go
   ```

3. **检查日志**
   启动时会看到如下日志：
   ```
   数据库连接和迁移成功（包含缓存表）
   ```

4. **验证表创建**
   连接到 MySQL 数据库：
   ```bash
   mysql -u root -p figma_deliver
   ```

   查看新表：
   ```sql
   SHOW TABLES LIKE 'figma_%';
   ```

   应该看到：
   ```
   figma_token_cooldowns
   figma_file_caches
   figma_render_queues
   figma_node_images
   ```

## 手动迁移（可选）

如果需要手动执行迁移，可以使用以下步骤：

### 步骤

1. **连接数据库**
   ```bash
   mysql -u root -p figma_deliver
   ```

2. **执行迁移脚本**
   ```bash
   source scripts/migrations/001_create_cache_tables.sql
   ```

3. **验证结果**
   ```sql
   -- 查看 Token 冷却表
   DESC figma_token_cooldowns;
   
   -- 查看文件缓存表
   DESC figma_file_caches;
   
   -- 查看渲染队列表
   DESC figma_render_queues;
   
   -- 查看节点图片表
   DESC figma_node_images;
   ```

## 表结构说明

### 1. figma_token_cooldowns (Token冷却管理)

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT | 主键 |
| figma_token | VARCHAR(255) | Figma Token（唯一） |
| last_file_request_time | INT UNSIGNED | 最后文件请求时间（Unix秒） |
| last_image_request_time | INT UNSIGNED | 最后图片请求时间（Unix秒） |
| created_at | INT UNSIGNED | 创建时间（Unix秒） |
| updated_at | INT UNSIGNED | 更新时间（Unix秒） |

**作用**: 记录每个 Token 的最后请求时间，确保 30秒 冷却间隔

### 2. figma_file_caches (节点树缓存)

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT | 主键 |
| file_key | VARCHAR(100) | Figma 文件 Key |
| root_node_id | VARCHAR(100) | 根节点 ID |
| node_ids | TEXT | 节点 ID 列表（逗号分隔） |
| file_data | MEDIUMTEXT | 节点树 JSON 数据 |
| file_version | VARCHAR(100) | Figma 文件版本号 |
| status | ENUM | 状态：waiting/loading/loaded/error |
| error_message | TEXT | 错误信息 |
| next_available_time | INT UNSIGNED | 下次可请求时间（Unix秒） |
| hit_count | INT UNSIGNED | 缓存命中次数 |
| created_at | INT UNSIGNED | 创建时间（Unix秒） |
| updated_at | INT UNSIGNED | 更新时间（Unix秒） |

**作用**: 缓存 Figma 文件节点树，避免重复请求

### 3. figma_render_queues (渲染队列)

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT | 主键 |
| figma_token | VARCHAR(255) | Figma Token |
| file_key | VARCHAR(100) | Figma 文件 Key |
| node_ids | TEXT | 节点 ID 列表（最多1000个） |
| format | VARCHAR(10) | 图片格式：png/jpg/svg |
| scale | DECIMAL(3,1) | 缩放比例 |
| status | ENUM | 状态：waiting/processing/completed/error/cancelled |
| progress | INT UNSIGNED | 进度 0-100 |
| total_nodes | INT UNSIGNED | 总节点数 |
| processed_nodes | INT UNSIGNED | 已处理节点数 |
| failed_nodes | INT UNSIGNED | 失败节点数 |
| error_message | TEXT | 错误信息 |
| next_available_time | INT UNSIGNED | 下次可请求时间（Unix秒） |
| started_at | INT UNSIGNED | 开始时间（Unix秒） |
| completed_at | INT UNSIGNED | 完成时间（Unix秒） |
| created_at | INT UNSIGNED | 创建时间（Unix秒） |
| updated_at | INT UNSIGNED | 更新时间（Unix秒） |

**作用**: 管理图片渲染请求队列，按 Token 排队处理

### 4. figma_node_images (节点图片缓存)

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT | 主键 |
| file_key | VARCHAR(100) | Figma 文件 Key |
| node_id | VARCHAR(100) | 节点 ID |
| format | VARCHAR(10) | 图片格式 |
| scale | DECIMAL(3,1) | 缩放比例 |
| figma_cdn_url | VARCHAR(1000) | Figma CDN URL（临时） |
| obs_url | VARCHAR(1000) | 华为云 OBS URL |
| obs_key | VARCHAR(500) | OBS 存储 Key |
| obs_expires_at | INT UNSIGNED | OBS 过期时间（Unix秒，1个月） |
| file_size | BIGINT UNSIGNED | 文件大小（字节） |
| width | INT UNSIGNED | 图片宽度 |
| height | INT UNSIGNED | 图片高度 |
| status | ENUM | 状态：pending/figma_cdn/obs_synced/error |
| error_message | TEXT | 错误信息 |
| hit_count | INT UNSIGNED | 访问次数 |
| last_accessed_at | INT UNSIGNED | 最后访问时间（Unix秒） |
| created_at | INT UNSIGNED | 创建时间（Unix秒） |
| updated_at | INT UNSIGNED | 更新时间（Unix秒） |

**作用**: 缓存单个节点的渲染图片信息

## 配置说明

### Figma API 配置

```yaml
figma_api:
  file_api_cooldown: 30        # Get File API 冷却时间（秒）
  images_api_cooldown: 30      # Get Images API 冷却时间（秒）
  max_nodes_per_request: 1000  # 批量请求最大节点数
```

### 缓存配置

```yaml
cache:
  obs_expires_days: 30  # OBS 文件过期时间（天）
```

### 华为云 OBS 配置

```yaml
huawei_cloud:
  obs:
    enabled: false                                      # 是否启用
    endpoint: "obs.cn-north-4.myhuaweicloud.com"       # 服务端点
    access_key_id: "YOUR_ACCESS_KEY"                   # 访问密钥
    secret_access_key: "YOUR_SECRET_KEY"               # 密钥
    bucket_name: "figma-deliver-cache"                 # 桶名称
    region: "cn-north-4"                               # 区域
    path_prefix: "figma_images/"                       # 路径前缀
    upload_concurrency: 5                              # 并发上传数
    upload_timeout_seconds: 300                        # 上传超时（秒）
```

### 调度器配置

```yaml
scheduler:
  render_queue_interval: 5    # 渲染队列处理间隔（秒）
  queue_cleanup_interval: 1   # 队列清理间隔（小时）
```

## 验证迁移

### 1. 检查表数量

```sql
SELECT COUNT(*) as table_count 
FROM information_schema.tables 
WHERE table_schema = 'figma_deliver' 
AND table_name LIKE 'figma_%';
```

应该返回至少 7 个表（包括原有的 figma_projects 和 figma_nodes）。

### 2. 检查索引

```sql
-- 检查 Token 索引
SHOW INDEX FROM figma_token_cooldowns WHERE Key_name = 'idx_figma_token';

-- 检查文件缓存索引
SHOW INDEX FROM figma_file_caches WHERE Key_name = 'idx_file_root';

-- 检查渲染队列索引
SHOW INDEX FROM figma_render_queues WHERE Key_name = 'idx_figma_token';

-- 检查节点图片索引
SHOW INDEX FROM figma_node_images WHERE Key_name = 'idx_node_format_scale';
```

### 3. 测试数据插入

```sql
-- 测试插入 Token 冷却记录
INSERT INTO figma_token_cooldowns (figma_token, last_file_request_time, last_image_request_time, created_at, updated_at) 
VALUES ('test_token_123', UNIX_TIMESTAMP(), 0, UNIX_TIMESTAMP(), UNIX_TIMESTAMP());

-- 查询结果
SELECT * FROM figma_token_cooldowns WHERE figma_token = 'test_token_123';

-- 清理测试数据
DELETE FROM figma_token_cooldowns WHERE figma_token = 'test_token_123';
```

## 回滚操作

如果需要回滚迁移，执行以下 SQL：

```sql
-- 警告：这将删除所有缓存数据！
DROP TABLE IF EXISTS figma_node_images;
DROP TABLE IF EXISTS figma_render_queues;
DROP TABLE IF EXISTS figma_file_caches;
DROP TABLE IF EXISTS figma_token_cooldowns;
```

## 注意事项

1. **数据持久性**: 所有时间字段使用 Unix 时间戳（INT UNSIGNED，秒）
2. **索引优化**: 已为常用查询字段添加索引，提高查询性能
3. **字符集**: 使用 utf8mb4 字符集，支持 Emoji 等特殊字符
4. **存储引擎**: 使用 InnoDB，支持事务和外键
5. **备份建议**: 迁移前建议备份现有数据库

## 下一步

完成数据库迁移后，可以继续实施：

1. ✅ 数据库表创建完成
2. ⏳ 实现核心业务逻辑
   - Token 冷却检查
   - 文件缓存管理
   - 渲染队列处理
3. ⏳ API 接口开发
4. ⏳ 前端界面调整
5. ⏳ 华为云 OBS 集成

## 故障排查

### 问题1: 表已存在错误

如果看到 "Table already exists" 错误：
- GORM AutoMigrate 会自动处理，不会报错
- 手动迁移时可以使用 `DROP TABLE IF EXISTS` 先删除

### 问题2: 索引创建失败

如果索引创建失败：
```sql
-- 查看现有索引
SHOW INDEX FROM figma_token_cooldowns;

-- 手动删除冲突索引
ALTER TABLE figma_token_cooldowns DROP INDEX idx_figma_token;

-- 重新创建
ALTER TABLE figma_token_cooldowns ADD UNIQUE INDEX idx_figma_token (figma_token);
```

### 问题3: 字符集问题

如果遇到字符集问题：
```sql
-- 修改表字符集
ALTER TABLE figma_token_cooldowns CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

## 相关文档

- [完整实施方案](./FIGMA_CACHE_RATE_LIMIT_SOLUTION.md)
- [迁移脚本](../scripts/migrations/001_create_cache_tables.sql)
- [模型定义](../internal/models/cache.go)

