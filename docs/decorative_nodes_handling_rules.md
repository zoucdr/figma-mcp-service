# 装饰性节点处理规则

## 概述

当父节点设置了图片资源模式（sprite/texture/slice）时，对子节点的处理遵循以下规则。

## 装饰性节点类型

以下节点类型被认为是**装饰性节点**：

| 节点类型 | 说明 | 示例 |
|---------|------|------|
| `BOOLEAN_OPERATION` | 布尔运算节点 | Union、Subtract、Intersect、Exclude |
| `VECTOR` | 矢量图形 | 自定义矢量路径 |
| `RECTANGLE` | 矩形 | 圆角矩形、普通矩形 |
| `ELLIPSE` | 椭圆/圆形 | 圆形、椭圆形 |
| `LINE` | 线条 | 直线、箭头 |
| `POLYGON` | 多边形 | 三角形、五边形等 |
| `STAR` | 星形 | 五角星等 |

## 处理规则

### 规则1：TEXT 类型始终保留
```
✅ 保留：任何 TEXT 节点
原因：文本需要动态渲染，不应包含在静态图片中
```

### 规则2：设置了 res_mode 的节点保留
```
✅ 保留：任何设置了 sprite/texture/slice 的节点
原因：这些节点会单独导出为图片资源

示例：
icon (VECTOR, sprite)  ← 保留（虽然是VECTOR，但设置了sprite）
```

### 规则3：装饰性节点条件剔除
```
❌ 剔除：装饰性节点 && 未设置 res_mode
✅ 保留：装饰性节点 && 设置了 res_mode

示例：
shape1 (RECTANGLE)          ← 剔除（装饰性，未设置res_mode）
shape2 (RECTANGLE, sprite)  ← 保留（设置了sprite）
```

### 规则4：容器节点条件保留
```
✅ 保留：容器节点 && 后代中包含需要保留的节点
❌ 剔除：容器节点 && 后代中没有需要保留的节点

需要保留的节点包括：
- TEXT 节点
- 设置了 res_mode 的节点（包括装饰性节点）
```

## 处理流程图

```
父节点设置了图片资源模式 (sprite/texture/slice)
│
├─ 遍历所有直接子节点
│  │
│  ├─ 步骤1：检查子节点是否设置了 res_mode？
│  │  ├─ 是 → ✅ 保留（会单独渲染）
│  │  └─ 否 → 继续下一步
│  │
│  ├─ 步骤2：检查子节点是否为装饰性类型？
│  │  ├─ 是 → ❌ 剔除（包含在父节点图片中）
│  │  └─ 否 → 继续下一步
│  │
│  ├─ 步骤3：检查子节点是否为 TEXT 类型？
│  │  ├─ 是 → ✅ 保留（动态文本）
│  │  └─ 否 → 继续下一步
│  │
│  └─ 步骤4：检查子节点后代中是否有需要保留的节点？
│     ├─ 是 → ✅ 保留（维持层级结构）
│     └─ 否 → ❌ 剔除（纯装饰性容器）
│
└─ 递归处理所有保留的子节点
```

## 实际案例

### 案例1：按钮组件
```
buttonContainer (sprite)
├─ background (RECTANGLE)       ← ❌ 剔除（装饰性，未设置res_mode）
├─ icon (VECTOR, sprite)        ← ✅ 保留（设置了sprite）
└─ labelText (TEXT)             ← ✅ 保留（TEXT类型）

结果：
- buttonContainer.png（只包含background）
- icon.png（单独导出）
- labelText 运行时渲染
```

### 案例2：卡片组件
```
card (texture)
├─ border (RECTANGLE)           ← ❌ 剔除（装饰性，未设置res_mode）
├─ shadow (ELLIPSE)             ← ❌ 剔除（装饰性，未设置res_mode）
├─ avatar (ELLIPSE, sprite)     ← ✅ 保留（设置了sprite）
└─ contentBox                   ← ✅ 保留（包含后代titleText）
   └─ titleText (TEXT)          ← ✅ 保留（TEXT类型）

结果：
- card.png（包含border和shadow，不包含avatar）
- avatar.png（单独导出）
- titleText 运行时渲染
```

### 案例3：图标按钮
```
iconButton (sprite)
├─ iconUnion (BOOLEAN_OPERATION)     ← ❌ 剔除（装饰性，未设置res_mode）
│  ├─ circle (ELLIPSE)               ← ❌ 剔除（父节点被剔除）
│  └─ arrow (VECTOR)                 ← ❌ 剔除（父节点被剔除）
└─ ripple (ELLIPSE)                  ← ❌ 剔除（装饰性，未设置res_mode）

结果：
- iconButton.png（包含所有装饰性子节点）
```

### 案例4：复杂图标（需要单独导出部分）
```
complexIcon (sprite)
├─ base (VECTOR)                     ← ❌ 剔除（装饰性，未设置res_mode）
├─ highlight (VECTOR, sprite)        ← ✅ 保留（设置了sprite）
└─ badge                             ← ✅ 保留（包含后代countText）
   ├─ badgeBg (ELLIPSE)              ← ❌ 剔除（装饰性，未设置res_mode）
   └─ countText (TEXT)               ← ✅ 保留（TEXT类型）

结果：
- complexIcon.png（包含base，不包含highlight）
- highlight.png（单独导出）
- countText 运行时渲染
```

## 关键要点

1. **优先级顺序**：
   - 第一优先级：检查是否设置了 res_mode
   - 第二优先级：检查是否为装饰性节点类型
   - 第三优先级：检查是否为 TEXT 类型
   - 第四优先级：检查后代中是否有需要保留的节点

2. **装饰性节点的灵活性**：
   - 默认情况下，装饰性节点会被包含在父节点的图片中
   - 但如果显式设置了 res_mode，则会被单独导出
   - 这给予了用户完全的控制权

3. **层级结构的维护**：
   - 如果子孙节点需要保留，则中间的容器节点也会被保留
   - 这确保了正确的层级关系和布局

4. **递归处理**：
   - 每个保留的子节点会继续递归应用相同的规则
   - 直到处理完整个子树

## 相关函数

- `isAlwaysRemovableNodeType(nodeType string) bool` - 判断是否为装饰性节点类型
- `hasResNodeOrTextInDescendants(nodeID, ...) bool` - 检查后代中是否有需要保留的节点
- `processNodeResMode(...)` - 主要的节点处理逻辑
- `markNodeAndDescendantsForRemoval(...)` - 标记节点及其后代为删除

## 调试技巧

启用调试日志后，可以看到详细的处理过程：

```
保留子节点 4368:321142 (设置了res_mode=sprite)
删除装饰性节点 4368:321144 (type=BOOLEAN_OPERATION, 未设置res_mode, 父节点=4368:321140为图片资源)
保留子节点 4368:321141 (TEXT类型)
保留子节点 4368:321140 (后代中包含图片或文字节点)
删除装饰性子节点 4368:321145 (type=VECTOR, 父节点=4368:321140为图片资源)
```

## 版本历史

- v1.0.0 - 基础实现
- v1.1.0 - 添加装饰性节点强制剔除
- v1.2.0 - 支持装饰性节点显式设置 res_mode（当前版本）

