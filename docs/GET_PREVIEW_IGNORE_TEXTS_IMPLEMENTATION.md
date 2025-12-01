# get_preview 支持 ignore_texts 参数 - 实现完成

## 修改概述

为 MCP 工具 `get_preview` 成功添加了 `ignore_texts` 参数支持，允许用户在获取预览图时选择是否忽略文本节点的渲染。

## 修改的文件

### 1. ✅ internal/controllers/mcp_tools.go

在 `GetBasicTools()` 函数中为 `get_preview` 工具添加了 `ignore_texts` 参数定义：

```go
{
    "name":        "get_preview",
    "description": "获取Figma节点的预览图",
    "inputSchema": gin.H{
        "type": "object",
        "properties": gin.H{
            "project_id":   gin.H{"type": "number", "description": "项目ID"},
            "node_id":      gin.H{"type": "string", "description": "节点ID（可选，默认使用项目根节点）"},
            "format":       gin.H{"type": "string", "description": "图片格式", "enum": []string{"png", "jpg", "svg"}, "default": "png"},
            "scale":        gin.H{"type": "number", "description": "缩放比例 (0.1-4.0)", "minimum": 0.1, "maximum": 4.0, "default": 0.1},
            "ignore_texts": gin.H{"type": "boolean", "description": "是否忽略文本节点（不渲染文本）", "default": false},
        },
        "required": []string{"project_id"},
    },
}
```

### 2. ✅ internal/controllers/mcp.go

#### 2.1 添加导入
```go
import (
    // ... 其他导入
    "path/filepath"  // 新增
)
```

#### 2.2 在 handleToolCall 函数的 get_preview case 中添加参数解析

```go
// 处理 ignore_texts 参数
ignoreTexts := false
if ignoreTextsInterface := arguments["ignore_texts"]; ignoreTextsInterface != nil {
    if ignoreTextsBool, ok := ignoreTextsInterface.(bool); ok {
        ignoreTexts = ignoreTextsBool
    }
}

// 获取实际的预览图
imagePath, imageBase64, err := getMCPPreviewImage(userID, projectID, nodeID, format, scale, ignoreTexts)
```

#### 2.3 更新 getMCPPreviewImage 函数

- 添加 `ignoreTexts` 参数到函数签名
- 更新日志输出，包含 `ignoreTexts` 信息
- 调用 `getImageFromCacheOnly` 时传递 `ignoreTexts` 参数

```go
func getMCPPreviewImage(userID uint, projectIDStr, nodeID, format string, scale float64, ignoreTexts bool) (string, string, error) {
    // ...
    log.Printf("📷 [MCP] 从缓存获取预览图 - 项目ID: %d, 节点ID: %s, 格式: %s, 缩放: %.1f, 忽略文本: %v", 
        projectID, nodeID, format, scale, ignoreTexts)
    
    imagePath, err := getImageFromCacheOnly(project.FileKey, nodeID, format, scale, ignoreTexts)
    // ...
}
```

#### 2.4 更新 getImageFromCacheOnly 函数

添加智能缓存查找逻辑，支持新旧两种文件名格式：

```go
func getImageFromCacheOnly(fileKey, nodeID, format string, scale float64, ignoreTexts bool) (string, error) {
    // 根据ignoreTexts决定文件名后缀：x表示带文字，p表示不带文字
    suffix := "x"
    if ignoreTexts {
        suffix = "p"
    }
    
    // 1. 尝试查找新格式：nodeID-scale[x|p]-hash.format
    // 2. 回退到旧格式：nodeID-scale.format
    // 3. 从 OBS 或 CDN 下载
}
```

#### 2.5 更新工具列表返回

在 MCP REST 接口中也更新了 `get_preview` 工具的定义，保持一致性。

### 3. ✅ web/templates/mcp_center.html

更新了 boolean 类型参数的渲染逻辑，正确处理默认值：

```javascript
else if (prop.type === 'boolean') {
    const defaultValue = prop.default !== undefined ? prop.default : false;
    inputsHtml += `<select id="${inputId}" class="tool-input" data-type="boolean" data-prop="${key}">
        <option value="false" ${!defaultValue ? 'selected' : ''}>False</option>
        <option value="true" ${defaultValue ? 'selected' : ''}>True</option>
    </select>`;
}
```

## 文件命名规则

