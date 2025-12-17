# 移除批量渲染功能 - 完整记录

## 修改日期
2025-12-01

## 问题描述
在移除 `dashboard.html` 和 `projects.html` 中的批量渲染功能后，出现了以下错误：

```
vue.min.js:6 Uncaught TypeError: Cannot read properties of undefined (reading 'props')
```

## 根本原因
`projects.js` 中的 Vue 实例使用了 `FigmaRenderMixin` mixin，但在 `projects.html` 中已经移除了 `figma-render.js` 的脚本引用，导致 `window.FigmaRenderMixin` 为 undefined。

## 解决方案

### 1. 移除模板文件中的渲染功能

#### `dashboard.html`
- ✅ 移除底部渲染按钮
- ✅ 移除渲染预览对话框
- ✅ 移除预览区渲染进度卡片
- ✅ 移除底部渲染状态栏

#### `projects.html`
- ✅ 移除分组渲染按钮
- ✅ 移除渲染对话框
- ✅ 移除渲染相关CSS样式
- ✅ 移除 `figma-render.js` 脚本引用

### 2. 修复JavaScript文件

#### `projects.js`

**修改前：**
```javascript
const ProjectListApp = {
    // 混入渲染功能
    mixins: [window.FigmaRenderMixin],
    
    data() {
        // ...
    }
};
```

**修改后：**
```javascript
const ProjectListApp = {
    data() {
        // ...
    }
};
```

**移除的方法：**
- `renderProjectNodes()` - 渲染单个项目节点
- `renderGroupNodes()` - 渲染整个分组的所有项目节点

**保留的方法：**
- `batchRefreshNodeTree()` - 批量更新节点树（此功能不属于渲染功能）

## 修改清单

| 文件 | 类型 | 修改内容 | 状态 |
|------|------|----------|------|
| `web/templates/dashboard.html` | 模板 | 移除渲染按钮、对话框、进度卡片、状态栏 | ✅ |
| `web/templates/projects.html` | 模板 | 移除渲染按钮、对话框、样式、脚本引用 | ✅ |
| `web/static/js/projects.js` | 脚本 | 移除 mixin 和渲染方法 | ✅ |

## 验证步骤

1. **清除浏览器缓存**
   - 强制刷新页面（Ctrl+Shift+R 或 Cmd+Shift+R）

2. **检查控制台**
   - 不应再有 "Cannot read properties of undefined" 错误
   - 不应再有对 `FigmaRenderMixin` 的引用错误

3. **功能测试**
   - 项目列表页面正常加载
   - 可以正常添加、编辑、删除项目
   - 分组展开/收起功能正常
   - 搜索和排序功能正常

## 注意事项

### 保留的功能
以下功能**未被移除**，仍然可用：

1. **批量更新节点树**（`batchRefreshNodeTree()`）
   - 用于更新节点的树形结构数据
   - 不涉及图片渲染

2. **导出功能**
   - 图文导出
   - 模板代码导出（Unity-AppUI、Android-Kotlin、iOS-Swift、Web-Vue）

### 影响范围
- **仅影响**：项目列表页面和Dashboard页面的批量渲染功能
- **不影响**：其他页面和功能

## 回滚方案

如果需要恢复批量渲染功能，需要：

1. 恢复 `web/templates/projects.html` 中的：
   - 渲染按钮
   - 渲染对话框
   - 渲染样式
   - `figma-render.js` 脚本引用

2. 恢复 `web/static/js/projects.js` 中的：
   - `mixins: [window.FigmaRenderMixin]`
   - `renderProjectNodes()` 方法
   - `renderGroupNodes()` 方法

3. 恢复 `web/templates/dashboard.html` 中的：
   - 渲染按钮和对话框
   - 渲染进度组件
   - 渲染状态栏

## 相关文件

### 已修改文件
- `web/templates/dashboard.html`
- `web/templates/projects.html`
- `web/static/js/projects.js`

### 依赖文件（未修改）
- `web/static/js/figma-render.js` - 渲染功能mixin（Dashboard仍在使用）
- `web/static/js/dashboard.js` - Dashboard主脚本

### 文档
- `docs/REMOVE_BATCH_RENDER_FEATURE.md` - 本文档

## 测试结果

- ✅ 项目列表页面加载正常
- ✅ 无JavaScript错误
- ✅ 所有非渲染功能正常工作
- ✅ Vue实例初始化成功

## 后续优化建议

1. **考虑移除 `figma-render.js`**
   - 如果Dashboard也不再需要批量渲染功能
   - 可以进一步简化代码结构

2. **代码清理**
   - 检查是否有其他地方引用了已删除的渲染方法
   - 清理相关的CSS类和样式定义

3. **文档更新**
   - 更新用户手册，说明渲染功能的变更
   - 更新API文档（如果有相关接口）

## 总结

本次修改成功移除了项目列表页面的批量渲染功能，并修复了由此引起的Vue初始化错误。所有核心功能保持正常运行，项目列表页面的用户体验得到改善（界面更简洁）。

