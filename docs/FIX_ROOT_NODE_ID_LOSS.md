# 修复根节点 ID 丢失问题

## 📋 问题描述

在 `GetFigmaNodeTree` 函数中，通过 WebSocket 从 Figma 插件获取节点树数据时，根节点的 ID 被硬编码为 `"0:0"`，导致原始节点 ID（如 `"46:1384"`）丢失。

## 🔍 问题根源

### 插件返回的数据格式

Figma 插件通过 `read_my_design` 命令返回的节点数据格式如下：

```json
{
  "document": {
    "absoluteBoundingBox": {
      "height": 1334,
      "width": 750,
      "x": -1780,
      "y": 1051
    },
    "children": [
      // 子节点...
    ],
    "id": "46:1384",
    "name": "技能说明",
    "type": "FRAME"
  },
  "nodeId": "46:1384"
}
```

**关键字段**：
- `document.id`: 节点本身的 ID（在 document 对象内部）
- `nodeId`: 外层的节点 ID（顶层字段）

### 旧代码的问题

在 `internal/services/figma.go` 的 `parseFigmaNodes` 函数中：

```go
func parseFigmaNodes(data map[string]interface{}, _ /*rootNodeID*/ string) []map[string]interface{} {
    // ...
    if document, ok := data["document"].(map[string]interface{}); ok {
        documentCopy := make(map[string]interface{})
        for k, v := range document {
            documentCopy[k] = v
        }
        documentCopy["id"] = "0:0" // ❌ 问题：硬编码为 "0:0"
        documentCopy["parent_id"] = nil
        nodes = append(nodes, documentCopy)
        // ...
    }
}
```

**问题点**：
- 第 912 行将根节点 ID 硬编码为 `"0:0"`
- 这覆盖了 `document` 中原有的 `id` 字段
- 导致原始节点 ID（如 `"46:1384"`）丢失

## ✅ 解决方案

### 修改策略

采用分层优先级策略来获取根节点 ID：

1. **优先级 1**：使用 `document.id`（如果存在且非空）
2. **优先级 2**：使用外层的 `nodeId`（如果存在且非空）
3. **优先级 3**：使用默认值 `"0:0"`（兼容旧数据）

### 修复后的代码

```go
func parseFigmaNodes(data map[string]interface{}, _ /*rootNodeID*/ string) []map[string]interface{} {
    var nodes []map[string]interface{}

    // 如果是节点查询（插件返回的格式：{ "document": {...}, "nodeId": "xxx" }）
    if document, ok := data["document"].(map[string]interface{}); ok {
        // 处理根节点 - 直接使用完整节点数据
        documentCopy := make(map[string]interface{})
        for k, v := range document {
            documentCopy[k] = v
        }
        
        // ✅ 优先使用 document 中的 id，如果不存在则使用外层的 nodeId
        // 这样可以保留原始节点ID（如 "46:1384"）
        var rootNodeID string
        if docID, hasID := document["id"].(string); hasID && docID != "" {
            rootNodeID = docID
        } else if nodeID, hasNodeID := data["nodeId"].(string); hasNodeID && nodeID != "" {
            rootNodeID = nodeID
            documentCopy["id"] = nodeID // 补充 id 字段
        } else {
            rootNodeID = "0:0" // 兼容旧数据：如果都没有则使用默认值
            documentCopy["id"] = "0:0"
        }
        
        documentCopy["parent_id"] = nil
        nodes = append(nodes, documentCopy)

        // 处理子节点
        if children, ok := document["children"].([]interface{}); ok {
            for _, child := range children {
                childNodes := parseNodeRecursive(child.(map[string]interface{}), rootNodeID)
                nodes = append(nodes, childNodes...)
            }
        }
    } else if nodeMap, ok := data["nodes"].(map[string]interface{}); ok {
        // ... 其他情况的处理
    }

    return nodes
}
```

## 🔄 兼容性

### 支持的数据格式

**格式 1：完整格式（推荐）**
```json
{
  "document": {
    "id": "46:1384",
    "name": "技能说明",
    "type": "FRAME",
    "children": [...]
  },
  "nodeId": "46:1384"
}
```
- ✅ 使用 `document.id` = `"46:1384"`

**格式 2：仅外层 nodeId**
```json
{
  "document": {
    "name": "技能说明",
    "type": "FRAME",
    "children": [...]
  },
  "nodeId": "46:1384"
}
```
- ✅ 使用 `nodeId` = `"46:1384"`，并补充到 `document.id`

