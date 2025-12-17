# 下载优化JSON按钮右键拷贝链接功能

## 功能概述
在Dashboard页面底部的"下载优化JSON"按钮上添加了右键菜单功能，允许用户通过右键菜单直接拷贝下载链接到剪贴板。

## 使用方法

### 方式一：直接点击下载
- 左键点击底部绿色圆形下载按钮，直接下载优化后的JSON数据

### 方式二：右键拷贝链接
1. 右键点击底部绿色圆形下载按钮
2. 在弹出的右键菜单中选择"拷贝下载链接"
3. 链接将被自动复制到剪贴板
4. 可以将链接粘贴到任何地方使用

## 右键菜单选项
- **拷贝下载链接**：将完整的下载API链接复制到剪贴板（包含域名和路径）
- **下载**：直接触发下载操作（与左键点击效果相同）

## 技术实现

### 1. HTML模板修改 (dashboard.html)
- 在下载按钮上添加了 `@contextmenu.prevent.native` 事件监听器
- 添加了右键菜单的DOM结构

### 2. JavaScript逻辑修改 (dashboard.js)
- 添加了 `downloadMenuVisible` 和 `downloadMenuStyle` 数据属性
- 实现了 `handleDownloadButtonContextMenu` 方法处理右键点击
- 实现了 `copyDownloadLink` 方法复制链接到剪贴板
- 实现了 `fallbackCopyToClipboard` 方法作为降级方案，兼容旧浏览器

### 3. CSS样式修改 (dashboard.css)
- 添加了 `.download-context-menu` 样式类
- 菜单样式与现有的层级菜单保持一致

## 浏览器兼容性
- 现代浏览器：使用 `navigator.clipboard.writeText()` API
- 旧版浏览器：自动降级到 `document.execCommand('copy')` 方法
- 支持所有主流浏览器（Chrome, Firefox, Safari, Edge等）

## 注意事项
1. 功能需要有效的MCP Token才能使用
2. 如果没有MCP Token，按钮会显示为禁用状态
3. 右键菜单会在点击其他区域时自动关闭
4. 复制的链接是完整的URL，包含域名和协议（http/https）

## 相关文件
- `web/templates/dashboard.html` - HTML模板
- `web/static/js/dashboard.js` - JavaScript逻辑
- `web/static/css/dashboard.css` - CSS样式


