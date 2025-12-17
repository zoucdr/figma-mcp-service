# Bug 修复 - 分组渲染支持多个 File Key 🐛

## 📋 问题描述

**问题**: `renderGroupNodes` 方法假设一个分组内所有项目都有相同的 `file_key`，但实际上同一个 `group_name` 分组内可能包含来自不同 Figma 文件的项目。

**影响**: 
- ❌ 只会使用第一个项目的 `file_key`
- ❌ 其他 `file_key` 的节点会被错误地关联到第一个文件
- ❌ Figma API 调用会失败（节点ID不属于该文件）

**复现场景**:
```
分组名称: "设计系统"
  - 项目A: file_key = "abc123" (按钮组件)
  - 项目B: file_key = "def456" (图标组件)
  - 项目C: file_key = "abc123" (卡片组件)

点击"批量渲染"
  ↓
旧逻辑: 使用 file_key = "abc123"
        收集所有节点 [节点A1, 节点B1, 节点C1]
        ↓
错误: 节点B1不属于文件abc123 ❌
```

---

## ✅ 修复方案

### 核心思路

1. **按 file_key 二次分组**: 将分组内的项目按 `file_key` 重新分组
2. **智能处理**:
   - 单个 `file_key`: 直接显示渲染对话框
   - 多个 `file_key`: 提示用户并为每个文件分别创建队列

### 修复后的流程

```
分组名称: "设计系统"
  - 项目A: file_key = "abc123" (按钮组件)
  - 项目B: file_key = "def456" (图标组件)  
  - 项目C: file_key = "abc123" (卡片组件)

点击"批量渲染"
  ↓
新逻辑: 按 file_key 分组
  - abc123: [节点A1, 节点C1]
  - def456: [节点B1]
  ↓
检测到 2 个不同的文件
  ↓
显示确认对话框:
  "该分组包含 2 个不同的 Figma 文件：
   - abc123...（2个项目，2个节点）
   - def456...（1个项目，1个节点）
   
   将为每个文件分别创建渲染队列。"
  ↓
用户点击"确定"
  ↓
为每个 file_key 分别创建渲染队列
  - 队列1: file_key=abc123, nodes=[A1, C1] ✅
  - 队列2: file_key=def456, nodes=[B1] ✅
  ↓
显示成功消息:
  "已为 2 个文件创建渲染队列，共 3 个节点"
```

---

## 💻 代码实现

### 修复前的代码

```javascript
async renderGroupNodes(group) {
    if (!group.fileKey) {  // ❌ 只使用第一个项目的 fileKey
        this.$message.warning('该分组缺少文件Key');
        return;
    }
    
    // 收集所有节点
    const allNodeIds = new Set();
    for (const project of group.projects) {
        const response = await axios.get(`/api/figma/project/${project.id}/nodes`);
        if (response.data.code === 0) {
            const nodes = response.data.data.nodes || [];
            nodes.forEach(node => {
                if (node.figma_node_id) {
                    allNodeIds.add(node.figma_node_id);  // ❌ 所有节点混在一起
                }
            });
        }
    }
    
    // ❌ 使用第一个 fileKey 渲染所有节点
    this.showRenderDialog(group.fileKey, Array.from(allNodeIds));
}
```

### 修复后的代码

