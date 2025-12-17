# MCP 基础工具直接调用实现

## 问题描述

之前所有的 MCP 工具调用都会通过 WebSocket 转发到 Figma 插件，包括基础工具（如 `get_preview`、`get_node_tree` 等）。这导致：

1. ❌ 基础工具需要 Figma 插件连接才能使用
2. ❌ 即使只是查询缓存数据，也必须打开 Figma 插件
3. ❌ 增加了不必要的 WebSocket 通信开销
4. ❌ 用户体验不佳（纯后端操作却需要前端连接）

## 解决方案

将 MCP 工具分为两类：
- **基础工具**: 直接在后端处理，不需要 Figma 插件连接
- **插件工具**: 通过 WebSocket 转发到 Figma 插件处理

## 实现细节

### 1. 后端改进

#### 1.1 修改 `SimulateMCPToolCall` 函数 (mcp_tools.go)

**位置**: `internal/controllers/mcp_tools.go`

**修改前**:
```go
func SimulateMCPToolCall(c *gin.Context) {
    // 总是转发到 Figma 插件 WebSocket
    conn, exists := mcpService.GetUserConnection(userID.(uint))
    if !exists {
        c.JSON(http.StatusNotFound, gin.H{"error": "未找到活跃的 Figma 连接"})
        return
    }
    response := HandleFigmaPluginToolCall(userID.(uint), conn.ConnectionID, req.Command, req.Params, requestID)
    c.JSON(http.StatusOK, response)
}
```

**修改后**:
```go
func SimulateMCPToolCall(c *gin.Context) {
    // 定义基础工具列表
    basicTools := map[string]bool{
        "get_preview":         true,
        "get_node_tree":       true,
        "set_nodes_modify":    true,
        "get_modify_prompt":   true,
        "clear_nodes_modifys": true,
        "get_ref_nodes":       true,
    }
    
    // 检查是否为基础工具
    if basicTools[req.Command] {
        // 基础工具：直接调用后端处理
        response := HandleBasicToolCall(userID.(uint), req.Command, req.Params, requestID)
        c.JSON(http.StatusOK, response)
        return
    }
    
    // 插件工具：需要 WebSocket 连接
    conn, exists := mcpService.GetUserConnection(userID.(uint))
    if !exists {
        c.JSON(http.StatusNotFound, gin.H{"error": "未找到活跃的 Figma 连接，请先打开 Figma 插件"})
        return
    }
    
    response := HandleFigmaPluginToolCall(userID.(uint), conn.ConnectionID, req.Command, req.Params, requestID)
    c.JSON(http.StatusOK, response)
}
```

#### 1.2 添加 `HandleBasicToolCall` 函数 (mcp.go)

**位置**: `internal/controllers/mcp.go`

```go
// HandleBasicToolCall 处理基础工具调用（公开函数，供 HTTP API 使用）
func HandleBasicToolCall(userID uint, toolName string, arguments map[string]interface{}, requestID interface{}) gin.H {
    // 基础工具不需要 connectionID，传空字符串
    return handleToolCall(userID, "", toolName, arguments, requestID)
}
```

### 2. 前端简化

#### 2.1 移除不必要的连接检查

**位置**: `web/templates/mcp_center.html`

**修改前**:
```javascript
async function sendMcpCall(command, params) {
    // 检查是否为基础工具
    const isBasicTool = basicTools.includes(command);
    
    if (!isFigmaConnected && !isBasicTool) {
        alert('此工具需要 Figma 插件连接');
        return;
    }
    // ...
}
```

**修改后**:
```javascript
async function sendMcpCall(command, params) {
    // 直接发送请求，后端会自动判断工具类型
    addMessage('📤 [请求] ' + command + '\n' + JSON.stringify(params, null, 2), 'request');
    
    const response = await fetch('/mcp-api/tool-call', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ command, params })
    });
    // ...
}
```

## 工具分类

### 基础工具（后端直接处理）

这些工具不需要 Figma 插件连接，直接在后端处理：

| 工具名称 | 描述 | 数据源 |
|---------|------|--------|
| `get_preview` | 获取节点预览图 | 本地缓存/OBS/CDN |
| `get_node_tree` | 获取节点树配置 | 数据库 |
| `set_nodes_modify` | 批量设置节点配置 | 数据库 |
| `get_modify_prompt` | 获取配置提示词 | 数据库 |
| `clear_nodes_modifys` | 清理节点修改 | 数据库 |
| `get_ref_nodes` | 获取依赖节点列表 | 数据库 |

### 插件工具（WebSocket 转发）

这些工具需要 Figma 插件连接，通过 WebSocket 转发：

| 工具名称 | 描述 | 需要连接 |
|---------|------|---------|
| `read_my_design` | 读取 Figma 设计 | ✅ Figma 插件 |
| `export_node_as_image` | 导出单个节点图片 | ✅ Figma 插件 |
| `export_nodes_as_images` | 批量导出节点图片 | ✅ Figma 插件 |

## 请求流程

### 基础工具请求流程

```
前端 (mcp_center.html)
    ↓
    POST /mcp-api/tool-call
    { command: "get_preview", params: {...} }
    ↓
后端 (mcp_tools.go: SimulateMCPToolCall)
    ↓
    检查: basicTools[command] == true
    ↓
    是基础工具
    ↓
后端 (mcp.go: HandleBasicToolCall)
    ↓
    调用 handleToolCall(userID, "", toolName, arguments, requestID)
    ↓
    执行具体逻辑 (switch case)
    - get_preview → getMCPPreviewImage
    - get_node_tree → 查询数据库
    - 其他...
    ↓
    返回 JSON 响应
    ↓
前端显示结果
```

