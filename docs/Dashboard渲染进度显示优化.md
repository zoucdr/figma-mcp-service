# Dashboard 渲染进度显示优化

## 问题描述

Dashboard 界面在点击渲染后存在以下问题：

1. **缺少进度监控**：渲染请求提交后，没有自动开始监控进度
2. **等待时间不准确**：`waiting` 状态下，使用的是 `next_available_time`（已废弃），无法准确显示等待时间
3. **缺少延迟查询**：渲染请求提交后立即查询可能查不到队列数据

## 解决方案

### 1. 后端修改

#### 1.1 添加冷却时间查询方法 (`internal/services/cache.go`)

```go
// GetTokenCooldownRemaining 获取 Token 冷却剩余时间（秒）
// 用于计算等待时间，取 File API 和 Images API 中较大的冷却时间
func (cs *CacheService) GetTokenCooldownRemaining(token string) (uint32, error) {
    // 获取 File API 冷却剩余时间
    _, fileRemaining := models.CheckFileAPICooldown(token, cs.FileAPICooldown)
    
    // 获取 Images API 冷却剩余时间
    _, imageRemaining := models.CheckImageAPICooldown(token, cs.ImagesAPICooldown)
    
    // 返回较大的冷却时间
    if fileRemaining > imageRemaining {
        return fileRemaining, nil
    }
    return imageRemaining, nil
}
```

**功能**：
- 从 `figma_token_cooldowns` 表查询 Token 的冷却剩余时间
- 取 File API 和 Images API 中较大的冷却时间
- 返回秒数，用于前端显示

#### 1.2 修改进度查询 API (`internal/controllers/cache.go`)

**接口**: `GET /api/figma/project/:project_id/render/progress`

**新增响应字段**:
```json
{
    "code": 0,
    "message": "success",
    "data": {
        "has_render": true,
        "render_id": 123,
        "status": "waiting",
        "progress": 0,
        "total_nodes": 100,
        "processed_nodes": 0,
        "failed_nodes": 0,
        "cooldown_remaining": 25,  // ✅ 新增：冷却剩余时间（秒）
        "error_message": "",
        "started_at": 1234567890,
        "completed_at": 0,
        "created_at": 1234567890,
        "updated_at": 1234567890
    }
}
```

**实现逻辑**:
```go
// 如果状态是 waiting，计算等待时间（从 figma_token_cooldowns 表）
if queue.Status == "waiting" {
    cooldownSeconds, err := cc.cacheService.GetTokenCooldownRemaining(queue.FigmaToken)
    if err == nil {
        responseData["cooldown_remaining"] = cooldownSeconds
    } else {
        responseData["cooldown_remaining"] = 0
    }
}
```

### 2. 前端修改

#### 2.1 修改进度数据结构 (`web/static/js/dashboard.js`)

```javascript
FigmaRenderManager.startProjectProgressPolling(
    this.project.id,
    // 进度回调
    (progress) => {
        this.projectRenderProgress = {
            hasRender: progress.has_render,
            renderId: progress.render_id,
            status: progress.status,
            progress: progress.progress || 0,
            processedNodes: progress.processed_nodes || 0,
            failedNodes: progress.failed_nodes || 0,
            totalNodes: progress.total_nodes || 0,
            cooldownRemaining: progress.cooldown_remaining || 0, // ✅ 新增
            nextAvailableTime: progress.next_available_time || 0, // 兼容旧字段
            message: this.formatRenderMessage(progress)
        };
    },
    ...
);
```

#### 2.2 修改消息格式化逻辑 (`web/static/js/dashboard.js`)

```javascript
switch (progress.status) {
    case 'waiting':
        // ✅ 优先使用 cooldown_remaining，如果没有则使用 next_available_time
        let waitSeconds = 0;
        if (progress.cooldown_remaining && progress.cooldown_remaining > 0) {
            waitSeconds = progress.cooldown_remaining;
        } else if (progress.next_available_time && progress.next_available_time > 0) {
            waitSeconds = this.calculateWaitSeconds(progress.next_available_time);
        }
        
        if (waitSeconds > 0) {
            return `等待中... 预计等待 ${this.formatWaitTime(waitSeconds)}`;
        }
        return '等待处理...';
    case 'processing':
        // ...
}
```

