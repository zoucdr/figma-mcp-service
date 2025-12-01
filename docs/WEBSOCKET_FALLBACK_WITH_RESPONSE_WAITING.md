# WebSocket 兜底机制 - 发送请求并监听响应

## 🎯 核心功能

所有通过 WebSocket 兜底的地方，现在会：
1. ✅ **自动发送** `read_my_design` 工具调用到 Figma 插件
2. ✅ **等待响应**（带 30 秒超时）
3. ✅ **自动缓存** 插件返回的数据（本地 + 数据库）
4. ✅ **直接返回** 解析后的节点数据

## 📋 实现细节

### 1. MCP 服务新增方法

#### `SendAndWaitForResponse` - 发送请求并等待响应

```go
// 发送请求并等待响应（带超时）
func (s *MCPService) SendAndWaitForResponse(
    connectionID string, 
    message WSMessage, 
    timeout time.Duration
) (interface{}, error)
```

**工作流程：**
```
1. 创建响应通道 (responseChan, errorChan)
2. 注册待处理请求到 pendingRequests (使用请求ID)
3. 发送 WebSocket 消息
4. 等待三种情况之一：
   ├─ 收到响应 → 返回数据
   ├─ 收到错误 → 返回错误
   └─ 超时 → 返回超时错误
5. 清理待处理请求
```

### 2. 服务层自动处理

#### `GetFigmaNodesWithUser` 完整流程

```go
func GetFigmaNodesWithUser(token, fileKey, nodeID string, userID uint) ([]map[string]interface{}, error) {
    // 1. 检查文件缓存
    cachedNodes, err := loadCachedNodes(fileKey, nodeID)
    if err == nil { return cachedNodes, nil }
    
    // 2. 检查数据库缓存
    dbCache, err := models.GetFileCache(fileKey, nodeID)
    if err == nil { return parseFigmaNodes(dbCache), nil }
    
    // 3. WebSocket 兜底（如果 userID > 0）
    if userID > 0 {
        connection, exists := mcpService.GetUserConnection(userID)
        if exists && connection != nil {
            // 构造 read_my_design 请求
            toolCallMsg := WSMessage{
                Type: "tool_call",
                ID:   fmt.Sprintf("req_service_%d_%s", time.Now().UnixNano(), nodeID),
                Message: map[string]interface{}{
                    "name": "read_my_design",
                    "params": map[string]interface{}{
                        "node_id": nodeID,
                    },
                },
            }
            
            // 发送并等待响应（30秒超时）
            response, err := mcpService.SendAndWaitForResponse(
                connection.ConnectionID, 
                toolCallMsg, 
                30*time.Second
            )
            
            if err != nil {
                return nil, fmt.Errorf("通过 Figma 插件获取数据失败: %v", err)
            }
            
            // 解析响应
            responseData := response.(map[string]interface{})
            
            // 自动缓存到本地和数据库
            go cacheToLocal(fileKey, nodeID, responseData)
            go cacheToDatabase(fileKey, nodeID, responseData)
            
            // 返回解析后的节点
            return parseFigmaNodes(responseData, nodeID), nil
        }
    }
    
    // 4. 无缓存且无 WebSocket，返回错误
    return nil, fmt.Errorf("节点数据缓存未找到")
}
```

### 3. WebSocket 消息处理

#### `HandleFigmaPluginResponse` 响应待处理请求

```go
func (s *MCPService) HandleFigmaPluginResponse(message []byte) error {
    // 解析消息
    var msg struct {
        Type    string                 `json:"type"`
        Message map[string]interface{} `json:"message"`
        ID      string                 `json:"id"`
    }
    json.Unmarshal(message, &msg)
    
    // 如果消息包含 requestID
    if requestID, ok := msg.Message["id"].(string); ok {
        // 检查是否有结果
        if result, hasResult := msg.Message["result"]; hasResult {
            // 响应待处理请求 → 触发 responseChan
            s.RespondToPendingRequest(requestID, result, nil)
        }
        
        // 检查是否有错误
        if errorMsg, hasError := msg.Message["error"]; hasError {
            // 响应待处理请求 → 触发 errorChan
            s.RespondToPendingRequest(requestID, nil, errors.New(errorMsg))
        }
    }
    
    return nil
}
```

