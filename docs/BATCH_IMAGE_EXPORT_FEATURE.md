# 批量图片导出功能

## 功能概述

新增了批量导出多个节点为图片的功能，支持一次性导出多个Figma节点，并将所有图片的base64数据以字典格式返回。

## 实现文件

### 1. Figma插件 (`figma-plugin/code.js`)

#### 新增命令处理
- 在 `handleCommand` 函数中添加了 `export_nodes_as_images` 命令支持

#### 新增函数 `exportNodesAsImages`
实现批量导出图片的核心功能。

**参数说明：**
- `nodeIds` (必需): 节点ID列表
  - 字符串格式：使用逗号分隔，如 `"1-123,1-456,1-789"`
  - 数组格式：`["1-123", "1-456", "1-789"]`
- `scale` (可选): 缩放比例，默认为 1
- `ignore_text` (可选): 是否忽略文本节点（不渲染文本），默认为 false

**功能特点：**
- 自动解析逗号分隔的字符串或数组格式
- 批量处理所有节点
- 自动转换节点ID格式（`-` 转为 `:`）
- 支持忽略文本节点选项
- 错误处理：单个节点失败不影响其他节点的导出
- 自动恢复临时隐藏的文本节点可见性

**返回格式：**
```javascript
{
  "images": {
    "1-123": "iVBORw0KGgoAAAANSUhEUgAA...",  // base64图片数据
    "1-456": "iVBORw0KGgoAAAANSUhEUgAA...",
    "1-789": "iVBORw0KGgoAAAANSUhEUgAA..."
  },
  "errors": {  // 仅当有错误时返回
    "1-999": "Node not found with ID: 1-999"
  },
  "total": 3,      // 总共处理的节点数
  "success": 2,    // 成功导出的数量
  "failed": 1      // 失败的数量
}
```

### 2. 服务端控制器 (`internal/controllers/mcp_tools.go`)

#### 新增MCP工具定义

在 `GetFigmaPluginTools()` 函数中添加了新的工具：

```go
{
    "name":        "export_nodes_as_images",
    "description": "批量导出多个节点为图片，返回所有图片的base64数据字典",
    "inputSchema": gin.H{
        "type": "object",
        "properties": gin.H{
            "nodeIds":     gin.H{"type": "string", "description": "节点ID列表，使用逗号分隔（例如：'1-123,1-456,1-789'）"},
            "scale":       gin.H{"type": "number", "description": "缩放比例", "default": 1},
            "ignore_text": gin.H{"type": "boolean", "description": "是否忽略文本节点（不渲染文本）", "default": false},
        },
        "required": []string{"nodeIds"},
    },
}
```

### 3. Web界面 (`web/templates/mcp_center.html`)

#### 增强图片显示逻辑

在响应处理部分添加了对批量图片的识别和展示：

**功能特点：**
- 自动识别批量图片导出结果（通过检查 `images` 字段）
- 为每张图片创建独立的展示区域，显示节点ID
- 显示统计信息（总数、成功数、失败数）
- 如果有错误，同时显示错误信息
- 图片展示样式美观，带边框和圆角
- 支持折叠/展开功能
- 支持一键复制功能

**显示效果：**
```
📨 [响应]
{
  "total": 3,
  "success": 3,
  "failed": 0,
  "images": "&3 张图片"
}

[图片展示区域]
节点 ID: 1-123
[图片]

节点 ID: 1-456
[图片]

节点 ID: 1-789
[图片]
```

## 使用示例

### 通过MCP调用

#### 示例1：基本用法（字符串格式）
```json
{
  "command": "export_nodes_as_images",
  "params": {
    "nodeIds": "1-123,1-456,1-789"
  }
}
```

#### 示例2：带缩放比例
```json
{
  "command": "export_nodes_as_images",
  "params": {
    "nodeIds": "1-123,1-456",
    "scale": 2
  }
}
```

