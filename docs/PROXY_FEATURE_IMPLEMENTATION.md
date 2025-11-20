# 代理功能实现文档

## 实施日期
2025-11-18

## 功能概述
为 Figma Deliver 添加基于用户的代理配置功能，允许每个用户配置自己的HTTP/HTTPS/SOCKS5代理，用于绕过Figma API的IP限流限制。

## 已完成的工作

### ✅ 1. 数据库模型修改

**文件**: `internal/models/user.go`

添加了两个新字段到 User 模型：
- `ProxyEnabled bool` - 是否启用代理
- `ProxyURL string` - 代理地址（最长500字符）

```go
type User struct {
    // ... 其他字段
    ProxyEnabled bool   `gorm:"default:false" json:"proxy_enabled"`
    ProxyURL     string `gorm:"size:500" json:"proxy_url"`
    // ...
}
```

### ✅ 2. 数据库迁移

**文件**: `scripts/add_proxy_fields_to_users.sql`

创建了数据库迁移脚本，添加代理配置字段。

**注意**: GORM 会在程序启动时自动执行迁移，无需手动运行SQL脚本。

### ✅ 3. 前端界面修改

#### A. HTML模板修改

**文件**: `web/templates/profile.html`

- 添加了"代理配置"卡片，包含：
  - 代理开关（Switch组件）
  - 代理地址输入框
  - 代理格式示例
  - 测试代理按钮
  - 使用说明（Alert组件）

- 在 `window.profileData` 中添加了代理数据：
  ```javascript
  proxyEnabled: {{ .proxyEnabled }},
  proxyUrl: '{{ .proxyUrl }}'
  ```

#### B. JavaScript修改

**文件**: `web/static/js/profile.js`

添加的功能：
1. 代理表单数据和验证规则
2. `onProxyEnableChange()` - 代理开关切换处理
3. `testProxy()` - 测试代理连接
4. 修改 `updateProfile()` 方法，包含代理配置的保存

### ✅ 4. 后端API修改

#### A. Profile相关接口

**文件**: `internal/controllers/auth.go`

1. **Profile页面渲染** - 添加代理数据到模板：
   ```go
   "proxyEnabled": user.ProxyEnabled,
   "proxyUrl":     user.ProxyURL,
   ```

2. **UpdateProfile接口** - 接收和保存代理配置：
   ```go
   proxyEnabled := c.PostForm("proxy_enabled") == "true"
   proxyURL := c.PostForm("proxy_url")
   ```

#### B. 测试代理接口

**文件**: `internal/controllers/api.go`

添加了 `TestProxy()` 函数：
- 路由: `POST /api/test-proxy`
- 功能: 测试代理连接是否可用
- 验证: 通过代理请求 Figma API

#### C. 用户模型更新

**文件**: `internal/models/user.go`

修改了 `UpdateProfileWithPrompts()` 方法，添加代理参数：
```go
func (u *User) UpdateProfileWithPrompts(
    username, figmaToken, compTypes, prompts, codePrompts string,
    proxyEnabled bool, 
    proxyURL string
) error
```

## 待完成的工作

### 🔧 5. Figma服务代理集成（最关键）

**文件**: `internal/services/figma.go`

需要实现：

#### A. 创建代理辅助函数

```go
// CreateProxyFunc 根据代理URL创建代理函数
func CreateProxyFunc(proxyURL string) (func(*http.Request) (*url.URL, error), error) {
    if proxyURL == "" {
        return nil, nil
    }
    
    parsedURL, err := url.Parse(proxyURL)
    if err != nil {
        return nil, fmt.Errorf("无效的代理URL: %v", err)
    }
    
    return http.ProxyURL(parsedURL), nil
}

// CreateHTTPClient 创建支持代理的HTTP客户端
func CreateHTTPClient(userID uint) *http.Client {
    // 获取用户配置
    var user models.User
    models.DB.First(&user, userID)
    
    transport := &http.Transport{}
    
    // 如果启用了代理，配置代理
    if user.ProxyEnabled && user.ProxyURL != "" {
        proxyFunc, err := CreateProxyFunc(user.ProxyURL)
        if err == nil && proxyFunc != nil {
            transport.Proxy = proxyFunc
            fmt.Printf("✅ 使用代理: %s\n", user.ProxyURL)
        } else {
            fmt.Printf("⚠️ 代理配置错误，使用直连: %v\n", err)
        }
    }
    
    return &http.Client{
        Transport: transport,
        Timeout:   30 * time.Second,
    }
}
```

#### B. 修改所有Figma API调用

需要修改以下函数，使用带代理的HTTP客户端：

1. `GetFigmaNodes(token, fileKey, nodeID string)` - 第 ~203 行
2. `DownloadPreviewFigmaImageWithOptions(...)` - 第 ~534 行
3. `DownloadFilteredPreviewFigmaImage(...)` - 第 ~1045 行
4. `DownloadFilteredPreviewFigmaImages(...)` - 第 ~1484 行
5. `downloadPreviewImageFile(...)` - 图片下载相关
6. 其他HTTP请求位置

