# 配置文件迁移指南：从 .env 到 YAML

## 概述

项目已从 `.env` 配置文件格式迁移到 `YAML` 格式，以提供更好的结构化配置支持和可读性。

## 变更内容

### 1. 配置文件格式

**旧格式** (`configs/config.env`):
```env
PORT=8080
GIN_MODE=debug
SESSION_SECRET=figma-deliver-secret-key
DB_HOST=localhost
DB_PORT=3306
DB_USER=figma_user
DB_PASS=figma_password
DB_NAME=figma_deliver
EXPORT_DIR=./exports
TEMP_DIR=./temp
```

**新格式** (`configs/config.yaml`):
```yaml
app:
  port: 8080
  gin_mode: debug
  session_secret: figma-deliver-secret-key

database:
  host: localhost
  port: 3306
  user: figma_user
  password: figma_password
  name: figma_deliver

storage:
  export_dir: ./exports
  temp_dir: ./temp
```

### 2. 文件位置

- 配置文件位置：`configs/config.yaml`
- 示例配置：`configs/config.example.yaml`

### 3. 优先级顺序

应用按以下优先级加载配置：

1. **环境变量**（最高优先级）- 用于 Docker/K8s 部署
2. **YAML 配置文件** - 本地开发和默认配置
3. **代码默认值**（最低优先级）

## 迁移步骤

### 本地开发环境

1. 复制示例配置文件：
```bash
cp configs/config.example.yaml configs/config.yaml
```

2. 编辑 `configs/config.yaml`，填入你的实际配置：
```yaml
app:
  port: 8080
  gin_mode: debug
  session_secret: your-secret-key

database:
  host: your-db-host
  port: 3306
  user: your-db-user
  password: your-db-password
  name: figma_deliver

storage:
  export_dir: ./exports
  temp_dir: ./temp
```

3. 删除旧的 `.env` 文件（可选）：
```bash
rm configs/config.env
```

### Docker 环境

Docker Compose 配置无需修改，继续使用环境变量：

```yaml
environment:
  - PORT=8080
  - GIN_MODE=release
  - SESSION_SECRET=your-secret
  - DB_HOST=mysql
  - DB_PORT=3306
  - DB_USER=figma_user
  - DB_PASS=figma_password
  - DB_NAME=figma_deliver
  - EXPORT_DIR=/app/exports
  - TEMP_DIR=/app/temp
```

环境变量会自动覆盖 YAML 配置文件中的值。

### Kubernetes 环境

K8s 部署同样使用 ConfigMap 和 Secret 提供环境变量，无需修改：

```yaml
env:
  - name: PORT
    valueFrom:
      configMapKeyRef:
        name: figma-deliver-config
        key: PORT
  - name: DB_USER
    valueFrom:
      secretKeyRef:
        name: figma-deliver-secret
        key: DB_USER
  # ... 其他环境变量
```

## 优势

1. **更好的结构化** - YAML 支持层级结构，配置更清晰
2. **更好的可读性** - 缩进和分组使配置更容易理解
3. **类型安全** - 可以更好地进行配置验证
4. **向后兼容** - 仍然支持环境变量覆盖
5. **开发友好** - 支持注释和更复杂的配置结构

## 配置查找顺序

应用会按以下顺序查找配置文件：

1. `configs/config.yaml`
2. `./configs/config.yaml`
3. `config.yaml`

如果没有找到配置文件，应用将报错并退出。

## 环境变量支持

以下环境变量可以覆盖 YAML 配置：

- `PORT` - 应用端口
- `GIN_MODE` - Gin 运行模式（debug/release）
- `SESSION_SECRET` - 会话密钥
- `DB_HOST` - 数据库主机
- `DB_PORT` - 数据库端口
- `DB_USER` - 数据库用户
- `DB_PASS` - 数据库密码
- `DB_NAME` - 数据库名称
- `EXPORT_DIR` - 导出目录
- `TEMP_DIR` - 临时目录

## 依赖更新

项目 `go.mod` 已更新，添加了 YAML 解析库：

```go
require (
    // ...
    gopkg.in/yaml.v3 v3.0.1
    // ...
)
```

更新依赖：
```bash
go mod tidy
go mod download
```

## 测试配置

运行应用测试配置是否正确加载：

```bash
go run main.go
```

你应该看到类似的日志输出：
```
当前工作目录: /path/to/project
正在加载配置文件: configs/config.yaml
已加载配置: debug 模式
服务器启动在 http://localhost:8080
```

## 故障排查

### 问题：找不到配置文件

**错误信息**：
```
加载配置文件失败: 未找到配置文件，请确保 config.yaml 存在于 configs 目录
```

**解决方法**：
1. 确保 `configs/config.yaml` 文件存在
2. 检查文件路径和文件名是否正确
3. 使用示例配置：`cp configs/config.example.yaml configs/config.yaml`

### 问题：YAML 格式错误

**错误信息**：
```
解析配置文件失败: yaml: ...
```

**解决方法**：
1. 检查 YAML 文件的缩进（使用空格，不使用 Tab）
2. 确保所有的键值对格式正确
3. 参考 `configs/config.example.yaml` 示例文件

### 问题：数据库连接失败

**解决方法**：
1. 检查 `database` 配置部分是否正确
2. 确保数据库服务正在运行
3. 验证数据库凭据是否正确

## 回滚到旧配置

如果需要临时回滚到 `.env` 格式，可以：

1. 通过环境变量提供所有配置
2. 或者恢复 `godotenv` 的使用（需要修改代码）

建议使用环境变量方式，不需要修改代码：

```bash
export PORT=8080
export GIN_MODE=debug
export SESSION_SECRET=your-secret
# ... 设置其他环境变量
go run main.go
```

## 支持

如有问题，请查看：
- 配置示例：`configs/config.example.yaml`
- 应用日志输出
- 环境变量设置

