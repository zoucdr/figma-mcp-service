# Figma API 速率限制功能实现记录

## 修改日期
2025-11-18

## 修改目的
限制对 Figma Images API (`api.figma.com/v1/images`) 的调用频率，确保在配置的时间间隔内只能调用一次该接口，超过频率的请求会自动排队等待。默认为10秒间隔，可通过配置文件或环境变量调整。

## 修改的文件

### 1. configs/config.yaml

#### 新增配置项（第18-22行）
```yaml
# Figma API 配置
figma_api:
  # Images API 速率限制（秒）- 两次调用之间的最小间隔
  # 建议值：10秒，可根据实际需要调整
  images_api_rate_limit: 10
```

### 2. main.go

#### 修改的 Config 结构体（第40-42行）
```go
FigmaAPI struct {
    ImagesAPIRateLimit int `yaml:"images_api_rate_limit"`
} `yaml:"figma_api"`
```

#### 在 loadConfig 函数中添加默认值（第460-462行）
```go
if appConfig.FigmaAPI.ImagesAPIRateLimit <= 0 {
    appConfig.FigmaAPI.ImagesAPIRateLimit = 10 // 默认10秒
}
```

#### 在 .env 兼容模式添加配置（第78-84行）
```go
// Figma API 配置默认值
rateLimitStr := getEnv("FIGMA_API_RATE_LIMIT", "10")
if rateLimit, err := strconv.Atoi(rateLimitStr); err == nil && rateLimit > 0 {
    appConfig.FigmaAPI.ImagesAPIRateLimit = rateLimit
} else {
    appConfig.FigmaAPI.ImagesAPIRateLimit = 10 // 默认10秒
}
```

#### 添加配置日志输出（第91行）
```go
log.Printf("Figma Images API 速率限制: %d 秒", appConfig.FigmaAPI.ImagesAPIRateLimit)
```

#### 新增 initServicesConfig 函数（第520-527行）
```go
func initServicesConfig() {
    os.Setenv("FIGMA_API_RATE_LIMIT", strconv.Itoa(appConfig.FigmaAPI.ImagesAPIRateLimit))
    log.Printf("Services 配置已初始化: Figma API 速率限制=%d秒", appConfig.FigmaAPI.ImagesAPIRateLimit)
}
```

#### 在 main 函数中调用配置初始化（第107-108行）
```go
// 初始化 services 配置
initServicesConfig()
```

### 3. internal/services/figma.go

#### 新增内容

1. **导入 sync 和 strconv 包**
   - 在 import 中添加 `"sync"` 和 `"strconv"`

2. **定义速率限制器结构**（第31-36行）
   ```go
   type figmaImagesAPIRateLimiter struct {
       mu           sync.Mutex
       lastCallTime time.Time
       minInterval  time.Duration
   }
   ```

3. **创建全局速率限制器实例**（第38-41行）
   ```go
   var globalFigmaImagesAPILimiter = &figmaImagesAPIRateLimiter{
       minInterval: getFigmaAPIRateLimit(),
   }
   ```

4. **新增 getFigmaAPIRateLimit 函数**（第43-57行）
   ```go
   func getFigmaAPIRateLimit() time.Duration {
       rateLimitStr := os.Getenv("FIGMA_API_RATE_LIMIT")
       if rateLimitStr == "" {
           return 10 * time.Second
       }
       
       rateLimit := 10
       if val, err := strconv.Atoi(rateLimitStr); err == nil && val > 0 {
           rateLimit = val
       }
       
       return time.Duration(rateLimit) * time.Second
   }
   ```
   - 从环境变量读取配置
   - 提供默认值保护
   - 验证配置有效性

5. **实现 Wait() 方法**（第59-79行）
   - 使用互斥锁确保线程安全
   - 检查距离上次调用的时间
   - 根据配置的间隔时间自动休眠等待
   - 更新最后调用时间
   - 输出详细的日志信息

#### 修改的函数

1. **DownloadPreviewFigmaImageWithOptions**（第638-640行）
   - 在调用 Figma API 之前添加速率限制检查
   ```go
   // ⚡ 速率限制：10秒内只能调用一次 Figma Images API
   fmt.Printf("🔒 等待 Figma Images API 速率限制检查...\n")
   globalFigmaImagesAPILimiter.Wait()
   ```

