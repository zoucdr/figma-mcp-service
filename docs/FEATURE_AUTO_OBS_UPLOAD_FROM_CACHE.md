# 功能：本地缓存图片自动上传到OBS

## 概述

当系统返回本地缓存的Figma图片时，会在后台自动检查该图片是否已上传到华为云OBS。如果未上传，则自动将本地缓存文件上传到OBS并更新数据库记录。

## 功能目标

1. ✅ **自动备份**：将本地缓存的图片自动备份到OBS云存储
2. ✅ **无缝体验**：后台异步处理，不阻塞图片返回
3. ✅ **去重优化**：检查数据库，避免重复上传
4. ✅ **持久化存储**：OBS提供更可靠的长期存储

## 工作流程

```
用户请求图片
    ↓
GetFigmaImage 控制器
    ↓
获取本地缓存图片（来自 temp/ 目录）
    ↓
【立即返回图片给用户】← 不阻塞
    ↓
【后台 goroutine】
    ↓
检查数据库是否已有 OBS URL？
    ├─→ 是 → 跳过上传
    ↓
    └─→ 否 → 继续
        ↓
        读取本地文件
        ↓
        上传到 OBS
        ↓
        更新/创建数据库记录
        ↓
        完成（日志记录）
```

## 代码实现

### 1. 控制器触发（`GetFigmaImage`）

**文件**: `internal/controllers/figma.go`

```go
fmt.Printf("Controller: 成功获取Figma图片, path=%s\n", imagePath)

// 后台检查并上传到 OBS（不阻塞返回）
go checkAndUploadToOBS(project.FileKey, nodeID, format, scale, imagePath)

// 返回图片（立即返回，不等待上传）
c.File(imagePath)
```

### 2. 后台上传逻辑（`checkAndUploadToOBS`）

**文件**: `internal/controllers/figma.go`

```go
func checkAndUploadToOBS(fileKey, nodeID, format string, scale float64, localFilePath string) {
    // 1. 检查数据库中是否已有 OBS URL
    image, err := models.GetNodeImage(fileKey, nodeID, format, scale)
    if err == nil && image != nil && image.OBSURL != "" {
        // 已有 OBS URL，跳过上传
        fmt.Printf("📦 OBS URL 已存在，跳过上传\n")
        return
    }

    // 2. 检查本地文件是否存在
    _, err = os.Stat(localFilePath)
    if err != nil {
        fmt.Printf("❌ 本地文件不存在，无法上传\n")
        return
    }

    // 3. 读取本地文件
    fileData, err := os.ReadFile(localFilePath)
    if err != nil {
        fmt.Printf("❌ 读取本地文件失败\n")
        return
    }

    // 4. 检查 OBS 服务是否启用
    if globalOBSService == nil || !globalOBSService.IsEnabled() {
        fmt.Printf("⚠️ OBS 服务未启用，跳过上传\n")
        return
    }

    // 5. 上传到 OBS
    contentType := determineContentType(format)
    obsURL, obsKey, fileSize, err := globalOBSService.UploadImageFromBytes(
        fileData, fileKey, nodeID, format, scale, contentType,
    )
    if err != nil {
        fmt.Printf("❌ 上传到 OBS 失败: %v\n", err)
        return
    }

    fmt.Printf("✅ 成功上传到 OBS: %s\n", obsURL)

    // 6. 更新或创建数据库记录
    if image != nil {
        // 更新现有记录
        image.OBSURL = obsURL
        image.OBSKey = obsKey
        image.FileSize = uint64(fileSize)
        models.UpdateNodeImage(image)
    } else {
        // 创建新记录
        newImage := &models.FigmaNodeImage{
            FileKey:     fileKey,
            NodeID:      nodeID,
            Format:      format,
            Scale:       scale,
            OBSURL:      obsURL,
            OBSKey:      obsKey,
            FileSize:    uint64(fileSize),
        }
        models.CreateNodeImage(newImage)
    }
}
```

### 3. 全局 OBS 服务设置（`main.go`）

```go
// 初始化OBS服务
obsService, err := services.NewOBSService(...)

// 设置全局 OBS 服务实例（供图片控制器使用）
controllers.SetGlobalOBSService(obsService)
```

## 触发场景

### 场景 1：用户访问项目图片

```http
GET /api/figma/project/8/image/1:123?format=png&scale=2.0
```

**流程**:
1. 检查本地缓存 → 找到 `temp/abc123/previews/1_123-2_0x-*.png`
2. 立即返回本地文件
3. **后台触发**: 检查并上传到 OBS

### 场景 2：本地缓存已存在但未上传

