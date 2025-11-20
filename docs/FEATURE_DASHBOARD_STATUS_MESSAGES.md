# Dashboard 节点树刷新状态提示

## 概述

优化了 Dashboard 页面刷新节点树功能，根据后端返回的 `status` 字段显示不同的用户提示信息。

## 修改文件

- `web/static/js/dashboard.js`

## 实现逻辑

在 `axios.get(nodeTreeUrl)` 的响应处理中，根据 `response.data.status` 字段显示不同的提示：

### 1. ✅ 刷新成功 (`status: "success"`)

```javascript
this.$message.success('✅ 节点树刷新成功');
```

**说明**：Token 冷却完成，成功从 Figma API 获取最新数据。

---

### 2. ⏰ 已加入队列 (`status: "queued"`)

```javascript
this.$message.info({
    message: `⏰ 请求已加入队列，预计 ${cooldownRemaining} 秒后处理`,
    duration: 5000,
    showClose: true
});
// 提前返回，不更新节点树
return Promise.resolve();
```

**特殊处理**：
- 显示倒计时信息
- 不更新节点树数据（保持当前数据）
- 提前返回，不执行后续流程
- 消息显示 5 秒，可手动关闭

**说明**：Token 还在冷却期，请求已加入队列等待处理。

---

### 3. ⚠️ 使用缓存数据 (`status: "cached"`)

```javascript
this.$message.warning({
    message: '⚠️ 无法连接 Figma API，已显示缓存数据（可能不是最新）',
    duration: 5000,
    showClose: true
});
```

**说明**：Figma API 请求失败，但从数据库返回了旧的缓存数据。

---

### 4. ❌ 刷新失败 (`status: "error"`)

```javascript
const errorMsg = response.data.error || '未知错误';
this.$message.error(`❌ 刷新失败: ${errorMsg}`);
```

**说明**：API 请求失败，且数据库中也没有可用的缓存数据。

---

## 用户体验优化

1. **仅在强制刷新时显示提示**
   - 正常加载节点树时（`forceRefresh=false`）不显示提示
   - 只有用户主动点击刷新按钮时才显示状态消息

2. **队列状态特殊处理**
   - 显示预计等待时间
   - 不更新节点树（避免用户困惑）
   - 提示用户稍后查看结果

3. **消息持久化**
   - 重要消息（队列、缓存）显示 5 秒
   - 可手动关闭
   - 错误消息自动关闭

4. **图标提示**
   - ✅ 成功
   - ⏰ 排队中
   - ⚠️ 警告（使用缓存）
   - ❌ 失败

## 代码位置

```javascript
// web/static/js/dashboard.js, 约 1679-1720 行

.then(response => {
    if (!response) {
        throw new Error('获取节点树失败：响应为空');
    }
    
    // 根据响应状态显示不同的提示信息
    const status = response.data.status;
    const message = response.data.message;
    
    if (forceRefresh) {
        // 只在强制刷新时显示状态提示
        switch (status) {
            case 'success':
                // ...
            case 'queued':
                // ...
            case 'cached':
                // ...
            case 'error':
                // ...
        }
    }
    
    this.nodes = response.data.nodes || [];
    // ...
})
```

## 测试场景

### 场景 1：正常刷新（Token 冷却完成）
1. 用户点击刷新按钮
2. 系统调用 Figma API 成功
3. 显示：✅ 节点树刷新成功
4. 节点树数据更新

### 场景 2：Token 冷却中
1. 用户在 30 秒内再次点击刷新
2. 系统将请求加入队列
3. 显示：⏰ 请求已加入队列，预计 25 秒后处理
4. 节点树数据不变（保持当前状态）

### 场景 3：Figma API 失败但有缓存
1. 用户点击刷新按钮
2. Figma API 请求失败
3. 系统返回数据库缓存数据
4. 显示：⚠️ 无法连接 Figma API，已显示缓存数据（可能不是最新）
5. 节点树显示缓存数据

### 场景 4：完全失败
1. 用户点击刷新按钮
2. Figma API 请求失败
3. 数据库也没有缓存
4. 显示：❌ 刷新失败: [错误详情]
5. 节点树保持当前状态

## 相关文档

- `docs/API_RefreshProjectNodeTree.md` - 后端 API 响应格式说明

