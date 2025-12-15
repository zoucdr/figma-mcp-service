# get_optimaze_document 工具说明

## 概述

`get_optimaze_document` 是一个新增的 MCP 工具，用于获取优化后的项目文档数据，包含完整的节点树结构和图片名称信息。该工具返回的 JSON 格式与导出功能生成的 `metadata.json` 类似。

## 工具信息

- **名称**: `get_optimaze_document`
- **描述**: 获取优化后的JSON数据，包含完整的节点树结构和图片名称信息（类似导出功能生成的metadata.json）
- **类型**: 基础工具（不需要 Figma 插件连接）

## 参数

### 必需参数

- `project_id` (number): 项目ID

### 可选参数

- `format` (string): 图片格式
  - 可选值: `"png"`, `"jpg"`, `"svg"`
  - 默认值: `"png"`

- `scale` (number): 图片缩放比例
  - 范围: 0.1 - 4.0
  - 默认值: 1.0

## 返回格式

工具返回一个 JSON 对象，包含以下字段：

```json
{
  "project": {
    "id": 1,
    "name": "项目名称",
    "file_key": "figma-file-key",
    "root_node_id": "root-node-id"
  },
  "node": {
    "id": "node-id",
    "name": "节点名称",
    "type": "节点类型",
    "img_path": "image-1x.png",      // 图片文件名（如果节点有图片）
    "img_name": "custom-name",        // 自定义图片名称
    "img_ext": "png",                 // 图片格式
    "res_mode": "sprite",             // 资源模式
    "horizontal": "LEFT",             // 水平约束
    "vertical": "TOP",                // 垂直约束
    "components": [...],              // 组件列表
    "children": [...]                 // 子节点（递归结构）
  },
  "export_config": {
    "format": "png",
    "scale": 1.0
  },
  "generated_at": "2024-12-02T10:30:00Z"
}
```

## 功能特点

### 1. 节点优化

- 应用所有用户配置的节点修改信息（`modifys`）
- 过滤不可见节点和被忽略的节点
- 根据 `res_mode` 处理节点结构
- 支持父子 `res_mode` 共存机制

### 2. 图片名称生成

对于设置了 `res_mode` 为 `sprite`、`texture` 或 `slice` 的节点，自动生成图片文件名：

- 使用 `img_name`（如果配置）或 `img_id`（如果配置）或节点 ID 作为基础名称
- 格式: `{basename}-{scale}x.{format}`
- 示例: `button-2x.png`、`icon-1x.jpg`

### 3. 树形结构

- 保持完整的父子节点关系
- 递归包含所有子节点
- 自动移除 `parent_id` 字段（仅用于内部处理）

### 4. 与导出功能的区别

| 特性 | get_optimaze_document | 导出功能 |
|------|---------------------|---------|
| 下载图片 | ❌ 否 | ✅ 是 |
| 返回格式 | JSON 文本 | ZIP 压缩包 |
| 图片路径 | 生成文件名 | 实际文件路径 |
| 执行速度 | ⚡ 快 | 🐌 慢（需要下载） |
| 用途 | 预览结构、代码生成 | 完整导出 |

## 使用场景

1. **预览文档结构**: 在不下载图片的情况下查看完整的节点树结构
2. **代码生成**: AI 可以根据返回的 JSON 数据生成 UI 代码
3. **配置验证**: 验证节点配置是否正确
4. **快速迭代**: 在修改配置后快速查看结果，无需等待图片下载

## 使用示例

### 基本调用

```json
{
  "command": "get_optimaze_document",
  "params": {
    "project_id": 1
  }
}
```

### 指定格式和缩放

```json
{
  "command": "get_optimaze_document",
  "params": {
    "project_id": 1,
    "format": "jpg",
    "scale": 2.0
  }
}
```

## API 端点

### HTTP API

```
POST /api/mcp/tools/simulate
Authorization: Bearer {token}
Content-Type: application/json

{
  "command": "get_optimaze_document",
  "params": {
    "project_id": 1
  }
}
```

### MCP 协议

```json
{
  "jsonrpc": "2.0",
  "id": "request-id",
  "method": "tools/call",
  "params": {
    "name": "get_optimaze_document",
    "arguments": {
      "project_id": 1
    }
  }
}
```

## 错误处理

可能的错误情况：

1. **项目不存在**: `项目信息获取失败`
2. **无权访问**: `无权访问该项目`
3. **没有节点数据**: `项目没有节点数据`
4. **获取节点树失败**: `获取节点树数据失败`

## 实现文件

- 工具定义: `internal/controllers/mcp_tools.go`
- 工具处理: `internal/controllers/mcp.go`
- 核心逻辑: `internal/services/export.go` - `GetOptimizedDocument` 函数

## 测试

可以使用提供的测试脚本进行测试：

```powershell
# 设置测试 Token
$env:FIGMA_TEST_TOKEN = "your-token-here"

# 运行测试
.\demo\test_get_optimaze_document.ps1
```

## 注意事项

1. 该工具不会实际下载图片，仅生成图片文件名
2. 需要先配置项目的节点信息和修改信息
3. 返回的数据结构与项目的实际配置相关
4. 图片文件名仅供参考，实际文件可能不存在

