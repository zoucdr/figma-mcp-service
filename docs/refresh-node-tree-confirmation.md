# 节点树刷新按钮二次确认功能

## 功能说明

为节点树刷新按钮添加二次确认对话框，提醒用户该操作会调用 Figma API 并消耗 API 配额。

## 修改内容

### 1. 前端模板修改 (`web/templates/dashboard.html`)

**修改位置**：第 100 行，节点树刷新按钮

**修改前**：
```html
<el-tooltip content="刷新节点树" placement="top">
    <el-button icon="el-icon-refresh" size="mini" circle @click="loadProjectData(true)" :loading="loading"></el-button>
</el-tooltip>
```

**修改后**：
```html
<el-tooltip content="刷新节点树" placement="top">
    <el-button icon="el-icon-refresh" size="mini" circle @click="confirmRefreshNodeTree" :loading="loading"></el-button>
</el-tooltip>
```

**说明**：将直接调用 `loadProjectData(true)` 改为调用 `confirmRefreshNodeTree()` 确认函数。

---

### 2. JavaScript 逻辑修改 (`web/static/js/dashboard.js`)

**添加位置**：第 1922 行之前（`loadProjectData` 函数之前）

**新增函数**：
```javascript
// 确认刷新节点树（弹出二次确认框）
confirmRefreshNodeTree() {
    this.$confirm('刷新节点树将从 Figma API 获取最新数据，这会消耗您的 API 配额。是否继续？', '确认刷新', {
        confirmButtonText: '确定刷新',
        cancelButtonText: '取消',
        type: 'warning',
        distinguishCancelAndClose: true,
        cancelButtonClass: 'el-button--default',
        customClass: 'refresh-confirm-dialog'
    }).then(() => {
        // 用户确认，执行刷新
        this.loadProjectData(true);
    }).catch((action) => {
        if (action === 'cancel') {
            this.$message.info('已取消刷新');
        }
    });
},
```

**功能说明**：
- 弹出 Element UI 确认对话框
- 提示用户该操作会消耗 API 配额
- 确认后执行 `loadProjectData(true)` 刷新操作
- 取消时显示"已取消刷新"提示

---

### 3. CSS 样式优化 (`web/static/css/dashboard.css`)

**添加位置**：文件末尾

**新增样式**：
```css
/* 刷新节点树确认对话框样式 */
.refresh-confirm-dialog .el-message-box__message {
    font-size: 14px;
    line-height: 1.6;
}

.refresh-confirm-dialog .el-message-box__btns {
    display: flex;
    justify-content: flex-end;
    gap: 10px;
}

.refresh-confirm-dialog .el-button--primary {
    background-color: #E6A23C;
    border-color: #E6A23C;
}

.refresh-confirm-dialog .el-button--primary:hover {
    background-color: #EBB563;
    border-color: #EBB563;
}
```

**样式说明**：
- 优化对话框文本的字体大小和行高
- 调整按钮布局，使用 flexbox 对齐
- 将确认按钮改为警告色（橙色），强调该操作的重要性
- 添加悬停效果

---

## 用户体验流程

### 操作流程

1. **用户点击刷新按钮**
   - 位置：节点树面板右上角的刷新图标按钮
   
2. **弹出确认对话框**
   - 标题：⚠️ 确认刷新
   - 内容：刷新节点树将从 Figma API 获取最新数据，这会消耗您的 API 配额。是否继续？
   - 按钮：
     - ✅ 确定刷新（橙色警告按钮）
     - ❌ 取消（默认灰色按钮）

3. **用户选择**
   - **确定刷新**：执行刷新操作，调用 Figma API
   - **取消**：显示"已取消刷新"提示，不执行任何操作
   - **关闭对话框（X）**：静默取消，不显示提示

### 刷新成功后的提示

根据不同的刷新结果，显示相应的提示：