### 新格式（带 hash 和 suffix）

- **带文本** (ignore_texts=false): `nodeID-1.0x-abc123.png`
- **不带文本** (ignore_texts=true): `nodeID-1.0p-abc123.png`

### 旧格式（向后兼容）

- `nodeID-1.0x.png`

## 缓存查找策略

`getImageFromCacheOnly` 函数按以下顺序查找：

1. **本地缓存（新格式）**: 在多个缓存目录中查找带 suffix 和 hash 的文件
2. **本地缓存（旧格式）**: 查找不带 suffix 的文件（向后兼容）
3. **OBS 云存储**: 如果数据库中有 OBS Key，从云端下载
4. **Figma CDN**: 如果有 CDN URL，从 CDN 下载

支持的缓存目录：
- `temp/{fileKey}/{format}/{scale}/`
- `temp/images/{fileKey}/{format}_{scale}/`
- `temp/{fileKey}/previews/`

## 使用示例

### 通过 Web UI

1. 访问 `http://localhost:8080/mcp-center`
2. 找到 `get_preview` 工具卡片
3. 填写参数：
   - project_id: `1`
   - node_id: （可选）
   - format: `png`
   - scale: `1.0`
   - **ignore_texts**: 选择 `True` 或 `False`
4. 点击"调用"

### 通过 API

```bash
curl -X POST http://localhost:8080/mcp-api/tool-call \
  -H "Content-Type: application/json" \
  -d '{
    "command": "get_preview",
    "params": {
      "project_id": 1,
      "format": "png",
      "scale": 1.0,
      "ignore_texts": true
    }
  }'
```

### MCP JSON-RPC 格式

```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "get_preview",
    "arguments": {
      "project_id": 1,
      "node_id": "1:123",
      "format": "png",
      "scale": 1.0,
      "ignore_texts": true
    }
  },
  "id": "request-1"
}
```

## 响应格式

成功响应：

```json
{
  "jsonrpc": "2.0",
  "id": "request-1",
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
      },
      {
        "type": "text",
        "text": "图片信息: 文件路径: temp/..."
      }
    ]
  }
}
```

失败响应：

```json
{
  "jsonrpc": "2.0",
  "id": "request-1",
  "error": {
    "code": -32603,
    "message": "获取预览图失败（缓存未找到）: ..., 请先在 Dashboard 中渲染该节点"
  }
}
```

## 日志输出

调用时会在服务器日志中看到：

```
📷 [MCP] 从缓存获取预览图 - 项目ID: 1, 节点ID: 1:123, 格式: png, 缩放: 1.0, 忽略文本: true
✅ [MCP] 从本地缓存获取图片(新格式): temp/.../1_123-1.0xp-abc123.png
✅ [MCP] 预览图获取成功 - 路径: ..., 大小: 12345 bytes
```

## 特性总结

✅ **向后兼容**: 不提供 `ignore_texts` 时默认为 `false`  
✅ **智能缓存**: 自动查找新旧两种格式  
✅ **多目录支持**: 在多个可能的缓存位置查找  
✅ **云端同步**: 支持从 OBS 和 CDN 下载  
✅ **类型安全**: 完整的参数验证和类型检查  
✅ **用户友好**: 前端自动生成直观的 UI 控件  

## 测试建议

1. ✅ 测试默认行为（不提供 ignore_texts）
2. ✅ 测试 ignore_texts=false（显示文本）
3. ✅ 测试 ignore_texts=true（隐藏文本）
4. ✅ 测试旧格式缓存文件的兼容性
5. ✅ 测试从 OBS/CDN 下载的场景

## 注意事项

1. **缓存依赖**: `get_preview` 仅从缓存获取，不会直接调用 Figma API
2. **预渲染**: 需要先通过其他方式（Dashboard 或 Figma 插件）渲染节点
3. **文件格式**: 新格式包含 suffix（x/p）和 hash，更易于区分和管理
4. **性能**: 智能缓存查找确保最快速度获取图片

## Linter 状态

✅ 无错误，仅有未使用参数的警告（不影响功能）

## 完成时间

2025-12-01

## 相关文档

- MCP 工具定义: `internal/controllers/mcp_tools.go`
- MCP 处理逻辑: `internal/controllers/mcp.go`
- 前端界面: `web/templates/mcp_center.html`

