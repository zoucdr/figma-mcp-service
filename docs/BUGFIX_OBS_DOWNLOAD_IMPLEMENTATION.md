# Bug 修复：实现 OBS 下载功能

## 问题描述

在使用图片预览功能时，系统会尝试从 OBS（对象存储服务）下载图片，但 `downloadImageFromOBS` 函数只是一个占位符，未实现实际的下载逻辑。

### 错误日志
```
找到 OBS Key: figma_images/wlo3fMIPfYdx30lNs7wAVc/png_1.0x/8-11102.png
⚠️ OBS 下载功能尚未实现
⚠️ 从 OBS 下载失败: OBS 下载功能尚未实现，尝试 Figma CDN
```

## 问题分析

1. `internal/services/figma.go` 中的 `downloadImageFromOBS` 函数未实现
2. 虽然 `internal/services/obs.go` 中已经有 `DownloadImage` 方法
3. 但 `downloadImageFromOBS` 无法访问 OBS 服务实例

## 解决方案

### 1. 在 `services` 包中添加全局 OBS 服务实例

**文件**: `internal/services/figma.go`

```go
// 全局 OBS 服务实例（由 main.go 设置）
var globalOBSService *OBSService

// SetGlobalOBSService 设置全局 OBS 服务实例
func SetGlobalOBSService(obs *OBSService) {
	globalOBSService = obs
}
```

### 2. 实现 `downloadImageFromOBS` 函数

**文件**: `internal/services/figma.go`

替换原来的占位符实现为：

```go
// downloadImageFromOBS 从 OBS 下载图片到本地缓存
func downloadImageFromOBS(obsKey, tempDir, safeNodeID, imageFormat string, imageScale float64) (string, error) {
	// 检查 OBS 服务是否可用
	if globalOBSService == nil || !globalOBSService.IsEnabled() {
		return "", fmt.Errorf("OBS 服务未启用")
	}

	fmt.Printf("📥 [OBS] 开始下载图片: %s\n", obsKey)

	// 从 OBS 下载图片数据
	imageData, contentType, err := globalOBSService.DownloadImage(obsKey)
	if err != nil {
		return "", fmt.Errorf("从 OBS 下载失败: %w", err)
	}

	fmt.Printf("✅ [OBS] 图片下载成功，大小: %d bytes, Content-Type: %s\n", len(imageData), contentType)

	// 确定文件扩展名
	fileExt := "." + imageFormat
	if imageFormat == "" {
		fileExt = ".png"
	}

	// 生成本地文件路径
	scaleStr := formatScaleForFilename(imageScale)
	fileName := fmt.Sprintf("%s-%s%s", safeNodeID, scaleStr, fileExt)
	localPath := filepath.Join(tempDir, fileName)

	// 写入本地文件
	err = os.WriteFile(localPath, imageData, 0644)
	if err != nil {
		return "", fmt.Errorf("写入本地文件失败: %w", err)
	}

	fmt.Printf("✅ [OBS] 图片已保存到本地: %s\n", localPath)
	return localPath, nil
}
```

### 3. 在 main.go 中设置全局 OBS 服务实例

**文件**: `main.go`

```go
// 设置全局 OBS 服务实例（供图片控制器和服务使用）
controllers.SetGlobalOBSService(obsService)
services.SetGlobalOBSService(obsService)  // 新增这一行
```

## 功能说明

实现后的 `downloadImageFromOBS` 函数会：

1. **检查 OBS 服务状态**
   - 验证 OBS 服务是否已启用
   - 如果未启用，返回错误（系统会回退到 Figma CDN）

2. **从 OBS 下载图片**
   - 调用 `globalOBSService.DownloadImage(obsKey)` 获取图片数据
   - 返回图片字节数据和 Content-Type

3. **保存到本地缓存**
   - 根据节点 ID 和缩放比例生成唯一的文件名
   - 将图片数据写入本地临时目录
   - 返回本地文件路径供后续使用

## 图片获取优先级

系统会按以下顺序尝试获取图片：

1. **本地缓存** - 如果本地已有缓存文件，直接返回
2. **数据库记录** - 查询 `figma_node_images` 表
3. **OBS 存储** ✅ - 如果有 `OBSKey`，从 OBS 下载（现已实现）
4. **Figma CDN** - 如果有 `FigmaCDNURL`，从 Figma CDN 下载
5. **裂图** - 所有途径都失败时返回占位图

## 优势

1. **提高访问速度**
   - OBS 通常比 Figma CDN 更快（特别是在中国区域）
   - 避免 Figma CDN URL 过期问题

2. **减少 Figma API 调用**
   - 不需要重新调用 Figma Images API 获取新 URL
   - 节省 API 配额

3. **提高可靠性**
   - OBS 提供持久化存储
   - 不受 Figma CDN URL 有效期限制

## 测试建议

1. 确保 OBS 配置正确（在 `configs/config.yaml` 中）
2. 验证图片已上传到 OBS（检查数据库中的 `obs_key` 字段）
3. 清除本地缓存后访问图片，观察是否从 OBS 下载
4. 检查日志输出，确认下载流程正常

## 日志示例

成功从 OBS 下载的日志：

```
📦 找到 OBS Key: figma_images/wlo3fMIPfYdx30lNs7wAVc/png_1.0x/8-11102.png
📥 [OBS] 开始下载图片: figma_images/wlo3fMIPfYdx30lNs7wAVc/png_1.0x/8-11102.png
✅ [OBS] 下载成功: figma_images/wlo3fMIPfYdx30lNs7wAVc/png_1.0x/8-11102.png (45678 bytes)
✅ [OBS] 图片下载成功，大小: 45678 bytes, Content-Type: image/png
✅ [OBS] 图片已保存到本地: temp/wlo3fMIPfYdx30lNs7wAVc/previews/8_11102-1.0x.png
✅ 成功从 OBS 下载图片到本地: temp/wlo3fMIPfYdx30lNs7wAVc/previews/8_11102-1.0x.png
```

## 相关文件

- `internal/services/figma.go` - 实现下载逻辑
- `internal/services/obs.go` - OBS 服务封装
- `main.go` - 初始化和设置全局服务实例
- `internal/models/cache.go` - 图片缓存数据模型

## 注意事项

1. OBS 服务需要在配置文件中正确配置
2. 如果 OBS 未启用，系统会自动回退到 Figma CDN
3. 下载的图片会保存到本地缓存，避免重复下载