2. **DownloadFilteredPreviewFigmaImage**（第1153-1155行）
   - 在调用 Figma API 之前添加速率限制检查
   ```go
   // ⚡ 速率限制：10秒内只能调用一次 Figma Images API
   fmt.Printf("🔒 等待 Figma Images API 速率限制检查...\n")
   globalFigmaImagesAPILimiter.Wait()
   ```

3. **DownloadFilteredPreviewFigmaImages**（第1596-1598行）
   - 在调用 Figma API 之前添加速率限制检查
   ```go
   // ⚡ 速率限制：10秒内只能调用一次 Figma Images API
   fmt.Printf("🔒 等待 Figma Images API 速率限制检查...\n")
   globalFigmaImagesAPILimiter.Wait()
   ```

### 2. 新增文档文件

- **docs/figma_api_rate_limit.md** - 详细的功能说明文档
- **docs/CHANGELOG_FIGMA_API_RATE_LIMIT.md** - 本修改记录文档

## 影响范围

### 直接影响的功能

1. **预览图获取**
   - 单个节点预览图
   - 过滤预览图
   - 批量预览图

2. **导出功能**
   - 项目导出时的图片下载
   - Swift 代码导出
   - Android 代码导出

3. **MCP 接口**
   - `get_preview` 工具调用

### 间接影响的文件（无需修改）

- `internal/controllers/figma.go` - 控制器层调用服务层，自动受到速率限制保护
- `internal/controllers/mcp.go` - MCP 控制器调用服务层，自动受到速率限制保护
- `internal/services/export.go` - 导出服务调用下载函数，自动受到速率限制保护

## 技术实现要点

1. **全局唯一实例**
   - 使用全局变量确保整个应用只有一个速率限制器实例
   - 所有 API 调用共享同一个限制器

2. **线程安全**
   - 使用 `sync.Mutex` 保证并发安全
   - 多个 goroutine 同时请求时会自动排队

3. **自动等待**
   - 使用 `time.Sleep()` 实现阻塞等待
   - 调用方无需关心速率限制的具体实现

4. **详细日志**
   - 显示等待时间和下次可调用时间
   - 便于调试和监控

## 优点

1. ✅ **透明实现** - 对业务代码无侵入
2. ✅ **全局控制** - 统一管理所有 API 调用
3. ✅ **自动排队** - 无需手动处理等待逻辑
4. ✅ **线程安全** - 支持高并发场景
5. ✅ **易于调试** - 提供清晰的日志输出
6. ✅ **易于维护** - 集中式配置，修改方便

## 配置方式

### 1. 修改配置文件（推荐）

在 `configs/config.yaml` 中修改：

```yaml
figma_api:
  images_api_rate_limit: 15  # 设置为15秒
```

### 2. 使用环境变量

```bash
export FIGMA_API_RATE_LIMIT=15
```

或在 `.env` 文件中：

```
FIGMA_API_RATE_LIMIT=15
```

### 配置优先级

1. 环境变量 `FIGMA_API_RATE_LIMIT`（最高）
2. 配置文件 `config.yaml`
3. 默认值：10秒

## 注意事项

1. 速率限制是全局的，会影响所有用户的所有请求
2. 如果有大量并发请求，可能会导致较长的排队等待时间
3. 建议配合缓存机制使用，减少不必要的 API 调用
4. 修改配置后需要重启服务才能生效
5. 建议配置值不小于 5 秒，避免触发 Figma API 限制

## 测试建议

1. 快速连续发起多个预览图请求，观察是否有等待提示
2. 检查日志中的时间间隔是否符合预期（≥10秒）
3. 测试并发场景下的排队机制
4. 验证缓存机制是否能有效减少 API 调用

## 后续优化建议

1. 考虑为不同用户实现独立的速率限制器
2. 可以添加速率限制配置到数据库或配置文件
3. 可以实现更复杂的令牌桶算法，允许短时间内的突发请求
4. 添加指标监控，统计 API 调用频率和等待时间

## 版本信息

- 实现版本：v1.0.0
- 默认速率限制间隔：10秒（可配置）
- 实现方式：全局互斥锁 + 时间检查
- 配置方式：YAML配置文件 / 环境变量
- 配置文件：`configs/config.yaml`

