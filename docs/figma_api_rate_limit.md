# Figma Images API 速率限制功能

## 概述

为了避免对 Figma API 造成过大压力，并防止触发 API 速率限制，我们实现了一个**基于 Token 的独立速率限制器**，确保每个 Figma Token 对 `api.figma.com/v1/images` 接口的调用频率不超过配置的时间间隔（默认 **15秒一次**）。

## 核心特性

- ✨ **基于 Token 隔离**：不同用户的 Token 拥有独立的速率限制器，互不影响
- 🔒 **自动管理**：系统自动为每个 Token 创建和管理独立的限制器
- 🚦 **线程安全**：使用读写锁保证高并发场景下的安全性
- 📊 **详细日志**：每个 Token 的速率限制状态都有独立的日志标识

## 实现方式

### 1. 速率限制器结构

在 `internal/services/figma.go` 中定义了两个核心结构：

#### 单个 Token 的限制器

```go
type figmaImagesAPIRateLimiter struct {
    mu           sync.Mutex
    lastCallTime time.Time
    minInterval  time.Duration
    token        string // Token 标识（用于日志）
}
```

#### 限制器管理器

```go
type figmaImagesAPIRateLimiterManager struct {
    mu          sync.RWMutex
    limiters    map[string]*figmaImagesAPIRateLimiter
    minInterval time.Duration
}
```

### 2. 全局管理器实例

创建了一个全局的限制器管理器：

```go
var globalFigmaImagesAPILimiterManager = &figmaImagesAPIRateLimiterManager{
    limiters:    make(map[string]*figmaImagesAPIRateLimiter),
    minInterval: getFigmaAPIRateLimit(),
}
```

### 3. 自动创建机制

当 API 调用发生时，系统会：
1. 根据 Token 查找对应的限制器
2. 如果不存在，自动为该 Token 创建新的限制器
3. 使用读写锁保证并发安全

```go
func (m *figmaImagesAPIRateLimiterManager) getOrCreateLimiter(token string) *figmaImagesAPIRateLimiter {
    // 先用读锁尝试获取
    m.mu.RLock()
    limiter, exists := m.limiters[token]
    m.mu.RUnlock()
    
    if exists {
        return limiter
    }
    
    // 不存在则创建新限制器
    m.mu.Lock()
    defer m.mu.Unlock()
    
    // 双重检查，防止并发创建
    limiter, exists = m.limiters[token]
    if exists {
        return limiter
    }
    
    // 创建新限制器
    limiter = &figmaImagesAPIRateLimiter{
        minInterval: m.minInterval,
        token:       tokenID,
    }
    m.limiters[token] = limiter
    
    return limiter
}
```

### 4. 等待机制

每个 Token 的限制器独立工作：
- 检查该 Token 距离上次调用的时间
- 如果不足配置的间隔，自动阻塞等待
- 等待完成后更新该 Token 的最后调用时间
- 输出带 Token 标识的详细日志

## 应用范围

速率限制器已经应用到所有调用 Figma Images API 的函数：

1. **DownloadPreviewFigmaImageWithOptions** (第640行)
   - 下载单个节点的预览图

2. **DownloadFilteredPreviewFigmaImage** (第1155行)
   - 下载过滤后的预览图（单个）

3. **DownloadFilteredPreviewFigmaImages** (第1598行)
   - 批量下载过滤后的预览图

## 工作流程

### 多用户场景（不同 Token）

```
用户A (TokenA):
  请求1 → 立即执行 → 记录时间 TA1
  请求2（3秒后）→ 等待12秒 → 执行（TA1+15秒）

用户B (TokenB):
  请求1 → 立即执行 → 记录时间 TB1  (与用户A无关，不受影响)
  请求2（5秒后）→ 等待10秒 → 执行（TB1+15秒）
```

### 单用户场景（同一 Token）

```
TokenX:
  请求1 → 立即执行 → 记录时间 T1
         ↓
  请求2（3秒后）→ 等待12秒 → 执行（T1+15秒）
                    ↓
                 记录时间 T2
                    ↓
  请求3（18秒后）→ 立即执行（距T2已超15秒）
```

## 日志输出

速率限制器会输出以下带 Token 标识的日志：

### 首次使用某个 Token
```
🔑 为 Token [figd_AbCdE***] 创建独立速率限制器，间隔=15s
🔒 等待 Figma Images API 速率限制检查 [Token:figd_AbCdE***]...
✅ [Token:figd_AbCdE***] Figma Images API 调用已允许，下次可调用时间: 14:30:45
```

### 需要等待时
```
🔒 等待 Figma Images API 速率限制检查 [Token:figd_AbCdE***]...
⏱️ [Token:figd_AbCdE***] Figma Images API 速率限制: 距离上次调用仅 3.5 秒，需要等待 11.5 秒
✅ [Token:figd_AbCdE***] Figma Images API 调用已允许，下次可调用时间: 14:30:45
```

