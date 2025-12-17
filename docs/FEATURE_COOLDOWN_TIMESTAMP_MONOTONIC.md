# 功能：冷却时间戳单调递增保护

## 背景

在 Figma API 冷却机制中，`figma_token_cooldowns` 表存储了每个 Token 的最后请求时间：
- `last_file_request_time`: 最后一次文件 API 请求时间
- `last_image_request_time`: 最后一次图片 API 请求时间

这些时间戳用于实现全局速率限制，确保不会过于频繁地调用 Figma API。

## 问题

在并发场景或异步操作中，可能出现时间戳回退的问题：

### 场景 1: 并发请求

```
时间轴:
T1: 请求A 开始处理，读取 last_image_request_time = 1000
T2: 请求B 开始处理，读取 last_image_request_time = 1000
T3: 请求A 完成，更新 last_image_request_time = 1005
T4: 请求B 完成，更新 last_image_request_time = 1003  ❌ 时间戳回退！
```

### 场景 2: Retry-After 与正常请求竞争

```
时间轴:
T1: 429 错误，Retry-After: 60，设置 last_image_request_time = now + 60 = 1060
T2: 另一个慢请求完成，尝试更新 last_image_request_time = 1010  ❌ 会覆盖冷却时间！
```

### 场景 3: 延迟的 defer 调用

```
时间轴:
T1: 请求A 调用 API (now = 1000)
T2: 请求B 调用 API (now = 1005)
T3: 请求B 的 defer 执行，更新 last_image_request_time = 1005
T4: 请求A 的 defer 延迟执行，更新 last_image_request_time = 1000  ❌ 时间戳回退！
```

### 问题影响

1. **冷却机制失效**: 时间戳回退会导致冷却期被缩短
2. **API 速率限制突破**: 可能触发更多的 429 错误
3. **Retry-After 失效**: `HandleRetryAfter` 设置的冷却时间可能被覆盖
4. **数据不一致**: 时间戳不再单调递增

## 解决方案

### 核心原则

**时间戳单调性保证**: 只允许时间戳向前移动，禁止回退。

在所有更新 `last_file_request_time` 和 `last_image_request_time` 的函数中，添加比较逻辑：

```
if new_value > old_value:
    更新时间戳
else:
    跳过更新
```

### 实现细节

#### 1. UpdateFileRequestTime

**修改前**:
```go
func UpdateFileRequestTime(token string) error {
	now := uint32(time.Now().Unix())
	return DB.Model(&FigmaTokenCooldown{}).
		Where("figma_token = ?", token).
		Updates(map[string]interface{}{
			"last_file_request_time": now,
			"updated_at":             now,
		}).Error
}
```

**修改后**:
```go
// UpdateFileRequestTime 更新文件请求时间
// 只有当新值大于当前值时才更新，避免时间戳回退
func UpdateFileRequestTime(token string) error {
	now := uint32(time.Now().Unix())
	
	// 获取当前冷却信息
	cooldown, err := GetOrCreateTokenCooldown(token)
	if err != nil {
		return err
	}
	
	// 只有在新值大于旧值时才更新
	if now <= cooldown.LastFileRequestTime {
		// 时间戳没有前进，跳过更新
		return nil
	}
	
	return DB.Model(&FigmaTokenCooldown{}).
		Where("figma_token = ?", token).
		Updates(map[string]interface{}{
			"last_file_request_time": now,
			"updated_at":             now,
		}).Error
}
```

**关键改进**:
- ✅ 先读取当前值 `cooldown.LastFileRequestTime`
- ✅ 比较 `now <= cooldown.LastFileRequestTime`
- ✅ 如果新值不大于旧值，直接返回 `nil`（跳过更新）
- ✅ 只有新值更大时才执行数据库更新

#### 2. UpdateImageRequestTime

