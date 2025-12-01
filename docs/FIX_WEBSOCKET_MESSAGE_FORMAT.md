# 修复 WebSocket 消息格式问题

## 🐛 问题描述

`GetFigmaNodesWithUser` 发送 WebSocket 请求后无法收到响应，而 `mcp_tools.go` 中的代码可以正常收到响应。

## 🔍 根本原因

### 问题 1: 错误的消息格式

**之前的代码（错误）：**
```go
toolCallMsg := WSMessage{
    Type: "tool_call",
    ID:   requestID,
    Message: map[string]interface{}{
        "name": "read_my_design",
        "params": map[string]interface{}{
            "nodeId": nodeID,
        },
    },
}
```

**正确的格式（参考 mcp_tools.go）：**
```go
message := map[string]interface{}{
    "id":      requestID,
    "command": "read_my_design",
    "params": map[string]interface{}{
        "nodeId": nodeID,
    },
}
```

### 问题 2: 错误的发送方法

**之前的代码：**
```go
mcpService.SendToConnection(connection.ConnectionID, toolCallMsg)
```

**正确的方法：**
```go
mcpService.BroadcastToChannel(connection.ConnectionID, channel, message)
```

### 问题 3: 未使用频道系统

Figma 插件在 **频道（channel）** 中监听消息，而不是直接监听连接。需要：
1. 获取用户的频道
2. 如果没有频道，加入默认频道 `figma-bridge`
3. 通过频道广播消息

## ✅ 解决方案

### 修改后的代码模式

```go
// 1. 构造请求ID和消息
requestID := fmt.Sprintf("req_service_%d_%s", time.Now().UnixNano(), strings.ReplaceAll(nodeID, ":", "_"))

// 2. 创建响应通道
responseChan := make(chan interface{}, 1)
errorChan := make(chan error, 1)

// 3. 注册待处理请求
mcpService.RegisterPendingRequest(requestID, responseChan, errorChan)
defer mcpService.UnregisterPendingRequest(requestID)

// 4. 构建消息（与 mcp_tools.go 相同的格式）
message := map[string]interface{}{
    "id":      requestID,
    "command": "read_my_design",
    "params": map[string]interface{}{
        "nodeId": nodeID,
    },
}

// 5. 获取或加入频道
channel := mcpService.GetUserChannel(connection.ConnectionID)
if channel == "" {
    channel = "figma-bridge"
    _ = mcpService.JoinChannel(connection.ConnectionID, channel)
}

// 6. 广播消息到频道
err := mcpService.BroadcastToChannel(connection.ConnectionID, channel, message)
if err != nil {
    return nil, fmt.Errorf("发送消息失败: %v", err)
}

// 7. 等待响应
select {
case response := <-responseChan:
    // 处理响应
case respErr := <-errorChan:
    return nil, respErr
case <-time.After(30 * time.Second):
    return nil, fmt.Errorf("请求超时")
}
```

## 📝 修改的文件

### 1. `internal/services/figma.go`

**`GetFigmaNodesWithUser` 函数**
- ✅ 改用 `BroadcastToChannel` 而不是 `SendToConnection`
- ✅ 使用正确的消息格式（`id`, `command`, `params`）
- ✅ 添加频道管理逻辑
- ✅ 手动管理响应通道（不使用 `SendAndWaitForResponse`）

### 2. `internal/controllers/cache.go`

**`RefreshProjectNodeTree` 函数**
- ✅ 改用 `BroadcastToChannel`
- ✅ 使用正确的消息格式
- ✅ 添加频道管理逻辑

**`BatchRefreshNodeTree` 函数**
- ✅ 改用 `BroadcastToChannel`
- ✅ 使用正确的消息格式
- ✅ 添加频道管理逻辑

### 3. `internal/controllers/figma.go`

**`GetFigmaNodeTree` 函数**
- ✅ 改用 `BroadcastToChannel`
- ✅ 使用正确的消息格式
- ✅ 添加频道管理逻辑

## 🔄 消息流程

