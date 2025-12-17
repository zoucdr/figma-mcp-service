# 重构：提取 GetFigmaNodesNoCache 函数

## 概述

本次重构的目标是将 Figma API 调用逻辑提取到独立的 `GetFigmaNodesNoCache` 函数中，简化代码结构并移除不必要的 `deleteCacheFile` 函数。

## 背景

之前的实现中：
1. `GetFigmaNodes` 函数包含了缓存检查和 API 调用的所有逻辑
2. `RefreshProjectNodeTree` 使用 `deleteCacheFile` 删除缓存文件后调用 `GetFigmaNodes`
3. `GetFigmaNodeTree` 在强制刷新时手动删除缓存文件
4. 存在重复的缓存保存逻辑

## 实现方案

### 1. 新增 `GetFigmaNodesNoCache` 函数

**文件**: `internal/services/figma.go`

创建新函数 `GetFigmaNodesNoCache`，包含以下功能：
- 直接调用 Figma API（跳过所有缓存检查）
- 自动保存响应到本地文件缓存
- 异步保存响应到数据库缓存（`figma_file_caches` 表）
- 提取所有节点 ID 并保存到数据库

```go
// GetFigmaNodesNoCache 获取Figma节点（不使用缓存，直接调用API并保存到缓存）
func GetFigmaNodesNoCache(token, fileKey, nodeID string) ([]map[string]interface{}, error) {
	fmt.Printf("📡 调用 Figma API 获取节点树（跳过缓存）: fileKey=%s, nodeID=%s\n", fileKey, nodeID)
	
	// 构建API URL
	url := fmt.Sprintf("https://api.figma.com/v1/files/%s", fileKey)
	if nodeID != "" {
		url = fmt.Sprintf("%s/nodes?ids=%s", url, nodeID)
	}

	// 创建请求、发送请求、处理响应
	// ... API 调用逻辑 ...

	// 缓存节点数据到本地文件
	if err := cacheNodes(fileKey, nodeID, respBody); err != nil {
		fmt.Printf("⚠️ 缓存节点树到本地文件失败: %v\n", err)
	} else {
		fmt.Printf("✅ 节点树已缓存到本地文件\n")
	}

	// 异步保存到数据库
	go func() {
		// 提取所有节点ID
		nodeIDList, err := models.ExtractAllNodeIDs(string(respBody))
		// ... 保存到数据库 ...
	}()

	return nodes, nil
}
```

### 2. 简化 `GetFigmaNodes` 函数

**文件**: `internal/services/figma.go`

重构 `GetFigmaNodes` 使用三级缓存策略：
1. 检查本地文件缓存
2. 检查数据库缓存
3. 调用 `GetFigmaNodesNoCache`

```go
// GetFigmaNodes 获取Figma节点，支持缓存
func GetFigmaNodes(token, fileKey, nodeID string) ([]map[string]interface{}, error) {
	// 1. 检查本地文件缓存
	cachedNodes, err := loadCachedNodes(fileKey, nodeID)
	if err == nil && cachedNodes != nil {
		fmt.Printf("✅ 使用本地缓存的节点树数据: fileKey=%s, nodeID=%s\n", fileKey, nodeID)
		return cachedNodes, nil
	}

	// 2. 从数据库查询节点树缓存
	dbCache, err := models.GetFileCache(fileKey, nodeID)
	if err == nil && dbCache != nil && dbCache.Status == "loaded" && dbCache.FileData != "" {
		// ... 使用数据库缓存 ...
		return nodes, nil
	}

	// 3. 本地缓存和数据库都没有，调用 Figma API
	return GetFigmaNodesNoCache(token, fileKey, nodeID)
}
```

### 3. 简化 `RefreshProjectNodeTree` 函数

**文件**: `internal/controllers/cache.go`

直接调用 `GetFigmaNodesNoCache`，无需手动删除缓存或保存数据：

```go
// 直接调用 Figma API 获取节点树（跳过缓存，GetFigmaNodesNoCache 会自动保存到数据库）
nodes, fetchErr := services.GetFigmaNodesNoCache(user.FigmaToken, project.FileKey, project.RootNodeID)
if fetchErr != nil {
	// 处理错误，尝试从数据库恢复
	// ...
}

// 刷新成功，GetFigmaNodesNoCache 已自动保存到数据库和本地缓存
// （无需手动处理缓存保存逻辑）
```

**移除的代码**：
- 删除了对 `deleteCacheFile` 的调用
- 删除了手动保存节点数据到数据库的 goroutine 代码（约50行）

### 4. 更新 `GetFigmaNodeTree` 函数

