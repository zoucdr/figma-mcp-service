# 定时任务调度器实现文档

## ✅ 完成状态

定时任务调度器已完整实现并成功编译！

## 功能概述

调度器服务（SchedulerService）负责自动化处理渲染队列和系统维护任务，是整个缓存系统的核心自动化组件。

## 核心功能

### 1. 渲染队列处理器

**功能**: 自动处理等待中的渲染队列

**执行间隔**: 5秒（可配置）

**处理流程**:

```
1. 获取所有待处理队列
   ↓
2. 检查 Token 冷却状态
   ↓
3. 调用 Figma Images API
   ↓
4. 保存图片 URL 到数据库
   ↓
5. 上传图片到 OBS
   ↓
6. 更新队列状态和进度
```

**详细步骤**:

```go
// 1. 获取等待队列
queues := queueService.GetWaitingQueues()

// 2. 对每个队列检查冷却
canRequest, nextTime := cacheService.CheckImageAPICooldown(token)

// 3. 调用 Figma API
imageURLs := callFigmaImagesAPI(fileKey, nodeIDs, format, scale, token)

// 4. 保存到数据库并上传 OBS
for nodeID, figmaURL := range imageURLs {
    // 创建/更新节点图片记录
    nodeImage := cacheService.CreateNodeImage(...)
    
    // 上传到 OBS（如果启用）
    obsURL, obsKey, fileSize := obsService.UploadImageFromURL(...)
    
    // 更新 OBS 信息
    cacheService.UpdateNodeImageOBS(...)
}

// 5. 更新队列状态
queueService.UpdateQueueStatus(queueID, "completed", 100)
```

### 2. 队列清理器

**功能**: 定期清理过期和卡住的队列记录

**执行间隔**: 1小时（可配置）

**清理规则**:

| 状态 | 条件 | 操作 |
|------|------|------|
| `completed` | 24小时前完成 | 删除记录 |
| `failed` | 7天前失败 | 删除记录 |
| `processing` | 1小时前开始 | 重置为 `waiting` |

## 配置参数

### 配置文件位置

```yaml
# configs/config.yaml
scheduler:
  render_queue_interval: 5      # 渲染队列处理间隔（秒）
  queue_cleanup_interval: 1     # 队列清理间隔（小时）
```

### 参数说明

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `render_queue_interval` | int | 5 | 渲染队列检查间隔（秒） |
| `queue_cleanup_interval` | int | 1 | 队列清理间隔（小时） |

## 代码结构

### 文件位置

```
internal/services/scheduler.go    # 调度器服务实现
main.go                          # 调度器初始化和启动
```

### 主要类型

#### SchedulerService

```go
type SchedulerService struct {
    cacheService         *CacheService
    queueService         *QueueService
    obsService           *OBSService
    renderQueueInterval  int
    queueCleanupInterval int
    stopChan             chan bool
    isRunning            bool
}
```

### 主要方法

| 方法 | 功能 |
|------|------|
| `NewSchedulerService()` | 创建调度服务实例 |
| `Start()` | 启动调度器 |
| `Stop()` | 停止调度器 |
| `runRenderQueueProcessor()` | 运行渲染队列处理器 |
| `processRenderQueue()` | 处理渲染队列 |
| `processQueue()` | 处理单个队列 |
| `callFigmaImagesAPI()` | 调用 Figma Images API |
| `runQueueCleanup()` | 运行队列清理器 |
| `cleanupQueues()` | 清理过期队列 |

## 使用方法

### 1. 启动调度器

调度器在应用启动时自动初始化和启动：

```go
// main.go

// 初始化调度服务
schedulerService := services.NewSchedulerService(
    cacheService,
    queueService,
    obsService,
    appConfig.Scheduler.RenderQueueInterval,
    appConfig.Scheduler.QueueCleanupInterval,
)

// 启动调度器
schedulerService.Start()
```

### 2. 停止调度器

```go
// 程序退出时自动停止
defer schedulerService.Stop()
```

## 日志示例

### 启动日志

```
🚀 [调度器] 启动成功
   - 渲染队列处理间隔: 5 秒
   - 队列清理间隔: 1 小时
✅ [渲染队列处理器] 已启动
✅ [队列清理器] 已启动
```

### 处理队列日志

```
📦 [渲染队列处理器] 发现 3 个待处理队列
🔄 [队列 #1] 开始处理
   - 文件: abc123xyz
   - Token: T9CJP2HFV9...
   - 节点数: 5
✅ [队列 #1] 获取到 5 个图片 URL
✅ [OBS] 上传成功: figma_images/abc123/png_2.0x/1-123.png (152847 bytes)
✅ [队列 #1] 处理完成: 5/5
```

### Token 冷却日志

```
⏳ [队列 #2] Token 冷却中，需等待 25 秒
```

### 清理日志

```
🧹 [队列清理器] 开始清理过期队列...
🗑️ [队列清理器] 清理了 10 个已完成队列
🔄 [队列清理器] 重置了 2 个卡住队列
✅ [队列清理器] 清理完成
```

## 队列状态流转

```
waiting (等待)
  ↓ (Token可用)
processing (处理中)
  ↓ (成功)
completed (完成)

或

processing (处理中)
  ↓ (部分成功)
partial (部分完成)

或

processing (处理中)
  ↓ (失败)
failed (失败)
  ↓ (重试)
waiting (等待)
```

## 进度更新

### 进度阶段

| 阶段 | 进度 | 说明 |
|------|------|------|
| 检查冷却 | 0% | 检查Token是否可用 |
| 调用API | 0-30% | 请求Figma Images API |
| 获取URL | 30% | 获取到图片URL |
| 上传OBS | 30-100% | 上传图片到OBS |
| 完成 | 100% | 所有图片处理完成 |

