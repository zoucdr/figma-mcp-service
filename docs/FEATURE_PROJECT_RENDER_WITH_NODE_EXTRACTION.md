# 项目渲染功能完善：节点提取与队列合并

## 概述

完善了项目渲染功能，实现了自动从数据库缓存提取节点树、依赖节点合并，以及队列合并时正确更新 `project_ids`。

## 实现的功能

### 1. Dashboard 使用项目渲染接口

**前端改动**：`web/static/js/dashboard.js`

#### 之前的实现
```javascript
// 1. 调用 /figma/project/:id/render_nodes 获取节点列表
// 2. 调用 /figma/manual/render 创建渲染队列
// 3. 手动传递 project_ids
```

**问题**：
- 需要两次 API 调用
- 手动管理节点提取
- 依赖节点需要单独处理

#### 现在的实现
```javascript
// 直接调用项目渲染接口
const renderResponse = await axios.post('/figma/project/render', {
    project_id: this.project.id,
    format: this.defaultImageFormat,
    scale: this.defaultImageScale,
    include_ref_nodes: true  // 自动包含依赖节点
});
```

**优势**：
- ✅ 单次 API 调用
- ✅ 自动提取节点树
- ✅ 自动合并依赖节点
- ✅ 自动更新 `project.render_id`
- ✅ 返回预计渲染时间

### 2. 后端节点提取实现

**文件**：`internal/controllers/cache.go` → `RenderProject` 函数

#### 节点树提取逻辑

```go
// 1. 从数据库缓存中提取项目节点树的所有节点ID
cache, err := models.GetFileCache(project.FileKey, project.RootNodeID)
if err != nil || cache == nil || cache.NodeIDs == "" {
    return "未找到节点树缓存，请先在 Dashboard 中刷新节点树"
}

// 解析 node_ids 字段（逗号分隔的节点ID列表）
nodeIDList := strings.Split(cache.NodeIDs, ",")
for _, nodeID := range nodeIDList {
    nodeID = strings.TrimSpace(nodeID)
    if nodeID != "" {
        nodeIDs = append(nodeIDs, nodeID)
    }
}
```

**数据来源**：`figma_file_caches` 表的 `node_ids` 字段

**特点**：
- 从数据库缓存读取（快速）
- 不访问 Figma API
- 包含完整节点树的所有节点

#### 依赖节点合并逻辑

```go
// 2. 如果包含依赖节点，添加 RefNodes
if req.IncludeRefNodes {
    refNodes, err := models.GetRefNodes(req.ProjectID)
    if err == nil && len(refNodes) > 0 {
        // 使用 map 去重
        nodeIDMap := make(map[string]bool)
        for _, nodeID := range nodeIDs {
            nodeIDMap[nodeID] = true
        }
        
        // 添加依赖节点（去重）
        for _, refNode := range refNodes {
            if !nodeIDMap[refNode] {
                nodeIDs = append(nodeIDs, refNode)
            }
        }
    }
}
```

**数据来源**：`figma_projects` 表的 `ref_nodes` 字段

**特点**：
- 自动去重
- 避免重复渲染
- 统计新增节点数量

#### 详细日志

```go
log.Printf("📋 [RenderProject] 开始提取项目节点: ProjectID=%d, FileKey=%s, RootNodeID=%s", ...)
log.Printf("✅ [RenderProject] 从节点树提取到 %d 个节点", len(nodeIDs))
log.Printf("📎 [RenderProject] 添加依赖节点: %d 个", len(refNodes))
log.Printf("✅ [RenderProject] 添加了 %d 个新的依赖节点（去重后）", addedCount)
log.Printf("✅ [RenderProject] 总共需要渲染 %d 个节点", len(nodeIDs))
```

### 3. 队列合并时更新 ProjectIDs

**文件**：`internal/services/queue.go` → `mergeToExistingQueue` 函数

#### 问题

之前当新节点合并到已有队列时，`project_ids` 没有被更新，导致：
- 新项目无法查询到合并的渲染进度
- `project.render_id` 关联丢失

#### 解决方案

修改 `mergeToExistingQueue` 函数签名，新增 `newProjectIDs` 参数：

```go
func (qs *QueueService) mergeToExistingQueue(
    queue *models.FigmaRenderQueue, 
    newNodeIDs []string,
    newProjectIDs []uint,  // 新增参数
) ([]uint, error)
```

#### 合并逻辑

```go
// 1. 解析已有的 projectIDs
var existingProjectIDs []uint
if queue.ProjectIDs != "" {
    json.Unmarshal([]byte(queue.ProjectIDs), &existingProjectIDs)
}

// 2. 去重合并 projectIDs
projectIDMap := make(map[uint]bool)
for _, pid := range existingProjectIDs {
    projectIDMap[pid] = true
}

addedProjectCount := 0
for _, pid := range newProjectIDs {
    if !projectIDMap[pid] {
        existingProjectIDs = append(existingProjectIDs, pid)
        projectIDMap[pid] = true
        addedProjectCount++
    }
}

// 3. 序列化并更新队列
jsonBytes, _ := json.Marshal(existingProjectIDs)
queue.ProjectIDs = string(jsonBytes)
```

