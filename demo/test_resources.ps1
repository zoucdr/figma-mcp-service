# 测试MCP resources功能

# 配置
$baseUrl = "http://localhost:8080"
$mcpToken = "mcp_your_token_here"  # 请替换为实际的MCP token

Write-Host "=== 测试MCP Resources功能 ===" -ForegroundColor Green
Write-Host ""

# 测试1: Initialize - 检查是否支持resources
Write-Host "1. 测试 Initialize - 验证resources能力" -ForegroundColor Yellow
$initRequest = @{
    jsonrpc = "2.0"
    method = "initialize"
    id = 1
    params = @{}
} | ConvertTo-Json

try {
    $response = Invoke-RestMethod -Uri "$baseUrl/mcp/$mcpToken" -Method Post -Body $initRequest -ContentType "application/json"
    Write-Host "✓ Initialize 成功" -ForegroundColor Green
    Write-Host "协议版本: $($response.result.protocolVersion)"
    Write-Host "能力:" -ForegroundColor Cyan
    Write-Host ($response.result.capabilities | ConvertTo-Json -Depth 5)
    Write-Host ""
} catch {
    Write-Host "✗ Initialize 失败: $_" -ForegroundColor Red
    exit 1
}

# 测试2: Resources List - 获取资源列表
Write-Host "2. 测试 Resources/List - 获取UI预览资源" -ForegroundColor Yellow
$listRequest = @{
    jsonrpc = "2.0"
    method = "resources/list"
    id = 2
} | ConvertTo-Json

$firstResourceUri = $null
try {
    $response = Invoke-RestMethod -Uri "$baseUrl/mcp/$mcpToken" -Method Post -Body $listRequest -ContentType "application/json"
    Write-Host "✓ Resources/List 成功" -ForegroundColor Green
    
    if ($response.result.resources.Count -eq 0) {
        Write-Host "⚠ 没有找到资源（可能用户没有项目或未配置Figma Token）" -ForegroundColor Yellow
    } else {
        Write-Host "找到 $($response.result.resources.Count) 个资源:" -ForegroundColor Cyan
        foreach ($resource in $response.result.resources) {
            Write-Host ""
            Write-Host "  - URI: $($resource.uri)" -ForegroundColor White
            Write-Host "    名称: $($resource.name)" -ForegroundColor White
            Write-Host "    描述: $($resource.description)" -ForegroundColor White
            Write-Host "    类型: $($resource.mimeType)" -ForegroundColor White
            
            # 保存第一个资源URI用于后续测试
            if ($null -eq $firstResourceUri) {
                $firstResourceUri = $resource.uri
            }
            
            if ($resource.url) {
                # 显示URL的前100个字符（避免输出太长）
                $urlPreview = $resource.url.Substring(0, [Math]::Min(100, $resource.url.Length))
                Write-Host "    URL: $urlPreview..." -ForegroundColor White
            } else {
                Write-Host "    URL: (未生成)" -ForegroundColor Yellow
            }
        }
    }
    Write-Host ""
} catch {
    Write-Host "✗ Resources/List 失败: $_" -ForegroundColor Red
    Write-Host $_.Exception.Message
    exit 1
}

# 测试3: Resources Read - 读取具体资源
if ($null -ne $firstResourceUri) {
    Write-Host "3. 测试 Resources/Read - 读取资源内容" -ForegroundColor Yellow
    Write-Host "读取资源: $firstResourceUri" -ForegroundColor Cyan
    
    $readRequest = @{
        jsonrpc = "2.0"
        method = "resources/read"
        id = 3
        params = @{
            uri = $firstResourceUri
        }
    } | ConvertTo-Json -Depth 5

    try {
        $response = Invoke-RestMethod -Uri "$baseUrl/mcp/$mcpToken" -Method Post -Body $readRequest -ContentType "application/json"
        Write-Host "✓ Resources/Read 成功" -ForegroundColor Green
        
        if ($response.result.contents.Count -gt 0) {
            $content = $response.result.contents[0]
            Write-Host "  URI: $($content.uri)" -ForegroundColor White
            Write-Host "  类型: $($content.mimeType)" -ForegroundColor White
            
            if ($content.blob) {
                $blobSize = $content.blob.Length
                Write-Host "  数据大小: $blobSize 字符 (Base64编码)" -ForegroundColor White
                Write-Host "  预览: $($content.blob.Substring(0, [Math]::Min(80, $content.blob.Length)))..." -ForegroundColor Gray
            }
        }
        Write-Host ""
    } catch {
        Write-Host "✗ Resources/Read 失败: $_" -ForegroundColor Red
        Write-Host $_.Exception.Message
    }
} else {
    Write-Host "3. 跳过 Resources/Read 测试（没有可用的资源）" -ForegroundColor Yellow
    Write-Host ""
}

Write-Host ""
Write-Host "=== 测试完成 ===" -ForegroundColor Green
Write-Host ""
Write-Host "提示:" -ForegroundColor Cyan
Write-Host "1. 请确保服务正在运行 (http://localhost:8080)"
Write-Host "2. 请将上面的 mcpToken 替换为实际的MCP Token"
Write-Host "3. 确保用户已配置 Figma Token 并创建了项目"
Write-Host "4. UI预览资源会在获取资源列表时自动生成"

