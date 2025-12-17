# 修复 Figma API 全局冷却检查逻辑

## 问题描述

之前的实现中，文件 API 和图片 API 的冷却检查是**独立的**：
- `CheckFileAPICooldown` 只检查 `last_file_request_time`
- `CheckImageAPICooldown` 只检查 `last_image_request_time`

但实际上，**Figma 的 API 速率限制是全局的**，不管调用哪个接口（文件 API 还是图片 API），都共享同一个速率限制。

## 问题场景

### ❌ 错误的行为（修复前）

```
时间线：
0s:  调用文件 API  → last_file_request_time = 0
10s: 调用图片 API  → last_image_request_time = 10
15s: 再次调用文件 API
     ✗ 检查：now(15) - last_file_request_time(0) = 15s >= 30s? NO, 等待15秒
     ✗ 实际应该：now(15) - last_image_request_time(10) = 5s < 30s，还需等待25秒
```

**结果**：允许请求，但 Figma 可能返回 429 错误

### ✅ 正确的行为（修复后）

```
时间线：
0s:  调用文件 API  → last_file_request_time = 0
10s: 调用图片 API  → last_image_request_time = 10
15s: 再次调用文件 API
     ✓ 检查：max(last_file_request_time(0), last_image_request_time(10)) = 10
     ✓ 判断：now(15) - 10 = 5s < 30s，还需等待 25 秒
```

**结果**：正确阻止请求，避免触发 Figma 的速率限制

## 解决方案

### 核心逻辑

**取两个时间戳中较大的那个**进行冷却判断：

```go
// Figma API 速率限制是全局的，需要检查两个时间戳中较大的那个
lastRequestTime := cooldown.LastFileRequestTime
if cooldown.LastImageRequestTime > lastRequestTime {
    lastRequestTime = cooldown.LastImageRequestTime
}

// 使用全局最后请求时间判断
elapsed := now - lastRequestTime
if elapsed >= cooldownSeconds {
    return true, 0 // 可以请求
}
```

## 代码修改

**文件**: `internal/models/cache.go`

### CheckFileAPICooldown

**修改前**:
```go
func CheckFileAPICooldown(token string, cooldownSeconds uint32) (bool, uint32) {
    // ...
    elapsed := now - cooldown.LastFileRequestTime  // ❌ 只检查文件API时间
    // ...
}
```

**修改后**:
```go
func CheckFileAPICooldown(token string, cooldownSeconds uint32) (bool, uint32) {
    // ...
    
    // Figma API 速率限制是全局的，需要检查两个时间戳中较大的那个
    lastRequestTime := cooldown.LastFileRequestTime
    if cooldown.LastImageRequestTime > lastRequestTime {
        lastRequestTime = cooldown.LastImageRequestTime
    }
    
    elapsed := now - lastRequestTime  // ✅ 使用全局最后请求时间
    // ...
}
```

### CheckImageAPICooldown

**修改前**:
```go
func CheckImageAPICooldown(token string, cooldownSeconds uint32) (bool, uint32) {
    // ...
    elapsed := now - cooldown.LastImageRequestTime  // ❌ 只检查图片API时间
    // ...
}
```

**修改后**:
```go
func CheckImageAPICooldown(token string, cooldownSeconds uint32) (bool, uint32) {
    // ...
    
    // Figma API 速率限制是全局的，需要检查两个时间戳中较大的那个
    lastRequestTime := cooldown.LastImageRequestTime
    if cooldown.LastFileRequestTime > lastRequestTime {
        lastRequestTime = cooldown.LastFileRequestTime
    }
    
    elapsed := now - lastRequestTime  // ✅ 使用全局最后请求时间
    // ...
}
```

## 测试场景

### 场景 1：交替调用不同 API

```go
// 时刻 0: 调用文件 API
UpdateFileRequestTime(token)  // last_file_request_time = 0

// 时刻 10: 调用图片 API  
UpdateImageRequestTime(token)  // last_image_request_time = 10

// 时刻 20: 再次调用文件 API
canCall, remaining := CheckFileAPICooldown(token, 30)
// 修复前: canCall=false, remaining=10 (错误)
// 修复后: canCall=false, remaining=20 (正确)
```

### 场景 2：快速连续调用

```go
// T=0: 文件 API
UpdateFileRequestTime(token)

// T=5: 图片 API (应该被阻止)
canCall, _ := CheckImageAPICooldown(token, 30)
// 修复前: canCall=true (错误，允许调用)
// 修复后: canCall=false (正确，阻止调用)
```

### 场景 3：冷却完成后的首次调用

```go
// T=0: 文件 API
UpdateFileRequestTime(token)

// T=35: 图片 API (冷却已完成)
canCall, _ := CheckImageAPICooldown(token, 30)
// 修复前: canCall=true ✓
// 修复后: canCall=true ✓ (两者都正确)
```

## 数据库状态示例

### 示例 1：文件 API 后立即调用图片 API

```sql
-- T=0: 调用文件 API
UPDATE figma_token_cooldowns 
SET last_file_request_time = 1700000000,
    updated_at = 1700000000
WHERE figma_token = 'xxx';

-- T=5: 尝试调用图片 API
-- 检查：max(1700000000, 0) = 1700000000
-- 经过时间：1700000005 - 1700000000 = 5 秒
-- 结果：5 < 30，还需等待 25 秒 → 阻止调用
```

### 示例 2：图片 API 后立即调用文件 API

