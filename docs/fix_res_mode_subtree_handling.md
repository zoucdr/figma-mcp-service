# 父子res_mode共存机制修复说明

## 问题描述

在 `APIGetOptimizedNodes` 处理节点树时，当父节点标记为资源图（sprite/texture/slice）时，子节点的处理逻辑存在问题。

### 问题场景

```
tree1 (sprite)
├─ subnode1
├─ subnode2
   ├─ subnode21
   └─ subnode22 (sprite)
```

**原逻辑（错误）**：
- ❌ 因为 `subnode22` 设置了 sprite，整个 `subnode2` 分支被保留
- ❌ `subnode21` 没有被剔除，即使它不是文本也没有设置res_mode

**正确逻辑**：
- ✅ `subnode22` 设置了sprite，应该保留（会单独渲染）
- ✅ `subnode2` 作为 `subnode22` 的父容器，应该保留（维持层级结构）
- ✅ `subnode21` 应该被剔除（装饰性节点，会被包含在父节点的图片中）
- ✅ `subnode1` 应该被剔除（装饰性节点，会被包含在 tree1 的图片中）

## 核心原则

当父节点设置了图片资源模式（sprite/texture/slice）时：

1. **保留设置了res_mode的直接/间接子节点**
   - 这些节点会单独渲染，从父节点图片中自动剔除

2. **保留TEXT类型的直接/间接子节点**
   - 文本节点需要动态渲染，不应包含在静态图片中

3. **保留包含res_mode或TEXT的子节点的祖先链**
   - 为了维持正确的层级结构，必须保留这些节点的父容器

4. **剔除纯装饰性的子节点**
   - 没有res_mode、不是TEXT、后代中也没有res_mode或TEXT的节点
   - 这些节点会被统一渲染到父节点的图片中

## 代码修复

### 修改的函数

#### 1. `processNodeResMode` - 第1738-1777行
```go
case "sprite", "slice", "texture":
    // 当父节点设置了图片资源模式时，只保留以下**直接子节点**：
    // 1. 子节点设置了res_mode（会单独渲染）
    // 2. 子节点为TEXT类型（保留为动态文本）
    // 3. 子节点的后代中包含设置了res_mode或TEXT类型的节点
```

**关键改动**：
- 明确只检查**直接子节点**
- 使用 `hasResNodeOrTextInDescendants` 检查后代中是否有需要保留的节点

#### 2. `processNodeResModeSingle` - 第1851-1890行
同样的逻辑修复，确保两个处理函数的行为一致。

#### 3. `hasResNodeOrTextInDescendants` - 新函数（原 `hasImageNodeInSubtree`）
```go
// hasResNodeOrTextInDescendants 检查节点的后代中是否包含设置了res_mode的节点或TEXT类型节点
// 用于判断子节点是否需要保留（父子res_mode共存机制）
// 注意：只检查后代节点，不检查节点本身
func hasResNodeOrTextInDescendants(nodeID string, childrenMap map[string][]*gin.H, nodeModifys map[string]map[string]interface{}) bool
```

**关键改动**：
- 重命名为更准确的名称
- 只递归检查后代节点
- 清晰的逻辑：先检查TEXT类型，再检查res_mode，最后递归

## 测试场景

### 场景1：嵌套的图片资源
```
container (sprite)          # 设置为sprite
├─ decoration1              # 应该被剔除（装饰性）
├─ decoration2              # 应该被剔除（装饰性）
└─ button (sprite)          # 应该保留（设置了sprite）
```

**预期结果**：
- 保留：container, button
- 剔除：decoration1, decoration2
- 渲染：container.png（不包含button）, button.png

### 场景2：多层嵌套
```
panel (texture)             # 设置为texture
├─ header                   # 应该保留（包含子节点titleText）
│  ├─ bgShape               # 应该被剔除（装饰性）
│  └─ titleText (TEXT)      # 应该保留（TEXT类型）
├─ content                  # 应该被剔除（纯装饰性容器）
│  ├─ shape1                # 应该被剔除（装饰性）
│  └─ shape2                # 应该被剔除（装饰性）
└─ closeBtn (sprite)        # 应该保留（设置了sprite）
```

**预期结果**：
- 保留：panel, header, titleText, closeBtn
- 剔除：bgShape, content, shape1, shape2
- 渲染：panel.png（背景，不包含closeBtn）, closeBtn.png
- titleText 作为动态文本在运行时渲染

### 场景3：深层嵌套的图片
```
card (sprite)               # 设置为sprite
├─ border                   # 应该保留（包含后代icon）
│  └─ wrapper               # 应该保留（包含后代icon）
│     └─ icon (sprite)      # 应该保留（设置了sprite）
└─ background               # 应该被剔除（装饰性）
```

**预期结果**：
- 保留：card, border, wrapper, icon
- 剔除：background
- 渲染：card.png（不包含icon）, icon.png

## 技术要点

### 1. 父子res_mode共存机制
- 父节点设置res_mode不影响子节点
- 子节点可以独立设置res_mode
- 子节点设置res_mode后，自动从父节点渲染中剔除

### 2. 祖先链保留
- 当需要保留某个深层节点时，必须保留其所有祖先节点
- 这样才能维持正确的层级结构和布局关系

### 3. 递归处理
- 从根节点开始递归处理所有节点
- 对每个设置了图片资源模式的节点，检查其所有直接子节点
- 递归检查后代中是否有需要保留的节点

### 4. 性能优化
- 使用 `nodesToRemove` map 标记需要删除的节点
- 避免重复处理已标记删除的节点
- 一次性剔除整个子树，不需要逐个处理

## 影响范围

### API接口
- `GET /api/:mcptoken/optimized-nodes` - 优化节点数据接口

### 影响的客户端
- Cursor MCP 工具
- Dashboard 前端
- 所有依赖优化节点数据的客户端

## 向后兼容性

✅ **完全向后兼容**

- 修复后的逻辑更加准确，不会破坏现有功能
- 只是更精确地剔除装饰性节点
- 不影响已有的项目配置

## 测试建议

1. **基础测试**：验证简单的父子res_mode场景
2. **嵌套测试**：验证多层嵌套的复杂场景
3. **混合测试**：验证图片、文本、装饰混合的场景
4. **边界测试**：验证空子树、全TEXT子树等边界情况

## 相关文件

- `internal/controllers/api.go` - 主要修改文件
- `internal/services/figma_service.go` - 图片渲染服务
- `figma-plugin/code.js` - Figma插件（图片导出）

## 版本信息

- 修复日期：2025-12-16
- 修复版本：v1.0.0
- 相关Issue：父子res_mode共存机制优化

## 总结

这次修复确保了父子res_mode共存机制的正确实现：
- ✅ 父节点设置图片资源模式时，只包含装饰性子节点
- ✅ 设置了res_mode的子节点会单独渲染
- ✅ TEXT节点被保留用于动态渲染
- ✅ 维持正确的层级结构
- ✅ 优化最终输出的JSON大小

