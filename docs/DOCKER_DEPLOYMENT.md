# Figma Deliver Docker 部署指南

本文档介绍如何使用 Docker 和 Docker Compose 部署 Figma Deliver 服务。

## 快速开始

### 1. 构建和启动服务

```bash
# 克隆项目（如果还没有）
git clone <your-repo-url>
cd figma-deliver/service

# 使用 Docker Compose 启动所有服务
docker-compose up -d

# 查看服务状态
docker-compose ps

# 查看日志
docker-compose logs -f figma-deliver
```

### 2. 访问服务

- **Web 界面**: http://localhost:8080
- **健康检查**: http://localhost:8080/about
- **MySQL 数据库**: localhost:3306
- **Redis 缓存**: localhost:6379

### 3. 默认登录信息

- **用户名**: admin
- **密码**: admin123

## 详细配置

### 环境变量配置

在 `docker-compose.yml` 中可以配置以下环境变量：

```yaml
environment:
  # 服务配置
  - PORT=8080                    # 服务端口
  - GIN_MODE=release            # Gin 模式 (debug/release)
  - SESSION_SECRET=your-secret  # 会话密钥（生产环境请修改）
  
  # 数据库配置
  - DB_HOST=mysql              # 数据库主机
  - DB_PORT=3306               # 数据库端口
  - DB_USER=figma_user         # 数据库用户
  - DB_PASS=figma_password     # 数据库密码
  - DB_NAME=figma_deliver      # 数据库名称
  
  # 文件存储配置
  - EXPORT_DIR=/app/exports    # 导出文件目录
  - TEMP_DIR=/app/temp         # 临时文件目录
```

### 持久化存储

Docker Compose 配置了以下持久化卷：

- `mysql_data`: MySQL 数据库数据
- `redis_data`: Redis 缓存数据
- `figma_exports`: 导出的设计文件
- `figma_temp`: 临时文件
- `figma_data`: 应用数据
- `figma_logs`: 应用日志

## 生产环境部署

### 1. 安全配置

在生产环境中，请务必修改以下配置：

```yaml
environment:
  # 修改默认密钥
  - SESSION_SECRET=your-very-secure-secret-key
  
  # 使用强密码
  - MYSQL_ROOT_PASSWORD=your-strong-root-password
  - MYSQL_PASSWORD=your-strong-user-password
```

### 2. 网络配置

```yaml
# 生产环境建议使用外部网络
networks:
  figma-network:
    external: true
```

### 3. 资源限制

```yaml
services:
  figma-deliver:
    deploy:
      resources:
        limits:
          cpus: '2.0'
          memory: 2G
        reservations:
          cpus: '0.5'
          memory: 512M
```

## Kubernetes 部署

### 1. 构建镜像

```bash
# 构建镜像
docker build -t figma-deliver:latest .

# 推送到镜像仓库
docker tag figma-deliver:latest your-registry/figma-deliver:latest
docker push your-registry/figma-deliver:latest
```

### 2. Kubernetes 配置示例

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: figma-deliver
spec:
  replicas: 3
  selector:
    matchLabels:
      app: figma-deliver
  template:
    metadata:
      labels:
        app: figma-deliver
    spec:
      containers:
      - name: figma-deliver
        image: your-registry/figma-deliver:latest
        ports:
        - containerPort: 8080
        env:
        - name: DB_HOST
          value: "mysql-service"
        - name: DB_USER
          valueFrom:
            secretKeyRef:
              name: mysql-secret
              key: username
        - name: DB_PASS
          valueFrom:
            secretKeyRef:
              name: mysql-secret
              key: password
        livenessProbe:
          httpGet:
            path: /about
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /about
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
        resources:
          limits:
            cpu: 2000m
            memory: 2Gi
          requests:
            cpu: 500m
            memory: 512Mi
```

## 常用命令

### Docker Compose 命令

```bash
# 启动服务
docker-compose up -d

# 停止服务
docker-compose down

# 重启服务
docker-compose restart figma-deliver

# 查看日志
docker-compose logs -f figma-deliver

# 进入容器
docker-compose exec figma-deliver sh

# 重新构建镜像
docker-compose build --no-cache

# 清理未使用的资源
docker system prune -a
```

### 数据库管理

```bash
# 连接到 MySQL
docker-compose exec mysql mysql -u figma_user -p figma_deliver

# 备份数据库
docker-compose exec mysql mysqldump -u figma_user -p figma_deliver > backup.sql

# 恢复数据库
docker-compose exec -T mysql mysql -u figma_user -p figma_deliver < backup.sql
```

## 故障排除

### 1. 服务无法启动

```bash
# 查看详细日志
docker-compose logs figma-deliver

# 检查容器状态
docker-compose ps

# 检查网络连接
docker-compose exec figma-deliver ping mysql
```

### 2. 数据库连接问题

```bash
# 检查数据库是否就绪
docker-compose exec mysql mysqladmin ping -h localhost -u figma_user -p

# 查看数据库日志
docker-compose logs mysql
```

### 3. 权限问题

```bash
# 检查文件权限
docker-compose exec figma-deliver ls -la /app

# 修复权限（如果需要）
docker-compose exec --user root figma-deliver chown -R appuser:appgroup /app
```

## 监控和日志

### 1. 健康检查

服务提供了内置的健康检查端点：

- **Docker**: 自动使用 `/about` 端点进行健康检查
- **Kubernetes**: 配置 livenessProbe 和 readinessProbe

### 2. 日志收集

```bash
# 实时查看日志
docker-compose logs -f figma-deliver

# 导出日志到文件
docker-compose logs figma-deliver > figma-deliver.log
```

### 3. 性能监控

建议在生产环境中集成以下监控工具：

- **Prometheus**: 指标收集
- **Grafana**: 可视化监控
- **ELK Stack**: 日志分析

## 更新和维护

### 1. 更新服务

```bash
# 拉取最新代码
git pull

# 重新构建并启动
docker-compose up -d --build

# 清理旧镜像
docker image prune -f
```

### 2. 数据备份

```bash
# 创建备份脚本
cat > backup.sh << 'EOF'
#!/bin/bash
DATE=$(date +%Y%m%d_%H%M%S)
docker-compose exec mysql mysqldump -u figma_user -p figma_deliver > backup_${DATE}.sql
docker-compose exec figma-deliver tar -czf exports_${DATE}.tar.gz /app/exports
EOF

chmod +x backup.sh
./backup.sh
```

## 支持

如果遇到问题，请：

1. 查看本文档的故障排除部分
2. 检查 GitLab Issues
3. 提交新的 Issue 并附上详细的错误日志
