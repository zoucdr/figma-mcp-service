# MCP功能测试脚本
# 用于测试MCP服务的各项功能

$baseUrl = "http://localhost:8080"
$username = "test_user"
$password = "test_password"
$figmaToken = "test_figma_token"

Write-Host "=== MCP功能测试 ===" -ForegroundColor Green

# 1. 用户注册/登录
Write-Host "1. 用户登录..." -ForegroundColor Yellow
try {
    $loginResponse = Invoke-RestMethod -Uri "$baseUrl/login" -Method POST -Body @{
        username = $username
        password = $password
    } -ContentType "application/x-www-form-urlencoded" -SessionVariable session
    
    if ($loginResponse.message -eq "登录成功") {
        Write-Host "✓ 登录成功" -ForegroundColor Green
    } else {
        Write-Host "✗ 登录失败，尝试注册..." -ForegroundColor Red
        
        $registerResponse = Invoke-RestMethod -Uri "$baseUrl/register" -Method POST -Body @{
            username = $username
            password = $password
            figma_token = $figmaToken
        } -ContentType "application/x-www-form-urlencoded" -WebSession $session
        
        if ($registerResponse.message -eq "注册成功") {
            Write-Host "✓ 注册成功" -ForegroundColor Green
        } else {
            throw "注册失败: $($registerResponse.error)"
        }
    }
} catch {
    Write-Host "✗ 登录/注册失败: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# 2. 检查MCP连接状态
Write-Host "`n2. 检查MCP连接状态..." -ForegroundColor Yellow
try {
    $statusResponse = Invoke-RestMethod -Uri "$baseUrl/mcp/status" -Method GET -WebSession $session
    Write-Host "MCP连接状态: $($statusResponse | ConvertTo-Json -Depth 3)" -ForegroundColor Cyan
} catch {
    Write-Host "✗ 获取MCP状态失败: $($_.Exception.Message)" -ForegroundColor Red
}

# 3. 创建MCP连接
Write-Host "`n3. 创建MCP连接..." -ForegroundColor Yellow
try {
    $connectResponse = Invoke-RestMethod -Uri "$baseUrl/mcp/connect" -Method POST -WebSession $session -ContentType "application/json"
    
    if ($connectResponse.success) {
        Write-Host "✓ MCP连接创建成功" -ForegroundColor Green
        $connectionId = $connectResponse.connection_id
        Write-Host "连接ID: $connectionId" -ForegroundColor Cyan
    } else {
        throw "连接创建失败: $($connectResponse.error)"
    }
} catch {
    Write-Host "✗ 创建MCP连接失败: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# 4. 测试MCP功能
Write-Host "`n4. 测试MCP功能..." -ForegroundColor Yellow

# 4.1 测试预览图获取
Write-Host "4.1 测试预览图获取..." -ForegroundColor Yellow
try {
    $previewResponse = Invoke-RestMethod -Uri "$baseUrl/mcp/$connectionId/preview/test_project/test_node" -Method GET -WebSession $session
    Write-Host "✓ 预览图获取测试成功" -ForegroundColor Green
    Write-Host "响应: $($previewResponse | ConvertTo-Json -Depth 3)" -ForegroundColor Cyan
} catch {
    Write-Host "✗ 预览图获取测试失败: $($_.Exception.Message)" -ForegroundColor Red
}

# 4.2 测试节点配置获取
Write-Host "`n4.2 测试节点配置获取..." -ForegroundColor Yellow
try {
    $configResponse = Invoke-RestMethod -Uri "$baseUrl/mcp/$connectionId/config/test_project/test_node" -Method GET -WebSession $session
    Write-Host "✓ 节点配置获取测试成功" -ForegroundColor Green
    Write-Host "响应: $($configResponse | ConvertTo-Json -Depth 3)" -ForegroundColor Cyan
} catch {
    Write-Host "✗ 节点配置获取测试失败: $($_.Exception.Message)" -ForegroundColor Red
}

# 4.3 测试配置提示词获取
Write-Host "`n4.3 测试配置提示词获取..." -ForegroundColor Yellow
try {
    $promptResponse = Invoke-RestMethod -Uri "$baseUrl/mcp/$connectionId/prompt/test_project/test_node?type=layout" -Method GET -WebSession $session
    Write-Host "✓ 配置提示词获取测试成功" -ForegroundColor Green
    Write-Host "响应: $($promptResponse | ConvertTo-Json -Depth 3)" -ForegroundColor Cyan
} catch {
    Write-Host "✗ 配置提示词获取测试失败: $($_.Exception.Message)" -ForegroundColor Red
}

# 4.4 测试节点配置设置
Write-Host "`n4.4 测试节点配置设置..." -ForegroundColor Yellow
try {
    $setConfigBody = @{
        width = 200
        height = 100
        x = 10
        y = 20
        color = "#FF0000"
    } | ConvertTo-Json
    
    $setConfigResponse = Invoke-RestMethod -Uri "$baseUrl/mcp/$connectionId/config/test_project/test_node" -Method POST -WebSession $session -Body $setConfigBody -ContentType "application/json"
    Write-Host "✓ 节点配置设置测试成功" -ForegroundColor Green
    Write-Host "响应: $($setConfigResponse | ConvertTo-Json -Depth 3)" -ForegroundColor Cyan
} catch {
    Write-Host "✗ 节点配置设置测试失败: $($_.Exception.Message)" -ForegroundColor Red
}

# 5. 获取调用日志
Write-Host "`n5. 获取MCP调用日志..." -ForegroundColor Yellow
try {
    $logsResponse = Invoke-RestMethod -Uri "$baseUrl/mcp/logs?limit=5" -Method GET -WebSession $session
    Write-Host "✓ 调用日志获取成功" -ForegroundColor Green
    Write-Host "日志数量: $($logsResponse.count)" -ForegroundColor Cyan
    
    if ($logsResponse.logs -and $logsResponse.logs.Count -gt 0) {
        Write-Host "最近的调用日志:" -ForegroundColor Cyan
        foreach ($log in $logsResponse.logs) {
            Write-Host "  - $($log.method) [$($log.status)] $($log.created_at)" -ForegroundColor Gray
        }
    }
} catch {
    Write-Host "✗ 获取调用日志失败: $($_.Exception.Message)" -ForegroundColor Red
}

# 6. 心跳测试
Write-Host "`n6. 心跳测试..." -ForegroundColor Yellow
try {
    $pingResponse = Invoke-RestMethod -Uri "$baseUrl/mcp/ping/$connectionId" -Method POST -WebSession $session
    Write-Host "✓ 心跳测试成功" -ForegroundColor Green
    Write-Host "响应: $($pingResponse | ConvertTo-Json -Depth 3)" -ForegroundColor Cyan
} catch {
    Write-Host "✗ 心跳测试失败: $($_.Exception.Message)" -ForegroundColor Red
}

# 7. 断开MCP连接
Write-Host "`n7. 断开MCP连接..." -ForegroundColor Yellow
try {
    $disconnectResponse = Invoke-RestMethod -Uri "$baseUrl/mcp/disconnect/$connectionId" -Method DELETE -WebSession $session
    
    if ($disconnectResponse.success) {
        Write-Host "✓ MCP连接断开成功" -ForegroundColor Green
    } else {
        throw "连接断开失败: $($disconnectResponse.error)"
    }
} catch {
    Write-Host "✗ 断开MCP连接失败: $($_.Exception.Message)" -ForegroundColor Red
}

Write-Host "`n=== MCP功能测试完成 ===" -ForegroundColor Green