```javascript
async renderGroupNodes(group) {
    const loading = this.$loading({
        lock: true,
        text: '正在收集节点信息...',
        spinner: 'el-icon-loading'
    });
    
    try {
        // ✅ 按 file_key 对项目进行分组
        const projectsByFileKey = {};
        
        for (const project of group.projects) {
            const fileKey = project.file_key;
            if (!fileKey) continue;
            
            if (!projectsByFileKey[fileKey]) {
                projectsByFileKey[fileKey] = {
                    fileKey: fileKey,
                    nodeIds: new Set(),
                    projectCount: 0
                };
            }
            
            // 获取项目节点
            const response = await axios.get(`/api/figma/project/${project.id}/nodes`);
            
            if (response.data.code === 0) {
                const nodes = response.data.data.nodes || [];
                nodes.forEach(node => {
                    if (node.figma_node_id) {
                        // ✅ 节点按 file_key 分组存储
                        projectsByFileKey[fileKey].nodeIds.add(node.figma_node_id);
                    }
                });
                projectsByFileKey[fileKey].projectCount++;
            }
        }
        
        loading.close();
        
        const fileKeys = Object.keys(projectsByFileKey);
        
        if (fileKeys.length === 0) {
            this.$message.warning('该分组没有可渲染的节点');
            return;
        }
        
        // ✅ 如果只有一个 file_key，直接渲染
        if (fileKeys.length === 1) {
            const fileKey = fileKeys[0];
            const data = projectsByFileKey[fileKey];
            
            if (data.nodeIds.size === 0) {
                this.$message.warning('该分组没有可渲染的节点');
                return;
            }
            
            this.showRenderDialog(fileKey, Array.from(data.nodeIds));
        } else {
            // ✅ 多个 file_key，询问用户并分别处理
            const fileKeysInfo = fileKeys.map(fk => {
                const data = projectsByFileKey[fk];
                return `- ${fk.substring(0, 8)}...（${data.projectCount}个项目，${data.nodeIds.size}个节点）`;
            }).join('\n');
            
            this.$confirm(
                `该分组包含 ${fileKeys.length} 个不同的 Figma 文件：\n\n${fileKeysInfo}\n\n将为每个文件分别创建渲染队列。`,
                '渲染多个文件',
                {
                    confirmButtonText: '确定',
                    cancelButtonText: '取消',
                    type: 'warning'
                }
            ).then(async () => {
                // ✅ 为每个 file_key 分别创建渲染队列
                let successCount = 0;
                let totalNodes = 0;
                
                for (const fileKey of fileKeys) {
                    const data = projectsByFileKey[fileKey];
                    const nodeIds = Array.from(data.nodeIds);
                    
                    if (nodeIds.length === 0) continue;
                    
                    try {
                        const response = await axios.post('/api/figma/manual/render', {
                            file_key: fileKey,
                            node_ids: nodeIds,
                            format: 'png',
                            scale: 2.0
                        });
                        
                        if (response.data.code === 0) {
                            successCount++;
                            totalNodes += nodeIds.length;
                        }
                    } catch (error) {
                        console.error(`创建渲染队列失败 (${fileKey}):`, error);
                    }
                }
                
                if (successCount > 0) {
                    this.$message.success(`已为 ${successCount} 个文件创建渲染队列，共 ${totalNodes} 个节点`);
                } else {
                    this.$message.error('创建渲染队列失败');
                }
            }).catch(() => {
                // 用户取消
            });
        }
    } catch (error) {
        loading.close();
        console.error('收集节点失败:', error);
        this.$message.error('收集节点信息失败，请稍后重试');
    }
}
```

---

## 🎯 功能特性

### 1. 智能检测

自动检测分组内有多少个不同的 `file_key`：

```javascript
const projectsByFileKey = {};

for (const project of group.projects) {
    const fileKey = project.file_key;
    if (!projectsByFileKey[fileKey]) {
        projectsByFileKey[fileKey] = {
            fileKey: fileKey,
            nodeIds: new Set(),
            projectCount: 0
        };
    }
    // 收集节点...
}
```

### 2. 单文件处理

如果分组内所有项目都来自同一个文件：

```javascript
if (fileKeys.length === 1) {
    const fileKey = fileKeys[0];
    const data = projectsByFileKey[fileKey];
    
    // 显示渲染对话框（用户可以调整参数）
    this.showRenderDialog(fileKey, Array.from(data.nodeIds));
}
```

**用户体验**: 
- ✅ 与之前一致，显示渲染对话框
- ✅ 可以选择格式、缩放比例
- ✅ 可以查看节点数量

### 3. 多文件处理

如果分组内包含多个不同的文件：

