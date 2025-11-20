# 前端集成指南 - Figma 渲染功能

## 📋 概述

本文档说明 Figma 节点渲染功能的前端集成方案，包括渲染按钮、进度跟踪、错误提示等功能。

## 🎯 实现功能

### ✅ 已完成功能

1. **渲染按钮**
   - 项目级别渲染按钮
   - 分组级别批量渲染按钮
   
2. **渲染对话框**
   - 参数配置（格式、缩放比例）
   - 实时进度显示
   - 详细状态信息
   
3. **进度跟踪**
   - 自动轮询队列状态
   - 实时进度条更新
   - 节点处理统计
   
4. **错误处理**
   - Token 冷却提示
   - API 速率限制提示
   - 通用错误提示

## 🏗️ 架构设计

### 文件结构

```
web/
├── static/
│   └── js/
│       ├── figma-render.js    # 渲染功能核心模块
│       └── projects.js         # 项目列表（集成渲染功能）
└── templates/
    └── projects.html           # 项目列表页面（包含渲染UI）
```

### 核心组件

#### 1. FigmaRenderManager（渲染管理器）

**功能**：
- 创建渲染队列
- 轮询队列状态
- 管理多个并发队列

**主要方法**：

```javascript
// 创建渲染队列
createRenderQueue(params, onProgress, onComplete, onError)

// 开始轮询队列状态
startPolling(queueId, onProgress, onComplete, onError)

// 停止轮询指定队列
stopPolling(queueId)

// 获取队列统计信息
getQueueStats()
```

**使用示例**：

```javascript
const result = await FigmaRenderManager.createRenderQueue(
    {
        fileKey: 'xxx',
        nodeIds: ['1:123', '1:124'],
        format: 'png',
        scale: 2.0
    },
    (progress) => {
        console.log('进度:', progress.progress, '%');
    },
    (result) => {
        console.log('完成:', result);
    },
    (error) => {
        console.error('错误:', error);
    }
);
```

#### 2. FigmaRenderMixin（Vue Mixin）

**功能**：
- 提供渲染相关的 Vue 组件状态和方法
- 可以混入任何需要渲染功能的组件

**数据属性**：

```javascript
{
    renderDialogVisible: false,    // 对话框可见性
    renderForm: {                   // 渲染表单
        fileKey: '',
        nodeIds: [],
        format: 'png',
        scale: 2.0
    },
    renderProgress: {               // 渲染进度
        queueId: null,
        status: 'waiting',
        progress: 0,
        processedNodes: 0,
        failedNodes: 0,
        totalNodes: 0,
        message: ''
    },
    rendering: false                // 是否正在渲染
}
```

**主要方法**：

```javascript
// 显示渲染对话框
showRenderDialog(fileKey, nodeIds)

// 开始渲染
startRender()

// 取消渲染
cancelRender()

// 格式化状态文本
formatStatus(status)

// 获取状态颜色
getStatusColor(status)
```

## 🔧 集成步骤

### 1. 引入渲染模块

在 HTML 模板中引入 `figma-render.js`：

```html
<!-- 渲染功能模块 -->
<script src="/static/js/figma-render.js?v={{ .timestamp }}"></script>
```

### 2. 混入渲染功能

在 Vue 组件中混入 `FigmaRenderMixin`：

```javascript
const MyApp = {
    // 混入渲染功能
    mixins: [window.FigmaRenderMixin],
    
    data() {
        return {
            // ... 其他数据
        };
    },
    
    methods: {
        // 触发渲染
        async myRenderMethod() {
            // 获取节点ID
            const nodeIds = ['1:123', '1:124'];
            const fileKey = 'xxx';
            
            // 显示渲染对话框
            this.showRenderDialog(fileKey, nodeIds);
        }
    }
};
```

### 3. 添加渲染对话框 UI

在 HTML 模板中添加渲染对话框：

