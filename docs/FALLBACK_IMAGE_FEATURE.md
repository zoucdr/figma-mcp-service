# 备用图片机制 - 图片访问降级策略 🖼️

## 📋 功能概述

当 Figma API 渲染图片失败或图片不可用时，系统会自动返回备用图片 `break.jpg`，确保前端始终能获得图片内容，提升用户体验。

**核心理念**: 永不失败，优雅降级

---

## 🎯 使用场景

### 会返回备用图片的情况

| 场景 | 说明 | 示例 |
|-----|------|------|
| 🔒 **未登录** | 用户未登录系统 | Session 失效 |
| ⚙️ **未配置 Token** | 用户未设置 Figma Token | 新用户首次使用 |
| 📝 **参数错误** | 缺少必要参数或格式错误 | 缺少 `file_key` 或 `node_id` |
| 💾 **查询失败** | 数据库查询缓存失败 | 数据库连接异常 |
| 🚫 **无缓存** | 图片尚未渲染完成 | 首次请求图片 |
| ⚠️ **队列创建失败** | 无法创建渲染队列 | Token 冷却中或系统错误 |

### 会返回真实图片的情况

| 场景 | 说明 | 来源 |
|-----|------|------|
| ✅ **OBS 缓存** | 图片已上传到华为云 OBS | 重定向到 OBS URL |
| ✅ **Figma CDN** | 图片在 Figma CDN 上 | 重定向到 Figma CDN URL |

---

## 🏗️ 实现原理

### 工作流程

```
GET /api/figma/node/image?file_key=xxx&node_id=123
   ↓
检查登录状态
   ↓
   ├─→ 未登录 → 返回 break.jpg ❌
   ↓
获取用户信息
   ↓
   ├─→ 失败 → 返回 break.jpg ❌
   ↓
检查 Figma Token
   ↓
   ├─→ 未配置 → 返回 break.jpg ❌
   ↓
验证请求参数
   ↓
   ├─→ 参数错误 → 返回 break.jpg ❌
   ↓
查询图片缓存
   ↓
   ├─→ 查询失败 → 返回 break.jpg ❌
   ↓
   ├─→ 有缓存
   │   ↓
   │   ├─→ OBS URL 存在 → 重定向到 OBS ✅
   │   ├─→ Figma CDN URL 存在 → 重定向到 Figma CDN ✅
   │   └─→ 都不存在 → 返回 break.jpg ❌
   ↓
   └─→ 无缓存
       ↓
       创建渲染队列（后台处理）
       ↓
       ├─→ 创建成功 → 返回 break.jpg 作为占位 ⏳
       └─→ 创建失败 → 返回 break.jpg ❌
```

### 核心代码

#### GetNodeImage 方法

```go
func (cc *CacheController) GetNodeImage(c *gin.Context) {
    // 验证登录、Token、参数...
    // 任何错误都调用 returnFallbackImage()
    
    // 查询缓存
    image, valid, err := cc.cacheService.GetNodeImage(...)
    if err != nil {
        cc.returnFallbackImage(c)
        return
    }

    if valid && image != nil {
        // 有缓存，重定向到真实图片
        if image.OBSURL != "" {
            c.Redirect(http.StatusFound, image.OBSURL)
            return
        } else if image.FigmaCDNURL != "" {
            c.Redirect(http.StatusFound, image.FigmaCDNURL)
            return
        }
    }

    // 无缓存，创建队列并返回备用图片
    cc.queueService.CreateRenderQueue(...)
    cc.returnFallbackImage(c)
}
```

#### returnFallbackImage 方法

```go
// returnFallbackImage 返回备用图片
func (cc *CacheController) returnFallbackImage(c *gin.Context) {
    // 直接返回 break.jpg 文件
    c.File("web/static/img/break.jpg")
}
```

---

## 📊 用户体验

### 优化前 vs 优化后

#### 优化前（返回错误）

```
请求图片 → 无缓存 → 返回 JSON 错误
{
  "error": "图片不可用"
}

前端显示: ❌ 错误提示或空白
用户体验: ⭐⭐ 差
```

