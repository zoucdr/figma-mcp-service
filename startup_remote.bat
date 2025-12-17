@echo off
chcp 65001
echo ====================================
echo 启动 Figma Deliver (远程数据库)
echo ====================================
echo.

REM 复制远程数据库配置
copy /Y "configs\config copy.yaml" "configs\config.yaml"

echo 使用配置: configs\config copy.yaml
echo 数据库: 10.84.97.48:3306
echo.

echo 启动服务...
figma-deliver.exe

pause

