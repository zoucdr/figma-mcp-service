# Figma Plugin 预览集成功能

## 概述

本文档描述了在 Figma Deliver 系统中实现的 Figma Plugin 预览集成功能。该功能允许 dashboard 页面直接通过 Figma Plugin API 获取节点预览，而不需要通过后端服务器，从而提供更快的预览体验和减少服务器负载。

## 功能特性

### 1. 预览策略
- **过滤预览功能**: **必须**使用 Figma Plugin API，不支持后端 API 回退
- **单个节点预览**: 优先使用 Figma Plugin API，当插件不可用时自动回退到后端 API
- **明确提示**: 过滤预览功能会明确提示用户需要在 Figma 环境中使用

### 2. 支持的预览类型
- **单个节点预览**: 支持预览单个 Figma 节点
- **批量节点预览**: 支持同时预览多个节点（用于过滤预览功能）
- **高质量图片**: 支持不同缩放比例和图片格式

### 3. 性能优化
- **Base64 数据URL**: Plugin API 返回 base64 编码的图片，无需额外网络请求
- **即时加载**: 插件生成的图片可以立即显示，无需等待网络加载
- **减少服务器负载**: 直接在客户端生成预览，减少服务器压力

## 技术实现

### 1. 文件结构

```
figma-plugin/
├── src/
│   └── code.js                 # 插件主逻辑，新增预览功能
├── ui/
│   ├── ui.html                # 插件UI界面
│   └── service.js             # 插件服务层，新增预览API
└── PLUGIN_PREVIEW_INTEGRATION.md

web/static/js/
├── figma-plugin-bridge.js     # 新增：插件通信桥梁
└── dashboard.js               # 修改：集成插件预览功能

web/templates/
└── dashboard.html             # 修改：引入插件桥梁脚本
```

### 2. 核心组件

#### A. Figma Plugin Bridge (`figma-plugin-bridge.js`)
- **作用**: 提供 dashboard 页面与 Figma Plugin 之间的通信桥梁
- **功能**:
  - 检测 Figma Plugin 是否可用
  - 提供预览 API 调用接口
  - 处理消息通信和错误处理
  - 管理请求超时和重试机制

#### B. Plugin 预览功能 (`figma-plugin/src/code.js`)
- **新增消息类型**:
  - `testPlugin`: 测试插件可用性
  - `previewNode`: 预览单个节点
  - `previewNodes`: 预览多个节点
- **功能**:
  - 使用 `figma.getNodeById()` 查找节点
  - 使用 `node.exportAsync()` 导出图片
  - 使用 `figma.base64Encode()` 编码图片
  - 返回 base64 数据URL

#### C. Dashboard 集成 (`dashboard.js`)
- **修改的函数**:
  - `addPreviewImage()`: 添加插件 API 支持
  - `getFilteredNodeImages()`: 批量预览支持插件 API
- **新增的辅助函数**:
  - `addPreviewImageFromPlugin()`: 使用插件 API 获取预览
  - `addPreviewImageFromBackend()`: 使用后端 API 获取预览
  - `processNodeBounds()`: 处理节点边界信息
  - `updatePreviewImagesList()`: 更新预览图片列表
  - `updateImageLoadingState()`: 更新加载状态
  - `updateMinimapDisplay()`: 更新缩略图显示

### 3. 通信流程

#### 单个节点预览流程:
```
Dashboard → FigmaPluginBridge → Figma Plugin → Figma API → Plugin → Bridge → Dashboard
```

1. Dashboard 调用 `figmaPluginBridge.previewNode(nodeId)`
2. Bridge 发送消息到 Figma Plugin
3. Plugin 使用 `figma.getNodeById()` 和 `node.exportAsync()` 获取图片
4. Plugin 返回 base64 编码的图片数据
5. Bridge 将结果返回给 Dashboard
6. Dashboard 显示预览图片

#### 批量节点预览流程:
```
Dashboard → FigmaPluginBridge → Figma Plugin → (循环处理多个节点) → Plugin → Bridge → Dashboard
```

### 4. 错误处理和回退机制

#### 检测机制:
- 启动时发送测试消息检测插件可用性
- 设置 3 秒超时检测插件响应
- 动态更新插件可用状态

#### 回退策略:
- 插件不可用时自动使用后端 API
- 插件调用失败时回退到后端 API
- 保持用户体验的一致性

#### 超时处理:
- 单个节点预览: 30 秒超时
- 批量节点预览: 60 秒超时
- 插件检测: 3 秒超时

## 使用场景

### 1. 在 Figma 环境中使用
当用户在 Figma 中打开 dashboard 页面时：
- 自动检测到 Figma Plugin 可用
- 所有预览请求优先使用插件 API
- 获得更快的预览加载速度

### 2. 在浏览器中使用
当用户在普通浏览器中访问 dashboard 时：
- 检测到插件不可用
- 自动回退到后端 API
- 保持原有的预览功能

### 3. 混合环境
在某些情况下（如插件版本不兼容）：
- 动态检测插件状态
- 智能选择最佳的预览方式
- 确保功能的可靠性

## 配置和部署

### 1. 前端配置
在 `dashboard.html` 中引入插件桥梁脚本：
```html
<script src="/static/js/figma-plugin-bridge.js?v={{ .timestamp }}"></script>
<script src="/static/js/dashboard.js?v={{ .timestamp }}"></script>
```

### 2. 插件配置
确保 Figma Plugin 包含预览功能：
- 更新 `manifest.json` 权限设置
- 部署包含预览功能的插件代码

### 3. 兼容性
- 支持所有现代浏览器
- 兼容 Figma 桌面版和网页版
- 向后兼容原有的后端 API

## 性能优势

### 1. 速度提升
- **无网络延迟**: 插件 API 直接在客户端生成图片
- **即时预览**: Base64 数据可以立即显示
- **并行处理**: 多个节点可以并行导出

### 2. 服务器负载减少
- **减少 API 调用**: 预览请求不再经过服务器
- **降低带宽使用**: 无需从服务器下载图片
- **提高可扩展性**: 服务器资源可用于其他功能

### 3. 用户体验改善
- **更快的响应**: 预览加载更快
- **更好的交互**: 减少等待时间
- **更稳定的服务**: 减少网络依赖

## 未来扩展

### 1. 功能扩展
- 支持更多图片格式和质量选项
- 添加预览缓存机制
- 支持批量导出功能

### 2. 性能优化
- 实现预览图片的本地缓存
- 添加预加载机制
- 优化大批量节点的处理

### 3. 用户体验
- 添加预览进度指示器
- 支持预览图片的拖拽和缩放
- 提供预览质量选择选项

## 总结

Figma Plugin 预览集成功能成功实现了 dashboard 页面与 Figma Plugin 的无缝集成，提供了更快、更高效的节点预览体验。通过智能的回退机制，确保了功能在各种环境下的可靠性和兼容性。这一功能显著提升了用户体验，同时减少了服务器负载，为系统的可扩展性奠定了基础。
