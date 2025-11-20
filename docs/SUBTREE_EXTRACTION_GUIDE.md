# Figma 子树提取实现指南

## 概述

本文档说明如何实现从已缓存的完整树中提取子树的功能，这是一个重要的性能优化点。

## 核心思想

当客户端请求某个节点的子树时，不需要每次都调用 Figma API。如果该节点已经包含在某个缓存的父树中，可以直接从缓存中提取出对应的子树返回。

### 优势

1. ✅ **减少 API 调用**: 避免重复请求已缓存的数据
2. ✅ **提高响应速度**: 从内存/数据库提取比 API 请求快得多
3. ✅ **降低成本**: 减少 Figma API 的配额消耗
4. ✅ **避免速率限制**: 不会触发 30 秒冷却

---

## 数据库设计

### node_ids 字段的作用

```sql
`node_ids` TEXT COMMENT '该树包含的所有节点 ID 列表（逗号分隔，用于子树查找）'
```

**示例值**:
```
"0:0,1:2,1:3,1:4,2:5,2:6,2:7,3:8,3:9"
```

这表示该缓存包含了这些节点ID的完整树结构。

### 查询策略

#### 步骤1: 精确匹配

```sql
-- 查找精确匹配的缓存
SELECT * FROM figma_file_caches
WHERE file_key = 'abc123'
  AND root_node_id = '1:3'  -- 请求的节点ID
  AND status = 'loaded';
```

#### 步骤2: 子树查找（关键优化）

如果精确匹配失败，查找包含该节点的父树：

```sql
-- 方法1：使用 FIND_IN_SET（适用于逗号分隔）
SELECT * FROM figma_file_caches
WHERE file_key = 'abc123'
  AND status = 'loaded'
  AND FIND_IN_SET('1:3', node_ids) > 0
ORDER BY LENGTH(node_ids) ASC  -- 优先返回最小的包含树
LIMIT 1;

-- 方法2：使用 LIKE（需要注意边界）
SELECT * FROM figma_file_caches
WHERE file_key = 'abc123'
  AND status = 'loaded'
  AND (node_ids LIKE '%,1:3,%' 
       OR node_ids LIKE '1:3,%' 
       OR node_ids LIKE '%,1:3' 
       OR node_ids = '1:3')
ORDER BY LENGTH(node_ids) ASC
LIMIT 1;
```

---

## Go 代码实现

### 1. 数据模型辅助方法

在 `internal/models/cache.go` 中添加：