```
┌─────────────────────────────────────────────────────────────┐
│  1. 服务层/控制器创建请求                                      │
│     - 生成唯一 requestID                                      │
│     - 构造消息 {id, command, params}                          │
└────────────────┬────────────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────────────┐
│  2. 注册待处理请求                                            │
│     mcpService.RegisterPendingRequest(requestID, ...)        │
└────────────────┬────────────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────────────┐
│  3. 获取或加入频道                                            │
│     - 尝试获取用户频道                                        │
│     - 如果没有，加入 "figma-bridge"                          │
└────────────────┬────────────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────────────┐
│  4. 广播消息到频道                                            │
│     mcpService.BroadcastToChannel(connID, channel, message)  │
└────────────────┬────────────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────────────┐
│  5. Figma 插件在频道中接收消息                                │
│     - 解析 command: "read_my_design"                         │
│     - 执行操作（读取节点数据）                                │
│     - 发送响应到 WebSocket                                   │
└────────────────┬────────────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────────────┐
│  6. MCP 服务接收响应                                          │
│     HandleFigmaPluginResponse(message)                       │
│     └─ RespondToPendingRequest(requestID, result, nil)       │
│        └─ 发送到 responseChan                                │
└────────────────┬────────────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────────────┐
│  7. 原始请求者收到响应                                        │
│     case response := <-responseChan:                         │
│        // 处理响应数据                                        │
└─────────────────────────────────────────────────────────────┘
```

## 🎯 关键点总结

1. **消息格式必须匹配**
   - 使用 `{id, command, params}` 而不是 `{Type, ID, Message}`
   - `command` 字段是工具名称（如 `read_my_design`）
   - `params` 中使用 `nodeId`（驼峰命名）

2. **必须使用频道系统**
   - Figma 插件在频道中监听
   - 默认频道是 `figma-bridge`
   - 使用 `BroadcastToChannel` 而不是 `SendToConnection`

3. **响应管理**
   - 使用 `RegisterPendingRequest` 注册请求
   - 使用 `UnregisterPendingRequest` 清理请求
   - `HandleFigmaPluginResponse` 会自动响应待处理请求

4. **超时保护**
   - 所有等待响应的地方都有 30 秒超时
   - 超时后返回友好的错误信息

## 📊 测试验证

### 1. 测试 GetFigmaNodesWithUser

```bash
# 清空缓存
rm -rf cache/nodes/*

# 请求节点树（会触发 WebSocket 请求）
curl -X GET "http://localhost:8080/figma/project/1/node-tree" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**预期日志：**
```
🔌 [GetFigmaNodesWithUser] 发现活跃 WebSocket 连接 (UserID=1, ConnectionID=xxx)
✅ [GetFigmaNodesWithUser] 已发送请求到频道 figma-bridge，等待响应...
✅ [GetFigmaNodesWithUser] 收到响应 (ID=req_service_xxx)
✅ [GetFigmaNodesWithUser] 收到插件响应，解析节点数据
✅ [GetFigmaNodesWithUser] 从 WebSocket 获取并解析了 15 个节点
```

### 2. 测试 RefreshProjectNodeTree

```bash
curl -X POST "http://localhost:8080/cache/refresh/project/1" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**预期日志：**
```
🔌 [RefreshProjectNodeTree] 发现活跃 WebSocket 连接，请求插件获取数据
✅ [RefreshProjectNodeTree] 已发送请求到频道 figma-bridge
```

### 3. 测试 BatchRefreshNodeTree

```bash
curl -X POST "http://localhost:8080/cache/batch-refresh" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "file_key": "xxx",
    "node_ids": ["1:123", "1:456"]
  }'
```

**预期日志：**
```
🔌 [BatchRefreshNodeTree] 发现活跃 WebSocket 连接，请求插件获取数据
✅ [BatchRefreshNodeTree] 已发送 2 个请求到频道 figma-bridge
```

## ✅ 修复完成

现在所有 WebSocket 兜底的地方都使用正确的：
- ✅ 消息格式
- ✅ 发送方法（BroadcastToChannel）
- ✅ 频道管理
- ✅ 响应处理

插件可以正常接收请求并返回响应！🎉

