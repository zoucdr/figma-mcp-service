# 华为云 OBS 集成指南

## 概述

本文档说明如何配置和使用华为云 OBS（对象存储服务）来缓存 Figma 图片。

## 功能特性

### ✅ 已实现功能

1. **上传功能**
   - 从 URL 下载图片并上传到 OBS
   - 从字节数组直接上传到 OBS
   - 批量上传（支持并发控制）

2. **下载功能**
   - 从 OBS 下载图片
   - 生成带签名的临时访问 URL

3. **管理功能**
   - 检查对象是否存在
   - 获取对象大小
   - 删除对象

4. **路径规范**
   - 自动生成标准化的存储路径
   - 格式：`{prefix}/{file_key}/{format}_{scale}x/{node_id}.{ext}`

## 配置说明

### 1. 配置文件 (`configs/config.yaml`)

```yaml
huawei_cloud:
  obs:
    enabled: true  # 是否启用 OBS
    endpoint: "obs.cn-north-4.myhuaweicloud.com"  # OBS 端点
    access_key_id: "YOUR_ACCESS_KEY"  # 访问密钥 ID
    secret_access_key: "YOUR_SECRET_KEY"  # 密钥
    bucket_name: "figma-deliver-cache"  # 桶名称
    region: "cn-north-4"  # 区域
    path_prefix: "figma_images/"  # 存储路径前缀
    upload_concurrency: 5  # 并发上传数
    upload_timeout_seconds: 300  # 上传超时（秒）
```

### 2. 获取华为云 OBS 凭证

1. **登录华为云控制台**
   - 访问：https://console.huaweicloud.com

2. **创建 OBS 桶**
   - 进入"对象存储服务 OBS"
   - 点击"创建桶"
   - 选择区域（如：华北-北京四 cn-north-4）
   - 设置桶名称（如：figma-deliver-cache）
   - 选择存储类别（标准存储）

3. **创建访问密钥**
   - 点击右上角用户名 → "我的凭证"
   - 左侧菜单选择"访问密钥"
   - 点击"新增访问密钥"
   - 下载 credentials.csv 文件（包含 Access Key ID 和 Secret Access Key）

4. **配置桶权限**（可选）
   - 如需公开访问：设置桶策略为"公共读"
   - 如需私有访问：使用签名 URL 访问

## 使用方法

### 服务初始化

OBS 服务在应用启动时自动初始化（`main.go`）：

```go
obsService, err := services.NewOBSService(services.OBSConfig{
    Enabled:              appConfig.HuaweiCloud.OBS.Enabled,
    Endpoint:             appConfig.HuaweiCloud.OBS.Endpoint,
    AccessKeyID:          appConfig.HuaweiCloud.OBS.AccessKeyID,
    SecretAccessKey:      appConfig.HuaweiCloud.OBS.SecretAccessKey,
    BucketName:           appConfig.HuaweiCloud.OBS.BucketName,
    Region:               appConfig.HuaweiCloud.OBS.Region,
    PathPrefix:           appConfig.HuaweiCloud.OBS.PathPrefix,
    UploadConcurrency:    appConfig.HuaweiCloud.OBS.UploadConcurrency,
    UploadTimeoutSeconds: appConfig.HuaweiCloud.OBS.UploadTimeoutSeconds,
})
```

### 代码示例

#### 1. 从 URL 上传图片

```go
obsURL, obsKey, fileSize, err := obsService.UploadImageFromURL(
    "https://figma-cdn.com/image.png",  // Figma CDN URL
    "abc123",                            // File Key
    "1:123",                             // Node ID
    "png",                               // Format
    2.0,                                 // Scale
)

if err != nil {
    log.Printf("上传失败: %v", err)
    return
}

log.Printf("上传成功!")
log.Printf("  OBS URL: %s", obsURL)
log.Printf("  OBS Key: %s", obsKey)
log.Printf("  文件大小: %d bytes", fileSize)
```

**生成的路径示例**:
```
figma_images/abc123/png_2.0x/1-123.png
```

#### 2. 批量上传图片