```
本地文件: temp/abc123/previews/1_123-2_0x-12345.png
数据库记录: 无 (或 obs_url 为空)

→ 自动上传到 OBS
→ 创建或更新数据库记录
```

### 场景 3：已上传到 OBS

```
本地文件: temp/abc123/previews/1_123-2_0x-12345.png
数据库记录: obs_url = "https://figma-deliver-cache.obs.cn-north-4.myhuaweicloud.com/..."

→ 检测到已上传，跳过
```

## 数据库记录示例

### 初始状态（仅本地缓存）

```sql
SELECT * FROM figma_node_images 
WHERE file_key = 'abc123' AND node_id = '1:123';

-- 可能返回空，或返回：
file_key     | node_id | format | scale | obs_url | obs_key
-------------|---------|--------|-------|---------|--------
abc123       | 1:123   | png    | 2.0   | (空)    | (空)
```

### 上传后状态

```sql
SELECT * FROM figma_node_images 
WHERE file_key = 'abc123' AND node_id = '1:123';

file_key | node_id | format | scale | obs_url                                    | obs_key
---------|---------|--------|-------|--------------------------------------------|---------------------------
abc123   | 1:123   | png    | 2.0   | https://figma-deliver-cache.obs...          | figma_images/abc123/...
```

## 错误处理

### 1. 本地文件不存在

```go
_, err = os.Stat(localFilePath)
if err != nil {
    // 文件已被删除或路径错误
    fmt.Printf("❌ 本地文件不存在，无法上传: %s\n", localFilePath)
    return  // 静默失败，不影响用户
}
```

### 2. OBS 服务未启用

```go
if globalOBSService == nil || !globalOBSService.IsEnabled() {
    fmt.Printf("⚠️ OBS 服务未启用，跳过上传\n")
    return  // 静默跳过
}
```

### 3. 上传失败

```go
obsURL, obsKey, fileSize, err := globalOBSService.UploadImageFromBytes(...)
if err != nil {
    fmt.Printf("❌ 上传到 OBS 失败: %v\n", err)
    return  // 仅记录日志，不影响用户
}
```

### 4. 数据库更新失败

```go
err = models.UpdateNodeImage(image)
if err != nil {
    fmt.Printf("❌ 更新数据库记录失败: %v\n", err)
    // OBS 已上传成功，但数据库未更新
    // 下次请求会重新触发上传（OBS SDK 会覆盖同名文件）
}
```

## 日志输出示例

### 成功上传

```
Controller: 成功获取Figma图片, path=temp/abc123/previews/1_123-2_0x-12345.png
Controller: 返回图片文件: temp/abc123/previews/1_123-2_0x-12345.png
✅ 成功上传到 OBS: temp/abc123/previews/1_123-2_0x-12345.png → https://figma-deliver-cache.obs...
💾 已创建数据库记录: nodeID=1:123, OBSURL=https://...
```

### 跳过上传（已存在）

```
Controller: 成功获取Figma图片, path=temp/abc123/previews/1_123-2_0x-12345.png
Controller: 返回图片文件: temp/abc123/previews/1_123-2_0x-12345.png
📦 OBS URL 已存在，跳过上传: fileKey=abc123, nodeID=1:123
```

### OBS 服务未启用

```
Controller: 成功获取Figma图片, path=temp/abc123/previews/1_123-2_0x-12345.png
Controller: 返回图片文件: temp/abc123/previews/1_123-2_0x-12345.png
⚠️ OBS 服务未启用，跳过上传
```

## 性能优化

### 1. 异步处理

```go
// 使用 goroutine，不阻塞图片返回
go checkAndUploadToOBS(project.FileKey, nodeID, format, scale, imagePath)

// 立即返回图片
c.File(imagePath)
```

**优势**:
- 用户体验无感知
- 图片返回速度不受OBS上传影响

### 2. 数据库预检

```go
// 先检查数据库，避免无效上传
image, err := models.GetNodeImage(fileKey, nodeID, format, scale)
if err == nil && image != nil && image.OBSURL != "" {
    return  // 已存在，跳过
}
```

**优势**:
- 减少重复上传
- 节省OBS流量和费用

### 3. 静默失败

所有错误都仅记录日志，不影响主流程：

```go
if err != nil {
    fmt.Printf("❌ 错误: %v\n", err)
    return  // 不抛出异常
}
```

**优势**:
- 即使OBS故障，用户仍能获取本地缓存图片
- 系统更健壮

## OBS 存储路径规则

### 路径生成逻辑