**修改后**:
```go
// UpdateImageRequestTime 更新图片请求时间
// 只有当新值大于当前值时才更新，避免时间戳回退
func UpdateImageRequestTime(token string) error {
	now := uint32(time.Now().Unix())
	
	// 获取当前冷却信息
	cooldown, err := GetOrCreateTokenCooldown(token)
	if err != nil {
		return err
	}
	
	// 只有在新值大于旧值时才更新
	if now <= cooldown.LastImageRequestTime {
		// 时间戳没有前进，跳过更新
		return nil
	}
	
	return DB.Model(&FigmaTokenCooldown{}).
		Where("figma_token = ?", token).
		Updates(map[string]interface{}{
			"last_image_request_time": now,
			"updated_at":              now,
		}).Error
}
```

**关键改进**:
- ✅ 先读取当前值 `cooldown.LastImageRequestTime`
- ✅ 比较 `now <= cooldown.LastImageRequestTime`
- ✅ 跳过不必要的更新

#### 3. SetCooldownUntil

这个函数用于处理 `Retry-After`，需要同时更新两个时间戳。

**修改后**:
```go
// SetCooldownUntil 设置冷却时间直到指定时间戳（用于处理 Retry-After）
// 同时更新文件和图片请求时间，确保期间无法调用任何 Figma API
// 只有当新值大于当前值时才更新，避免时间戳回退
func SetCooldownUntil(token string, untilTimestamp uint32) error {
	now := uint32(time.Now().Unix())
	
	// 获取当前冷却信息
	cooldown, err := GetOrCreateTokenCooldown(token)
	if err != nil {
		return err
	}
	
	// 计算需要更新的字段
	updates := make(map[string]interface{})
	updates["updated_at"] = now
	
	// 只有在新值大于旧值时才更新 last_file_request_time
	if untilTimestamp > cooldown.LastFileRequestTime {
		updates["last_file_request_time"] = untilTimestamp
	}
	
	// 只有在新值大于旧值时才更新 last_image_request_time
	if untilTimestamp > cooldown.LastImageRequestTime {
		updates["last_image_request_time"] = untilTimestamp
	}
	
	// 如果没有需要更新的时间字段，直接返回
	if len(updates) == 1 { // 只有 updated_at
		return nil
	}
	
	return DB.Model(&FigmaTokenCooldown{}).
		Where("figma_token = ?", token).
		Updates(updates).Error
}
```

**关键改进**:
- ✅ 动态构建 `updates` map
- ✅ 分别比较两个时间戳，只更新需要的字段
- ✅ 如果两个时间戳都不需要更新，直接返回

## 执行流程

### 场景 1: 正常更新（时间前进）

```
UpdateImageRequestTime(token)
  ↓
获取当前值: cooldown.LastImageRequestTime = 1000
  ↓
now = 1005
  ↓
比较: 1005 > 1000  ✅
  ↓
执行数据库更新: last_image_request_time = 1005
  ↓
返回 nil (成功)
```

### 场景 2: 跳过更新（时间未前进）

```
UpdateImageRequestTime(token)
  ↓
获取当前值: cooldown.LastImageRequestTime = 1060
  ↓
now = 1005
  ↓
比较: 1005 <= 1060  ❌
  ↓
跳过数据库更新
  ↓
返回 nil (跳过)
```

### 场景 3: SetCooldownUntil 部分更新

```
SetCooldownUntil(token, 1050)
  ↓
获取当前值:
  - cooldown.LastFileRequestTime = 1040
  - cooldown.LastImageRequestTime = 1060
  ↓
比较:
  - 1050 > 1040  ✅  → 更新 last_file_request_time
  - 1050 <= 1060 ❌  → 跳过 last_image_request_time
  ↓
执行数据库更新: 
  updates = {
    "last_file_request_time": 1050,
    "updated_at": now
  }
  ↓
返回 nil (成功)
```

## 并发场景处理

### 并发请求 - 问题修复

