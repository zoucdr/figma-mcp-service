# 修复：渲染队列 NextAvailableTime 未设置

## 问题描述

在创建渲染队列时，`next_available_time` 字段被硬编码为 `0`，导致调度器无法正确判断队列何时可以开始执行。这会造成：

1. **忽略 Figma API 冷却时间**：即使 Token 正在冷却期，队列仍会立即被调度器选中
2. **可能触发速率限制**：调度器可能在冷却期内尝试调用 Figma API，导致 429 错误
3. **队列执行效率低下**：无法提前规划队列执行顺序

### 问题代码

**文件**: `internal/services/queue.go`

```go
queue := &models.FigmaRenderQueue{
    FigmaToken:        token,
    FileKey:           fileKey,
    NodeIDs:           joinNodeIDsForQueue(batch),
    Format:            format,
    Scale:             scale,
    Status:            "waiting",
    Progress:          0,
    TotalNodes:        uint(len(batch)),
    ProcessedNodes:    0,
    FailedNodes:       0,
    NextAvailableTime: 0,  // ❌ 硬编码为 0
    CreatedAt:         now,
    UpdatedAt:         now,
}
```

## 解决方案

### 1. 实现 `calculateNextAvailableTime` 方法

创建一个新方法来计算队列的下次可用时间，基于 Token 的冷却状态。

**文件**: `internal/services/queue.go`

```go
// calculateNextAvailableTime 计算队列的下次可用时间（基于 Token 冷却状态）
func (qs *QueueService) calculateNextAvailableTime(token string) uint32 {
	// 获取 Token 的冷却记录
	cooldown, err := models.GetOrCreateTokenCooldown(token)
	if err != nil {
		log.Printf("⚠️ [Queue] 获取 Token 冷却状态失败: %v，使用当前时间", err)
		return uint32(time.Now().Unix())
	}

	now := uint32(time.Now().Unix())

	// Figma API 速率限制是全局的，需要检查两个时间戳中较大的那个
	lastRequestTime := cooldown.LastFileRequestTime
	if cooldown.LastImageRequestTime > lastRequestTime {
		lastRequestTime = cooldown.LastImageRequestTime
	}

	// 如果从未请求过，可以立即执行
	if lastRequestTime == 0 {
		log.Printf("✅ [Queue] Token 未使用过，队列可立即执行")
		return now
	}

	// 计算下次可用时间（最后请求时间 + 冷却时间）
	cooldownSeconds := uint32(30) // Figma API 冷却时间为 30 秒
	nextAvailableTime := lastRequestTime + cooldownSeconds

	// 如果下次可用时间在过去，使用当前时间
	if nextAvailableTime < now {
		log.Printf("✅ [Queue] Token 冷却已完成，队列可立即执行")
		return now
	}

	// 计算需要等待的时间
	waitSeconds := nextAvailableTime - now
	log.Printf("⏳ [Queue] Token 冷却中，队列需等待 %d 秒后执行 (next_available_time=%d)",
		waitSeconds, nextAvailableTime)

	return nextAvailableTime
}
```

### 2. 在创建队列时调用该方法

**文件**: `internal/services/queue.go`

```go
// createNewQueues 创建新队列（按1000个节点拆分）
func (qs *QueueService) createNewQueues(...) ([]uint, error) {
	now := uint32(time.Now().Unix())
	queueIDs := []uint{}

	// 获取 Token 的冷却状态，计算下次可用时间
	nextAvailableTime := qs.calculateNextAvailableTime(token)

	// 按最大节点数拆分
	for i := 0; i < len(nodeIDs); i += int(qs.MaxNodesPerRequest) {
		// ...
		
		queue := &models.FigmaRenderQueue{
			// ...
			NextAvailableTime: nextAvailableTime,  // ✅ 使用计算值
			CreatedAt:         now,
			UpdatedAt:         now,
		}
		
		// ...
	}
}
```

### 3. 在合并队列时更新 NextAvailableTime

**文件**: `internal/services/queue.go`

```go
func (qs *QueueService) mergeToExistingQueue(...) ([]uint, error) {
	// ... 去重合并逻辑
	
	// 更新现有队列
	queue.NodeIDs = joinNodeIDsForQueue(mergedNodeIDs)
	queue.TotalNodes = uint(len(mergedNodeIDs))
	queue.UpdatedAt = uint32(time.Now().Unix())
	
	// 如果 NextAvailableTime 为 0 或已过期，重新计算
	now := uint32(time.Now().Unix())
	if queue.NextAvailableTime == 0 || queue.NextAvailableTime < now {
		queue.NextAvailableTime = qs.calculateNextAvailableTime(queue.FigmaToken)
	}
	
	// ...
}
```