### 无需等待时
```
🔒 等待 Figma Images API 速率限制检查 [Token:figd_AbCdE***]...
✅ [Token:figd_AbCdE***] Figma Images API 调用已允许，下次可调用时间: 14:30:45
```

### 多个 Token 同时工作
```
🔑 为 Token [figd_User1***] 创建独立速率限制器，间隔=15s
✅ [Token:figd_User1***] Figma Images API 调用已允许，下次可调用时间: 14:30:30

🔑 为 Token [figd_User2***] 创建独立速率限制器，间隔=15s
✅ [Token:figd_User2***] Figma Images API 调用已允许，下次可调用时间: 14:30:35
```

## 并发处理

- 使用 `sync.RWMutex` 管理限制器映射表，支持高并发读取
- 每个 Token 的限制器使用 `sync.Mutex` 保证独立的线程安全
- 同一 Token 的并发请求会自动排队
- 不同 Token 的请求完全独立，互不阻塞
- 双重检查锁定（Double-Checked Locking）防止并发创建

## 优点

1. ✅ **基于 Token 隔离**：不同用户互不影响，公平分配资源
2. ✅ **自动管理**：系统自动为每个 Token 创建和维护限制器
3. ✅ **自动排队**：超过频率的请求会自动等待，无需手动处理
4. ✅ **透明实现**：对调用方来说是透明的，无需修改业务逻辑
5. ✅ **详细日志**：每个 Token 有独立的日志标识，便于调试和监控
6. ✅ **线程安全**：使用读写锁和互斥锁保证多线程环境下的安全性
7. ✅ **高性能**：读写锁优化，多用户并发场景下性能更好

## 注意事项

1. ✅ **Token 独立**：每个 Token 有独立的速率限制，不同用户互不影响
2. ⚠️ **同一 Token 排队**：同一 Token 的并发请求会排队等待
3. 💾 **缓存优先**：缓存机制可以有效减少 API 调用次数
4. 🔄 **批处理建议**：建议在客户端也实现适当的请求批处理策略
5. 🔑 **Token 管理**：限制器自动管理，无需手动清理

## 配置调整

### 方法1：修改配置文件（推荐）

在 `configs/config.yaml` 中修改速率限制配置：

```yaml
# Figma API 配置
figma_api:
  # Images API 速率限制（秒）- 两次调用之间的最小间隔
  # 建议值：10秒，可根据实际需要调整
  images_api_rate_limit: 10  # 修改这里的数值
```

### 方法2：使用环境变量

设置环境变量 `FIGMA_API_RATE_LIMIT`：

```bash
export FIGMA_API_RATE_LIMIT=15  # 设置为15秒
```

或在 `.env` 文件中：

```
FIGMA_API_RATE_LIMIT=15
```

### 配置优先级

1. 环境变量 `FIGMA_API_RATE_LIMIT`（最高优先级）
2. 配置文件 `config.yaml` 中的 `figma_api.images_api_rate_limit`
3. 默认值：10秒（如果以上都未配置）

### 配置说明

- **单位**：秒
- **最小值**：建议不小于 5 秒，避免触发 Figma API 限制
- **推荐值**：10-15 秒
- **注意**：修改配置后需要重启服务才能生效

## 相关代码文件

- `internal/services/figma.go` - 速率限制器实现和应用
- `internal/controllers/figma.go` - 使用预览图API的控制器
- `internal/controllers/mcp.go` - MCP接口也会间接使用

## 测试验证

### 验证基于 Token 的独立限制

#### 测试1：同一 Token 的速率限制

1. 使用同一个 Token 连续快速请求多个预览图
2. 观察日志输出：
   ```
   🔑 为 Token [figd_xxx***] 创建独立速率限制器，间隔=15s
   ✅ [Token:figd_xxx***] 第1个请求 - 立即执行
   ⏱️ [Token:figd_xxx***] 第2个请求 - 需要等待 12 秒
   ```
3. 确认同一 Token 的API调用间隔不少于配置的秒数

#### 测试2：不同 Token 的独立性

1. 使用两个不同的 Token 同时请求
2. 观察日志输出：
   ```
   🔑 为 Token [figd_AAA***] 创建独立速率限制器，间隔=15s
   ✅ [Token:figd_AAA***] 调用已允许
   
   🔑 为 Token [figd_BBB***] 创建独立速率限制器，间隔=15s
   ✅ [Token:figd_BBB***] 调用已允许  (无需等待，与 Token AAA 独立)
   ```
3. 确认不同 Token 的请求互不影响

#### 测试3：并发场景

1. 同时发起多个用户的请求
2. 验证每个 Token 都有独立的速率控制
3. 确认不会出现一个用户阻塞其他用户的情况

## 更新日期

2025-11-18

