# MCP 测试中心前端改进

## 改进概述

对 `web/templates/mcp_center.html` 进行了两项重要改进，以更好地支持 `get_preview` 工具的 `ignore_texts` 参数和其他基础工具的调用。

## 改进 1: Boolean 参数处理优化

### 问题
之前的 `callTool` 函数在第 663 行有这样的逻辑：
```javascript
if (value === '') return; // 跳过空值
```

这会导致所有空值参数被跳过，但对于 boolean 类型的下拉选择框，值永远不会为空（总是 `"true"` 或 `"false"`）。然而，这个逻辑可能影响未来的扩展。

### 解决方案

修改参数处理逻辑，明确区分 boolean 类型和其他类型：

```javascript
// 对于 boolean 类型，即使值为空也要处理（转换为 false）
// 对于其他类型，空值跳过
if (value === '' && type !== 'boolean') return;

if (type === 'number' || type === 'integer') {
    value = Number(value);
} else if (type === 'boolean') {
    // boolean 类型：'true' 转为 true，其他都转为 false
    value = value === 'true';
} else if (type === 'object' || type === 'array') {
    // ...
}
```

### 优势
- ✅ 确保 boolean 参数总是被正确处理
- ✅ `ignore_texts=false` 会被正确发送（而不是被跳过）
- ✅ 更清晰的类型处理逻辑

## 改进 2: 基础工具无需 Figma 连接

### 问题
之前的 `sendMcpCall` 函数会检查 Figma 连接状态：
```javascript
if (!isFigmaConnected) {
    addMessage('❌ Figma 未连接', 'error');
    alert('请先连接 Figma 插件');
    return;
}
```

这会阻止所有工具的调用，包括 `get_preview` 这样的基础工具。但实际上：
- **基础工具**（如 `get_preview`）只从缓存读取，不需要 Figma 插件连接
- **插件工具**（如 `export_node_as_image`）才需要 WebSocket 连接

### 解决方案

1. **定义基础工具列表**：
```javascript
let basicTools = [
    'get_preview', 
    'get_node_tree', 
    'set_nodes_modify', 
    'get_modify_prompt', 
    'clear_nodes_modifys', 
    'get_ref_nodes'
];
```

2. **条件检查连接状态**：
```javascript
// 检查是否为基础工具（基础工具不需要 Figma 插件连接）
const isBasicTool = basicTools.includes(command);

if (!isFigmaConnected && !isBasicTool) {
    addMessage('❌ Figma 未连接', 'error');
    alert('此工具需要 Figma 插件连接，请先在 Figma 中打开插件');
    return;
}
```

### 优势
- ✅ 基础工具可以在未连接 Figma 时使用
- ✅ 更好的用户体验（不需要总是连接插件）
- ✅ 更清晰的错误提示（区分基础工具和插件工具）
- ✅ 支持纯后端缓存查询场景

## 使用场景

### 场景 1: 使用 get_preview 查询缓存（无需 Figma 连接）

1. 用户访问 MCP 测试中心
2. **不需要**打开 Figma 插件或连接
3. 直接填写 `get_preview` 参数：
   - project_id: `1`
   - ignore_texts: `True`
4. 点击"调用"
5. 如果缓存存在，立即返回图片

**之前**: ❌ 提示"请先连接 Figma 插件"  
**现在**: ✅ 直接查询缓存并返回结果

### 场景 2: 使用插件工具（需要 Figma 连接）

1. 用户访问 MCP 测试中心
2. 在 Figma 中打开插件并连接
3. 使用 `export_node_as_image` 等工具
4. 实时渲染并返回图片

**之前**: ✅ 正常工作  
**现在**: ✅ 正常工作（保持一致）

### 场景 3: ignore_texts 参数测试

```javascript
// 测试 1: ignore_texts = false（显示文本）
{
  "command": "get_preview",
  "params": {
    "project_id": 1,
    "ignore_texts": false  // ✅ 正确发送 false 值
  }
}

// 测试 2: ignore_texts = true（隐藏文本）
{
  "command": "get_preview",
  "params": {
    "project_id": 1,
    "ignore_texts": true  // ✅ 正确发送 true 值
  }
}
```

## 技术细节

### Boolean 参数的完整处理流程

1. **UI 渲染**（第 616-621 行）：
```javascript
else if (prop.type === 'boolean') {
    const defaultValue = prop.default !== undefined ? prop.default : false;
    inputsHtml += `<select id="${inputId}" class="tool-input" data-type="boolean" data-prop="${key}">
        <option value="false" ${!defaultValue ? 'selected' : ''}>False</option>
        <option value="true" ${defaultValue ? 'selected' : ''}>True</option>
    </select>`;
}
```

2. **值提取**（第 658-678 行）：
```javascript
inputs.forEach(input => {
    const key = input.getAttribute('data-prop');
    const type = input.getAttribute('data-type');
    let value = input.value; // 获取 "true" 或 "false" 字符串
    
    // boolean 类型不跳过
    if (value === '' && type !== 'boolean') return;
    
    if (type === 'boolean') {
        value = value === 'true'; // 转换为真正的 boolean
    }
    
    params[key] = value;
});
```

3. **API 调用**（第 700-709 行）：
```javascript
const response = await fetch('/mcp-api/tool-call', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
        command: command,
        params: params  // ignore_texts 正确为 boolean 类型
    })
});
```

### 基础工具列表

当前定义的基础工具（不需要 Figma 插件连接）：
- ✅ `get_preview` - 从缓存获取预览图
- ✅ `get_node_tree` - 获取节点树配置
- ✅ `set_nodes_modify` - 批量设置节点配置
- ✅ `get_modify_prompt` - 获取配置提示词
- ✅ `clear_nodes_modifys` - 清理节点修改
- ✅ `get_ref_nodes` - 获取依赖节点列表

需要 Figma 插件连接的工具：
- ⚠️ `read_my_design` - 从 Figma 读取设计
- ⚠️ `export_node_as_image` - 导出单个节点图片
- ⚠️ `export_nodes_as_images` - 批量导出节点图片

## 测试建议

### 测试 1: Boolean 参数处理

1. 访问 MCP 测试中心
2. 找到 `get_preview` 工具
3. 设置 `ignore_texts` 为 `False`
4. 查看请求日志，确认发送了 `"ignore_texts": false`
5. 设置 `ignore_texts` 为 `True`
6. 查看请求日志，确认发送了 `"ignore_texts": true`

### 测试 2: 基础工具无需连接

1. **不要**打开 Figma 或连接插件
2. 确认状态显示"Figma 未连接"
3. 使用 `get_preview` 工具（假设有缓存）
4. 应该能正常调用，不会提示需要连接
5. 尝试使用 `export_node_as_image`
6. 应该提示"此工具需要 Figma 插件连接"

### 测试 3: 完整流程

1. 先在 Dashboard 渲染节点（生成缓存）
2. 访问 MCP 测试中心（不连接 Figma）
3. 调用 `get_preview`，`ignore_texts=false`
4. 应该返回带文本的缓存图片
5. 调用 `get_preview`，`ignore_texts=true`
6. 应该返回不带文本的缓存图片（如果存在）

## 兼容性

- ✅ 向后兼容：不影响现有的插件工具
- ✅ 渐进增强：基础工具获得更好的体验
- ✅ 清晰提示：用户能清楚知道哪些工具需要连接

## 相关文件

- `web/templates/mcp_center.html` - MCP 测试中心前端
- `internal/controllers/mcp_tools.go` - 工具定义
- `internal/controllers/mcp.go` - 工具处理逻辑

## 完成日期

2025-12-01

