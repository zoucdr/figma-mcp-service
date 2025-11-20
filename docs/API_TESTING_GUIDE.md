# 缓存与队列 API 测试指南

## 概述

本文档提供缓存与队列管理系统的 API 测试方法和示例。

## 前提条件

1. **启动服务**
   ```bash
   cd d:\figma-deliver\service
   .\figma-deliver.exe
   ```

2. **登录获取 Session**
   - 访问 `http://localhost:8080/login`
   - 登录后浏览器会保存 session cookie

3. **配置 Figma Token**
   - 访问个人资料页面
   - 配置你的 Figma Token

## API 接口列表

### 1. 节点树刷新请求

**接口**: `POST /figma/file/refresh`

**说明**: 强制将节点树加入刷新队列

**请求示例**:
```bash
curl -X POST http://localhost:8080/figma/file/refresh \
  -H "Content-Type: application/json" \
  -b "figma-deliver-session=YOUR_SESSION_COOKIE" \
  -d '{
    "file_key": "abc123xyz",
    "root_node_id": "1:123"
  }'
```

**响应示例**:
```json
{
  "code": 0,
  "message": "已加入刷新队列",
  "data": {
    "cache_id": 1,
    "status": "waiting",
    "next_available_time": 1732003800,
    "estimated_wait_seconds": 25
  }
}
```

**状态说明**:
- `waiting`: 等待处理
- `loading`: 正在加载
- `loaded`: 已加载成功
- `error`: 加载失败

---

### 2. 获取节点树请求状态

**接口**: `GET /figma/file/cache/status/:cache_id`

**说明**: 查询节点树缓存状态，如果已加载成功会返回文件数据

**请求示例**:
```bash
curl http://localhost:8080/figma/file/cache/status/1 \
  -b "figma-deliver-session=YOUR_SESSION_COOKIE"
```

**响应示例** (waiting):
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "cache_id": 1,
    "status": "waiting",
    "progress": 0,
    "error_message": "",
    "created_at": 1732003200,
    "updated_at": 1732003200
  }
}
```

**响应示例** (loaded):
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "cache_id": 1,
    "status": "loaded",
    "progress": 100,
    "file_data": "{...JSON数据...}",
    "error_message": "",
    "created_at": 1732003200,
    "updated_at": 1732003290
  }
}
```

---

### 3. 项目渲染请求

**接口**: `POST /figma/project/:project_id/render`

**说明**: 渲染项目中的所有节点（包括依赖节点）

**请求示例**:
```bash
curl -X POST http://localhost:8080/figma/project/1/render \
  -H "Content-Type: application/json" \
  -b "figma-deliver-session=YOUR_SESSION_COOKIE" \
  -d '{
    "project_id": 1,
    "format": "png",
    "scale": 2.0,
    "include_ref_nodes": true
  }'
```

**参数说明**:
- `format`: 图片格式，可选值：`png`, `jpg`, `svg`
- `scale`: 缩放比例，范围：0.5 ~ 4.0
- `include_ref_nodes`: 是否包含依赖节点

**响应示例**:
```json
{
  "code": 0,
  "message": "已加入渲染队列",
  "data": {
    "queue_ids": [123, 124, 125],
    "total_nodes": 2500,
    "estimated_duration_seconds": 5030
  }
}
```

---

### 4. 手动触发渲染

**接口**: `POST /figma/manual/render`

**说明**: 手动指定节点ID列表进行渲染

**请求示例**:
```bash
curl -X POST http://localhost:8080/figma/manual/render \
  -H "Content-Type: application/json" \
  -b "figma-deliver-session=YOUR_SESSION_COOKIE" \
  -d '{
    "file_key": "abc123xyz",
    "node_ids": ["1:123", "1:456", "1:789"],
    "format": "png",
    "scale": 2.0
  }'
```

**响应示例**:
```json
{
  "code": 0,
  "message": "已加入渲染队列",
  "data": {
    "queue_ids": [126],
    "total_nodes": 3
  }
}
```

---

### 5. 获取渲染队列信息

**接口**: `GET /figma/render/queue?status=waiting,processing`

**说明**: 查询当前用户的渲染队列

**请求示例**:
```bash
curl "http://localhost:8080/figma/render/queue?status=waiting,processing" \
  -b "figma-deliver-session=YOUR_SESSION_COOKIE"
```

**参数说明**:
- `status`: 队列状态过滤，逗号分隔，可选值：
  - `waiting`: 等待处理
  - `processing`: 正在处理
  - `completed`: 已完成
  - `error`: 处理失败
  - `cancelled`: 已取消

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "queues": [
      {
        "id": 123,
        "file_key": "abc123xyz",
        "format": "png",
        "scale": 2.0,
        "status": "processing",
        "progress": 65,
        "total_nodes": 1000,
        "processed_nodes": 650,
        "failed_nodes": 5,
        "next_available_time": 0,
        "created_at": 1732003200,
        "updated_at": 1732003530
      }
    ],
    "total": 1
  }
}
```

---

### 6. 获取队列统计信息

**接口**: `GET /figma/render/queue/stats`

**说明**: 获取当前用户的队列统计信息

**请求示例**:
```bash
curl http://localhost:8080/figma/render/queue/stats \
  -b "figma-deliver-session=YOUR_SESSION_COOKIE"
