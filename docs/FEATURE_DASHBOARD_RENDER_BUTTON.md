# Dashboard 页面渲染功能

## 功能概述

在 Dashboard 编辑页面的底部控制栏添加"渲染"按钮，用户可以直接渲染当前项目的所有节点图片，无需弹框交互。

## 功能特点

### 1. 一键渲染

- ✅ 点击即可渲染当前项目的所有节点
- ✅ 自动获取项目的渲染节点列表（包含依赖节点）
- ✅ 使用当前设置的图片格式和缩放比例
- ✅ 无需弹框，直接创建渲染队列

### 2. 智能状态管理

- ✅ 按钮显示加载状态（loading）
- ✅ 渲染中禁用按钮，防止重复提交
- ✅ 项目未加载时禁用按钮
- ✅ 显示图标和文字状态

### 3. 完整的错误处理

- ✅ 404 - 节点树未找到，提示刷新
- ✅ 400 - Figma API 错误提示
- ✅ 429 - 速率限制提示
- ✅ 其他错误的友好提示

## UI 设计

### 按钮位置

位于底部控制栏，在"API访问"按钮和"清除缓存"按钮之间：

```
[图片格式] [缩放比例] [图文导出] [API访问] [渲染] [清除缓存] [模板代码]
```

### 按钮样式

```html
<el-button 
    class="render-btn" 
    type="primary" 
    size="small"
    @click="renderCurrentProject"
    :loading="isRendering"
    :disabled="!project || !project.id"
    title="渲染节点图片">
    <i class="el-icon-picture" v-if="!isRendering"></i>
    渲染
</el-button>
```

**特点**:
- 主题色（primary）突出重要性
- 小尺寸（small）与其他按钮保持一致
- 加载时显示旋转图标
- 非加载时显示图片图标
- Tooltip 显示功能说明

## 实现流程

### 1. 用户点击渲染按钮

```javascript
renderCurrentProject()
```

### 2. 验证项目状态

```javascript
if (!this.project || !this.project.id) {
    this.$message.warning('请先选择项目');
    return;
}

if (this.isRendering) {
    this.$message.warning('渲染任务正在进行中，请稍候');
    return;
}
```

### 3. 获取渲染节点列表

**API 请求**:
```http
GET /figma/project/:project_id/render_nodes
```

**响应数据**:
```json
{
  "code": 0,
  "message": "获取渲染节点列表成功",
  "data": {
    "file_key": "oNOE3NSRYt023cdpXVHs4y",
    "node_ids": ["1:123", "1:124", "1:125", ...],
    "count": 50
  }
}
```

### 4. 创建渲染队列

**API 请求**:
```http
POST /figma/manual/render
Content-Type: application/json

{
  "file_key": "oNOE3NSRYt023cdpXVHs4y",
  "node_ids": ["1:123", "1:124", ...],
  "format": "png",
  "scale": 1.0
}
```

**响应数据**:
```json
{
  "code": 0,
  "message": "渲染队列已创建",
  "data": {
    "queue_id": 123,
    "node_count": 50,
    "estimated_time": 150
  }
}
```

### 5. 显示成功消息

```javascript
this.$message.success({
    message: `渲染队列已创建，共 ${nodeCount} 个节点\n格式: PNG, 缩放: 1.0x`,
    duration: 3000
});
```

## 核心代码

### HTML 模板

**文件**: `web/templates/dashboard.html`

```html
<!-- 渲染按钮 -->
<el-button 
    class="render-btn" 
    type="primary" 
    size="small"
    @click="renderCurrentProject"
    :loading="isRendering"
    :disabled="!project || !project.id"
    title="渲染节点图片">
    <i class="el-icon-picture" v-if="!isRendering"></i>
    渲染
</el-button>
```

### Vue 数据

**文件**: `web/static/js/dashboard.js`

```javascript
data() {
    return {
        // ... 其他数据
        defaultImageFormat: 'png', // 默认图片格式
        defaultImageScale: 1.0, // 默认图片缩放比例
        isRendering: false, // 是否正在渲染
    }
}
```

