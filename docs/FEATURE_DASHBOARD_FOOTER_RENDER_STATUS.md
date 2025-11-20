# Dashboard 底部渲染状态栏功能

## 概述

在 Dashboard 底部添加固定的渲染状态栏，实时显示当前项目的渲染进度，包括渲染状态、进度百分比和节点数量。同时确保创建渲染时正确关联项目ID。

## 背景

之前的实现中：
1. 渲染进度只在顶部 header 中显示，位置不显眼
2. 创建渲染队列时未传递 `project_ids` 参数
3. `figma_projects` 表的 `render_id` 字段未被正确更新
4. 前端无法通过项目查询到对应的渲染状态

## 设计方案

### 1. 底部状态栏设计

#### 显示逻辑
- 仅在有渲染任务且状态为 `waiting` 或 `processing` 时显示
- 固定在页面底部，不随滚动消失
- 渲染完成或出错时自动隐藏

#### 信息展示
```
[图标] 渲染状态  |  [进度数字] 已处理/总数 节点  [进度条] 百分比%  [信息图标]
```

**三个区域**：
1. **左侧区域**：状态图标 + 状态文本
   - 等待中：时钟图标 + "渲染等待中"
   - 进行中：加载图标（旋转动画）+ "渲染进行中"

2. **中间区域**：进度详情
   - 节点数量：`已处理节点数 / 总节点数 节点`
   - 进度条：渐变色进度条
   - 百分比：`XX%`

3. **右侧区域**：信息提示
   - 信息图标（悬停显示详细信息）

### 2. 项目ID关联

#### 创建渲染时传递项目ID

修改 `renderCurrentProject` 函数，在调用 `/figma/manual/render` 时传递 `project_ids`：

```javascript
const renderResponse = await axios.post('/figma/manual/render', {
    file_key: fileKey,
    node_ids: nodeIds,
    project_ids: [this.project.id],  // 关联当前项目
    format: this.defaultImageFormat,
    scale: this.defaultImageScale
});
```

#### 后端关联逻辑

后端 `ManualRender` 控制器会：
1. 接收 `project_ids` 参数
2. 创建渲染队列
3. 将 `render_id` 更新到 `figma_projects` 表

```go
// internal/controllers/cache.go
if len(req.ProjectIDs) > 0 && len(queueIDs) > 0 {
    renderID := queueIDs[0]
    for _, projectID := range req.ProjectIDs {
        models.DB.Model(&models.FigmaProject{}).
            Where("id = ?", projectID).
            Update("render_id", renderID)
    }
}
```

### 3. 数据流程

```
用户点击渲染按钮
    ↓
前端：renderCurrentProject()
    ├─ 1. 获取渲染节点列表
    ├─ 2. 创建渲染队列（带 project_ids）
    └─ 3. 后端更新 project.render_id
    ↓
前端：startWatchingProjectRender()
    ├─ 定时轮询 /figma/project/:id/render/progress
    ├─ 更新 projectRenderProgress 数据
    └─ 底部状态栏自动显示/更新
    ↓
渲染完成/出错
    └─ 底部状态栏自动隐藏
```

## 实现细节

### 1. HTML 模板（dashboard.html）

在主容器结束前添加底部状态栏：

```html
<!-- 底部渲染状态栏 -->
<div 
    v-if="projectRenderProgress.hasRender && (projectRenderProgress.status === 'waiting' || projectRenderProgress.status === 'processing')" 
    class="dashboard-footer-status">
    <div class="status-content">
        <!-- 左侧：状态图标和文本 -->
        <div class="status-left">
            <i class="el-icon-loading" v-if="projectRenderProgress.status === 'processing'"></i>
            <i class="el-icon-time" v-else></i>
            <span class="status-text">
                <template v-if="projectRenderProgress.status === 'waiting'">
                    渲染等待中
                </template>
                <template v-else>
                    渲染进行中
                </template>
            </span>
        </div>
        
        <!-- 中间：进度详情 -->
        <div class="status-center">
            <span class="status-numbers">
                {{ projectRenderProgress.processedNodes }} / {{ projectRenderProgress.totalNodes }} 节点
            </span>
            <el-progress 
                :percentage="projectRenderProgress.progress" 
                :stroke-width="6"
                :show-text="false"
                class="status-progress">
            </el-progress>
            <span class="status-percentage">{{ projectRenderProgress.progress }}%</span>
        </div>
        
        <!-- 右侧：信息提示 -->
        <div class="status-right">
            <el-tooltip :content="projectRenderProgress.message" placement="top">
                <i class="el-icon-info"></i>
            </el-tooltip>
        </div>
    </div>
</div>
```

