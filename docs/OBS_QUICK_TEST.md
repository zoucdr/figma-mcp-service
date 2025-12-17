# OBS 快速测试指南

## 测试前准备

### 1. 配置 OBS

编辑 `configs/config.yaml`：

```yaml
huawei_cloud:
  obs:
    enabled: true  # ⚠️ 确保已启用
    endpoint: "obs.cn-north-4.myhuaweicloud.com"
    access_key_id: "YOUR_ACCESS_KEY"
    secret_access_key: "YOUR_SECRET_KEY"
    bucket_name: "figma-deliver-cache"
    region: "cn-north-4"
    path_prefix: "figma_images/"
    upload_concurrency: 5
    upload_timeout_seconds: 300
```

### 2. 启动服务

```bash
cd d:\figma-deliver\service
.\figma-deliver.exe
```

### 3. 检查启动日志

看到以下日志表示 OBS 初始化成功：

```
✅ [OBS] 初始化成功
   - Endpoint: obs.cn-north-4.myhuaweicloud.com
   - Bucket: figma-deliver-cache
   - Region: cn-north-4
   - PathPrefix: figma_images/
```

如果看到 `📦 [OBS] OBS服务未启用`，说明 `enabled: false`，需要修改配置。

## 测试方法

### 方法1: 使用 API 测试（推荐）

#### 测试上传图片到 OBS

当你调用渲染 API 时，系统会自动：
1. 从 Figma API 获取图片 URL
2. 下载图片
3. 上传到 OBS
4. 保存 OBS URL 到数据库

```bash
# 创建渲染队列（会触发 OBS 上传）
curl -X POST http://localhost:8080/api/figma/manual/render \
  -H "Content-Type: application/json" \
  -d '{
    "file_key": "YOUR_FILE_KEY",
    "node_ids": ["1:123", "1:456"],
    "format": "png",
    "scale": 2.0
  }'
```

#### 查看上传日志

在服务日志中查找：

```
📥 [OBS] 开始下载图片: https://...
✅ [OBS] 图片下载成功，大小: 152847 bytes
✅ [OBS] 上传成功
   - OBS Key: figma_images/YOUR_FILE_KEY/png_2.0x/1-123.png
   - OBS URL: https://obs.cn-north-4.myhuaweicloud.com/figma-deliver-cache/...
   - 文件大小: 152847 bytes
```

### 方法2: 直接测试（需要修改代码）

创建测试文件 `test_obs.go`：

```go
package main

import (
	"log"
	"github.com/figma-deliver/internal/services"
)

func main() {
	// 初始化 OBS 服务
	obsService, err := services.NewOBSService(services.OBSConfig{
		Enabled:              true,
		Endpoint:             "obs.cn-north-4.myhuaweicloud.com",
		AccessKeyID:          "YOUR_ACCESS_KEY",
		SecretAccessKey:      "YOUR_SECRET_KEY",
		BucketName:           "figma-deliver-cache",
		Region:               "cn-north-4",
		PathPrefix:           "figma_images/",
		UploadConcurrency:    5,
		UploadTimeoutSeconds: 300,
	})
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}
	defer obsService.Close()

	// 测试上传
	testImageURL := "https://s3-alpha.figma.com/checksum/abc123"
	obsURL, obsKey, fileSize, err := obsService.UploadImageFromURL(
		testImageURL,
		"test_file_key",
		"1:123",
		"png",
		2.0,
	)

	if err != nil {
		log.Fatalf("❌ 上传失败: %v", err)
	}

	log.Printf("✅ 上传成功!")
	log.Printf("   OBS URL: %s", obsURL)
	log.Printf("   OBS Key: %s", obsKey)
	log.Printf("   文件大小: %d bytes", fileSize)

	// 测试下载
	data, contentType, err := obsService.DownloadImage(obsKey)
	if err != nil {
		log.Fatalf("❌ 下载失败: %v", err)
	}

	log.Printf("✅ 下载成功!")
	log.Printf("   内容类型: %s", contentType)
	log.Printf("   数据大小: %d bytes", len(data))

	// 测试检查存在
	exists, err := obsService.CheckObjectExists(obsKey)
	if err != nil {
		log.Fatalf("❌ 检查失败: %v", err)
	}

	log.Printf("✅ 对象存在: %v", exists)
}
```

运行测试：

```bash
cd d:\figma-deliver\service
go run test_obs.go
```

### 方法3: 在华为云控制台验证

1. **登录华为云控制台**
   - https://console.huaweicloud.com

2. **进入 OBS 服务**
   - 对象存储服务 → 对象存储

3. **选择桶**
   - 点击配置的桶名称（如：figma-deliver-cache）

4. **查看文件**
   - 进入 `figma_images/` 目录
   - 应该能看到上传的图片

5. **验证文件结构**
   ```
   figma_images/
   └── {file_key}/
       └── png_2.0x/
           └── 1-123.png
   ```