#### 优化后（返回备用图片）

```
请求图片 → 无缓存 → 返回 break.jpg

前端显示: 🖼️ 占位图片（break.jpg）
用户体验: ⭐⭐⭐⭐⭐ 优秀
```

### 用户感知

| 场景 | 优化前 | 优化后 |
|-----|-------|-------|
| 图片加载失败 | ❌ 看到错误信息 | ✅ 看到占位图 |
| 图片渲染中 | ⏳ 看到"正在加载" | ✅ 看到占位图 |
| 未登录访问 | ❌ 看到错误提示 | ✅ 看到占位图 |
| Token 失效 | ❌ 看到错误提示 | ✅ 看到占位图 |

---

## 🎨 备用图片设计建议

### 当前图片: `break.jpg`

建议设计要求：
- ✅ **清晰的占位符**: 让用户知道这是临时图片
- ✅ **友好的提示**: 可包含"图片加载中"或"图片不可用"文字
- ✅ **品牌一致性**: 与系统整体风格一致
- ✅ **适当的尺寸**: 不要过大，建议 < 100KB

### 示例设计方案

#### 方案 A: 简约风格
```
┌─────────────────────┐
│                     │
│    📷 图片加载中    │
│                     │
│   Figma Deliver     │
│                     │
└─────────────────────┘
```

#### 方案 B: 错误提示风格
```
┌─────────────────────┐
│                     │
│    ⚠️ 图片不可用    │
│                     │
│  请稍后刷新重试     │
│                     │
└─────────────────────┘
```

#### 方案 C: 品牌风格
```
┌─────────────────────┐
│   [Logo]            │
│                     │
│  Figma Deliver      │
│  图片正在准备中...   │
│                     │
└─────────────────────┘
```

---

## 🔄 完整流程示例

### 示例 1: 首次请求图片

```
1. 前端请求: GET /api/figma/node/image?file_key=xxx&node_id=123

2. 后端处理:
   - 检查缓存：无
   - 创建渲染队列：成功
   - 返回 break.jpg

3. 前端显示: 
   <img src="/api/figma/node/image?..."> 
   显示 break.jpg 占位图

4. 后台调度器:
   - 5秒后处理队列
   - 调用 Figma API
   - 上传到 OBS
   - 保存缓存记录

5. 用户刷新页面:
   - 再次请求同一图片
   - 缓存已存在
   - 重定向到 OBS URL
   - 显示真实图片 ✅
```

### 示例 2: Token 冷却中

```
1. 前端请求: GET /api/figma/node/image?file_key=xxx&node_id=456

2. 后端处理:
   - 检查缓存：无
   - 尝试创建队列：失败（Token 冷却中）
   - 返回 break.jpg

3. 前端显示:
   显示 break.jpg 占位图
   （用户不会看到错误）

4. 30秒后:
   - Token 冷却结束
   - 调度器自动处理队列
   - 图片渲染完成

5. 用户再次访问:
   - 缓存已存在
   - 显示真实图片 ✅
```

### 示例 3: 未登录访问

```
1. 前端请求: GET /api/figma/node/image?file_key=xxx&node_id=789
   （用户未登录）

2. 后端处理:
   - 检查登录：未登录
   - 返回 break.jpg

3. 前端显示:
   显示 break.jpg 占位图
   （不会显示"401 未授权"错误）

4. 用户体验:
   - 页面正常显示
   - 看到占位图而非错误
   - 可以继续浏览其他内容 ✅
```

---

## 📝 API 响应对比

### 优化前（返回错误 JSON）

#### 成功时
```http
HTTP/1.1 302 Found
Location: https://obs.cn-north-4.myhuaweicloud.com/...
```

#### 失败时
```http
HTTP/1.1 500 Internal Server Error
Content-Type: application/json

{
  "error": "查询缓存失败"
}
```

### 优化后（始终返回图片）

#### 成功时（有缓存）
```http
HTTP/1.1 302 Found
Location: https://obs.cn-north-4.myhuaweicloud.com/...
```

