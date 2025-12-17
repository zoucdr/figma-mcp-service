# FigmaFileCache 表拆分指南

## 概述

本次重构将 `figma_file_caches` 表拆分为两个独立的表，实现数据缓存和请求队列的职责分离。

## 拆分方案

### 拆分前
**figma_file_caches** - 既负责数据缓存，又负责请求队列
- ID, FigmaToken, FileKey, RootNodeID, NodeIDs, FileData, FileVersion, Status, ErrorMessage, HitCount, CreatedAt, UpdatedAt

### 拆分后

#### 1. figma_file_caches - 纯数据缓存表
- ID, FileKey, RootNodeID, NodeIDs, FileData, FileVersion, HitCount, CreatedAt, UpdatedAt
- **职责**：只存储已成功加载的 Figma 文件数据
- **移除字段**：FigmaToken, Status, ErrorMessage（这些属于队列管理）

#### 2. figma_file_fetch_queues - 文件获取请求队列表（新增）
- ID, FigmaToken, FileKey, RootNodeID, NodeIDs, Status, ErrorMessage, StartedAt, CompletedAt, CreatedAt, UpdatedAt
- **职责**：管理文件获取请求的队列状态（waiting, loading, loaded, error）
- **状态流转**：waiting → loading → loaded/error

## 数据流程

### 请求文件数据时
1. 先检查 `figma_file_caches` 表是否有缓存数据
2. 如果没有缓存，创建 `figma_file_fetch_queues` 记录（status=waiting）
3. 调度器处理队列，状态变为 loading
4. 成功获取数据后：
   - 创建或更新 `figma_file_caches` 记录（保存数据）
   - 更新 `figma_file_fetch_queues` 状态为 loaded

### 查询数据时
- 直接从 `figma_file_caches` 获取已缓存的数据（不需要检查 status）
- 查询 `figma_file_fetch_queues` 了解请求处理状态

## 迁移步骤

### 1. 备份数据（生产环境必须）
```sql
CREATE TABLE figma_file_caches_backup_20251125 AS SELECT * FROM figma_file_caches;
```

### 2. 执行迁移脚本
```bash
# 执行数据库迁移脚本
mysql -u [user] -p [database] < migrations/005_split_file_cache_and_fetch_queue.sql
```

### 3. 更新应用代码
迁移脚本会自动：
- 创建新的 `figma_file_fetch_queues` 表
- 将非 `loaded` 状态的记录迁移到新表
- 从 `figma_file_caches` 删除非 `loaded` 的记录
- 删除 `figma_file_caches` 表中的 `figma_token`, `status`, `error_message` 字段

### 4. 重启服务
```bash
# 停止服务
./stop.sh

# 重新编译
go build -o bin/figma-service main.go

# 启动服务
./start.sh
```

## 代码变更说明

### 模型变更
- `internal/models/cache.go`
  - 修改 `FigmaFileCache` 结构，移除队列相关字段
  - 新增 `FigmaFileFetchQueue` 结构
  - 新增队列操作方法：`CreateFileFetchQueue`, `GetFileFetchQueue`, `UpdateFileFetchQueueStatus` 等

### 服务层变更
- `internal/services/cache.go`
  - `CreateFileCache` → `CreateFileFetchQueue`（创建队列）
  - 新增 `CreateOrUpdateFileCache`（创建/更新缓存数据）
  - `UpdateFileCacheStatus` → `UpdateFileFetchQueueStatus`
  - `GetFileCache` 现在只返回已加载的数据，不再检查 status

- `internal/services/scheduler.go`
  - `processFileCacheQueue` 改为处理 `FigmaFileFetchQueue` 表

### 控制器变更
- `internal/controllers/cache.go`
  - 刷新节点树接口：同时检查缓存表和队列表
  - 查询状态接口：分别查询两个表返回状态

## 测试验证

### 1. 验证数据迁移
```sql
-- 检查新表是否创建成功
SHOW TABLES LIKE 'figma_file_fetch_queues';

-- 检查数据是否正确迁移
SELECT COUNT(*) FROM figma_file_fetch_queues;

-- 检查旧表是否只保留 loaded 数据
SELECT COUNT(*), status FROM figma_file_caches GROUP BY status;
-- 应该报错，因为 status 字段已删除
```

### 2. 功能测试
1. **刷新节点树**
   - 创建队列记录
   - 调度器处理队列
   - 成功后创建缓存数据

2. **查询缓存数据**
   - 直接从缓存表获取数据
   - 不受队列状态影响

3. **查询处理状态**
   - 从队列表获取状态
   - 支持 waiting/loading/loaded/error 状态

## 优势

1. **职责清晰**：数据缓存和请求队列各司其职
2. **性能优化**：查询缓存数据时不需要过滤 status
3. **易于维护**：两个表的字段和索引更加精简
4. **扩展性好**：后续可以独立优化缓存策略和队列处理逻辑

## 回滚方案

如果需要回滚到旧版本：

```sql
-- 1. 恢复备份表
DROP TABLE figma_file_caches;
CREATE TABLE figma_file_caches AS SELECT * FROM figma_file_caches_backup_20251125;

-- 2. 删除新表
DROP TABLE figma_file_fetch_queues;
```

然后回退代码到重构前的版本并重启服务。

## 注意事项

1. **迁移期间服务会短暂不可用**，建议在低峰期执行
2. **必须先执行数据库迁移**，再更新应用代码
3. **保留备份表至少一周**，确认无问题后再删除
4. **监控日志**，确保队列处理正常工作

