# MCP Resources 快速入门

## 功能说明

现在 Figma Deliver Service 支持 MCP Resources 功能，允许客户端（如 Cursor IDE）通过 MCP 协议获取项目的 UI 预览图。

## 核心特性

✅ **自动发现项目** - 列出用户的所有 Figma 项目  
✅ **UI 预览图** - 为每个项目提供根节点的预览图  
✅ **Base64 编码** - 图片数据直接在 MCP 响应中返回  
✅ **权限验证** - 只能访问自己的项目资源

## 支持的 MCP 方法

### 1. initialize

声明支持 resources 能力：

```json
{
  "jsonrpc": "2.0",
  "method": "initialize",
  "id": 1
}
```

### 2. resources/list

获取所有可用资源：

```json
{
  "jsonrpc": "2.0",
  "method": "resources/list",
  "id": 2
}
```

### 3. resources/read

读取具体资源内容：

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

## 资源 URI 格式

```
figma://project/{project_id}/ui_preview
```

示例：
- `figma://project/1/ui_preview` - 项目 1 的 UI 预览图
- `figma://project/2/ui_preview` - 项目 2 的 UI 预览图

## 在 Cursor 中使用

1. **配置 MCP 服务器**

编辑 Cursor 的 MCP 配置文件，添加：

```json
{
  "mcpServers": {
    "figma-deliver": {
      "url": "http://localhost:8080/mcp/{your_mcp_token}"
    }
  }
}
```

2. **自动加载资源**

Cursor 会自动调用 `resources/list` 获取可用的 UI 预览图资源。

3. **在对话中使用**

AI 助手可以访问这些 UI 预览图，提供基于实际设计的建议。

## 测试

运行测试脚本验证功能：

```powershell
# 1. 修改脚本中的 MCP Token
# 编辑 demo/test_resources.ps1
# $mcpToken = "mcp_your_actual_token"

# 2. 运行测试
.\demo\test_resources.ps1
```

测试结果会显示：
- ✓ Initialize 是否支持 resources
- ✓ 可用的资源列表
- ✓ 读取资源内容是否成功

## 前置条件

使用此功能需要：

1. ✅ 用户已登录系统
2. ✅ 用户已配置 Figma Token
3. ✅ 用户已创建至少一个项目
4. ✅ 服务正在运行 (http://localhost:8080)

## 常见问题

### Q: 为什么 resources/list 返回空列表？

A: 可能的原因：
- 用户未配置 Figma Token
- 用户还没有创建任何项目

### Q: resources/read 返回"项目不存在"错误？

A: 可能的原因：
- 项目 ID 不正确
- 项目不属于当前用户
- 项目已被删除

### Q: 预览图生成失败？

A: 可能的原因：
- Figma Token 无效或过期
- 网络连接问题
- Figma API 限流

## 技术细节

### 预览图参数

- **格式**: PNG
- **缩放比例**: 0.1 (10%)
- **编码**: Base64

### 性能优化

- 预览图会缓存到本地临时目录
- 避免重复下载相同的预览图
- 错误不会阻塞其他资源的返回

## 相关文档

- 📄 [完整功能说明](./MCP_RESOURCES_FEATURE.md)
- 📄 [实现总结](./MCP_RESOURCES_IMPLEMENTATION_SUMMARY.md)
- 📄 [MCP 集成指南](./MCP_INTEGRATION_GUIDE.md)

## 更新日志

### v1.0.0 (2024)

- ✅ 添加 resources/list 支持
- ✅ 添加 resources/read 支持
- ✅ 实现 UI 预览图资源类型
- ✅ 添加用户权限验证
- ✅ 提供完整的错误处理

## 技术支持

如有问题，请查看：
1. 服务日志输出
2. MCP 调用日志（Web 界面）
3. 测试脚本的详细输出