#### 2.3 添加延迟查询逻辑 (`web/static/js/dashboard.js`)

```javascript
if (renderResponse.data.code === 0) {
    // ... 显示成功消息 ...
    
    // ✅ 延迟 1 秒后开始查询渲染进度
    setTimeout(() => {
        console.log('📊 [Dashboard] 开始监控渲染进度...');
        this.startWatchingProjectRender();
    }, 1000);
}
```

#### 2.4 修改 HTML 模板 (`web/templates/dashboard.html`)

**渲染进度卡片**:
```html
<div class="status-row" v-if="projectRenderProgress.status === 'waiting' && (projectRenderProgress.cooldownRemaining > 0 || projectRenderProgress.nextAvailableTime > 0)">
    <span class="status-label">等待时间：</span>
    <span class="status-value wait-time">
        {{ formatWaitTime(projectRenderProgress.cooldownRemaining || calculateWaitSeconds(projectRenderProgress.nextAvailableTime)) }}
    </span>
</div>
```

**底部状态栏**:
```html
<template v-if="projectRenderProgress.status === 'waiting'">
    渲染等待中
    <span v-if="projectRenderProgress.cooldownRemaining > 0 || projectRenderProgress.nextAvailableTime > 0" class="wait-time-hint">
        (预计等待 {{ formatWaitTime(projectRenderProgress.cooldownRemaining || calculateWaitSeconds(projectRenderProgress.nextAvailableTime)) }})
    </span>
</template>
```

## 工作流程

```
┌─────────────┐
│ 用户点击渲染 │
└──────┬──────┘
       │
       ▼
┌─────────────────────┐
│ POST /project/:id/render │
└──────┬──────────────┘
       │
       ▼
┌─────────────────┐
│ 创建渲染队列成功  │
└──────┬──────────┘
       │
       ▼
┌─────────────────┐
│ 显示成功消息     │
└──────┬──────────┘
       │
       ▼ (延迟 1 秒)
┌───────────────────────────┐
│ 开始监控渲染进度          │
│ startWatchingProjectRender() │
└──────┬────────────────────┘
       │
       ▼
┌────────────────────────────┐
│ 每 2 秒查询一次            │
│ GET /project/:id/render/progress │
└──────┬─────────────────────┘
       │
       ├──► status = waiting
       │     ├─► 显示等待中
       │     └─► 显示冷却剩余时间
       │          (从 figma_token_cooldowns 表)
       │
       ├──► status = processing
       │     ├─► 显示处理中
       │     └─► 显示进度条
       │
       └──► status = completed/failed
             └─► 停止监控
                 └─► 刷新页面数据
```

## 数据流向

```
┌──────────────────┐
│ figma_render_queues │
│ - id               │
│ - figma_token      │
│ - status (waiting) │
│ - total_nodes      │
│ - processed_nodes  │
└────────┬───────────┘
         │
         │ (关联查询)
         ▼
┌──────────────────────┐
│ figma_token_cooldowns │
│ - figma_token         │
│ - last_file_request_time   │
│ - last_image_request_time  │
│ - retry_after_seconds      │
│ - retry_after_set_at       │
└────────┬──────────────┘
         │
         │ (计算)
         ▼
    cooldown_remaining
    (剩余冷却秒数)
         │
         │ (返回给前端)
         ▼
┌─────────────────┐
│ Dashboard UI    │
│ - 等待时间: 25秒 │
│ - 进度: 0/100   │
└─────────────────┘
```

## 关键改进点

### ✅ 1. 准确的等待时间

- **问题**: 使用 `next_available_time` 字段（已废弃）
- **解决**: 从 `figma_token_cooldowns` 表实时计算
- **效果**: 等待时间准确反映 Token 冷却状态