```go
// OBSService.generateOBSKey()
obsKey := fmt.Sprintf("%s%s/%s_%s_%.1fx.%s", 
    s.pathPrefix,     // "figma_images/"
    fileKey,           // "abc123"
    fileKey,           // "abc123"
    nodeID,            // "1:123"
    scale,             // 2.0
    format,            // "png"
)
// 结果: "figma_images/abc123/abc123_1:123_2.0x.png"
```

### 路径示例

```
figma_images/abc123/abc123_1:123_2.0x.png
figma_images/abc123/abc123_1:124_1.0x.jpg
figma_images/def456/def456_2:789_3.0x.svg
```

## 与调度器渲染队列的区别

| 功能          | 本地缓存自动上传               | 调度器渲染队列                |
|---------------|-------------------------------|-------------------------------|
| **触发时机**  | 用户访问本地缓存图片时         | 手动创建渲染队列或定时任务     |
| **来源**      | 已存在的本地缓存文件           | 通过 Figma API 获取新图片      |
| **优先级**    | 被动触发，无优先级              | 主动处理，有队列管理           |
| **数据库字段**| `figma_node_images` 表         | `figma_render_queues` 表       |
| **冷却检查**  | 无（已有缓存）                 | 需要检查 API 冷却              |
| **适用场景**  | 补充备份已有缓存               | 主动渲染新图片或更新图片       |

### 协同工作

```
场景1：用户访问已缓存图片
→ 返回本地缓存
→ 本地缓存自动上传功能触发（本功能）

场景2：用户请求渲染新图片
→ 创建渲染队列
→ 调度器处理（调用 Figma API）
→ 上传到 OBS（调度器内置）
→ 保存到数据库

场景3：本地缓存过期被清理
→ 用户下次访问
→ 创建渲染队列（调度器处理）
→ 可从 OBS 恢复（如果之前已上传）
```

## 测试验证

### 测试步骤

1. **确保有本地缓存**:
   ```bash
   ls temp/abc123/previews/
   # 输出: 1_123-2_0x-12345.png
   ```

2. **访问图片接口**:
   ```http
   GET /api/figma/project/8/image/1:123?format=png&scale=2.0
   ```

3. **观察日志**:
   ```
   ✅ 成功上传到 OBS: temp/.../*.png → https://...
   💾 已创建数据库记录: nodeID=1:123
   ```

4. **验证数据库**:
   ```sql
   SELECT obs_url, obs_key FROM figma_node_images 
   WHERE file_key = 'abc123' AND node_id = '1:123';
   ```

5. **再次访问（应跳过上传）**:
   ```
   📦 OBS URL 已存在，跳过上传
   ```

### 验证 OBS 文件

```bash
# 通过 OBS 控制台或 SDK 检查
# 路径: figma_images/abc123/abc123_1:123_2.0x.png
```

## 配置要求

### 1. OBS 服务必须启用

```yaml
# config.yaml
huawei_cloud:
  obs:
    enabled: true  # ⚠️ 必须为 true
    endpoint: "obs.cn-north-4.myhuaweicloud.com"
    access_key_id: "YOUR_ACCESS_KEY"
    secret_access_key: "YOUR_SECRET_KEY"
    bucket_name: "figma-deliver-cache"
```

### 2. 数据库表已迁移

确保 `figma_node_images` 表存在并包含以下字段：
- `file_key`
- `node_id`
- `format`
- `scale`
- `obs_url`
- `obs_key`
- `file_size`

## 优势总结

✅ **自动化**：无需手动干预，自动备份到云端  
✅ **非阻塞**：后台异步处理，不影响响应速度  
✅ **去重优化**：检查数据库，避免重复上传  
✅ **容错性强**：静默失败，不影响主流程  
✅ **持久化**：OBS 提供可靠的长期存储  
✅ **可恢复**：本地缓存清理后可从 OBS 恢复  

## 注意事项

1. **OBS 费用**：每次上传和存储都会产生费用，建议配置合理的过期策略
2. **并发控制**：多个用户同时访问同一图片时，可能触发多次上传（OBS会覆盖）
3. **文件大小**：大文件上传可能较慢，但不会阻塞响应
4. **数据一致性**：OBS上传成功但数据库更新失败时，下次会重新上传

---

**实现时间**: 2025-11-19  
**修改文件**:
- `internal/controllers/figma.go` (添加 `checkAndUploadToOBS` 函数)
- `main.go` (设置全局 OBS 服务实例)

**相关文档**:
- [OBS集成指南](OBS_INTEGRATION_GUIDE.md)
- [调度器实现](SCHEDULER_IMPLEMENTATION.md)
- [Figma缓存系统完整方案](FIGMA_CACHE_SYSTEM_COMPLETE.md)

