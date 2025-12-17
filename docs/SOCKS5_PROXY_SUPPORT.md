# SOCKS5 代理支持文档

## 更新内容

已为 Figma Deliver 添加完整的 SOCKS5 代理支持！

## ✅ 支持的代理类型

现在系统支持以下三种代理类型：

### 1. HTTP 代理
```
http://127.0.0.1:7890
http://proxy.example.com:8080
http://username:password@proxy.example.com:8080
```

### 2. HTTPS 代理
```
https://proxy.example.com:8443
https://username:password@proxy.example.com:8443
```

### 3. SOCKS5 代理 ⭐ 新增
```
socks5://127.0.0.1:9090
socks5://127.0.0.1:1080
```

## 🔧 技术实现

### 修改的文件

1. **`internal/services/figma.go`**
   - 添加 `golang.org/x/net/proxy` 导入
   - 修改 `CreateHTTPClientWithToken()` 支持 SOCKS5
   - 修改 `CreateHTTPClient()` 支持 SOCKS5

2. **`internal/controllers/api.go`**
   - 添加 `golang.org/x/net/proxy` 导入
   - 修改 `TestProxy()` 支持 SOCKS5 测试

### 核心代码逻辑

```go
if parsedURL.Scheme == "socks5" {
    // SOCKS5代理使用 golang.org/x/net/proxy
    dialer, dialErr := proxy.SOCKS5("tcp", parsedURL.Host, nil, proxy.Direct)
    if dialErr != nil {
        // 错误处理
    } else {
        transport.Dial = dialer.Dial
    }
} else if parsedURL.Scheme == "http" || parsedURL.Scheme == "https" {
    // HTTP/HTTPS代理使用标准方式
    transport.Proxy = http.ProxyURL(parsedURL)
}
```

## 🚀 使用方法

### 1. 配置 SOCKS5 代理

1. 启动您的 SOCKS5 代理服务（如 V2Ray, Shadowsocks）
2. 登录 Figma Deliver
3. 进入"个人资料"页面
4. 找到"代理配置"卡片
5. 启用代理开关
6. 输入 SOCKS5 代理地址，例如：
   ```
   socks5://127.0.0.1:9090
   ```
7. 点击"测试代理连接"按钮
8. 如果测试成功，点击"保存所有设置"

### 2. 验证代理工作

保存后，控制台会显示：

```
✅ 用户 your_username 使用SOCKS5代理: socks5://127.0.0.1:9090
```

## 🐛 故障排除

### 问题 1: 测试代理失败

**可能原因：**
- SOCKS5 代理服务未运行
- 端口号不正确
- 防火墙阻止连接

**解决方法：**
```bash
# 检查端口是否监听
netstat -an | findstr "9090"

# 确保SOCKS5服务正在运行
# 例如：V2Ray, Shadowsocks, SSR 等
```

### 问题 2: 代理地址格式错误

**正确格式：**
```
socks5://127.0.0.1:9090  ✅ 正确
socks5://localhost:9090  ✅ 正确

127.0.0.1:9090           ❌ 缺少协议前缀
socks://127.0.0.1:9090   ❌ 应该是 socks5
```

### 问题 3: 控制台显示"代理配置失败"

检查以下内容：
1. SOCKS5 服务是否允许本地连接
2. 端口是否正确
3. 是否有其他程序占用该端口

## 📊 常见 SOCKS5 代理工具

### V2Ray / V2RayN
- 默认 SOCKS5 端口: `10808`
- 配置: `socks5://127.0.0.1:10808`

### Shadowsocks
- 默认端口: `1080`
- 配置: `socks5://127.0.0.1:1080`

### SSR (ShadowsocksR)
- 默认端口: `1080` 或 `1086`
- 配置: `socks5://127.0.0.1:1080`

### Clash
- 默认 SOCKS5 端口: `7891`
- 配置: `socks5://127.0.0.1:7891`

## 🎯 优势对比

| 特性 | HTTP代理 | SOCKS5代理 |
|-----|---------|-----------|
| 协议支持 | HTTP/HTTPS | 所有TCP协议 |
| 速度 | 较快 | 更快 |
| 安全性 | 中等 | 较高 |
| 应用层 | 是 | 否（传输层） |
| 常用场景 | Web浏览 | 全局代理 |

SOCKS5 的优势：
- ✅ 更底层，支持所有TCP连接
- ✅ 不修改数据包头部
- ✅ 更好的性能
- ✅ 更适合需要绕过审查的场景

## 🔒 安全建议

1. **本地使用**
   - 优先使用 `127.0.0.1` 而不是 `localhost`
   - 确保SOCKS5服务只监听本地

2. **远程代理**
   - 使用加密的SOCKS5代理
   - 避免使用公共的免费代理
   - 确保代理服务器可信

3. **认证（未来支持）**
   - 当前版本不支持SOCKS5用户名密码认证
   - 计划在未来版本添加

## 📝 日志输出

### 成功使用 SOCKS5
```
✅ 用户 username 使用SOCKS5代理: socks5://127.0.0.1:9090
```

### 使用 HTTP 代理
```
✅ 用户 username 使用HTTP代理: http://127.0.0.1:7890
```

### 代理配置失败
```
⚠️ 用户 username SOCKS5代理配置失败，使用直连: connection refused
```

### 不支持的协议
```
⚠️ 用户 username 不支持的代理类型: socks4，使用直连
```

## 🎉 测试建议

### 测试步骤

1. **启动 SOCKS5 代理服务**
   ```bash
   # 例如启动 V2Ray
   v2ray.exe -config config.json
   ```

2. **配置代理**
   - 在个人资料页面配置 `socks5://127.0.0.1:10808`

3. **测试连接**
   - 点击"测试代理连接"
   - 应该看到"代理连接测试成功"

4. **验证功能**
   - 创建或打开一个 Figma 项目
   - 获取节点树
   - 下载预览图
   - 检查控制台输出

### 期望结果

所有 Figma API 请求应该通过 SOCKS5 代理，并且：
- ✅ 可以成功获取节点信息
- ✅ 可以下载预览图片
- ✅ 不会出现 429 错误（如果之前被限流）
- ✅ 控制台显示"使用SOCKS5代理"

## 🔗 相关文档

- [PROXY_FEATURE_IMPLEMENTATION.md](./PROXY_FEATURE_IMPLEMENTATION.md) - 代理功能完整文档
- [PROXY_FEATURE_QUICK_START.md](./PROXY_FEATURE_QUICK_START.md) - 快速开始指南
- [http_error_logging_improvements.md](./http_error_logging_improvements.md) - HTTP错误日志

## 📦 依赖包

- `golang.org/x/net/proxy` - SOCKS5 支持
- 该包已包含在 `golang.org/x/net` 中，无需额外安装

## 🆕 版本历史

### v1.1.0 - 2025-11-18
- ✅ 添加 SOCKS5 代理支持
- ✅ 支持 HTTP/HTTPS/SOCKS5 三种代理类型
- ✅ 自动识别代理协议类型
- ✅ 测试代理功能支持 SOCKS5

## 💡 未来计划

- [ ] SOCKS5 用户名密码认证
- [ ] 代理自动切换（失败时尝试其他代理）
- [ ] 代理性能监控
- [ ] 代理池支持


