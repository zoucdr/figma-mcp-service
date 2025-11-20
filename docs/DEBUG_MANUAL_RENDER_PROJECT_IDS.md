# 调试：ManualRender 项目ID传递问题

## 问题描述

在调用 `/figma/manual/render` 接口时，`ProjectIDs` 参数可能是空数组，导致 `figma_projects` 表的 `render_id` 字段未被正确更新。

## 原因分析

可能的原因：
1. 前端 `this.project.id` 为 `undefined` 或 `null`
2. 前端传递了 `project_ids`，但值为空数组 `[]`
3. 后端 JSON 解析问题

## 调试日志

### 已添加的日志点

#### 1. 前端日志（dashboard.js）

```javascript
console.log('📋 [Dashboard] 准备创建渲染队列:', {
    project_id: this.project.id,
    file_key: fileKey,
    node_count: nodeIds.length,
    format: this.defaultImageFormat,
    scale: this.defaultImageScale
});
```

**查看位置**：浏览器开发者工具 Console

**正常输出**：
```
📋 [Dashboard] 准备创建渲染队列: {
    project_id: 123,
    file_key: "xxx",
    node_count: 50,
    format: "png",
    scale: 1.0
}
```

**异常输出**：
```
📋 [Dashboard] 准备创建渲染队列: {
    project_id: undefined,  // ← 问题：项目ID未定义
    file_key: "xxx",
    ...
}
```

#### 2. 后端日志（cache.go）

**日志 A：接收请求参数**
```go
log.Printf("📋 [ManualRender] 接收到的请求参数: FileKey=%s, NodeIDs数量=%d, ProjectIDs=%v, Format=%s, Scale=%.1f",
    req.FileKey, len(req.NodeIDs), req.ProjectIDs, req.Format, req.Scale)
```

**正常输出**：
```
📋 [ManualRender] 接收到的请求参数: FileKey=xxx, NodeIDs数量=50, ProjectIDs=[123], Format=png, Scale=1.0
```

**异常输出 1**：
```
📋 [ManualRender] 接收到的请求参数: FileKey=xxx, NodeIDs数量=50, ProjectIDs=[], Format=png, Scale=1.0
↑ 问题：前端传了空数组
```

**异常输出 2**：
```
📋 [ManualRender] 接收到的请求参数: FileKey=xxx, NodeIDs数量=50, ProjectIDs=<nil>, Format=png, Scale=1.0
↑ 问题：后端未接收到该字段
```

**日志 B：更新 render_id**
```go
log.Printf("✅ [ManualRender] 更新项目的 render_id: ProjectIDs=%v, RenderID=%d", req.ProjectIDs, renderID)
log.Printf("✅ [ManualRender] 已更新项目 %d 的 render_id=%d", projectID, renderID)
```

**正常输出**：
```
✅ [ManualRender] 更新项目的 render_id: ProjectIDs=[123], RenderID=456
✅ [ManualRender] 已更新项目 123 的 render_id=456
```

**日志 C：未更新的警告**
```go
log.Printf("⚠️ [ManualRender] 未更新 render_id: ProjectIDs长度=%d, QueueIDs长度=%d", len(req.ProjectIDs), len(queueIDs))
```

**异常输出**：
```
⚠️ [ManualRender] 未更新 render_id: ProjectIDs长度=0, QueueIDs长度=1
↑ 问题：ProjectIDs 为空，跳过了更新逻辑
```

## 排查步骤

### 步骤 1：检查前端项目对象

在浏览器 Console 中运行：
```javascript
console.log('当前项目:', window.ProjectEditorApp.$data.project);
console.log('项目ID:', window.ProjectEditorApp.$data.project?.id);
```

**预期结果**：
```javascript
当前项目: {id: 123, name: "测试项目", file_key: "xxx", ...}
项目ID: 123
```

**如果是 undefined**：
- 检查项目是否正确加载
- 检查 URL 参数中是否包含项目ID
- 检查后端是否正确返回项目数据

### 步骤 2：检查网络请求

1. 打开浏览器开发者工具 → Network 标签
2. 点击"渲染"按钮
3. 找到 `/figma/manual/render` 请求
4. 查看 Request Payload

**正常的 Payload**：
```json
{
    "file_key": "xxx",
    "node_ids": ["1:23", "1:24", ...],
    "project_ids": [123],  // ← 应该包含项目ID
    "format": "png",
    "scale": 1.0
}
```

**异常的 Payload**：
```json
{
    "file_key": "xxx",
    "node_ids": ["1:23", "1:24", ...],
    "project_ids": [],  // ← 空数组
    "format": "png",
    "scale": 1.0
}
```

或者：
```json
{
    "file_key": "xxx",
    "node_ids": ["1:23", "1:24", ...],
    // project_ids 字段缺失
    "format": "png",
    "scale": 1.0
}
```

### 步骤 3：检查后端日志

查看服务器日志输出：

```bash
# Windows PowerShell
Get-Content .\logs\app.log -Tail 50 -Wait

# 或者直接运行程序查看输出
go run main.go
```

