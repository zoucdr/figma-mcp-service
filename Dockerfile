# Figma Deliver Service Dockerfile - 适用于Kubernetes环境
# 基于Go 1.20构建的Figma设计资源导出服务

# 构建阶段
FROM golang:1.20-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装必要的工具 (使用清华大学镜像源)
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.tuna.tsinghua.edu.cn/g' /etc/apk/repositories && \
    apk update && apk add --no-cache git ca-certificates tzdata

# 复制 go mod 文件（优先复制以利用Docker缓存）
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源码
COPY . .

# 确保模块正确初始化并验证
RUN go mod tidy && go mod verify

# 构建应用 - 优化K8s环境下的二进制大小和启动性能
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -a -installsuffix cgo \
    -ldflags="-w -s -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo 'v1.0.0')" \
    -o figma-deliver ./main.go

# 运行阶段 - 使用精简的基础镜像以减小体积，提高K8s部署速度
FROM alpine:3.19

# 添加Kubernetes相关标签
LABEL app.kubernetes.io/name="figma-deliver" \
      app.kubernetes.io/version="1.0" \
      app.kubernetes.io/component="figma-service" \
      app.kubernetes.io/part-of="figma-deliver-system" \
      app.kubernetes.io/managed-by="docker" \
      maintainer="Figma Deliver Team"

# 安装必要的运行时依赖 (使用清华大学镜像源)
# 添加curl用于健康检查，ca-certificates用于HTTPS，tzdata用于时区
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.tuna.tsinghua.edu.cn/g' /etc/apk/repositories && \
    apk update && \
    apk --no-cache add ca-certificates tzdata curl && \
    update-ca-certificates

# 设置时区
ENV TZ=Asia/Shanghai

# 创建非root用户 - 使用固定UID/GID以便在K8s中设置安全上下文
RUN addgroup -g 1001 appgroup && \
    adduser -D -u 1001 -G appgroup appuser

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/figma-deliver .

# 复制配置文件（YAML格式）
COPY --from=builder /app/configs/config.yaml ./configs/

# 复制Web静态资源和模板
COPY --from=builder /app/web ./web

# 复制入口点脚本
COPY docker-entrypoint.sh /app/
RUN chmod +x /app/docker-entrypoint.sh

# 创建必要的目录并设置适当的权限
# 使用更严格的权限设置，遵循最小权限原则
RUN mkdir -p logs exports temp data && \
    chown -R appuser:appgroup /app && \
    chmod -R 750 /app && \
    chmod 770 /app/logs && \
    chmod 770 /app/exports && \
    chmod 770 /app/temp && \
    chmod 770 /app/data && \
    chmod 500 /app/figma-deliver && \
    chmod 440 /app/configs/config.yaml

# 切换到非root用户
USER appuser

# 暴露端口
EXPOSE 8080

# 健康检查 - K8s会使用自己的探针，但保留Docker健康检查以便兼容
# 在K8s中，应该使用livenessProbe和readinessProbe代替
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/about || exit 1

# K8s环境变量
ENV K8S_READY=true

# 默认环境变量
ENV PORT=8080
ENV GIN_MODE=release
ENV EXPORT_DIR=/app/exports
ENV TEMP_DIR=/app/temp

# 入口点和启动命令
ENTRYPOINT ["/bin/sh", "/app/docker-entrypoint.sh"]
CMD ["./figma-deliver"]
