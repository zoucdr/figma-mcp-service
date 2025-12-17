# Bug修复说明 - URL重复拼接问题

## 问题描述
在v1.0版本中，复制下载链接时出现URL重复拼接的问题：

**错误的输出**：
```
http://127.0.0.1:8080http://127.0.0.1:8080/api/mcp_1_xxx/optimized_nodes?file_key=xxx&root_node_id=xxx
```

**正确的输出**：
```
http://127.0.0.1:8080/api/mcp_1_xxx/optimized_nodes?file_key=xxx&root_node_id=xxx
```

## 问题原因
在 `copyDownloadLink()` 方法中，错误地将 `window.location.origin` 与 `apiDownloadUrl` 拼接：

```javascript
// ❌ 错误的代码（v1.0）
const fullUrl = window.location.origin + this.apiDownloadUrl;
```

但实际上 `apiDownloadUrl` 计算属性已经包含了完整的URL（包括域名）：

```javascript
// apiDownloadUrl 的实现
apiDownloadUrl() {
    // ...
    const baseUrl = window.location.origin;  // 已经包含了域名
    const fileKey = encodeURIComponent(this.project.file_key);
    const rootNodeId = encodeURIComponent(this.project.root_node_id);
    
    return `${baseUrl}/api/${mcpToken}/optimized_nodes?file_key=${fileKey}&root_node_id=${rootNodeId}`;
}
```

因此，再次添加 `window.location.origin` 会导致域名重复。

## 修复方案

### 修改文件
- `web/static/js/dashboard.js` (第446-471行)

### 修改内容
```javascript
// ✅ 正确的代码（v1.2）
// apiDownloadUrl已经是完整的URL（包含域名），直接使用
const fullUrl = this.apiDownloadUrl;
```

### 代码对比

**修改前（v1.0）**：
```javascript
copyDownloadLink() {
    if (!this.apiDownloadUrl) {
        this.$message.warning('链接不可用');
        return;
    }
    
    // 构建完整URL（包含域名）
    const fullUrl = window.location.origin + this.apiDownloadUrl;  // ❌ 错误
    
    // ...
}
```

**修改后（v1.2）**：
```javascript
copyDownloadLink() {
    if (!this.apiDownloadUrl) {
        this.$message.warning('链接不可用');
        return;
    }
    
    // apiDownloadUrl已经是完整的URL（包含域名），直接使用
    const fullUrl = this.apiDownloadUrl;  // ✅ 正确
    
    // ...
}
```

## 测试验证

### 测试步骤
1. 右键点击下载按钮
2. 选择"拷贝下载链接"
3. 粘贴到文本编辑器中
4. 检查URL格式是否正确

### 预期结果
复制的URL应该是正确的格式：
```
http://127.0.0.1:8080/api/mcp_1_xxx/optimized_nodes?file_key=xxx&root_node_id=xxx
```

不应该出现域名重复：
```
http://127.0.0.1:8080http://127.0.0.1:8080/api/...  ❌
```

## 影响范围
- **影响版本**：v1.0 - v1.1
- **修复版本**：v1.2+
- **影响功能**：右键菜单的"拷贝下载链接"功能
- **其他功能**：不影响直接点击下载功能

## 相关问题
这是一个典型的URL拼接错误，在处理URL时需要注意：
- 如果URL已经是完整路径（包含协议和域名），不要再添加baseUrl
- 如果URL是相对路径（如 `/api/xxx`），才需要添加baseUrl
- 建议在计算属性中统一处理URL格式，避免在多处重复处理

## 经验教训
1. 在拼接URL前，先检查源URL是否已经包含域名
2. 使用计算属性统一管理URL，避免重复处理
3. 添加单元测试验证URL格式的正确性
4. 在开发过程中及时测试复制的链接是否可用

## 版本记录
- v1.0: 初始实现，存在URL重复拼接问题
- v1.1: 优化菜单位置，但仍存在URL问题
- v1.2: 修复URL重复拼接问题 ✅

