# 🚀 Figma节点修饰AI提示词

## 🎯 任务目标

智能修饰Figma项目节点，确保符合预览效果及可交互性，实现：

* 命名规范化
* 层级优化
* 显示控制
* 布局配置
* 组件识别
* 资源指派
* 图片导出配置

---

## ⚙️ 执行流程

### ⚠️ 开始前必读

**🔴 三大核心规则（违反必重做）：**

1. **完整性规则**：只要节点未设置 `res_mode`，其子树所有节点都必须修饰！
   - 不允许遗漏任何节点
   - 不确定的节点必须预览后再处理
   - 必须递归处理到叶子节点

2. **INSTANCE规则**：必须处理INSTANCE的所有子节点
   - 使用完整ID格式：`I父节点ID;子节点ID`
   - 不要跳过INSTANCE的子节点
   - 递归处理INSTANCE的整个子树

3. **预览验证规则**：对不确定的节点必须预览验证
   - 特别是含TEXT的同级节点
   - 看到预览后再决定配置
   - 禁止盲目猜测

---

### **步骤1：获取整体原型图**

```python
get_preview(project_id, scale=0.1)
```

**目的**：了解整体布局结构、交互控件位置、静态图片与文字分布、视觉层次。

---

### **步骤2：获取节点树（必需）**

```python
get_node_tree(project_id, node_id?)
```

**分析内容：**

* 节点基础信息：ID、名称、类型
* 层级关系：父子、兄弟、嵌套深度
* 已有配置：`extra_img`、`res_mode`、`components`、`ignore`等
* 节点特征：是否含 `TEXT` 子节点、尺寸

---

### **步骤3：分组逐步分析（拆解分析）**

**分析内容：**

* 提取所有GROUP类型的节点
* 所有GROUP组，逐步分析
* 整理体合并分析

---

### **步骤3.5：不确定节点预览验证（必需）**

**对于无法确定功能的节点，特别是含TEXT的同级节点，必须进行预览验证：**

```python
get_preview(project_id, node_id=uncertain_node_id, scale=0.3)
```

**适用场景**：
1. 节点名称不明确，无法判断用途
2. 同级存在TEXT节点，但不确定是否为可交互组件
3. 节点结构复杂，无法从树结构判断整合策略
4. 需要验证元素是否为独立交互单元

**处理原则**：
- 看到预览后再决定是否整合、设置组件类型
- 避免盲目猜测导致错误配置
- 优先查看包含TEXT的容器节点预览

---

### **步骤4：容器整合分析（高优先级）**

**在处理子节点之后，必须优先判断容器是否可整合为资源单元。**

#### ✅ 触发条件（需全部满足）

1. 子树中设置过 `extra_img:true` 或 `res_mode`
2. 其他子节点均为装饰性元素（`BOOLEAN_OPERATION` / `VECTOR` / `ELLIPSE` / `RECTANGLE`）
3. 子树中无 `TEXT` 节点
4. 节点语义完整（如图标组、按钮背景、头像框等）
5. 子节点数量 ≤ 10
6. 无上下文说明需要注意拆解图

#### 🧩 处理逻辑

```json
[
  {"id": "container", "rename": "icon_group", "res_mode": "sprite"},
  {"id": "child_img", "res_mode": ""}
]
```

* 将容器节点标记为资源（sprite/texture/slice）。
* 清除子节点的 `res_mode`。
* 保留子节点的其他属性（如组件、约束）。

#### ⚡ 优势

* 减少资源碎片、优化性能、保持语义完整、简化管理。

---

### **步骤5：节点修饰（必需）**

```python
set_nodes_modify(project_id, modifys)
```

修饰内容包括：
`rename`、`components`、`res_mode`、`horizontal`、`vertical`、`img_name`、`img_id`、`imageDownloadType`、`ignore`、`parent_id`。

---

### **步骤6：完整性验证（关键）**

#### 🔍 修饰完整性规则