```go
// 准备图片映射（节点ID -> Figma CDN URL）
imageMap := map[string]string{
    "1:123": "https://figma-cdn.com/image1.png",
    "1:456": "https://figma-cdn.com/image2.png",
    "1:789": "https://figma-cdn.com/image3.png",
}

// 批量上传
results := obsService.BatchUploadImagesFromURLs(
    imageMap,
    "abc123",  // File Key
    "png",     // Format
    2.0,       // Scale
)

// 处理结果
for _, result := range results {
    if result.Error != nil {
        log.Printf("❌ 节点 %s 上传失败: %v", result.NodeID, result.Error)
    } else {
        log.Printf("✅ 节点 %s 上传成功: %s", result.NodeID, result.OBSURL)
    }
}
```

#### 3. 下载图片

```go
data, contentType, err := obsService.DownloadImage("figma_images/abc123/png_2.0x/1-123.png")

if err != nil {
    log.Printf("下载失败: %v", err)
    return
}

log.Printf("下载成功! 大小: %d bytes, 类型: %s", len(data), contentType)
```

#### 4. 生成临时访问 URL

```go
// 生成有效期为 1 小时的签名 URL
signedURL, err := obsService.GenerateSignedURL(
    "figma_images/abc123/png_2.0x/1-123.png",
    3600,  // 有效期 3600 秒（1小时）
)

if err != nil {
    log.Printf("生成签名URL失败: %v", err)
    return
}

log.Printf("签名URL: %s", signedURL)
```

#### 5. 检查对象是否存在

```go
exists, err := obsService.CheckObjectExists("figma_images/abc123/png_2.0x/1-123.png")

if err != nil {
    log.Printf("检查失败: %v", err)
    return
}

if exists {
    log.Println("对象存在")
} else {
    log.Println("对象不存在")
}
```

#### 6. 删除对象

```go
err := obsService.DeleteObject("figma_images/abc123/png_2.0x/1-123.png")

if err != nil {
    log.Printf("删除失败: %v", err)
    return
}

log.Println("删除成功")
```

## 存储路径规范

### 路径格式

```
{path_prefix}/{file_key}/{format}_{scale}x/{node_id}.{ext}
```

### 组成部分

1. **path_prefix**: 配置的路径前缀（如：`figma_images/`）
2. **file_key**: Figma 文件 Key
3. **format**: 图片格式（png/jpg/svg）
4. **scale**: 缩放比例（如：1.0x, 2.0x）
5. **node_id**: 节点 ID（冒号替换为连字符）
6. **ext**: 文件扩展名

### 路径示例

```
figma_images/abc123xyz/png_2.0x/1-123.png
figma_images/abc123xyz/jpg_1.0x/1-456.jpeg
figma_images/def456/svg_1.0x/2-789.svg
```

## 与缓存系统集成

### 自动上传流程

1. **从 Figma API 获取图片 URL**
   - 调用 Figma Images API 获取临时 CDN URL

2. **保存到数据库**
   - 保存 `figma_cdn_url` 到 `figma_node_images` 表

3. **异步上传到 OBS**
   - 后台任务从 Figma CDN 下载图片
   - 上传到华为云 OBS
   - 更新 `obs_url` 和 `obs_key`

4. **设置过期时间**
   - `obs_expires_at` = 当前时间 + 30天

### 读取优先级

```
1. 检查 obs_url 是否存在且未过期
   ├─ 是 → 返回 OBS URL
   └─ 否 → 继续检查

2. 检查 figma_cdn_url 是否存在
   ├─ 是 → 返回 Figma CDN URL（并触发 OBS 上传）
   └─ 否 → 加入渲染队列
```

## 日志示例

### 成功上传

```
📥 [OBS] 开始下载图片: https://figma-cdn.com/image.png
✅ [OBS] 图片下载成功，大小: 152847 bytes
✅ [OBS] 上传成功
   - OBS Key: figma_images/abc123/png_2.0x/1-123.png
   - OBS URL: https://obs.cn-north-4.myhuaweicloud.com/figma-deliver-cache/figma_images/abc123/png_2.0x/1-123.png
   - 文件大小: 152847 bytes
```