**文件**: `internal/controllers/figma.go`

在强制刷新时直接使用 `GetFigmaNodesNoCache`：

```go
if forceRefresh {
	// 强制从API获取新数据（跳过缓存）
	nodes, fetchErr = services.GetFigmaNodesNoCache(user.FigmaToken, project.FileKey, project.RootNodeID)
} else {
	// 使用缓存或者获取新数据
	nodes, fetchErr = services.GetFigmaNodes(user.FigmaToken, project.FileKey, project.RootNodeID)
}
```

**移除的代码**：
- 删除了手动删除缓存文件的代码（约10行）

### 5. 删除 `deleteCacheFile` 函数

**文件**: `internal/controllers/cache.go`

完全删除了 `deleteCacheFile` 函数（约80行），因为：
1. 不再需要手动删除缓存文件
2. `GetFigmaNodesNoCache` 会自动覆盖缓存
3. 减少了代码复杂度和维护成本

**移除的导入**：
```go
// 不再需要
"os"
"path/filepath"
```

## 优势

### 1. 代码复用
- Figma API 调用逻辑集中在一个函数中
- 避免了重复的缓存保存代码

### 2. 简化维护
- `RefreshProjectNodeTree` 从约80行减少到约30行
- 删除了 `deleteCacheFile` 函数（约80行）
- 总共减少了约130行代码

### 3. 更清晰的职责分离
- `GetFigmaNodes`: 处理缓存读取和 API 调用
- `GetFigmaNodesNoCache`: 直接调用 API 并保存缓存
- `RefreshProjectNodeTree`: 业务逻辑，无需关心缓存细节

### 4. 自动化缓存管理
- 调用 `GetFigmaNodesNoCache` 自动保存到本地文件和数据库
- 无需手动管理缓存文件的删除和创建
- 减少了缓存不一致的风险

### 5. 更好的性能
- 减少了文件系统操作（不再需要删除文件）
- 异步保存到数据库，不阻塞响应
- 缓存覆盖比删除+创建更高效

## 调用关系

```
GetFigmaNodes (带缓存)
├── 检查本地文件缓存
├── 检查数据库缓存
└── GetFigmaNodesNoCache (无缓存)
    ├── 调用 Figma API
    ├── 保存到本地文件缓存
    └── 异步保存到数据库缓存

RefreshProjectNodeTree
└── 直接调用 GetFigmaNodesNoCache
    (自动覆盖旧缓存)

GetFigmaNodeTree (forceRefresh=true)
└── 直接调用 GetFigmaNodesNoCache
    (自动覆盖旧缓存)
```

## 测试

编译测试通过：
```bash
cd D:\figma-deliver\service
go build
# Exit code: 0
```

## 相关文件

### 修改的文件
1. `internal/services/figma.go`
   - 新增：`GetFigmaNodesNoCache` 函数
   - 修改：`GetFigmaNodes` 函数，简化逻辑

2. `internal/controllers/cache.go`
   - 修改：`RefreshProjectNodeTree` 函数
   - 删除：`deleteCacheFile` 函数
   - 移除：`os` 和 `path/filepath` 导入

3. `internal/controllers/figma.go`
   - 修改：`GetFigmaNodeTree` 函数（forceRefresh 分支）

### 依赖的功能
- `models.GetFileCache()`: 从数据库读取缓存
- `models.CreateFileCache()`: 创建数据库缓存记录
- `models.UpdateFileCache()`: 更新数据库缓存记录
- `models.ExtractAllNodeIDs()`: 提取节点树中的所有节点 ID
- `cacheNodes()`: 保存节点数据到本地文件
- `loadCachedNodes()`: 从本地文件加载缓存

## 后续优化建议

1. **添加缓存失效策略**
   - 可以在 `GetFigmaNodesNoCache` 中添加 `file_version` 参数
   - 当检测到文件版本变化时自动更新缓存

2. **增强错误处理**
   - 在数据库缓存保存失败时，添加重试机制
   - 记录缓存保存失败的详细日志

3. **性能监控**
   - 添加缓存命中率统计
   - 监控 API 调用频率和响应时间

4. **缓存清理机制**
   - 实现定期清理过期的本地文件缓存
   - 在数据库中添加缓存过期时间字段

## 总结

本次重构通过提取 `GetFigmaNodesNoCache` 函数，大幅简化了代码结构，提高了代码可维护性和复用性。同时删除了不必要的 `deleteCacheFile` 函数，减少了约130行代码，降低了维护成本。

新的实现方式更加直观和高效，缓存管理完全自动化，减少了人为错误的可能性。

