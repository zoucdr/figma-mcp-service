# 修复：UpdateNodeImage 只更新指定字段

## 问题描述

在 `UpdateNodeImage` 函数中使用 `DB.Save()` 时，GORM 会尝试更新所有字段，包括零值字段。这导致在更新 OBS 信息时，可能会覆盖数据库中已有的非零值数据。

### 问题表现

```go
// 在 UpdateNodeImageOBS 中创建部分字段的对象
image := &models.FigmaNodeImage{
    ID:           imageID,
    OBSKey:       obsKey,
    OBSExpiresAt: expiresAt,
    FileSize:     fileSize,
    Width:        width,        // 可能为 0
    Height:       height,       // 可能为 0
    Status:       "obs_synced",
    UpdatedAt:    now,
    // 其他字段未设置，为零值
}

// 使用 DB.Save() 会将所有未设置的字段更新为零值
err := DB.Save(image).Error
```

**结果**: 数据库中的 `file_key`、`node_id`、`format`、`scale`、`figma_cdn_url` 等字段可能被清空。

## 解决方案

### 1. 修改 `UpdateNodeImage` 使用 `Updates`

将 `DB.Save()` 改为 `DB.Updates()`，只更新非零值字段。

**文件**: `internal/models/cache.go`

```go
func UpdateNodeImage(image *FigmaNodeImage) error {
	// 使用 Updates 方法，只更新非零值字段
	// 但需要特别处理 UpdatedAt，确保它总是被更新
	return DB.Model(&FigmaNodeImage{}).
		Where("id = ?", image.ID).
		Updates(image).Error
}
```

**优点**:
- 只更新非零值字段
- 保留原有数据

**缺点**:
- 无法将字段更新为零值（如果需要）

### 2. 新增 `UpdateNodeImageOBSInfo` 方法

创建专门的方法，使用 `map[string]interface{}` 精确指定要更新的字段。

**文件**: `internal/models/cache.go`

```go
// UpdateNodeImageOBSInfo 更新节点图片的OBS信息（只更新指定字段）
func UpdateNodeImageOBSInfo(id uint, obsKey string, obsExpiresAt uint32, fileSize uint64, status string) error {
	updates := map[string]interface{}{
		"obs_key":        obsKey,
		"obs_expires_at": obsExpiresAt,
		"file_size":      fileSize,
		"status":         status,
		"updated_at":     uint32(time.Now().Unix()),
	}
	
	return DB.Model(&FigmaNodeImage{}).
		Where("id = ?", id).
		Updates(updates).Error
}
```

**优点**:
- 精确控制更新的字段
- 可以更新零值字段（如果需要）
- 避免误覆盖其他字段

### 3. 修改 `UpdateNodeImageOBS` 使用新方法

**文件**: `internal/services/cache.go`

```go
// UpdateNodeImageOBS 更新节点图片的OBS信息
func (cs *CacheService) UpdateNodeImageOBS(imageID uint, obsKey string, fileSize uint64, width, height uint) error {
	now := uint32(time.Now().Unix())
	expiresAt := now + (cs.OBSExpiresDays * 24 * 3600)

	// 使用专门的方法只更新 OBS 相关字段
	err := models.UpdateNodeImageOBSInfo(imageID, obsKey, expiresAt, fileSize, "obs_synced")
	if err != nil {
		log.Printf("❌ [NodeImage] 更新OBS信息失败 id=%d: %v", imageID, err)
		return err
	}

	log.Printf("✅ [NodeImage] 已更新OBS信息 id=%d, obsKey=%s, expiresAt=%d, size=%d bytes",
		imageID, obsKey, expiresAt, fileSize)

	return nil
}
```

## 修改对比

### 之前（有问题）

```go
// cache.go
image := &models.FigmaNodeImage{
    ID:           imageID,
    OBSKey:       obsKey,
    OBSExpiresAt: expiresAt,
    FileSize:     fileSize,
    Width:        width,
    Height:       height,
    Status:       "obs_synced",
    UpdatedAt:    now,
    // file_key, node_id, format, scale 等字段未设置（零值）
}
err := models.UpdateNodeImage(image)

// cache.go (UpdateNodeImage)
func UpdateNodeImage(image *FigmaNodeImage) error {
    return DB.Save(image).Error  // 会更新所有字段，包括零值
}
```

