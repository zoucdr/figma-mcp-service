#!/bin/bash

# Figma Deliver 构建脚本
# 用于构建 Docker 镜像和部署

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 默认配置
IMAGE_NAME="figma-deliver"
IMAGE_TAG="latest"
REGISTRY=""
PUSH_IMAGE=false
DEPLOY_K8S=false

# 显示帮助信息
show_help() {
    echo "Figma Deliver 构建脚本"
    echo ""
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  -n, --name NAME        镜像名称 (默认: figma-deliver)"
    echo "  -t, --tag TAG          镜像标签 (默认: latest)"
    echo "  -r, --registry REG     镜像仓库地址"
    echo "  -p, --push             构建后推送镜像"
    echo "  -k, --k8s              部署到 Kubernetes"
    echo "  -h, --help             显示帮助信息"
    echo ""
    echo "示例:"
    echo "  $0                                    # 构建本地镜像"
    echo "  $0 -t v1.0.0 -p                     # 构建并推送 v1.0.0 版本"
    echo "  $0 -r registry.example.com -p -k    # 构建、推送并部署到 K8s"
}

# 解析命令行参数
while [[ $# -gt 0 ]]; do
    case $1 in
        -n|--name)
            IMAGE_NAME="$2"
            shift 2
            ;;
        -t|--tag)
            IMAGE_TAG="$2"
            shift 2
            ;;
        -r|--registry)
            REGISTRY="$2"
            shift 2
            ;;
        -p|--push)
            PUSH_IMAGE=true
            shift
            ;;
        -k|--k8s)
            DEPLOY_K8S=true
            shift
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            echo -e "${RED}未知选项: $1${NC}"
            show_help
            exit 1
            ;;
    esac
done

# 构建完整镜像名称
if [ -n "$REGISTRY" ]; then
    FULL_IMAGE_NAME="${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}"
else
    FULL_IMAGE_NAME="${IMAGE_NAME}:${IMAGE_TAG}"
fi

echo -e "${BLUE}=== Figma Deliver 构建脚本 ===${NC}"
echo -e "${BLUE}镜像名称: ${FULL_IMAGE_NAME}${NC}"
echo -e "${BLUE}推送镜像: ${PUSH_IMAGE}${NC}"
echo -e "${BLUE}部署 K8s: ${DEPLOY_K8S}${NC}"
echo ""

# 检查 Docker 是否可用
if ! command -v docker &> /dev/null; then
    echo -e "${RED}错误: Docker 未安装或不可用${NC}"
    exit 1
fi

# 检查 Dockerfile 是否存在
if [ ! -f "Dockerfile" ]; then
    echo -e "${RED}错误: 未找到 Dockerfile${NC}"
    exit 1
fi

# 构建镜像
echo -e "${YELLOW}开始构建 Docker 镜像...${NC}"
docker build -t "$FULL_IMAGE_NAME" .

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ 镜像构建成功: ${FULL_IMAGE_NAME}${NC}"
else
    echo -e "${RED}✗ 镜像构建失败${NC}"
    exit 1
fi

# 推送镜像
if [ "$PUSH_IMAGE" = true ]; then
    if [ -z "$REGISTRY" ]; then
        echo -e "${YELLOW}警告: 未指定镜像仓库，跳过推送${NC}"
    else
        echo -e "${YELLOW}推送镜像到仓库...${NC}"
        docker push "$FULL_IMAGE_NAME"
        
        if [ $? -eq 0 ]; then
            echo -e "${GREEN}✓ 镜像推送成功${NC}"
        else
            echo -e "${RED}✗ 镜像推送失败${NC}"
            exit 1
        fi
    fi
fi

# 部署到 Kubernetes
if [ "$DEPLOY_K8S" = true ]; then
    if ! command -v kubectl &> /dev/null; then
        echo -e "${RED}错误: kubectl 未安装或不可用${NC}"
        exit 1
    fi
    
    if [ ! -f "k8s-deployment.yaml" ]; then
        echo -e "${RED}错误: 未找到 k8s-deployment.yaml${NC}"
        exit 1
    fi
    
    echo -e "${YELLOW}部署到 Kubernetes...${NC}"
    
    # 更新部署文件中的镜像名称
    sed -i.bak "s|image: figma-deliver:latest|image: ${FULL_IMAGE_NAME}|g" k8s-deployment.yaml
    
    # 应用部署配置
    kubectl apply -f k8s-deployment.yaml
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ Kubernetes 部署成功${NC}"
        
        # 等待部署完成
        echo -e "${YELLOW}等待部署就绪...${NC}"
        kubectl rollout status deployment/figma-deliver --timeout=300s
        
        # 显示服务状态
        echo -e "${BLUE}服务状态:${NC}"
        kubectl get pods -l app=figma-deliver
        kubectl get svc figma-deliver-service
        
    else
        echo -e "${RED}✗ Kubernetes 部署失败${NC}"
        exit 1
    fi
    
    # 恢复原始部署文件
    mv k8s-deployment.yaml.bak k8s-deployment.yaml
fi

echo ""
echo -e "${GREEN}=== 构建完成 ===${NC}"
echo -e "${GREEN}镜像: ${FULL_IMAGE_NAME}${NC}"

# 显示镜像信息
echo ""
echo -e "${BLUE}镜像信息:${NC}"
docker images | grep "$IMAGE_NAME" | head -5

# 显示后续操作建议
echo ""
echo -e "${BLUE}后续操作:${NC}"
echo "  # 本地运行"
echo "  docker run -p 8080:8080 $FULL_IMAGE_NAME"
echo ""
echo "  # 使用 Docker Compose"
echo "  docker-compose up -d"
echo ""
if [ "$DEPLOY_K8S" = true ]; then
    echo "  # 查看 K8s 服务状态"
    echo "  kubectl get all -l app=figma-deliver"
    echo ""
    echo "  # 查看日志"
    echo "  kubectl logs -f deployment/figma-deliver"
    echo ""
fi
