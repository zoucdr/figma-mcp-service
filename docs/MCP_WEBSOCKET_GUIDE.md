# MCP WebSocket 集成使用说明

## 概述

Figma Deliver MCP 服务现已支持通过 WebSocket 与 Figma 插件实时通信，使 Cursor 等 MCP 客户端能够直接操作 Figma 设计文件。

## 架构

```
Cursor (MCP Client) 
    ↓ MCP Protocol (HTTP/JSONRPC)
Figma Deliver Server (Go)
    ↓ WebSocket
Figma Plugin (JavaScript)
    ↓ Plugin API
Figma 设计文件
```

## 功能特性

### 1. 动态工具列表

系统根据 WebSocket 连接状态动态加载工具：

**基础工具**（无需 WebSocket）：
- `get_preview` - 获取节点预览图
- `get_node_tree` - 获取节点配置
- `set_nodes_modify` - 批量设置节点配置
- `get_modify_prompt` - 获取配置提示词
- `clear_nodes_modifys` - 清理节点修改
- `get_ref_nodes` - 获取依赖节点

**Figma 插件工具**（需要 WebSocket 连接）：
- `join_channel` - 加入频道
- `get_document_info` - 获取文档信息
- `get_selection` - 获取选中节点
- `read_my_design` - 读取设计详情
- `get_node_info` - 获取节点信息
- `create_frame` - 创建 Frame
- `create_text` - 创建文本
- `set_text_content` - 修改文本内容
- `set_fill_color` - 设置填充颜色
- `move_node` - 移动节点
- `resize_node` - 调整节点大小
- `delete_node` - 删除节点
- `clone_node` - 克隆节点
- `scan_text_nodes` - 扫描文本节点
- `set_multiple_text_contents` - 批量修改文本
- `export_node_as_image` - 导出为图片

### 2. 频道管理

系统支持多频道通信，每个用户/项目可以有独立的频道：
- 自动频道：`user_{userID}`
- 项目频道：`project_{projectID}`
- 自定义频道：用户指定

### 3. 请求-响应机制

- 30秒超时保护
- 自动重试机制
- 进度更新支持（大批量操作）
- 错误详细反馈

## 配置步骤

### 1. 服务器端配置

MCP Token 是用户的身份认证凭证，用于：
- HTTP API 认证
- WebSocket 连接认证
- MCP 协议通信

获取 MCP Token：
1. 登录 Figma Deliver 服务
2. 进入"个人资料"页面
3. 点击"生成 MCP Token"按钮
4. 复制生成的 Token

### 2. Figma 插件配置

1. 在 Figma 中打开插件
2. 插件会使用 user 的 MCP Token 自动连接到服务器
3. 插件会自动加入对应的频道

### 3. Cursor 配置

在 Cursor 的 MCP 配置文件中添加：

**方式一：使用 HTTP 协议** (推荐)

```json
{
  "mcpServers": {
    "figma-deliver": {
      "url": "http://localhost:8080/mcp/{YOUR_MCP_TOKEN}",
      "transport": "stdio",
      "headers": {
        "Content-Type": "application/json"
      }
    }
  }
}
```

**方式二：使用自定义启动脚本**

创建 `figma-mcp-client.js`:

```javascript
#!/usr/bin/env node

const fetch = require('node-fetch');
const readline = require('readline');

const MCP_TOKEN = process.argv[2] || 'YOUR_MCP_TOKEN';
const SERVER_URL = `http://localhost:8080/mcp/${MCP_TOKEN}`;

const rl = readline.createInterface({
  input: process.stdin,
  output: process.stdout,
  terminal: false
});

// 处理来自 stdin 的 JSONRPC 请求
rl.on('line', async (line) => {
  try {
    const request = JSON.parse(line);
    
    const response = await fetch(SERVER_URL, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(request),
    });
    
    const result = await response.json();
    console.log(JSON.stringify(result));
  } catch (error) {
    console.error(JSON.stringify({
      jsonrpc: '2.0',
      id: null,
      error: {
        code: -32603,
        message: error.message
      }
    }));
  }
});
```

Cursor 配置：

```json
{
  "mcpServers": {
    "figma-deliver": {
      "command": "node",
      "args": ["path/to/figma-mcp-client.js", "YOUR_MCP_TOKEN"]
    }
  }
}
```

## 使用流程

### 1. 建立连接

```
1. 打开 Figma 文件
2. 启动 Figma Deliver 插件
3. 插件自动连接到服务器 WebSocket
4. 在 Cursor 中启用 MCP 服务
```

### 2. 使用工具

在 Cursor 中，你可以：

**查看当前设计**：
```
User: 读取当前选中的设计元素
AI: 使用 read_my_design 工具...
```

**创建元素**：
```
User: 在坐标(100, 100)创建一个红色的按钮
AI: 使用 create_frame 创建背景...
    使用 create_text 添加文字...
    使用 set_fill_color 设置颜色...
```

**批量操作**：
```
User: 扫描所有文本，将中文替换为英文
AI: 使用 scan_text_nodes 扫描...
    使用 set_multiple_text_contents 批量修改...
