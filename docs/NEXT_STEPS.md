# 下一步工作计划

## 当前状态

✅ **已完成**:
- 数据库设计和模型（100%）
- 缓存服务、队列服务、OBS服务、调度服务（100%）
- API 接口实现（100%）
- OBS 集成测试通过（100%）
- 定时任务调度器（100%）
- 完整文档（100%）
- 编译通过 ✅

🔄 **服务状态**: 后台运行中

---

## 推荐工作顺序

### 阶段 1: 后端功能测试（本阶段）⭐

#### 1.1 验证调度器运行状态

**检查启动日志**:

在终端或日志文件中查找以下输出：

```
✅ 数据库连接和迁移成功（包含缓存表）
✅ [OBS] 初始化成功
✅ 缓存和队列服务初始化完成
✅ 调度服务初始化完成
🚀 [调度器] 启动成功
   - 渲染队列处理间隔: 5 秒
   - 队列清理间隔: 1 小时
✅ [渲染队列处理器] 已启动
✅ [队列清理器] 已启动
服务器启动在 http://localhost:8080
```

**预期行为**:
- ✅ 调度器每5秒检查一次队列（即使队列为空）
- ✅ 日志中应该看到 `📭 [渲染队列处理器] 无待处理队列`（如果队列为空）

---

#### 1.2 测试 API 接口

**测试 1: 创建渲染队列**

```bash
# 在新的 PowerShell 窗口执行
curl -X POST http://localhost:8080/api/figma/manual/render `
  -H "Content-Type: application/json" `
  -d '{
    "figma_token": "YOUR_FIGMA_TOKEN",
    "file_key": "YOUR_FILE_KEY",
    "node_ids": ["1:2"],
    "format": "png",
    "scale": 2.0
  }'
```

**预期响应**:
```json
{
  "code": 0,
  "message": "渲染队列已创建",
  "data": {
    "queue_id": 1,
    "status": "waiting",
    "total_nodes": 1,
    "message": "队列已创建，正在处理中..."
  }
}
```

**检查日志**:
应该看到：
```
✅ [Queue] 渲染队列创建成功 id=1
📦 [渲染队列处理器] 发现 1 个待处理队列
🔄 [队列 #1] 开始处理
```

---

**测试 2: 查询队列状态**

```bash
curl "http://localhost:8080/api/figma/render/queue?figma_token=YOUR_TOKEN"
```

**预期响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "queues": [
      {
        "id": 1,
        "file_key": "YOUR_FILE_KEY",
        "status": "processing",  // 或 completed
        "progress": 65,
        "total_nodes": 1,
        "processed_nodes": 0,
        "failed_nodes": 0
      }
    ],
    "total": 1
  }
}
```

---

**测试 3: 查询队列统计**

```bash
curl "http://localhost:8080/api/figma/render/queue/stats?figma_token=YOUR_TOKEN"
```

**预期响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total": 1,
    "waiting": 0,
    "processing": 1,
    "completed": 0,
    "failed": 0
  }
}
```

---

#### 1.3 观察自动处理流程

**期望看到的日志序列**:

```
🔄 [队列 #1] 开始处理
   - 文件: YOUR_FILE_KEY
   - Token: YOUR_TOKEN...
   - 节点数: 1

✅ [队列 #1] 获取到 1 个图片 URL

📥 [OBS] 开始下载图片: https://s3-alpha.figma.com/...
✅ [OBS] 图片下载成功，大小: 152847 bytes
✅ [OBS] 上传成功
   - OBS Key: figma_images/YOUR_FILE_KEY/png_2.0x/1-2.png
   - OBS URL: https://...
   - 文件大小: 152847 bytes

✅ [队列 #1] 处理完成: 1/1
```

---

#### 1.4 测试图片获取

```bash
# 等待队列处理完成后
curl -o test_image.png "http://localhost:8080/api/figma/node/image?file_key=YOUR_FILE_KEY&node_id=1:2&format=png&scale=2.0"
```

