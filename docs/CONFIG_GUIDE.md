# Figma Deliver 配置指南

## 配置文件位置

主配置文件：`configs/config.yaml`

示例配置文件：`configs/config.example.yaml`

## 快速开始

1. **复制示例配置文件**
   ```bash
   cp configs/config.example.yaml configs/config.yaml
   ```

2. **修改配置**
   编辑 `configs/config.yaml`，根据实际情况修改各项配置

3. **重启服务**
   ```bash
   # 如果服务正在运行，重启以使配置生效
   ./figma-deliver
   ```

## 配置项说明

### 应用配置 (app)

- **port**: 服务监听端口，默认 `8080`
- **gin_mode**: Gin 框架模式
  - `debug`: 开发模式（详细日志）
  - `release`: 生产模式（精简日志）
  - `test`: 测试模式
- **session_secret**: Session 加密密钥，生产环境务必修改

### 数据库配置 (database)

- **host**: MySQL 服务器地址
- **port**: MySQL 端口，默认 `3306`
- **user**: 数据库用户名
- **password**: 数据库密码
- **name**: 数据库名称

### 存储配置 (storage)

- **export_dir**: 导出文件存储目录，默认 `./exports`
- **temp_dir**: 临时文件存储目录，默认 `./temp`

### Figma API 配置 (figma_api)

- **images_api_rate_limit**: Images API 速率限制（秒）
  - **默认值**: 10 秒
  - **推荐值**: 10-15 秒
  - **最小值**: 建议不小于 5 秒
  - **说明**: 控制对 `api.figma.com/v1/images` 接口的调用频率

## 环境变量支持

所有配置项都支持通过环境变量覆盖：

```bash
# 应用配置
export PORT=8080
export GIN_MODE=release
export SESSION_SECRET=your-secret

# 数据库配置
export DB_HOST=127.0.0.1
export DB_PORT=3306
export DB_USER=root
export DB_PASS=password
export DB_NAME=figma_deliver

# 存储配置
export EXPORT_DIR=./exports
export TEMP_DIR=./temp

# Figma API 配置
export FIGMA_API_RATE_LIMIT=15
```

## 配置优先级

1. **环境变量**（最高优先级）
2. **配置文件** (`config.yaml`)
3. **默认值**（最低优先级）

## 常见配置场景

### 场景1：开发环境

```yaml
app:
  port: "8080"
  gin_mode: "debug"
  session_secret: "dev-secret"

figma_api:
  images_api_rate_limit: 5  # 开发环境可以设置更短的间隔
```

### 场景2：生产环境

```yaml
app:
  port: "80"
  gin_mode: "release"
  session_secret: "your-production-secret-key-here"

figma_api:
  images_api_rate_limit: 15  # 生产环境建议设置较长的间隔
```

### 场景3：Docker 部署

使用环境变量配置：

```dockerfile
ENV GIN_MODE=release
ENV PORT=8080
ENV FIGMA_API_RATE_LIMIT=10
ENV DB_HOST=mysql-server
ENV DB_PORT=3306
```

或使用 docker-compose.yml：

```yaml
services:
  figma-deliver:
    environment:
      - GIN_MODE=release
      - PORT=8080
      - FIGMA_API_RATE_LIMIT=10
      - DB_HOST=mysql
      - DB_PORT=3306
```

### 场景4：Kubernetes 部署

使用 ConfigMap：

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: figma-deliver-config
data:
  GIN_MODE: "release"
  PORT: "8080"
  FIGMA_API_RATE_LIMIT: "10"
```

使用 Secret 存储敏感信息：

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: figma-deliver-secret
type: Opaque
stringData:
  SESSION_SECRET: "your-secret-key"
  DB_PASS: "your-db-password"
```

## 配置验证

服务启动时会输出加载的配置信息：

```
2025/11/18 14:30:00 当前工作目录: /path/to/figma-deliver
2025/11/18 14:30:00 已加载配置: release 模式
2025/11/18 14:30:00 Figma Images API 速率限制: 10 秒
2025/11/18 14:30:00 Services 配置已初始化: Figma API 速率限制=10秒
```

检查日志确认配置是否正确加载。

## 配置修改生效

**重要**：修改配置后必须重启服务才能生效。

```bash
# 停止服务
kill -SIGTERM <pid>

# 或使用 systemd
sudo systemctl restart figma-deliver

# 或使用 Docker
docker-compose restart figma-deliver
```

## 故障排查

### 配置文件未找到

错误信息：
```
YAML配置加载失败，尝试使用 .env 文件
```

解决方法：
1. 确认 `configs/config.yaml` 文件存在
2. 检查文件路径是否正确
3. 使用环境变量作为替代方案

### 配置格式错误

错误信息：
```
解析配置文件失败: yaml: line X: ...
```

解决方法：
1. 检查 YAML 格式是否正确（缩进、冒号、引号等）
2. 使用 YAML 验证工具检查语法
3. 参考 `config.example.yaml` 示例文件

### 速率限制不生效

检查项：
1. 确认配置已正确加载（查看启动日志）
2. 确认已重启服务
3. 检查是否有环境变量覆盖了配置文件的值

## 更多信息

- [Figma API 速率限制详细说明](./figma_api_rate_limit.md)
- [修改日志](./CHANGELOG_FIGMA_API_RATE_LIMIT.md)

## 技术支持

如遇到配置问题，请：
1. 查看服务启动日志
2. 检查配置文件格式
3. 确认环境变量设置
4. 参考示例配置文件

