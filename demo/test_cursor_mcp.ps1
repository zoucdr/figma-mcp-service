# Cursor MCP 接口测试脚本
# 用于测试无认证的HTTP MCP接口

param(
    [string]$BaseUrl = "http://localhost:8080",
    [string]$Token = "",
    [string]$ProjectId = "test_project_123",
    [string]$NodeId = "test_node_456"
)

Write-Host "=== Cursor MCP 接口测试 ===" -ForegroundColor Green
Write-Host "基础URL: $BaseUrl" -ForegroundColor Yellow
Write-Host "Token: $Token" -ForegroundColor Yellow
Write-Host "项目ID: $ProjectId" -ForegroundColor Yellow
Write-Host "节点ID: $NodeId" -ForegroundColor Yellow
Write-Host ""

if ([string]::IsNullOrEmpty($Token)) {
    Write-Host "错误: 请提供Token参数" -ForegroundColor Red
    Write-Host "用法: .\test_cursor_mcp.ps1 -Token 'your_token_here'" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "获取Token的步骤:" -ForegroundColor Cyan
    Write-Host "1. 访问 $BaseUrl" -ForegroundColor White
    Write-Host "2. 登录您的账户" -ForegroundColor White
    Write-Host "3. 进入项目页面，点击MCP连接状态按钮" -ForegroundColor White
    Write-Host "4. 点击'连接MCP'按钮创建连接" -ForegroundColor White
    Write-Host "5. 复制生成的连接ID作为Token使用" -ForegroundColor White
    exit 1
}

# 测试函数
function Test-CursorMCPEndpoint {
    param(
        [string]$Method,
        [string]$Endpoint,
        [string]$Description,
        [hashtable]$Body = $null
    )
    
    Write-Host "测试: $Description" -ForegroundColor Cyan
    Write-Host "请求: $Method $Endpoint" -ForegroundColor Gray
    
    try {
        $uri = "$BaseUrl/mcp/$Token$Endpoint"
        
        if ($Body) {
            $jsonBody = $Body | ConvertTo-Json -Depth 10
            Write-Host "请求体: $jsonBody" -ForegroundColor Gray
            
            $response = Invoke-RestMethod -Uri $uri -Method $Method -Body $jsonBody -ContentType "application/json" -ErrorAction Stop
        } else {
            $response = Invoke-RestMethod -Uri $uri -Method $Method -ErrorAction Stop
        }
        
        Write-Host "✓ 成功" -ForegroundColor Green
        Write-Host "响应: $($response | ConvertTo-Json -Depth 5)" -ForegroundColor White
        Write-Host ""
        return $true
    }
    catch {
        Write-Host "✗ 失败" -ForegroundColor Red
        Write-Host "错误: $($_.Exception.Message)" -ForegroundColor Red
        if ($_.Exception.Response) {
            $statusCode = $_.Exception.Response.StatusCode.value__
            Write-Host "状态码: $statusCode" -ForegroundColor Red
        }
        Write-Host ""
        return $false
    }
}

# 开始测试
Write-Host "开始测试 Cursor MCP 接口..." -ForegroundColor Green
Write-Host ""

$testResults = @()

# 1. 测试连接状态
$testResults += Test-CursorMCPEndpoint -Method "GET" -Endpoint "/status" -Description "检查连接状态"

# 2. 测试获取预览图
$testResults += Test-CursorMCPEndpoint -Method "GET" -Endpoint "/preview/$ProjectId/$NodeId" -Description "获取预览图"

# 3. 测试获取节点配置
$testResults += Test-CursorMCPEndpoint -Method "GET" -Endpoint "/config/$ProjectId/$NodeId" -Description "获取节点配置"

# 4. 测试设置节点配置
$configData = @{
    width = 800
    height = 600
    background = "#FFFFFF"
    border = "1px solid #E0E0E0"
    padding = "16px"
}
$testResults += Test-CursorMCPEndpoint -Method "POST" -Endpoint "/config/$ProjectId/$NodeId" -Description "设置节点配置" -Body $configData

# 5. 测试获取默认配置提示词
$testResults += Test-CursorMCPEndpoint -Method "GET" -Endpoint "/prompt/$ProjectId/$NodeId" -Description "获取默认配置提示词"

# 6. 测试获取UI配置提示词
$testResults += Test-CursorMCPEndpoint -Method "GET" -Endpoint "/prompt/$ProjectId/$NodeId?type=ui" -Description "获取UI配置提示词"

# 7. 测试获取布局配置提示词
$testResults += Test-CursorMCPEndpoint -Method "GET" -Endpoint "/prompt/$ProjectId/$NodeId?type=layout" -Description "获取布局配置提示词"

# 测试结果汇总
Write-Host "=== 测试结果汇总 ===" -ForegroundColor Green
$successCount = ($testResults | Where-Object { $_ -eq $true }).Count
$totalCount = $testResults.Count

Write-Host "总测试数: $totalCount" -ForegroundColor White
Write-Host "成功: $successCount" -ForegroundColor Green
Write-Host "失败: $($totalCount - $successCount)" -ForegroundColor Red

if ($successCount -eq $totalCount) {
    Write-Host ""
    Write-Host "🎉 所有测试通过！Cursor MCP接口工作正常。" -ForegroundColor Green
    Write-Host ""
    Write-Host "现在您可以在Cursor中使用以下URL配置MCP:" -ForegroundColor Cyan
    Write-Host "$BaseUrl/mcp/$Token" -ForegroundColor Yellow
} else {
    Write-Host ""
    Write-Host "⚠️  部分测试失败，请检查服务器状态和Token有效性。" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "测试完成。" -ForegroundColor Green