### 插件工具请求流程

```
前端 (mcp_center.html)
    ↓
    POST /mcp-api/tool-call
    { command: "export_node_as_image", params: {...} }
    ↓
后端 (mcp_tools.go: SimulateMCPToolCall)
    ↓
    检查: basicTools[command] == false
    ↓
    是插件工具
    ↓
    检查 WebSocket 连接
    ↓
    是否连接？
    ├─ 否 → 返回错误 "未找到活跃的 Figma 连接"
    └─ 是 → 继续
    ↓
后端 (mcp_tools.go: HandleFigmaPluginToolCall)
    ↓
    通过 WebSocket 发送到 Figma 插件
    ↓
Figma 插件处理
    ↓
    通过 WebSocket 返回结果
    ↓
后端接收并转发响应
    ↓
前端显示结果
```

## 优势

### 1. ✅ 更好的用户体验

- 基础工具无需 Figma 插件连接
- 查询缓存数据速度更快
- 减少用户操作步骤

### 2. ✅ 更清晰的架构

- 工具分类明确
- 职责分离清晰
- 代码更易维护

### 3. ✅ 更高的性能

- 减少 WebSocket 通信
- 直接访问后端数据
- 降低延迟

### 4. ✅ 更好的错误处理

- 基础工具：后端直接返回错误
- 插件工具：明确提示需要连接

## 测试场景

### 测试 1: 基础工具（无需连接）

**步骤**:
1. 访问 MCP 测试中心
2. **不要**打开 Figma 插件
3. 确认状态显示"Figma 未连接"
4. 调用 `get_preview` 工具
5. 填写参数并点击"调用"

**预期结果**:
- ✅ 成功调用（无需连接）
- ✅ 如果缓存存在，返回图片
- ✅ 如果缓存不存在，返回错误提示

**实际行为**:
```json
{
  "jsonrpc": "2.0",
  "id": "sim_1234567890",
  "result": {
    "content": [
      {
        "type": "text",
        "text": "预览图获取成功 - 项目ID: 1, 节点ID: 1:123, 格式: png, 缩放: 1.0"
      },
      {
        "type": "image",
        "data": "iVBORw0KGgoAAAANSUhEUgAA...",
        "mimeType": "image/png"
      }
    ]
  }
}
```

### 测试 2: 插件工具（需要连接）

**步骤**:
1. 访问 MCP 测试中心
2. **不要**打开 Figma 插件
3. 确认状态显示"Figma 未连接"
4. 调用 `export_node_as_image` 工具
5. 填写参数并点击"调用"

**预期结果**:
- ❌ 调用失败
- ❌ 返回错误："未找到活跃的 Figma 连接，请先打开 Figma 插件"

**实际行为**:
```json
{
  "error": "未找到活跃的 Figma 连接，请先打开 Figma 插件"
}
```

### 测试 3: ignore_texts 参数（基础工具）

**步骤**:
1. 访问 MCP 测试中心（无需连接）
2. 调用 `get_preview`，设置 `ignore_texts=true`
3. 查看请求和响应

**预期结果**:
```javascript
// 请求
{
  "command": "get_preview",
  "params": {
    "project_id": 1,
    "ignore_texts": true  // ✅ boolean 类型
  }
}

// 后端日志
📷 [MCP] 从缓存获取预览图 - 项目ID: 1, 节点ID: 1:123, 格式: png, 缩放: 1.0, 忽略文本: true

// 响应
{
  "jsonrpc": "2.0",
  "result": {
    "content": [...]
  }
}
```

## API 端点

### POST /mcp-api/tool-call

**请求格式**:
```json
{
  "command": "get_preview",
  "params": {
    "project_id": 1,
    "node_id": "1:123",
    "format": "png",
    "scale": 1.0,
    "ignore_texts": true
  }
}
```

**响应格式（成功）**:
```json
{
  "jsonrpc": "2.0",
  "id": "sim_1733123456789",
  "result": {
    "content": [
      {
        "type": "text",
        "text": "预览图获取成功 - ..."
      },
      {
        "type": "image",
        "data": "base64_encoded_image...",
        "mimeType": "image/png"
      }
    ]
  }
}
```

**响应格式（错误）**:
```json
{
  "jsonrpc": "2.0",
  "id": "sim_1733123456789",
  "error": {
    "code": -32603,
    "message": "获取预览图失败（缓存未找到）: ..."
  }
}
```

## 代码结构

```
internal/controllers/
├── mcp.go
│   ├── HandleBasicToolCall()      ← 新增：公开的基础工具处理函数
│   └── handleToolCall()            ← 现有：实际的工具处理逻辑
│
└── mcp_tools.go
    └── SimulateMCPToolCall()       ← 修改：增加工具类型判断
        ├── 基础工具 → HandleBasicToolCall()
        └── 插件工具 → HandleFigmaPluginToolCall()
```

## 相关文件

- ✅ `internal/controllers/mcp_tools.go` - HTTP API 入口，工具分类
- ✅ `internal/controllers/mcp.go` - 工具处理逻辑
- ✅ `web/templates/mcp_center.html` - 前端调用界面

## 完成日期

2025-12-01

## 注意事项

1. **向后兼容**: 插件工具的行为保持不变
2. **清晰错误**: 插件工具在未连接时会明确提示
3. **性能提升**: 基础工具响应速度更快
4. **易于扩展**: 添加新的基础工具只需更新 `basicTools` map