**问题**: `DB.Save()` 会将 `file_key`、`node_id` 等未设置的字段更新为空字符串。

### 之后（修复）

```go
// cache.go
err := models.UpdateNodeImageOBSInfo(imageID, obsKey, expiresAt, fileSize, "obs_synced")

// cache.go (UpdateNodeImageOBSInfo)
func UpdateNodeImageOBSInfo(id uint, obsKey string, obsExpiresAt uint32, fileSize uint64, status string) error {
    updates := map[string]interface{}{
        "obs_key":        obsKey,
        "obs_expires_at": obsExpiresAt,
        "file_size":      fileSize,
        "status":         status,
        "updated_at":     uint32(time.Now().Unix()),
    }
    
    return DB.Model(&FigmaNodeImage{}).
        Where("id = ?", id).
        Updates(updates).Error
}
```

**优点**: 只更新指定的 5 个字段，不影响其他字段。

## SQL 对比

### 之前（Save）

```sql
UPDATE figma_node_images 
SET 
    file_key = '',           -- 被清空！
    node_id = '',            -- 被清空！
    format = '',             -- 被清空！
    scale = 0,               -- 被清空！
    figma_cdn_url = '',      -- 被清空！
    obs_key = 'figma_images/xxx/png_1.0x/50-80.png',
    obs_expires_at = 1734567890,
    file_size = 12345,
    width = 0,
    height = 0,
    status = 'obs_synced',
    updated_at = 1702967890
WHERE id = 34;
```

### 之后（Updates with map）

```sql
UPDATE figma_node_images 
SET 
    obs_key = 'figma_images/xxx/png_1.0x/50-80.png',
    obs_expires_at = 1734567890,
    file_size = 12345,
    status = 'obs_synced',
    updated_at = 1702967890
WHERE id = 34;
```

**注意**: 只更新 5 个字段，`file_key`、`node_id`、`format`、`scale`、`figma_cdn_url` 等字段保持不变。

## 其他改进

### 空 URL 验证

在 scheduler 中添加了对空 URL 的验证：

**文件**: `internal/services/scheduler.go`

```go
for nodeID, figmaURL := range imageURLs {
    // 验证 URL 是否有效
    if figmaURL == "" || figmaURL == "null" {
        log.Printf("⚠️ [队列 #%d] 节点 %s 的图片 URL 为空，跳过", queue.ID, nodeID)
        failedCount++
        // 更新进度
        err = s.queueService.UpdateQueueProgress(queue.ID, uint(successCount), uint(failedCount))
        continue
    }
    
    // ... 继续处理
}
```

**原因**: Figma API 可能返回某些节点的 URL 为 `null` 或空字符串，需要跳过这些节点。

### 改进的状态管理

```go
if err != nil {
    log.Printf("⚠️ [队列 #%d] 上传 OBS 失败 (%s): %v", queue.ID, nodeID, err)
    nodeImage.Status = "figma_cdn"
    nodeImage.ErrorMessage = fmt.Sprintf("OBS上传失败: %v", err)
} else {
    err = s.cacheService.UpdateNodeImageOBS(nodeImage.ID, obsKey, uint64(fileSize), 0, 0)
    if err != nil {
        nodeImage.Status = "figma_cdn"
    } else {
        nodeImage.Status = "obs_synced"  // 成功后由 UpdateNodeImageOBS 设置
        log.Printf("✅ [队列 #%d] 节点 %s 已上传到OBS: %s", queue.ID, nodeID, obsKey)
    }
}
```

## 数据库字段说明

### figma_node_images 表结构