```go
// ExtractAllNodeIDs 从 Figma 文件树 JSON 中提取所有节点ID
func ExtractAllNodeIDs(fileData string) ([]string, error) {
	var data map[string]interface{}
	err := json.Unmarshal([]byte(fileData), &data)
	if err != nil {
		return nil, err
	}

	nodeIDs := make([]string, 0)
	extractNodeIDsRecursive(data, &nodeIDs)
	return nodeIDs, nil
}

// extractNodeIDsRecursive 递归提取节点ID
func extractNodeIDsRecursive(node interface{}, nodeIDs *[]string) {
	nodeMap, ok := node.(map[string]interface{})
	if !ok {
		return
	}

	// 提取当前节点ID
	if id, exists := nodeMap["id"]; exists {
		if idStr, ok := id.(string); ok {
			*nodeIDs = append(*nodeIDs, idStr)
		}
	}

	// 递归处理子节点
	if children, exists := nodeMap["children"]; exists {
		if childrenArray, ok := children.([]interface{}); ok {
			for _, child := range childrenArray {
				extractNodeIDsRecursive(child, nodeIDs)
			}
		}
	}

	// 处理 Figma 文档结构的特殊字段
	if document, exists := nodeMap["document"]; exists {
		extractNodeIDsRecursive(document, nodeIDs)
	}
}

// ExtractSubtree 从完整树中提取子树
func ExtractSubtree(fileData string, targetNodeID string) (string, error) {
	var data map[string]interface{}
	err := json.Unmarshal([]byte(fileData), &data)
	if err != nil {
		return "", err
	}

	// 查找目标节点
	subtree := findNodeInTree(data, targetNodeID)
	if subtree == nil {
		return "", fmt.Errorf("节点 %s 不存在于树中", targetNodeID)
	}

	// 将子树转换为 JSON
	subtreeJSON, err := json.Marshal(subtree)
	if err != nil {
		return "", err
	}

	return string(subtreeJSON), nil
}

// findNodeInTree 在树中查找指定节点
func findNodeInTree(node interface{}, targetNodeID string) interface{} {
	nodeMap, ok := node.(map[string]interface{})
	if !ok {
		return nil
	}

	// 检查当前节点
	if id, exists := nodeMap["id"]; exists {
		if idStr, ok := id.(string); ok && idStr == targetNodeID {
			return node
		}
	}

	// 在子节点中查找
	if children, exists := nodeMap["children"]; exists {
		if childrenArray, ok := children.([]interface{}); ok {
			for _, child := range childrenArray {
				result := findNodeInTree(child, targetNodeID)
				if result != nil {
					return result
				}
			}
		}
	}

	// 在 document 字段中查找
	if document, exists := nodeMap["document"]; exists {
		return findNodeInTree(document, targetNodeID)
	}

	return nil
}

// ContainsNodeID 检查 node_ids 字符串是否包含指定节点ID
func ContainsNodeID(nodeIDs, targetNodeID string) bool {
	if nodeIDs == "" {
		return false
	}
	
	// 精确匹配，避免部分匹配问题
	// 例如 "1:123" 不应匹配 "1:1234"
	ids := strings.Split(nodeIDs, ",")
	for _, id := range ids {
		if strings.TrimSpace(id) == targetNodeID {
			return true
		}
	}
	return false
}

// GetFileCacheContainingNode 获取包含指定节点的文件缓存
func GetFileCacheContainingNode(fileKey, nodeID string) (*FigmaFileCache, error) {
	var cache FigmaFileCache
	
	// 先尝试精确匹配
	err := DB.Where("file_key = ? AND root_node_id = ? AND status = ?", 
		fileKey, nodeID, "loaded").First(&cache).Error
	
	if err == nil {
		return &cache, nil
	}
	
	// 如果精确匹配失败，查找包含该节点的父树
	var caches []FigmaFileCache
	err = DB.Where("file_key = ? AND status = ?", fileKey, "loaded").
		Order("LENGTH(node_ids) ASC").  // 优先返回最小的包含树
		Find(&caches).Error
	
	if err != nil {
		return nil, err
	}
	
	// 检查哪个缓存包含目标节点
	for _, c := range caches {
		if ContainsNodeID(c.NodeIDs, nodeID) {
			return &c, nil
		}
	}
	
	return nil, gorm.ErrRecordNotFound
}
```

### 2. 服务层实现

在 `internal/services/cache.go` 中添加：