```

**导出图片**：
```
User: 导出当前选中节点为PNG
AI: 使用 export_node_as_image...
```

## API 端点

### MCP 协议端点

```
POST /mcp/{mcp_token}
Content-Type: application/json

{
  "jsonrpc": "2.0",
  "method": "tools/list",
  "id": 1
}
```

### WebSocket 端点

```
ws://localhost:8080/mcp/ws
Query: connection_id={mcp_token}
```

## 消息格式

### 发送到 Figma 插件

```json
{
  "type": "message",
  "channel": "user_123",
  "message": {
    "id": "uuid",
    "command": "create_frame",
    "params": {
      "x": 100,
      "y": 100,
      "width": 200,
      "height": 100
    }
  }
}
```

### 从 Figma 插件接收

```json
{
  "type": "broadcast",
  "channel": "user_123",
  "message": {
    "id": "uuid",
    "result": {
      "success": true,
      "nodeId": "123:456",
      "name": "Frame 1"
    }
  }
}
```

## 错误处理

### 常见错误

1. **"没有活跃的WebSocket连接"**
   - 确保 Figma 插件已启动并连接
   - 检查 WebSocket 连接状态

2. **"用户未加入任何频道"**
   - 首先调用 `join_channel` 工具
   - 或让插件自动加入频道

3. **"请求超时"**
   - 检查 Figma 是否响应（可能在执行复杂操作）
   - 检查网络连接
   - 查看插件控制台日志

4. **"无效的连接token"**
   - 检查 MCP Token 是否正确
   - 重新生成 MCP Token

## 调试技巧

### 1. 查看 WebSocket 连接状态

访问 Dashboard 页面，节点树标题右侧会显示 WebSocket 连接状态：
- 🔗 绿色 = 已连接
- 🔌 灰色 = 未连接

点击状态图标可打开 WebSocket 调试界面。

### 2. 查看 MCP 调用日志

访问 `/mcp/{project_id}/logs` 查看所有 MCP 调用记录，包括：
- 请求参数
- 响应数据
- 执行时间
- 错误信息

### 3. WebSocket 测试页面

访问 `/mcp-ws-test` 可以：
- 测试 WebSocket 连接
- 手动发送消息
- 查看实时日志

### 4. 插件控制台

在 Figma 中打开开发者控制台（Ctrl+Option+I / Ctrl+Alt+I）查看插件日志。

## 性能优化

### 1. 批量操作

对于大量节点操作，使用批量工具：
- `set_multiple_text_contents` 代替多次 `set_text_content`
- `delete_multiple_nodes` 代替多次 `delete_node`

### 2. 进度反馈

批量操作会发送进度更新：
```json
{
  "type": "progress_update",
  "commandId": "uuid",
  "progress": 50,
  "processedItems": 50,
  "totalItems": 100,
  "message": "Processing..."
}
```

### 3. 频道管理

- 为每个项目使用独立频道
- 避免在同一频道中混合不同操作
- 定期清理不活跃的连接

## 安全注意事项

1. **保护 MCP Token**
   - 不要在代码中硬编码
   - 不要提交到版本控制
   - 定期更换 Token

2. **WebSocket 认证**
   - 所有 WebSocket 连接都需要有效的 MCP Token
   - Token 绑定到特定用户
   - 自动验证用户权限

3. **数据访问**
   - 用户只能访问自己的项目
   - 所有操作都有日志记录
   - 支持权限审计

## 示例场景

### 场景 1：自动化设计审查

```typescript
// 1. 连接到频道
await tools.join_channel({ channel: "project_123" });

// 2. 获取所有文本节点
const textNodes = await tools.scan_text_nodes({ nodeId: "root" });

// 3. 检查文本内容
const issues = textNodes.filter(node => 
  node.text.length > 50 || 
  !node.text.match(/^[A-Z]/)
);

// 4. 生成报告
console.log(`发现 ${issues.length} 个文本问题`);
```

### 场景 2：批量本地化

```typescript
// 1. 扫描所有文本
const textNodes = await tools.scan_text_nodes({ nodeId: "screen-id" });

// 2. 翻译文本
const translations = textNodes.map(node => ({
  nodeId: node.id,
  text: translate(node.text, 'zh-CN', 'en-US')
}));

// 3. 批量更新
await tools.set_multiple_text_contents({
  nodeId: "screen-id",
  text: translations
});
```

### 场景 3：设计系统一致性检查

```typescript
// 1. 获取所有组件
const components = await tools.get_local_components();

// 2. 检查颜色一致性
for (const comp of components) {
  const info = await tools.get_node_info({ nodeId: comp.id });
  if (info.fills && !isValidBrandColor(info.fills[0])) {
    console.warn(`组件 ${comp.name} 使用了非品牌颜色`);
  }
}
```

## 支持与反馈

如有问题或建议，请：
1. 查看日志文件
2. 访问调试页面
3. 联系技术支持

---

**版本**: 1.0.0  
**更新日期**: 2025-12-01



