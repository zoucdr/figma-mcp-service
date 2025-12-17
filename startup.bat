@echo off
echo ===== Figma Deliver 启动脚本 =====
echo.

echo 1. 查找并结束现有的 figma-deliver 进程...
taskkill /F /IM figma-deliver.exe >nul 2>&1
if %ERRORLEVEL% EQU 0 (
    echo    已结束现有的 figma-deliver 进程
) else (
    echo    未发现正在运行的 figma-deliver 进程
)
echo.

echo 2. 查找并结束占用 8080 端口的进程...
for /f "tokens=5" %%a in ('netstat -ano ^| findstr :8080 ^| findstr LISTENING') do (
    if not "%%a" == "" (
        echo    发现进程 PID: %%a 占用端口 8080，正在结束...
        taskkill /F /PID %%a >nul 2>&1
        if %ERRORLEVEL% EQU 0 (
            echo    成功结束进程 PID: %%a
        ) else (
            echo    无法结束进程 PID: %%a，可能需要管理员权限
        )
    )
)
echo.

echo 3. 检查是否存在可执行文件...
go build -o figma-deliver.exe ./cmd
echo.

echo 4. 启动应用程序...
echo    应用程序已启动，请访问 http://localhost:8080
echo    按 Ctrl+C 可以终止程序
echo.
echo ===== 应用程序日志 =====
figma-deliver.exe
