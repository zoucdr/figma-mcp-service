# 调试 project_ids 传递问题

## 问题现象

后端日志显示 `ProjectIDs=[]` 为空，前端没有正确传递项目ID。

## 调试步骤

### 1. 打开浏览器开发者工具

按 `F12` 打开浏览器开发者工具，切换到 **Console（控制台）** 标签页。

### 2. 执行渲染操作

#### 方式A：单个项目渲染
1. 在项目列表页点击某个项目的 "渲染" 按钮
2. 在弹出的对话框中点击 "开始渲染"

#### 方式B：分组渲染
1. 点击某个分组的 "批量渲染" 按钮
2. 如果只有一个文件，会弹出对话框，点击 "开始渲染"
3. 如果有多个文件，会直接创建渲染队列

### 3. 查看控制台日志

控制台应该会输出以下调试信息：

#### ✅ 正常情况（有 project_ids）

```javascript
// 步骤1：收集项目信息时
🚀 [renderProjectNodes] 调用 showRenderDialog，项目ID: 123

// 步骤2：显示渲染对话框
🎯 [showRenderDialog] 接收参数: {fileKey: "xxx", nodeIds: 50, projectIds: [123]}
📝 [showRenderDialog] renderForm 已更新: {
    fileKey: "xxx",
    nodeIds: [...],
    projectIds: [123],  // ✅ 有值
    format: "png",
    scale: 2.0
}

// 步骤3：创建渲染队列
🔍 [FigmaRenderManager] 添加 project_ids 到请求: [123]
📤 [FigmaRenderManager] 发送渲染请求: {
    file_key: "xxx",
    node_ids: [...],
    project_ids: [123],  // ✅ 有值
    format: "png",
    scale: 2.0
}
```

#### ❌ 异常情况（没有 project_ids）

```javascript
// 步骤1：如果这一步没有输出，说明调用方式有问题
// （缺失）

// 步骤2：显示渲染对话框
🎯 [showRenderDialog] 接收参数: {fileKey: "xxx", nodeIds: 50, projectIds: undefined}
📝 [showRenderDialog] renderForm 已更新: {
    fileKey: "xxx",
    nodeIds: [...],
    projectIds: [],  // ❌ 空数组
    format: "png",
    scale: 2.0
}

// 步骤3：创建渲染队列
⚠️ [FigmaRenderManager] params.projectIds 为空或不存在: []
📤 [FigmaRenderManager] 发送渲染请求: {
    file_key: "xxx",
    node_ids: [...],
    // ❌ 缺少 project_ids
    format: "png",
    scale: 2.0
}
```

### 4. 查看网络请求

切换到 **Network（网络）** 标签页：

1. 找到 `/figma/manual/render` 请求
2. 点击该请求查看详情
3. 切换到 **Payload（有效负载）** 或 **Request（请求）** 标签
4. 查看请求体中是否包含 `project_ids` 字段

#### ✅ 正确的请求体
```json
{
  "file_key": "xxx",
  "node_ids": ["1:2", "1:3", ...],
  "project_ids": [123],  // ✅ 存在
  "format": "png",
  "scale": 2.0
}
```

#### ❌ 错误的请求体
```json
{
  "file_key": "xxx",
  "node_ids": ["1:2", "1:3", ...],
  // ❌ 缺少 project_ids
  "format": "png",
  "scale": 2.0
}
```

### 5. 查看后端日志

查看服务器控制台输出：

#### ✅ 正确接收
```
📋 [ManualRender] 接收到的请求参数: FileKey=xxx, NodeIDs数量=50, ProjectIDs=[123], Format=png, Scale=2.0
```

#### ❌ 未接收到
```
📋 [ManualRender] 接收到的请求参数: FileKey=xxx, NodeIDs数量=50, ProjectIDs=[], Format=png, Scale=2.0
```

## 可能的问题原因

### 原因 1：浏览器缓存

**症状：**
- 修改了代码但前端没有生效
- 控制台看不到新增的调试日志

**解决方案：**
1. 按 `Ctrl + F5` 强制刷新页面
2. 或者清除浏览器缓存后刷新
3. 或者打开无痕模式测试

### 原因 2：文件未更新

**症状：**
- 确认浏览器缓存已清除
- 仍然看不到调试日志

**解决方案：**
1. 检查文件修改时间是否最新
2. 重新保存文件
3. 重启服务器

**验证命令：**
```bash
# Windows PowerShell
Get-ItemProperty web\static\js\figma-render.js | Select-Object LastWriteTime
Get-ItemProperty web\static\js\projects.js | Select-Object LastWriteTime
```

