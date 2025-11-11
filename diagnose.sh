#!/bin/bash

echo "=== Figma Deliver Docker 诊断脚本 ==="
echo ""

echo "1. 当前工作目录:"
pwd
echo ""

echo "2. 检查 Dockerfile 是否存在:"
if [ -f "Dockerfile" ]; then
    echo "✅ Dockerfile 存在"
    echo "文件大小: $(wc -l < Dockerfile) 行"
    echo "前几行内容:"
    head -10 Dockerfile
else
    echo "❌ Dockerfile 不存在"
fi
echo ""

echo "3. 检查 go.mod 是否存在:"
if [ -f "go.mod" ]; then
    echo "✅ go.mod 存在"
    echo "模块名: $(head -1 go.mod)"
else
    echo "❌ go.mod 不存在"
fi
echo ""

echo "4. 检查 go.sum 是否存在:"
if [ -f "go.sum" ]; then
    echo "✅ go.sum 存在"
    echo "依赖数量: $(wc -l < go.sum) 行"
else
    echo "❌ go.sum 不存在"
fi
echo ""

echo "5. 检查主程序入口:"
if [ -f "cmd/main.go" ]; then
    echo "✅ cmd/main.go 存在"
else
    echo "❌ cmd/main.go 不存在"
fi
echo ""

echo "6. 检查 .dockerignore:"
if [ -f ".dockerignore" ]; then
    echo "✅ .dockerignore 存在"
    echo "忽略规则数量: $(wc -l < .dockerignore) 行"
else
    echo "❌ .dockerignore 不存在"
fi
echo ""

echo "7. 检查 Docker 构建上下文:"
echo "当前目录文件列表:"
ls -la | head -20
echo ""

echo "8. 验证 Go 模块:"
if command -v go &> /dev/null; then
    echo "Go 版本: $(go version)"
    if [ -f "go.mod" ]; then
        echo "模块验证:"
        go mod verify 2>&1 || echo "模块验证失败"
    fi
else
    echo "Go 未安装"
fi
echo ""

echo "9. 推荐的构建命令:"
echo "docker build -t figma-deliver:latest ."
echo ""

echo "10. 如果问题仍然存在，请检查:"
echo "- 是否在正确的目录 (service/) 中运行构建"
echo "- 是否有其他 Dockerfile 在父目录或其他位置"
echo "- 是否使用了 Docker Compose 或其他构建工具"
echo "- CI/CD 系统是否使用了不同的 Dockerfile"
