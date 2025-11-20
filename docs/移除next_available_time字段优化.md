# 移除 next_available_time 字段优化

## 问题描述

原系统中存在以下问题：

1. **冗余字段**：`figma_file_caches` 和 `figma_render_queues` 表中都有 `next_available_time` 字段，而系统已经有统一的 `figma_token_cooldowns` 表来管理 Token 冷却时间。
2. **休眠逻辑错误**：调度器在没有"立即可执行"的任务时进入休眠模式，但忽略了数据库中有大量等待冷却期结束的任务，导致这些任务无法被及时处理。
3. **逻辑不一致**：同一个冷却时间信息在多个地方维护，容易造成数据不一致。

## 解决方案

### 1. 数据库迁移

**迁移文件**: `migrations/remove_next_available_time_fields.sql`

移除冗余字段：
- 从 `figma_file_caches` 表移除 `next_available_time` 字段
- 从 `figma_render_queues` 表移除 `next_available_time` 字段
- 移除相关索引

### 2. 模型层修改

**文件**: `internal/models/cache.go`

- 从 `FigmaFileCache` 结构体移除 `NextAvailableTime` 字段
- 从 `FigmaRenderQueue` 结构体移除 `NextAvailableTime` 字段
- 修改 `GetWaitingRenderQueues()` 函数，移除对 `next_available_time` 的查询条件

### 3. 服务层修改

#### 3.1 Scheduler Service (`internal/services/scheduler.go`)

**主要修改**：

1. **渲染队列处理器**：
   - 保持轮询机制，不再因为无"立即可执行"任务而休眠
   - 在 `processQueue()` 中，遇到 Token 冷却时直接返回，保持队列状态为 `waiting`
   - 移除了对 `next_available_time` 的更新逻辑

2. **429 错误处理**：
   - 移除队列表中的 `next_available_time` 更新
   - Token 冷却时间由 `figma_token_cooldowns` 表统一管理
   - `HandleRetryAfter()` 已在 `callFigmaImagesAPI()` 中调用

3. **文件缓存队列处理器**：
   - 查询条件从 `status = 'waiting' AND next_available_time <= now` 改为 `status = 'waiting'`
   - 排序条件从 `next_available_time ASC` 改为 `updated_at ASC`
   - 移除对 `next_available_time` 的更新

#### 3.2 Queue Service (`internal/services/queue.go`)

**移除的内容**：

1. 移除 `calculateNextAvailableTime()` 函数
2. 移除 `SetQueueNextAvailableTime()` 函数
3. 在 `createNewQueues()` 中移除对 `NextAvailableTime` 的赋值
4. 在队列合并逻辑中移除对 `NextAvailableTime` 的更新

#### 3.3 Cache Service (`internal/services/cache.go`)

- 在 `CreateFileCache()` 中移除 `NextAvailableTime` 字段的赋值

#### 3.4 Figma Service (`internal/services/figma.go`)

- 在 `GetFigmaNodesNoCache()` 中移除 `NextAvailableTime` 字段的赋值

### 4. 控制器层修改

#### 4.1 Cache Controller (`internal/controllers/cache.go`)

**主要修改**：

1. **RefreshFileResponse 结构体**：
   - 移除 `NextAvailableTime` 字段
   - 保留 `EstimatedWaitSeconds` 字段用于返回预计等待时间

2. **RefreshFile() 方法**：
   - 移除对 `next_available_time` 的计算和赋值
   - 简化逻辑，只返回 `estimated_wait_seconds`

3. **GetRenderQueues() 方法**：
   - 移除响应中的 `next_available_time` 字段

4. **GetRenderQueue() 方法**：
   - 移除响应中的 `next_available_time` 字段

5. **RefreshProjectNodeTree() 方法**：
   - 移除创建/更新队列时对 `NextAvailableTime` 的设置
   - 移除响应中的 `next_available_time` 字段

#### 4.2 Figma Controller (`internal/controllers/figma.go`)

- 在 `GetFigmaNodeTree()` 错误响应中移除 `next_available_time` 字段

## 核心改进

### 1. 统一的冷却管理

所有 Token 冷却时间信息由 `figma_token_cooldowns` 表统一管理：

```go
type FigmaTokenCooldown struct {
    ID                    uint   `gorm:"primaryKey"`
    FigmaToken            string `gorm:"size:255;not null;uniqueIndex"`
    LastFileRequestTime   uint32 `gorm:"default:0"`   // File API 最后请求时间
    LastImageRequestTime  uint32 `gorm:"default:0"`   // Image API 最后请求时间
    RetryAfterSeconds     uint32 `gorm:"default:0"`   // Retry-After 秒数
    RetryAfterSetAt       uint32 `gorm:"default:0"`   // Retry-After 设置时间
    CreatedAt             uint32 `gorm:"not null"`
    UpdatedAt             uint32 `gorm:"not null"`
}
```

### 2. 持续轮询机制

调度器不再进入休眠模式，而是持续轮询 `waiting` 状态的任务：

- **优点**：
  - 即使任务在冷却期内，也能及时在冷却结束后被处理
  - 简化了逻辑，减少了数据同步问题
  
- **性能优化**：
  - 在 `CheckImageAPICooldown()` 和 `CheckFileAPICooldown()` 中快速判断是否可以执行
  - 数据库查询量略有增加，但换来了更可靠的任务处理

### 3. 清晰的职责分离

- **`figma_token_cooldowns` 表**：负责记录 Token 的使用时间和冷却信息
- **`figma_render_queues` 表**：负责记录渲染任务的详细信息和状态
- **`figma_file_caches` 表**：负责记录文件缓存的数据和状态
- **调度器**：负责轮询任务并检查冷却状态，统一调度

## 运行迁移

执行以下 SQL 脚本：

```bash
mysql -u your_username -p your_database < migrations/remove_next_available_time_fields.sql
```

或者在 MySQL 客户端中直接运行：

```sql
source migrations/remove_next_available_time_fields.sql;
```

## 验证

1. **编译项目**：
   ```bash
   go build -o figma-service.exe
   ```

2. **启动服务**：
   ```bash
   ./figma-service.exe
   ```

3. **观察日志**：
   - 调度器应该持续处理 `waiting` 状态的任务
   - 遇到冷却期的任务会输出日志并等待下次轮询
   - Token 冷却时间由 `figma_token_cooldowns` 表统一管理

## 兼容性说明

- **前端修改**：前端代码中如果有依赖 `next_available_time` 字段的逻辑，需要改为使用 `cooldown_remaining` 或 `estimated_wait_seconds` 字段。
- **API 响应格式**：部分 API 响应中移除了 `next_available_time` 字段，前端需要适配。

## 总结

这次优化解决了以下问题：

1. ✅ 移除了冗余的 `next_available_time` 字段
2. ✅ 修复了调度器休眠逻辑导致任务无法及时处理的问题
3. ✅ 统一了冷却时间管理，减少数据不一致风险
4. ✅ 简化了代码逻辑，提高了可维护性
5. ✅ 保持了系统的性能和稳定性

