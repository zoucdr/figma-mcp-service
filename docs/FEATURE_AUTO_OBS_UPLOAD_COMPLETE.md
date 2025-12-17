# 功能完成：本地缓存图片自动上传到 OBS

## 功能概述

当用户访问已缓存的 Figma 图片时，系统会在后台自动检查该图片是否已上传到华为云 OBS。如果未上传，则自动将本地缓存文件上传到 OBS 并更新数据库记录。

## ✅ 已完成的功能

### 1. 后台异步上传

**触发点**: `GetFigmaImage` 控制器返回本地缓存图片后

```go
// 后台检查并上传到 OBS（不阻塞返回）
go checkAndUploadToOBS(project.FileKey, nodeID, format, scale, imagePath)

// 立即返回图片给用户
c.File(imagePath)
```

### 2. 智能去重检查

**数据库预检**: 避免重复上传

```go
// 检查数据库中是否已有 OBS URL
image, err := models.GetNodeImage(fileKey, nodeID, format, scale)
if err == nil && image != nil && image.OBSURL != "" {
    // 已有 OBS URL，跳过上传
    return
}
```

### 3. OBS 上传

**支持多种图片格式**: PNG, JPG, SVG

```go
// 确定 Content-Type
contentType := "image/png"
if format == "jpg" || format == "jpeg" {
    contentType = "image/jpeg"
} else if format == "svg" {
    contentType = "image/svg+xml"
}

// 上传到 OBS
obsURL, obsKey, fileSize, err := globalOBSService.UploadImageFromBytes(
    fileData, fileKey, nodeID, format, scale, contentType,
)
```

### 4. 数据库记录管理

**并发安全处理**: 处理唯一键冲突

```go
err = models.CreateNodeImage(newImage)
if err != nil {
    // 创建失败，可能是并发创建导致的唯一键冲突
    // 尝试重新获取记录并更新
    existingImage, getErr := models.GetNodeImage(fileKey, nodeID, format, scale)
    if getErr == nil && existingImage != nil {
        // 更新现有记录
        existingImage.OBSURL = obsURL
        existingImage.OBSKey = obsKey
        existingImage.FileSize = uint64(fileSize)
        models.UpdateNodeImage(existingImage)
    }
}
```

### 5. 全局 API 冷却检查优化

**修复**: Figma API 的速率限制是全局的，不区分文件 API 和图片 API

```go
// 取两个时间戳中较大的那个
lastRequestTime := cooldown.LastFileRequestTime
if cooldown.LastImageRequestTime > lastRequestTime {
    lastRequestTime = cooldown.LastImageRequestTime
}

elapsed := now - lastRequestTime
if elapsed >= cooldownSeconds {
    return true, 0 // 可以请求
}
```

## 🔧 技术细节

### OBS 配置

**配置文件**: `configs/config.yaml`

```yaml
huawei_cloud:
  obs:
    enabled: true
    endpoint: "test-figma.obs.cn-north-4.myhuaweicloud.com"  # 可使用自定义域名
    access_key_id: "YOUR_ACCESS_KEY"
    secret_access_key: "YOUR_SECRET_KEY"
    bucket_name: "figma-deliver-cache"
    region: "cn-north-4"
    path_prefix: "figma_images/"
    upload_concurrency: 5
    upload_timeout_seconds: 300
```

### OBS 存储路径

**格式**: `figma_images/{fileKey}/png_1.0x/{nodeID}.png`

**示例**:
```
figma_images/oNOE3NSRYt023cdpXVHs4y/png_1.0x/50-80.png
figma_images/abc123/jpg_2.0x/1-123.jpg
figma_images/def456/svg_1.0x/2-789.svg
```

### 数据库表结构

**表名**: `figma_node_images`

**关键字段**:
- `file_key`: Figma 文件 Key
- `node_id`: 节点 ID (如 `50:80`)
- `format`: 图片格式 (`png`, `jpg`, `svg`)
- `scale`: 缩放比例 (如 `1.0`, `2.0`)
- `obs_url`: OBS 访问 URL
- `obs_key`: OBS 对象 Key
- `file_size`: 文件大小（字节）

**唯一索引**: `idx_node_format_scale` (`file_key`, `node_id`, `format`, `scale`)

## 📊 工作流程

