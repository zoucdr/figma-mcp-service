# WebSocket 图片兜底功能

## 📋 功能说明

在 `DownloadPreviewFigmaImageWithOptions` 函数中，当所有常规途径（本地缓存、数据库缓存、OBS、Figma CDN）都失败时，如果当前用户有活跃的 WebSocket 连接，系统会尝试通过 WebSocket 从 Figma 插件直接获取图片数据，作为最后的兜底手段。

## 🔄 工作流程

```
用户请求图片
    ↓
1. 检查本地缓存
    ↓ (未找到)
2. 检查数据库缓存 (figma_node_images)
    ↓ (未找到)
3. 尝试从 OBS 下载
    ↓ (失败)
4. 尝试从 Figma CDN 下载
    ↓ (失败)
5. 检查 WebSocket 连接是否可用
    ↓ (可用)
6. 发送 export_node_as_image 命令到 Figma 插件
    ↓
7. 等待响应 (30秒超时)
    ↓
8. 收到 base64 图片数据
    ↓
9. 解码并保存到本地
    ↓
10. 异步上传到华为云 OBS
    ↓
11. 保存到数据库缓存
    ↓
12. 返回本地文件给用户
    ↓ (如果所有途径都失败)
返回裂图
```

## 📦 插件响应格式

Figma 插件返回的图片数据格式：

```json
{
  "format": "PNG",
  "imageData": "iVBORw0KG...base64数据...",
  "mimeType": "image/png",
  "nodeId": "46:1386",
  "scale": 1
}
```

**字段说明**：
- `format`: 图片格式（PNG, JPG, SVG, PDF）
- `imageData`: base64 编码的图片数据
- `mimeType`: MIME 类型（image/png, image/jpeg 等）
- `nodeId`: 节点 ID
- `scale`: 缩放比例

## 🔧 实现细节

### 1. 修改的函数签名

```go
// 旧签名
func DownloadPreviewFigmaImageWithOptions(
    token, fileKey, nodeID, imageFormat string, 
    imageScale float64, 
    useCache bool
) (string, error)

// 新签名 - 添加了 userID 参数
func DownloadPreviewFigmaImageWithOptions(
    token, fileKey, nodeID, imageFormat string, 
    imageScale float64, 
    useCache bool,
    userID uint  // 用于 WebSocket 兜底，为 0 则不尝试
) (string, error)
```

### 2. WebSocket 兜底逻辑

```go
// 5. 如果提供了有效的 userID，尝试通过 WebSocket 从 Figma 插件获取图片
if userID > 0 {
    fmt.Printf("🔌 [DownloadPreviewFigmaImageWithOptions] 尝试通过 WebSocket 获取图片 (UserID=%d)\n", userID)
    localPath, err := tryGetImageViaWebSocket(fileKey, nodeID, imageFormat, imageScale, userID, tempDir, safeNodeID)
    if err == nil && localPath != "" {
        fmt.Printf("✅ [DownloadPreviewFigmaImageWithOptions] 成功通过 WebSocket 获取图片: %s\n", localPath)
        return localPath, nil
    }
    fmt.Printf("⚠️ [DownloadPreviewFigmaImageWithOptions] WebSocket 获取失败: %v\n", err)
} else {
    fmt.Printf("⚠️ [DownloadPreviewFigmaImageWithOptions] 未提供 userID，跳过 WebSocket 兜底\n")
}

// 6. 都失败了，返回裂图
fmt.Printf("❌ 所有获取途径都失败，返回裂图\n")
return getFallbackImage()
```

### 3. tryGetImageViaWebSocket 函数

```go
func tryGetImageViaWebSocket(
    fileKey, nodeID, imageFormat string, 
    imageScale float64, 
    userID uint, 
    tempDir, safeNodeID string
) (string, error)
```

**功能**：
1. 检查用户是否有活跃的 WebSocket 连接
2. 构造 `export_node_as_image` 工具调用
3. 发送请求到 Figma 插件
4. 等待响应（30秒超时）
5. 解析 base64 图片数据
6. 调用 `handleImageDataFromPlugin` 处理图片

### 4. handleImageDataFromPlugin 函数

```go
func handleImageDataFromPlugin(
    fileKey, nodeID, imageFormat string, 
    imageScale float64, 
    imageDataB64 string, 
    responseData map[string]interface{}, 
    tempDir, safeNodeID string
) (string, error)
```

**功能**：
1. 解码 base64 图片数据
2. 确定 MIME 类型和文件扩展名
3. 保存到本地缓存
4. 异步上传到华为云 OBS
5. 保存或更新数据库缓存记录

## 📝 调用示例

### Controller 层调用（有 WebSocket 兜底）

```go
imagePath, imgErr = services.DownloadPreviewFigmaImageWithOptions(
    user.FigmaToken, 
    project.FileKey, 
    nodeID, 
    format, 
    scale, 
    useCache,
    user.ID  // ✅ 提供 userID，启用 WebSocket 兜底
)
```