### Vue 方法

**文件**: `web/static/js/dashboard.js`

```javascript
// 渲染当前项目的节点
async renderCurrentProject() {
    if (!this.project || !this.project.id) {
        this.$message.warning('请先选择项目');
        return;
    }
    
    if (this.isRendering) {
        this.$message.warning('渲染任务正在进行中，请稍候');
        return;
    }
    
    try {
        this.isRendering = true;
        
        // 1. 获取项目的渲染节点列表
        const nodesResponse = await axios.get(`/figma/project/${this.project.id}/render_nodes`);
        
        if (nodesResponse.data.code !== 0) {
            throw new Error(nodesResponse.data.message || '获取节点列表失败');
        }
        
        const nodeIds = nodesResponse.data.data.node_ids || [];
        const fileKey = nodesResponse.data.data.file_key;
        
        if (nodeIds.length === 0) {
            this.$message.warning('没有可渲染的节点');
            this.isRendering = false;
            return;
        }
        
        // 2. 创建渲染队列
        const renderResponse = await axios.post('/figma/manual/render', {
            file_key: fileKey,
            node_ids: nodeIds,
            format: this.defaultImageFormat,
            scale: this.defaultImageScale
        });
        
        if (renderResponse.data.code === 0) {
            const queueId = renderResponse.data.data.queue_id;
            const nodeCount = renderResponse.data.data.node_count;
            
            this.$message.success({
                message: `渲染队列已创建，共 ${nodeCount} 个节点\n格式: ${this.defaultImageFormat.toUpperCase()}, 缩放: ${this.defaultImageScale}x`,
                duration: 3000
            });
            
            console.log('渲染队列已创建:', {
                queue_id: queueId,
                node_count: nodeCount,
                format: this.defaultImageFormat,
                scale: this.defaultImageScale
            });
        } else {
            throw new Error(renderResponse.data.message || '创建渲染队列失败');
        }
    } catch (error) {
        console.error('渲染失败:', error);
        
        if (error.response) {
            const status = error.response.status;
            const message = error.response.data?.message || error.message;
            
            if (status === 404) {
                this.$alert(
                    '未找到节点树数据，请先刷新节点树',
                    '节点树未找到',
                    {
                        confirmButtonText: '确定',
                        type: 'warning'
                    }
                );
            } else if (status === 400) {
                this.$alert(
                    message,
                    'Figma API 错误',
                    {
                        confirmButtonText: '确定',
                        type: 'error'
                    }
                );
            } else if (status === 429) {
                this.$alert(
                    'Figma API 请求过于频繁，请稍后重试',
                    '速率限制',
                    {
                        confirmButtonText: '确定',
                        type: 'warning'
                    }
                );
            } else {
                this.$message.error(message || '渲染失败');
            }
        } else {
            this.$message.error(error.message || '渲染失败');
        }
    } finally {
        this.isRendering = false;
    }
}
```

## 使用场景

### 场景 1: 首次渲染项目

1. 用户在 Dashboard 编辑页面选择一个项目
2. 调整底部的图片格式（PNG/JPG/SVG）
3. 调整缩放比例（0.5x - 4.0x）
4. 点击"渲染"按钮
5. 系统自动获取所有需要渲染的节点
6. 创建渲染队列，后台开始渲染
7. 显示成功消息

### 场景 2: 节点修改后重新渲染

1. 用户修改了节点的属性（如 ResMode、Component 等）
2. 保存修改
3. 点击"渲染"按钮重新生成图片
4. 清除旧缓存（可选）
5. 渲染完成后刷新预览

### 场景 3: 不同格式/比例渲染

1. 用户切换图片格式为 JPG
2. 切换缩放比例为 2.0x
3. 点击"渲染"按钮
4. 系统使用新的格式和比例创建渲染队列
5. 生成高清图片资源

## 与 Projects 页面的区别

