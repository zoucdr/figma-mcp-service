# 修复：调度器 Retry-After 处理和冷却时间更新

## 问题描述

在调度器处理渲染队列时遇到 Figma API 429 错误，但没有正确处理 `Retry-After` 头部信息，也没有更新 Token 的请求时间记录。

### 问题场景

```
[队列 #5] Figma API 调用失败: Figma API 错误 (状态码 429): 
{"status":429,"err":"Rate limit exceeded"}
```

### 存在的问题

1. **未处理 Retry-After 头**: 429 错误时未检查和处理 `Retry-After` 头部信息
2. **未更新请求时间**: 无论成功或失败，都应该更新 `last_image_request_time`
3. **队列重试逻辑不完善**: 429 错误时没有延长队列的 `next_available_time`
4. **缺少调试信息**: 错误时没有打印完整的响应头信息

## 解决方案

### 1. 在 API 调用时更新请求时间

使用 `defer` 确保无论成功或失败，都记录此次 API 调用。

**文件**: `internal/services/scheduler.go`

```go
// callFigmaImagesAPI 调用 Figma Images API
func (s *SchedulerService) callFigmaImagesAPI(fileKey, nodeIDs, format string, scale float64, token string) (map[string]string, error) {
	// 无论成功或失败，都记录此次 API 调用时间
	defer func() {
		now := uint32(time.Now().Unix())
		err := models.UpdateImageRequestTime(token)
		if err != nil {
			log.Printf("⚠️ [Figma API] 更新 last_image_request_time 失败: %v", err)
		} else {
			log.Printf("✅ [Figma API] 已更新 last_image_request_time: %d", now)
		}
	}()
	
	// ... 原有代码 ...
}
```

**关键点**:
- ✅ 使用 `defer` 确保在任何返回路径都会执行
- ✅ 调用 `models.UpdateImageRequestTime(token)` 更新时间戳
- ✅ 记录更新结果

### 2. 处理 Retry-After 头部信息

在检测到错误状态码时，打印完整的响应头并处理 `Retry-After`。

**文件**: `internal/services/scheduler.go`

```go
// 检查状态码
if resp.StatusCode != 200 {
	// 打印完整的响应头信息（用于调试）
	log.Printf("❌ [Figma API] 响应状态码: %d", resp.StatusCode)
	log.Printf("📋 [Figma API] 响应头信息:")
	for key, values := range resp.Header {
		for _, value := range values {
			log.Printf("    %s: %s", key, value)
		}
	}
	log.Printf("📄 [Figma API] 响应体: %s", string(body))
	
	// 处理 429 速率限制错误
	if resp.StatusCode == http.StatusTooManyRequests {
		// 检查并处理 Retry-After 头
		HandleRetryAfter(token, resp)
		
		// 获取 Retry-After 信息用于错误消息
		retryAfter := resp.Header.Get("Retry-After")
		if retryAfter != "" {
			return nil, fmt.Errorf("Figma API 速率限制 (状态码 429, Retry-After: %s): %s", retryAfter, string(body))
		}
	}
	
	return nil, fmt.Errorf("Figma API 错误 (状态码 %d): %s", resp.StatusCode, string(body))
}
```

**关键点**:
- ✅ 打印完整的响应头（所有 key-value 对）
- ✅ 打印响应体用于调试
- ✅ 调用 `HandleRetryAfter(token, resp)` 处理速率限制
- ✅ 在错误消息中包含 `Retry-After` 值

### 3. 成功时也检查 Retry-After

即使请求成功，服务器也可能返回 `Retry-After` 头作为预防性措施。

**文件**: `internal/services/scheduler.go`

```go
if len(result.Images) == 0 {
	return nil, fmt.Errorf("未获取到任何图片 URL")
}

// 即使成功，也检查是否有 Retry-After（预防性处理）
HandleRetryAfter(token, resp)

return result.Images, nil
```

### 4. 队列 429 错误处理增强

在 `processQueue` 中，当遇到 429 错误时，需要：
1. 读取 Token 的冷却信息
2. 计算下次可用时间
3. 更新队列的 `next_available_time`
4. 重新排队等待

**文件**: `internal/services/scheduler.go`