**修改模式**：
```go
// 原来的代码：
client := &http.Client{Timeout: 30 * time.Second}

// 修改为：
client := CreateHTTPClient(userID)  // 需要传递userID参数
```

**注意事项**：
- 需要在这些函数的参数中添加 `userID uint`
- 或者在调用时从 token 查找对应的 user
- 建议方案：在函数签名中添加 `userID` 参数

### 🔧 6. 路由注册

**文件**: `main.go`

需要注册测试代理接口：
```go
api := r.Group("/api")
{
    // ... 现有路由
    api.POST("/test-proxy", controllers.TestProxy)  // 新增
}
```

## 使用说明

### 用户配置代理

1. 登录系统
2. 进入"个人资料"页面
3. 找到"代理配置"卡片
4. 启用代理开关
5. 输入代理地址，例如：
   - HTTP代理: `http://127.0.0.1:7890`
   - SOCKS5代理: `socks5://127.0.0.1:1080`
   - 带认证: `http://user:pass@proxy.com:8080`
6. 点击"测试代理连接"验证配置
7. 点击"保存所有设置"

### 代理格式

支持的代理类型：
- **HTTP代理**: `http://host:port`
- **HTTPS代理**: `https://host:port`
- **SOCKS5代理**: `socks5://host:port`
- **带认证的代理**: `http://username:password@host:port`

### 常见代理工具

1. **Clash** - 默认端口 7890
   - HTTP: `http://127.0.0.1:7890`
   
2. **V2Ray** - 默认端口 10808
   - SOCKS5: `socks5://127.0.0.1:10808`
   
3. **Shadowsocks** - 自定义端口
   - SOCKS5: `socks5://127.0.0.1:1080`

## 技术细节

### 代理工作原理

1. 用户在前端配置代理URL
2. 后端保存到数据库
3. 发送Figma API请求时：
   - 检查用户是否启用代理
   - 如果启用，创建带代理配置的HTTP Transport
   - 使用该Transport创建HTTP Client
   - 所有请求通过代理服务器转发

### 好处

1. **绕过IP限流**: 通过更换代理IP，避免429错误
2. **用户级配置**: 每个用户可以使用不同的代理
3. **灵活切换**: 可以随时启用/禁用代理
4. **测试功能**: 保存前可以测试代理是否可用

## 数据库Schema变更

```sql
ALTER TABLE users ADD COLUMN proxy_enabled BOOLEAN DEFAULT FALSE;
ALTER TABLE users ADD COLUMN proxy_url VARCHAR(500) DEFAULT '';
CREATE INDEX idx_users_proxy_enabled ON users(proxy_enabled);
```

## API接口

### 1. 更新用户资料（含代理配置）
```
POST /profile/update
Content-Type: multipart/form-data

参数:
- username: 用户名
- figma_token: Figma Token
- comp_types: 控件类型
- prompts: 修饰提示词
- code_prompts: 代码生成提示词
- proxy_enabled: 是否启用代理（true/false）
- proxy_url: 代理地址
```

### 2. 测试代理连接
```
POST /api/test-proxy
Content-Type: application/json

请求体:
{
  "proxy_url": "http://127.0.0.1:7890"
}

响应:
{
  "message": "代理连接测试成功",
  "status": 200
}
```

## 安全考虑

1. **代理URL验证**: 前端和后端都进行格式验证
2. **用户隔离**: 每个用户只能配置自己的代理
3. **测试功能**: 允许用户在保存前测试代理
4. **错误处理**: 代理失败时回退到直连

## 性能影响

- **代理开启时**: 请求延迟取决于代理服务器速度
- **代理关闭时**: 无影响，直接连接
- **数据库查询**: 每次请求需查询用户代理配置（可优化为缓存）

## 未来优化建议

1. **缓存用户代理配置**: 减少数据库查询
2. **代理池支持**: 支持多个代理轮换
3. **自动重试**: 代理失败时自动切换到直连
4. **统计功能**: 记录代理使用情况和成功率
5. **全局代理选项**: 支持系统级代理配置

## 测试清单

- [ ] 用户可以启用/禁用代理
- [ ] 用户可以保存代理配置
- [ ] 测试代理功能正常工作
- [ ] HTTP代理可用
- [ ] SOCKS5代理可用
- [ ] 带认证的代理可用
- [ ] 代理失败时有友好的错误提示
- [ ] 所有Figma API请求都使用代理
- [ ] 禁用代理后使用直连
- [ ] 数据库正确存储代理配置

## 相关文件清单

### 修改的文件
1. `internal/models/user.go` - User模型
2. `internal/controllers/auth.go` - Profile接口
3. `internal/controllers/api.go` - TestProxy接口
4. `web/templates/profile.html` - 前端界面
5. `web/static/js/profile.js` - 前端逻辑

### 新增的文件
1. `scripts/add_proxy_fields_to_users.sql` - 数据库迁移
2. `docs/PROXY_FEATURE_IMPLEMENTATION.md` - 本文档

### 待修改的文件
1. `internal/services/figma.go` - 添加代理支持（核心）
2. `main.go` - 注册新路由