```go
// GetFileCacheWithSubtreeExtraction 获取文件缓存，支持子树提取
func (cs *CacheService) GetFileCacheWithSubtreeExtraction(fileKey, nodeID string) (*models.FigmaFileCache, string, error) {
	// 查找包含该节点的缓存
	cache, err := models.GetFileCacheContainingNode(fileKey, nodeID)
	if err != nil {
		return nil, "", err
	}
	
	// 更新命中次数
	cache.HitCount++
	models.UpdateFileCache(cache)
	
	// 如果是精确匹配，直接返回
	if cache.RootNodeID == nodeID {
		log.Printf("✅ [Cache] 精确匹配命中: file=%s, node=%s", fileKey, nodeID)
		return cache, cache.FileData, nil
	}
	
	// 需要提取子树
	log.Printf("🔍 [Cache] 从父树提取子树: file=%s, node=%s, parent_root=%s", 
		fileKey, nodeID, cache.RootNodeID)
	
	subtreeData, err := models.ExtractSubtree(cache.FileData, nodeID)
	if err != nil {
		log.Printf("❌ [Cache] 子树提取失败: %v", err)
		return nil, "", err
	}
	
	log.Printf("✅ [Cache] 子树提取成功: %d bytes", len(subtreeData))
	
	// 可选：保存提取的子树为新的缓存记录（加速后续查询）
	// go cs.saveExtractedSubtreeCache(fileKey, nodeID, subtreeData)
	
	return cache, subtreeData, nil
}

// saveExtractedSubtreeCache 保存提取的子树为新缓存（可选）
func (cs *CacheService) saveExtractedSubtreeCache(fileKey, nodeID, subtreeData string) {
	// 提取子树的所有节点ID
	nodeIDs, err := models.ExtractAllNodeIDs(subtreeData)
	if err != nil {
		log.Printf("⚠️ [Cache] 提取节点ID失败: %v", err)
		return
	}
	
	// 创建新的缓存记录
	now := uint32(time.Now().Unix())
	cache := &models.FigmaFileCache{
		FileKey:     fileKey,
		RootNodeID:  nodeID,
		NodeIDs:     strings.Join(nodeIDs, ","),
		FileData:    subtreeData,
		Status:      "loaded",
		HitCount:    0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	
	err = models.CreateFileCache(cache)
	if err != nil {
		log.Printf("⚠️ [Cache] 保存子树缓存失败: %v", err)
	} else {
		log.Printf("✅ [Cache] 子树缓存已保存: node=%s, node_count=%d", nodeID, len(nodeIDs))
	}
}
```

### 3. 控制器层实现

在 `internal/controllers/cache.go` 中使用：

```go
// GetFileTree 获取文件树（支持子树提取）
func GetFileTree(c *gin.Context) {
	fileKey := c.Query("file_key")
	nodeID := c.DefaultQuery("node_id", "")  // 空表示整个文件
	
	if fileKey == "" {
		c.JSON(400, gin.H{"error": "file_key is required"})
		return
	}
	
	// 使用支持子树提取的查询方法
	cache, fileData, err := cacheService.GetFileCacheWithSubtreeExtraction(fileKey, nodeID)
	
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 缓存未找到，需要请求 Figma API
			c.JSON(404, gin.H{
				"error": "cache not found",
				"message": "请求已加入队列，请稍后重试",
			})
			return
		}
		
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	
	// 返回文件树数据
	c.Header("Content-Type", "application/json")
	c.String(200, fileData)
}
```

---

## 性能优化建议

### 1. 索引优化

虽然 `node_ids` 是 TEXT 字段，无法直接建索引，但可以：

```sql
-- 为 file_key + status 建立复合索引（已有）
INDEX `idx_file_status` (`file_key`, `status`)
```

### 2. 缓存策略

```go
// 策略1：懒加载子树缓存
// 只在首次提取时保存，避免冗余

// 策略2：热点子树预缓存
// 统计 hit_count，为高频访问的子树预先创建缓存

// 策略3：定期清理冷数据
// 清理 hit_count = 0 且创建时间超过 7 天的子树缓存
```

### 3. 内存缓存

```go
// 使用 go-cache 或 redis 缓存热门子树
var memCache = cache.New(5*time.Minute, 10*time.Minute)

func GetFileCacheWithMemCache(fileKey, nodeID string) (string, error) {
	cacheKey := fmt.Sprintf("%s:%s", fileKey, nodeID)
	
	// 先查内存缓存
	if data, found := memCache.Get(cacheKey); found {
		return data.(string), nil
	}
	
	// 查数据库
	_, fileData, err := cacheService.GetFileCacheWithSubtreeExtraction(fileKey, nodeID)
	if err != nil {
		return "", err
	}
	
	// 存入内存缓存
	memCache.Set(cacheKey, fileData, cache.DefaultExpiration)
	
	return fileData, nil
}
```

---

## 测试用例

