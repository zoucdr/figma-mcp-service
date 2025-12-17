# Cursor MCP 集成指南

本指南将帮助您在Cursor中配置Figma Deliver的MCP服务。

## 前提条件

1. 确保Figma Deliver服务正在运行（默认端口8080）
2. 已经在Web界面中登录并创建了MCP连接

## 配置步骤

### 1. 获取MCP Token

**方法一：在Profile界面生成（推荐）**

1. 打开Figma Deliver Web界面：http://localhost:8080
2. 登录您的账户
3. 点击右上角用户名，选择"个人资料"
4. 在"MCP Token"部分点击"生成Token"按钮
5. 复制生成的Token（格式类似：`mcp_1_ABC123DEF456...`）
6. 在同一页面可以看到完整的Cursor配置示例

**方法二：在项目页面创建连接**

1. 进入项目页面，点击右上角的MCP连接状态按钮
2. 点击"连接MCP"按钮创建连接
3. 复制生成的连接ID

### 2. 配置Cursor MCP

#### 方法一：在Cursor设置中配置（推荐）

1. 打开Cursor，进入设置 (Ctrl/Cmd + ,)
2. 搜索 "MCP" 或找到 "Model Context Protocol" 设置
3. 添加新的MCP服务器配置：

```json
{
  "mcpServers": {
    "FigmaDeliver": {
      "url": "http://127.0.0.1:8080/mcp/YOUR_TOKEN_HERE"
    }
  }
}
```

4. 将 `YOUR_TOKEN_HERE` 替换为您在步骤1中获取的token

#### 方法二：使用配置文件

1. 在用户配置目录创建或编辑 `mcp.json` 文件：
   - Windows: `C:\Users\{username}\.cursor\mcp.json`
   - macOS: `~/.cursor/mcp.json`
   - Linux: `~/.cursor/mcp.json`

```json
{
  "mcpServers": {
    "FigmaDeliver": {
      "url": "http://127.0.0.1:8080/mcp/YOUR_TOKEN_HERE"
    }
  }
}
```

2. 将 `YOUR_TOKEN_HERE` 替换为您在步骤1中获取的token

#### 方法二：直接HTTP调用

您也可以直接在Cursor中使用HTTP请求调用MCP接口：

```bash
# 检查连接状态
curl "http://localhost:8080/mcp/YOUR_TOKEN_HERE/status"

# 获取预览图
curl "http://localhost:8080/mcp/YOUR_TOKEN_HERE/preview/PROJECT_ID/NODE_ID"

# 获取节点配置
curl "http://localhost:8080/mcp/YOUR_TOKEN_HERE/config/PROJECT_ID/NODE_ID"

# 设置节点配置
curl -X POST "http://localhost:8080/mcp/YOUR_TOKEN_HERE/config/PROJECT_ID/NODE_ID" \
  -H "Content-Type: application/json" \
  -d '{"width": 800, "height": 600, "background": "#FFFFFF"}'

# 获取配置提示词
curl "http://localhost:8080/mcp/YOUR_TOKEN_HERE/prompt/PROJECT_ID/NODE_ID?type=ui"

# MCP协议调用示例
curl -X POST "http://localhost:8080/mcp/YOUR_TOKEN_HERE" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
      "name": "get_preview",
      "arguments": {
        "project_id": "PROJECT_ID",
        "node_id": "NODE_ID"
      }
    }
  }'
```

## 可用的MCP功能

### 1. 获取预览图 (`get_preview`)
- **路径**: `/mcp/{token}/preview/{project_id}/{node_id}`
- **方法**: GET
- **描述**: 获取指定节点的预览图信息

### 2. 获取节点配置 (`get_node_tree`)
- **路径**: `/mcp/{token}/config/{project_id}/{node_id}`
- **方法**: GET
- **描述**: 获取节点的详细配置信息

### 3. 设置节点配置 (`set_nodes_modify`)
- **路径**: `/mcp/{token}/config/{project_id}/{node_id}`
- **方法**: POST
- **描述**: 更新节点的配置参数

### 4. 获取配置提示词 (`get_modify_prompt`)
- **路径**: `/mcp/{token}/prompt/{project_id}/{node_id}`
- **方法**: GET
- **参数**: `type` (可选: default, ui, layout)
- **描述**: 获取针对特定配置类型的提示词

### 5. 检查连接状态 (`check_status`)
- **路径**: `/mcp/{token}/status`
- **方法**: GET
- **描述**: 检查MCP连接的状态和活动情况

### 6. MCP协议调用
- **路径**: `/mcp/{token}`
- **方法**: POST
- **描述**: 通过标准MCP协议调用工具

## 使用示例

### 在Cursor中使用MCP工具

一旦配置完成，您可以在Cursor中这样使用：

```
@figma-deliver 请帮我获取项目123中节点456的预览图
```

```
@figma-deliver 请获取项目123中节点456的配置信息
```

```
@figma-deliver 请为项目123中的节点456设置宽度为800px，高度为600px
```

```
@figma-deliver 请给我一些关于UI配置的提示词，项目ID是123，节点ID是456
```

## 故障排除

### 1. 连接失败
- 确保Figma Deliver服务正在运行
- 检查token是否正确
- 确认服务器地址和端口

### 2. Token过期
- 在Web界面重新创建MCP连接
- 更新配置文件中的token

### 3. 权限错误
- 确保您已经在Web界面中登录
- 检查token是否属于正确的用户

## 高级配置

### 自定义服务器地址

如果您的服务运行在不同的地址或端口，请修改配置中的URL：

```json
{
  "mcpServers": {
    "figma-deliver": {
      "command": "curl",
      "args": [
        "-s",
        "-X", "GET",
        "-H", "Content-Type: application/json",
        "http://YOUR_SERVER:YOUR_PORT/cursor-mcp/YOUR_TOKEN_HERE/status"
      ]
    }
  }
}
```

### 环境变量配置

您可以使用环境变量来管理配置：

```bash
export FIGMA_DELIVER_URL="http://localhost:8080"
export FIGMA_DELIVER_TOKEN="your_token_here"
```

然后在配置文件中引用：

```json
{
  "mcpServers": {
    "figma-deliver": {
      "command": "sh",
      "args": [
        "-c",
        "curl -s -X GET -H 'Content-Type: application/json' $FIGMA_DELIVER_URL/cursor-mcp/$FIGMA_DELIVER_TOKEN/status"
      ]
    }
  }
}
```

## 测试

在完成配置后，您可以运行测试脚本来验证连接：

```bash
# 在项目根目录运行
./demo/test_cursor_mcp.ps1 -Token "your_token_here"

# 或者指定自定义参数
./demo/test_cursor_mcp.ps1 -Token "your_token_here" -BaseUrl "http://localhost:8080" -ProjectId "your_project_id" -NodeId "your_node_id"
```

测试脚本将验证所有MCP接口的功能，包括：
- 连接状态检查
- 预览图获取
- 节点配置读取和设置
- 配置提示词获取

## 支持

如果您遇到问题，请：

1. 检查Figma Deliver服务的日志
2. 在Web界面查看MCP调用日志
3. 确认网络连接和防火墙设置
4. 运行测试脚本验证接口功能

更多信息请参考项目文档或联系技术支持。
