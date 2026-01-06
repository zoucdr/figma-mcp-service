# Quick test for background reconnection
# 测试 Figma 插件后台重连功能

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Figma 插件后台重连测试" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

Write-Host "本脚本将帮助你测试 Figma 插件的后台重连功能" -ForegroundColor Yellow
Write-Host ""

# Check if server is running
Write-Host "[1/3] 检查 WebSocket 服务器状态..." -ForegroundColor Green
$serverRunning = $false
try {
    $response = Invoke-WebRequest -Uri "http://localhost:8080/health" -TimeoutSec 2 -ErrorAction SilentlyContinue
    if ($response.StatusCode -eq 200) {
        $serverRunning = $true
        Write-Host "✅ 服务器正在运行" -ForegroundColor Green
    }
} catch {
    Write-Host "❌ 服务器未运行" -ForegroundColor Red
    Write-Host "请先启动服务器: .\startup.bat" -ForegroundColor Yellow
    exit 1
}

Write-Host ""

# Instructions
Write-Host "[2/3] 测试准备清单:" -ForegroundColor Green
Write-Host "  ✓ WebSocket 服务器已启动" -ForegroundColor Gray
Write-Host "  □ 在 Figma 中打开插件" -ForegroundColor Yellow
Write-Host "  □ 连接到 WebSocket 服务器" -ForegroundColor Yellow
Write-Host "  □ 打开浏览器开发者工具（查看日志）" -ForegroundColor Yellow
Write-Host ""

$ready = Read-Host "准备好开始测试了吗? (y/n)"
if ($ready -ne "y" -and $ready -ne "Y") {
    Write-Host "测试已取消" -ForegroundColor Yellow
    exit 0
}

Write-Host ""
Write-Host "[3/3] 开始测试..." -ForegroundColor Green
Write-Host ""

# Test scenarios
$scenarios = @(
    @{
        Name = "场景 1: 短时间后台切换"
        Steps = @(
            "1. 确认插件已连接到服务器",
            "2. 切换到其他应用（例如浏览器、VSCode）",
            "3. 等待 10 秒",
            "4. 切回 Figma",
            "5. 检查连接是否自动恢复"
        )
        Expected = "连接应该立即恢复，无需手动重连"
    },
    @{
        Name = "场景 2: 长时间后台挂起"
        Steps = @(
            "1. 确认插件已连接到服务器",
            "2. 切换到其他应用",
            "3. 等待 5 分钟",
            "4. 切回 Figma",
            "5. 检查是否自动重连"
        )
        Expected = "应该自动检测到连接断开并重连"
    },
    @{
        Name = "场景 3: 服务器断开重连"
        Steps = @(
            "1. 确认插件已连接到服务器",
            "2. 按 Ctrl+C 停止此脚本（服务器仍在运行）",
            "3. 在插件中观察重连行为",
            "4. 查看控制台日志"
        )
        Expected = "应该看到心跳失败和自动重连日志"
    }
)

$currentScenario = 1
foreach ($scenario in $scenarios) {
    Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Cyan
    Write-Host "$($scenario.Name)" -ForegroundColor Cyan
    Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Cyan
    Write-Host ""
    
    Write-Host "测试步骤:" -ForegroundColor Yellow
    foreach ($step in $scenario.Steps) {
        Write-Host "  $step" -ForegroundColor Gray
    }
    Write-Host ""
    
    Write-Host "预期结果:" -ForegroundColor Yellow
    Write-Host "  $($scenario.Expected)" -ForegroundColor Gray
    Write-Host ""
    
    if ($currentScenario -lt $scenarios.Count) {
        $continue = Read-Host "完成测试后按 Enter 继续下一个场景..."
    }
    
    Write-Host ""
    $currentScenario++
}

Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Cyan
Write-Host "测试完成！" -ForegroundColor Green
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Cyan
Write-Host ""

Write-Host "查看详细测试指南:" -ForegroundColor Yellow
Write-Host "  demo\test_background_reconnect.md" -ForegroundColor Cyan
Write-Host ""

Write-Host "查看技术实现文档:" -ForegroundColor Yellow
Write-Host "  figma-plugin\WEBSOCKET_RECONNECT.md" -ForegroundColor Cyan
Write-Host ""

Write-Host "有任何问题请查看控制台日志或参考文档" -ForegroundColor Gray