| 字段 | 类型 | 说明 | 更新时机 |
|------|------|------|----------|
| `id` | uint | 主键 | 创建时 |
| `file_key` | string | Figma 文件 key | 创建时 |
| `node_id` | string | 节点 ID | 创建时 |
| `format` | string | 图片格式（png/jpg/svg） | 创建时 |
| `scale` | float64 | 缩放比例 | 创建时 |
| `figma_cdn_url` | string | Figma CDN URL | 创建时/更新时 |
| `obs_key` | string | OBS 对象 Key | **OBS 上传后** |
| `obs_expires_at` | uint32 | OBS 过期时间 | **OBS 上传后** |
| `file_size` | uint64 | 文件大小 | **OBS 上传后** |
| `width` | uint | 图片宽度 | OBS 上传后（可选） |
| `height` | uint | 图片高度 | OBS 上传后（可选） |
| `status` | string | 状态 | 各阶段更新 |
| `error_message` | string | 错误信息 | 失败时 |
| `hit_count` | uint | 命中次数 | 每次访问 |
| `last_accessed_at` | uint32 | 最后访问时间 | 每次访问 |
| `created_at` | uint32 | 创建时间 | 创建时 |
| `updated_at` | uint32 | 更新时间 | 每次更新 |

### 状态值说明

| 状态 | 说明 | OBS Key | Figma CDN URL |
|------|------|---------|---------------|
| `pending` | 等待处理 | ❌ 无 | ✅ 有 |
| `figma_cdn` | 只有 Figma CDN | ❌ 无 | ✅ 有 |
| `obs_synced` | 已同步到 OBS | ✅ 有 | ✅ 有 |
| `error` | 处理失败 | ❌ 无 | ❌ 无 |

## 测试验证

### 测试 1：创建记录

```go
// 创建新记录
nodeImage, err := s.cacheService.CreateNodeImage(
    "oNOE3NSRYt023cdpXVHs4y",
    "50:80",
    "png",
    1.0,
    "https://figma-alpha-api.s3.us-west-2.amazonaws.com/...",
)
```

**预期数据库记录**:
```json
{
  "id": 34,
  "file_key": "oNOE3NSRYt023cdpXVHs4y",
  "node_id": "50:80",
  "format": "png",
  "scale": 1.0,
  "figma_cdn_url": "https://figma-alpha-api.s3.us-west-2.amazonaws.com/...",
  "obs_key": "",
  "obs_expires_at": 0,
  "file_size": 0,
  "status": "pending",
  "created_at": 1702967890,
  "updated_at": 1702967890
}
```

### 测试 2：更新 OBS 信息

```go
// 上传到 OBS 后更新
err := s.cacheService.UpdateNodeImageOBS(
    34,
    "figma_images/xxx/png_1.0x/50-80.png",
    12345,
    0,
    0,
)
```

**预期数据库记录**（只更新 OBS 相关字段）:
```json
{
  "id": 34,
  "file_key": "oNOE3NSRYt023cdpXVHs4y",  // 保持不变 ✅
  "node_id": "50:80",                      // 保持不变 ✅
  "format": "png",                         // 保持不变 ✅
  "scale": 1.0,                            // 保持不变 ✅
  "figma_cdn_url": "https://...",          // 保持不变 ✅
  "obs_key": "figma_images/xxx/png_1.0x/50-80.png",  // 已更新 ✅
  "obs_expires_at": 1734567890,            // 已更新 ✅
  "file_size": 12345,                      // 已更新 ✅
  "status": "obs_synced",                  // 已更新 ✅
  "updated_at": 1702967900                 // 已更新 ✅
}
```

## 相关文档

- [Figma 缓存系统完整方案](FIGMA_CACHE_SYSTEM_COMPLETE.md)
- [OBS 集成](OBS_INTEGRATION.md)

---

**修复时间**: 2025-11-19  
**修改文件**:
- `internal/models/cache.go` - 修改 `UpdateNodeImage`，新增 `UpdateNodeImageOBSInfo`
- `internal/services/cache.go` - 修改 `UpdateNodeImageOBS` 使用新方法
- `internal/services/scheduler.go` - 添加空 URL 验证和改进错误处理

**测试状态**: ✅ 编译通过，等待实际运行验证