**核心原则**：只要节点未被设置 `res_mode`，其子树所有节点都必须修饰！

**检查清单**：
1. **遍历整个节点树**：确保每个节点都被处理（不处理给出原因）
2. **子树完整性**：
   - 如果父节点设置了 `res_mode` → 子节点可以不处理（整体导出）
   - 如果父节点未设置 `res_mode` → **必须处理所有子节点**
3. **INSTANCE节点的子节点**：必须处理，使用完整ID格式
4. **不确定的节点**：可单独预览一下这个节点，不要遗漏

#### ❌ 常见遗漏场景

| 场景 | 错误做法 | 正确做法 |
| --- | ------- | ------- |
| INSTANCE的子节点 | 只处理INSTANCE本身 | 必须处理所有子节点 |
| 深层嵌套节点 | 只处理前两层 | 递归处理到叶子节点 |
| 装饰性图形 | 不处理，认为不重要 | 预览确认后处理 |
| GROUP内的节点 | 只处理GROUP本身 | 处理GROUP及其所有子节点 |

#### ✅ 修饰覆盖检查

**验证方法**：
```
1. 统计节点树总节点数（N）
   - 递归计数所有节点（包括INSTANCE的子节点）
   - 使用深度优先遍历确保不遗漏
   
2. 统计修饰配置数量（M）
   - 计算modifys数组的长度
   
3. 统计设置了res_mode的父节点子树节点数（K）
   - 如果父节点设置了res_mode，其子树所有节点计入K
   - 递归统计子树大小
   
4. 检查：M + K ≥ N（或接近N）
   - 如果M + K < N，说明有节点遗漏
   - 找出遗漏的节点ID并补充处理
```

**节点统计工具函数示例**：
```javascript
function countNodes(node) {
  let count = 1; // 当前节点
  if (node.children) {
    for (let child of node.children) {
      count += countNodes(child); // 递归统计子节点
    }
  }
  return count;
}

function countCoveredByResMode(node, modifys) {
  const modify = modifys.find(m => m.id === node.id);
  if (modify && modify.res_mode) {
    // 如果设置了res_mode，统计整个子树
    return countNodes(node) - 1; // 减1是因为父节点本身要单独计算
  }
  let count = 0;
  if (node.children) {
    for (let child of node.children) {
      count += countCoveredByResMode(child, modifys);
    }
  }
  return count;
}
```

**示例**：
```json
// 节点树
{
  "id": "A",
  "type": "FRAME",
  "children": [
    {"id": "B", "type": "GROUP", "children": [
      {"id": "C", "type": "VECTOR"},
      {"id": "D", "type": "RECTANGLE"}
    ]},
    {"id": "E", "type": "TEXT"}
  ]
}

// 正确修饰（5个节点，5个配置）
{"id": "A", "rename": "container"},
{"id": "B", "rename": "iconGroup", "res_mode": "sprite"},  // ← 设置了res_mode
// C和D不需要处理（父节点B已设置res_mode，会整体导出）
{"id": "E", "rename": "labelText"}

// 或者（3个节点，3个配置）
{"id": "A", "rename": "container"},
{"id": "B", "rename": "iconGroup"},  // ← 未设置res_mode
{"id": "C", "res_mode": "sprite"},  // ← 必须处理子节点C
{"id": "D", "res_mode": "sprite"},  // ← 必须处理子节点D
{"id": "E", "rename": "labelText"}

// ❌ 错误修饰（遗漏C和D）
{"id": "A", "rename": "container"},
{"id": "B", "rename": "iconGroup"},  // 未设置res_mode
// C和D被遗漏了！
{"id": "E", "rename": "labelText"}
```

---

## 📋 Figma节点类型说明

### 🎨 基础节点类型

