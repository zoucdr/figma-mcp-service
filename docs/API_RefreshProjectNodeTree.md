# RefreshProjectNodeTree API 响应状态说明

## API 端点
```
POST /api/cache/projects/:project_id/refresh
```

## 响应状态类型

后端会根据不同情况返回不同的 `status` 字段，前端需要根据此字段显示相应的提示信息。

---

### 1. ✅ 刷新成功 (`status: "success"`)

**HTTP 状态码**: `200 OK`

**响应格式**:
```json
{
  "success": true,
  "status": "success",
  "message": "节点树刷新成功",
  "nodes": [...]
}
```

**前端提示**:
```
✅ 节点树刷新成功
```

**说明**: Token 冷却完成，直接调用 Figma API 成功获取最新数据。

---

### 2. ⏰ 已加入队列 (`status: "queued"`)

**HTTP 状态码**: `202 Accepted`

**响应格式**:
```json
{
  "success": true,
  "status": "queued",
  "message": "请求已加入队列，等待处理",
  "cooldown_remaining": 25,
  "next_available_time": 1732089125
}
```

**前端提示**:
```
⏰ 请求已加入队列，预计 {cooldown_remaining} 秒后处理
您可以稍后查看刷新结果
```

**说明**: Token 还在冷却期，请求已加入队列，调度器会在冷却完成后自动处理。

**前端处理建议**:
- 显示倒计时
- 禁用刷新按钮
- 可选：轮询查询缓存状态

---

### 3. 📦 使用缓存数据 (`status: "cached"`)

**HTTP 状态码**: `200 OK`

**响应格式**:
```json
{
  "success": true,
  "status": "cached",
  "message": "Figma API 请求失败，已返回缓存数据",
  "warning": "数据可能不是最新的",
  "nodes": [...],
  "error": "API 错误详情"
}
```

**前端提示**:
```
⚠️ 无法连接 Figma API，已显示缓存数据
数据可能不是最新的
```

**说明**: Token 冷却完成，尝试调用 Figma API 失败，从数据库返回旧缓存数据。

---

### 4. ❌ 刷新失败 (`status: "error"`)

**HTTP 状态码**: `500 Internal Server Error`

**响应格式**:
```json
{
  "success": false,
  "status": "error",
  "error": "获取Figma节点树失败: 具体错误信息"
}
```

**前端提示**:
```
❌ 刷新失败: {error}
```

**说明**: Token 冷却完成，调用 Figma API 失败，且数据库中也没有可用的缓存数据。

---

## 前端实现示例

```javascript
async function refreshProjectNodeTree(projectId) {
    try {
        const response = await fetch(`/api/cache/projects/${projectId}/refresh`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            }
        });
        
        const data = await response.json();
        
        switch (data.status) {
            case 'success':
                // 刷新成功
                showSuccessMessage('✅ 节点树刷新成功');
                // 更新节点树数据
                updateNodeTree(data.nodes);
                break;
                
            case 'queued':
                // 已加入队列
                showInfoMessage(`⏰ 请求已加入队列，预计 ${data.cooldown_remaining} 秒后处理`);
                // 可选：开始倒计时
                startCountdown(data.cooldown_remaining);
                // 可选：轮询查询状态
                pollCacheStatus(projectId);
                break;
                
            case 'cached':
                // 使用缓存数据
                showWarningMessage('⚠️ 无法连接 Figma API，已显示缓存数据（可能不是最新）');
                // 更新节点树数据
                updateNodeTree(data.nodes);
                break;
                
            case 'error':
                // 刷新失败
                showErrorMessage(`❌ 刷新失败: ${data.error}`);
                break;
                
            default:
                showErrorMessage('❌ 未知的响应状态');
        }
        
    } catch (error) {
        showErrorMessage(`❌ 请求失败: ${error.message}`);
    }
}

// 可选：轮询查询缓存状态
function pollCacheStatus(projectId) {
    const intervalId = setInterval(async () => {
        const response = await fetch(`/api/cache/projects/${projectId}/status`);
        const data = await response.json();
        
        if (data.status === 'loaded') {
            clearInterval(intervalId);
            showSuccessMessage('✅ 节点树已刷新完成');
            // 重新加载节点树
            loadProjectNodeTree(projectId);
        } else if (data.status === 'error') {
            clearInterval(intervalId);
            showErrorMessage('❌ 刷新失败');
        }
    }, 5000); // 每 5 秒查询一次
    
    // 最多查询 60 秒
    setTimeout(() => clearInterval(intervalId), 60000);
}
```

---

## 状态流转图

```
用户点击刷新
      ↓
检查 Token 冷却
      ↓
  还在冷却？
   ↙      ↘
 是         否
  ↓          ↓
queued    调用 API
  ↓          ↓
等待调度    成功？
  ↓        ↙    ↘
loaded  是      否
       ↓         ↓
    success   有缓存？
              ↙    ↘
            是      否
             ↓       ↓
          cached  error
```

---

## 缓存状态查询 API（可选实现）

如果前端需要查询队列中的刷新进度，可以调用：

```
GET /api/cache/projects/:project_id/status
```

**响应格式**:
```json
{
  "status": "waiting",  // waiting, loading, loaded, error
  "next_available_time": 1732089125,
  "updated_at": 1732089100
}
```

---

## 注意事项

1. **队列状态持久化**: 队列请求会保存在数据库的 `figma_file_caches` 表中
2. **自动处理**: 调度器会定期扫描并处理队列中的请求
3. **幂等性**: 多次调用刷新接口不会创建多个队列，而是更新现有队列的时间
4. **前端优化**: 建议在刷新按钮上显示冷却倒计时，避免用户频繁点击

