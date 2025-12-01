# 批量图片上传华为云OBS功能

## 功能概述

在批量导出Figma节点图片后，自动将所有图片上传到华为云对象存储（OBS），并返回访问URL和存储Key。

## 实现原理

### 工作流程

```
1. 用户调用 export_nodes_as_images
   ↓
2. Figma插件批量导出图片（base64格式）
   ↓
3. 服务端接收图片数据
   ↓
4. 自动解码base64并上传到华为云OBS
   ↓
5. 保存图片信息到数据库缓存
   ↓
6. 返回包含OBS URL的完整结果
```

### 关键实现

#### 1. 自动触发上传 (`mcp_tools.go`)

在 `HandleFigmaPluginToolCall` 函数中，当检测到 `export_nodes_as_images` 命令时，自动调用 `handleBatchImageUpload` 函数：

```go
// 特殊处理 export_nodes_as_images：上传到华为云OBS
if toolName == "export_nodes_as_images" {
    result = handleBatchImageUpload(result, arguments)
}
```

#### 2. 批量上传处理 (`handleBatchImageUpload`)

**功能特点：**
- 检查OBS服务是否启用
- 批量解码base64图片数据
- 并发上传到华为云OBS
- 收集上传结果（成功/失败）
- 异步保存到数据库缓存

**关键代码逻辑：**

```go
// 1. 检查OBS服务
if globalOBSService == nil || !globalOBSService.IsEnabled() {
    resultMap["obs_enabled"] = false
    resultMap["obs_message"] = "OBS服务未启用，图片未上传到云端"
    return resultMap
}

// 2. 遍历所有图片
for nodeID, base64Data := range images {
    // 解码base64
    imageBytes, err := decodeBase64(base64Str)
    
    // 上传到OBS
    obsURL, obsKey, fileSize, err := globalOBSService.UploadImageFromBytes(
        imageBytes, fileKey, nodeID, format, scale, contentType,
    )
    
    // 收集结果
    obsURLs[nodeID] = obsURL
    obsKeys[nodeID] = obsKey
    
    // 异步保存到数据库
    go saveImageToCache(fileKey, nodeID, format, scale, obsKey, obsURL, fileSize)
}
```

#### 3. Base64解码 (`decodeBase64`)

支持多种base64格式：
- 纯base64字符串
- 带数据URI前缀的base64（如 `data:image/png;base64,xxx`）
- 自动去除空白字符

```go
func decodeBase64(base64Str string) ([]byte, error) {
    // 移除可能的数据URI前缀
    if idx := strings.Index(base64Str, ","); idx != -1 {
        base64Str = base64Str[idx+1:]
    }
    
    // 移除空白字符
    base64Str = strings.TrimSpace(base64Str)
    
    // 解码
    return base64.StdEncoding.DecodeString(base64Str)
}
```

#### 4. 数据库缓存 (`saveImageToCache`)

异步保存图片元数据到数据库：
- 检查是否已存在记录（更新）
- 不存在则创建新记录
- 保存OBS Key、URL、文件大小等信息
- 设置7天过期时间

## 返回数据结构

### OBS启用时的返回格式

```json
{
  "images": {
    "1-123": "base64图片数据...",
    "1-456": "base64图片数据...",
    "1-789": "base64图片数据..."
  },
  "total": 3,
  "success": 3,
  "failed": 0,
  "obs_enabled": true,
  "obs_urls": {
    "1-123": "https://bucket.endpoint.com/path/to/image1.png",
    "1-456": "https://bucket.endpoint.com/path/to/image2.png",
    "1-789": "https://bucket.endpoint.com/path/to/image3.png"
  },
  "obs_keys": {
    "1-123": "exports/batch_xxx/1-123_1.0.png",
    "1-456": "exports/batch_xxx/1-456_1.0.png",
    "1-789": "exports/batch_xxx/1-789_1.0.png"
  },
  "obs_uploaded": 3,
  "obs_failed": 0
}
```

### 部分上传失败时

```json
{
  "images": {
    "1-123": "base64图片数据...",
    "1-456": "base64图片数据..."
  },
  "total": 3,
  "success": 2,
  "failed": 1,
  "errors": {
    "1-789": "Node not found with ID: 1-789"
  },
  "obs_enabled": true,
  "obs_urls": {
    "1-123": "https://bucket.endpoint.com/path/to/image1.png"
  },
  "obs_keys": {
    "1-123": "exports/batch_xxx/1-123_1.0.png"
  },
  "obs_uploaded": 1,
  "obs_failed": 1,
  "obs_errors": {
    "1-456": "上传失败: connection timeout"
  }
}
```

### OBS未启用时

```json
{
  "images": {
    "1-123": "base64图片数据...",
    "1-456": "base64图片数据..."
  },
  "total": 2,
  "success": 2,
  "failed": 0,
  "obs_enabled": false,
  "obs_message": "OBS服务未启用，图片未上传到云端"
}
```

## Web界面增强

### 显示OBS上传状态

在 `mcp_center.html` 中，批量图片导出结果会显示：

1. **统计信息**：
   - 总数、成功数、失败数
   - OBS启用状态
   - OBS上传成功/失败数量

2. **每张图片的详细信息**：
   - 节点ID
   - 上传状态（☁️ 已上传到华为云）
   - OBS访问URL（可点击）
   - 图片预览

### 示例显示效果

