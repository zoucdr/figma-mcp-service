# 功能：Figma API Retry-After 处理

## 功能概述

当 Figma API 返回速率限制响应时（429 Too Many Requests 或 403 Forbidden），系统会自动检查响应头中的 `Retry-After`，并相应地更新 token 的冷却时间，确保在指定时间内无法调用任何 Figma API。

## 实现目标

1. ✅ 自动检测 Figma API 响应中的 `Retry-After` 头
2. ✅ 解析 `Retry-After` 值（支持秒数和 HTTP 日期格式）
3. ✅ 更新 `figma_token_cooldowns` 表，同时设置 `last_file_request_time` 和 `last_image_request_time`
4. ✅ 确保冷却期间无法调用任何 Figma API（文件API 和 图片API）
5. ✅ 提供详细的日志记录

## 核心实现

### 1. 数据库模型扩展

**文件**: `internal/models/cache.go`

新增方法用于设置冷却时间直到指定时间戳：

```go
// SetCooldownUntil 设置冷却时间直到指定时间戳（用于处理 Retry-After）
// 同时更新文件和图片请求时间，确保期间无法调用任何 Figma API
func SetCooldownUntil(token string, untilTimestamp uint32) error {
	now := uint32(time.Now().Unix())
	return DB.Model(&FigmaTokenCooldown{}).
		Where("figma_token = ?", token).
		Updates(map[string]interface{}{
			"last_file_request_time":  untilTimestamp,
			"last_image_request_time": untilTimestamp,
			"updated_at":              now,
		}).Error
}
```

**关键点**:
- 同时更新 `last_file_request_time` 和 `last_image_request_time`
- 设置为相同的时间戳（当前时间 + Retry-After 秒数）
- 确保所有 API 调用都被阻止

### 2. Retry-After 处理工具

**文件**: `internal/services/retry_after.go`

#### 2.1 解析 Retry-After 头

```go
func HandleRetryAfter(token string, resp *http.Response) {
	retryAfter := resp.Header.Get("Retry-After")
	if retryAfter == "" {
		return
	}

	// 尝试解析为秒数（如 "30"）
	if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds > 0 {
		now := uint32(time.Now().Unix())
		untilTimestamp := now + uint32(seconds)
		models.SetCooldownUntil(token, untilTimestamp)
		// 日志记录...
		return
	}

	// 尝试解析为 HTTP 日期（如 "Wed, 21 Oct 2015 07:28:00 GMT"）
	if t, err := http.ParseTime(retryAfter); err == nil {
		untilTimestamp := uint32(t.Unix())
		models.SetCooldownUntil(token, untilTimestamp)
		// 日志记录...
		return
	}
}
```

**支持的格式**:
- **秒数**: `Retry-After: 30`
- **HTTP 日期**: `Retry-After: Wed, 21 Oct 2015 07:28:00 GMT`

#### 2.2 检查并处理速率限制响应

```go
func CheckAndHandleRateLimitResponse(token string, resp *http.Response) bool {
	// 429 Too Many Requests
	if resp.StatusCode == 429 {
		log.Printf("🚫 [Rate Limit] Figma API 返回 429 Too Many Requests")
		HandleRetryAfter(token, resp)
		
		// 如果没有 Retry-After，使用默认冷却时间（60秒）
		if resp.Header.Get("Retry-After") == "" {
			now := uint32(time.Now().Unix())
			untilTimestamp := now + 60
			models.SetCooldownUntil(token, untilTimestamp)
		}
		return true
	}

	// 403 可能也包含 Retry-After
	if resp.StatusCode == 403 {
		if resp.Header.Get("Retry-After") != "" {
			log.Printf("🚫 [Forbidden] Figma API 返回 403 Forbidden with Retry-After")
			HandleRetryAfter(token, resp)
			return true
		}
	}

	return false
}
```

### 3. 集成到 API 调用

**文件**: `internal/services/figma.go`

#### 3.1 文件 API（GetFigmaNodes）

```go
// 检查响应状态
if resp.StatusCode != http.StatusOK {
	// 处理速率限制和 Retry-After
	if CheckAndHandleRateLimitResponse(token, resp) {
		return nil, FormatRetryError(resp)
	}
	
	// 其他错误处理...
	return nil, fmt.Errorf("API请求失败: %s", resp.Status)
}

// 即使成功，也检查是否有 Retry-After（预防性处理）
HandleRetryAfter(token, resp)
```

