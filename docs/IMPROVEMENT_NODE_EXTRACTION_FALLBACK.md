# 节点提取回退机制改进

**日期**: 2025-11-19

## 📋 问题描述

在 `RenderProject` 接口中，当 `figma_file_caches` 表中的 `node_ids` 字段为空时，直接返回错误，导致无法进行渲染。

实际上，即使 `node_ids` 为空，我们仍然可以从 `file_data` 字段（完整的节点树 JSON 数据）中提取所有节点ID。

## ✅ 解决方案

### 1. 改进节点提取逻辑

在 `internal/controllers/cache.go` 的 `RenderProject` 函数中，增加了回退机制：

```go
// 解析 node_ids 字段（逗号分隔的节点ID列表）
if cache.NodeIDs != "" {
    // 优先使用预提取的 node_ids 字段（性能最优）
    nodeIDList := strings.Split(cache.NodeIDs, ",")
    for _, nodeID := range nodeIDList {
        nodeID = strings.TrimSpace(nodeID)
        if nodeID != "" {
            nodeIDs = append(nodeIDs, nodeID)
        }
    }
    log.Printf("✅ [RenderProject] 从 node_ids 字段提取到 %d 个节点", len(nodeIDs))
} else if cache.FileData != "" {
    // 如果 node_ids 为空，尝试从 file_data 中提取（回退方案）
    log.Printf("⚠️ [RenderProject] node_ids 为空，尝试从 file_data 中提取节点")
    
    extractedIDs, err := models.ExtractAllNodeIDs(cache.FileData)
    if err != nil {
        // 提取失败，返回错误
        return
    }
    
    nodeIDs = extractedIDs
    log.Printf("✅ [RenderProject] 从 file_data 提取到 %d 个节点", len(nodeIDs))
    
    // 异步更新 cache.NodeIDs 字段，便于下次快速访问
    go func() {
        nodeIDsStr := strings.Join(extractedIDs, ",")
        cache.NodeIDs = nodeIDsStr
        models.UpdateFileCache(cache)
        log.Printf("✅ [RenderProject] 已更新 node_ids 缓存 (节点数=%d)", len(extractedIDs))
    }()
} else {
    // file_data 也为空，无法提取节点
    return error
}
```

### 2. 优势

#### 性能优化
- **优先使用 `node_ids` 字段**：如果已经预提取并存储，直接使用，性能最优（字符串分割）
- **回退到 `file_data` 解析**：如果 `node_ids` 为空，从完整 JSON 中递归提取（性能稍低，但能正常工作）
- **异步更新缓存**：提取成功后，异步更新 `node_ids` 字段，下次访问时直接使用

#### 容错性
- 兼容旧数据：对于早期创建的缓存记录（可能 `node_ids` 为空），仍然能正常工作
- 自动修复：首次从 `file_data` 提取后，会自动填充 `node_ids` 字段

## 📝 涉及文件

### 修改的文件
- `internal/controllers/cache.go` (lines 289-333)
  - 修改了 `RenderProject` 函数的节点提取逻辑
  - 添加了从 `file_data` 提取节点的回退机制
  - 添加了异步更新 `node_ids` 缓存的逻辑

### 使用的现有函数
- `models.ExtractAllNodeIDs(fileData string) ([]string, error)`
  - 位于 `internal/models/cache.go` (lines 541-551)
  - 递归解析 Figma 节点树 JSON，提取所有节点ID
  - 处理 Figma 文档结构的特殊字段（`document`, `children` 等）

## 🔄 工作流程

### 正常情况（node_ids 已预提取）
1. 从数据库查询 `figma_file_caches`
2. 读取 `node_ids` 字段（逗号分隔的字符串）
3. 分割字符串，得到节点ID数组
4. ✅ 继续后续渲染流程

### 回退情况（node_ids 为空）
1. 从数据库查询 `figma_file_caches`
2. 发现 `node_ids` 为空
3. 读取 `file_data` 字段（完整 JSON）
4. 调用 `models.ExtractAllNodeIDs()` 递归提取所有节点ID
5. ✅ 继续后续渲染流程
6. 🔄 **异步更新**：将提取的节点ID列表保存到 `node_ids` 字段

### 失败情况（数据完全为空）
1. 从数据库查询 `figma_file_caches`
2. 发现 `node_ids` 和 `file_data` 都为空
3. ❌ 返回错误："节点树缓存数据为空，请在 Dashboard 中重新刷新节点树"

## 🎯 效果

- ✅ 提高了系统的容错性
- ✅ 兼容旧的缓存数据
- ✅ 自动修复不完整的缓存记录
- ✅ 性能优先：优先使用预提取的 `node_ids`
- ✅ 降低用户操作成本：不需要手动重新刷新节点树

## 📊 日志示例

### 场景1：使用预提取的 node_ids
```
📋 [RenderProject] 开始提取项目节点: ProjectID=123, FileKey=xxx, RootNodeID=0:1
✅ [RenderProject] 从 node_ids 字段提取到 45 个节点
```

### 场景2：回退到 file_data 提取
```
📋 [RenderProject] 开始提取项目节点: ProjectID=123, FileKey=xxx, RootNodeID=0:1
⚠️ [RenderProject] node_ids 为空，尝试从 file_data 中提取节点 (FileKey=xxx, RootNodeID=0:1)
✅ [RenderProject] 从 file_data 提取到 45 个节点
✅ [RenderProject] 已更新 node_ids 缓存 (CacheID=456, 节点数=45)
```

### 场景3：数据完全为空
```
📋 [RenderProject] 开始提取项目节点: ProjectID=123, FileKey=xxx, RootNodeID=0:1
❌ [RenderProject] 节点树缓存数据为空 (FileKey=xxx, RootNodeID=0:1, CacheID=456, Status=loaded)
```

## 🔗 相关功能

- `RefreshProjectNodeTree` (cache.go): 刷新节点树时会同时保存 `file_data` 和 `node_ids`
- `GetFigmaNodesNoCache` (figma.go): 从 Figma API 获取节点树时也会保存到数据库
- `ExtractAllNodeIDs` (cache.go): 通用的节点ID提取函数

## ✅ 测试结果

- ✅ 编译通过 (`go build`)
- ✅ 代码逻辑验证通过
- ✅ 日志输出完整清晰