```go
// 调用 Figma Images API
imageURLs, err := s.callFigmaImagesAPI(queue.FileKey, queue.NodeIDs, queue.Format, queue.Scale, queue.FigmaToken)
if err != nil {
	log.Printf("❌ [队列 #%d] Figma API 调用失败: %v", queue.ID, err)

	// 检查是否是 429 速率限制错误
	if strings.Contains(err.Error(), "429") {
		// 429 错误，需要重新排队并延长等待时间
		log.Printf("⏳ [队列 #%d] 遇到 429 速率限制，重新排队等待", queue.ID)
		
		// 获取 Token 冷却信息，计算下次可用时间
		cooldown, err := models.GetOrCreateTokenCooldown(queue.FigmaToken)
		if err != nil {
			log.Printf("⚠️ [队列 #%d] 获取冷却信息失败: %v", queue.ID, err)
		} else {
			// 取两个时间戳中较大的作为下次可用时间
			nextAvailableTime := cooldown.LastFileRequestTime
			if cooldown.LastImageRequestTime > nextAvailableTime {
				nextAvailableTime = cooldown.LastImageRequestTime
			}
			
			// 更新队列的下次可用时间
			now := uint32(time.Now().Unix())
			if nextAvailableTime > now {
				waitSeconds := nextAvailableTime - now
				log.Printf("⏰ [队列 #%d] 设置下次可用时间: %d (需等待 %d 秒)", queue.ID, nextAvailableTime, waitSeconds)
				
				// 更新队列状态和下次可用时间
				updateErr := models.DB.Model(&models.FigmaRenderQueue{}).
					Where("id = ?", queue.ID).
					Updates(map[string]interface{}{
						"status":              "waiting",
						"next_available_time": nextAvailableTime,
						"updated_at":          now,
					}).Error
				
				if updateErr != nil {
					log.Printf("❌ [队列 #%d] 更新队列时间失败: %v", queue.ID, updateErr)
				}
			} else {
				// 冷却时间已过，30秒后重试
				nextAvailableTime = now + 30
				log.Printf("⏰ [队列 #%d] 冷却时间已过，30秒后重试: %d", queue.ID, nextAvailableTime)
				
				updateErr := models.DB.Model(&models.FigmaRenderQueue{}).
					Where("id = ?", queue.ID).
					Updates(map[string]interface{}{
						"status":              "waiting",
						"next_available_time": nextAvailableTime,
						"updated_at":          now,
					}).Error
				
				if updateErr != nil {
					log.Printf("❌ [队列 #%d] 更新队列时间失败: %v", queue.ID, updateErr)
				}
			}
		}
		
		// 标记为有活跃队列
		s.hasActiveQueues = true
	} else {
		// 非 429 错误，标记为失败
		log.Printf("❌ [队列 #%d] API调用失败（非速率限制错误），标记为失败", queue.ID)
		err = s.queueService.UpdateQueueStatus(queue.ID, "failed", 0)
		if err != nil {
			log.Printf("❌ [队列 #%d] 更新状态失败: %v", queue.ID, err)
		}
	}
	return
}
```

**关键逻辑**:

1. **检测 429 错误**: `strings.Contains(err.Error(), "429")`
2. **获取冷却信息**: `models.GetOrCreateTokenCooldown(queue.FigmaToken)`
3. **计算下次可用时间**: 取 `LastFileRequestTime` 和 `LastImageRequestTime` 中较大的值
4. **更新队列**:
   - 如果 `nextAvailableTime > now`: 使用该时间（说明 `Retry-After` 已更新）
   - 如果 `nextAvailableTime <= now`: 设置为 `now + 30`（默认 30 秒后重试）
5. **标记活跃队列**: `s.hasActiveQueues = true` 确保调度器继续检查

## 调用流程

### 正常流程

```
processQueue()
  ↓
callFigmaImagesAPI()
  ↓
defer: UpdateImageRequestTime(token)  ← 记录请求时间
  ↓
HTTP Request to Figma API
  ↓
StatusCode = 200 ✅
  ↓
HandleRetryAfter(token, resp)  ← 检查是否有 Retry-After
  ↓
return imageURLs
```

### 429 错误流程