### ✅ 2. 自动开始监控

- **问题**: 渲染请求后需要手动刷新页面
- **解决**: 延迟 1 秒后自动开始轮询
- **效果**: 用户体验流畅，无需手动操作

### ✅ 3. 延迟查询

- **问题**: 立即查询可能查不到队列数据
- **解决**: 延迟 1 秒再开始查询
- **效果**: 给数据库写入留出时间

### ✅ 4. 兼容旧字段

- **问题**: 已有代码依赖 `next_available_time`
- **解决**: 同时支持新旧字段，优先使用新字段
- **效果**: 平滑过渡，不影响已有功能

## 测试验证

### 1. 渲染请求测试

```javascript
// 1. 打开 Dashboard
// 2. 点击"渲染项目"按钮
// 3. 观察控制台输出
// 应该看到：
// ✅ [Dashboard] 渲染队列已创建: { queue_ids: [1], total_nodes: 100, ... }
// (等待 1 秒)
// 📊 [Dashboard] 开始监控渲染进度...
```

### 2. 等待时间显示测试

```javascript
// 1. 快速连续点击渲染（触发冷却）
// 2. 观察进度卡片
// 应该看到：
// 状态：等待中
// 等待时间：25秒 (动态倒计时)
// 进度：0 / 100 节点
```

### 3. 处理中状态测试

```javascript
// 1. 等待冷却结束
// 2. 观察进度卡片
// 应该看到状态变化：
// 等待中 → 渲染中
// 进度条开始增长
// 节点数逐渐增加
```

### 4. API 响应测试

```bash
# 查询渲染进度
curl -X GET "http://localhost:8080/api/figma/project/1/render/progress" \
  -H "Cookie: session=xxx"

# 响应示例（waiting 状态）
{
  "code": 0,
  "message": "success",
  "data": {
    "has_render": true,
    "status": "waiting",
    "cooldown_remaining": 25,  // ✅ 冷却剩余时间
    "progress": 0,
    "total_nodes": 100,
    "processed_nodes": 0,
    "failed_nodes": 0
  }
}

# 响应示例（processing 状态）
{
  "code": 0,
  "message": "success",
  "data": {
    "has_render": true,
    "status": "processing",
    "progress": 45,
    "total_nodes": 100,
    "processed_nodes": 45,
    "failed_nodes": 2
  }
}
```

## UI 显示效果

### 等待中状态

```
┌─────────────────────────────┐
│ ⏰ 渲染进度              [▲] │
├─────────────────────────────┤
│ 状态：[等待中]               │
│ 等待时间：25秒               │
│ 进度：0 / 100 节点           │
│ ▓░░░░░░░░░░░░░░░░░░░░ 0%     │
└─────────────────────────────┘
```

### 处理中状态

```
┌─────────────────────────────┐
│ ⟳ 渲染进度              [▲] │
├─────────────────────────────┤
│ 状态：[渲染中]               │
│ 进度：45 / 100 节点          │
│ ▓▓▓▓▓▓▓▓▓░░░░░░░░░░░░ 45%    │
└─────────────────────────────┘
```

### 底部状态栏

**等待中**:
```
⏰ 渲染等待中 (预计等待 25秒)   0 / 100 节点   ░░░░░░░░░░ 0%   ℹ️
```

**处理中**:
```
⟳ 渲染进行中   45 / 100 节点   ▓▓▓▓▓░░░░░ 45%   ℹ️
```

## 总结

这次优化实现了：

1. ✅ **延迟 1 秒查询**：渲染请求后自动开始监控
2. ✅ **准确的等待时间**：从 `figma_token_cooldowns` 表实时计算
3. ✅ **实时进度显示**：waiting 和 processing 状态都有清晰的提示
4. ✅ **良好的用户体验**：无需手动刷新，进度自动更新
5. ✅ **向下兼容**：同时支持新旧字段，平滑过渡

系统现在能够准确显示渲染进度和等待时间，大大提升了用户体验。