#### 示例3：忽略文本节点
```json
{
  "command": "export_nodes_as_images",
  "params": {
    "nodeIds": "1-123,1-456,1-789",
    "scale": 1,
    "ignore_text": true
  }
}
```

#### 示例4：数组格式
```json
{
  "command": "export_nodes_as_images",
  "params": {
    "nodeIds": ["1-123", "1-456", "1-789"],
    "scale": 1.5
  }
}
```

### 通过Web界面

1. 访问 MCP 功能测试中心
2. 确保 Figma 插件已连接
3. 找到 `export_nodes_as_images` 工具卡片
4. 输入参数：
   - **nodeIds**: `1-123,1-456,1-789`
   - **scale**: `1` (可选)
   - **ignore_text**: `false` (可选)
5. 点击"调用"按钮
6. 在调用日志中查看结果和图片

## 技术细节

### 错误处理
- 如果 `nodeIds` 参数缺失，返回错误："Missing nodeIds parameter"
- 如果 `nodeIds` 为空，返回错误："nodeIds is empty"
- 如果节点不存在，该节点会被记录在 `errors` 中，不影响其他节点
- 如果节点不支持导出，该节点会被记录在 `errors` 中

### 性能考虑
- 按顺序处理节点（避免并发导致的Figma API限制）
- 使用自定义base64编码函数（优化性能）
- 仅在需要时隐藏/恢复文本节点

### 兼容性
- 支持Figma节点ID的两种格式：
  - 带冒号：`1:123`
  - 带横杠：`1-123`（会自动转换为冒号格式）

## 返回数据结构

### 成功响应
```json
{
  "images": {
    "node_id_1": "base64_image_data_1",
    "node_id_2": "base64_image_data_2",
    ...
  },
  "total": 3,
  "success": 3,
  "failed": 0
}
```

### 部分失败响应
```json
{
  "images": {
    "1-123": "base64_data...",
    "1-456": "base64_data..."
  },
  "errors": {
    "1-999": "Node not found with ID: 1-999"
  },
  "total": 3,
  "success": 2,
  "failed": 1
}
```

### 完全失败响应
```json
{
  "images": {},
  "errors": {
    "1-123": "Node not found with ID: 1-123",
    "1-456": "Node does not support exporting: 1-456"
  },
  "total": 2,
  "success": 0,
  "failed": 2
}
```

## 与单张图片导出的对比

| 功能 | export_node_as_image | export_nodes_as_images |
|------|---------------------|------------------------|
| 节点参数 | `nodeId` (单个) | `nodeIds` (多个，逗号分隔) |
| 返回格式 | 单个图片对象 | 图片字典 + 统计信息 |
| 错误处理 | 失败则整体失败 | 部分失败不影响其他 |
| 适用场景 | 导出单个节点 | 批量导出多个节点 |

## 使用场景

1. **批量导出设计资产**
   - 一次性导出多个图标
   - 导出多个按钮状态
   - 导出多个组件变体

2. **自动化工作流**
   - 设计系统资产批量导出
   - CI/CD管道中的资产更新
   - 自动化设计验证

3. **性能优化**
   - 减少往返请求次数
   - 提高批量操作效率

## 注意事项

1. 节点ID格式要求：
   - 使用逗号分隔，不要有多余空格
   - 建议格式：`"1-123,1-456,1-789"`
   
2. 性能建议：
   - 避免一次性导出过多节点（建议不超过20个）
   - 对于大型节点，建议降低 `scale` 值
   
3. 内存占用：
   - 批量导出会占用较多内存
   - base64数据会增加传输大小

## 未来改进方向

1. 支持并行导出（需要评估Figma API限制）
2. 支持进度回调
3. 支持更多导出格式（JPG、SVG、PDF）
4. 支持导出到云存储（OSS）
5. 添加导出队列管理

## 相关文件

- `figma-plugin/code.js` - Figma插件核心逻辑
- `internal/controllers/mcp_tools.go` - MCP工具定义和处理
- `web/templates/mcp_center.html` - Web测试界面

## 更新日期

2025-12-01