#### `RespondToPendingRequest` 发送响应到等待通道

```go
func (s *MCPService) RespondToPendingRequest(requestID string, response interface{}, err error) bool {
    pending, exists := s.pendingRequests[requestID]
    if !exists { return false }
    
    if err != nil {
        pending.ErrorChan <- err  // 触发 SendAndWaitForResponse 的 case err := <-errorChan
    } else {
        pending.ResponseChan <- response  // 触发 SendAndWaitForResponse 的 case response := <-responseChan
    }
    
    return true
}
```

## 🔄 完整数据流

```
┌─────────────────────────────────────────────────────────────┐
│  1. 用户请求节点树                                            │
│     GET /figma/project/:id/node-tree                         │
└────────────────┬────────────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────────────┐
│  2. 控制器调用 GetFigmaNodesWithUser(token, fileKey, nodeID, userID) │
└────────────────┬────────────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────────────┐
│  3. 检查缓存                                                  │
│     ├─ 文件缓存 → 有 ✅ 返回                                 │
│     └─ 数据库缓存 → 有 ✅ 返回                               │
└────────────────┬────────────────────────────────────────────┘
                 │ 无缓存
                 ▼
┌─────────────────────────────────────────────────────────────┐
│  4. WebSocket 兜底（userID > 0）                              │
│     ├─ 检查活跃连接                                          │
│     └─ 构造 read_my_design 请求                              │
└────────────────┬────────────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────────────┐
│  5. SendAndWaitForResponse                                   │
│     ├─ 创建响应通道                                          │
│     ├─ 注册 pendingRequests[requestID]                       │
│     ├─ 发送 WebSocket 消息                                   │
│     └─ 等待... (最多 30 秒)                                  │
└────────────────┬────────────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────────────┐
│  6. Figma 插件接收请求                                        │
│     ├─ 读取当前选中节点设计数据                              │
│     └─ 发送响应到 WebSocket                                  │
└────────────────┬────────────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────────────┐
│  7. MCP 服务接收响应                                          │
│     └─ HandleFigmaPluginResponse(message)                    │
│        └─ RespondToPendingRequest(requestID, result, nil)    │
│           └─ 发送到 responseChan                             │
└────────────────┬────────────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────────────┐
│  8. SendAndWaitForResponse 收到响应                           │
│     case response := <-responseChan:                         │
│        return response, nil                                  │
└────────────────┬────────────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────────────┐
│  9. GetFigmaNodesWithUser 处理响应                            │
│     ├─ 解析节点数据                                          │
│     ├─ 保存到本地缓存 (异步)                                 │
│     ├─ 保存到数据库缓存 (异步)                               │
│     └─ 返回解析后的节点                                      │
└────────────────┬────────────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────────────┐
│  10. 控制器返回成功响应                                       │
│      { "nodes": [...], "source": "plugin" }                  │
└─────────────────────────────────────────────────────────────┘
```

## 📊 超时处理

如果 30 秒内未收到响应：

```go
case <-time.After(30 * time.Second):
    return nil, fmt.Errorf("请求超时，未在 30s 内收到插件响应")
```

**用户体验：**
- ❌ 返回 500 错误："通过 Figma 插件获取数据失败: 请求超时，未在 30s 内收到插件响应"
- 💡 提示用户检查：
  - Figma 插件是否仍在运行
  - 网络连接是否正常
  - 节点数据是否过大

## 🎯 支持 WebSocket 兜底的所有函数

