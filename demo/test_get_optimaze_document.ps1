# 测试 get_optimaze_document 工具
# 这个脚本用于测试获取优化后的文档数据功能

$baseUrl = "http://localhost:8080"

# 从环境变量或配置文件读取Token
$token = $env:FIGMA_TEST_TOKEN
if (-not $token) {
    Write-Host "请设置环境变量 FIGMA_TEST_TOKEN" -ForegroundColor Red
    exit 1
}

# 项目ID（需要替换为实际的项目ID）
$projectId = 1

Write-Host "`n========== 测试 get_optimaze_document 工具 ==========" -ForegroundColor Cyan

# 1. 测试基本调用（使用默认参数）
Write-Host "`n[测试1] 获取优化文档（默认参数）..." -ForegroundColor Yellow
$body1 = @{
    command = "get_optimaze_document"
    params = @{
        project_id = $projectId
    }
} | ConvertTo-Json

try {
    $response1 = Invoke-RestMethod -Uri "$baseUrl/api/mcp/tools/simulate" `
        -Method POST `
        -Headers @{
            "Authorization" = "Bearer $token"
            "Content-Type" = "application/json"
        } `
        -Body $body1
    
    Write-Host "✓ 测试1成功" -ForegroundColor Green
    Write-Host "响应内容:" -ForegroundColor Cyan
    $response1 | ConvertTo-Json -Depth 10
} catch {
    Write-Host "✗ 测试1失败: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host "详细错误: $($_.ErrorDetails.Message)" -ForegroundColor Red
}

# 2. 测试指定格式和缩放
Write-Host "`n[测试2] 获取优化文档（指定格式jpg，缩放2.0）..." -ForegroundColor Yellow
$body2 = @{
    command = "get_optimaze_document"
    params = @{
        project_id = $projectId
        format = "jpg"
        scale = 2.0
    }
} | ConvertTo-Json

try {
    $response2 = Invoke-RestMethod -Uri "$baseUrl/api/mcp/tools/simulate" `
        -Method POST `
        -Headers @{
            "Authorization" = "Bearer $token"
            "Content-Type" = "application/json"
        } `
        -Body $body2
    
    Write-Host "✓ 测试2成功" -ForegroundColor Green
    Write-Host "响应内容:" -ForegroundColor Cyan
    $response2 | ConvertTo-Json -Depth 10
} catch {
    Write-Host "✗ 测试2失败: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host "详细错误: $($_.ErrorDetails.Message)" -ForegroundColor Red
}

# 3. 测试获取工具列表（验证新工具是否已添加）
Write-Host "`n[测试3] 验证工具列表中是否包含 get_optimaze_document..." -ForegroundColor Yellow
try {
    $toolsList = Invoke-RestMethod -Uri "$baseUrl/api/mcp/tools/list" `
        -Method GET `
        -Headers @{
            "Authorization" = "Bearer $token"
        }
    
    $tool = $toolsList.tools | Where-Object { $_.name -eq "get_optimaze_document" }
    
    if ($tool) {
        Write-Host "✓ 工具已成功添加到列表中" -ForegroundColor Green
        Write-Host "工具定义:" -ForegroundColor Cyan
        $tool | ConvertTo-Json -Depth 10
    } else {
        Write-Host "✗ 工具未找到" -ForegroundColor Red
    }
} catch {
    Write-Host "✗ 测试3失败: $($_.Exception.Message)" -ForegroundColor Red
}

Write-Host "`n========== 测试完成 ==========" -ForegroundColor Cyan

