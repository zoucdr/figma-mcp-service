# 修复 GetFigmaNodeTree 数据格式不一致问题

## 问题描述

在 `GetFigmaNodeTree` 函数中，存在数据格式不一致的问题：
- ✅ 从文件缓存获取的数据（通过 `LoadCachedNodesFromFile`）格式正确
- ❌ 从数据库缓存获取的数据（通过 `GetFileCache` 或 `GetFileCacheContainingNode`）格式和层级不一样

## 根本原因

两种缓存途径使用了不同的数据解析逻辑：

### 文件缓存路径（正确）
```go
// internal/services/figma.go
LoadCachedNodesFromFile() 
  → loadCachedNodes() 
  → parseFigmaNodes()  // ✅ 返回扁平化的节点列表
```

### 数据库缓存路径（错误）
```go
// internal/controllers/figma.go (修复前)
GetFileCache() 
  → 直接解析 JSON
  → 手动提取 document/nodes  // ❌ 未使用统一的解析逻辑
```

## 解决方案

### 1. 统一数据解析逻辑

在 `internal/services/figma.go` 中导出新函数：

```go
// ParseFigmaNodesFromData 解析Figma节点数据（导出给 controller 使用）
// 这个函数提供与 LoadCachedNodesFromFile 相同的数据格式转换
func ParseFigmaNodesFromData(data map[string]interface{}, rootNodeID string) ([]map[string]interface{}, error) {
	nodes := parseFigmaNodes(data, rootNodeID)
	if len(nodes) == 0 {
		return nil, fmt.Errorf("未能解析出有效的节点数据")
	}
	return nodes, nil
}
```

### 2. 更新 GetFigmaNodeTree 函数

在 `internal/controllers/figma.go` 中更新数据库缓存处理逻辑：

**修复前（错误）：**
```go
// 解析节点数据
if nodesData, ok := result["nodes"].(map[string]interface{}); ok {
	// 多节点格式
	nodes = make([]map[string]interface{}, 0)
	for _, nodeData := range nodesData {
		if nodeMap, ok := nodeData.(map[string]interface{}); ok {
			if document, ok := nodeMap["document"].(map[string]interface{}); ok {
				nodes = append(nodes, document)
			}
		}
	}
} else if document, ok := result["document"].(map[string]interface{}); ok {
	// 单节点格式
	nodes = []map[string]interface{}{document}
}
```

**修复后（正确）：**
```go
// 使用与 LoadCachedNodesFromFile 相同的 parseFigmaNodes 函数
dbNodes, err := services.ParseFigmaNodesFromData(result, project.RootNodeID)
if err != nil {
	log.Printf("❌ [GetFigmaNodeTree] 解析节点数据失败: %v", err)
	needAPIFallback = true
} else if len(dbNodes) == 0 {
	log.Printf("⚠️ [GetFigmaNodeTree] 数据库缓存解析后无有效节点")
	needAPIFallback = true
} else {
	nodes = dbNodes
	cacheSource = "database"
	log.Printf("✅ [GetFigmaNodeTree] 从数据库缓存获取节点树 (FileKey=%s, NodeID=%s, 节点数=%d)",
		project.FileKey, project.RootNodeID, len(nodes))
}
```

### 3. 改进错误处理

将数据库缓存解析失败的情况视为可恢复错误，自动回退到 API 获取：

```go
if !needAPIFallback {
	// 解析 JSON 数据
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(fileDataToUse), &result); err != nil {
		log.Printf("❌ [GetFigmaNodeTree] 解析数据库缓存JSON失败: %v", err)
		needAPIFallback = true  // 标记需要API兜底而不是直接返回错误
	} else {
		// ... 解析节点数据
	}
}
```

## 数据格式说明

### parseFigmaNodes 返回的格式

返回一个扁平化的节点列表，每个节点包含完整的 Figma 节点属性：

```go
[]map[string]interface{}{
	{
		"id": "1:2",
		"name": "Frame 1",
		"type": "FRAME",
		"parent_id": "0:0",
		"visible": true,
		"absoluteBoundingBox": {...},
		"children": [...],
		// ... 其他 Figma 节点属性
	},
	{
		"id": "1:3",
		"name": "Text",
		"type": "TEXT",
		"parent_id": "1:2",
		// ...
	},
	// ...
}
```

### 特点

1. **扁平化结构**：所有节点在同一层级的数组中
2. **包含 parent_id**：通过 `parent_id` 字段维护树形关系
3. **完整属性**：保留所有 Figma 节点原始属性
4. **递归展开**：子节点被递归提取并添加到列表中

## 测试验证

修复后，需要验证以下场景：

1. ✅ 文件缓存命中时，数据格式正确
2. ✅ 数据库缓存命中时（精确匹配），数据格式与文件缓存一致
3. ✅ 数据库缓存命中时（父树提取子树），数据格式与文件缓存一致
4. ✅ 所有缓存未命中时，API兜底正常工作
5. ✅ 数据库缓存解析失败时，能够正确回退到API兜底

## 影响范围

- ✅ 不影响文件缓存路径（已经正确）
- ✅ 修复数据库缓存路径的格式问题
- ✅ 不改变对外API接口
- ✅ 保持与前端的兼容性

## 相关文件

- `internal/controllers/figma.go` - GetFigmaNodeTree 函数
- `internal/services/figma.go` - ParseFigmaNodesFromData 函数（新增）
- `internal/services/figma.go` - parseFigmaNodes 函数（内部使用）