```
用户请求图片
    ↓
GetFigmaImage 控制器
    ↓
查找本地缓存 (temp/...)
    ↓
【立即返回图片】← 不阻塞，用户无感知
    ↓
【后台 goroutine】
    ↓
检查数据库是否有 OBS URL？
    ├─→ 有 → 跳过上传（日志：📦 OBS URL 已存在）
    ↓
    └─→ 无 → 继续上传流程
        ↓
        检查本地文件是否存在？
        ├─→ 否 → 退出（日志：❌ 本地文件不存在）
        ↓
        └─→ 是 → 读取文件数据
            ↓
            检查 OBS 服务是否启用？
            ├─→ 否 → 退出（日志：⚠️ OBS 服务未启用）
            ↓
            └─→ 是 → 上传到 OBS
                ↓
                上传成功？
                ├─→ 否 → 退出（日志：❌ 上传到 OBS 失败）
                ↓
                └─→ 是 → 更新数据库
                    ↓
                    数据库记录存在？
                    ├─→ 是 → 更新记录
                    │         ↓
                    │         日志：💾 已更新数据库
                    ↓
                    └─→ 否 → 创建记录
                              ↓
                              创建成功？
                              ├─→ 是 → 日志：💾 已创建数据库记录
                              ↓
                              └─→ 否（唯一键冲突）
                                    ↓
                                    重新获取记录并更新
                                    ↓
                                    日志：💾 重试更新数据库成功
```

## 🔍 日志示例

### 成功场景

```
Controller: 成功获取Figma图片, path=temp/oNOE3NSRYt023cdpXVHs4y/previews/50_80-1_0x-12345.png
Controller: 返回图片文件: temp/oNOE3NSRYt023cdpXVHs4y/previews/50_80-1_0x-12345.png

✅ 成功上传到 OBS: temp/oNOE3NSRYt023cdpXVHs4y/previews/50_80-1_0x-12345.png 
   → https://obs.cn-north-4.myhuaweicloud.com/figma-deliver-cache/figma_images/...
💾 已创建数据库记录: nodeID=50:80, OBSURL=https://...
```

### 跳过上传（已存在）

```
Controller: 成功获取Figma图片, path=temp/oNOE3NSRYt023cdpXVHs4y/previews/50_80-1_0x-12345.png
Controller: 返回图片文件: temp/oNOE3NSRYt023cdpXVHs4y/previews/50_80-1_0x-12345.png

📦 OBS URL 已存在，跳过上传: fileKey=oNOE3NSRYt023cdpXVHs4y, nodeID=50:80
```

### 并发冲突处理

```
⚠️ 创建数据库记录失败（可能已存在）: Error 1062: Duplicate entry '...' for key 'idx_node_format_scale'
💾 重试更新数据库成功: nodeID=50:80, OBSURL=https://...
```

### OBS 服务未启用

```
⚠️ OBS 服务未启用，跳过上传
```

## 🎯 与调度器渲染队列的区别

| 特性              | 本地缓存自动上传                | 调度器渲染队列                   |
|-------------------|--------------------------------|--------------------------------|
| **触发时机**      | 用户访问本地缓存图片时          | 手动创建渲染队列或定时任务       |
| **数据来源**      | 已存在的本地缓存文件            | 通过 Figma API 获取新图片        |
| **执行方式**      | 后台 goroutine，异步执行        | 定时任务，批量处理               |
| **速率限制检查**  | 不需要（已有缓存）              | 需要检查 Figma API 冷却          |
| **优先级**        | 被动触发，低优先级               | 主动处理，有队列管理             |
| **适用场景**      | 补充备份历史缓存                | 主动渲染新图片或更新图片         |

## ⚠️ 注意事项

### 1. 并发安全

多个用户同时访问同一图片可能触发多次上传：
- OBS 会覆盖同名文件（最终一致）
- 数据库有唯一键保护
- 通过重试机制处理冲突

### 2. OBS 费用

每次上传和存储都会产生费用：
- 上传流量费用
- 存储空间费用
- 下载流量费用

建议：
- 配置合理的过期策略（当前：30天）
- 监控 OBS 使用量

### 3. 自定义域名配置