| 函数 | 位置 | 发送请求 | 等待响应 | 自动缓存 |
|------|------|---------|---------|---------|
| `GetFigmaNodesWithUser` | `services/figma.go` | ✅ | ✅ | ✅ |
| `getOptimizedProjectData` | `controllers/api.go` | ✅ | ✅ | ✅ |
| `getOptimizedProjectDataForCache` | `controllers/figma.go` | ✅ | ✅ | ✅ |
| `GetRefNodeDetails` | `controllers/figma.go` | ✅ | ✅ | ✅ |
| `ProcessExportJob` | `services/export.go` | ✅ | ✅ | ✅ |
| `getOptimizedNodesForSwift` | `services/swift_export.go` | ✅ | ✅ | ✅ |
| `getOptimizedNodesForAndroid` | `services/android_export.go` | ✅ | ✅ | ✅ |

## 📝 日志输出

### 发送请求
```
🔌 [GetFigmaNodesWithUser] 发现活跃 WebSocket 连接 (UserID=1, ConnectionID=abc123)，发送插件请求并等待响应
✅ [SendAndWaitForResponse] 已发送请求 (ID=req_service_1234567890_1:123)，等待响应...
```

### 收到响应
```
✅ [SendAndWaitForResponse] 收到响应 (ID=req_service_1234567890_1:123)
✅ [GetFigmaNodesWithUser] 收到插件响应，解析节点数据
✅ [GetFigmaNodesWithUser] 已保存到本地缓存
✅ [GetFigmaNodesWithUser] 已创建数据库缓存
✅ [GetFigmaNodesWithUser] 从 WebSocket 获取并解析了 15 个节点
```

### 错误情况
```
❌ [SendAndWaitForResponse] 收到错误响应 (ID=req_service_1234567890_1:123): 节点未找到
```

### 超时
```
⏰ [SendAndWaitForResponse] 请求超时 (ID=req_service_1234567890_1:123, timeout=30s)
```

## 🔧 配置参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| 超时时间 | 30秒 | `SendAndWaitForResponse` 的等待时间 |
| 请求ID格式 | `req_service_{timestamp}_{nodeID}` | 唯一标识每个请求 |
| 缓存策略 | 异步保存 | 不阻塞主流程 |

## ✅ 优势

1. **同步体验**：虽然底层是异步 WebSocket，但对调用方来说是同步的
2. **自动缓存**：无需手动处理缓存逻辑
3. **超时保护**：避免无限等待
4. **错误处理**：统一的错误返回格式
5. **日志完整**：每个步骤都有详细日志

## 🚀 测试步骤

### 1. 启动服务
```bash
go run main.go
```

### 2. 打开 Figma 插件并连接

### 3. 清空缓存（模拟无缓存场景）
```bash
# 删除本地缓存文件
rm -rf cache/nodes/*

# 清空数据库缓存
DELETE FROM figma_file_caches;
```

### 4. 请求节点树
```bash
curl -X GET "http://localhost:8080/figma/project/1/node-tree" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### 5. 观察日志
应该看到完整的请求-响应流程：
```
⚠️ 本地缓存未找到
🔍 查询数据库缓存: fileKey=xxx, nodeID=1:123
⚠️ 数据库缓存未找到或状态异常
🔌 [GetFigmaNodesWithUser] 发现活跃 WebSocket 连接 (UserID=1, ConnectionID=abc)，发送插件请求并等待响应
✅ [SendAndWaitForResponse] 已发送请求 (ID=req_service_xxx)，等待响应...
✅ [SendAndWaitForResponse] 收到响应 (ID=req_service_xxx)
✅ [GetFigmaNodesWithUser] 收到插件响应，解析节点数据
✅ [GetFigmaNodesWithUser] 已保存到本地缓存
✅ [GetFigmaNodesWithUser] 已创建数据库缓存
✅ [GetFigmaNodesWithUser] 从 WebSocket 获取并解析了 15 个节点
```

## 🎉 完成！

现在所有 WebSocket 兜底的地方都会：
- ✅ 自动发送 `read_my_design` 请求
- ✅ 等待并监听插件响应
- ✅ 自动缓存返回的数据
- ✅ 提供完整的错误处理和超时保护