| 特性 | Projects 页面 | Dashboard 页面 |
|------|--------------|---------------|
| **渲染方式** | 需要弹框选择格式/比例 | 直接使用当前设置 |
| **节点选择** | 支持选择特定节点 | 渲染所有项目节点 |
| **批量操作** | 支持渲染整个分组 | 单个项目渲染 |
| **用户体验** | 需要2步操作 | 1步完成 |
| **适用场景** | 精确控制渲染参数 | 快速批量渲染 |

## 错误处理

### 1. 节点树未找到 (404)

**场景**: 项目创建后从未刷新节点树

**提示**:
```
节点树未找到

未找到节点树数据，请先刷新节点树

[确定]
```

**解决方案**: 点击顶部的"刷新节点树"按钮

### 2. Figma API 错误 (400)

**场景**: Figma API 返回错误（如 Token 无效）

**提示**:
```
Figma API 错误

{具体错误信息}

[确定]
```

**解决方案**: 检查 Figma Token 是否有效，或联系管理员

### 3. 速率限制 (429)

**场景**: 请求过于频繁，触发 Figma 速率限制

**提示**:
```
速率限制

Figma API 请求过于频繁，请稍后重试

[确定]
```

**解决方案**: 等待 30 秒后重试

### 4. 没有可渲染的节点

**场景**: 所有节点都被标记为 ignore

**提示**:
```
没有可渲染的节点
```

**解决方案**: 检查节点设置，取消部分节点的 ignore 标记

### 5. 重复渲染

**场景**: 用户在渲染进行中再次点击按钮

**提示**:
```
渲染任务正在进行中，请稍候
```

**解决方案**: 等待当前渲染完成

## 状态流转

```
[空闲] 
  ↓ 点击"渲染"
[渲染中] (按钮 loading, 禁用)
  ↓ 获取节点列表
[创建队列]
  ↓ 成功
[完成] → [空闲]
  ↓ 失败
[错误提示] → [空闲]
```

## 性能优化

### 1. 防抖处理

```javascript
if (this.isRendering) {
    this.$message.warning('渲染任务正在进行中，请稍候');
    return;
}
```

**优点**: 防止用户快速多次点击造成重复请求

### 2. 异步操作

```javascript
async renderCurrentProject() {
    // 使用 async/await 确保顺序执行
    const nodesResponse = await axios.get(...);
    const renderResponse = await axios.post(...);
}
```

**优点**: 确保获取节点列表后再创建渲染队列

### 3. 错误恢复

```javascript
finally {
    this.isRendering = false;
}
```

**优点**: 无论成功或失败，都重置状态，确保按钮可再次使用

## 后续优化建议

### 1. 渲染进度显示

在底部控制栏增加渲染进度条，实时显示：
- 队列中的节点总数
- 已完成的节点数
- 失败的节点数

### 2. 渲染历史

记录用户的渲染历史：
- 渲染时间
- 图片格式和比例
- 节点数量
- 渲染状态

### 3. 智能推荐

根据项目特点推荐最佳渲染参数：
- UI 界面建议 PNG + 2.0x
- 图标建议 SVG
- 照片建议 JPG

### 4. 批量格式渲染

支持一次渲染多种格式/比例组合：
```javascript
[
  { format: 'png', scale: 1.0 },
  { format: 'png', scale: 2.0 },
  { format: 'jpg', scale: 1.0 }
]
```

## 相关文档

- [Figma 缓存系统完整方案](FIGMA_CACHE_SYSTEM_COMPLETE.md)
- [渲染队列实现](SCHEDULER_IMPLEMENTATION.md)
- [Projects 页面渲染功能](PROJECTS_RENDER_FEATURE.md)

---

**实现时间**: 2025-11-19  
**修改文件**:
- `web/templates/dashboard.html` - 添加渲染按钮 UI
- `web/static/js/dashboard.js` - 添加渲染逻辑和状态管理

**测试状态**: ✅ 编译通过，等待实际运行验证

