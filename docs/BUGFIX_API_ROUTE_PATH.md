# 修复 API 路由路径不匹配问题

## 问题描述

前端调用渲染节点接口时返回 404 错误：

```
[GIN] 2025/11/19 - 17:35:42 | 404 | 0s | 127.0.0.1 | GET "/api/figma/project/8/render_nodes"
```

## 问题原因

**路径不一致**：
- ❌ 前端调用：`/api/figma/project/:project_id/render_nodes`
- ✅ 后端路由：`/figma/project/:project_id/render_nodes`

前端多了一个 `/api` 前缀。

## 解决方案

### 统一使用 `/figma/...` 路径

#### 修改的文件

**1. `web/static/js/projects.js`** (3处修改)

```javascript
// ❌ 修改前
axios.get(`/api/figma/project/${project.id}/render_nodes`)
axios.post('/api/figma/manual/render', {...})

// ✅ 修改后
axios.get(`/figma/project/${project.id}/render_nodes`)
axios.post('/figma/manual/render', {...})
```

**2. `web/static/js/figma-render.js`** (3处修改)

```javascript
// ❌ 修改前
axios.post('/api/figma/manual/render', {...})
axios.get('/api/figma/render/queue')
axios.get('/api/figma/render/queue/stats')

// ✅ 修改后
axios.post('/figma/manual/render', {...})
axios.get('/figma/render/queue')
axios.get('/figma/render/queue/stats')
```

## 路由规范

### ✅ 正确的路由路径

所有 Figma 相关的 API 统一使用 `/figma/...` 前缀：

| 功能 | 路径 |
|------|------|
| 获取渲染节点 | `GET /figma/project/:project_id/render_nodes` |
| 手动渲染 | `POST /figma/manual/render` |
| 项目渲染 | `POST /figma/project/:project_id/render` |
| 获取渲染队列 | `GET /figma/render/queue` |
| 获取队列统计 | `GET /figma/render/queue/stats` |
| 获取节点图片 | `GET /figma/node/image` |
| 解析链接 | `POST /figma/parse` |
| 获取节点 | `GET /figma/node/:file_key/:node_id` |

### 🔍 其他 API 路由组

- `/api/:mcptoken/...` - MCP API（无需登录，通过 token 验证）
- `/figma/...` - Figma API（需要登录验证）
- `/mcp/:connection_id` - Cursor MCP（通过 connection_id 验证）

## 后端路由注册

```go
// main.go

// Figma API路由组
figmaGroup := r.Group("/figma")
figmaGroup.Use(middleware.RequireLogin())

// 渲染相关路由
figmaGroup.GET("/project/:project_id/render_nodes", controllers.GetFigmaRenderNodeInfo)
figmaGroup.POST("/manual/render", cacheController.ManualRender)
figmaGroup.GET("/render/queue", cacheController.GetRenderQueue)
figmaGroup.GET("/render/queue/stats", cacheController.GetQueueStatistics)
figmaGroup.GET("/node/image", cacheController.GetNodeImage)
```

## 验证方法

### 1. 检查路由是否正确注册

```bash
# 启动服务后，查看日志
# 应该能看到类似输出：
# [GIN-debug] GET    /figma/project/:project_id/render_nodes
```

### 2. 测试 API 调用

```bash
# 使用 curl 测试（需要登录 cookie）
curl -X GET "http://localhost:8080/figma/project/8/render_nodes" \
  -H "Cookie: figma-deliver-session=xxx"
```

### 3. 前端测试

1. 打开浏览器开发者工具（F12）
2. 切换到 Network 标签
3. 点击"渲染"按钮
4. 检查请求路径是否为 `/figma/project/8/render_nodes`（不应该有 `/api` 前缀）

## 总结

✅ 统一了所有渲染相关 API 的路由前缀  
✅ 前端和后端路径完全匹配  
✅ 修复了 404 错误  
✅ 遵循了现有的路由规范

---

**修复时间**: 2025-11-19  
**影响范围**: 
- `web/static/js/projects.js`
- `web/static/js/figma-render.js`
- 所有渲染相关的前端功能

