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

1. **装饰性节点类型的条件剔除**
   - 装饰性节点类型包括：
     - BOOLEAN_OPERATION（布尔运算节点）
     - VECTOR（矢量图形）
     - RECTANGLE（矩形）
     - ELLIPSE（椭圆）
     - LINE（线条）
     - POLYGON（多边形）
     - STAR（星形）
   - **如果未设置res_mode**：强制剔除，包含在父节点的图片中
   - **如果显式设置了res_mode（sprite/texture/slice）**：保留并单独导出

2. **保留设置了res_mode的直接/间接子节点**
   - 这些节点会单独渲染，从父节点图片中自动剔除

3. **保留TEXT类型的直接/间接子节点**
   - 文本节点需要动态渲染，不应包含在静态图片中

4. **保留包含res_mode或TEXT的子节点的祖先链**
   - 为了维持正确的层级结构，必须保留这些节点的父容器
   - 但祖先链中不能只包含装饰性节点

5. **剔除其他装饰性的子节点**
   - 没有res_mode、不是TEXT、后代中也没有res_mode或TEXT的节点
   - 这些节点会被统一渲染到父节点的图片中

## 代码修复（更新版）

### 新增函数

#### 1. `isAlwaysRemovableNodeType` - 判断装饰性节点类型
```go
// 装饰性节点类型列表（在父节点为图片资源时始终剔除）
- BOOLEAN_OPERATION: 布尔运算节点（Union、Subtract等）
- VECTOR: 矢量图形节点
- RECTANGLE: 矩形节点
- ELLIPSE: 椭圆节点
- LINE: 线条节点
- POLYGON: 多边形节点
- STAR: 星形节点
```

### 修改的函数

#### 1. `processNodeResMode` - 第1738-1783行
```go
case "sprite", "slice", "texture":
    // 当父节点设置了图片资源模式时，只保留以下**直接子节点**：
    // 1. 子节点设置了res_mode（会单独渲染）
    // 2. 子节点为TEXT类型（保留为动态文本）
    // 3. 子节点的后代中包含设置了res_mode或TEXT类型的节点
```

**关键改动**：
- 明确只检查**直接子节点**
- 添加**装饰性节点类型强制剔除**逻辑
- 使用 `isAlwaysRemovableNodeType` 判断是否为装饰性节点
- 使用 `hasResNodeOrTextInDescendants` 检查后代中是否有需要保留的节点

#### 2. `processNodeResModeSingle` - 第1867-1906行
同样的逻辑修复，确保两个处理函数的行为一致。

#### 3. `hasResNodeOrTextInDescendants` - 优化函数（原 `hasImageNodeInSubtree`）
```go
// hasResNodeOrTextInDescendants 检查节点的后代中是否包含设置了res_mode的节点或TEXT类型节点
// 用于判断子节点是否需要保留（父子res_mode共存机制）
// 注意：只检查后代节点，不检查节点本身
// 注意：装饰性节点类型（BOOLEAN_OPERATION、VECTOR等）不算作需要保留的节点
func hasResNodeOrTextInDescendants(nodeID string, childrenMap map[string][]*gin.H, nodeModifys map[string]map[string]interface{}) bool
```

**关键改动**：
- 重命名为更准确的名称
- 只递归检查后代节点
- **跳过装饰性节点类型**（BOOLEAN_OPERATION、VECTOR等）
- 清晰的逻辑：先过滤装饰性节点，再检查TEXT类型，再检查res_mode，最后递归

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
│  ├─ bgShape (RECTANGLE)   # 应该被剔除（装饰性节点，未设置res_mode）
│  └─ titleText (TEXT)      # 应该保留（TEXT类型）
├─ content                  # 应该被剔除（纯装饰性容器）
│  ├─ shape1 (VECTOR)       # 应该被剔除（装饰性节点，未设置res_mode）
│  └─ shape2 (ELLIPSE)      # 应该被剔除（装饰性节点，未设置res_mode）
└─ closeBtn (sprite)        # 应该保留（设置了sprite）
```

**预期结果**：
- 保留：panel, header, titleText, closeBtn
- 剔除：bgShape, content, shape1, shape2
- 渲染：panel.png（背景，不包含closeBtn）, closeBtn.png
- titleText 作为动态文本在运行时渲染

### 场景2.1：装饰性节点显式设置res_mode
```
panel (texture)             # 设置为texture
├─ background (RECTANGLE, sprite)  # 应该保留（虽然是装饰性类型，但设置了sprite）
├─ icon (VECTOR, sprite)    # 应该保留（虽然是装饰性类型，但设置了sprite）
└─ decoration (VECTOR)      # 应该被剔除（装饰性节点，未设置res_mode）
```

**预期结果**：
- 保留：panel, background, icon
- 剔除：decoration
- 渲染：panel.png（不包含background和icon）, background.png, icon.png

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
- ✅ **强制剔除装饰性节点类型**（BOOLEAN_OPERATION、VECTOR等）
- ✅ 设置了res_mode的子节点会单独渲染
- ✅ TEXT节点被保留用于动态渲染
- ✅ 维持正确的层级结构
- ✅ **更激进的剔除策略**，确保装饰性节点不会因为层级关系被意外保留
- ✅ 优化最终输出的JSON大小

## 修复版本

### v1.0.0 (2025-12-16)
- 初始修复：实现父子res_mode共存机制
- 修复祖先链保留逻辑

### v1.1.0 (2025-12-16)
- **新增装饰性节点类型强制剔除**
- 添加 `isAlwaysRemovableNodeType` 函数
- 优化 `hasResNodeOrTextInDescendants` 函数，跳过装饰性节点
- 更激进的剔除策略，确保 BOOLEAN_OPERATION、VECTOR 等装饰性节点被剔除
- 添加详细的调试日志，便于排查问题

### v1.2.0 (2025-12-16) ⭐ 当前版本
- **重要修复**：装饰性节点如果显式设置了res_mode，则不会被强制剔除
- 优化逻辑顺序：先检查res_mode，再判断是否为装饰性节点
- 更新 `hasResNodeOrTextInDescendants` 函数，正确识别设置了res_mode的装饰性节点
- 支持场景：用户想要单独导出某个VECTOR或BOOLEAN_OPERATION节点作为图片资源


