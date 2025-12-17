# GetFigmaNodeTree API 兜底功能

## 功能概述

为 `GetFigmaNodeTree` 端点添加 Figma API 兜底功能，在缓存未找到时，满足冷却条件的情况下自动调用 Figma API 获取数据。

## 修改文件

- `internal/controllers/figma.go` - `GetFigmaNodeTree` 函数

## 实现逻辑

### 三级数据获取策略

`GetFigmaNodeTree` 现在按以下顺序尝试获取节点树数据：

1. **文件缓存**（最快）
   - 路径：`temp/{fileKey}/documents/{nodeID}.json`
   - 优点：无需数据库查询，响应最快
   - 返回：`cache_source: "file"`

2. **数据库缓存**（次优）
   - 表：`figma_file_caches`
   - 支持精确匹配和父树查找
   - 返回：`cache_source: "database"`

3. **Figma API 兜底**（最后手段）
   - 条件：Token 冷却完成
   - 自动保存到数据库和文件缓存
   - 返回：`cache_source: "api"`

### API 兜底条件检查

```go
// 1. 检查用户 Figma Token
if user.FigmaToken == "" {
    return 404 // 缓存不存在，请刷新
}

// 2. 检查 CacheService 是否可用
cacheService, exists := c.Get("cacheService")
if !exists {
    return 404 // 缓存不存在，请刷新
}

// 3. 检查 Token 冷却状态
canRequest, waitSeconds, err := cs.CheckFileAPICooldown(user.FigmaToken)
if !canRequest {
    return 503 // API 冷却中，请稍后重试
    // 返回剩余等待时间和下次可用时间
}

// 4. 冷却完成，调用 API
cs.UpdateFileRequestTime(user.FigmaToken)
nodes, err := services.GetFigmaNodesNoCache(token, fileKey, nodeID)
```

## API 响应格式

### 成功响应（不同数据源）

#### 文件缓存
```json
{
  "nodes": [...],
  "cache_source": "file",
  "message": "数据来自文件缓存"
}
```

#### 数据库缓存
```json
{
  "nodes": [...],
  "cache_source": "database",
  "from_database": true,
  "message": "数据来自数据库缓存"
}
```

#### Figma API（兜底成功）
```json
{
  "nodes": [...],
  "cache_source": "api",
  "from_api": true,
  "message": "数据来自 Figma API（已自动缓存）"
}
```

### 错误响应

#### 缓存不存在，API 冷却中
```json
{
  "error": "节点树缓存不存在，且 API 冷却中，请稍后重试",
  "cooldown_remaining": 300,
  "next_available_time": 1700123456
}
```
HTTP Status: `503 Service Unavailable`

#### Token 未设置
```json
{
  "error": "节点树缓存不存在，请先在 Dashboard 中刷新节点树"
}
```
HTTP Status: `404 Not Found`

#### API 调用失败
```json
{
  "error": "获取节点树失败: <具体错误信息>"
}
```
HTTP Status: `500 Internal Server Error`

## 使用场景

### 场景 1：首次访问项目（无任何缓存）
1. 文件缓存 ❌
2. 数据库缓存 ❌
3. 检查 Token 冷却 ✅
4. 调用 Figma API ✅
5. 自动保存缓存
6. 返回数据，`cache_source: "api"`

### 场景 2：缓存过期，Token 冷却中
1. 文件缓存 ❌
2. 数据库缓存 ❌
3. 检查 Token 冷却 ❌（还需等待 300 秒）
4. 返回 `503`，提示冷却剩余时间

### 场景 3：正常访问（有缓存）
1. 文件缓存 ✅
2. 直接返回，`cache_source: "file"`
3. 不调用 API，节省配额

## 优势

1. **提升可用性**：即使缓存缺失，也能自动恢复数据
2. **保护 API 配额**：优先使用缓存，只在必要时调用 API
3. **尊重冷却限制**：不会在冷却期间强制调用 API
4. **自动缓存同步**：API 调用成功后自动更新缓存
5. **用户友好**：清晰的错误提示和状态说明

## 与 RefreshProjectNodeTree 的区别

| 特性 | GetFigmaNodeTree | RefreshProjectNodeTree |
|-----|------------------|------------------------|
| 主要用途 | 获取数据（优先缓存） | 强制刷新数据 |
| 缓存优先级 | 高（优先使用） | 无（跳过缓存） |
| API 调用 | 兜底（缓存失败时） | 主动（每次都尝试） |
| 冷却期行为 | 返回 503 错误 | 加入刷新队列 |
| 队列支持 | 无 | 有（waiting 状态） |

## 日志示例

### 成功从 API 兜底
```
⚠️ [GetFigmaNodeTree] 文件缓存未找到，尝试数据库缓存 (FileKey=xxx, NodeID=xxx)
⚠️ [GetFigmaNodeTree] 数据库缓存未找到，尝试 Figma API 兜底 (FileKey=xxx, NodeID=xxx)
✅ [Token:figd_xxxx...] File API 冷却完成，可以调用
🔄 [GetFigmaNodeTree] 尝试从 Figma API 获取节点树 (FileKey=xxx, NodeID=xxx)
✅ [GetFigmaNodeTree] 从 Figma API 获取节点树成功 (FileKey=xxx, NodeID=xxx, 节点数=42)
```

### API 冷却中
```
⚠️ [GetFigmaNodeTree] 文件缓存未找到，尝试数据库缓存 (FileKey=xxx, NodeID=xxx)
⚠️ [GetFigmaNodeTree] 数据库缓存未找到，尝试 Figma API 兜底 (FileKey=xxx, NodeID=xxx)
⏰ [Token:figd_xxxx...] File API 冷却中，还需等待 300 秒
⏰ [GetFigmaNodeTree] Token 冷却中，需等待 300 秒 (FileKey=xxx, NodeID=xxx)
```

## 依赖要求

需要在 Gin Context 中注入 `cacheService` 实例：

```go
// main.go 或 router 配置中
router.Use(func(c *gin.Context) {
    c.Set("cacheService", cacheService)
    c.Next()
})
```

## 测试建议

1. **有缓存场景**：验证优先使用缓存
2. **无缓存 + 冷却完成**：验证 API 兜底成功
3. **无缓存 + 冷却中**：验证返回 503 状态
4. **Token 未设置**：验证返回 404 状态
5. **API 调用失败**：验证错误处理

## 注意事项

1. API 兜底会消耗 Figma API 配额
2. 冷却期间无法自动恢复，需用户稍后重试
3. 建议前端显示冷却剩余时间，提升用户体验
4. API 调用成功后会自动保存到数据库和文件缓存，下次访问将直接使用缓存