| 节点类型 | 说明 | 处理策略 | 示例场景 |
| ------- | ---- | ------- | ------- |
| `FRAME` | 容器框架，最常用的布局容器 | **按容器组处理**：可设置约束、组件、资源模式 | 面板、卡片、按钮容器 |
| `GROUP` | 分组容器，纯逻辑分组 | **按容器组处理**：统一容器处理策略 | 图标组、按钮组 |
| `INSTANCE` | 组件实例，引用组件的具体使用 | **按容器组处理**：与FRAME/GROUP相同策略 | 具体的按钮实例 |
| `COMPONENT` | 组件定义，可复用的设计元素 | 优先保持原有结构，谨慎修改 | 按钮组件、图标组件 |
| `TEXT` | 文本节点 | 不设置资源模式，可设置组件和约束 | 标题、标签、按钮文字 |

### 📦 容器组统一处理策略

**适用节点类型**：`FRAME`、`GROUP`、`INSTANCE`

**统一处理原则**：
- 可设置布局约束（`horizontal`、`vertical`）
- 可识别为UI组件（`components`）
- 可设置资源模式（`res_mode`）
- 可进行容器整合分析
- 统一的命名规范要求

### ⚠️ INSTANCE节点特殊处理规则

**INSTANCE节点**是组件实例，其子节点ID格式为 `I父节点ID;子节点ID`

**关键规则**：
1. **必须处理INSTANCE的所有子节点**：INSTANCE类型节点的子节点同样需要修饰
2. **子节点ID格式**：使用完整的子节点ID（包含`I`前缀和`;`分隔符）
3. **递归处理**：INSTANCE的子节点可能还有子节点，需要递归处理整个子树
4. **不要忽略**：不要因为是INSTANCE的子节点就跳过处理，不认识就预览当前节点图分析

**示例**：
```json
{
  "id": "8:13775",
  "name": "equipmentSlot1",
  "type": "INSTANCE",
  "children": [
    {
      "id": "I8:13775;1448:6276",  // ← 注意ID格式
      "name": "Rectangle 1573",
      "type": "VECTOR"
    }
  ]
}
```

处理时需要修饰子节点：
```json
{"id": "8:13775", "rename": "equipmentSlot1", "components": ["Button"]}
```

### 🎯 图形节点类型

| 节点类型 | 说明 | 处理策略 | 导出建议 |
| ------- | ---- | ------- | ------- |
| `RECTANGLE` | 矩形图形 | 装饰性→整合到父级，独立性→设置资源模式 | 背景、边框 |
| `ELLIPSE` | 椭圆/圆形图形 | 同矩形处理策略 | 头像框、装饰圆点 |
| `POLYGON` | 多边形图形 | 通常为装饰性，整合到父级 | 箭头、星形图标 |
| `STAR` | 星形图形 | 装饰性图形，整合处理 | 评分星星、装饰 |
| `VECTOR` | 矢量路径 | 复杂图形可独立导出，简单装饰整合 | 图标、复杂图形 |
| `LINE` | 线条 | 通常为装饰性，整合到父级 | 分割线、边框线 |

### 🔧 特殊节点类型

| 节点类型 | 说明 | 处理策略 | 注意事项 |
| ------- | ---- | ------- | ------- |
| `SLICE` | 切片导出区域 | 保持原有配置，不修改 | Figma导出标记 |
| `SECTION` | 分区标记 | 组织性节点，通常不处理 | 设计稿组织结构 |

### 🖼️ 媒体节点类型

| 节点类型 | 说明 | 处理策略 | 导出配置 |
| ------- | ---- | ------- | ------- |
| `IMAGE` | 图片节点 | 设置为`texture`或`sprite` | 根据尺寸选择格式 |
| `VIDEO` | 视频节点 | 通常不处理或标记为占位 | 游戏引擎不直接支持 |

### 🎛️ 节点处理优先级

#### 1️⃣ 可跳过的节点
- `SECTION` - 分区节点  
- 名称以`#`开头的节点（注释节点）

#### 2️⃣ 容器组节点（统一处理）
**节点类型**：`FRAME`、`GROUP`、`INSTANCE`

