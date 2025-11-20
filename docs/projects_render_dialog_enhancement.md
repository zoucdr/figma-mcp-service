# Projects页面渲染对话框增强功能

## 更新说明

优化了 `projects.html` 页面中的渲染对话框，现在能够显示详细的渲染进度、状态、等待时间等完整信息。

## 问题描述

**原问题：**
- 渲染对话框虽然显示了进度条，但缺少详细信息
- 没有显示等待时间（Token冷却期）
- 状态信息不够明确
- 缺少队列ID等调试信息

## 解决方案

### 1. 增强的渲染对话框UI

#### 新增显示内容

```
┌─────────────────────────────────────────┐
│ 渲染节点图片                             │
├─────────────────────────────────────────┤
│ [等待中] 正在处理...                      │
│                                          │
│ ╔══════════════════════════════╗         │
│ ║  ⏰ 预计等待时间：30秒         ║ ← 新增 │
│ ╚══════════════════════════════╝         │
│                                          │
│ [━━━━━━━━━━━━━━━━━━━] 0%               │
│                                          │
│ 详细信息：                                │
│ • 状态：      等待中          ← 新增      │
│ • 总节点数：  100                        │
│ • 已处理：    0                          │
│ • 队列ID：    12345          ← 新增      │
└─────────────────────────────────────────┘
```

### 2. 代码修改

#### A. 模板文件 (projects.html)

**等待时间提示（紫色渐变卡片）：**
```html
<!-- 等待时间提示 -->
<div v-if="renderProgress.status === 'waiting' && renderProgress.nextAvailableTime > 0" 
     class="wait-time-info">
    <i class="el-icon-time"></i>
    <span>预计等待时间：{{ formatWaitTime(calculateWaitSeconds(renderProgress.nextAvailableTime)) }}</span>
</div>
```

**增强的详细信息：**
```html
<div class="progress-details">
    <div class="detail-item">
        <span class="label">状态：</span>
        <span class="value">{{ getStatusText(renderProgress.status) }}</span>
    </div>
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
    <div class="detail-item" v-if="renderProgress.queueId">
        <span class="label">队列ID:</span>
        <span class="value">{{ renderProgress.queueId }}</span>
    </div>
</div>
```

**CSS样式：**
```css
.wait-time-info {
    display: flex;
    align-items: center;
    padding: 12px 15px;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    border-radius: 8px;
    margin-bottom: 20px;
    color: #fff;
    font-size: 14px;
    box-shadow: 0 4px 12px rgba(102, 126, 234, 0.3);
}

.wait-time-info i {
    font-size: 18px;
    margin-right: 10px;
    animation: pulse 2s ease-in-out infinite;
}
```

#### B. JavaScript (figma-render.js)

**FigmaRenderMixin 新增方法：**

1. **计算等待秒数：**
```javascript
calculateWaitSeconds(nextAvailableTime) {
    if (!nextAvailableTime) return 0;
    const now = Math.floor(Date.now() / 1000);
    const wait = nextAvailableTime - now;
    return Math.max(0, wait);
}
```

2. **格式化等待时间：**
```javascript
formatWaitTime(seconds) {
    if (seconds < 60) {
        return `${seconds}秒`;
    } else if (seconds < 3600) {
        const minutes = Math.floor(seconds / 60);
        const secs = seconds % 60;
        return secs > 0 ? `${minutes}分${secs}秒` : `${minutes}分钟`;
    } else {
        const hours = Math.floor(seconds / 3600);
        const minutes = Math.floor((seconds % 3600) / 60);
        return minutes > 0 ? `${hours}小时${minutes}分钟` : `${hours}小时`;
    }
}
```

3. **获取状态类型：**
```javascript
getStatusType(status) {
    const typeMap = {
        'waiting': 'info',
        'processing': 'primary',
        'completed': 'success',
        'failed': 'danger',
        'error': 'danger',
        'cancelled': 'warning'
    };
    return typeMap[status] || 'info';
}
```

4. **获取状态文本：**
```javascript
getStatusText(status) {
    const textMap = {
        'waiting': '等待中',
        'processing': '渲染中',
        'completed': '已完成',
        'failed': '失败',
        'error': '错误',
        'cancelled': '已取消'
    };
    return textMap[status] || status;
}
```

**进度回调增强：**
```javascript
// 进度回调
(progress) => {
    this.renderProgress = {
        ...this.renderProgress,
        ...progress,
        nextAvailableTime: progress.nextAvailableTime || progress.next_available_time || 0
    };
    
    // 更新消息（包含等待时间）
    if (progress.status === 'waiting') {
        if (progress.nextAvailableTime || progress.next_available_time) {
            const wait = this.calculateWaitSeconds(progress.nextAvailableTime || progress.next_available_time);
            this.renderProgress.message = wait > 0 ? 
                `等待中... 预计等待 ${this.formatWaitTime(wait)}` : 
                '等待处理...';
        } else {
            this.renderProgress.message = '等待处理...';
        }
    } else if (progress.status === 'processing') {
        this.renderProgress.message = `处理中... ${progress.processedNodes}/${progress.totalNodes}`;
    }
}
```

**队列状态检查增强：**
```javascript
// FigmaRenderManager.checkQueueStatus
onProgress({
    queueId: queue.id,
    status: queue.status,
    progress: queue.progress || 0,
    processedNodes: queue.processed_nodes || 0,
    failedNodes: queue.failed_nodes || 0,
    totalNodes: queue.total_nodes || 0,
    nextAvailableTime: queue.next_available_time || 0,  // ← 新增
    errorMessage: queue.error_message || ''              // ← 新增
});
```