```html
<!-- 渲染对话框 -->
<el-dialog 
    title="渲染节点图片" 
    :visible.sync="renderDialogVisible" 
    width="600px"
    :close-on-click-modal="false"
    :close-on-press-escape="!rendering">
    
    <div v-if="!rendering" class="render-form">
        <!-- 渲染表单 -->
        <el-form label-width="100px">
            <el-form-item label="文件Key">
                <el-input v-model="renderForm.fileKey" disabled></el-input>
            </el-form-item>
            <el-form-item label="节点数量">
                <el-tag type="info">{{ renderForm.nodeIds.length }} 个节点</el-tag>
            </el-form-item>
            <el-form-item label="图片格式">
                <el-select v-model="renderForm.format" style="width: 100%">
                    <el-option label="PNG" value="png"></el-option>
                    <el-option label="JPG" value="jpg"></el-option>
                    <el-option label="SVG" value="svg"></el-option>
                </el-select>
            </el-form-item>
            <el-form-item label="缩放比例">
                <el-select v-model="renderForm.scale" style="width: 100%">
                    <el-option label="1x" :value="1.0"></el-option>
                    <el-option label="2x (推荐)" :value="2.0"></el-option>
                    <el-option label="3x" :value="3.0"></el-option>
                    <el-option label="4x" :value="4.0"></el-option>
                </el-select>
            </el-form-item>
        </el-form>
    </div>
    
    <!-- 渲染进度 -->
    <div v-else class="render-progress">
        <div class="progress-header">
            <el-tag :type="renderProgress.status === 'completed' ? 'success' : 'info'">
                {{ formatStatus(renderProgress.status) }}
            </el-tag>
            <span class="progress-message">{{ renderProgress.message }}</span>
        </div>
        
        <!-- 进度条 -->
        <el-progress 
            :percentage="renderProgress.progress" 
            :status="renderProgress.status === 'completed' ? 'success' : null"
            :stroke-width="20">
        </el-progress>
        
        <!-- 详细信息 -->
        <div class="progress-details">
            <div class="detail-item">
                <span class="label">总节点数:</span>
                <span class="value">{{ renderProgress.totalNodes }}</span>
            </div>
            <div class="detail-item">
                <span class="label">已处理:</span>
                <span class="value success">{{ renderProgress.processedNodes }}</span>
            </div>
            <div class="detail-item" v-if="renderProgress.failedNodes > 0">
                <span class="label">失败:</span>
                <span class="value error">{{ renderProgress.failedNodes }}</span>
            </div>
        </div>
    </div>
    
    <div slot="footer" class="dialog-footer">
        <el-button @click="cancelRender" v-if="!rendering || renderProgress.status === 'completed'">
            {{ renderProgress.status === 'completed' ? '关闭' : '取消' }}
        </el-button>
        <el-button type="primary" @click="startRender" v-if="!rendering" :loading="rendering">
            开始渲染
        </el-button>
    </div>
</el-dialog>
```

## 🎨 UI 设计

### 渲染按钮

#### 项目级别按钮

```html
<el-button 
    size="mini" 
    type="success" 
    icon="el-icon-picture" 
    @click="renderProjectNodes(project)" 
    title="渲染节点图片">
    渲染
</el-button>
```

#### 分组级别按钮

```html
<el-button 
    size="mini" 
    type="success" 
    icon="el-icon-picture" 
    @click.stop="renderGroupNodes(group)"
    title="渲染该分组所有节点">
    批量渲染
</el-button>
```

### 状态显示

#### 状态映射

| 状态 | 显示文本 | 颜色 |
|------|---------|------|
| `waiting` | 等待中 | 灰色 `#909399` |
| `preparing` | 准备中 | 蓝色 `#409EFF` |
| `processing` | 处理中 | 蓝色 `#409EFF` |
| `completed` | 已完成 | 绿色 `#67C23A` |
| `failed` | 失败 | 红色 `#F56C6C` |
| `error` | 错误 | 红色 `#F56C6C` |

## 🚨 错误处理

### 错误类型

#### 1. Token 冷却错误 (code: 1001)