```javascript
else {
    // 生成文件列表信息
    const fileKeysInfo = fileKeys.map(fk => {
        const data = projectsByFileKey[fk];
        return `- ${fk.substring(0, 8)}...（${data.projectCount}个项目，${data.nodeIds.size}个节点）`;
    }).join('\n');
    
    // 显示确认对话框
    this.$confirm(
        `该分组包含 ${fileKeys.length} 个不同的 Figma 文件：\n\n${fileKeysInfo}\n\n将为每个文件分别创建渲染队列。`,
        '渲染多个文件',
        { confirmButtonText: '确定', cancelButtonText: '取消', type: 'warning' }
    ).then(async () => {
        // 为每个文件分别创建队列
        for (const fileKey of fileKeys) {
            await axios.post('/api/figma/manual/render', {
                file_key: fileKey,
                node_ids: Array.from(data.nodeIds),
                format: 'png',
                scale: 2.0
            });
        }
    });
}
```

**用户体验**:
- ✅ 清晰展示文件信息
- ✅ 统计每个文件的项目数和节点数
- ✅ 用户可以确认或取消
- ✅ 自动批量创建队列
- ✅ 显示总体成功信息

---

## 📊 对比示例

### 场景 1: 单个文件（正常情况）

```
分组: "按钮组件"
  - 项目A: file_key = "abc123"
  - 项目B: file_key = "abc123"
  - 项目C: file_key = "abc123"

点击"批量渲染"
  ↓
检测到 1 个文件
  ↓
显示渲染对话框（与单个项目渲染相同）✅
```

### 场景 2: 多个文件

```
分组: "设计系统"
  - 项目A: file_key = "abc123" (按钮)
  - 项目B: file_key = "def456" (图标)
  - 项目C: file_key = "ghi789" (卡片)

点击"批量渲染"
  ↓
检测到 3 个文件
  ↓
显示确认对话框:
  "该分组包含 3 个不同的 Figma 文件：
   - abc123...（1个项目，5个节点）
   - def456...（1个项目，20个节点）
   - ghi789...（1个项目，8个节点）
   
   将为每个文件分别创建渲染队列。"
  ↓
用户点击"确定"
  ↓
创建 3 个渲染队列 ✅
显示: "已为 3 个文件创建渲染队列，共 33 个节点"
```

---

## 🔄 完整工作流程

### 用户操作流程

```
1. 用户点击分组的"批量渲染"按钮
   ↓
2. 显示加载动画: "正在收集节点信息..."
   ↓
3. 后台收集并按 file_key 分组
   ↓
4. 判断文件数量
   ↓
   ├─→ 单个文件
   │   ↓
   │   显示渲染对话框
   │   ↓
   │   用户配置参数
   │   ↓
   │   点击"开始渲染"
   │   ↓
   │   创建渲染队列 ✅
   │
   └─→ 多个文件
       ↓
       显示确认对话框（列出所有文件）
       ↓
       用户点击"确定"或"取消"
       ↓
       ├─→ 确定: 为每个文件创建队列 ✅
       └─→ 取消: 中止操作 ⏹️
```

---

## 🎨 用户界面

### 单文件场景

```
┌────────────────────────────────────┐
│     渲染节点图片                    │
├────────────────────────────────────┤
│                                     │
│  文件Key: abc123...                │
│  节点数量: 15 个节点                │
│                                     │
│  图片格式: [PNG ▼]                 │
│  缩放比例: [2x (推荐) ▼]          │
│                                     │
│            [取消]  [开始渲染]      │
└────────────────────────────────────┘
```

### 多文件场景

```
┌────────────────────────────────────┐
│     ⚠️ 渲染多个文件                 │
├────────────────────────────────────┤
│                                     │
│  该分组包含 3 个不同的 Figma 文件: │
│                                     │
│  - abc123...（1个项目，5个节点）   │
│  - def456...（1个项目，20个节点）  │
│  - ghi789...（1个项目，8个节点）   │
│                                     │
│  将为每个文件分别创建渲染队列。    │
│                                     │
│            [取消]  [确定]          │
└────────────────────────────────────┘
```