```
processQueue()
  ↓
callFigmaImagesAPI()
  ↓
defer: UpdateImageRequestTime(token)  ← 记录请求时间
  ↓
HTTP Request to Figma API
  ↓
StatusCode = 429 ❌
  ↓
打印完整响应头和响应体
  ↓
HandleRetryAfter(token, resp)  ← 更新 Token 冷却时间
  ↓
return error "429"
  ↓
processQueue 检测到 "429" 错误
  ↓
GetOrCreateTokenCooldown(token)
  ↓
计算 nextAvailableTime = max(LastFileRequestTime, LastImageRequestTime)
  ↓
更新队列: status="waiting", next_available_time=计算值
  ↓
标记 hasActiveQueues=true
  ↓
调度器稍后重试
```

## 日志输出示例

### 成功请求

```
🌐 [Figma API] 请求 URL: https://api.figma.com/v1/images/xxx?ids=1:100,1:101&format=png&scale=1.0
🌐 [Figma API] Token: MV1O7AVIY...
✅ [Figma API] 已更新 last_image_request_time: 1731000000
```

### 429 错误（带 Retry-After）

```
🌐 [Figma API] 请求 URL: https://api.figma.com/v1/images/xxx?ids=1:100,1:101&format=png&scale=1.0
🌐 [Figma API] Token: MV1O7AVIY...
❌ [Figma API] 响应状态码: 429
📋 [Figma API] 响应头信息:
    Content-Type: application/json
    Retry-After: 60
    X-RateLimit-Limit: 1000
    X-RateLimit-Remaining: 0
    X-RateLimit-Reset: 1731000060
📄 [Figma API] 响应体: {"status":429,"err":"Rate limit exceeded"}
⏳ Figma API 冷却时间已更新至 2025-11-19 20:01:00 (Retry-After: 60)
✅ [Figma API] 已更新 last_image_request_time: 1731000000
❌ [队列 #5] Figma API 调用失败: Figma API 速率限制 (状态码 429, Retry-After: 60): {"status":429,"err":"Rate limit exceeded"}
⏳ [队列 #5] 遇到 429 速率限制，重新排队等待
⏰ [队列 #5] 设置下次可用时间: 1731000060 (需等待 60 秒)
```

### 429 错误（无 Retry-After）

```
🌐 [Figma API] 请求 URL: https://api.figma.com/v1/images/xxx?ids=1:100,1:101&format=png&scale=1.0
🌐 [Figma API] Token: MV1O7AVIY...
❌ [Figma API] 响应状态码: 429
📋 [Figma API] 响应头信息:
    Content-Type: application/json
📄 [Figma API] 响应体: {"status":429,"err":"Rate limit exceeded"}
✅ [Figma API] 已更新 last_image_request_time: 1731000000
❌ [队列 #5] Figma API 调用失败: Figma API 错误 (状态码 429): {"status":429,"err":"Rate limit exceeded"}
⏳ [队列 #5] 遇到 429 速率限制，重新排队等待
⏰ [队列 #5] 冷却时间已过，30秒后重试: 1731000030
```

## 关键改进点

### 1. 使用 defer 确保记录请求时间

**优点**:
- ✅ 无论成功、失败、panic，都会执行
- ✅ 代码更简洁，无需在每个返回点都调用
- ✅ 确保时间记录的准确性

### 2. 完整的响应头打印

**优点**:
- ✅ 便于调试和诊断问题
- ✅ 可以看到所有速率限制相关的头部信息
- ✅ 帮助理解 Figma API 的行为

**打印的头部信息包括**:
- `Retry-After`: 重试等待时间
- `X-RateLimit-Limit`: 速率限制总数
- `X-RateLimit-Remaining`: 剩余配额
- `X-RateLimit-Reset`: 配额重置时间
- `Content-Type`: 响应类型
- 其他所有响应头

### 3. 智能队列重试

**优点**:
- ✅ 429 错误时自动重新排队，不会丢失任务
- ✅ 根据 Token 冷却状态动态计算下次可用时间
- ✅ 避免无效的 API 调用
- ✅ 保持队列活跃状态，确保调度器继续处理

**重试时间计算**:
```
nextAvailableTime = max(LastFileRequestTime, LastImageRequestTime)

if nextAvailableTime > now:
    使用该时间（Retry-After 已设置）
else:
    使用 now + 30（默认 30 秒后重试）
```

### 4. 全局 API 冷却时间

