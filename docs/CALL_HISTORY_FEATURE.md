# MCP 调用记录功能

## 功能概述

Figma-Deliver MCP 插件现在支持调用记录功能，可以追踪和显示所有通过 MCP 发送到 Figma 的命令调用。

## 功能特点

### 1. 调用记录显示
- 显示最近 50 条 MCP 调用记录
- 按时间倒序排列（最新的在最上面）
- 记录自动保存到 Figma 客户端存储中

### 2. 记录信息包含
- **命令名称**: 被调用的 MCP 工具名称（如 `get_document_info`, `get_selection` 等）
- **调用时间**: 显示相对时间（刚刚、5分钟前、1小时前等）
- **调用参数**: 以 JSON 格式显示传递的参数
- **执行状态**: 成功或失败
- **错误信息**: 如果执行失败，显示详细错误信息

### 3. 可视化设计
- 成功调用：绿色左边框 + 绿色状态标签
- 失败调用：红色左边框 + 红色状态标签
- 错误消息以红色背景突出显示

### 4. 交互功能
- 清空记录按钮：一键清除所有历史记录
- 自动滚动：支持浏览大量历史记录
- 参数折叠：长参数可滚动查看

## 使用方法

### 查看调用记录
1. 打开 Figma-Deliver MCP 插件
2. 点击顶部的 "调用记录" 标签
3. 查看所有历史调用记录

### 清空记录
1. 进入 "调用记录" 标签
2. 点击 "清空记录" 按钮
3. 确认清空操作

## 技术实现

### 前端（ui.html）
- 调用记录存储在 `state.callHistory` 数组中
- 使用 `addCallToHistory()` 函数记录每次调用
- 使用 `renderCallHistory()` 函数渲染列表
- 通过 `formatTime()` 格式化显示时间

### 后端（code.js）
- 使用 `figma.clientStorage` API 持久化存储记录
- `saveCallHistory()`: 保存记录到存储
- `loadCallHistory()`: 从存储加载记录
- 在命令执行成功或失败时返回命令信息

### 数据结构
```javascript
{
  id: "unique-id",           // 唯一标识
  command: "get_selection",  // 命令名称
  params: {...},             // 命令参数
  status: "success",         // 状态: success 或 error
  error: null,               // 错误信息（如果有）
  timestamp: Date            // 时间戳
}
```

## 支持的命令

所有通过 WebSocket 发送到插件的 MCP 命令都会被记录，包括但不限于：

- `get_document_info` - 获取文档信息
- `get_selection` - 获取选中节点
- `get_node_info` - 获取节点信息
- `read_my_design` - 读取设计信息
- `create_frame` - 创建 Frame
- `create_text` - 创建文本
- `set_text_content` - 修改文本内容
- `move_node` - 移动节点
- `resize_node` - 调整节点大小
- `delete_node` - 删除节点
- 以及其他所有 MCP 工具命令

## 注意事项

1. **存储限制**: 仅保存最近 50 条记录
2. **数据持久化**: 记录保存在 Figma 客户端本地存储中，不会上传到服务器
3. **性能优化**: 大量记录会被自动截断以保持性能
4. **时间显示**: 使用相对时间，超过24小时显示具体日期时间

## 未来优化

- [ ] 支持按命令类型筛选
- [ ] 支持搜索功能
- [ ] 导出调用记录为 JSON
- [ ] 显示调用耗时
- [ ] 调用统计图表