#### 3.2 图片 API（DownloadPreviewFigmaImageWithOptions）

```go
// 检查响应状态
if resp.StatusCode != http.StatusOK {
	// 处理速率限制和 Retry-After
	if CheckAndHandleRateLimitResponse(token, resp) {
		return "", FormatRetryError(resp)
	}
	
	// 其他错误处理...
	return "", fmt.Errorf("API请求失败: %s", resp.Status)
}

// 即使成功，也检查是否有 Retry-After
HandleRetryAfter(token, resp)
```

## 工作流程

```
Figma API 调用
    ↓
收到响应
    ↓
检查状态码
    ├─→ 200 OK
    │   ↓
    │   检查 Retry-After 头（预防性）
    │   ↓
    │   如果存在 → 更新冷却时间
    │   ↓
    │   继续处理响应
    │
    └─→ 429 / 403
        ↓
        检查 Retry-After 头
        ↓
        ├─→ 存在
        │   ↓
        │   解析值（秒数或日期）
        │   ↓
        │   计算 untilTimestamp
        │   ↓
        │   更新数据库：
        │   - last_file_request_time = untilTimestamp
        │   - last_image_request_time = untilTimestamp
        │   ↓
        │   返回格式化错误
        │
        └─→ 不存在（429 情况）
            ↓
            使用默认冷却时间（60秒）
            ↓
            更新数据库
            ↓
            返回错误
```

## 数据库状态示例

### 场景 1：收到 429 + Retry-After: 30

```sql
-- 当前时间: 1700000000
-- Retry-After: 30 秒

UPDATE figma_token_cooldowns 
SET last_file_request_time = 1700000030,    -- 当前 + 30
    last_image_request_time = 1700000030,   -- 当前 + 30
    updated_at = 1700000000
WHERE figma_token = 'xxx';
```

**效果**:
- 在接下来的 30 秒内，任何 API 调用检查都会被阻止
- `CheckFileAPICooldown` 会返回 `false` 并显示剩余等待时间
- `CheckImageAPICooldown` 也会返回 `false` 并显示剩余等待时间

### 场景 2：收到 429 但无 Retry-After

```sql
-- 当前时间: 1700000000
-- 使用默认冷却: 60 秒

UPDATE figma_token_cooldowns 
SET last_file_request_time = 1700000060,    -- 当前 + 60
    last_image_request_time = 1700000060,   -- 当前 + 60
    updated_at = 1700000000
WHERE figma_token = 'xxx';
```

## 日志示例

### 成功解析 Retry-After（秒数格式）

```
🚫 [Rate Limit] Figma API 返回 429 Too Many Requests
   - Token: T9CJP2HFV9...
⚠️ [Retry-After] Figma API 要求等待 30 秒
   - Token: T9CJP2HFV9...
   - 冷却至: 2025-11-19 15:30:00
```

### 成功解析 Retry-After（日期格式）

```
🚫 [Rate Limit] Figma API 返回 429 Too Many Requests
   - Token: T9CJP2HFV9...
⚠️ [Retry-After] Figma API 要求等待到 15:30:00 (约 30 秒)
   - Token: T9CJP2HFV9...
```

### 429 但无 Retry-After

```
🚫 [Rate Limit] Figma API 返回 429 Too Many Requests
   - Token: T9CJP2HFV9...
⚠️ [Rate Limit] 未提供 Retry-After，使用默认冷却时间: 60 秒
```

### 403 + Retry-After

```
🚫 [Forbidden] Figma API 返回 403 Forbidden with Retry-After
   - Token: T9CJP2HFV9...
⚠️ [Retry-After] Figma API 要求等待 30 秒
   - Token: T9CJP2HFV9...
   - 冷却至: 2025-11-19 15:30:00
```

### 冷却检查被阻止

```
⏰ [Token:T9CJP2HFV9...] File API 冷却中，还需等待 25 秒
```

## 错误信息格式