#### 更新调用

```go
if matchedQueue != nil {
    // 合并到已有队列（同时合并 projectIDs）
    return qs.mergeToExistingQueue(matchedQueue, nodeIDs, actualProjectIDs)
}
```

#### 详细日志

```go
log.Printf("📎 [Queue] 合并 ProjectIDs: 原有=%v, 新增=%d, 合并后=%v", ...)
log.Printf("✅ [Queue] 已合并到现有队列 id=%d, ProjectIDs=%v", queue.ID, existingProjectIDs)
```

## 数据流程

### 完整渲染流程

```
用户在 Dashboard 点击"渲染"按钮
    ↓
前端：POST /figma/project/render
    {
        project_id: 123,
        format: "png",
        scale: 1.0,
        include_ref_nodes: true
    }
    ↓
后端：RenderProject 处理
    ├─ 1. 从 figma_file_caches 提取节点树 (node_ids 字段)
    ├─ 2. 从 figma_projects 提取依赖节点 (ref_nodes 字段)
    ├─ 3. 合并并去重节点ID
    └─ 4. 调用 CreateRenderQueue(projectIDs=[123])
    ↓
QueueService：CreateRenderQueue
    ├─ 查找是否有可合并的队列
    ├─ 如果有：mergeToExistingQueue(queue, nodeIDs, [123])
    │   ├─ 合并节点ID（去重）
    │   └─ 合并 project_ids（去重）
    └─ 如果没有：createNewQueues(projectIDs=[123])
    ↓
更新 figma_projects.render_id = queue.ID
    ↓
返回响应：
    {
        queue_ids: [456],
        total_nodes: 150,
        estimated_duration_seconds: 320
    }
    ↓
前端显示：渲染队列已创建，共 150 个节点，预计时间约 5 分 20 秒
    ↓
底部状态栏出现，显示实时进度
```

### 队列合并场景

#### 场景 1：首次渲染

```
项目A (id=123) 首次渲染
    ↓
没有可合并的队列
    ↓
创建新队列：
    queue.project_ids = "[123]"
    queue.node_ids = "1:1,1:2,1:3,..."
    ↓
更新: projects[123].render_id = queue.id
```

#### 场景 2：同项目再次渲染

```
项目A (id=123) 再次渲染（可能添加了新节点）
    ↓
找到可合并的队列（相同 token + fileKey + format + scale）
    ↓
合并到现有队列：
    queue.node_ids = "1:1,1:2,1:3,...,1:4,1:5" (新增去重)
    queue.project_ids = "[123]" (已存在，不重复添加)
    ↓
projects[123].render_id 保持不变
```

#### 场景 3：不同项目合并到同一队列

```
项目B (id=456) 渲染（与项目A使用相同的 fileKey）
    ↓
找到可合并的队列（项目A的队列）
    ↓
合并到现有队列：
    queue.node_ids = "合并的节点ID" (去重)
    queue.project_ids = "[123, 456]" (新增 456)
    ↓
更新: projects[456].render_id = queue.id
    ↓
现在两个项目都可以查询到该队列的进度
```

## API 变化

### 请求参数

**之前（ManualRender）**：
```json
{
    "file_key": "xxx",
    "node_ids": ["1:1", "1:2", ...],  // 手动提供
    "project_ids": [123],              // 可选
    "format": "png",
    "scale": 1.0
}
```

**现在（RenderProject）**：
```json
{
    "project_id": 123,                 // 必需
    "format": "png",                   // 必需
    "scale": 1.0,                      // 必需
    "include_ref_nodes": true          // 可选，默认 false
}
```

### 响应数据

**之前**：
```json
{
    "code": 0,
    "message": "已加入渲染队列",
    "data": {
        "queue_ids": [456],
        "total_nodes": 150
    }
}
```

**现在**：
```json
{
    "code": 0,
    "message": "已加入渲染队列",
    "data": {
        "queue_ids": [456],
        "total_nodes": 150,
        "estimated_duration_seconds": 320  // 新增
    }
}
```

## 优势对比

| 方面 | 之前 | 现在 |
|------|------|------|
| **API 调用次数** | 2次（获取节点 + 创建队列） | 1次 |
| **节点提取** | 手动实现 | 自动从数据库提取 |
| **依赖节点** | 需要单独处理 | 自动合并（可选） |
| **节点去重** | 前端处理 | 后端处理 |
| **ProjectIDs 更新** | 仅新队列 | 新队列 + 合并队列 |
| **预计时间** | 无 | 自动计算 |
| **错误提示** | 简单 | 详细（节点树未找到等） |
| **日志记录** | 少 | 详细（每个步骤） |