关键日志：
```
📋 [ManualRender] 接收到的请求参数: ...
⚠️ [ManualRender] 未更新 render_id: ...  // ← 如果看到这个，说明 ProjectIDs 为空
✅ [ManualRender] 更新项目的 render_id: ...  // ← 如果看到这个，说明正常
```

### 步骤 4：检查数据库

查询 `figma_projects` 表的 `render_id` 字段：

```sql
SELECT id, name, render_id, updated_at 
FROM figma_projects 
WHERE id = 123;  -- 替换为实际的项目ID
```

**正常情况**：
- `render_id` 应该是最近创建的渲染队列ID（非0）
- `updated_at` 应该是最近的时间戳

**异常情况**：
- `render_id` 为 0（默认值，未被更新）
- `updated_at` 是旧的时间戳

## 可能的修复方案

### 方案 1：前端 project.id 为空

**问题**：`this.project.id` 为 `undefined`

**修复**：确保项目正确加载

```javascript
// renderCurrentProject 函数开始处添加检查
async renderCurrentProject() {
    if (!this.project || !this.project.id) {
        this.$message.error('项目信息未加载，请刷新页面');
        console.error('项目对象:', this.project);
        return;
    }
    
    console.log('✅ 项目ID:', this.project.id);
    
    // ... 继续原有逻辑
}
```

### 方案 2：确保传递非空数组

**问题**：`this.project.id` 可能是 0 或其他假值

**修复**：过滤无效值

```javascript
// 确保 project_ids 不为空
const projectIds = this.project?.id ? [this.project.id] : [];

console.log('📋 [Dashboard] 项目IDs:', projectIds);

if (projectIds.length === 0) {
    console.warn('⚠️ 项目ID无效，将不关联渲染队列');
}

const renderResponse = await axios.post('/figma/manual/render', {
    file_key: fileKey,
    node_ids: nodeIds,
    project_ids: projectIds,
    format: this.defaultImageFormat,
    scale: this.defaultImageScale
});
```

### 方案 3：后端提供默认值

**问题**：前端确实没传或传了空数组

**修复**：后端从 file_key 反查项目

```go
// 如果 ProjectIDs 为空，尝试从 file_key 查找项目
if len(req.ProjectIDs) == 0 {
    var projects []models.FigmaProject
    err := models.DB.Where("file_key = ? AND user_id = ?", req.FileKey, userID).
        Find(&projects).Error
    if err == nil && len(projects) > 0 {
        for _, p := range projects {
            req.ProjectIDs = append(req.ProjectIDs, p.ID)
        }
        log.Printf("💡 [ManualRender] 从 FileKey 反查到项目IDs: %v", req.ProjectIDs)
    }
}
```

### 方案 4：从 URL 参数获取项目ID

**问题**：Dashboard 可能从 URL 加载项目

**修复**：确保 URL 参数正确解析

```javascript
// 在 mounted() 生命周期中
mounted() {
    // 从 URL 获取项目ID
    const urlParams = new URLSearchParams(window.location.search);
    const projectId = urlParams.get('id');
    
    console.log('📋 URL 项目ID:', projectId);
    
    if (projectId && this.project) {
        this.project.id = parseInt(projectId);
    }
    
    // ... 其他初始化逻辑
}
```

## 验证修复

### 1. 前端验证

浏览器 Console 应该看到：
```
📋 [Dashboard] 准备创建渲染队列: {project_id: 123, ...}
✅ 项目ID: 123
```

### 2. 后端验证

服务器日志应该看到：
```
📋 [ManualRender] 接收到的请求参数: ... ProjectIDs=[123] ...
✅ [ManualRender] 更新项目的 render_id: ProjectIDs=[123], RenderID=456
✅ [ManualRender] 已更新项目 123 的 render_id=456
```

### 3. 数据库验证

```sql
SELECT id, name, render_id FROM figma_projects WHERE id = 123;
```

应该看到 `render_id` 被更新为最新的队列ID。

### 4. 底部状态栏验证

- 点击渲染按钮后
- 底部应该出现渲染状态栏
- 显示进度和节点数

如果底部状态栏没有出现，说明 `render_id` 没有正确关联。

## 相关代码位置

### 前端
- `web/static/js/dashboard.js:1225` - `renderCurrentProject` 函数
- `web/static/js/dashboard.js:1256` - 创建渲染请求（带调试日志）

### 后端
- `internal/controllers/cache.go:593` - `ManualRenderRequest` 结构体
- `internal/controllers/cache.go:602` - `ManualRender` 处理函数
- `internal/controllers/cache.go:630` - 接收参数日志
- `internal/controllers/cache.go:650` - 更新 render_id 逻辑

### 数据库
- `figma_projects.render_id` - 项目关联的渲染队列ID
- `figma_render_queues.project_ids` - 渲染队列关联的项目ID列表

## 总结

通过添加详细的调试日志，可以快速定位问题：

1. **前端日志**：确认 `this.project.id` 的值
2. **网络日志**：确认请求 Payload 是否包含 `project_ids`
3. **后端日志**：确认后端是否接收到 `ProjectIDs`
4. **数据库查询**：确认 `render_id` 是否被正确更新

根据日志输出，选择相应的修复方案。现在运行程序并点击渲染按钮，查看日志输出即可诊断问题所在。

