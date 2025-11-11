# Figma Deliver Plugin 集成指南

本指南将帮助开发人员将 Figma Deliver Plugin 集成到现有的 Figma Deliver 服务中。

## 前提条件

- 已安装并配置 Figma Deliver 服务
- 具有 Figma 开发者账户（用于发布插件）

## 集成步骤

### 1. 配置后端服务

确保后端服务已正确配置，并支持以下 API 端点：

- `POST /login` - 用户认证
- `GET /figma/node/:file_key/:node_id` - 获取 Figma 节点
- `POST /figma/project/:project_id/export` - 导出项目

### 2. 设置 CORS

为了允许插件与服务进行通信，需要在后端服务中配置 CORS（跨源资源共享）：

```go
// 在 main.go 中添加 CORS 中间件
r.Use(func(c *gin.Context) {
    c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
    c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
    c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
    c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

    if c.Request.Method == "OPTIONS" {
        c.AbortWithStatus(204)
        return
    }

    c.Next()
})
```

### 3. 修改认证机制

确保认证 API 返回可用于后续请求的令牌：

```go
// 在 controllers/auth.go 中
func Login(c *gin.Context) {
    // ... 验证用户凭据 ...
    
    // 返回令牌
    c.JSON(http.StatusOK, gin.H{
        "token": token,
        "user": user,
    })
}
```

### 4. 自定义插件配置

根据您的服务配置修改插件代码：

1. 打开 `figma-plugin/ui/service.js`
2. 根据需要调整 API 路径和参数

### 5. 测试集成

1. 在开发模式下安装插件
2. 确保后端服务正在运行
3. 尝试认证和导出操作
4. 检查服务日志以确认请求正常处理

### 6. 发布插件

1. 在 [Figma Plugin 开发者页面](https://www.figma.com/plugin-docs/publishing/) 创建新插件
2. 上传插件文件
3. 填写插件信息和说明
4. 提交审核

## 安全考虑

- 确保所有 API 端点都受到适当的认证保护
- 考虑使用 HTTPS 来保护通信
- 实施速率限制以防止滥用

## 故障排除

### 跨域问题

如果遇到跨域错误，请检查：

- CORS 头是否正确设置
- 请求是否使用正确的方法和头

### 认证问题

如果认证失败，请检查：

- 令牌格式是否正确
- 认证头是否正确传递
- 服务器是否正确验证令牌

## 扩展功能

您可以通过以下方式扩展插件功能：

- 添加更多导出选项
- 支持批量导出
- 实现设计版本控制
- 添加团队协作功能

## 参考资源

- [Figma Plugin API 文档](https://www.figma.com/plugin-docs/api/api-overview/)
- [Figma Deliver 服务 API 文档](./API_DOCS.md)