```javascript
{
    type: 'cooldown',
    message: 'Token 冷却中，请等待 30 秒后重试',
    waitSeconds: 30,
    nextTime: 1699999999
}
```

**处理方式**：显示警告对话框，提示用户等待时间

```javascript
this.$alert(
    `Token 冷却中，请等待 ${error.waitSeconds} 秒后重试`,
    'Token 冷却',
    {
        confirmButtonText: '确定',
        type: 'warning'
    }
);
```

#### 2. API 速率限制错误 (status: 429)

**处理方式**：显示警告对话框，提示用户稍后重试

```javascript
this.$alert(
    'Figma API 请求过于频繁，请稍后重试',
    '速率限制',
    {
        confirmButtonText: '确定',
        type: 'warning'
    }
);
```

#### 3. 队列失败错误

```javascript
{
    type: 'queue_failed',
    message: '渲染失败',
    queueId: 123
}
```

**处理方式**：显示错误对话框

```javascript
this.$alert(
    error.message || '渲染失败，请稍后重试',
    '渲染错误',
    {
        confirmButtonText: '确定',
        type: 'error'
    }
);
```

## 🔄 工作流程

### 完整渲染流程

```
用户点击渲染按钮
    ↓
获取项目节点信息 (GET /api/figma/project/:id/nodes)
    ↓
显示渲染对话框，用户配置参数
    ↓
用户点击"开始渲染"
    ↓
创建渲染队列 (POST /api/figma/manual/render)
    ↓
    ├─→ 成功: 开始轮询队列状态
    │   ↓
    │   每 2 秒轮询一次 (GET /api/figma/render/queue)
    │   ↓
    │   更新进度条和状态信息
    │   ↓
    │   ├─→ 状态 = 'completed': 显示完成信息
    │   ├─→ 状态 = 'failed': 显示错误信息
    │   └─→ 状态 = 'processing': 继续轮询
    │
    └─→ 失败 (冷却/速率限制): 显示错误提示
```

### 状态轮询机制

- **轮询间隔**: 2 秒
- **自动停止条件**:
  - 队列状态为 `completed`
  - 队列状态为 `failed` 或 `error`
  - 用户手动取消
  - 队列不存在

## 📊 API 接口

### 1. 创建渲染队列

**请求**：
```http
POST /api/figma/manual/render
Content-Type: application/json

{
    "file_key": "xxx",
    "node_ids": ["1:123", "1:124"],
    "format": "png",
    "scale": 2.0
}
```

**响应**（成功）：
```json
{
    "code": 0,
    "message": "渲染队列已创建",
    "data": {
        "queue_id": 123,
        "total_nodes": 2
    }
}
```

**响应**（Token 冷却）：
```json
{
    "code": 1001,
    "message": "Token 冷却中，请等待 25 秒后重试",
    "data": {
        "wait_seconds": 25,
        "next_available_time": 1699999999
    }
}
```

### 2. 获取队列状态

**请求**：
```http
GET /api/figma/render/queue
```

**响应**：
```json
{
    "code": 0,
    "data": {
        "queues": [
            {
                "id": 123,
                "file_key": "xxx",
                "status": "processing",
                "progress": 50,
                "total_nodes": 10,
                "processed_nodes": 5,
                "failed_nodes": 0,
                "created_at": 1699999999,
                "updated_at": 1699999999
            }
        ]
    }
}
```

### 3. 获取队列统计

**请求**：
```http
GET /api/figma/render/queue/stats
```

**响应**：
```json
{
    "code": 0,
    "data": {
        "total": 10,
        "waiting": 2,
        "processing": 1,
        "completed": 6,
        "failed": 1
    }
}
```

## 🎯 使用示例

### 示例 1: 渲染单个项目

```javascript
async renderProject(project) {
    // 获取项目节点
    const response = await axios.get(`/api/figma/project/${project.id}/nodes`);
    const nodes = response.data.data.nodes;
    const nodeIds = nodes.map(node => node.figma_node_id);
    
    // 显示渲染对话框
    this.showRenderDialog(project.file_key, nodeIds);
}
```