**预期**:
- ✅ 下载一个 PNG 图片文件
- ✅ 文件可以正常打开

---

### 阶段 2: 实现子树提取功能

根据刚刚更新的文档 `SUBTREE_EXTRACTION_GUIDE.md`，实现以下功能：

#### 2.1 添加模型方法

在 `internal/models/cache.go` 中添加：

```go
// 1. ExtractAllNodeIDs - 提取所有节点ID
// 2. ExtractSubtree - 提取子树
// 3. ContainsNodeID - 检查节点是否存在
// 4. GetFileCacheContainingNode - 查找包含节点的缓存
```

**参考**: `docs/SUBTREE_EXTRACTION_GUIDE.md` 第 "Go 代码实现" 部分

#### 2.2 更新服务层

在 `internal/services/cache.go` 中添加：

```go
// GetFileCacheWithSubtreeExtraction - 支持子树提取的查询
```

#### 2.3 更新控制器

在 `internal/controllers/cache.go` 中修改 `RefreshFile` 方法，使用新的查询方法。

#### 2.4 测试子树提取

```bash
# 先请求完整文件树
curl -X POST http://localhost:8080/api/figma/file/refresh \
  -H "Content-Type: application/json" \
  -d '{"file_key": "YOUR_FILE_KEY", "root_node_id": ""}'

# 等待加载完成后，请求子树（应该从缓存提取，不调用API）
curl -X POST http://localhost:8080/api/figma/file/refresh \
  -H "Content-Type: application/json" \
  -d '{"file_key": "YOUR_FILE_KEY", "root_node_id": "1:2"}'
```

**预期日志**:
```
🔍 [Cache] 从父树提取子树: file=YOUR_FILE_KEY, node=1:2, parent_root=
✅ [Cache] 子树提取成功: 25648 bytes
```

---

### 阶段 3: 前端集成

#### 3.1 添加渲染按钮

**位置**: 项目详情页或节点列表页

**功能**:
- 点击"渲染选中节点"按钮
- 调用 `POST /api/figma/manual/render` API
- 显示队列ID和状态

**示例代码** (React/Vue):

```javascript
async function handleRender() {
  const response = await fetch('/api/figma/manual/render', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      figma_token: userToken,
      file_key: fileKey,
      node_ids: selectedNodeIds,
      format: 'png',
      scale: 2.0
    })
  });
  
  const data = await response.json();
  if (data.code === 0) {
    showMessage('渲染队列已创建: ' + data.data.queue_id);
    startPollingQueue(data.data.queue_id);
  }
}
```

---

#### 3.2 进度条展示

**轮询队列状态**:

```javascript
function startPollingQueue(queueId) {
  const interval = setInterval(async () => {
    const response = await fetch(`/api/figma/render/queue?figma_token=${token}`);
    const data = await response.json();
    
    const queue = data.data.queues.find(q => q.id === queueId);
    if (!queue) {
      clearInterval(interval);
      return;
    }
    
    // 更新进度条
    updateProgressBar(queue.progress);
    updateStatusText(queue.status);
    
    if (queue.status === 'completed' || queue.status === 'failed') {
      clearInterval(interval);
      handleQueueComplete(queue);
    }
  }, 2000); // 每2秒轮询一次
}
```

---

#### 3.3 错误提示弹窗

**处理 429 错误**:

```javascript
async function handleRender() {
  try {
    const response = await fetch('/api/figma/manual/render', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(renderData)
    });
    
    const data = await response.json();
    
    if (data.code === 1001) {
      // Token 冷却中
      showErrorDialog({
        title: 'Token 冷却中',
        message: `需要等待 ${data.data.wait_seconds} 秒后重试`,
        nextTime: data.data.next_available_time
      });
    } else if (data.code === 1002) {
      // Figma API 错误
      showErrorDialog({
        title: 'Figma API 错误',
        message: data.message,
        type: 'error'
      });
    }
  } catch (error) {
    showErrorDialog({
      title: '请求失败',
      message: error.message,
      type: 'error'
    });
  }
}
```