---

## 🛡️ 错误处理

### 1. 无节点可渲染

```javascript
if (fileKeys.length === 0) {
    this.$message.warning('该分组没有可渲染的节点');
    return;
}
```

### 2. 单个文件无节点

```javascript
if (data.nodeIds.size === 0) {
    this.$message.warning('该分组没有可渲染的节点');
    return;
}
```

### 3. 部分文件创建失败

```javascript
try {
    // 创建队列...
    successCount++;
} catch (error) {
    console.error(`创建渲染队列失败 (${fileKey}):`, error);
    // 继续处理下一个文件
}

// 最后统计结果
if (successCount > 0) {
    this.$message.success(`已为 ${successCount} 个文件创建渲染队列，共 ${totalNodes} 个节点`);
} else {
    this.$message.error('创建渲染队列失败');
}
```

### 4. 用户取消操作

```javascript
this.$confirm(/* ... */).then(async () => {
    // 用户确认
}).catch(() => {
    // 用户取消 - 静默处理，不显示错误
});
```

---

## ✅ 测试清单

### 功能测试

- [ ] **单文件分组**: 显示渲染对话框，正常渲染
- [ ] **多文件分组**: 显示确认对话框，列出所有文件
- [ ] **确认渲染**: 为每个文件创建队列，显示成功消息
- [ ] **取消操作**: 点击取消，不创建队列
- [ ] **无节点**: 显示警告消息
- [ ] **部分失败**: 显示成功数量，记录失败日志

### 边界测试

- [ ] **空分组**: 正确提示"没有可渲染的节点"
- [ ] **无 file_key**: 跳过该项目
- [ ] **API 失败**: 继续处理其他文件
- [ ] **网络错误**: 显示友好错误提示

### 用户体验测试

- [ ] **加载状态**: 显示加载动画
- [ ] **信息清晰**: 文件信息清晰易懂
- [ ] **操作流畅**: 无卡顿，响应及时
- [ ] **反馈及时**: 成功/失败消息明确

---

## 📝 注意事项

### 1. 格式和缩放固定

在多文件场景下，所有文件使用相同的参数：
- 格式: PNG
- 缩放: 2.0x

**原因**: 避免为每个文件单独配置参数，简化操作流程

**改进建议**: 如需支持自定义参数，可以：
- 显示一个参数配置对话框
- 或者允许用户为每个文件单独配置

### 2. 并发请求

当前实现是串行处理（一个接一个）：

```javascript
for (const fileKey of fileKeys) {
    await axios.post(/* ... */);  // 等待完成再处理下一个
}
```

**优点**: 
- ✅ 避免触发速率限制
- ✅ 控制服务器负载

**改进建议**: 如需并发处理，可以使用 `Promise.all()`

### 3. 节点去重

使用 `Set` 自动去重：

```javascript
nodeIds: new Set()  // 自动去重
```

**好处**: 同一个节点在多个项目中出现时，只会渲染一次

---

## 🎉 总结

### 修复内容

1. ✅ **正确处理多文件分组**: 按 `file_key` 二次分组
2. ✅ **智能判断**: 单文件直接渲染，多文件询问用户
3. ✅ **批量创建**: 自动为每个文件创建渲染队列
4. ✅ **友好提示**: 清晰展示文件信息和统计数据
5. ✅ **错误处理**: 部分失败也能继续处理

### 用户体验提升

- 🎯 **更准确**: 不会错误关联节点和文件
- 🎨 **更清晰**: 明确告知用户操作内容
- 🚀 **更高效**: 一键批量处理多个文件
- 🛡️ **更安全**: 用户确认后再执行

---

**文档版本**: v1.0  
**修复时间**: 2025-11-19  
**影响范围**: `web/static/js/projects.js` - `renderGroupNodes` 方法  
**Bug 严重性**: 🔴 高（功能错误）  
**修复状态**: ✅ 已完成

---

**🎊 Bug 修复完成！感谢您的细心发现！**

