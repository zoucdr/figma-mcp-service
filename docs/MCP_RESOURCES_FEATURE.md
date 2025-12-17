# MCP Resources 功能说明

## 概述

本文档描述了 Figma Deliver Service 中新增的 MCP Resources 功能。该功能允许客户端（如 Cursor IDE）通过 MCP 协议获取用户项目的 UI 预览图资源。

## 功能特性

### 1. 资源类型

新增了一种名为 `ui_preview` 的资源类型，用于提供项目的 UI 预览图。

### 2. 自动生成预览图

当客户端请求资源列表时，服务会：
- 获取用户的所有 Figma 项目
- 为每个项目的根节点生成预览图
- 将预览图编码为 base64 格式的 data URL
- 返回包含预览图 URL 的资源列表

### 3. 资源格式

每个 UI 预览资源包含以下字段：

```json
{
  "uri": "figma://project/{project_id}/ui_preview",
  "name": "[UI预览] {项目名称}",
  "description": "项目 {项目名称} 的UI预览图 (文件: {file_key}, 根节点: {root_node_id})",
  "mimeType": "image/png",
  "url": "data:image/png;base64,..."
}
```

## MCP 协议支持

### Initialize 响应

在 `initialize` 方法的响应中，服务现在会声明支持 resources：

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocolVersion": "2024-11-05",
    "capabilities": {
      "tools": {
        "listChanged": true
      },
      "resources": {
        "subscribe": false,
        "listChanged": true
      }
    },
    "serverInfo": {
      "name": "figma-deliver-mcp",
      "version": "1.0.0"
    }
  }
}
```

### Resources/List 方法

客户端可以通过 `resources/list` 方法获取资源列表：

**请求示例:**

```json
{
  "jsonrpc": "2.0",
  "method": "resources/list",
  "id": 2
}
```

**响应示例:**

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "resources": [
      {
        "uri": "figma://project/1/ui_preview",
        "name": "[UI预览] 我的设计项目",
        "description": "项目 我的设计项目 的UI预览图 (文件: abc123, 根节点: 0:1)",
        "mimeType": "image/png",
        "url": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgA..."
      },
      {
        "uri": "figma://project/2/ui_preview",
        "name": "[UI预览] 移动应用界面",
        "description": "项目 移动应用界面 的UI预览图 (文件: def456, 根节点: 0:2)",
        "mimeType": "image/png",
        "url": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgB..."
      }
    ]
  }
}
```

### Resources/Read 方法

客户端可以通过 `resources/read` 方法读取具体资源的内容：

**请求示例:**

```json
{
  "jsonrpc": "2.0",
  "method": "resources/read",
  "id": 3,
  "params": {
    "uri": "figma://project/1/ui_preview"
  }
}
```

**响应示例:**

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "contents": [
      {
        "uri": "figma://project/1/ui_preview",
        "mimeType": "image/png",
        "blob": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
      }
    ]
  }
}
```

**注意:** `blob` 字段包含 Base64 编码的图片数据，客户端可以直接解码使用。

## 使用场景

### 在 Cursor IDE 中使用

1. **配置 MCP 连接**
   
   在 Cursor 的 MCP 配置文件中添加 Figma Deliver Service：

   ```json
   {
     "mcpServers": {
       "figma-deliver": {
         "url": "http://localhost:8080/mcp/{your_mcp_token}"
       }
     }
   }
   ```

2. **访问 UI 预览资源**

   Cursor IDE 会自动调用 `resources/list` 方法获取可用资源，用户可以在 IDE 中直接查看项目的 UI 预览图。

3. **与 AI 助手集成**

   AI 助手（如 Claude）可以访问这些预览图资源，在对话中参考实际的 UI 设计，提供更准确的建议。

## 实现细节

### 预览图参数

- **格式**: PNG
- **缩放比例**: 0.1 (10%)
- **编码**: Base64 Data URL

### 性能考虑

1. **缓存机制**
   
   预览图会被下载到本地临时目录，避免重复从 Figma API 获取。

2. **异步处理**

   每个项目的预览图生成是独立的，失败的项目不会影响其他项目的资源返回。

3. **错误处理**

   如果某个项目的预览图生成失败，资源列表仍会包含该项目的条目，但会在描述中标注错误信息。

## 权限验证

- 只返回当前用户拥有的项目资源
- 需要用户已配置有效的 Figma Token
- 需要用户已创建至少一个项目

## 错误场景

### 1. 用户未配置 Figma Token

返回空资源列表：

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "resources": []
  }
}
```

### 2. 用户没有项目

返回空资源列表：

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "resources": []
  }
}
```

### 3. 预览图生成失败

资源仍会返回，但不包含 URL 字段，描述中会包含错误信息：

```json
{
  "uri": "figma://project/1/ui_preview",
  "name": "[UI预览] 我的项目",
  "description": "项目 我的项目 的UI预览图 (文件: abc123, 根节点: 0:1) (预览图生成失败: 网络错误)",
  "mimeType": "image/png"
}
```

## 测试

使用提供的测试脚本验证功能：

```powershell
.\demo\test_resources.ps1
```

测试内容包括：
1. 验证 Initialize 响应中的 resources 能力
2. 调用 resources/list 获取资源列表
3. 检查返回的资源格式和内容

## 未来改进

可能的功能扩展：

1. **可配置的预览图参数**
   - 允许客户端指定缩放比例、格式等
   
2. **增量更新支持**
   - 实现 `resources/subscribe` 功能，当项目更新时推送通知
   
3. **更多资源类型**
   - 节点配置资源
   - 导出历史记录资源
   - 设计规范资源

4. **性能优化**
   - 实现智能缓存策略
   - 支持懒加载（只在需要时生成预览图）
   - 并行处理多个项目的预览图生成

## 相关文件

- `internal/controllers/mcp.go` - MCP 协议处理和 resources 实现
- `internal/services/figma.go` - Figma API 交互和预览图下载
- `demo/test_resources.ps1` - 功能测试脚本

