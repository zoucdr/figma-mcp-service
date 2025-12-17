# 优化 downloadFigmaImageWithCache 函数

## 概述

优化了 `downloadFigmaImageWithCache` 函数（位于 `internal/controllers/api.go`），改进图片获取优先级策略，优先使用 WebSocket 获取最新数据。

## 修改内容

### 函数位置
- **文件**: `internal/controllers/api.go`
- **函数**: `downloadFigmaImageWithCache`
- **调用者**: `APIDownloadFigmaImage` API 接口

### 优化获取优先级

#### 旧流程（按顺序）：
1. 检查本地缓存文件
2. 查询数据库（华为云 OBS）
3. 通过 WebSocket 获取（作为兜底）

#### 新流程（优化后）：
1. **✨ 优先检查 WebSocket 连接**
   - 如果用户有活跃的 WebSocket 连接
   - 直接通过 WebSocket 获取**最新数据**
   - WebSocket 成功后自动更新：OBS、数据库、本地文件
2. 检查本地缓存文件（WebSocket 不可用或失败时）
3. 查询数据库并从 OBS 下载
4. 返回错误（所有途径都失败）

## 代码变更详情

### 步骤 1：优先 WebSocket（最新数据）

```go
// 步骤1：优先检查 WebSocket 是否可用，如果可用则使用最新数据
fmt.Printf("步骤1：检查 WebSocket 连接状态 (userID=%d)\n", userID)
mcpService := services.GetMCPService()
if mcpService != nil {
    connection, exists := mcpService.GetUserConnection(userID)
    if exists && connection != nil && mcpService.HasActiveWebSocketConnection(connection.ConnectionID) {
        fmt.Printf("✅ WebSocket 连接存在，优先使用 WebSocket 获取最新图片数据 (ignoreTexts=%v)\n", ignoreTexts)

        // 通过WebSocket调用Figma插件导出图片
        imagePath, err := services.TryGetImageViaWebSocket(fileKey, nodeIDs, format, scale, userID, tempDir, safeNodeIDs, ignoreTexts)
        if err == nil {
            fmt.Printf("✅ 成功通过 WebSocket 获取最新图片: %s\n", imagePath)
            // WebSocket 成功获取，handleImageDataFromPlugin 已经自动更新了 OBS、数据库和本地文件
            return imagePath, nil
        }
        fmt.Printf("⚠️ WebSocket 获取失败: %v，尝试降级到缓存\n", err)
    } else {
        fmt.Printf("⚠️ WebSocket 连接不存在，使用缓存数据\n")
    }
}
```

### 步骤 2：本地缓存（降级）

```go
// 步骤2：WebSocket 不可用或失败，检查本地缓存文件
existingFile := findLatestNodeImageWithIgnoreTexts(tempDir, safeNodeIDs, scale, ignoreTexts)
if existingFile != "" {
    fmt.Printf("✅ 找到节点 %s 的本地缓存图片: %s\n", nodeIDs, existingFile)
    return existingFile, nil
}
fmt.Printf("⚠️ 本地缓存未找到\n")
```

### 步骤 3：数据库 + OBS（再降级）

```go
// 步骤3：从数据库查找华为云OBS URL
fmt.Printf("步骤3：从数据库查找华为云OBS URL\n")
var nodeImage models.FigmaNodeImage
err = models.DB.Where("file_key = ? AND node_id = ? AND format = ? AND scale = ? AND ignore_texts = ? AND status IN ('obs_synced', 'figma_cdn')",
    fileKey, nodeIDs, format, scale, ignoreTexts).First(&nodeImage).Error

if err == nil && nodeImage.OBSKey != "" {
    // 找到了OBS记录，尝试从OBS下载
    imagePath, err := services.DownloadImageFromOBS(nodeImage.OBSKey, tempDir, safeNodeIDs, format, scale, ignoreTexts)
    if err == nil {
        fmt.Printf("✅ 从OBS下载成功: %s\n", imagePath)
        return imagePath, nil
    }
}
```

### 重要改进：数据库查询添加 `ignore_texts` 条件

**旧查询**：
```go
err = models.DB.Where("file_key = ? AND node_id = ? AND format = ? AND scale = ? AND status IN ('obs_synced', 'figma_cdn')",
    fileKey, nodeIDs, format, scale).First(&nodeImage).Error
```

