# 重构：禁止直接访问 Figma Images API

## 修改概述

重构 `DownloadPreviewFigmaImageWithOptions` 函数，禁止直接调用 Figma API 下载图片。现在该函数只从**本地缓存**、**数据库**、**OBS 桶**和 **Figma CDN** 获取图片，如果所有途径都失败，则返回裂图（`break.jpg`）。

## 修改原因

1. ✅ **避免重复的 API 调用**：已有统一的渲染接口负责调用 Figma API
2. ✅ **减少速率限制风险**：降低对 Figma API 的直接调用频率
3. ✅ **职责分离**：
   - **渲染接口**：负责主动调用 Figma API，生成图片，存储到数据库/OBS
   - **下载接口**：负责从已有的缓存/数据库/OBS 获取图片
4. ✅ **更好的用户体验**：失败时返回裂图，而不是报错

## 核心修改

### 修改文件
- `internal/services/figma.go`

### 函数签名（保持不变）

```go
func DownloadPreviewFigmaImageWithOptions(
    token, fileKey, nodeID, imageFormat string, 
    imageScale float64, 
    useCache bool
) (string, error)
```

### 新的获取流程

```
开始
  ↓
1️⃣ 检查本地缓存
  ├─→ 找到 → 返回本地路径 ✅
  └─→ 未找到 ↓
  
2️⃣ 查询数据库记录
  ├─→ 未找到 → 返回裂图 🖼️
  └─→ 找到 ↓
  
3️⃣ 尝试从 OBS 下载
  ├─→ 成功 → 保存到本地 → 返回本地路径 ✅
  └─→ 失败 ↓
  
4️⃣ 尝试从 Figma CDN 下载
  ├─→ 成功 → 保存到本地 → 返回本地路径 ✅
  └─→ 失败 ↓
  
5️⃣ 返回裂图 🖼️
```

## 详细实现

### 1. 检查本地缓存

```go
// 1. 检查本地缓存文件
tempDir := filepath.Join("temp", fileKey, "previews")
safeNodeID := strings.NewReplacer(":", "_", ";", "_", ...).Replace(nodeID)

if useCache {
    existingFile := findLatestNodeImage(tempDir, safeNodeID, imageScale)
    if existingFile != "" {
        fmt.Printf("✅ 找到本地缓存图片: %s\n", existingFile)
        return existingFile, nil
    }
}
```

### 2. 查询数据库记录

```go
// 2. 查询数据库中的图片记录
image, err := models.GetNodeImage(fileKey, nodeID, imageFormat, imageScale)
if err != nil || image == nil {
    fmt.Printf("⚠️ 数据库中未找到图片记录\n")
    return getFallbackImage()
}
```

**数据库表**: `figma_node_images`

查询条件:
- `file_key` = `fileKey`
- `node_id` = `nodeID`
- `format` = `imageFormat`
- `scale` = `imageScale`

### 3. 从 OBS 下载（优先）

```go
// 3. 优先使用 OBS Key（如果存在）
if image.OBSKey != "" {
    fmt.Printf("📦 找到 OBS Key: %s\n", image.OBSKey)
    
    localPath, err := downloadImageFromOBS(image.OBSKey, tempDir, safeNodeID, imageFormat, imageScale)
    if err == nil && localPath != "" {
        fmt.Printf("✅ 成功从 OBS 下载图片到本地: %s\n", localPath)
        return localPath, nil
    }
    fmt.Printf("⚠️ 从 OBS 下载失败: %v，尝试 Figma CDN\n", err)
}
```

**注意**: `downloadImageFromOBS` 函数当前标记为 TODO，需要后续实现 OBS SDK 下载逻辑。

### 4. 从 Figma CDN 下载（备选）

```go
// 4. 使用 Figma CDN URL（如果存在）
if image.FigmaCDNURL != "" {
    fmt.Printf("🌐 找到 Figma CDN URL: %s\n", image.FigmaCDNURL)
    
    localPath, err := downloadImageFromURL(image.FigmaCDNURL, tempDir, safeNodeID, imageFormat, imageScale)
    if err == nil && localPath != "" {
        fmt.Printf("✅ 成功从 Figma CDN 下载图片到本地: %s\n", localPath)
        return localPath, nil
    }
    fmt.Printf("⚠️ 从 Figma CDN 下载失败: %v\n", err)
}
```

**`downloadImageFromURL` 实现**:
- 使用标准 HTTP 客户端下载图片
- 计算 MD5 哈希值作为文件名的一部分
- 去重：相同内容的图片不重复保存
- 自动清理过多的旧文件

### 5. 返回裂图

```go
// 5. 都失败了，返回裂图
fmt.Printf("❌ 所有获取途径都失败，返回裂图\n")
return getFallbackImage()
```

```go
// getFallbackImage 获取裂图路径
func getFallbackImage() (string, error) {
    fallbackPath := filepath.Join("web", "static", "img", "break.jpg")
    if _, err := os.Stat(fallbackPath); os.IsNotExist(err) {
        return "", fmt.Errorf("裂图文件不存在: %s", fallbackPath)
    }
    fmt.Printf("🖼️ 返回裂图: %s\n", fallbackPath)
    return fallbackPath, nil
}
```

