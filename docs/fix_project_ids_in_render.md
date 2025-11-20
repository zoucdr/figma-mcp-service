# 修复 Projects 页面渲染时未传递 project_ids 的问题

## 问题描述

在 `projects.html` 页面中点击"渲染"按钮时，前端调用 `/figma/manual/render` API，但**没有传递 `project_ids` 参数**，导致后端无法关联渲染队列到具体的项目。

### 影响范围
- 单个项目渲染（点击项目的渲染按钮）
- 分组渲染（点击分组的"批量渲染"按钮）

### 后端日志
```
📋 [ManualRender] 接收到的请求参数: FileKey=xxx, NodeIDs数量=100, ProjectIDs=[], Format=png, Scale=2.0
```
可以看到 `ProjectIDs=[]` 为空数组。

## 根本原因

### 1. 前端数据收集不完整

**问题代码（projects.js）：**
```javascript
// 只收集了 nodeIds，没有收集 projectIds
if (!projectsByFileKey[fileKey]) {
    projectsByFileKey[fileKey] = {
        fileKey: fileKey,
        nodeIds: new Set(),
        projectCount: 0
        // ❌ 缺少 projectIds
    };
}
```

### 2. API 调用缺少参数

**问题代码（projects.js）：**
```javascript
const response = await axios.post('/figma/manual/render', {
    file_key: fileKey,
    node_ids: nodeIds,
    format: 'png',
    scale: 2.0
    // ❌ 缺少 project_ids
});
```

### 3. showRenderDialog 不支持 projectIds

**问题代码（figma-render.js）：**
```javascript
showRenderDialog(fileKey, nodeIds) {
    this.renderForm.fileKey = fileKey;
    this.renderForm.nodeIds = nodeIds || [];
    // ❌ 没有设置 projectIds
    this.renderDialogVisible = true;
}
```

### 4. renderForm 数据结构缺少字段

**问题代码（figma-render.js）：**
```javascript
renderForm: {
    fileKey: '',
    nodeIds: [],
    format: 'png',
    scale: 2.0
    // ❌ 缺少 projectIds 字段
}
```

## 解决方案

### 修改 1: 收集项目ID（projects.js）

```javascript
if (!projectsByFileKey[fileKey]) {
    projectsByFileKey[fileKey] = {
        fileKey: fileKey,
        nodeIds: new Set(),
        projectIds: new Set(), // ✅ 添加 projectIds 收集
        projectCount: 0
    };
}

// 收集节点时同时收集项目ID
if (response.data.code === 0) {
    const nodeIds = response.data.data.node_ids || [];
    nodeIds.forEach(nodeId => {
        projectsByFileKey[fileKey].nodeIds.add(nodeId);
    });
    projectsByFileKey[fileKey].projectIds.add(project.id); // ✅ 添加项目ID
    projectsByFileKey[fileKey].projectCount++;
}
```

### 修改 2: API 调用传递 project_ids（projects.js）

```javascript
// 多文件批量渲染
const data = projectsByFileKey[fileKey];
const nodeIds = Array.from(data.nodeIds);
const projectIds = Array.from(data.projectIds); // ✅ 转换为数组

const response = await axios.post('/figma/manual/render', {
    file_key: fileKey,
    node_ids: nodeIds,
    project_ids: projectIds, // ✅ 传递项目ID列表
    format: 'png',
    scale: 2.0
});
```

```javascript
// 单文件渲染（显示对话框）
this.showRenderDialog(fileKey, Array.from(data.nodeIds), Array.from(data.projectIds)); // ✅ 传递项目IDs
```

```javascript
// 单个项目渲染
this.showRenderDialog(response.data.data.file_key, nodeIds, [project.id]); // ✅ 传递项目ID
```

### 修改 3: showRenderDialog 支持 projectIds（figma-render.js）

```javascript
showRenderDialog(fileKey, nodeIds, projectIds) {
    this.renderForm.fileKey = fileKey;
    this.renderForm.nodeIds = nodeIds || [];
    this.renderForm.projectIds = projectIds || []; // ✅ 设置项目ID列表
    this.renderDialogVisible = true;
}
```

### 修改 4: renderForm 添加 projectIds 字段（figma-render.js）

```javascript
renderForm: {
    fileKey: '',
    nodeIds: [],
    projectIds: [], // ✅ 添加项目ID列表字段
    format: 'png',
    scale: 2.0
}
```

### 修改 5: createRenderQueue 传递 projectIds（figma-render.js）

```javascript
async createRenderQueue(params, onProgress, onComplete, onError) {
    try {
        const requestData = {
            file_key: params.fileKey,
            node_ids: params.nodeIds,
            format: params.format || 'png',
            scale: params.scale || 2.0
        };
        
        // ✅ 如果有 projectIds，添加到请求中
        if (params.projectIds && params.projectIds.length > 0) {
            requestData.project_ids = params.projectIds;
        }
        
        const response = await axios.post('/figma/manual/render', requestData);
        // ...
    }
}
```

### 修改 6: resetRenderForm 重置 projectIds（figma-render.js）

```javascript
resetRenderForm() {
    this.renderForm = {
        fileKey: '',
        nodeIds: [],
        projectIds: [], // ✅ 重置项目ID列表
        format: 'png',
        scale: 2.0
    };
    // ...
}
```

## 数据流程

### 修复前（错误）