### 进度计算

```go
// 基础进度：30%（获取到URL）
baseProgress := 30

// 处理进度：30-100%
processed := successCount
total := totalCount
progress := baseProgress + (processed * 70 / total)
```

## Figma API 调用

### API 端点

```
GET https://api.figma.com/v1/images/{file_key}
```

### 请求参数

| 参数 | 类型 | 示例 | 说明 |
|------|------|------|------|
| `ids` | string | "1:123,1:456" | 节点ID列表（逗号分隔） |
| `format` | string | "png" | 图片格式 |
| `scale` | float | "2.0" | 缩放比例 |

### 请求头

```
X-Figma-Token: YOUR_FIGMA_TOKEN
```

### 响应格式

```json
{
  "err": null,
  "images": {
    "1:123": "https://s3-alpha.figma.com/img/...",
    "1:456": "https://s3-alpha.figma.com/img/..."
  }
}
```

## 错误处理

### 1. Token 冷却

```go
if !canRequest {
    log.Printf("⏳ Token 冷却中，需等待 %d 秒", waitSeconds)
    queueService.UpdateQueueStatus(queueID, "waiting", 0)
    return
}
```

### 2. Figma API 错误

```go
// 429 速率限制错误
if strings.Contains(err.Error(), "429") {
    queueService.UpdateQueueStatus(queueID, "waiting", 0)
} else {
    queueService.UpdateQueueStatus(queueID, "failed", 0)
}
```

### 3. OBS 上传失败

```go
// 即使 OBS 上传失败，也保留 Figma CDN URL
if err != nil {
    nodeImage.Status = "figma_only"
} else {
    nodeImage.Status = "completed"
}
```

## 性能优化

### 1. 并发控制

- 每个队列串行处理（避免Token冷却冲突）
- 队列内图片串行处理（保证顺序）
- OBS上传使用内部并发控制（见OBS服务）

### 2. 资源管理

```go
// 使用 Goroutine 运行定时任务
go s.runRenderQueueProcessor()
go s.runQueueCleanup()

// 使用 Channel 优雅停止
select {
case <-ticker.C:
    processRenderQueue()
case <-s.stopChan:
    return
}
```

### 3. 数据库优化

```go
// 批量查询等待队列
queues := GetWaitingQueues()

// 索引优化（见数据库设计）
- idx_status
- idx_next_time
- idx_updated_at
```

## 监控指标

### 关键指标

| 指标 | 说明 |
|------|------|
| 队列处理数 | 每分钟处理的队列数 |
| 成功率 | 成功处理的队列占比 |
| 平均耗时 | 单个队列平均处理时间 |
| Token冷却次数 | Token因冷却而等待的次数 |
| OBS上传成功率 | OBS上传成功占比 |

### 日志过滤

```powershell
# 查看调度器日志
.\figma-deliver.exe | Select-String "\[调度器\]"

# 查看队列处理日志
.\figma-deliver.exe | Select-String "\[队列 #"

# 查看清理日志
.\figma-deliver.exe | Select-String "\[队列清理器\]"
```

## 故障排查

### 问题1: 队列一直处于 waiting 状态

**原因**:
- Token 处于冷却期
- 调度器未启动
- 配置的检查间隔过长

**解决**:
1. 检查调度器是否启动
2. 检查Token冷却状态
3. 减小 `render_queue_interval`

### 问题2: 队列处理失败

**原因**:
- Figma API Token 无效
- Figma API 429 限制
- 网络连接问题

**解决**:
1. 验证 Figma Token 有效性
2. 检查 API 调用频率
3. 查看详细错误日志

### 问题3: OBS 上传失败

**原因**:
- OBS 未启用
- OBS 凭证错误
- 网络问题

**解决**:
1. 检查 OBS 配置
2. 验证 OBS 凭证
3. 即使 OBS 失败，Figma CDN URL 仍可用

## 最佳实践

### 1. 配置调优

```yaml
# 开发环境
scheduler:
  render_queue_interval: 5      # 频繁检查
  queue_cleanup_interval: 1     # 频繁清理

# 生产环境
scheduler:
  render_queue_interval: 10     # 适当减少频率
  queue_cleanup_interval: 6     # 减少清理频率
```

### 2. 监控告警

- 监控队列积压数量
- 监控处理失败率
- 监控 Figma API 调用频率
- 监控 OBS 上传成功率

### 3. 容错设计

- Token 冷却自动等待
- API 失败自动重试
- OBS 失败降级到 Figma CDN
- 卡住队列自动重置

## 集成测试

### 测试步骤

1. **启动服务**
   ```bash
   .\figma-deliver.exe
   ```

2. **创建渲染队列**
   ```bash
   curl -X POST http://localhost:8080/api/figma/manual/render \
     -H "Content-Type: application/json" \
     -d '{
       "file_key": "YOUR_FILE_KEY",
       "node_ids": ["1:123"],
       "format": "png",
       "scale": 2.0
     }'
   ```

3. **观察日志**
   - 应该看到调度器启动日志
   - 5秒后看到队列处理日志
   - 看到 Figma API 调用日志
   - 看到 OBS 上传日志

4. **验证结果**
   ```bash
   curl http://localhost:8080/api/figma/render/queue
   ```

## 下一步

- ✅ 调度器已完成
- ⏳ 前端集成（添加渲染按钮和进度条）
- ⏳ 前端集成（错误提示弹窗）
- ⏳ 完整测试和优化

## 相关文档

- [完整实施方案](./FIGMA_CACHE_RATE_LIMIT_SOLUTION.md)
- [OBS 集成指南](./OBS_INTEGRATION_GUIDE.md)
- [API 测试指南](./API_TESTING_GUIDE.md)
- [数据库迁移指南](./DATABASE_MIGRATION_GUIDE.md)