## 新增辅助函数

### 1. `getFallbackImage()`

获取裂图文件路径。

```go
func getFallbackImage() (string, error)
```

**功能**:
- 返回 `web/static/img/break.jpg` 的路径
- 检查文件是否存在

### 2. `downloadImageFromOBS()`

从 OBS 下载图片到本地缓存（待实现）。

```go
func downloadImageFromOBS(obsKey, tempDir, safeNodeID, imageFormat string, imageScale float64) (string, error)
```

**功能**:
- 根据 `obsKey` 从华为云 OBS 下载图片
- 保存到本地缓存目录
- 返回本地文件路径

**当前状态**: 🚧 TODO（标记为待实现）

### 3. `downloadImageFromURL()`

从指定 URL 下载图片到本地缓存。

```go
func downloadImageFromURL(imageURL, tempDir, safeNodeID, imageFormat string, imageScale float64) (string, error)
```

**功能**:
- 从 Figma CDN 或其他 URL 下载图片
- 计算 MD5 哈希值用于去重
- 保存到本地缓存
- 返回本地文件路径

## 移除的功能

### ❌ 不再直接调用 Figma Images API

**之前的代码**:
```go
// ⚡ 速率限制：基于 Token 的独立速率限制
limiter := globalFigmaImagesAPILimiterManager.getOrCreateLimiter(token)
limiter.Wait()

// 构建API URL
apiUrl := fmt.Sprintf("https://api.figma.com/v1/images/%s?ids=%s&format=%s&scale=%.1f&contentOnly=true",
    fileKey, figmaNodeID, imageFormat, imageScale)

// 发送请求（使用支持代理的客户端）
client := CreateHTTPClientWithToken(token)
resp, err := client.Do(req)
// ... 处理响应
```

**现在**: ✅ 完全移除，不再直接调用 Figma API

## 使用场景对比

### 场景 1：前端预览节点图片

**之前**:
```
用户请求 → 检查本地缓存 → [未找到] → 调用 Figma API → 下载图片 → 返回
```

**现在**:
```
用户请求 → 检查本地缓存 → [未找到] → 查数据库 → [未找到] → 返回裂图 🖼️
```

**优点**:
- 不会因为缺失图片而触发 Figma API 调用
- 返回裂图作为占位符，用户体验更好

### 场景 2：渲染接口已处理的节点

**之前**:
```
用户请求 → 检查本地缓存 → [未找到] → 调用 Figma API → 下载图片 → 返回
```

**现在**:
```
用户请求 → 检查本地缓存 → [未找到] → 查数据库 → [找到] → 从 OBS/CDN 下载 → 返回
```

**优点**:
- 利用数据库和 OBS 缓存，避免重复调用 Figma API
- 速度更快，稳定性更高

### 场景 3：渲染接口尚未处理的节点

**之前**:
```
用户请求 → 检查本地缓存 → [未找到] → 调用 Figma API → 下载图片 → 返回
```

**现在**:
```
用户请求 → 检查本地缓存 → [未找到] → 查数据库 → [未找到] → 返回裂图 🖼️
提示用户: "请先使用渲染功能生成图片"
```

**优点**:
- 明确职责分离：必须先渲染，再查看
- 避免无意义的 API 调用

## 日志输出示例

### 成功从本地缓存获取

```
📖 获取Figma图片: fileKey=xxx, nodeID=50:80, format=png, scale=1.0, useCache=true
缓存目录路径: temp/xxx/previews
原始节点ID: 50:80, 安全节点ID: 50_80
查找缓存图片: tempDir=temp/xxx/previews, safeNodeID=50_80, scale=1.0
✅ 找到本地缓存图片: temp/xxx/previews/50_80-1-abc123.png
```

### 从数据库 → OBS 获取

```
📖 获取Figma图片: fileKey=xxx, nodeID=50:80, format=png, scale=1.0, useCache=true
⚠️ 本地缓存未找到
🔍 查询数据库: fileKey=xxx, nodeID=50:80, format=png, scale=1.0
📦 找到 OBS Key: figma_images/xxx/png_1.0x/50-80.png
✅ 成功从 OBS 下载图片到本地: temp/xxx/previews/50_80-1-def456.png
```

### 从数据库 → Figma CDN 获取

```
📖 获取Figma图片: fileKey=xxx, nodeID=50:80, format=png, scale=1.0, useCache=true
⚠️ 本地缓存未找到
🔍 查询数据库: fileKey=xxx, nodeID=50:80, format=png, scale=1.0
📦 找到 OBS Key: figma_images/xxx/png_1.0x/50-80.png
⚠️ 从 OBS 下载失败: OBS 下载功能尚未实现，尝试 Figma CDN
🌐 找到 Figma CDN URL: https://figma-alpha-api.s3.us-west-2.amazonaws.com/...
开始从 URL 下载图片: https://figma-alpha-api.s3.us-west-2.amazonaws.com/...
图片下载完成: temp/xxx/previews/50_80-1-ghi789.png (12345 字节)
✅ 成功从 Figma CDN 下载图片到本地: temp/xxx/previews/50_80-1-ghi789.png
```