**关键点**：
- 使用 `v-if` 控制显示/隐藏
- 条件：`hasRender && (status === 'waiting' || status === 'processing')`
- 完成或出错时自动隐藏

### 2. JavaScript 逻辑（dashboard.js）

#### 修改 `renderCurrentProject` 函数

添加 `project_ids` 参数：

```javascript
// 2. 创建渲染队列（关联当前项目ID）
const renderResponse = await axios.post('/figma/manual/render', {
    file_key: fileKey,
    node_ids: nodeIds,
    project_ids: [this.project.id],  // ← 新增：关联当前项目
    format: this.defaultImageFormat,
    scale: this.defaultImageScale
});
```

#### 使用现有的渲染进度监控

`dashboard.js` 已经集成了 `FigmaRenderMixin`，其中包含：
- `projectRenderProgress` 数据对象
- `startWatchingProjectRender()` 方法
- `stopWatchingProjectRender()` 方法

这些在 `mounted()` 和 `beforeDestroy()` 生命周期中自动调用。

### 3. CSS 样式（dashboard.css）

#### 固定底部样式

```css
.dashboard-footer-status {
    position: fixed;
    bottom: 0;
    left: 0;
    right: 0;
    height: 50px;
    background: linear-gradient(90deg, #1e1e2e 0%, #252536 100%);
    border-top: 1px solid rgba(255, 255, 255, 0.1);
    box-shadow: 0 -2px 10px rgba(0, 0, 0, 0.3);
    z-index: 1000;
    display: flex;
    align-items: center;
    padding: 0 20px;
}
```

#### 布局样式

```css
.status-content {
    width: 100%;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 20px;
}

.status-left {
    display: flex;
    align-items: center;
    gap: 10px;
    min-width: 150px;
}

.status-center {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 15px;
    max-width: 600px;
}

.status-right {
    display: flex;
    align-items: center;
    gap: 10px;
}
```

#### 动画效果

```css
.status-left i.el-icon-loading {
    animation: rotating 2s linear infinite;
}

@keyframes rotating {
    0% { transform: rotate(0deg); }
    100% { transform: rotate(360deg); }
}
```

#### 进度条样式

```css
.status-progress .el-progress-bar__outer {
    background-color: rgba(255, 255, 255, 0.1);
}

.status-progress .el-progress-bar__inner {
    background: linear-gradient(90deg, #409EFF 0%, #66D9EF 100%);
}
```

#### 响应式调整

```css
/* 当有底部状态栏时，调整主内容区域 */
.app-container:has(.dashboard-footer-status) {
    padding-bottom: 50px;
}
```

## 数据结构

### projectRenderProgress 对象

```javascript
{
    hasRender: true,              // 是否有关联的渲染任务
    renderId: 123,                // 渲染队列ID
    status: 'processing',         // 状态：waiting, processing, completed, error
    progress: 45,                 // 进度百分比 (0-100)
    processedNodes: 45,           // 已处理节点数
    failedNodes: 2,               // 失败节点数
    totalNodes: 100,              // 总节点数
    message: '渲染进行中 45/100'  // 详细信息
}
```

### API 响应格式

`GET /figma/project/:project_id/render/progress`

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "has_render": true,
        "render_id": 123,
        "status": "processing",
        "progress": 45,
        "total_nodes": 100,
        "processed_nodes": 45,
        "failed_nodes": 2,
        "error_message": "",
        "next_available_time": 1234567890,
        "started_at": 1234567890,
        "completed_at": 0,
        "created_at": 1234567890,
        "updated_at": 1234567890
    }
}
```

## 用户交互流程

### 正常流程

```
1. 用户打开项目 Dashboard
   ↓
2. 点击"渲染"按钮
   ↓
3. 前端发送渲染请求（带 project_ids）
   ↓
4. 后端创建渲染队列，更新 project.render_id
   ↓
5. 底部状态栏出现，显示"渲染等待中"
   ↓
6. 渲染开始，状态变为"渲染进行中"，图标开始旋转
   ↓
7. 进度条和数字实时更新
   ↓
