# WebSocket 后台重连快速参考

## 🎯 关键特性

### 三层防护
```
Layer 1: 可见性检测 (前台恢复时)
    ↓
Layer 2: 心跳异常检测 (间隔 > 10s)
    ↓
Layer 3: 健康检查 (每 10 秒)
```

### 自动触发场景
- ✅ 从后台切回前台
- ✅ 网络从离线恢复在线
- ✅ 心跳发送失败
- ✅ 定期健康检查发现断开

### 重连策略
```
重连延迟: 1s → 2s → 4s → 5s (最大)
最大延迟: 5 秒
网络/可见性恢复: 立即 (0 延迟)
```

## 📋 配置参数

```javascript
// 在 ui.html 中修改
state: {
  heartbeatInterval: 3000,      // 心跳间隔 (ms)
  healthCheckInterval: 10000,   // 健康检查间隔 (ms)
  minReconnectDelay: 1000,      // 最小重连延迟 (ms)
  maxReconnectDelay: 5000,      // 最大重连延迟 (ms)
}
```

## 🔍 快速诊断

### 检查连接状态 (浏览器控制台)
```javascript
console.log('Connected:', state.connected);
console.log('Socket state:', state.socket?.readyState);
// 0=CONNECTING, 1=OPEN, 2=CLOSING, 3=CLOSED
```

### 查看日志关键词
```
✅ 正常:
- "✅ WebSocket 连接已打开"
- "💓 发送心跳 ping"
- "🔍 健康检查: 连接正常"

⚠️ 异常检测:
- "⚠️ 检测到心跳间隔异常"
- "🔍 健康检查: 发现连接已断开"
- "检测到连接丢失，准备重连..."

🔄 重连中:
- "🔄 尝试重连... (第 X 次)"
- "⏰ 计划 X 秒后进行第 X 次重连..."
```

### 强制重连
1. 打开浏览器控制台
2. 执行: `if (state.socket) state.socket.close();`
3. 观察自动重连

## 🧪 快速测试

### 场景 1: 短时间后台
```
1. 连接服务器 ✓
2. 切换应用 10 秒
3. 切回 Figma
4. 应该立即恢复 ✓
```

### 场景 2: 长时间后台
```
1. 连接服务器 ✓
2. 切换应用 5 分钟
3. 切回 Figma
4. 应该自动重连 ✓
```

### 场景 3: 网络恢复
```
1. 连接服务器 ✓
2. 关闭网络
3. 恢复网络
4. 应该立即重连 ✓
```

## 🛠️ 常见操作

### 停止自动重连
点击插件中的"取消自动重连"按钮

### 手动重连
点击"连接"按钮

### 清除状态
```javascript
// 浏览器控制台
state.manualDisconnect = true;
if (state.socket) state.socket.close();
location.reload();
```

## 📊 性能指标

| 指标 | 值 |
|------|-----|
| 心跳间隔 | 3 秒 |
| 健康检查 | 10 秒 |
| CPU 使用 | < 0.1% |
| 内存占用 | < 1MB |
| 网络流量 | ~100 字节/3秒 |

## 🚨 故障排除

### 问题: 切回前台后没有自动重连
**解决**:
1. 检查是否点了"断开连接"
2. 查看控制台是否有报错
3. 确认服务器运行正常
4. 手动点击"连接"

### 问题: 重连次数过多
**解决**:
1. 点击"取消自动重连"
2. 检查服务器日志
3. 检查网络连接
4. 修复后手动重连

### 问题: 日志没有心跳信息
**可能原因**:
- 页面在后台（正常，会清理心跳）
- 连接已断开
- 心跳被挂起（会被异常检测发现）

## 📖 详细文档

- **技术实现**: `figma-plugin/WEBSOCKET_RECONNECT.md`
- **测试指南**: `demo/test_background_reconnect.md`
- **修复总结**: `docs/fix-background-reconnect-summary.md`

## 🧑‍💻 开发调试

### 模拟后台挂起
```javascript
// 停止心跳 15 秒
clearInterval(state.heartbeatTimer);
setTimeout(() => startHeartbeat(), 15000);
```

### 模拟连接断开
```javascript
if (state.socket) state.socket.close();
```

### 查看所有定时器
```javascript
console.log('Heartbeat:', state.heartbeatTimer);
console.log('Health check:', state.healthCheckTimer);
console.log('Reconnect:', state.reconnectTimer);
```

## ✅ 验证清单

- [ ] 短时间后台能立即恢复
- [ ] 长时间后台能自动重连
- [ ] 网络恢复能立即重连
- [ ] 手动断开不会自动重连
- [ ] 重连失败有指数退避
- [ ] 可以取消自动重连
- [ ] 心跳日志正常输出
- [ ] 健康检查正常工作

全部通过 = 功能正常 ✅

