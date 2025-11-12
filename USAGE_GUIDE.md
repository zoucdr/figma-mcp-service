# Figma Deliver 使用指南

## 概述

Figma Deliver 是一个集成了 Figma Plugin 的设计交付系统，支持直接从 Figma 导出设计稿并在 Dashboard 中预览。系统现在支持两种预览模式：
- **Figma Plugin API 预览**（推荐）：直接在 Figma 中生成预览，速度更快
- **后端 API 预览**：通过服务器生成预览，作为备用方案

## 快速开始

### 1. 启动后端服务

```bash
# 进入项目目录
cd D:\figma-deliver\service

# 构建项目
go build -o cmd/figma-deliver.exe cmd/main.go

# 启动服务器
.\cmd\figma-deliver.exe
```

服务器将在 `http://localhost:8080` 启动。

### 2. 安装 Figma Plugin

#### 方法一：开发模式安装
1. 打开 Figma 桌面应用
2. 进入 `Plugins` > `Development` > `Import plugin from manifest...`
3. 选择 `figma-plugin/manifest.json` 文件
4. 插件将出现在 Plugins 菜单中

#### 方法二：打包安装
1. 运行打包脚本：
   ```bash
   cd figma-plugin
   .\package.bat
   ```
2. 将生成的 `figma-deliver-plugin.zip` 文件分享给团队成员
3. 在 Figma 中导入插件

### 3. 配置插件

1. 在 Figma 中打开插件：`Plugins` > `Figma Deliver` > `Settings`
2. 配置服务器地址：`http://localhost:8080`
3. 输入用户名和密码进行认证
4. 认证成功后即可使用

## 主要功能

### 1. 设计导出功能

#### 从 Figma 导出到服务器
1. 在 Figma 中选择要导出的设计稿或组件
2. 打开插件：`Plugins` > `Figma Deliver` > `Export to Figma Deliver`
3. 设置项目名称和导出选项
4. 点击 "导出到服务器" 按钮
5. 等待导出完成，可以下载导出的文件

#### 支持的导出格式
- **图片格式**：PNG、JPG、SVG
- **缩放比例**：0.5x、1x、2x、3x、4x
- **批量导出**：支持选择多个节点同时导出

### 2. Dashboard 预览功能

#### 访问 Dashboard
1. 在浏览器中访问：`http://localhost:8080/dashboard`
2. 登录系统（使用相同的用户名密码）
3. 查看项目列表和设计预览

#### 预览系统
系统提供两种预览方式：

**过滤预览功能**（必须使用 Figma Plugin API）：
- ✅ 速度更快，即时生成
- ✅ 不占用服务器资源
- ✅ 支持批量预览
- ⚠️ **必须**在 Figma 环境中使用，不支持后端 API 回退

**单个节点预览**（智能切换）：
- 🔄 优先使用 Figma Plugin API（在 Figma 环境中）
- 🔄 自动回退到后端 API（在浏览器中）
- ✅ 可在任何环境中使用

#### 预览操作
1. **单个节点预览**：点击节点名称查看预览
2. **批量预览**：展开 "过滤预览" 查看所有子节点
3. **缩放调整**：使用缩放滑块调整预览清晰度
4. **格式切换**：在 PNG/JPG 之间切换预览格式

### 3. 项目管理

#### 创建新项目
1. 在 Dashboard 中点击 "新建项目"
2. 输入项目信息和 Figma 文件链接
3. 系统会自动解析 Figma 文件结构

#### 管理现有项目
- **查看项目详情**：点击项目卡片
- **编辑项目信息**：使用编辑按钮
- **删除项目**：在项目设置中删除

## 高级功能

### 1. 批量操作

#### 批量导出节点
```javascript
// 在 Figma Plugin 中选择多个节点
const selectedNodes = figma.currentPage.selection;
// 使用插件的批量导出功能
```

#### 批量预览
- 在 Dashboard 中展开 "过滤预览"
- 系统会自动加载所有子节点的预览图

### 2. 性能优化

#### 预览缓存
- 后端预览支持智能缓存
- 相同参数的预览会复用缓存结果
- 可在设置中清除缓存

#### 图片压缩
- 支持多种压缩级别
- 自动根据网络状况调整图片质量
- 支持渐进式加载

### 3. 团队协作

#### 用户管理
- 支持多用户注册和登录
- 每个用户有独立的项目空间
- 支持项目权限管理

#### 版本控制
- 自动记录设计变更历史
- 支持版本对比和回滚
- 导出文件包含版本信息

## 故障排除