### 所有途径都失败，返回裂图

```
📖 获取Figma图片: fileKey=xxx, nodeID=50:80, format=png, scale=1.0, useCache=true
⚠️ 本地缓存未找到
🔍 查询数据库: fileKey=xxx, nodeID=50:80, format=png, scale=1.0
⚠️ 数据库中未找到图片记录
❌ 所有获取途径都失败，返回裂图
🖼️ 返回裂图: web/static/img/break.jpg
```

## 配合使用的接口

### 统一渲染接口

**路由**: `POST /api/figma/project/:project_id/node-tree/refresh`

**功能**:
1. 调用 Figma API 获取节点树
2. 调用 Figma Images API 渲染图片
3. 将图片上传到 OBS
4. 将记录保存到数据库 `figma_node_images`

### 图片获取接口

**路由**: `GET /api/figma/image`

**参数**:
- `file_key`
- `node_id`
- `format` (可选，默认 png)
- `scale` (可选，默认 1.0)

**功能**:
1. 调用 `DownloadPreviewFigmaImageWithOptions` 获取图片
2. 返回图片文件（如果失败，返回裂图）

## 待实现功能

### 🚧 OBS 下载功能

```go
func downloadImageFromOBS(obsKey, tempDir, safeNodeID, imageFormat string, imageScale float64) (string, error) {
    // TODO: 实现从 OBS 下载图片的逻辑
    // 1. 使用华为云 OBS SDK
    // 2. 根据 obsKey 下载对象
    // 3. 保存到本地缓存
    // 4. 返回本地文件路径
    return "", fmt.Errorf("OBS 下载功能尚未实现")
}
```

**需要**:
- 注入 `OBSService` 实例到 `figma.go`
- 调用 `obsService.DownloadImage(obsKey)` 方法
- 保存文件到 `tempDir` 并返回路径

## 优势总结

| 方面 | 之前 | 现在 |
|------|------|------|
| **职责分离** | ❌ 获取接口也调用 Figma API | ✅ 渲染和获取职责明确 |
| **速率限制风险** | ⚠️ 多个入口都可能触发 API 调用 | ✅ 只有渲染接口调用 API |
| **用户体验** | ❌ 缺失图片时报错 | ✅ 返回裂图作为占位符 |
| **缓存利用率** | ⚠️ 优先本地缓存，次选 API | ✅ 多级缓存：本地 → DB → OBS → CDN |
| **系统稳定性** | ⚠️ Figma API 不稳定影响获取 | ✅ 依赖自有缓存，更稳定 |
| **开发意图** | ⚠️ 用户可能无意触发大量 API | ✅ 必须显式调用渲染接口 |

## 兼容性

### ✅ API 签名保持不变

```go
func DownloadPreviewFigmaImageWithOptions(
    token, fileKey, nodeID, imageFormat string, 
    imageScale float64, 
    useCache bool
) (string, error)
```

### ✅ 返回值保持不变

- 成功：返回本地文件路径
- 失败：返回错误（或裂图路径）

### ⚠️ 行为变化

- **之前**: 可能会调用 Figma API
- **现在**: 永远不会调用 Figma API

**影响**:
- 依赖此函数主动下载新图片的代码需要调整
- 建议：先调用渲染接口，再调用此函数获取

## 测试建议

### 测试 1：本地缓存命中

1. 确保 `temp/fileKey/previews/` 下有缓存文件
2. 调用 `DownloadPreviewFigmaImageWithOptions(..., useCache=true)`
3. 验证返回本地路径，不查询数据库

### 测试 2：数据库 + OBS 流程

1. 清空本地缓存
2. 数据库中存在记录，且有 `obs_key`
3. 调用函数
4. 验证查询数据库 → 从 OBS 下载 → 返回本地路径

### 测试 3：数据库 + Figma CDN 流程

1. 清空本地缓存
2. 数据库中存在记录，有 `figma_cdn_url`，但 `obs_key` 为空或下载失败
3. 调用函数
4. 验证从 Figma CDN 下载 → 返回本地路径

### 测试 4：返回裂图

1. 清空本地缓存
2. 数据库中无记录
3. 调用函数
4. 验证返回 `web/static/img/break.jpg`

### 测试 5：裂图文件缺失

1. 删除 `web/static/img/break.jpg`
2. 触发裂图场景
3. 验证返回错误：`"裂图文件不存在"`

## 相关文档

- [Figma 缓存系统完整方案](FIGMA_CACHE_SYSTEM_COMPLETE.md)
- [Retry-After 处理](FEATURE_RETRY_AFTER_HANDLING.md)
- [OBS 集成](OBS_INTEGRATION.md)

---

**实现时间**: 2025-11-19  
**修改文件**: `internal/services/figma.go`  
**测试状态**: ✅ 编译通过，等待功能测试  
**待完成**: 🚧 实现 `downloadImageFromOBS` 函数