**处理策略**：
- 容器整合分析：评估是否可整合为单一资源
- 组件识别：根据命名和结构识别UI组件
- 布局约束：设置适当的水平和垂直约束
- 资源模式：根据内容决定是否设置资源模式

#### 3️⃣ 独立资源节点
- 单独的`IMAGE`节点
- 复杂的`VECTOR`图形
- 带有`extra_img: true`标记的节点

#### 4️⃣ 文本组件节点
- `TEXT`节点（UI文本组件）
- 可设置组件类型和约束
- 不设置资源模式

### 🔍 节点分析流程

```
1. 检查节点类型 → 确定基础处理策略
   ├─ 容器组（FRAME/GROUP/INSTANCE）→ 统一容器处理流程
   ├─ 图形节点 → 装饰性分析
   ├─ 文本节点 → 组件识别
   └─ 特殊节点 → 保持原状

2. 分析子节点结构 → 判断是否可整合
3. 检查命名模式 → 识别UI组件类型  
4. 评估尺寸大小 → 选择资源模式
5. 确定布局位置 → 配置约束参数
```

### 📦 容器组处理流程

**适用于**：`FRAME`、`GROUP`、`INSTANCE`

```
1. 容器分析
   ├─ 检查子节点类型和数量
   ├─ 评估是否包含TEXT节点
   └─ 判断是否为装饰性容器

2. 整合判断
   ├─ 符合整合条件 → 设置容器资源模式，清除子节点资源模式
   ├─ 不符合整合 → 保持子节点独立处理
   └─ 混合情况 → 部分整合

3. 组件识别
   ├─ 根据命名模式识别UI组件类型
   ├─ 分析交互特征（按钮、输入框等）
   └─ 设置对应的components配置

4. 约束配置
   ├─ 分析在父容器中的位置
   ├─ 设置水平约束（LEFT/CENTER/RIGHT/LEFT_RIGHT/SCALE）
   └─ 设置垂直约束（TOP/CENTER/BOTTOM/TOP_BOTTOM/SCALE）
```

---

## 🧠 决策逻辑与规则

### 1️⃣ 命名规范

* 允许：字母、数字（不能以数字开头）
* 禁止：中文、空格、连字符、特殊符号
* 标准：`小写开头的驼峰命名法`

| 后缀                  | UGUI组件     | 场景    |
| ------------------- | ---------- | ----- |
| `Btn`              | Button     | 可点击按钮 |
| `Tog`              | Toggle     | 开关    |
| `Img`              | Image      | 静态图片  |
| `Raw`              | RawImage   | 动态纹理  |
| `Bg`               | Texture    | 背景图   |
| `Text`              | Text       | 文本    |
| `Input`            | InputField | 输入框   |
| `Slider`           | Slider     | 滑动条   |
| `Scroll`           | ScrollRect | 滚动视图  |
| `Mask`             | Mask       | 裁剪容器  |
| `Panel` / `Group` | -          | 分组容器  |

> ⚠️ 同层级名称必须唯一，重名加序号或位置后缀。

---

### 2️⃣ 组件识别

| 组件           | 识别条件                 | 示例            |
| ------------ | -------------------- | ------------- |
| `Button`     | 名称含"button/btn/按钮"   | back_btn      |
| `Toggle`     | 名称含"开关/toggle/check" | sound_tog     |
| `Mask`       | 用于头像裁剪等              | avatar_mask   |
| `InputField` | 名称含"输入/input"        | name_input    |
| `ScrollRect` | 名称含"scroll/list"     | list_scroll   |
| `Slider`     | 名称含"滑动/slider"       | volume_slider |

---

### 3️⃣ 资源模式（res_mode）

#### 必要条件

* 节点类型 ≠ TEXT
* 子树不包含 TEXT
* 叶子节点未设置 res_mode

#### 充分条件

* 有 `extra_img:true` 标记
* 节点自身为图片资源
* 子树无文本，且无需拆图
* 包含 `BOOLEAN_OPERATION` 子节点