```
📨 [响应]
{
  "total": 3,
  "success": 3,
  "failed": 0,
  "images": "&3 张图片",
  "obs_enabled": true,
  "obs_uploaded": 3,
  "obs_failed": 0
}

[图片展示区域]
━━━━━━━━━━━━━━━━━━━━━━━━
节点 ID: 1-123
☁️ 已上传到华为云
URL: https://bucket.endpoint.com/exports/batch_xxx/1-123_1.0.png
[图片预览]
━━━━━━━━━━━━━━━━━━━━━━━━
节点 ID: 1-456
☁️ 已上传到华为云
URL: https://bucket.endpoint.com/exports/batch_xxx/1-456_1.0.png
[图片预览]
━━━━━━━━━━━━━━━━━━━━━━━━
```

## 数据库存储

### 存储表结构

使用 `figma_node_images` 表存储图片信息：

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| file_key | TEXT | 文件Key（批量导出时使用时间戳） |
| node_id | TEXT | 节点ID |
| format | TEXT | 图片格式（png/jpg/svg） |
| scale | REAL | 缩放比例 |
| figma_cdn_url | TEXT | Figma CDN URL（或OBS URL） |
| obs_key | TEXT | OBS存储Key |
| obs_expires_at | INTEGER | OBS过期时间 |
| file_size | INTEGER | 文件大小（字节） |
| status | TEXT | 状态（obs_synced） |
| created_at | INTEGER | 创建时间 |
| updated_at | INTEGER | 更新时间 |

### 查询示例

```sql
-- 查询某个节点的所有缓存图片
SELECT * FROM figma_node_images WHERE node_id = '1-123';

-- 查询最近上传的批量图片
SELECT * FROM figma_node_images 
WHERE file_key LIKE 'batch_%' 
ORDER BY created_at DESC 
LIMIT 10;

-- 统计OBS上传情况
SELECT 
    COUNT(*) as total,
    SUM(CASE WHEN obs_key != '' THEN 1 ELSE 0 END) as obs_synced,
    SUM(file_size) as total_size
FROM figma_node_images;
```

## 配置要求

### OBS服务配置

在 `config.yaml` 中配置华为云OBS：

```yaml
obs:
  enabled: true
  endpoint: "obs.cn-north-4.myhuaweicloud.com"
  access_key_id: "YOUR_ACCESS_KEY"
  secret_access_key: "YOUR_SECRET_KEY"
  bucket_name: "your-bucket-name"
  region: "cn-north-4"
  path_prefix: "exports/"
  upload_concurrency: 5
  upload_timeout_seconds: 300
```

## 使用示例

### 1. 基本批量导出并上传

```json
{
  "command": "export_nodes_as_images",
  "params": {
    "nodeIds": "1-123,1-456,1-789",
    "scale": 2
  }
}
```

**结果**：3张图片自动导出并上传到OBS

### 2. 大规模批量导出

```json
{
  "command": "export_nodes_as_images",
  "params": {
    "nodeIds": "1-100,1-101,1-102,1-103,1-104,1-105,1-106,1-107,1-108,1-109",
    "scale": 1,
    "ignore_text": false
  }
}
```

**结果**：10张图片批量上传到OBS，返回所有URL

## 性能优化

### 1. 异步数据库保存

使用 `go` 关键字异步保存到数据库，不阻塞主流程：

```go
go saveImageToCache(fileKey, nodeID, format, scale, obsKey, obsURL, fileSize)
```

### 2. 批量处理

一次性处理多个图片，减少网络往返次数。

### 3. 错误隔离

单个图片上传失败不影响其他图片的处理。

## 错误处理

### OBS服务未启用

```json
{
  "obs_enabled": false,
  "obs_message": "OBS服务未启用，图片未上传到云端"
}
```

### 部分图片上传失败

```json
{
  "obs_uploaded": 2,
  "obs_failed": 1,
  "obs_errors": {
    "1-789": "上传失败: network timeout"
  }
}
```

### Base64解码失败

```json
{
  "obs_errors": {
    "1-123": "解码base64失败: illegal base64 data"
  }
}
```

## 安全考虑

1. **访问控制**：
   - 使用华为云IAM进行身份验证
   - OBS Key安全存储

2. **数据隔离**：
   - 每个用户的图片独立存储
   - 使用PathPrefix隔离不同类型的文件

3. **过期管理**：
   - 设置7天过期时间
   - 定期清理过期数据

## 监控和日志

### 上传日志

```
✅ [OBS] 上传成功: exports/batch_xxx/1-123_1.0.png (45678 bytes)
❌ [OBS] 上传失败: error=network timeout
```

### 统计信息

- 总上传数量
- 成功率
- 平均文件大小
- 上传耗时

## 故障排查

### 问题1：图片上传失败

**检查项：**
1. OBS服务是否启用
2. 网络连接是否正常
3. OBS配置是否正确
4. Bucket权限是否足够

### 问题2：Base64解码失败

**检查项：**
1. 图片数据是否完整
2. Base64格式是否正确
3. 数据传输是否有截断

### 问题3：数据库保存失败

**检查项：**
1. 数据库连接是否正常
2. 表结构是否正确
3. 磁盘空间是否充足

## 未来改进

1. **并发上传**：支持多个图片并发上传到OBS
2. **进度回调**：实时返回上传进度
3. **断点续传**：支持大文件断点续传
4. **CDN加速**：配置CDN加速访问
5. **自动清理**：定期清理过期的OBS文件
6. **批量下载**：支持批量下载OBS中的图片

## 相关文件

- `internal/controllers/mcp_tools.go` - 批量上传核心逻辑
- `internal/services/obs.go` - OBS服务实现
- `web/templates/mcp_center.html` - Web界面显示
- `internal/models/cache.go` - 数据库模型
- `docs/BATCH_IMAGE_EXPORT_FEATURE.md` - 批量导出功能文档

## 更新日期

2025-12-01

