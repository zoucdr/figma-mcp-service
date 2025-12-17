# 数据库初始化修复说明

## 问题
在拆分 `figma_file_cache` 表后，新创建的 `figma_file_fetch_queues` 表没有在数据库初始化时自动创建，导致运行时报错：
```
Error 1146 (42S02): Table 'figma_deliver.figma_file_fetch_queues' doesn't exist
```

## 解决方案

### 1. 添加 AutoMigrate
在 `internal/models/db.go` 的 `InitDB()` 函数中，将 `FigmaFileFetchQueue` 添加到 `AutoMigrate` 列表：

```go
err = DB.AutoMigrate(
    // ... 其他模型 ...
    &FigmaFileCache{},
    &FigmaFileFetchQueue{},  // ✅ 新增
    &FigmaRenderQueue{},
    // ... 其他模型 ...
)
```

### 2. 添加队列重置逻辑
将原来的 `resetProcessingRenderQueues()` 函数升级为 `resetProcessingQueues()`，同时重置两种队列：

```go
func resetProcessingQueues() {
    // 1. 重置文件获取队列（loading → waiting）
    // 2. 重置渲染队列（processing → waiting）
}
```

## 使用方法

### 方式一：自动迁移（推荐）
服务启动时会自动创建表结构，无需手动执行 SQL：

```bash
# 重新编译
go build -o bin/figma-service.exe main.go

# 启动服务（会自动创建表）
./startup.bat
```

### 方式二：手动执行迁移脚本
如果需要保留已有数据并进行迁移，执行迁移脚本：

```bash
mysql -u root -p figma_deliver < migrations/005_split_file_cache_and_fetch_queue.sql
```

## 验证

启动服务后，检查日志：

```
✅ 成功连接到MySQL数据库
✅ 数据库连接和迁移成功（包含缓存表）
✅ 没有需要重置的队列任务  # 或显示已重置的任务数
```

查询数据库验证表已创建：

```sql
-- 查看新表结构
DESC figma_file_fetch_queues;

-- 查看所有缓存相关表
SHOW TABLES LIKE 'figma_%cache%';
SHOW TABLES LIKE 'figma_%queue%';
```

## 注意事项

1. **首次启动**：如果是首次启动或空数据库，GORM 会自动创建所有表
2. **已有数据**：如果已有 `figma_file_caches` 表且包含 `status` 字段，建议先执行迁移脚本
3. **服务重启**：服务重启时会自动将 `loading` 状态的任务重置为 `waiting`，确保不会丢失任务

## 相关文件

- `internal/models/db.go` - 数据库初始化
- `internal/models/cache.go` - 模型定义
- `migrations/005_split_file_cache_and_fetch_queue.sql` - 迁移脚本
- `docs/FILE_CACHE_SPLIT_GUIDE.md` - 详细重构指南