```

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total_queues": 3,
    "waiting_count": 1,
    "processing_count": 2,
    "total_nodes": 2500,
    "processed_nodes": 1650,
    "overall_progress": 66
  }
}
```

---

### 7. 获取节点图片

**接口**: `GET /figma/node/image?file_key=xxx&node_id=xxx&format=png&scale=2.0`

**说明**: 获取节点图片，如果有缓存直接重定向到图片URL，否则加入渲染队列

**请求示例**:
```bash
curl "http://localhost:8080/figma/node/image?file_key=abc123xyz&node_id=1:123&format=png&scale=2.0" \
  -b "figma-deliver-session=YOUR_SESSION_COOKIE" \
  -L  # 自动跟随重定向
```

**参数说明**:
- `file_key`: Figma 文件 Key（必需）
- `node_id`: 节点 ID（必需）
- `format`: 图片格式（可选，默认 png）
- `scale`: 缩放比例（可选，默认 1.0）

**响应**:

**情况1**: 有缓存
- HTTP 302 重定向到图片 URL（OBS 或 Figma CDN）

**情况2**: 无缓存
- HTTP 202 Accepted
```json
{
  "code": 202,
  "message": "图片正在生成，请稍后重试",
  "data": {
    "status": "pending"
  }
}
```

---

## 测试流程

### 场景1: 刷新节点树

1. **发起刷新请求**
   ```bash
   curl -X POST http://localhost:8080/figma/file/refresh \
     -H "Content-Type: application/json" \
     -b "cookies.txt" \
     -d '{"file_key": "test123", "root_node_id": "1:1"}'
   ```

2. **轮询查询状态**
   ```bash
   # 获取返回的 cache_id，例如 1
   curl http://localhost:8080/figma/file/cache/status/1 -b "cookies.txt"
   ```

3. **检查状态变化**
   - `waiting` → `loading` → `loaded`

### 场景2: 渲染项目图片

1. **创建渲染任务**
   ```bash
   curl -X POST http://localhost:8080/figma/project/1/render \
     -H "Content-Type: application/json" \
     -b "cookies.txt" \
     -d '{
       "project_id": 1,
       "format": "png",
       "scale": 2.0,
       "include_ref_nodes": true
     }'
   ```

2. **查询队列状态**
   ```bash
   curl "http://localhost:8080/figma/render/queue?status=processing,waiting" \
     -b "cookies.txt"
   ```

3. **查看统计信息**
   ```bash
   curl http://localhost:8080/figma/render/queue/stats -b "cookies.txt"
   ```

### 场景3: 获取节点图片

1. **首次请求**（无缓存）
   ```bash
   curl "http://localhost:8080/figma/node/image?file_key=test123&node_id=1:123&format=png&scale=2.0" \
     -b "cookies.txt" \
     -v
   ```
   - 返回 202，表示正在生成

2. **等待渲染完成**
   - 查询队列状态，等待 `status` 变为 `completed`

3. **再次请求**（有缓存）
   ```bash
   curl "http://localhost:8080/figma/node/image?file_key=test123&node_id=1:123&format=png&scale=2.0" \
     -b "cookies.txt" \
     -L -o image.png
   ```
   - 返回 302 重定向到图片URL，图片保存为 `image.png`

---

## 错误处理

### 401 Unauthorized
```json
{
  "error": "未登录"
}
```
**解决**: 先登录获取 session cookie

### 400 Bad Request
```json
{
  "error": "未配置 Figma Token"
}
```
**解决**: 在个人资料页面配置 Figma Token

### 404 Not Found
```json
{
  "error": "项目不存在"
}
```
**解决**: 检查 project_id 是否正确

---

## 使用 Postman 测试

1. **设置环境变量**
   - `base_url`: `http://localhost:8080`
   - `session_cookie`: 登录后获取的 cookie 值

2. **配置认证**
   - 在 Headers 中添加：
     ```
     Cookie: figma-deliver-session={{session_cookie}}
     ```

3. **导入测试集合**（可选）
   - 根据上述示例创建 Postman Collection
   - 保存常用请求以便重复测试

---

## 监控日志

启动服务后，日志会显示：

```
✅ [Token:figd_abc12...] File API 冷却完成，可以调用
📝 [Token:figd_abc12...] 已更新 File API 请求时间
🆕 [FileCache] 已创建缓存记录 id=1, fileKey=test123, rootNodeID=1:1, status=waiting
✅ [Queue] 共创建 3 个渲染队列，总节点数=2500
📋 [Queue] 当前有 3 个等待处理的队列
```

---

## 下一步

- ⏳ **实现定时任务**: 后台自动处理渲染队列
- ⏳ **集成华为云 OBS**: 图片持久化存储
- ⏳ **前端界面**: 添加渲染按钮和进度条

---

## 常见问题

### Q: 为什么请求被拒绝？
A: 检查 Token 冷却时间（30秒），查看日志中的冷却提示。

### Q: 队列一直是 waiting 状态？
A: 需要实现定时任务处理器才能自动处理队列。目前只是创建了队列记录。

### Q: 如何清空测试数据？
A: 直接清空数据库表：
```sql
TRUNCATE TABLE figma_token_cooldowns;
TRUNCATE TABLE figma_file_caches;
TRUNCATE TABLE figma_render_queues;
TRUNCATE TABLE figma_node_images;
```