### 批量上传

```
📦 [OBS] 开始批量上传，共 10 个图片
📥 [OBS] 开始下载图片: https://figma-cdn.com/image1.png
📥 [OBS] 开始下载图片: https://figma-cdn.com/image2.png
...
✅ [OBS] 批量上传完成: 成功 10/10
```

## 故障排查

### 问题1: OBS 服务初始化失败

**错误信息**:
```
❌ OBS服务初始化失败: endpoint is invalid
```

**解决方法**:
- 检查 `endpoint` 配置是否正确
- 确认区域代码（如：cn-north-4）
- 去掉协议前缀（使用 `obs.cn-north-4.myhuaweicloud.com` 而不是 `https://...`）

### 问题2: 上传失败 - 权限不足

**错误信息**:
```
❌ [OBS] 上传失败: status code: 403, error code: AccessDenied
```

**解决方法**:
- 检查 Access Key ID 和 Secret Access Key 是否正确
- 确认 IAM 用户具有 OBS 写权限
- 检查桶策略和 ACL 配置

### 问题3: 下载失败 - 对象不存在

**错误信息**:
```
❌ [OBS] 下载失败: status code: 404, error code: NoSuchKey
```

**解决方法**:
- 检查 OBS Key 是否正确
- 使用 `CheckObjectExists` 验证对象是否存在
- 检查路径是否包含正确的前缀

### 问题4: 超时错误

**错误信息**:
```
❌ [OBS] 下载图片失败: context deadline exceeded
```

**解决方法**:
- 增加 `upload_timeout_seconds` 配置
- 检查网络连接
- 考虑使用华为云 ECS 以获得更好的网络性能

## 性能优化

### 1. 并发控制

```yaml
huawei_cloud:
  obs:
    upload_concurrency: 5  # 同时上传 5 个文件
```

**建议值**:
- 小图片（< 100KB）：10-20
- 中等图片（100KB - 1MB）：5-10
- 大图片（> 1MB）：3-5

### 2. 超时设置

```yaml
huawei_cloud:
  obs:
    upload_timeout_seconds: 300  # 5 分钟
```

**建议值**:
- 标准场景：300 秒
- 大文件上传：600 秒
- 网络较差：900 秒

### 3. CDN 加速（可选）

如果需要更快的访问速度，可以配置华为云 CDN：

1. 在华为云控制台创建 CDN 加速域名
2. 将 OBS 桶设置为源站
3. 使用 CDN 域名替代 OBS 域名生成 URL

## 成本估算

### OBS 计费项

1. **存储费用**
   - 标准存储：约 ¥0.099/GB/月
   - 示例：1000 张图片，每张 100KB = 100MB = ¥0.01/月

2. **流量费用**
   - 外网下行流量：约 ¥0.50/GB
   - 示例：10000 次访问，每次 100KB = 1GB = ¥0.50

3. **请求费用**
   - PUT 请求：约 ¥0.01/万次
   - GET 请求：约 ¥0.01/万次

### 节省成本建议

1. **启用 OBS 生命周期规则**
   - 自动转储冷数据到低频访问存储
   - 自动删除过期文件

2. **使用 CDN**
   - 减少 OBS 的外网流量费用
   - 提高访问速度

3. **压缩图片**
   - 在上传前压缩图片
   - 减少存储和流量成本

## 下一步

- ⏳ **实现定时任务**: 自动处理渲染队列并上传到 OBS
- ⏳ **实现清理机制**: 定期清理过期的 OBS 文件
- ⏳ **监控和告警**: 监控 OBS 使用情况和成本

## 相关文档

- [完整实施方案](./FIGMA_CACHE_RATE_LIMIT_SOLUTION.md)
- [API 测试指南](./API_TESTING_GUIDE.md)
- [数据库迁移指南](./DATABASE_MIGRATION_GUIDE.md)
- [华为云 OBS 官方文档](https://support.huaweicloud.com/obs/index.html)

