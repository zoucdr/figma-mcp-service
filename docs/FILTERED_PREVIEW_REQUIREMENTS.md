# 过滤预览功能要求

## 📋 功能说明

根据你的要求，**过滤预览功能必须走 figma-plugin**，不允许智能切换到后端 API。

## ✅ 已完成的修改

### 1. 强制使用 Figma Plugin API
- 修改了 `web/static/js/dashboard.js` 中的 `getFilteredNodeImages()` 函数
- 移除了后端 API 回退逻辑
- 当 Figma Plugin 不可用时，直接返回空结果并显示错误提示

### 2. 用户界面提示
- 在 `web/templates/dashboard.html` 中的过滤预览标题旁添加了信息图标
- 鼠标悬停时显示提示："过滤预览功能需要在Figma环境中使用Figma Plugin"

### 3. 错误处理
- 当插件不可用时，显示明确的错误消息
- 提示用户确保 Figma Plugin 已正确安装和配置

### 4. 文档更新
- 更新了所有相关文档，明确说明过滤预览功能的要求
- 区分了过滤预览（强制 Plugin）和单个节点预览（智能切换）的不同策略

## 🔧 技术实现

### 修改前的逻辑（智能切换）：
```javascript
// 优先尝试使用Figma Plugin API
if (window.figmaPluginBridge && window.figmaPluginBridge.isAvailable()) {
    // 使用 Plugin API
} else {
    // 回退到后端 API
}
```

### 修改后的逻辑（强制 Plugin）：
```javascript
// 过滤预览功能必须使用Figma Plugin API
if (!window.figmaPluginBridge || !window.figmaPluginBridge.isAvailable()) {
    console.error('过滤预览功能需要Figma Plugin支持，但插件不可用');
    this.$message.error('过滤预览功能需要在Figma环境中使用，请确保Figma Plugin已正确安装和配置');
    return {};
}
// 只使用 Plugin API，不回退
```

## 🎯 功能区别

| 功能 | API 策略 | 环境要求 | 回退机制 |
|------|----------|----------|----------|
| **过滤预览** | 强制 Figma Plugin API | 必须在 Figma 中 | ❌ 无回退 |
| **单个节点预览** | 智能切换 | 任何环境 | ✅ 自动回退到后端 |

## 🚀 使用体验

### 在 Figma 环境中：
1. 过滤预览功能正常工作，使用 Plugin API 快速生成预览
2. 单个节点预览也使用 Plugin API，速度更快

### 在浏览器环境中：
1. 过滤预览功能显示错误提示，不可用
2. 单个节点预览自动使用后端 API，仍然可用

## 📱 用户提示

- **界面提示**：过滤预览标题旁的信息图标
- **错误消息**：当插件不可用时的明确提示
- **文档说明**：在所有相关文档中明确说明要求

## 🔍 测试建议

1. **在 Figma 中测试**：
   - 安装并配置 Figma Plugin
   - 测试过滤预览功能是否正常工作
   - 验证单个节点预览也使用 Plugin API

2. **在浏览器中测试**：
   - 直接访问 Dashboard 页面
   - 验证过滤预览功能显示错误提示
   - 确认单个节点预览仍然可用（使用后端 API）

## 📝 注意事项

- 过滤预览功能现在完全依赖 Figma Plugin，确保插件稳定性
- 用户需要明确了解在不同环境中的功能可用性
- 建议在 Figma 环境中使用完整功能，在浏览器中查看已有预览

---

**总结**：过滤预览功能现在严格要求使用 Figma Plugin API，不再有后端 API 回退机制，确保了功能的专一性和性能优势。