## 实现逻辑

### 计算流程

```
获取 Token 冷却记录
  ↓
获取最后请求时间 (max(LastFileRequestTime, LastImageRequestTime))
  ↓
判断请求历史
  ├─ 从未请求 → 返回当前时间 (立即可用)
  ├─ 冷却已完成 → 返回当前时间 (立即可用)
  └─ 正在冷却 → 返回 lastRequestTime + 30秒
```

### 场景分析

#### 场景 1: Token 从未使用过

**状态**:
```
lastFileRequestTime = 0
lastImageRequestTime = 0
```

**计算**:
```
lastRequestTime = max(0, 0) = 0
nextAvailableTime = now (立即可用)
```

**日志**:
```
✅ [Queue] Token 未使用过，队列可立即执行
```

#### 场景 2: Token 冷却已完成

**状态**:
```
now = 1731000000
lastFileRequestTime = 1730999950 (50秒前)
lastImageRequestTime = 1730999940 (60秒前)
cooldownSeconds = 30
```

**计算**:
```
lastRequestTime = max(1730999950, 1730999940) = 1730999950
nextAvailableTime = 1730999950 + 30 = 1730999980
1730999980 < 1731000000 → 冷却已完成
返回: now (立即可用)
```

**日志**:
```
✅ [Queue] Token 冷却已完成，队列可立即执行
```

#### 场景 3: Token 正在冷却

**状态**:
```
now = 1731000000
lastFileRequestTime = 1730999985 (15秒前)
lastImageRequestTime = 1730999980 (20秒前)
cooldownSeconds = 30
```

**计算**:
```
lastRequestTime = max(1730999985, 1730999980) = 1730999985
nextAvailableTime = 1730999985 + 30 = 1731000015
1731000015 > 1731000000 → 还在冷却期
需要等待: 1731000015 - 1731000000 = 15秒
返回: 1731000015
```

**日志**:
```
⏳ [Queue] Token 冷却中，队列需等待 15 秒后执行 (next_available_time=1731000015)
```

## 调度器集成

### 队列查询

调度器在选择队列时会检查 `next_available_time`：

**文件**: `internal/models/cache.go`

```go
func GetAvailableRenderQueues(limit int) ([]FigmaRenderQueue, error) {
	var queues []FigmaRenderQueue
	now := uint32(time.Now().Unix())
	result := DB.Where("status = ? AND next_available_time <= ?", "waiting", now).
		Order("created_at ASC").
		Limit(limit).
		Find(&queues)
	
	return queues, result.Error
}
```

**效果**: 只有 `next_available_time <= now` 的队列才会被选中执行

### 执行流程

```
调度器轮询
  ↓
查询可用队列 (next_available_time <= now)
  ↓
选择队列
  ├─ NextAvailableTime = 1731000000, Now = 1731000020 → ✅ 可执行
  ├─ NextAvailableTime = 1731000030, Now = 1731000020 → ⏸️ 跳过（还需等待10秒）
  └─ NextAvailableTime = 0, Now = 1731000020 → ✅ 可执行（兼容旧数据）
```

## 测试验证

### 测试 1: 创建新队列

**输入**:
```go
CreateRenderQueue(
    token: "fig_abc123",
    fileKey: "oNOE3NSRYt023cdpXVHs4y",
    nodeIDs: ["1:123", "1:124", ...],
    format: "png",
    scale: 1.0
)
```

**Token 状态**:
```
LastFileRequestTime: 1731000000 (刚刚使用过)
LastImageRequestTime: 1730999980
Now: 1731000005
```

**预期结果**:
```
nextAvailableTime = 1731000000 + 30 = 1731000030
队列需要等待 25 秒后执行

日志:
⏳ [Queue] Token 冷却中，队列需等待 25 秒后执行 (next_available_time=1731000030)
🆕 [Queue] 已创建渲染队列 id=1, nodes=100, format=png, scale=1.0
```

### 测试 2: 合并到已有队列

**输入**:
```
已有队列 #1:
  NextAvailableTime: 0 (旧数据)
  
新请求: nodeIDs = ["1:125", "1:126"]
```

**处理流程**:
```
检查 NextAvailableTime: 0 < now → 需要重新计算
计算新的 NextAvailableTime: 1731000030
更新队列

日志:
📝 [Queue] 准备合并节点到队列 id=1, 原节点数=100, 新增节点数=2, 合并后总数=102
✅ [Queue] 已合并到现有队列 id=1, 新增节点数=2, 总节点数=102
```