### Service 层调用（无 WebSocket 兜底）

```go
imagePath, err := DownloadPreviewFigmaImageWithOptions(
    token, 
    fileKey, 
    nodeID, 
    format, 
    scale, 
    true,
    0  // ❌ userID=0，跳过 WebSocket 兜底
)
```

## 🎯 使用场景

### 场景 1：用户在 Figma 插件中查看预览

- **情况**：用户已登录，Figma 插件已连接
- **行为**：如果本地缓存、数据库、OBS、CDN 都没有图片，系统会通过 WebSocket 从插件获取
- **优势**：即使服务端没有缓存，也能实时获取最新的图片

### 场景 2：导出任务后台处理

- **情况**：后台导出任务，没有用户 WebSocket 连接
- **行为**：只使用缓存和 CDN，不尝试 WebSocket
- **优势**：避免等待不存在的 WebSocket 响应，提高效率

## 🔄 与现有功能的关系

### GetFigmaImage 接口

之前实现的 `GetFigmaImage` 接口在 **无缓存且有 WebSocket 连接时** 会主动通过插件获取图片。

```go
// GetFigmaImage 接口的 WebSocket 逻辑（主动获取）
if !hasCacheOrDB && hasWebSocket {
    // 发送 export_node_as_image 命令
    // 等待响应
    // 解码并保存图片
}
```

### DownloadPreviewFigmaImageWithOptions 函数

现在 `DownloadPreviewFigmaImageWithOptions` 函数在 **所有途径失败时** 才尝试 WebSocket（兜底）。

```go
// DownloadPreviewFigmaImageWithOptions 的 WebSocket 逻辑（兜底）
if localCache_failed && dbCache_failed && OBS_failed && CDN_failed {
    if hasWebSocket {
        // 尝试通过 WebSocket 获取
    } else {
        // 返回裂图
    }
}
```

## ⚡ 性能优化

1. **异步上传 OBS**：图片保存到本地后立即返回，OBS 上传在后台进行
2. **超时控制**：WebSocket 请求 30 秒超时，避免长时间等待
3. **智能跳过**：如果 `userID=0`，直接跳过 WebSocket 尝试，快速返回裂图

## 🚨 注意事项

1. **避免过度依赖**：WebSocket 兜底应该是最后手段，正常情况下应该有缓存
2. **超时时间**：30 秒是合理的，既不会让用户等太久，也能给插件足够的时间
3. **错误处理**：WebSocket 失败后要优雅地返回裂图，不要抛出异常
4. **日志记录**：详细记录每一步的执行情况，便于排查问题

## 📊 日志示例

```
📖 获取Figma图片: fileKey=xxx, nodeID=46:1386, format=png, scale=1.0, useCache=true, userID=123
⚠️ 本地缓存未找到
⚠️ 数据库中未找到图片记录
🔌 [DownloadPreviewFigmaImageWithOptions] 尝试通过 WebSocket 获取图片 (UserID=123)
🔌 [tryGetImageViaWebSocket] 发现活跃 WebSocket 连接 (UserID=123, ConnectionID=conn_xxx)
✅ [tryGetImageViaWebSocket] 已发送请求到频道 figma-bridge，等待响应...
✅ [tryGetImageViaWebSocket] 收到响应 (ID=req_img_xxx)
✅ [tryGetImageViaWebSocket] 收到 base64 图片数据，大小: 1234 bytes
🎨 [handleImageDataFromPlugin] 开始处理插件图片: fileKey=xxx, nodeID=46:1386, format=png, scale=1.0
✅ [handleImageDataFromPlugin] 图片解码成功，大小: 5678 bytes
✅ [handleImageDataFromPlugin] 图片已保存到本地: temp/xxx/previews/46_1386-1-abc123.png
📤 [handleImageDataFromPlugin] 开始上传到 OBS
✅ [handleImageDataFromPlugin] 成功上传到 OBS: images/xxx/46:1386/46_1386-1-abc123.png
✅ [handleImageDataFromPlugin] 已创建数据库记录
✅ [DownloadPreviewFigmaImageWithOptions] 成功通过 WebSocket 获取图片: temp/xxx/previews/46_1386-1-abc123.png
```

## 🎉 总结

通过添加 WebSocket 兜底功能，系统的鲁棒性得到了大幅提升：

1. ✅ **多层次保障**：本地缓存 → 数据库 → OBS → CDN → WebSocket → 裂图
2. ✅ **智能选择**：根据 `userID` 参数智能决定是否尝试 WebSocket
3. ✅ **异步优化**：OBS 上传不阻塞主流程，提升响应速度
4. ✅ **优雅降级**：任何步骤失败都能继续尝试下一步，最终返回裂图保证服务可用

这使得即使在缓存未命中、OBS 故障、CDN 失效等极端情况下，只要用户有 WebSocket 连接，系统仍能通过 Figma 插件获取图片，大大提高了系统的可用性和用户体验。