- ✅ **成功**：`节点树刷新成功`
- ⏰ **加入队列**：`请求已加入队列，预计 X 秒后处理`（保持当前数据）
- ⚠️ **使用缓存**：`无法连接 Figma API，已显示缓存数据（可能不是最新）`
- ❌ **失败**：`刷新失败: 错误信息`

---

## 技术细节

### Element UI Confirm 配置

```javascript
{
    confirmButtonText: '确定刷新',      // 确认按钮文本
    cancelButtonText: '取消',           // 取消按钮文本
    type: 'warning',                     // 对话框类型（警告）
    distinguishCancelAndClose: true,    // 区分取消和关闭操作
    cancelButtonClass: 'el-button--default', // 取消按钮样式类
    customClass: 'refresh-confirm-dialog'    // 自定义样式类
}
```

### Promise 处理

- `.then()` - 用户点击"确定刷新"
- `.catch(action)` - 用户点击"取消"或关闭对话框
  - `action === 'cancel'` - 用户点击"取消"按钮
  - `action === 'close'` - 用户点击"X"关闭对话框

---

## 为什么需要二次确认？

1. **API 配额保护**
   - Figma API 有调用频率限制
   - 避免用户频繁误点导致配额浪费

2. **提醒用户操作后果**
   - 明确告知该操作会调用外部 API
   - 让用户做出知情决策

3. **防止意外操作**
   - 刷新按钮位置显眼，容易误触
   - 二次确认可以避免误操作

4. **符合最佳实践**
   - 对于有成本、不可撤销的操作应有确认步骤
   - 提升应用的专业性和用户体验

---

## 相关文件

- `web/templates/dashboard.html` - 刷新按钮模板
- `web/static/js/dashboard.js` - 确认逻辑实现
- `web/static/css/dashboard.css` - 对话框样式

---

## 测试验证

### 测试步骤

1. **点击刷新按钮**
   - ✅ 应弹出确认对话框
   - ✅ 对话框显示警告图标和提示文本
   - ✅ 按钮样式正确（橙色确认按钮）

2. **点击"确定刷新"**
   - ✅ 关闭对话框
   - ✅ 执行刷新操作（显示加载动画）
   - ✅ 根据结果显示相应提示

3. **点击"取消"**
   - ✅ 关闭对话框
   - ✅ 显示"已取消刷新"提示
   - ✅ 不执行刷新操作

4. **点击对话框右上角"X"**
   - ✅ 关闭对话框
   - ✅ 不显示任何提示
   - ✅ 不执行刷新操作

5. **在加载中时点击刷新按钮**
   - ✅ 按钮处于禁用状态（`:loading="loading"`）
   - ✅ 无法重复点击

---

## 后续优化建议

1. **添加冷却时间显示**
   - 在确认对话框中显示当前的 API 冷却剩余时间
   - 如果在冷却期内，提示用户等待时间

2. **记住用户选择**
   - 添加"不再提示"复选框（可选）
   - 使用 localStorage 保存用户偏好

3. **快捷键支持**
   - 支持 Ctrl+R 或 F5 快捷键刷新
   - 快捷键操作同样需要确认

4. **批量操作警告**
   - 如果用户在短时间内多次刷新
   - 给出更强烈的警告提示

---

## 截图示意（文字描述）

### 确认对话框

```
┌─────────────────────────────────────────┐
│  ⚠️  确认刷新                            │
├─────────────────────────────────────────┤
│                                         │
│  刷新节点树将从 Figma API 获取最新数据，│
│  这会消耗您的 API 配额。是否继续？      │
│                                         │
│                         ┌──────┐ ┌─────┐│
│                         │ 取消 │ │确定 ││
│                         └──────┘ └─────┘│
└─────────────────────────────────────────┘
```

---

## 更新日志

**2024-11-27**
- ✅ 添加节点树刷新按钮二次确认功能
- ✅ 优化确认对话框样式
- ✅ 完善用户提示信息
- ✅ 编写完整文档