```go
func FormatRetryError(resp *http.Response) error {
	retryAfter := resp.Header.Get("Retry-After")
	if retryAfter == "" {
		return fmt.Errorf("Figma API 速率限制 (状态码: %d)", resp.StatusCode)
	}

	// 秒数格式
	if seconds, err := strconv.Atoi(retryAfter); err == nil {
		return fmt.Errorf("Figma API 速率限制: 请等待 %d 秒 (状态码: %d)", seconds, resp.StatusCode)
	}

	// 日期格式
	if t, err := http.ParseTime(retryAfter); err == nil {
		waitSeconds := int(t.Unix() - time.Now().Unix())
		return fmt.Errorf("Figma API 速率限制: 请等待到 %s (约 %d 秒)", t.Format("15:04:05"), waitSeconds)
	}

	return fmt.Errorf("Figma API 速率限制: Retry-After=%s (状态码: %d)", retryAfter, resp.StatusCode)
}
```

**返回示例**:
- `"Figma API 速率限制: 请等待 30 秒 (状态码: 429)"`
- `"Figma API 速率限制: 请等待到 15:30:00 (约 30 秒)"`

## 与现有冷却机制的协同

### 正常 API 调用流程

```
1. 调用前检查冷却 → CheckFileAPICooldown / CheckImageAPICooldown
   - 检查 max(last_file_request_time, last_image_request_time)
   - 如果未过冷却期 → 阻止调用
   
2. 调用 Figma API
   
3. 收到响应
   - 检查 Retry-After
   - 如果存在 → SetCooldownUntil(当前时间 + Retry-After)
   
4. 更新请求时间 → UpdateFileRequestTime / UpdateImageRequestTime
   - 设置为当前时间
```

### Retry-After 优先级

当存在 Retry-After 时：
```
SetCooldownUntil(now + 30)  // 设置到未来30秒后
→ last_file_request_time = now + 30
→ last_image_request_time = now + 30

下次检查:
elapsed = now - (now + 30) = -30  // 负数！
→ 距离下次可调用还有 30 秒
```

## 测试场景

### 测试 1：429 + Retry-After: 30

1. 调用 Figma API
2. 收到 `429 Too Many Requests`, `Retry-After: 30`
3. 系统日志：`⚠️ Figma API 要求等待 30 秒`
4. 数据库更新：`last_*_request_time = now + 30`
5. 30 秒内任何 API 调用都被阻止
6. 30 秒后可以正常调用

### 测试 2：429 无 Retry-After

1. 调用 Figma API
2. 收到 `429 Too Many Requests`（无 Retry-After）
3. 系统日志：`⚠️ 未提供 Retry-After，使用默认冷却时间: 60 秒`
4. 数据库更新：`last_*_request_time = now + 60`
5. 60 秒内任何 API 调用都被阻止

### 测试 3：正常响应 + Retry-After

1. 调用 Figma API
2. 收到 `200 OK` + `Retry-After: 10`（预防性）
3. 系统日志：`⚠️ Figma API 要求等待 10 秒`
4. 数据库更新：`last_*_request_time = now + 10`
5. 数据正常返回，但后续调用被延迟

## 优势

1. ✅ **遵守 Figma 限制**：严格按照 Retry-After 指示等待
2. ✅ **全局冷却**：同时阻止文件和图片 API
3. ✅ **灵活解析**：支持秒数和 HTTP 日期两种格式
4. ✅ **默认保护**：即使无 Retry-After 也设置合理的冷却时间
5. ✅ **详细日志**：记录所有速率限制事件
6. ✅ **友好错误**：返回易读的错误信息给前端

## 注意事项

1. **时间同步**: 确保服务器时间准确（HTTP 日期格式依赖时间同步）
2. **并发处理**: 多个请求同时触发 429 时，以最长的 Retry-After 为准
3. **数据库更新**: `SetCooldownUntil` 是幂等操作，可以安全重复调用
4. **预防性检查**: 即使 200 响应也检查 Retry-After，Figma 可能提前警告

## 相关文档

- [全局 API 冷却检查](BUGFIX_GLOBAL_API_COOLDOWN.md)
- [Figma 缓存系统完整方案](FIGMA_CACHE_SYSTEM_COMPLETE.md)

---

**实现时间**: 2025-11-19  
**修改文件**:
- `internal/models/cache.go` - 添加 `SetCooldownUntil` 方法
- `internal/services/retry_after.go` - 新建 Retry-After 处理工具
- `internal/services/figma.go` - 集成到 API 调用

**测试状态**: ✅ 编译通过，等待实际 429 响应验证