#### 模式选择

| 模式        | 场景    | 尺寸建议     |
| --------- | ----- | -------- |
| `sprite`  | 小图标   | <512×512 |
| `texture` | 大背景   | >512×512 |
| `slice`   | 可拉伸面板 | 任意       |

#### 判定流程

1. **节点类型检查**：
   - `TEXT` → 跳过资源模式设置
   - `SECTION`/`SLICE` → 保持原状
   
2. **资源模式判定**：
   - 若 `extra_img:true` → 优先考虑整合
   - 若子树含 TEXT → 跳过
   - 若叶子节点已设置 → 跳过
   - 若容器符合整合条件 → 整合处理
   
3. **类型特定处理**：
   - `IMAGE` 节点 → 根据尺寸选择 `texture`/`sprite`
   - **容器组**（`FRAME`/`GROUP`/`INSTANCE`）→ 统一容器整合分析
   - `VECTOR`/`RECTANGLE`/`ELLIPSE` → 评估复杂度决定独立或整合
   
4. **语义判断**：
   - 名称含 `border/frame/dialog/panel` → `slice`
   - 名称含 `bg/background` → `texture`
   - 名称含 `icon/btn` → `sprite`
   
5. **尺寸判断**：
   - 尺寸 > 512×512 → `texture`
   - 其他默认 → `sprite`

---

### 4️⃣ 布局约束配置

#### 水平约束（horizontal）

| 约束类型        | 值            | 场景           |
| ------------- | ------------ | ------------ |
| 左对齐         | `LEFT`       | 左侧固定元素     |
| 右对齐         | `RIGHT`      | 右侧固定元素     |
| 水平居中        | `CENTER`     | 居中元素       |
| 左右拉伸        | `LEFT_RIGHT` | 顶部/底部栏     |
| 缩放          | `SCALE`      | 等比例缩放元素    |

#### 垂直约束（vertical）

| 约束类型        | 值            | 场景           |
| ------------- | ------------ | ------------ |
| 顶部对齐        | `TOP`        | 顶部固定元素     |
| 底部对齐        | `BOTTOM`     | 底部固定元素     |
| 垂直居中        | `CENTER`     | 居中元素       |
| 上下拉伸        | `TOP_BOTTOM` | 左右侧边栏      |
| 缩放          | `SCALE`      | 等比例缩放元素    |

#### 常见布局组合

* **全屏背景**：`horizontal: "LEFT_RIGHT"`, `vertical: "TOP_BOTTOM"`
* **顶部导航栏**：`horizontal: "LEFT_RIGHT"`, `vertical: "TOP"`
* **底部按钮栏**：`horizontal: "LEFT_RIGHT"`, `vertical: "BOTTOM"`
* **居中弹窗**：`horizontal: "CENTER"`, `vertical: "CENTER"`
* **左上角按钮**：`horizontal: "LEFT"`, `vertical: "TOP"`
* **右上角按钮**：`horizontal: "RIGHT"`, `vertical: "TOP"`

---

### 5️⃣ 图片导出

| 属性                  | 说明                  |
| ------------------- | ------------------- |
| `img_ext` | 有透明度→PNG，无透明度大图→JPG |
| `img_name`          | 自定义导出名              |
| `img_id`            | 关联相同的图片的节点ID          |

---

### 6️⃣ 其他配置

* `ignore`：移除不导入节点
* `parent_id`：移动层级位置

---

## 🧩 决策优先级

1. **容器整合** > 子节点分散
2. **语义完整性** > 技术细节
3. **预览验证** > 推测判断
4. **最小化修改** > 冗余配置

---

## 🧾 输出格式

