# 优化导出图片下载流程

## 概述

优化了 `DownloadFigmaImageForExport` 函数（原名 `DownloadPreviewFigmaImageForExport`），改进图片获取优先级策略，优先使用 WebSocket 获取最新数据。

## 主要改进

### 1. 函数重命名
- **旧名称**: `DownloadPreviewFigmaImageForExport`
- **新名称**: `DownloadFigmaImageForExport`
- 移除 "Preview" 以反映其通用的图片下载功能

### 2. 优化获取优先级

#### 旧流程（按顺序）：
1. 检查本地缓存文件
2. 查询数据库记录
3. 从 OBS 下载
4. 从 Figma CDN 下载
5. 通过 WebSocket 获取（作为兜底）

#### 新流程（优化后）：
1. **✨ 优先检查 WebSocket 连接**
   - 如果 userID > 0 且 WebSocket 连接存在
   - 直接通过 WebSocket 获取**最新数据**
   - WebSocket 成功后自动更新：OBS、数据库、本地文件
2. 检查本地缓存文件（WebSocket 不可用或失败时）
3. 查询数据库记录
4. 从 OBS 下载
5. 从 Figma CDN 下载
6. 返回错误（所有途径都失败）

### 3. 优化原因

#### WebSocket 优先的优势：
- **数据最新**：直接从 Figma 实时获取，确保数据是最新的
- **自动更新**：`handleImageDataFromPlugin` 会自动更新所有存储层
  - 保存到本地文件
  - 上传到华为云 OBS
  - 更新数据库记录
- **适合导出场景**：导出时通常需要最新、最准确的数据

#### 降级策略：
- 如果 WebSocket 不可用或失败，自动降级到缓存数据
- 保证系统的高可用性和容错能力

## 代码变更

### 文件：`internal/services/export.go`

#### 1. 函数签名和实现

```go
// 旧函数名
func DownloadPreviewFigmaImageForExport(...)

// 新函数名
func DownloadFigmaImageForExport(token, fileKey, nodeID, imageFormat string, imageScale float64, ignoreTexts bool, userID uint) (string, error)
```

#### 2. 新增 WebSocket 优先检查逻辑

```go
// 1. 优先检查 WebSocket 是否可用
if userID > 0 {
    fmt.Printf("🔌 检查 WebSocket 连接状态 (UserID=%d)\n", userID)
    
    // 检查用户是否有活跃的 WebSocket 连接
    mcpService := GetMCPService()
    if mcpService != nil {
        _, exists := mcpService.GetUserConnection(userID)
        if exists {
            fmt.Printf("✅ WebSocket 连接存在，优先使用 WebSocket 获取最新数据\n")
            localPath, err := TryGetImageViaWebSocket(...)
            if err == nil && localPath != "" {
                fmt.Printf("✅ 成功通过 WebSocket 获取最新图片: %s\n", localPath)
                // WebSocket 成功获取，handleImageDataFromPlugin 已经自动更新了 OBS、数据库和本地文件
                return localPath, nil
            }
            fmt.Printf("⚠️ WebSocket 获取失败: %v，尝试降级到缓存\n", err)
        }
    }
}

// 2. 降级到本地缓存
existingFile := findLatestNodeImageWithIgnoreTexts(...)
// ...后续降级逻辑
```

## 使用场景

### 适用场景
1. **导出功能**：需要最新的 Figma 设计数据
2. **实时预览**：用户正在 Figma 中编辑并导出
3. **增量更新**：设计有更新时需要重新导出

### 效果
- ✅ 确保导出的图片是最新的
- ✅ 自动更新所有缓存层（本地、OBS、数据库）
- ✅ 降级策略保证高可用性
- ✅ 减少过期缓存导致的问题

## 日志输出

新流程的日志示例：

```
📖 获取Figma图片(导出): fileKey=xxx, nodeID=xxx, format=png, scale=1.0, ignoreTexts=false, userID=123
缓存目录路径: temp/xxx/previews
原始节点ID: xxx, 安全节点ID: xxx
🔌 检查 WebSocket 连接状态 (UserID=123)
✅ WebSocket 连接存在，优先使用 WebSocket 获取最新数据 (ignoreTexts=false)
✅ 成功通过 WebSocket 获取最新图片: temp/xxx/previews/xxx-1x.png
```

或降级场景：

```
🔌 检查 WebSocket 连接状态 (UserID=123)
⚠️ WebSocket 连接不存在，使用缓存数据
✅ 找到本地缓存图片: temp/xxx/previews/xxx-1x.png
```

## 注意事项

1. **WebSocket 连接检查**：仅在 `userID > 0` 时才检查 WebSocket
2. **自动更新**：WebSocket 成功后，`handleImageDataFromPlugin` 会自动处理所有更新
3. **容错机制**：WebSocket 失败时自动降级，不影响系统可用性
4. **性能考虑**：WebSocket 连接检查非常快速，不影响性能

## 相关函数

- `TryGetImageViaWebSocket`: 通过 WebSocket 获取图片
- `handleImageDataFromPlugin`: 处理插件返回的图片数据，自动更新 OBS、数据库、本地文件
- `GetMCPService`: 获取 MCP 服务实例
- `GetUserConnection`: 检查用户的 WebSocket 连接状态

## 测试建议

1. **WebSocket 可用场景**：
   - 用户已连接 Figma 插件
   - 导出功能应该获取最新数据
   - 验证 OBS、数据库、本地文件都已更新

2. **WebSocket 不可用场景**：
   - 用户未连接插件
   - 应该降级到缓存数据
   - 验证降级流程正常工作

3. **数据一致性**：
   - WebSocket 获取后，验证所有存储层数据一致
   - 后续调用应该能从缓存快速获取