### 测试 3: Token 冷却完成

**输入**:
```
Token 状态:
  LastFileRequestTime: 1730999950 (50秒前)
  Now: 1731000000
```

**预期结果**:
```
nextAvailableTime = now (立即可用)

日志:
✅ [Queue] Token 冷却已完成，队列可立即执行
🆕 [Queue] 已创建渲染队列 id=2, nodes=50, format=png, scale=2.0
```

## 日志输出示例

### 创建队列时

#### Token 可立即使用
```
✅ [Queue] Token 未使用过，队列可立即执行
🆕 [Queue] 已创建渲染队列 id=1, nodes=100, format=png, scale=1.0
```

#### Token 冷却已完成
```
✅ [Queue] Token 冷却已完成，队列可立即执行
🆕 [Queue] 已创建渲染队列 id=2, nodes=50, format=png, scale=2.0
```

#### Token 正在冷却
```
⏳ [Queue] Token 冷却中，队列需等待 25 秒后执行 (next_available_time=1731000030)
🆕 [Queue] 已创建渲染队列 id=3, nodes=200, format=jpg, scale=1.0
```

### 合并队列时

#### NextAvailableTime 有效（未过期）
```
📝 [Queue] 准备合并节点到队列 id=1, 原节点数=100, 新增节点数=10, 合并后总数=110
✅ [Queue] 已合并到现有队列 id=1, 新增节点数=10, 总节点数=110
```

#### NextAvailableTime 无效（为0或已过期）
```
📝 [Queue] 准备合并节点到队列 id=2, 原节点数=50, 新增节点数=5, 合并后总数=55
⏳ [Queue] Token 冷却中，队列需等待 20 秒后执行 (next_available_time=1731000040)
✅ [Queue] 已合并到现有队列 id=2, 新增节点数=5, 总节点数=55
```

## 边界情况处理

### 1. Token 冷却记录不存在

**场景**: 数据库中没有该 Token 的冷却记录

**处理**:
```go
cooldown, err := models.GetOrCreateTokenCooldown(token)
if err != nil {
    log.Printf("⚠️ [Queue] 获取 Token 冷却状态失败: %v，使用当前时间", err)
    return uint32(time.Now().Unix())
}
```

**结果**: 返回当前时间，队列可立即执行

### 2. 时钟回拨

**场景**: 系统时间被调整到过去

**处理**:
```go
if nextAvailableTime < now {
    log.Printf("✅ [Queue] Token 冷却已完成，队列可立即执行")
    return now
}
```

**结果**: 即使计算出的时间在过去，也会返回当前时间

### 3. 多个队列同时创建

**场景**: 同一 Token 在短时间内创建多个队列

**处理**: 所有队列都会得到相同的 `nextAvailableTime`（基于最后请求时间）

**结果**: 调度器会按创建顺序（`created_at ASC`）依次执行

## 性能优化

### 1. 减少数据库查询

只在需要时查询 Token 冷却状态：
- 创建新队列时查询一次
- 合并队列时只在 `NextAvailableTime` 无效时查询

### 2. 批量创建队列优化

**场景**: 拆分为多个队列时（> 1000 个节点）

**优化**: 只计算一次 `nextAvailableTime`，所有拆分出的队列共用

```go
// 获取 Token 的冷却状态，计算下次可用时间
nextAvailableTime := qs.calculateNextAvailableTime(token)

// 按最大节点数拆分
for i := 0; i < len(nodeIDs); i += int(qs.MaxNodesPerRequest) {
    // ... 每个队列都使用相同的 nextAvailableTime
    queue := &models.FigmaRenderQueue{
        NextAvailableTime: nextAvailableTime,  // 共用
        // ...
    }
}
```

## 相关文档

- [渲染队列系统设计](RENDER_QUEUE_DESIGN.md)
- [Figma API 速率限制处理](FIGMA_API_RATE_LIMIT.md)
- [调度器实现](SCHEDULER_IMPLEMENTATION.md)

---

**修复时间**: 2025-11-19  
**修改文件**: `internal/services/queue.go`  
**核心改进**:
- 实现 `calculateNextAvailableTime` 方法
- 创建队列时正确设置 `NextAvailableTime`
- 合并队列时更新 `NextAvailableTime`（如果无效）
- 增加详细的日志输出

**测试状态**: ✅ 编译通过，等待实际运行验证