#### 失败时（无缓存/错误）
```http
HTTP/1.1 200 OK
Content-Type: image/jpeg
Content-Length: 45678

[break.jpg 的二进制数据]
```

---

## 🛠️ 前端使用

### 标准用法

```html
<!-- 直接使用，无需错误处理 -->
<img src="/api/figma/node/image?file_key=xxx&node_id=123&format=png&scale=2" 
     alt="节点图片">
```

### 优化用法（带重试）

```html
<img id="nodeImage" 
     src="/api/figma/node/image?file_key=xxx&node_id=123" 
     alt="节点图片">

<script>
const img = document.getElementById('nodeImage');

// 5秒后刷新一次（如果是占位图，可能已渲染完成）
setTimeout(() => {
    // 添加时间戳避免缓存
    const url = new URL(img.src);
    url.searchParams.set('_t', Date.now());
    img.src = url.toString();
}, 5000);
</script>
```

### React 用法

```jsx
function FigmaImage({ fileKey, nodeId }) {
    const [src, setSrc] = useState(
        `/api/figma/node/image?file_key=${fileKey}&node_id=${nodeId}`
    );
    const [retryCount, setRetryCount] = useState(0);

    useEffect(() => {
        // 每5秒重试一次，最多3次
        if (retryCount < 3) {
            const timer = setTimeout(() => {
                setSrc(`/api/figma/node/image?file_key=${fileKey}&node_id=${nodeId}&_t=${Date.now()}`);
                setRetryCount(retryCount + 1);
            }, 5000);
            
            return () => clearTimeout(timer);
        }
    }, [retryCount, fileKey, nodeId]);

    return (
        <img 
            src={src} 
            alt="Figma节点" 
            onError={() => {
                // 即使加载失败，也会显示 break.jpg，无需特殊处理
            }}
        />
    );
}
```

### Vue 用法

```vue
<template>
    <img :src="imageSrc" alt="Figma节点" />
</template>

<script>
export default {
    props: ['fileKey', 'nodeId'],
    data() {
        return {
            retryCount: 0
        };
    },
    computed: {
        imageSrc() {
            const timestamp = this.retryCount > 0 ? `&_t=${Date.now()}` : '';
            return `/api/figma/node/image?file_key=${this.fileKey}&node_id=${this.nodeId}${timestamp}`;
        }
    },
    mounted() {
        // 5秒后重试
        setTimeout(() => {
            this.retryCount = 1;
        }, 5000);
    }
};
</script>
```

---

## 🎯 最佳实践

### 1. 自动重试

```javascript
// 智能重试：初次显示占位图，后台渲染完成后自动更新
function loadFigmaImage(fileKey, nodeId, imgElement) {
    const baseUrl = `/api/figma/node/image?file_key=${fileKey}&node_id=${nodeId}`;
    
    // 首次加载
    imgElement.src = baseUrl;
    
    // 5秒后重试一次
    setTimeout(() => {
        imgElement.src = `${baseUrl}&_t=${Date.now()}`;
    }, 5000);
    
    // 30秒后再重试一次（Token冷却时间）
    setTimeout(() => {
        imgElement.src = `${baseUrl}&_t=${Date.now()}`;
    }, 30000);
}
```

### 2. 加载状态提示

```javascript
// 在图片上叠加加载状态
function showLoadingOverlay(imgElement) {
    const overlay = document.createElement('div');
    overlay.className = 'loading-overlay';
    overlay.textContent = '图片渲染中...';
    imgElement.parentElement.appendChild(overlay);
    
    // 5秒后移除提示
    setTimeout(() => {
        overlay.remove();
    }, 5000);
}
```

### 3. 缓存检测

```javascript
// 检测是否是备用图片（可选）
async function isFallbackImage(imgUrl) {
    const response = await fetch(imgUrl, { method: 'HEAD' });
    const contentLength = response.headers.get('Content-Length');
    
    // break.jpg 的大小（假设约 50KB）
    return contentLength < 100000;
}
```

---

## 🔍 调试和监控

### 日志记录