### 示例 2: 批量渲染分组

```javascript
async renderGroup(group) {
    // 收集分组内所有项目的节点
    const allNodeIds = new Set();
    
    for (const project of group.projects) {
        const response = await axios.get(`/api/figma/project/${project.id}/nodes`);
        const nodes = response.data.data.nodes || [];
        nodes.forEach(node => {
            if (node.figma_node_id) {
                allNodeIds.add(node.figma_node_id);
            }
        });
    }
    
    // 显示渲染对话框
    this.showRenderDialog(group.fileKey, Array.from(allNodeIds));
}
```

## 🔍 调试技巧

### 查看渲染管理器状态

```javascript
// 在浏览器控制台中
console.log('正在轮询的队列:', FigmaRenderManager.pollingQueues);
console.log('轮询定时器:', FigmaRenderManager.pollTimer);
```

### 手动停止轮询

```javascript
// 停止指定队列的轮询
FigmaRenderManager.stopPolling(queueId);

// 清除所有轮询
FigmaRenderManager.pollingQueues.clear();
```

### 查看队列统计

```javascript
const stats = await FigmaRenderManager.getQueueStats();
console.log('队列统计:', stats);
```

## 🚀 性能优化

### 1. 轮询优化

- 使用单一定时器管理多个队列
- 队列完成后自动停止轮询
- 避免重复创建定时器

### 2. 内存管理

- 完成的队列自动从轮询列表中移除
- 对话框关闭时清理状态

### 3. 用户体验

- 显示详细的进度信息
- 提供友好的错误提示
- 支持取消操作

## ✅ 测试清单

### 功能测试

- [ ] 点击项目渲染按钮，对话框正常显示
- [ ] 点击分组渲染按钮，对话框正常显示
- [ ] 修改渲染参数（格式、缩放），参数正确传递
- [ ] 开始渲染后，进度条正常更新
- [ ] 渲染完成后，显示成功信息
- [ ] 渲染失败后，显示错误信息
- [ ] Token 冷却时，显示冷却提示
- [ ] 取消渲染后，轮询停止

### 边界测试

- [ ] 项目没有节点时，显示警告
- [ ] 网络错误时，显示友好提示
- [ ] 多次快速点击渲染按钮，不会重复创建队列
- [ ] 同时渲染多个项目，轮询正常工作

## 📝 注意事项

1. **Token 管理**
   - 确保用户已配置有效的 Figma Token
   - 注意 Token 冷却时间（30秒）

2. **节点数量限制**
   - 单次请求最多 1000 个节点
   - 超过限制时自动分批处理

3. **轮询频率**
   - 默认 2 秒轮询一次
   - 不建议设置过快，避免影响服务器性能

4. **浏览器兼容性**
   - 需要支持 ES6+
   - 需要支持 async/await
   - 需要支持 Fetch API 或 Axios

## 🔗 相关文档

- [后端 API 文档](./API_TESTING_GUIDE.md)
- [缓存机制文档](./FIGMA_CACHE_RATE_LIMIT_SOLUTION.md)
- [调度器文档](./SCHEDULER_IMPLEMENTATION.md)
- [OBS 集成文档](./OBS_INTEGRATION_GUIDE.md)

## 📌 总结

前端渲染功能已经完整实现，包括：

✅ **核心功能**
- 渲染按钮（项目级别、分组级别）
- 渲染对话框（参数配置、进度显示）
- 状态轮询（自动更新、智能停止）
- 错误处理（冷却提示、速率限制提示）

✅ **用户体验**
- 实时进度反馈
- 友好的错误提示
- 流畅的交互体验
- 详细的状态信息

✅ **技术实现**
- 模块化设计
- Vue Mixin 复用
- 事件驱动架构
- 智能轮询机制

---

**文档版本**: v1.0  
**更新时间**: 2025-11-19  
**作者**: AI Assistant