```sql
-- T=0: 调用图片 API
UPDATE figma_token_cooldowns 
SET last_image_request_time = 1700000000,
    updated_at = 1700000000
WHERE figma_token = 'xxx';

-- T=10: 尝试调用文件 API
-- 检查：max(0, 1700000000) = 1700000000
-- 经过时间：1700000010 - 1700000000 = 10 秒
-- 结果：10 < 30，还需等待 20 秒 → 阻止调用
```

## 影响范围

### ✅ 受影响的接口

所有使用 Figma API 的接口都受此修复影响：

1. **文件 API**
   - `GET /figma/project/:project_id/node-tree/refresh`
   - 节点树刷新功能

2. **图片 API**
   - `GET /figma/node/image`
   - 渲染队列处理（调度器）

### 🎯 预期改进

- ✅ **减少 429 错误**：避免违反 Figma 的全局速率限制
- ✅ **更准确的冷却**：正确计算剩余等待时间
- ✅ **提升稳定性**：防止因速率限制导致的服务中断

## 为什么需要分别记录两个时间戳？

虽然检查时使用**全局最后请求时间**，但仍需分别记录两个时间戳，原因如下：

### 1. 精确追踪
```go
// 可以知道上次调用的是哪个 API
if cooldown.LastFileRequestTime > cooldown.LastImageRequestTime {
    // 上次是文件 API
} else {
    // 上次是图片 API
}
```

### 2. 监控统计
```sql
-- 统计各类 API 的调用频率
SELECT 
    COUNT(CASE WHEN last_file_request_time > last_image_request_time THEN 1 END) as file_api_calls,
    COUNT(CASE WHEN last_image_request_time > last_file_request_time THEN 1 END) as image_api_calls
FROM figma_token_cooldowns;
```

### 3. 调试诊断
```
日志：
[Token:ABC123] 上次文件 API: 2025-11-19 10:00:00
[Token:ABC123] 上次图片 API: 2025-11-19 10:05:00
[Token:ABC123] 全局冷却: 还需等待 25 秒
```

## Figma API 速率限制说明

### 官方限制

根据 Figma 文档，速率限制为：
- **限制**: 每个 Token 每 30 秒最多 1 次请求
- **范围**: 全局限制，不区分 API 类型
- **错误**: 超限返回 `429 Too Many Requests`

### 限制类型

```
Token-based Rate Limit (全局)
├── GET /v1/files/:file_key
├── GET /v1/files/:file_key/nodes
├── GET /v1/images/:file_key
└── GET /v1/files/:file_key/versions
    ↓
   30 秒冷却期（共享）
```

## 日志示例

### 修复前（错误）

```
[10:00:00] ✅ File API 冷却完成，可以调用
[10:00:00] 📝 已更新 File API 请求时间
[10:00:10] ✅ Images API 冷却完成，可以调用  ← 错误：应该等待
[10:00:10] 📝 已更新 Images API 请求时间
[10:00:10] ❌ Figma API 错误: 429 Too Many Requests
```

### 修复后（正确）

```
[10:00:00] ✅ File API 冷却完成，可以调用
[10:00:00] 📝 已更新 File API 请求时间
[10:00:10] ⏰ Images API 冷却中，还需等待 20 秒  ← 正确阻止
[10:00:35] ✅ Images API 冷却完成，可以调用
[10:00:35] 📝 已更新 Images API 请求时间
```

## 测试验证

### 单元测试

```go
func TestGlobalCooldown(t *testing.T) {
    token := "test_token"
    
    // T=0: 调用文件 API
    UpdateFileRequestTime(token)
    
    // T=0: 立即调用图片 API（应该被阻止）
    canCall, remaining := CheckImageAPICooldown(token, 30)
    assert.False(t, canCall)
    assert.Equal(t, uint32(30), remaining)
    
    // T=35: 冷却完成
    time.Sleep(35 * time.Second)
    canCall, _ = CheckImageAPICooldown(token, 30)
    assert.True(t, canCall)
}
```

### 集成测试

```bash
# 1. 调用文件 API
curl -X GET "http://localhost:8080/figma/project/8/node-tree/refresh"

# 2. 立即调用图片 API（应返回 429）
curl -X GET "http://localhost:8080/figma/node/image?file_key=xxx&node_id=1:123"

# 预期：返回冷却中错误，而不是触发 Figma 的 429
```

## 性能影响

### ✅ 无性能损失
- 只是多了一次 `if` 比较（纳秒级）
- 数据库查询次数不变
- 整体逻辑更简洁

### ✅ 减少无效请求
- 避免触发 Figma 429 错误
- 减少重试次数
- 提升整体效率

## 总结

✅ 修复了冷却检查逻辑，正确反映 Figma API 的全局速率限制  
✅ 取两个时间戳中较大的值进行判断  
✅ 同时保留两个时间戳用于追踪和统计  
✅ 减少 429 错误，提升系统稳定性  
✅ 无性能损失，逻辑更正确

---

**修复时间**: 2025-11-19  
**修改文件**: `internal/models/cache.go`  
**影响函数**: 
- `CheckFileAPICooldown`
- `CheckImageAPICooldown`

**相关文档**: 
- [节点树刷新冷却检查](BUGFIX_NODE_TREE_REFRESH_WITH_COOLDOWN.md)
- [Figma缓存系统完整方案](FIGMA_CACHE_RATE_LIMIT_SOLUTION.md)