**修复前**:
```
时间轴:
T1: 请求A 读取 last_image_request_time = 1000
T2: 请求B 读取 last_image_request_time = 1000
T3: 请求A 完成，更新 last_image_request_time = 1005  ✅
T4: 请求B 完成，更新 last_image_request_time = 1003  ❌ 时间戳回退！
```

**修复后**:
```
时间轴:
T1: 请求A 读取 last_image_request_time = 1000
T2: 请求B 读取 last_image_request_time = 1000
T3: 请求A 完成
    - 获取当前值: 1000
    - 比较: 1005 > 1000  ✅
    - 更新 last_image_request_time = 1005
T4: 请求B 完成
    - 获取当前值: 1005  ← 已被请求A更新
    - 比较: 1003 <= 1005  ❌
    - 跳过更新  ✅ 时间戳保持为 1005
```

### Retry-After 与正常请求 - 问题修复

**修复前**:
```
T1: 429 错误，设置 last_image_request_time = 1060  ✅
T2: 慢请求完成，更新 last_image_request_time = 1010  ❌ 覆盖冷却时间！
```

**修复后**:
```
T1: 429 错误
    - SetCooldownUntil(token, 1060)
    - 更新 last_image_request_time = 1060  ✅
T2: 慢请求完成
    - UpdateImageRequestTime(token)
    - 获取当前值: 1060
    - 比较: 1010 <= 1060  ❌
    - 跳过更新  ✅ 冷却时间保持为 1060
```

## 性能影响

### 额外开销

每次更新都需要先读取当前值：
```go
cooldown, err := GetOrCreateTokenCooldown(token)
```

**开销分析**:
- ➕ 增加 1 次 SELECT 查询
- ➖ 减少可能的无效 UPDATE 查询
- ✅ 避免时间戳回退的逻辑错误
- ✅ 保证数据一致性

**优化考虑**:
- SELECT 操作通常比 UPDATE 快
- 可以利用数据库缓存
- 跳过更新避免了不必要的写操作
- 整体性能影响可接受

### 数据库查询对比

**修改前**:
```
UpdateImageRequestTime → 1 UPDATE 查询
```

**修改后（需要更新）**:
```
UpdateImageRequestTime → 1 SELECT + 1 UPDATE = 2 查询
```

**修改后（跳过更新）**:
```
UpdateImageRequestTime → 1 SELECT = 1 查询 (节省了 UPDATE)
```

## 测试场景

### 测试 1: 正常时间前进

**操作**:
```go
// 当前: last_image_request_time = 1000
time.Sleep(2 * time.Second)
UpdateImageRequestTime(token)  // now = 1002
```

**预期**:
- ✅ 数据库更新: `last_image_request_time = 1002`
- ✅ 返回 `nil`

### 测试 2: 时间戳相同

**操作**:
```go
// 当前: last_image_request_time = 1000
// 立即调用（同一秒内）
UpdateImageRequestTime(token)  // now = 1000
```

**预期**:
- ✅ 跳过数据库更新
- ✅ 返回 `nil`
- ✅ `last_image_request_time` 保持为 1000

### 测试 3: 时间戳回退

**操作**:
```go
// 设置冷却时间
SetCooldownUntil(token, 1060)  // last_image_request_time = 1060

// 尝试用更小的值更新
UpdateImageRequestTime(token)  // now = 1010
```

**预期**:
- ✅ 跳过数据库更新
- ✅ 返回 `nil`
- ✅ `last_image_request_time` 保持为 1060

### 测试 4: 并发更新

**操作**:
```go
// 当前: last_image_request_time = 1000

go func() {
    time.Sleep(1 * time.Second)
    UpdateImageRequestTime(token)  // T=1001
}()

go func() {
    time.Sleep(2 * time.Second)
    UpdateImageRequestTime(token)  // T=1002
}()

go func() {
    time.Sleep(1.5 * time.Second)
    UpdateImageRequestTime(token)  // T=1001 或 1002
}()
```

