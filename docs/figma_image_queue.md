# Figma 图片请求队列机制

## 概述

为了防止多个并发请求重复向 Figma API 请求相同的图片，我们实现了一个请求队列去重机制。该机制确保对于相同参数的图片请求，在任何时刻只会有一个实际的 API 调用。

## 工作原理

### 1. 请求唯一标识生成

每个图片请求都会根据以下参数生成一个唯一的 MD5 哈希值作为标识：

- `fileKey`: Figma 文件 Key
- `nodeID`: 节点 ID
- `format`: 图片格式（png/jpg/svg）
- `scale`: 缩放比例
- `excludeModified`: 是否排除修改的节点
- `projectID`: 项目 ID

```go
requestKey := generateRequestKey(fileKey, nodeID, format, scale, excludeModified, projectID)
```

### 2. 队列管理

使用全局的 `imageRequestQueue` 管理器：

```go
type imageRequestQueue struct {
    mu       sync.Mutex
    requests map[string]*imageRequest
}
```

- `mu`: 互斥锁，保证并发安全
- `requests`: 存储正在进行中的请求

### 3. 请求处理流程

```
┌─────────────────────────────────────────────────────────────┐
│                      请求到达                                │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
                 生成请求 Key (MD5)
                         │
                         ▼
            ┌────────────────────────┐
            │  检查队列中是否已存在   │
            └────────┬───────────────┘
                     │
         ┌───────────┴───────────┐
         │                       │
         ▼                       ▼
    已存在请求               首次请求
         │                       │
         ▼                       ▼
    等待完成                 创建新请求
      (阻塞)                     │
         │                       ▼
         │                  调用 Figma API
         │                       │
         │                       ▼
         │                  保存结果
         │                       │
         │                       ▼
         │              从队列中移除
         │                       │
         │                       ▼
         │              通知所有等待者
         │                       │
         └───────────┬───────────┘
                     │
                     ▼
              返回图片路径
```

### 4. 核心代码实现

#### 4.1 请求等待机制

```go
func (q *imageRequestQueue) getOrWaitForImage(
    requestKey string,
    fetchFunc func() (string, error),
) (string, error) {
    q.mu.Lock()

    // 检查是否已有相同请求在进行
    if req, exists := q.requests[requestKey]; exists {
        q.mu.Unlock()
        <-req.done // 等待请求完成
        return req.imagePath, req.err
    }

    // 创建新请求
    req := &imageRequest{done: make(chan struct{})}
    q.requests[requestKey] = req
    q.mu.Unlock()

    // 执行获取操作
    imagePath, err := fetchFunc()
    req.imagePath = imagePath
    req.err = err

    // 清理并通知
    q.mu.Lock()
    delete(q.requests, requestKey)
    q.mu.Unlock()
    
    close(req.done)

    return imagePath, err
}
```

#### 4.2 在 GetFigmaImage 中使用

```go
// 生成请求唯一标识
requestKey := generateRequestKey(project.FileKey, nodeID, format, scale, excludeModified, projectID)

// 使用队列管理器获取图片
imagePath, err := globalImageQueue.getOrWaitForImage(requestKey, func() (string, error) {
    // 实际的 Figma API 调用
    return services.DownloadPreviewFigmaImageWithOptions(...)
})
```

## 性能优势

### 示例场景

假设有 10 个并发请求同时请求相同的图片：

**没有队列机制：**
- 10 次 Figma API 调用
- 10 次图片下载
- 消耗 10 倍的带宽和 API 配额

**有队列机制：**
- 1 次 Figma API 调用
- 1 次图片下载
- 其他 9 个请求等待并共享结果
- 节省 90% 的资源

### 测试结果

运行单元测试验证：

```bash
cd internal/controllers
go test -v -run TestImageRequestQueue
```

测试输出显示：
```
✓ 成功：10个并发请求只触发了1次实际的图片获取操作
```

## 使用场景

该队列机制特别适用于以下场景：

1. **前端并发请求**: 用户快速切换节点时，避免重复请求
2. **批量预览加载**: 多个组件同时加载相同的图片资源
3. **页面刷新**: 页面刷新时多个请求同时到达
4. **分布式环境**: 多个用户同时访问相同的项目节点

## 注意事项

1. **缓存优先**: 队列机制不会替代文件缓存，仍然优先使用本地缓存
2. **并发安全**: 使用 `sync.Mutex` 确保线程安全
3. **错误传播**: 如果首个请求失败，所有等待的请求都会收到相同的错误
4. **自动清理**: 请求完成后会自动从队列中移除，不会造成内存泄漏

## 相关文件

- `internal/controllers/figma.go` - 主要实现
- `internal/controllers/figma_queue_test.go` - 单元测试
- `internal/services/figma.go` - Figma API 服务层

## 未来改进

可能的优化方向：

1. **超时机制**: 为等待的请求添加超时保护
2. **请求统计**: 记录队列命中率和性能指标
3. **分布式队列**: 如果使用多实例部署，可以考虑使用 Redis 实现分布式队列
4. **优先级队列**: 为不同类型的请求设置不同的优先级

