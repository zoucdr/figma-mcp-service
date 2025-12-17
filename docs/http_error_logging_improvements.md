# HTTP错误日志详细输出改进

## 修改日期
2025-11-18

## 问题描述
当 Figma API 返回错误（特别是 429 Too Many Requests）时，原有代码只打印了状态码，没有读取响应体和响应头中的详细错误信息，导致难以诊断问题。

## 解决方案

### 1. 新增辅助函数 `printDetailedHTTPError`

在 `internal/services/figma.go` 中添加了一个新的辅助函数，用于打印详细的 HTTP 错误信息：

```go
func printDetailedHTTPError(resp *http.Response, responseBody []byte)
```

**功能**：
- 打印 HTTP 状态码
- 打印所有响应头（包括限流相关的头信息）
- 打印响应体内容
- 尝试解析 JSON 格式的错误消息
- 特别处理 429 错误，显示限流相关信息：
  - `Retry-After`: 建议等待时间
  - `X-RateLimit-Limit`: 速率限制
  - `X-RateLimit-Remaining`: 剩余配额
  - `X-RateLimit-Reset`: 配额重置时间

### 2. 修改的函数

修改了以下 4 个函数的错误处理逻辑：

1. **GetFigmaNodes** (行 ~187)
   - 获取 Figma 节点信息时的错误处理

2. **DownloadPreviewFigmaImageWithOptions** (行 ~525)
   - 下载预览图片时的错误处理

3. **DownloadFilteredPreviewFigmaImage** (行 ~1036)
   - 下载过滤后的预览图片时的错误处理

4. **DownloadFilteredPreviewFigmaImages** (行 ~1475)
   - 批量下载过滤后的预览图片时的错误处理

### 3. 修改模式

**修改前**：
```go
// 检查响应状态
if resp.StatusCode != http.StatusOK {
    return nil, fmt.Errorf("API请求失败: %s", resp.Status)
}

// 读取响应体
responseBody, err := io.ReadAll(resp.Body)
```

**修改后**：
```go
// 先读取响应体（无论状态码如何都需要读取）
responseBody, err := io.ReadAll(resp.Body)
if err != nil {
    return nil, fmt.Errorf("读取API响应失败: %v", err)
}

// 检查响应状态
if resp.StatusCode != http.StatusOK {
    // 打印详细的错误信息
    printDetailedHTTPError(resp, responseBody)
    return nil, fmt.Errorf("API请求失败: %s", resp.Status)
}
```

## 输出示例

当遇到 429 错误时，现在会输出如下详细信息：

```
响应状态码: 429

========== HTTP 错误详情 ==========
状态码: 429 Too Many Requests

--- 响应头 (Response Headers) ---
  Content-Type: application/json
  Retry-After: 60
  X-RateLimit-Limit: 200
  X-RateLimit-Remaining: 0
  X-RateLimit-Reset: 1700000000

--- 响应体 (Response Body) ---
{"err":"Rate limit exceeded","status":429}

  错误消息: Rate limit exceeded
  状态: 429

⚠️  已达到 Figma API 请求速率限制
⏱️  建议等待时间: 60 秒
📊  速率限制: 200 请求/分钟
📊  剩余配额: 0
🔄  配额重置时间: 1700000000
===================================

API请求失败: 429 Too Many Requests
```

## 益处

1. **更好的问题诊断**：可以看到 Figma API 返回的详细错误信息
2. **限流信息可见**：清楚地知道何时可以重试请求
3. **调试便利**：响应头和响应体的完整信息有助于定位问题
4. **用户友好**：使用表情符号和中文提示，更易于理解

## 注意事项

- 此修改不影响程序的正常功能，只是增强了错误日志输出
- 所有修改都向后兼容
- 代码已通过编译测试