**工作原理**:
- File API 和 Image API 共享冷却时间
- 任意一个 API 调用后，两个 API 都需要等待
- `HandleRetryAfter` 同时更新 `last_file_request_time` 和 `last_image_request_time`

**实现**:
```go
// SetCooldownUntil 设置冷却时间直到指定时间戳
func SetCooldownUntil(token string, untilTimestamp uint32) error {
	now := uint32(time.Now().Unix())
	return DB.Model(&FigmaTokenCooldown{}).
		Where("figma_token = ?", token).
		Updates(map[string]interface{}{
			"last_file_request_time":  untilTimestamp,
			"last_image_request_time": untilTimestamp,  // 同时更新两个字段
			"updated_at":              now,
		}).Error
}
```

## 数据库更新

### FigmaTokenCooldown 更新时机

| 时机 | 方法 | 更新字段 | 值 |
|------|------|----------|-----|
| Images API 调用后 | `UpdateImageRequestTime` | `last_image_request_time` | `now` |
| File API 调用后 | `UpdateFileRequestTime` | `last_file_request_time` | `now` |
| 收到 Retry-After | `SetCooldownUntil` | `last_file_request_time`<br>`last_image_request_time` | `now + Retry-After` |

### FigmaRenderQueue 更新时机

| 时机 | 更新字段 | 值 | 说明 |
|------|----------|-----|------|
| 遇到 429 错误 | `status`<br>`next_available_time`<br>`updated_at` | `"waiting"`<br>计算值<br>`now` | 重新排队 |
| 处理中 | `status` | `"processing"` | 开始处理 |
| 成功 | `status`<br>`progress` | `"completed"`<br>`100` | 完成 |
| 失败（非429） | `status` | `"failed"` | 标记失败 |

## 测试场景

### 场景 1: 正常请求

**操作**: 调用 Figma Images API，返回 200

**预期**:
- ✅ `last_image_request_time` 更新为当前时间
- ✅ 返回图片 URL 列表
- ✅ 队列状态更新为 `processing` → `completed`

### 场景 2: 429 错误（有 Retry-After）

**操作**: 调用 Figma Images API，返回 429，Retry-After: 60

**预期**:
- ✅ `last_image_request_time` 更新为当前时间
- ✅ `last_file_request_time` 和 `last_image_request_time` 都更新为 `now + 60`
- ✅ 打印完整的响应头和响应体
- ✅ 队列状态更新为 `waiting`
- ✅ 队列 `next_available_time` 更新为 `now + 60`
- ✅ 60 秒后调度器重新处理队列

### 场景 3: 429 错误（无 Retry-After）

**操作**: 调用 Figma Images API，返回 429，无 Retry-After

**预期**:
- ✅ `last_image_request_time` 更新为当前时间
- ✅ 打印完整的响应头和响应体
- ✅ 队列状态更新为 `waiting`
- ✅ 队列 `next_available_time` 更新为 `now + 30`（默认）
- ✅ 30 秒后调度器重新处理队列

### 场景 4: 其他错误（400, 500 等）

**操作**: 调用 Figma Images API，返回 400/500

**预期**:
- ✅ `last_image_request_time` 更新为当前时间
- ✅ 打印完整的响应头和响应体
- ✅ 队列状态更新为 `failed`
- ✅ 不再重试

## 相关代码文件

- `internal/services/scheduler.go` - 调度器主逻辑
- `internal/services/retry_after.go` - Retry-After 处理
- `internal/models/cache.go` - Token 冷却管理
- `internal/services/queue.go` - 队列管理

## 相关文档

- [Retry-After 处理机制](FEATURE_RETRY_AFTER_HANDLING.md)
- [渲染队列设计](RENDER_QUEUE_DESIGN.md)
- [Figma API 速率限制](FIGMA_API_RATE_LIMIT_SOLUTION.md)

---

**修复时间**: 2025-11-19  
**修改文件**: 
- `internal/services/scheduler.go`
- `internal/controllers/figma.go` (临时禁用部分功能)

**核心改进**:
- ✅ 使用 `defer` 确保所有 API 调用都记录时间
- ✅ 打印完整的响应头和响应体用于调试
- ✅ 处理 `Retry-After` 头部信息
- ✅ 429 错误时智能重新排队
- ✅ 根据 Token 冷却状态动态计算重试时间

**测试状态**: ✅ 编译通过，等待实际运行验证