### 测试场景1: 精确匹配

```go
func TestExactMatch(t *testing.T) {
	// 缓存中存在 root_node_id = "1:123" 的记录
	cache, data, err := GetFileCacheWithSubtreeExtraction("abc123", "1:123")
	assert.NoError(t, err)
	assert.Equal(t, "1:123", cache.RootNodeID)
	assert.NotEmpty(t, data)
}
```

### 测试场景2: 子树提取

```go
func TestSubtreeExtraction(t *testing.T) {
	// 缓存中只有 root_node_id = "" (整个文件)
	// 但 node_ids 包含 "1:123,1:124,2:456"
	cache, data, err := GetFileCacheWithSubtreeExtraction("abc123", "1:123")
	assert.NoError(t, err)
	assert.Equal(t, "", cache.RootNodeID)  // 来自完整文件树
	assert.Contains(t, data, `"id":"1:123"`)  // 提取的子树
}
```

### 测试场景3: 缓存未找到

```go
func TestCacheNotFound(t *testing.T) {
	_, _, err := GetFileCacheWithSubtreeExtraction("abc123", "9:999")
	assert.Error(t, err)
	assert.Equal(t, gorm.ErrRecordNotFound, err)
}
```

---

## 日志示例

### 精确匹配

```
✅ [Cache] 精确匹配命中: file=abc123, node=1:123
```

### 子树提取

```
🔍 [Cache] 从父树提取子树: file=abc123, node=1:123, parent_root=
✅ [Cache] 子树提取成功: 25648 bytes
✅ [Cache] 子树缓存已保存: node=1:123, node_count=15
```

### 缓存未命中

```
❌ [Cache] 缓存未找到: file=abc123, node=9:999
📝 [Cache] 已加入刷新队列
```

---

## 最佳实践

### 1. 节点ID提取时机

```go
// 在保存 Figma API 响应时立即提取
func SaveFigmaFileResponse(fileKey, nodeID string, response []byte) {
	// 提取所有节点ID
	nodeIDs, _ := ExtractAllNodeIDs(string(response))
	
	cache := &FigmaFileCache{
		FileKey:    fileKey,
		RootNodeID: nodeID,
		NodeIDs:    strings.Join(nodeIDs, ","),  // ⚠️ 立即保存
		FileData:   string(response),
		Status:     "loaded",
	}
	
	CreateFileCache(cache)
}
```

### 2. 错误处理

```go
// 提取失败时降级到 API 请求
cache, data, err := GetFileCacheWithSubtreeExtraction(fileKey, nodeID)
if err != nil {
	if err == gorm.ErrRecordNotFound {
		// 未找到缓存，请求 API
		return RequestFigmaAPI(fileKey, nodeID)
	}
	// 提取失败，也请求 API
	return RequestFigmaAPI(fileKey, nodeID)
}
```

### 3. 监控指标

```go
// 统计子树提取的性能
type CacheMetrics struct {
	ExactMatchCount   int64
	SubtreeExtractCount int64
	ExtractionTimeMs  int64
	APIMissCount      int64
}

// 在每次查询后记录
func RecordCacheMetrics(matchType string, duration time.Duration) {
	// 发送到 Prometheus/Grafana
}
```

---

## 总结

子树提取功能是一个重要的性能优化：

1. ✅ **减少 API 调用** - 从缓存中提取而不是重新请求
2. ✅ **提高响应速度** - 子树提取比 API 请求快 10-100 倍
3. ✅ **降低成本** - 减少 Figma API 配额消耗
4. ✅ **灵活缓存** - 支持完整文件树和子树的灵活缓存策略

实现关键点：
- `node_ids` 字段存储所有节点ID
- 三级查找策略：精确匹配 → 子树查找 → API请求
- JSON 递归解析和子树提取
- 可选的子树缓存持久化

---

**下一步**: 实现节点树刷新 API 时，确保调用 `ExtractAllNodeIDs` 提取并保存 `node_ids`。