---

### 阶段 4: 测试和优化

#### 4.1 端到端测试

测试完整流程：

1. ✅ 用户点击"渲染"按钮
2. ✅ 创建队列成功
3. ✅ 调度器自动处理
4. ✅ 调用 Figma API
5. ✅ 上传到 OBS
6. ✅ 更新队列状态
7. ✅ 前端显示进度
8. ✅ 完成后显示成功

#### 4.2 错误场景测试

1. **Token 冷却测试**
   - 连续两次请求同一 Token
   - 预期：第二次提示需要等待

2. **队列合并测试**
   - 相同文件的多个请求
   - 预期：自动合并为一个队列

3. **API 失败测试**
   - 使用无效的 Token 或 File Key
   - 预期：队列状态变为 failed

4. **OBS 上传失败测试**
   - 临时禁用 OBS
   - 预期：保留 Figma CDN URL，状态为 figma_only

#### 4.3 性能优化

1. **数据库查询优化**
   - 检查慢查询
   - 添加必要的索引

2. **并发调优**
   - 调整 OBS 并发上传数
   - 优化队列处理间隔

3. **监控指标**
   - API 调用频率
   - 缓存命中率
   - 队列处理速度
   - OBS 上传成功率

---

## 快速测试清单

### ✅ 基础功能测试

- [ ] 服务启动成功
- [ ] 调度器正常运行
- [ ] 创建渲染队列 API
- [ ] 查询队列状态 API
- [ ] 队列自动处理
- [ ] OBS 上传成功
- [ ] 获取节点图片

### ✅ 优化功能测试

- [ ] Token 冷却控制
- [ ] 队列智能合并
- [ ] 子树提取（待实现）
- [ ] 缓存命中率统计

### ✅ 前端集成测试

- [ ] 渲染按钮功能
- [ ] 进度条展示
- [ ] 错误提示弹窗
- [ ] 队列状态实时更新

---

## 当前建议的具体行动

### 立即执行（5分钟）

```bash
# 1. 检查服务是否在运行
# 查看进程
tasklist | findstr figma-deliver

# 2. 测试 API 是否响应
curl http://localhost:8080/api/figma/render/queue/stats?figma_token=test

# 3. 查看日志
# 在运行服务的窗口查看输出
```

### 今天完成（2-3小时）

1. **完整 API 测试**（30分钟）
   - 使用真实的 Figma Token 和 File Key
   - 测试完整的渲染流程
   - 验证 OBS 上传

2. **实现子树提取**（1小时）
   - 复制文档中的代码
   - 添加到相应文件
   - 测试功能

3. **简单前端测试**（1小时）
   - 在现有页面添加"渲染"按钮
   - 实现基础的队列查询
   - 显示简单的状态信息

---

## 需要的准备

1. **Figma Token**: 有效的 Figma Personal Access Token
2. **File Key**: 测试用的 Figma 文件
3. **节点 ID**: 文件中存在的节点 ID

---

## 文档参考

- [完整实施方案](./FIGMA_CACHE_RATE_LIMIT_SOLUTION.md)
- [子树提取指南](./SUBTREE_EXTRACTION_GUIDE.md)
- [API 测试指南](./API_TESTING_GUIDE.md)
- [调度器文档](./SCHEDULER_IMPLEMENTATION.md)
- [后端完成报告](./BACKEND_COMPLETE_SUMMARY.md)

---

## 问题排查

如果遇到问题，检查：

1. **服务未启动**
   ```bash
   cd d:\figma-deliver\service
   .\figma-deliver.exe
   ```

2. **数据库连接失败**
   - 检查 MySQL 是否运行
   - 验证 config.yaml 中的数据库配置

3. **OBS 错误**
   - 检查 OBS 配置是否正确
   - 验证网络连接

4. **API 调用失败**
   - 检查 Figma Token 是否有效
   - 验证 File Key 是否正确

---

**下一步推荐**: 先完成基础功能测试，确保核心流程正常工作，然后再实现优化功能。

