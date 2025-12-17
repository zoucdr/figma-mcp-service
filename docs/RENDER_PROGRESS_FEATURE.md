# 渲染进度监控功能

## 功能概述

在 Dashboard 界面中，系统会自动监控当前项目的渲染状态，并实时显示渲染进度和预计等待时间。

## 主要功能

### 1. 自动启动监控

当用户打开 Dashboard 页面时，系统会自动检查当前项目是否有正在进行的渲染任务，并开始监控。

**实现位置：**
- `web/static/js/dashboard.js` - `mounted()` 生命周期钩子
- `web/static/js/figma-render.js` - `FigmaRenderManager.startProjectProgressPolling()`

### 2. 渲染状态显示

系统会在两个位置显示渲染状态：

#### 顶部导航栏
- 位置：项目ID和Figma链接之间
- 显示内容：
  - 等待中：显示"等待中 (预计等待时间)"
  - 渲染中：显示"渲染中 X/Y"和进度条

#### 底部状态栏
- 位置：页面最底部（固定位置）
- 显示内容：
  - 左侧：状态图标和文本（包含等待时间提示）
  - 中间：进度条和节点处理数量
  - 右侧：详细信息图标（鼠标悬停显示完整消息）

### 3. 等待时间预估

当渲染任务处于等待状态时，系统会根据后端返回的 `next_available_time` 字段计算并显示预计等待时间。

**时间格式化：**
- 小于60秒：显示为"X秒"
- 60秒-3600秒：显示为"X分Y秒"或"X分钟"
- 大于3600秒：显示为"X小时Y分钟"或"X小时"

**实现函数：**
- `calculateWaitSeconds()` - 计算剩余等待秒数
- `formatWaitTime()` - 格式化时间显示

### 4. 实时轮询更新

系统每2秒自动轮询一次渲染进度，更新显示内容。

**轮询机制：**
- 轮询间隔：2000ms
- 自动停止条件：
  - 渲染完成
  - 渲染失败
  - 渲染取消
  - 没有渲染任务

### 5. 页面刷新后继续监控

即使用户刷新页面，只要项目还有未完成的渲染任务，系统会自动恢复监控。

## 技术实现

### 前端组件

1. **dashboard.html**
   - 添加了等待时间显示的模板代码
   - 引入了 `figma-render.js` 脚本

2. **dashboard.js**
   - 添加了 `calculateWaitSeconds()` 方法
   - 添加了 `formatWaitTime()` 方法
   - 更新了 `formatRenderMessage()` 方法，支持等待时间显示
   - 在 `projectRenderProgress` 数据中添加了 `nextAvailableTime` 字段

3. **figma-render.js**
   - 优化了 `startProjectProgressPolling()` 方法
   - 确保传递所有进度字段（包括 `next_available_time`）
   - 添加了回调函数缓存机制

4. **dashboard.css**
   - 添加了 `.wait-time-hint` 样式类

### 后端API

**接口：** `GET /api/figma/project/:project_id/render/progress`

**返回数据：**
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
    "next_available_time": 1700000000,
    "error_message": "",
    "started_at": "2024-01-01T00:00:00Z",
    "completed_at": null,
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
}
```

**关键字段说明：**
- `has_render`: 是否有渲染任务
- `status`: 渲染状态（waiting/processing/completed/failed/error/cancelled）
- `next_available_time`: Unix时间戳（秒），表示预计可以开始处理的时间
- `progress`: 进度百分比（0-100）
- `processed_nodes`: 已处理节点数
- `total_nodes`: 总节点数

## 测试方法

### 1. 启动渲染任务

1. 登录系统，打开一个项目的 Dashboard 页面
2. 设置图片格式和缩放比例
3. 点击底部的"渲染"按钮
4. 观察页面是否显示渲染状态

### 2. 测试等待状态显示

当系统Token处于冷却期时：
1. 顶部导航栏应显示："等待中 (预计等待 X秒/分钟)"
2. 底部状态栏应显示："渲染等待中 (预计等待 X秒/分钟)"
3. 时间应该每2秒更新一次（倒计时）

### 3. 测试渲染中状态显示

当渲染开始处理时：
1. 顶部导航栏应显示："渲染中 X/Y"和进度条
2. 底部状态栏应显示处理进度和百分比
3. 进度应该实时更新

### 4. 测试页面刷新

1. 在渲染进行中时，刷新页面（F5）
2. 页面加载完成后，应该自动恢复显示渲染状态
3. 进度应该继续更新，不会丢失

### 5. 测试完成状态

当渲染完成时：
1. 应该显示成功提示消息
2. 渲染状态栏应该自动隐藏
3. 页面数据应该自动刷新

## 常见问题

### Q1: 为什么看不到等待时间？
A: 只有在渲染状态为 `waiting` 且后端返回了 `next_available_time` 字段时才会显示等待时间。如果Token没有处于冷却期，则不会显示。

### Q2: 为什么刷新页面后渲染状态消失了？
A: 检查以下几点：
1. 确保 `figma-render.js` 已正确加载
2. 检查浏览器控制台是否有JavaScript错误
3. 确认项目的 `render_id` 字段有值

### Q3: 等待时间为什么不准确？
A: 等待时间是基于Token冷却时间计算的估算值，实际等待时间可能因网络延迟、队列排队等因素有所差异。

### Q4: 如何手动停止监控？
A: 监控会在以下情况自动停止：
- 渲染完成
- 渲染失败
- 离开Dashboard页面
- 切换到其他项目

如需手动停止，可以在浏览器控制台执行：
```javascript
FigmaRenderManager.stopProjectProgressPolling(projectId);
```

## 相关文件

### 前端文件
- `web/templates/dashboard.html` - Dashboard页面模板
- `web/static/js/dashboard.js` - Dashboard主逻辑
- `web/static/js/figma-render.js` - 渲染管理器
- `web/static/css/dashboard.css` - Dashboard样式

### 后端文件
- `internal/controllers/cache.go` - 缓存和渲染控制器
- `internal/services/queue.go` - 渲染队列服务

## 更新日志

### 2024-01-01
- ✅ 添加渲染进度自动监控功能
- ✅ 实现等待时间预估显示
- ✅ 优化轮询机制，支持页面刷新后继续监控
- ✅ 美化渲染状态UI显示