**预期**:
- ✅ 最终 `last_image_request_time` = 1002（最大值）
- ✅ 无数据竞争
- ✅ 无时间戳回退

### 测试 5: SetCooldownUntil 选择性更新

**操作**:
```go
// 当前状态:
// last_file_request_time = 1040
// last_image_request_time = 1060

SetCooldownUntil(token, 1050)
```

**预期**:
- ✅ 更新 `last_file_request_time = 1050`（1050 > 1040）
- ✅ 跳过 `last_image_request_time`（1050 <= 1060）
- ✅ 返回 `nil`

## 日志示例

### 正常更新

```
✅ [Figma API] 已更新 last_image_request_time: 1731000005
```

### 跳过更新（调试日志）

如需要，可以添加调试日志：
```go
if now <= cooldown.LastImageRequestTime {
    log.Printf("⏭️ [Cooldown] 跳过更新 last_image_request_time: now=%d <= current=%d",
        now, cooldown.LastImageRequestTime)
    return nil
}
```

输出：
```
⏭️ [Cooldown] 跳过更新 last_image_request_time: now=1731000005 <= current=1731000060
```

## 相关函数

### 调用这些更新函数的地方

1. **UpdateFileRequestTime**:
   - `internal/services/figma.go` - `GetFigmaNodesNoCache` 调用后
   - `internal/controllers/cache.go` - 刷新节点树时

2. **UpdateImageRequestTime**:
   - `internal/services/scheduler.go` - `callFigmaImagesAPI` 的 defer
   - 其他调用 Figma Images API 的地方

3. **SetCooldownUntil**:
   - `internal/services/retry_after.go` - `HandleRetryAfter`

### 函数调用链

```
Figma API 调用
  ↓
defer UpdateImageRequestTime(token)
  ↓
获取当前冷却信息
  ↓
比较时间戳
  ↓
决定是否更新

并行:
  
429 错误
  ↓
HandleRetryAfter(token, resp)
  ↓
SetCooldownUntil(token, now + retryAfter)
  ↓
获取当前冷却信息
  ↓
比较两个时间戳
  ↓
选择性更新字段
```

## 最佳实践

### 1. 始终使用提供的函数

❌ **不要直接更新数据库**:
```go
// 错误示例
DB.Model(&FigmaTokenCooldown{}).
    Where("figma_token = ?", token).
    Update("last_image_request_time", time.Now().Unix())
```

✅ **使用封装的函数**:
```go
// 正确示例
models.UpdateImageRequestTime(token)
```

### 2. 不要假设更新一定成功

更新函数可能跳过更新（返回 `nil` 但实际未更新）：

```go
err := models.UpdateImageRequestTime(token)
if err != nil {
    // 处理错误
}
// 注意：err == nil 不代表一定更新了数据库
// 可能因为时间戳未前进而跳过更新
```

### 3. 理解冷却时间的优先级

`SetCooldownUntil` 设置的时间具有最高优先级，因为它通常来自服务器的 `Retry-After` 头部。

优先级顺序：
1. `Retry-After` 设置的时间（最高）
2. 实际 API 调用的时间
3. 历史记录的时间（最低）

## 相关文档

- [Retry-After 处理机制](FEATURE_RETRY_AFTER_HANDLING.md)
- [调度器 Retry-After 和冷却时间](BUGFIX_SCHEDULER_RETRY_AFTER_AND_COOLDOWN.md)
- [Figma API 速率限制方案](FIGMA_API_RATE_LIMIT_SOLUTION.md)

---

**实现时间**: 2025-11-19  
**修改文件**: `internal/models/cache.go`  
**核心原则**: 时间戳单调递增，禁止回退  
**关键函数**:
- `UpdateFileRequestTime`
- `UpdateImageRequestTime`
- `SetCooldownUntil`

**测试状态**: ✅ 编译通过，等待实际运行验证

