#!/bin/sh

# Figma Deliver Service Docker 入口点脚本
# 用于初始化容器环境和启动服务

set -e

echo "=== Figma Deliver Service 启动中 ==="
echo "时间: $(date)"
echo "工作目录: $(pwd)"
echo "用户: $(whoami)"

# 检查必要的目录
echo "检查必要目录..."
for dir in logs exports temp data; do
    if [ ! -d "/app/$dir" ]; then
        echo "创建目录: /app/$dir"
        mkdir -p "/app/$dir"
    fi
done

# 检查配置文件
echo "检查配置文件..."
if [ ! -f "/app/configs/config.env" ]; then
    if [ -f "/app/configs/config.example.env" ]; then
        echo "使用示例配置文件作为默认配置"
        cp /app/configs/config.example.env /app/configs/config.env
    else
        echo "警告: 未找到配置文件，将使用环境变量或默认值"
    fi
fi

# 显示关键环境变量
echo "=== 环境变量配置 ==="
echo "PORT: ${PORT:-8080}"
echo "GIN_MODE: ${GIN_MODE:-release}"
echo "DB_HOST: ${DB_HOST:-localhost}"
echo "DB_PORT: ${DB_PORT:-3306}"
echo "DB_NAME: ${DB_NAME:-figma_deliver}"
echo "EXPORT_DIR: ${EXPORT_DIR:-/app/exports}"
echo "TEMP_DIR: ${TEMP_DIR:-/app/temp}"

# 等待数据库连接（如果配置了数据库）
if [ -n "$DB_HOST" ] && [ "$DB_HOST" != "localhost" ]; then
    echo "等待数据库连接: $DB_HOST:$DB_PORT"
    timeout=30
    while ! nc -z "$DB_HOST" "$DB_PORT" 2>/dev/null; do
        timeout=$((timeout - 1))
        if [ $timeout -le 0 ]; then
            echo "警告: 数据库连接超时，继续启动服务"
            break
        fi
        echo "等待数据库连接... (剩余 ${timeout}s)"
        sleep 1
    done
    
    if nc -z "$DB_HOST" "$DB_PORT" 2>/dev/null; then
        echo "数据库连接成功"
    fi
fi

# 检查二进制文件
if [ ! -f "/app/figma-deliver-service" ]; then
    echo "错误: 未找到服务二进制文件"
    exit 1
fi

if [ ! -x "/app/figma-deliver-service" ]; then
    echo "错误: 服务二进制文件不可执行"
    exit 1
fi

# 检查Web资源
if [ ! -d "/app/web" ]; then
    echo "警告: 未找到Web资源目录"
fi

echo "=== 启动 Figma Deliver Service ==="
echo "命令: $@"

# 执行传入的命令
exec "$@"