如果使用自定义域名（如 `test-figma.obs.cn-north-4.myhuaweicloud.com`）：
- 需要在华为云 OBS 控制台绑定域名
- 域名需要正确解析到 OBS 服务
- 签名计算基于自定义域名

### 4. 错误处理

所有错误都是静默处理：
- 不影响用户获取图片
- 仅记录日志
- 下次访问会重试

### 5. OBS 上传失败

如果 OBS 上传失败：
- 用户仍能正常获取本地缓存
- 日志会记录错误
- 下次访问会重新尝试上传

## 🧪 测试验证

### 测试步骤

1. **启动服务**:
   ```bash
   ./figma-deliver.exe
   ```

2. **访问图片接口**:
   ```http
   GET /api/figma/project/8/image/50:80?format=png&scale=1.0
   ```

3. **观察日志**:
   ```
   ✅ 成功上传到 OBS
   💾 已创建数据库记录
   ```

4. **验证数据库**:
   ```sql
   SELECT file_key, node_id, format, scale, obs_url, file_size 
   FROM figma_node_images 
   WHERE file_key = 'oNOE3NSRYt023cdpXVHs4y' 
     AND node_id = '50:80';
   ```

5. **再次访问（应跳过上传）**:
   ```
   📦 OBS URL 已存在，跳过上传
   ```

6. **验证 OBS 文件**:
   - 登录华为云 OBS 控制台
   - 检查桶：`figma-deliver-cache`
   - 查看路径：`figma_images/oNOE3NSRYt023cdpXVHs4y/png_1.0x/50-80.png`

## 📝 相关代码文件

### 核心实现

- **`internal/controllers/figma.go`**
  - `checkAndUploadToOBS()`: 后台上传逻辑
  - `GetFigmaImage()`: 触发上传的入口

- **`internal/services/obs.go`**
  - `UploadImageFromBytes()`: OBS 上传实现
  - `uploadToOBS()`: 底层上传方法

- **`internal/models/cache.go`**
  - `FigmaNodeImage`: 数据库模型
  - `GetNodeImage()`: 查询记录
  - `CreateNodeImage()`: 创建记录
  - `UpdateNodeImage()`: 更新记录

### 配置文件

- **`configs/config.yaml`**
  - OBS 配置
  - 缓存配置

### 主程序

- **`main.go`**
  - 初始化 OBS 服务
  - 设置全局 OBS 实例

## 🔄 全局冷却检查优化

### 问题

之前的实现中，文件 API 和图片 API 的冷却检查是独立的，但 Figma API 的速率限制是全局的。

### 解决方案

修改 `CheckFileAPICooldown` 和 `CheckImageAPICooldown`，取两个时间戳中较大的值：

```go
// Figma API 速率限制是全局的，需要检查两个时间戳中较大的那个
lastRequestTime := cooldown.LastFileRequestTime
if cooldown.LastImageRequestTime > lastRequestTime {
    lastRequestTime = cooldown.LastImageRequestTime
}

elapsed := now - lastRequestTime
if elapsed >= cooldownSeconds {
    return true, 0 // 可以请求
}
```

### 效果

- ✅ 准确遵守 Figma 的全局速率限制
- ✅ 减少 429 错误
- ✅ 提升系统稳定性

## 📚 相关文档

- [OBS 集成指南](OBS_INTEGRATION_GUIDE.md)
- [调度器实现](SCHEDULER_IMPLEMENTATION.md)
- [全局 API 冷却修复](BUGFIX_GLOBAL_API_COOLDOWN.md)
- [Figma 缓存系统完整方案](FIGMA_CACHE_SYSTEM_COMPLETE.md)

---

## ✅ 功能验收清单

- [x] 后台异步上传（不阻塞图片返回）
- [x] 数据库预检（避免重复上传）
- [x] OBS 上传成功
- [x] 数据库记录创建/更新
- [x] 并发冲突处理（唯一键冲突重试）
- [x] 全局 API 冷却检查修复
- [x] 静默错误处理（不影响主流程）
- [x] 日志记录完整
- [x] 编译通过
- [x] OBS 配置验证通过

---

**实现时间**: 2025-11-19  
**修改文件**:
- `internal/controllers/figma.go`
- `internal/models/cache.go`
- `configs/config.yaml`
- `main.go`

**测试状态**: ✅ 基础测试通过，等待生产环境验证