## 常见测试场景

### 场景1: 测试基本上传

```bash
# 1. 调用手动渲染 API
curl -X POST http://localhost:8080/api/figma/manual/render \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "file_key": "abc123xyz",
    "node_ids": ["1:123"],
    "format": "png",
    "scale": 1.0
  }'

# 2. 查看响应（应包含 queue_id）
# 3. 在服务日志中查找 OBS 上传日志
# 4. 在华为云控制台验证文件是否存在
```

### 场景2: 测试批量上传

```bash
# 1. 提交包含多个节点的渲染请求
curl -X POST http://localhost:8080/api/figma/manual/render \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "file_key": "abc123xyz",
    "node_ids": ["1:123", "1:456", "1:789", "2:111", "2:222"],
    "format": "png",
    "scale": 2.0
  }'

# 2. 在日志中查找批量上传提示：
# 📦 [OBS] 开始批量上传，共 5 个图片
# ✅ [OBS] 批量上传完成: 成功 5/5
```

### 场景3: 测试缓存命中

```bash
# 1. 第一次请求（会上传到 OBS）
curl -X GET "http://localhost:8080/api/figma/node/image?file_key=abc123&node_id=1:123&format=png&scale=2.0" \
  -H "Authorization: Bearer YOUR_TOKEN"

# 2. 第二次请求（应该从 OBS 返回，不需要重新上传）
curl -X GET "http://localhost:8080/api/figma/node/image?file_key=abc123&node_id=1:123&format=png&scale=2.0" \
  -H "Authorization: Bearer YOUR_TOKEN"

# 3. 在日志中应该看到缓存命中提示
```

### 场景4: 测试错误处理

#### 错误的 Access Key

```yaml
huawei_cloud:
  obs:
    access_key_id: "WRONG_KEY"
```

**预期结果**: 启动时报错
```
❌ OBS服务初始化失败: authentication failed
```

#### 错误的 Bucket

```yaml
huawei_cloud:
  obs:
    bucket_name: "non-existent-bucket"
```

**预期结果**: 上传时报错
```
❌ [OBS] 上传失败: status code: 404, error code: NoSuchBucket
```

## 监控和日志

### 关键日志标识

| 日志前缀 | 含义 |
|---------|------|
| `✅ [OBS]` | 操作成功 |
| `❌ [OBS]` | 操作失败 |
| `📥 [OBS]` | 开始下载 |
| `📦 [OBS]` | 批量操作 |
| `🔗 [OBS]` | 生成 URL |
| `🗑️ [OBS]` | 删除操作 |
| `🔒 [OBS]` | 关闭连接 |

### 日志过滤

查看所有 OBS 相关日志：

```bash
# Windows PowerShell
.\figma-deliver.exe | Select-String "\[OBS\]"

# Linux/Mac
./figma-deliver | grep "\[OBS\]"
```

### 统计成功率

```bash
# 统计上传成功数
cat service.log | grep "\[OBS\] 上传成功" | wc -l

# 统计上传失败数
cat service.log | grep "\[OBS\] 上传失败" | wc -l
```

## 性能基准

### 预期性能指标

| 操作 | 文件大小 | 预期时间 |
|------|---------|---------|
| 上传 | 100KB | 1-2 秒 |
| 上传 | 1MB | 3-5 秒 |
| 下载 | 100KB | 0.5-1 秒 |
| 下载 | 1MB | 1-2 秒 |
| 批量上传（5个，100KB 各） | 500KB | 2-3 秒（并发） |

### 测量实际性能

在代码中添加计时：

```go
start := time.Now()
obsURL, obsKey, fileSize, err := obsService.UploadImageFromURL(...)
duration := time.Since(start)

log.Printf("⏱️ [OBS] 上传耗时: %v", duration)
```

## 故障排查清单

- [ ] OBS 服务已启用 (`enabled: true`)
- [ ] Access Key ID 和 Secret Access Key 正确
- [ ] Endpoint 格式正确（不含 `https://`）
- [ ] Bucket 已创建且名称正确
- [ ] Region 与 Bucket 区域一致
- [ ] IAM 用户具有 OBS 读写权限
- [ ] 网络可以访问华为云 OBS
- [ ] 服务启动日志显示 OBS 初始化成功
- [ ] 测试图片 URL 可访问

## 下一步

测试通过后，可以继续：

1. **实现定时任务**
   - 自动处理渲染队列
   - 自动上传图片到 OBS

2. **实现前端集成**
   - 显示 OBS 上传进度
   - 显示缓存命中状态

3. **监控和优化**
   - 监控 OBS 使用情况
   - 优化上传并发数
   - 实现断点续传

## 相关文档

- [OBS 集成指南](./OBS_INTEGRATION_GUIDE.md)
- [完整实施方案](./FIGMA_CACHE_RATE_LIMIT_SOLUTION.md)
- [API 测试指南](./API_TESTING_GUIDE.md)