### 常见问题

#### 1. 插件无法连接服务器
**症状**：插件显示 "服务器未响应" 错误

**解决方案**：
1. 检查服务器是否正在运行：`http://localhost:8080/health`
2. 确认服务器地址配置正确
3. 检查防火墙设置
4. 查看服务器日志排查错误

#### 2. 预览图片无法加载
**症状**：Dashboard 中预览图片显示加载失败

**解决方案**：
1. 检查是否在 Figma 环境中（Plugin API 预览）
2. 确认网络连接正常（后端 API 预览）
3. 清除浏览器缓存
4. 检查图片格式和大小限制

#### 3. 导出失败
**症状**：插件导出过程中出现错误

**解决方案**：
1. 确认选择的节点有效
2. 检查导出参数设置
3. 确认服务器存储空间充足
4. 查看插件控制台错误信息

### 调试模式

#### 启用调试日志
```bash
# 设置环境变量启用调试模式
set DEBUG=true
.\cmd\figma-deliver.exe
```

#### 查看插件日志
1. 在 Figma 中打开开发者工具：`Help` > `Developer Tools`
2. 查看 Console 面板的日志信息
3. 检查 Network 面板的网络请求

## API 参考

### 后端 API

#### 认证相关
- `POST /login` - 用户登录
- `POST /register` - 用户注册
- `POST /plugin/login` - 插件专用登录（返回 JWT）

#### 项目管理
- `GET /projects` - 获取项目列表
- `POST /projects` - 创建新项目
- `PUT /projects/:id` - 更新项目信息
- `DELETE /projects/:id` - 删除项目

#### Figma 集成
- `POST /figma/parse` - 解析 Figma 链接
- `GET /figma/tree/:project_id` - 获取节点树
- `GET /figma/image/:project_id/:node_id` - 获取单个节点预览
- `GET /figma/images/:project_id/:node_id` - 获取批量节点预览

### Plugin API

#### 预览功能
```javascript
// 单个节点预览
window.figmaPluginBridge.previewNode(nodeId, scale, format)

// 批量节点预览  
window.figmaPluginBridge.previewNodes(nodeIds, scale, format)

// 检查插件可用性
window.figmaPluginBridge.isAvailable()
```

## 部署指南

### 生产环境部署

#### 1. 环境配置
```bash
# 设置生产环境变量
export JWT_SECRET="your-super-secret-jwt-key"
export DB_HOST="your-database-host"
export DB_PORT="5432"
export DB_NAME="figma_deliver"
export DB_USER="your-db-user"
export DB_PASSWORD="your-db-password"
```

#### 2. 构建和部署
```bash
# 构建生产版本
go build -ldflags="-s -w" -o figma-deliver cmd/main.go

# 启动服务
./figma-deliver
```

#### 3. 反向代理配置（Nginx）
```nginx
server {
    listen 80;
    server_name your-domain.com;
    
    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```

### Docker 部署

#### Dockerfile
```dockerfile
FROM golang:1.19-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o figma-deliver cmd/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/figma-deliver .
COPY --from=builder /app/web ./web
CMD ["./figma-deliver"]
```

#### docker-compose.yml
```yaml
version: '3.8'
services:
  app:
    build: .
    ports:
      - "8080:8080"
    environment:
      - JWT_SECRET=your-secret-key
      - DB_HOST=db
    depends_on:
      - db
  
  db:
    image: postgres:13
    environment:
      - POSTGRES_DB=figma_deliver
      - POSTGRES_USER=postgres
      - POSTGRES_PASSWORD=password
    volumes:
      - postgres_data:/var/lib/postgresql/data

volumes:
  postgres_data:
```

## 更新日志

### v1.2.0 - Plugin 预览集成
- ✨ 新增 Figma Plugin API 预览功能
- ✨ 智能预览策略（Plugin API 优先，后端 API 备用）
- 🚀 大幅提升预览加载速度
- 🔧 优化批量预览性能
- 📚 完善使用文档

### v1.1.0 - 基础功能
- ✨ Figma Plugin 基础功能
- ✨ Dashboard 预览系统
- ✨ 用户认证和项目管理
- 🔐 JWT 认证机制
- 📱 响应式界面设计

## 技术支持

如果在使用过程中遇到问题，请：

1. 查看本使用指南的故障排除部分
2. 检查服务器和插件日志
3. 确认系统环境和依赖版本
4. 联系技术支持团队

---

**注意**：本系统仍在持续开发中，功能和接口可能会有变化。请关注更新日志获取最新信息。