```
用户点击渲染
    ↓
收集节点信息
    ↓
projectsByFileKey = {
    fileKey: 'xxx',
    nodeIds: Set[...],
    projectCount: 5
    // ❌ 没有 projectIds
}
    ↓
调用 /figma/manual/render
    {
        file_key: 'xxx',
        node_ids: [...],
        format: 'png',
        scale: 2.0
        // ❌ 没有 project_ids
    }
    ↓
后端接收到 ProjectIDs = []
    ↓
❌ 渲染队列无法关联到项目
```

### 修复后（正确）

```
用户点击渲染
    ↓
收集节点信息
    ↓
projectsByFileKey = {
    fileKey: 'xxx',
    nodeIds: Set[...],
    projectIds: Set[1, 2, 3], // ✅ 收集项目ID
    projectCount: 3
}
    ↓
调用 /figma/manual/render
    {
        file_key: 'xxx',
        node_ids: [...],
        project_ids: [1, 2, 3], // ✅ 传递项目ID
        format: 'png',
        scale: 2.0
    }
    ↓
后端接收到 ProjectIDs = [1, 2, 3]
    ↓
✅ 渲染队列成功关联到项目
    ↓
✅ 更新项目的 render_id 字段
```

## 修改文件清单

1. **web/static/js/projects.js**
   - `renderGroupNodes()` 方法
     - 添加 `projectIds` 集合收集
     - 在多文件渲染时传递 `project_ids`
     - 在单文件渲染时传递 `projectIds` 到 `showRenderDialog`
   - `renderProjectNodes()` 方法
     - 传递项目ID到 `showRenderDialog`

2. **web/static/js/figma-render.js**
   - FigmaRenderMixin
     - `renderForm` 添加 `projectIds` 字段
     - `showRenderDialog()` 方法支持接收 `projectIds` 参数
     - `resetRenderForm()` 方法重置 `projectIds`
   - FigmaRenderManager
     - `createRenderQueue()` 方法传递 `project_ids` 到后端

## 测试验证

### 测试场景 1: 单个项目渲染

**步骤：**
1. 在项目列表页点击某个项目的"渲染"按钮
2. 在渲染对话框点击"开始渲染"
3. 查看后端日志

**预期结果：**
```
📋 [ManualRender] 接收到的请求参数: FileKey=xxx, NodeIDs数量=50, ProjectIDs=[123], Format=png, Scale=2.0
```
✅ `ProjectIDs` 包含该项目的ID

### 测试场景 2: 分组渲染（单文件）

**步骤：**
1. 点击分组的"批量渲染"按钮
2. 如果该分组只有一个 file_key，会弹出渲染对话框
3. 点击"开始渲染"
4. 查看后端日志

**预期结果：**
```
📋 [ManualRender] 接收到的请求参数: FileKey=xxx, NodeIDs数量=150, ProjectIDs=[123, 124, 125], Format=png, Scale=2.0
```
✅ `ProjectIDs` 包含该分组所有项目的ID

### 测试场景 3: 分组渲染（多文件）

**步骤：**
1. 点击分组的"批量渲染"按钮
2. 如果该分组有多个 file_key，会直接批量创建渲染队列
3. 查看后端日志

**预期结果：**
```
📋 [ManualRender] 接收到的请求参数: FileKey=xxx, NodeIDs数量=80, ProjectIDs=[123, 124], Format=png, Scale=2.0
📋 [ManualRender] 接收到的请求参数: FileKey=yyy, NodeIDs数量=70, ProjectIDs=[125], Format=png, Scale=2.0
```
✅ 每个文件的 `ProjectIDs` 都正确传递

### 验证后端处理

**检查项目的 render_id：**
```sql
SELECT id, name, render_id FROM figma_projects WHERE id IN (123, 124, 125);
```

**预期结果：**
```
| id  | name      | render_id |
|-----|-----------|-----------|
| 123 | Project A | 456       |
| 124 | Project B | 456       |
| 125 | Project C | 457       |
```
✅ `render_id` 已正确更新

## 影响分析

### 修复前的问题

1. **渲染队列无关联：**
   - 渲染队列创建成功，但不知道属于哪个项目
   - 项目的 `render_id` 字段不会更新

2. **进度无法显示：**
   - Dashboard 页面无法自动显示渲染进度
   - 用户不知道项目的渲染状态

3. **数据不一致：**
   - 渲染完成后，项目状态不会更新
   - 无法追踪哪些项目参与了渲染

### 修复后的改进

1. **完整关联：** ✅
   - 渲染队列正确关联到项目
   - 项目的 `render_id` 正确更新

2. **进度可见：** ✅
   - Dashboard 页面自动显示渲染进度
   - 用户清楚了解渲染状态

3. **数据一致：** ✅
   - 渲染完成后项目状态同步更新
   - 可以准确追踪项目渲染历史

## 兼容性

- ✅ 向后兼容：`project_ids` 在后端是可选参数
- ✅ 不影响现有功能：Dashboard 页面渲染不受影响
- ✅ 渐进增强：即使没有 `project_ids` 也能渲染，只是无法关联

## 版本信息

- **修复日期：** 2024-11-19
- **版本：** v1.4.0
- **状态：** ✅ 已完成

## 总结

通过在前端收集和传递 `project_ids` 参数，成功修复了 Projects 页面渲染时无法关联项目的问题。现在：

- ✅ 单个项目渲染会传递项目ID
- ✅ 分组渲染会传递所有项目ID
- ✅ 后端能正确更新项目的 `render_id`
- ✅ Dashboard 可以显示渲染进度
- ✅ 数据关联完整准确