8. 渲染完成，状态栏自动消失
```

### 状态变化

| 状态 | 图标 | 文本 | 进度条 | 行为 |
|------|------|------|--------|------|
| `waiting` | `el-icon-time` (静态) | 渲染等待中 | 显示当前进度 | 状态栏显示 |
| `processing` | `el-icon-loading` (旋转) | 渲染进行中 | 实时更新 | 状态栏显示 |
| `completed` | - | - | - | 状态栏隐藏 |
| `error` | - | - | - | 状态栏隐藏 |
| `cancelled` | - | - | - | 状态栏隐藏 |

## 优势

### 1. 用户体验提升
- ✅ 底部固定，随时可见
- ✅ 不遮挡主要工作区域
- ✅ 自动显示/隐藏
- ✅ 实时更新进度

### 2. 信息完整性
- ✅ 显示详细的节点数量
- ✅ 显示百分比进度
- ✅ 显示当前状态
- ✅ 悬停可查看详细信息

### 3. 视觉效果
- ✅ 渐变色背景
- ✅ 旋转动画
- ✅ 进度条渐变
- ✅ 悬停交互效果

### 4. 数据准确性
- ✅ 正确关联项目ID
- ✅ 准确查询渲染状态
- ✅ 实时同步进度

## 技术细节

### 1. 响应式设计

使用 `:has()` 伪类自动调整内容区域：
```css
.app-container:has(.dashboard-footer-status) {
    padding-bottom: 50px;
}
```

### 2. 性能优化

- 使用 `v-if` 而非 `v-show`，未渲染时不创建 DOM
- 固定高度（50px），避免回流
- CSS 动画使用 GPU 加速（transform）

### 3. 可访问性

- 使用语义化的状态文本
- 提供 Tooltip 详细信息
- 清晰的视觉层次

## 测试场景

### 场景 1：创建渲染任务

```
操作：点击"渲染"按钮
预期：
  1. 底部状态栏出现
  2. 显示"渲染等待中"或"渲染进行中"
  3. 显示 0/N 节点
  4. 进度条为 0%
```

### 场景 2：渲染进行中

```
状态：渲染队列正在处理
预期：
  1. 图标旋转动画
  2. 节点数实时更新
  3. 进度条实时增长
  4. 百分比实时变化
```

### 场景 3：渲染完成

```
状态：所有节点处理完成
预期：
  1. 底部状态栏自动隐藏
  2. 主内容区域恢复正常高度
```

### 场景 4：渲染失败

```
状态：渲染出错或取消
预期：
  1. 底部状态栏自动隐藏
  2. 顶部显示错误消息（原有逻辑）
```

### 场景 5：切换项目

```
操作：在有渲染任务时切换到另一个项目
预期：
  1. 如果新项目也有渲染任务，继续显示其状态
  2. 如果新项目无渲染任务，状态栏隐藏
```

## 相关文件

### 修改的文件

1. **web/templates/dashboard.html**
   - 新增：底部渲染状态栏 HTML 结构
   - 位置：主容器结束前（1445行之前）
   - 行数：+37 行

2. **web/static/js/dashboard.js**
   - 修改：`renderCurrentProject()` 函数
   - 新增：`project_ids: [this.project.id]` 参数
   - 行数：+1 行

3. **web/static/css/dashboard.css**
   - 新增：`.dashboard-footer-status` 及相关样式
   - 位置：文件末尾
   - 行数：+103 行

### 依赖的功能

1. **FigmaRenderMixin** (figma-render.js)
   - `projectRenderProgress` 数据对象
   - `startWatchingProjectRender()` 方法
   - `stopWatchingProjectRender()` 方法
   - `formatRenderMessage()` 方法

2. **后端 API**
   - `POST /figma/manual/render` - 创建渲染队列
   - `GET /figma/project/:id/render/progress` - 查询渲染进度

3. **数据库字段**
   - `figma_projects.render_id` - 关联渲染队列ID
   - `figma_render_queues.project_ids` - 关联的项目ID列表

## 局限性

### 1. 单任务显示
- 仅显示当前项目的渲染状态
- 无法同时显示多个项目的渲染

### 2. 高度固定
- 状态栏高度固定为 50px
- 移动端可能需要调整

### 3. 浏览器兼容性
- `:has()` 伪类需要较新的浏览器
- 旧版浏览器可能不支持自动调整

## 后续优化建议

### 1. 多任务支持
显示全局所有渲染任务的汇总信息

### 2. 可折叠设计
添加折叠/展开按钮，节省屏幕空间

### 3. 详细日志
点击状态栏可查看详细的渲染日志

### 4. 错误重试
渲染失败时提供快速重试按钮

### 5. 通知集成
渲染完成时发送桌面通知

## 总结

本次功能实现了：

1. ✅ Dashboard 底部固定渲染状态栏
2. ✅ 实时显示渲染进度和状态
3. ✅ 渲染时正确关联项目ID
4. ✅ 美观的视觉设计和动画效果
5. ✅ 自动显示/隐藏逻辑
6. ✅ 编译测试通过

底部状态栏提供了更好的用户体验，用户可以在工作的同时随时查看渲染进度，无需切换界面或滚动页面。同时确保了项目与渲染队列的正确关联，使得进度查询准确无误。

