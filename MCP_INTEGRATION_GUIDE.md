# MCP (Model Context Protocol) 集成指南

## 概述

本项目已成功集成了MCP（Model Context Protocol）支持，为每个用户提供独立的MCP服务连接。MCP服务提供以下核心功能：

- 获取预览图
- 获取节点有效配置信息
- 获取配置方案提示词
- 配置节点参数

## 功能特性

### 1. 用户隔离
- 每个用户拥有独立的MCP连接
- 连接状态独立管理
- 调用日志按用户分离

### 2. 核心功能
- **预览图获取**: 获取Figma节点的预览图片
- **节点配置**: 读取和设置节点的配置参数
- **配置提示词**: 获取针对不同配置类型的AI提示词
- **连接管理**: 创建、断开、状态检查

### 3. 日志记录
- 完整的API调用日志
- 包含请求参数、响应数据、执行时间
- 错误信息记录

## API 接口

### 连接管理

#### 创建MCP连接
```http
POST /mcp/connect
```

#### 断开MCP连接
```http
DELETE /mcp/disconnect/{connection_id}
```

#### 获取连接状态
```http
GET /mcp/status/{connection_id}
GET /mcp/status  # 根据用户ID查找
```

#### 心跳检测
```http
POST /mcp/ping/{connection_id}
```

### 核心功能

#### 获取预览图
```http
GET /mcp/{connection_id}/preview/{project_id}/{node_id}
```

#### 获取节点配置
```http
GET /mcp/{connection_id}/config/{project_id}/{node_id}
```

#### 设置节点配置
```http
POST /mcp/{connection_id}/config/{project_id}/{node_id}
Content-Type: application/json

{
  "width": 200,
  "height": 100,
  "x": 10,
  "y": 20,
  "color": "#FF0000"
}
```

#### 获取配置提示词
```http
GET /mcp/{connection_id}/prompt/{project_id}/{node_id}?type=layout
```

支持的配置类型：
- `layout`: 布局配置
- `style`: 样式配置
- `animation`: 动画配置
- `default`: 默认配置

### 管理功能

#### 获取调用日志
```http
GET /mcp/logs?limit=20&offset=0
```

#### 列出所有连接
```http
GET /mcp/connections
```

## Web界面集成

### MCP管理面板

在项目编辑页面（Dashboard）中，点击节点树标题栏的MCP连接状态按钮可以打开MCP管理面板。

面板功能：
1. **连接状态显示**: 显示当前MCP连接状态
2. **连接管理**: 创建和断开MCP连接
3. **功能测试**: 测试各项MCP功能
4. **调用日志**: 查看最近的API调用记录

### 状态指示器

- 🟢 绿色圆形按钮：MCP已连接
- 🔴 红色X按钮：MCP未连接
- ⏳ 加载动画：正在处理连接请求

## 数据库结构

### MCPConnection - MCP连接信息
- `user_id`: 用户ID
- `connection_id`: 唯一连接标识
- `status`: 连接状态
- `last_ping`: 最后心跳时间

### MCPCallLog - MCP调用日志
- `user_id`: 用户ID
- `connection_id`: 连接ID
- `method`: 调用方法
- `parameters`: 请求参数（JSON）
- `response`: 响应数据（JSON）
- `status`: 调用状态
- `duration`: 执行时间

### MCPNodeConfig - MCP节点配置
- `user_id`: 用户ID
- `project_id`: 项目ID
- `node_id`: 节点ID
- `config_data`: 配置数据（JSON）
- `config_type`: 配置类型

### MCPPreviewCache - MCP预览图缓存
- `user_id`: 用户ID
- `project_id`: 项目ID
- `node_id`: 节点ID
- `preview_data`: 预览图二进制数据
- `expires_at`: 过期时间

## 测试

### 自动化测试脚本

运行PowerShell测试脚本：
```powershell
.\demo\test_mcp.ps1
```

测试内容包括：
1. 用户登录/注册
2. MCP连接创建
3. 各项功能测试
4. 调用日志检查
5. 连接断开

### 手动测试

1. 启动服务：`go run cmd/main.go`
2. 访问：`http://localhost:8080`
3. 登录后进入项目编辑页面
4. 点击MCP状态按钮打开管理面板
5. 测试各项功能

## 配置说明

### 环境变量

MCP服务使用现有的数据库配置，无需额外配置。

### 连接超时

- 连接过期时间：30分钟无活动
- 清理周期：每5分钟检查一次
- 心跳间隔：建议5-10分钟

## 故障排除

### 常见问题

1. **连接创建失败**
   - 检查用户是否已登录
   - 确认数据库连接正常

2. **功能调用失败**
   - 验证连接ID是否有效
   - 检查参数格式是否正确

3. **日志记录异常**
   - 确认数据库表已正确创建
   - 检查用户权限

### 调试方法

1. 查看服务器日志
2. 检查浏览器控制台
3. 使用API测试工具验证接口

## 扩展开发

### 添加新功能

1. 在 `internal/services/mcp.go` 中添加服务方法
2. 在 `internal/controllers/mcp.go` 中添加控制器方法
3. 在 `cmd/main.go` 中添加路由
4. 更新Web界面（可选）

### 自定义配置类型

在 `GetConfigPrompt` 方法中添加新的配置类型和对应的提示词。

## 安全考虑

- 所有MCP接口都需要用户登录
- 连接ID与用户ID绑定，防止越权访问
- 敏感数据不在日志中记录
- 定期清理过期连接和缓存

## 性能优化

- 预览图缓存机制
- 连接池管理
- 批量操作支持
- 异步处理长时间操作

---

*本文档描述了MCP集成的完整功能和使用方法。如有问题，请查看相关源码或联系开发团队。*