**renderProgress 初始化：**
```javascript
renderProgress: {
    queueId: null,
    status: 'waiting',
    progress: 0,
    processedNodes: 0,
    failedNodes: 0,
    totalNodes: 0,
    nextAvailableTime: 0,  // ← 新增
    message: ''
}
```

## 功能特性

### 1. 等待时间显示

**触发条件：**
- 渲染状态为 `waiting`
- `nextAvailableTime` > 0（Token处于冷却期）

**显示效果：**
- 紫色渐变背景（醒目）
- 时钟图标动画（脉动效果）
- 实时倒计时（每2秒更新）

**时间格式：**
| 秒数 | 显示 |
|------|------|
| 30 | 30秒 |
| 90 | 1分30秒 |
| 120 | 2分钟 |
| 3700 | 1小时1分钟 |
| 7200 | 2小时 |

### 2. 状态标签颜色

| 状态 | 标签颜色 | 进度条状态 |
|------|---------|-----------|
| waiting | 蓝色 (info) | null |
| processing | 蓝色 (primary) | null |
| completed | 绿色 (success) | success |
| failed | 红色 (danger) | exception |
| error | 红色 (danger) | exception |
| cancelled | 橙色 (warning) | null |

### 3. 详细信息面板

**显示内容：**
- 状态：中文状态文本
- 总节点数：待渲染的总节点数
- 已处理：已完成渲染的节点数
- 失败：渲染失败的节点数（仅失败时显示）
- 队列ID：用于调试和追踪

**样式：**
- 浅灰色背景
- 清晰的标签-值布局
- 成功/失败状态不同颜色

## 数据流程

```
用户点击"开始渲染"
    ↓
调用 FigmaRenderManager.createRenderQueue()
    ↓
服务器创建队列，返回 queue_id
    ↓
开始轮询队列状态（每2秒）
    ↓
GET /figma/render/queue
    ↓
返回队列详情：
{
    id, status, progress,
    processed_nodes, failed_nodes, total_nodes,
    next_available_time  ← 关键字段
}
    ↓
触发 onProgress 回调
    ↓
更新 renderProgress 数据
    ↓
Vue 响应式更新 UI
    ↓
显示等待时间、进度、详细信息
```

## 测试场景

### 场景 1：Token冷却期（等待状态）

**前提条件：**
- Figma Token 处于冷却期
- `next_available_time` 大于当前时间

**预期结果：**
- ✅ 显示紫色等待时间卡片
- ✅ 显示倒计时（如"30秒"）
- ✅ 时钟图标脉动动画
- ✅ 状态显示"等待中"
- ✅ 进度条为0%

### 场景 2：正在渲染（处理状态）

**前提条件：**
- 队列状态为 `processing`
- 有已处理和总节点数

**预期结果：**
- ✅ 不显示等待时间卡片
- ✅ 状态显示"渲染中"
- ✅ 进度条正常增长
- ✅ 详细信息实时更新
- ✅ 显示 "处理中... X/Y"

### 场景 3：渲染完成

**前提条件：**
- 队列状态为 `completed`

**预期结果：**
- ✅ 状态标签变绿色
- ✅ 进度条显示成功状态
- ✅ 显示"已完成"
- ✅ 3秒后自动关闭对话框

### 场景 4：渲染失败

**前提条件：**
- 队列状态为 `failed` 或 `error`
- 有失败节点数

**预期结果：**
- ✅ 状态标签变红色
- ✅ 进度条显示失败状态
- ✅ 显示失败节点数
- ✅ 显示错误消息

## 效果对比

### 修改前

```
渲染节点图片
─────────────────
[等待中] 处理中...

[━━━━━━━━] 0%

总节点数: 100
已处理: 0
```
❌ 缺少等待时间
❌ 缺少详细状态
❌ 信息不够完整

### 修改后

```
渲染节点图片
─────────────────
[等待中] 等待中... 预计等待 30秒

╔════════════════════╗
║ ⏰ 预计等待时间：30秒 ║ ← 新增
╚════════════════════╝

[━━━━━━━━━━] 0%

详细信息：
• 状态：     等待中
• 总节点数： 100
• 已处理：   0
• 队列ID：   12345
```
✅ 显示等待时间
✅ 状态一目了然
✅ 信息完整详细

## 优势

1. **信息透明：** 用户清楚知道需要等待多久
2. **状态明确：** 各种状态用颜色和文字清晰区分
3. **便于调试：** 显示队列ID便于问题追踪
4. **体验友好：** 倒计时让等待不再焦虑

## 兼容性

- ✅ 复用 FigmaRenderMixin，代码统一
- ✅ 与 dashboard.html 的实现保持一致
- ✅ 支持所有主流浏览器
- ✅ 不影响现有功能

## 修改文件清单

1. **web/templates/projects.html**
   - 增强渲染进度显示
   - 添加等待时间卡片
   - 添加状态详情
   - 添加CSS样式

2. **web/static/js/figma-render.js**
   - FigmaRenderMixin 添加辅助方法
   - 增强进度回调逻辑
   - 添加 `nextAvailableTime` 支持
   - 队列状态检查返回完整信息

## 版本信息

- **更新日期：** 2024-11-19
- **版本：** v1.3.0
- **状态：** ✅ 已完成

## 总结

通过增强 projects.html 的渲染对话框，成功实现了：
- ✅ 完整的渲染进度信息显示
- ✅ 等待时间预估和倒计时
- ✅ 清晰的状态标识
- ✅ 详细的调试信息
- ✅ 优雅的UI设计

现在用户在项目列表页渲染时，可以获得与 dashboard 页面一致的详细进度信息！