### 原因 3：调用方式不对

**症状：**
- 控制台有日志，但 `projectIds` 为 `undefined` 或空数组

**排查步骤：**

**检查点 1：调用 showRenderDialog 时是否传递了第三个参数**

在 `projects.js` 中搜索 `showRenderDialog`，确认所有调用都传递了 `projectIds`：

```javascript
// ✅ 正确：传递了项目ID
this.showRenderDialog(fileKey, nodeIds, [project.id]);

// ❌ 错误：缺少第三个参数
this.showRenderDialog(fileKey, nodeIds);
```

**检查点 2：projectIds 是否正确收集**

在 `renderGroupNodes` 方法中，检查是否添加了项目ID：

```javascript
// ✅ 正确：收集了项目ID
projectsByFileKey[fileKey].projectIds.add(project.id);

// ❌ 错误：没有收集项目ID
// （缺失该行代码）
```

**检查点 3：renderForm 是否包含 projectIds 字段**

在 `figma-render.js` 中，检查 `renderForm` 初始化：

```javascript
// ✅ 正确：包含 projectIds
renderForm: {
    fileKey: '',
    nodeIds: [],
    projectIds: [],  // ✅ 存在
    format: 'png',
    scale: 2.0
}

// ❌ 错误：缺少 projectIds
renderForm: {
    fileKey: '',
    nodeIds: [],
    // ❌ 缺少
    format: 'png',
    scale: 2.0
}
```

### 原因 4：Vue 响应式问题

**症状：**
- `showRenderDialog` 收到了 `projectIds`
- 但 `createRenderQueue` 时 `params.projectIds` 为空

**排查步骤：**

在 `startRender` 方法中添加日志：

```javascript
async startRender() {
    console.log('🔍 [startRender] renderForm:', this.renderForm);
    console.log('🔍 [startRender] renderForm.projectIds:', this.renderForm.projectIds);
    
    // ...
    
    const result = await FigmaRenderManager.createRenderQueue(
        this.renderForm,
        // ...
    );
}
```

如果 `renderForm.projectIds` 为空，说明 Vue 响应式出现问题。

**解决方案：**
```javascript
// 使用 Vue.set 确保响应式
this.$set(this.renderForm, 'projectIds', projectIds || []);

// 或者使用对象展开
this.renderForm = {
    ...this.renderForm,
    projectIds: projectIds || []
};
```

## 快速测试脚本

在浏览器控制台执行以下代码测试：

```javascript
// 测试 renderForm 结构
console.log('renderForm:', app.$data.renderForm);

// 测试 showRenderDialog 方法
app.showRenderDialog('test-file-key', ['1:2', '1:3'], [123, 124]);
console.log('设置后的 renderForm:', app.$data.renderForm);

// 清理测试
app.renderDialogVisible = false;
app.resetRenderForm();
```

## 完整的调试检查清单

- [ ] 1. 浏览器开发者工具已打开
- [ ] 2. 已清除浏览器缓存（Ctrl + F5）
- [ ] 3. 控制台能看到调试日志
- [ ] 4. `showRenderDialog` 收到了 `projectIds` 参数
- [ ] 5. `renderForm.projectIds` 有值
- [ ] 6. `createRenderQueue` 收到了 `params.projectIds`
- [ ] 7. 网络请求包含 `project_ids` 字段
- [ ] 8. 后端日志显示 `ProjectIDs=[...]` 有值

## 已知解决方案

### 方案 1：确保所有文件已更新

```bash
# 检查文件修改时间
ls -l web/static/js/figma-render.js
ls -l web/static/js/projects.js

# 如果时间不对，重新保存文件
```

### 方案 2：确保传递了 projectIds

修改所有调用 `showRenderDialog` 的地方：

```javascript
// 单个项目渲染
this.showRenderDialog(fileKey, nodeIds, [project.id]);

// 分组渲染
this.showRenderDialog(fileKey, nodeIds, Array.from(projectIds));
```

### 方案 3：添加调试日志

在关键位置添加 `console.log` 跟踪数据流。

## 最终验证

执行完整的渲染流程后，确认：

1. **前端控制台：** 看到完整的调试日志链
2. **网络请求：** `project_ids` 字段存在且有值
3. **后端日志：** `ProjectIDs=[...]` 不为空
4. **数据库：** 项目的 `render_id` 字段已更新

```sql
SELECT id, name, render_id FROM figma_projects WHERE id = 123;
```

如果以上都确认无误，说明 `project_ids` 已正确传递！