## 测试场景

### 场景 1：正常渲染

```
前提：项目已刷新节点树
操作：点击"渲染"按钮
预期：
  ✅ 提取所有节点
  ✅ 包含依赖节点
  ✅ 创建渲染队列
  ✅ 显示预计时间
  ✅ 底部状态栏出现
```

### 场景 2：节点树未刷新

```
前提：项目未刷新节点树（数据库无缓存）
操作：点击"渲染"按钮
预期：
  ❌ 返回错误："未找到节点树缓存，请先在 Dashboard 中刷新节点树"
```

### 场景 3：队列合并（同项目）

```
前提：项目A已有待处理的渲染队列
操作：再次点击"渲染"按钮
预期：
  ✅ 合并到现有队列
  ✅ project_ids 不重复
  ✅ 节点去重
  ✅ 底部状态栏更新进度
```

### 场景 4：队列合并（不同项目）

```
前提：
  - 项目A (fileKey=xxx) 已有待处理队列
  - 项目B (fileKey=xxx, 相同文件) 点击渲染
操作：项目B点击"渲染"按钮
预期：
  ✅ 合并到项目A的队列
  ✅ project_ids = [A_id, B_id]
  ✅ 两个项目都可以查询进度
  ✅ 两个项目的底部状态栏都显示
```

### 场景 5：包含依赖节点

```
前提：项目配置了 ref_nodes
操作：点击"渲染"按钮（include_ref_nodes=true）
预期：
  ✅ 提取节点树所有节点
  ✅ 添加 ref_nodes 中的节点
  ✅ 自动去重
  ✅ 日志显示新增的依赖节点数量
```

## 相关文件

### 修改的文件

1. **web/static/js/dashboard.js**
   - 修改：`renderCurrentProject()` 函数
   - 改为调用 `/figma/project/render`
   - 添加 `include_ref_nodes: true`
   - 显示预计时间

2. **internal/controllers/cache.go**
   - 完善：`RenderProject()` 函数
   - 从 `figma_file_caches` 提取节点树
   - 从 `ref_nodes` 合并依赖节点
   - 添加详细日志

3. **internal/services/queue.go**
   - 修改：`mergeToExistingQueue()` 函数签名
   - 新增：`newProjectIDs` 参数
   - 实现：projectIDs 合并逻辑
   - 更新：调用处传递 projectIDs

### 数据表依赖

1. **figma_file_caches**
   - `file_key` - 文件标识
   - `root_node_id` - 根节点ID
   - `node_ids` - 逗号分隔的所有节点ID（提取来源）

2. **figma_projects**
   - `id` - 项目ID
   - `file_key` - 文件标识
   - `root_node_id` - 根节点ID
   - `ref_nodes` - 依赖节点（JSON 数组）
   - `render_id` - 关联的渲染队列ID

3. **figma_render_queues**
   - `id` - 队列ID
   - `figma_token` - Token
   - `file_key` - 文件标识
   - `node_ids` - 待渲染节点ID
   - `project_ids` - 关联的项目ID列表（JSON 数组）
   - `format` - 图片格式
   - `scale` - 缩放比例

## 日志示例

### 成功渲染日志

```
📋 [RenderProject] 开始提取项目节点: ProjectID=123, FileKey=xxx, RootNodeID=0:1
✅ [RenderProject] 从节点树提取到 145 个节点
📎 [RenderProject] 添加依赖节点: 8 个
✅ [RenderProject] 添加了 5 个新的依赖节点（去重后）
✅ [RenderProject] 总共需要渲染 150 个节点
🔄 [Queue] 检查是否可以合并到现有队列...
📝 [Queue] 准备合并节点到队列 id=456, 原节点数=100, 新增节点数=50, 合并后总数=150
📎 [Queue] 合并 ProjectIDs: 原有=[111], 新增=1, 合并后=[111, 123]
✅ [Queue] 已合并到现有队列 id=456, 新增节点数=50, 总节点数=150, ProjectIDs=[111, 123]
```

### 节点树未找到日志

```
📋 [RenderProject] 开始提取项目节点: ProjectID=123, FileKey=xxx, RootNodeID=0:1
⚠️ [RenderProject] 未找到节点树缓存，请先刷新节点树
```

## 总结

本次改进实现了：

1. ✅ **简化前端调用**：从2次API减少到1次
2. ✅ **自动节点提取**：从数据库缓存自动提取节点树
3. ✅ **依赖节点合并**：自动合并 ref_nodes 并去重
4. ✅ **队列 ProjectIDs 更新**：合并时正确更新 project_ids
5. ✅ **预计时间计算**：返回渲染预计耗时
6. ✅ **详细日志记录**：每个步骤都有日志追踪
7. ✅ **友好错误提示**：节点树未找到时明确提示
8. ✅ **编译测试通过**：所有修改已验证

现在项目渲染功能更加完善和易用！