```json
{
  "project_id": 507,
  "modifys": [
    {"id": "1:101", "rename": "main_bg", "res_mode": "texture", "imageDownloadType": "jpg", "horizontal": "LEFT_RIGHT", "vertical": "TOP_BOTTOM"},
    {"id": "1:102", "rename": "top_bar", "horizontal": "LEFT_RIGHT", "vertical": "TOP"},
    {"id": "1:103", "rename": "back_btn", "components": ["Button"], "horizontal": "LEFT", "vertical": "TOP"},
    {"id": "1:107", "rename": "close_btn", "components": ["Button"], "horizontal": "RIGHT", "vertical": "TOP"},
    {"id": "1:110", "rename": "bottom_btn_group", "horizontal": "LEFT_RIGHT", "vertical": "BOTTOM"},
    {"id": "1:111", "rename": "confirm_btn", "components": ["Button"], "horizontal": "CENTER", "vertical": "CENTER"}
  ]
}
```

**附加说明**：

* 修改节点数、重命名数量、组件配置数量、资源模式数量。
* 说明关键修改及对应原因。
* **必须说明修饰覆盖率**：如"共处理 X 个节点，节点树总数 Y，覆盖率 Z%"
* **必须说明INSTANCE子节点处理情况**：如"处理了 N 个INSTANCE的子节点"

---

## ✅ 检查清单

### 🎯 节点类型处理
* `TEXT` 节点未设置资源模式
* `IMAGE` 节点已配置适当的资源模式
* **容器组节点**（`FRAME`/`GROUP`/`INSTANCE`）统一按容器策略处理
* `COMPONENT` 节点结构未被破坏
* **INSTANCE的子节点已处理**（使用完整ID格式）

### 📝 命名与组件
* 命名符合规则且唯一
* 组件识别正确
* 节点类型与组件类型匹配

### 🖼️ 资源配置
* 图片资源配置无冲突
* 容器整合节点生效
* 无冗余或重复res_mode
* 资源模式与节点类型匹配

### 📐 布局约束
* 水平和垂直约束与布局匹配
* 约束配置符合节点类型特征
* 容器节点约束合理

### ⚠️ 完整性检查（最重要）
* **所有节点都已处理**（修饰或备注）
* **未设置res_mode的节点，其子树已完整处理**
* **INSTANCE的所有子节点已修饰**
* **深层嵌套节点未遗漏**
* 修饰数量与节点树规模匹配
* 对不确定的节点已进行预览验证

---

## 🌟 核心原则

> **分析节点类型** → 理解设计意图 → 遵循命名规范 → 识别组件 → 整合资源 → 保持层级语义 → 配置布局约束 → **验证完整性** → 输出简洁一致。

### 🎯 关键决策点

1. **节点类型优先**：根据Figma节点类型确定基础处理策略
2. **结构完整性**：保持组件和实例的原有结构
3. **语义一致性**：确保修改后的配置符合节点的设计意图
4. **性能优化**：通过合理的资源整合减少导出碎片
5. **布局适配**：配置合适的约束确保多分辨率适配
6. **🔴 修饰完整性**：**未设置res_mode的节点，必须处理其所有子节点**
7. **INSTANCE处理**：必须递归处理INSTANCE的所有子节点
8. **预览验证**：对不确定的节点（特别是含TEXT同级节点）进行预览验证

### ⚠️ 三大禁忌

1. **禁止遗漏节点**：每个节点都必须被处理（未处理给出原因）
2. **禁止跳过INSTANCE子节点**：INSTANCE的子节点必须使用完整ID修饰
3. **禁止盲目猜测**：不确定的节点必须预览后再处理

---

## 🔄 完整执行流程总结

```
1. 获取原型图 → 了解整体布局
2. 获取节点树 → 分析节点结构
3. 逐组分析 → 理解功能分区
4. 预览验证 → 确认不确定节点
5. 容器整合 → 优化资源结构
6. 节点修饰 → 应用配置规则
7. ✅ 完整性检查 → 确保无遗漏
   ├─ 检查所有节点是否处理
   ├─ 验证INSTANCE子节点
   ├─ 确认res_mode逻辑正确
   └─ 统计修饰覆盖率
```

---