**新查询**：
```go
err = models.DB.Where("file_key = ? AND node_id = ? AND format = ? AND scale = ? AND ignore_texts = ? AND status IN ('obs_synced', 'figma_cdn')",
    fileKey, nodeIDs, format, scale, ignoreTexts).First(&nodeImage).Error
```

这确保查询时会匹配正确的 `ignore_texts` 标记，避免混淆带文字和不带文字的图片。

## 优化原因

### 1. WebSocket 优先的优势
- **数据最新**：直接从 Figma 实时获取，确保数据是最新的
- **自动更新**：`handleImageDataFromPlugin` 会自动更新所有存储层
  - 保存到本地文件
  - 上传到华为云 OBS
  - 更新数据库记录
- **适合 API 场景**：API 下载通常需要最新、最准确的数据

### 2. 降级策略保证可用性
- 如果 WebSocket 不可用或失败，自动降级到缓存数据
- 多层缓存确保系统的高可用性和容错能力

### 3. 数据一致性
- 添加 `ignore_texts` 查询条件，确保获取正确版本的图片
- 避免带文字和不带文字的图片混淆

## 使用场景

### API 接口
- `APIDownloadFigmaImage`: `/api/v1/mcp/:mcptoken/download-image`
- 用于客户端通过 API 下载 Figma 图片

### 效果
- ✅ 确保下载的图片是最新的
- ✅ 自动更新所有缓存层（本地、OBS、数据库）
- ✅ 降级策略保证高可用性
- ✅ 正确处理 `ignore_texts` 参数

## 日志输出示例

### WebSocket 可用时：
```
开始下载Figma图片（带缓存）: fileKey=xxx, nodeIDs=xxx, format=png, scale=1.0, ignoreTexts=false
步骤1：检查 WebSocket 连接状态 (userID=123)
✅ WebSocket 连接存在，优先使用 WebSocket 获取最新图片数据 (ignoreTexts=false)
✅ 成功通过 WebSocket 获取最新图片: temp/xxx/previews/xxx-1x.png
```

### WebSocket 不可用，降级到缓存：
```
开始下载Figma图片（带缓存）: fileKey=xxx, nodeIDs=xxx, format=png, scale=1.0, ignoreTexts=false
步骤1：检查 WebSocket 连接状态 (userID=123)
⚠️ WebSocket 连接不存在，使用缓存数据
✅ 找到节点 xxx 的本地缓存图片: temp/xxx/previews/xxx-1x.png
```

### 从 OBS 下载：
```
开始下载Figma图片（带缓存）: fileKey=xxx, nodeIDs=xxx, format=png, scale=1.0, ignoreTexts=false
步骤1：检查 WebSocket 连接状态 (userID=123)
⚠️ WebSocket 连接不存在，使用缓存数据
⚠️ 本地缓存未找到
步骤3：从数据库查找华为云OBS URL
📦 找到OBS记录: obsKey=xxx, status=obs_synced
✅ 从OBS下载成功: temp/xxx/previews/xxx-1x.png
```

## 相关函数

- `services.GetMCPService()`: 获取 MCP 服务实例
- `mcpService.GetUserConnection()`: 获取用户的 WebSocket 连接
- `mcpService.HasActiveWebSocketConnection()`: 检查 WebSocket 连接是否活跃
- `services.TryGetImageViaWebSocket()`: 通过 WebSocket 获取图片
- `handleImageDataFromPlugin()`: 处理插件返回的图片数据（自动更新所有存储）

## 相关文档

- [优化导出图片下载流程](./OPTIMIZE_DOWNLOAD_FIGMA_IMAGE_FOR_EXPORT.md) - `DownloadFigmaImageForExport` 函数的优化

## 测试建议

1. **WebSocket 可用场景**：
   - 用户已连接 Figma 插件
   - API 下载应该获取最新数据
   - 验证 OBS、数据库、本地文件都已更新

2. **WebSocket 不可用场景**：
   - 用户未连接插件
   - 应该降级到缓存数据
   - 验证降级流程正常工作

3. **ignore_texts 参数**：
   - 测试 `ignore_texts=true` 和 `ignore_texts=false` 分别获取正确的图片版本
   - 验证数据库查询条件正确匹配

4. **数据一致性**：
   - WebSocket 获取后，验证所有存储层数据一致
   - 后续调用应该能从缓存快速获取