当前系统不会记录返回备用图片的日志（避免日志过多），但可以通过以下方式监控：

#### 方法 1: 响应头标记

```go
func (cc *CacheController) returnFallbackImage(c *gin.Context) {
    // 添加自定义响应头，方便前端识别
    c.Header("X-Image-Type", "fallback")
    c.Header("X-Image-Reason", "cache-miss")
    c.File("web/static/img/break.jpg")
}
```

#### 方法 2: 日志统计

```go
var fallbackCounter atomic.Uint64

func (cc *CacheController) returnFallbackImage(c *gin.Context) {
    fallbackCounter.Add(1)
    c.File("web/static/img/break.jpg")
}

// 定期输出统计
go func() {
    ticker := time.NewTicker(1 * time.Hour)
    for range ticker.C {
        count := fallbackCounter.Swap(0)
        if count > 0 {
            log.Printf("📊 [备用图片] 过去1小时返回 %d 次备用图片", count)
        }
    }
}()
```

---

## 📈 性能优化

### 1. 备用图片缓存

```go
// 优化：缓存 break.jpg 文件内容，避免重复读取
var (
    fallbackImageData []byte
    fallbackImageOnce sync.Once
)

func (cc *CacheController) returnFallbackImage(c *gin.Context) {
    fallbackImageOnce.Do(func() {
        data, err := ioutil.ReadFile("web/static/img/break.jpg")
        if err != nil {
            log.Printf("❌ 读取备用图片失败: %v", err)
            return
        }
        fallbackImageData = data
    })
    
    if len(fallbackImageData) > 0 {
        c.Data(http.StatusOK, "image/jpeg", fallbackImageData)
    } else {
        c.File("web/static/img/break.jpg")
    }
}
```

### 2. CDN 部署

将 `break.jpg` 部署到 CDN，减轻服务器压力：

```go
func (cc *CacheController) returnFallbackImage(c *gin.Context) {
    // 重定向到 CDN
    c.Redirect(http.StatusFound, "https://cdn.example.com/break.jpg")
}
```

---

## ✅ 测试清单

### 功能测试

- [ ] **未登录**: 返回 break.jpg
- [ ] **未配置 Token**: 返回 break.jpg
- [ ] **参数错误**: 返回 break.jpg
- [ ] **查询失败**: 返回 break.jpg
- [ ] **无缓存**: 返回 break.jpg
- [ ] **有缓存（OBS）**: 重定向到 OBS URL
- [ ] **有缓存（Figma CDN）**: 重定向到 Figma CDN URL
- [ ] **队列创建失败**: 返回 break.jpg

### 性能测试

- [ ] **并发请求**: 1000+ 并发请求 break.jpg，响应正常
- [ ] **文件大小**: break.jpg < 100KB
- [ ] **响应时间**: < 50ms

### 用户体验测试

- [ ] **前端显示**: 图片正常显示，无错误
- [ ] **自动刷新**: 5秒后自动获取真实图片
- [ ] **长期等待**: 30秒后再次刷新能获取真实图片

---

## 🎉 总结

### 优势

1. ✅ **永不失败**: 任何情况下都能返回图片
2. ✅ **用户体验**: 看到占位图而非错误
3. ✅ **前端简化**: 无需复杂的错误处理
4. ✅ **后台处理**: 自动创建队列，下次访问获得真图
5. ✅ **优雅降级**: 符合现代Web设计理念

### 适用场景

- 🖼️ 图片展示页面
- 📱 移动端应用
- 🌐 公开访问的页面
- 🔄 需要自动重试的场景

### 注意事项

1. ⚠️ **备用图片设计**: 应清晰标识这是占位图
2. ⚠️ **文件大小**: 保持 break.jpg 文件较小
3. ⚠️ **自动刷新**: 前端建议实现自动刷新机制
4. ⚠️ **性能监控**: 定期检查备用图片返回频率

---

**文档版本**: v1.0  
**实现时间**: 2025-11-19  
**功能状态**: ✅ 已完成  
**作者**: AI Assistant

---

**🎊 备用图片机制已完成！享受更好的用户体验！**