**格式 3：旧格式（兼容）**
```json
{
  "document": {
    "name": "技能说明",
    "type": "FRAME",
    "children": [...]
  }
}
```
- ✅ 使用默认值 `"0:0"`（向后兼容）

## 🧪 测试场景

### 场景 1：正常数据（document.id 存在）

**输入**：
```json
{
  "document": {
    "id": "46:1384",
    "name": "技能说明"
  },
  "nodeId": "46:1384"
}
```

**预期输出**：
```go
nodes[0]["id"] == "46:1384"  // ✅ 保留原始 ID
```

### 场景 2：document.id 缺失

**输入**：
```json
{
  "document": {
    "name": "技能说明"
  },
  "nodeId": "46:1384"
}
```

**预期输出**：
```go
nodes[0]["id"] == "46:1384"  // ✅ 使用外层 nodeId
```

### 场景 3：两者都缺失（旧数据）

**输入**：
```json
{
  "document": {
    "name": "技能说明"
  }
}
```

**预期输出**：
```go
nodes[0]["id"] == "0:0"  // ✅ 使用默认值（兼容旧数据）
```

### 场景 4：ID 不一致（异常情况）

**输入**：
```json
{
  "document": {
    "id": "46:1384",
    "name": "技能说明"
  },
  "nodeId": "46:9999"
}
```

**预期输出**：
```go
nodes[0]["id"] == "46:1384"  // ✅ 优先使用 document.id
```

## 📊 影响范围

### 受影响的函数

1. **`parseFigmaNodes`** (`internal/services/figma.go`)
   - ✅ 直接修复：根节点 ID 获取逻辑

2. **`GetFigmaNodeTree`** (`internal/controllers/figma.go`)
   - ✅ 间接受益：通过 WebSocket 获取节点树时，根节点 ID 正确

3. **节点树缓存**
   - ✅ 缓存的数据包含正确的根节点 ID

### 不受影响的场景

- **`nodes` 格式的数据**：已有正确的处理逻辑（第 923-966 行）
- **Figma API 返回的数据**：通常使用 `nodes` 格式，不走 `document` 分支

## 🎯 验证方法

### 1. 检查日志

在调用 `GetFigmaNodeTree` 后，查看返回的节点数据：

```json
{
  "nodes": [
    {
      "id": "46:1384",  // ✅ 应该是原始 ID，而不是 "0:0"
      "name": "技能说明",
      "type": "FRAME",
      "parent_id": null
    },
    // ... 子节点
  ],
  "cache_source": "plugin"
}
```

### 2. 测试 API

```bash
# 调用获取节点树接口
GET /api/figma/projects/{project_id}/node-tree

# 检查返回的根节点 ID
curl -X GET "http://localhost:8080/api/figma/projects/1/node-tree" \
  -H "Cookie: session=..." \
  | jq '.nodes[0].id'

# 预期输出：实际的节点 ID（如 "46:1384"），而不是 "0:0"
```

### 3. 检查数据库缓存

```sql
-- 查询缓存的节点数据
SELECT file_key, root_node_id, node_ids 
FROM figma_file_caches 
WHERE file_key = 'your_file_key' 
  AND root_node_id = '46:1384';

-- 应该能找到对应的缓存记录
```

## 🚀 后续优化建议

1. **统一数据格式**
   - 建议 Figma 插件始终在 `document.id` 中返回节点 ID
   - 外层的 `nodeId` 作为冗余字段，确保兼容性

2. **添加数据验证**
   - 如果 `document.id` 和 `nodeId` 都存在但不一致，记录警告日志
   - 帮助发现潜在的数据不一致问题

3. **单元测试**
   - 为 `parseFigmaNodes` 添加单元测试
   - 覆盖所有数据格式场景

## 📝 相关文件

- `internal/services/figma.go` - `parseFigmaNodes()` 函数
- `internal/controllers/figma.go` - `GetFigmaNodeTree()` 函数
- `figma-plugin/code.js` - Figma 插件端实现（`read_my_design` 命令）

## ✅ 总结

通过采用分层优先级策略，修复后的代码能够：
- ✅ 正确保留原始节点 ID（如 `"46:1384"`）
- ✅ 兼容旧数据（使用 `"0:0"` 作为兜底）
- ✅ 支持多种数据格式（`document.id` 和 `nodeId`）
- ✅ 不破坏现有功能（向后兼容）

问题已完全解决！🎉

