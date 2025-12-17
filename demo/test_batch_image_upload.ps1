# 批量图片导出并上传到OBS测试脚本

$baseUrl = "http://localhost:8080"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "批量图片导出并上传到OBS - 测试脚本" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# 1. 测试场景1：基本批量导出（3个节点）
Write-Host "测试场景1: 基本批量导出（3个节点）" -ForegroundColor Yellow
Write-Host "----------------------------------------"

$body1 = @{
    command = "export_nodes_as_images"
    params = @{
        nodeIds = "1-123,1-456,1-789"
        scale = 1
        ignore_text = $false
    }
} | ConvertTo-Json

Write-Host "请求体:" -ForegroundColor Green
Write-Host $body1
Write-Host ""

try {
    $response1 = Invoke-RestMethod -Uri "$baseUrl/mcp-api/tool-call" `
        -Method POST `
        -ContentType "application/json" `
        -Body $body1
    
    Write-Host "响应:" -ForegroundColor Green
    $response1 | ConvertTo-Json -Depth 10
    Write-Host ""
    
    if ($response1.result.content[0].text) {
        $result = $response1.result.content[0].text | ConvertFrom-Json
        
        Write-Host "导出统计:" -ForegroundColor Cyan
        Write-Host "  总数: $($result.total)"
        Write-Host "  成功: $($result.success)"
        Write-Host "  失败: $($result.failed)"
        
        if ($result.obs_enabled) {
            Write-Host ""
            Write-Host "OBS上传统计:" -ForegroundColor Cyan
            Write-Host "  已启用: $($result.obs_enabled)"
            Write-Host "  已上传: $($result.obs_uploaded)"
            Write-Host "  失败: $($result.obs_failed)"
            
            if ($result.obs_urls) {
                Write-Host ""
                Write-Host "OBS URLs:" -ForegroundColor Green
                $result.obs_urls.PSObject.Properties | ForEach-Object {
                    Write-Host "  $($_.Name): $($_.Value)"
                }
            }
        } else {
            Write-Host ""
            Write-Host "OBS状态: 未启用" -ForegroundColor Yellow
            Write-Host "  消息: $($result.obs_message)"
        }
    }
} catch {
    Write-Host "错误: $_" -ForegroundColor Red
}

Write-Host ""
Write-Host ""

# 2. 测试场景2：带缩放的批量导出
Write-Host "测试场景2: 带缩放的批量导出（scale=2）" -ForegroundColor Yellow
Write-Host "----------------------------------------"

$body2 = @{
    command = "export_nodes_as_images"
    params = @{
        nodeIds = "2-100,2-101,2-102"
        scale = 2
        ignore_text = $false
    }
} | ConvertTo-Json

Write-Host "请求体:" -ForegroundColor Green
Write-Host $body2
Write-Host ""

try {
    $response2 = Invoke-RestMethod -Uri "$baseUrl/mcp-api/tool-call" `
        -Method POST `
        -ContentType "application/json" `
        -Body $body2
    
    Write-Host "响应摘要:" -ForegroundColor Green
    
    if ($response2.result.content[0].text) {
        $result = $response2.result.content[0].text | ConvertFrom-Json
        
        Write-Host "导出: $($result.success)/$($result.total) 成功"
        
        if ($result.obs_enabled) {
            Write-Host "OBS上传: $($result.obs_uploaded)/$($result.success) 成功"
        }
    }
} catch {
    Write-Host "错误: $_" -ForegroundColor Red
}

Write-Host ""
Write-Host ""

# 3. 测试场景3：忽略文本的批量导出
Write-Host "测试场景3: 忽略文本的批量导出" -ForegroundColor Yellow
Write-Host "----------------------------------------"

$body3 = @{
    command = "export_nodes_as_images"
    params = @{
        nodeIds = "3-200,3-201"
        scale = 1
        ignore_text = $true
    }
} | ConvertTo-Json

Write-Host "请求体:" -ForegroundColor Green
Write-Host $body3
Write-Host ""

try {
    $response3 = Invoke-RestMethod -Uri "$baseUrl/mcp-api/tool-call" `
        -Method POST `
        -ContentType "application/json" `
        -Body $body3
    
    Write-Host "响应摘要:" -ForegroundColor Green
    
    if ($response3.result.content[0].text) {
        $result = $response3.result.content[0].text | ConvertFrom-Json
        
        Write-Host "导出: $($result.success)/$($result.total) 成功"
        Write-Host "忽略文本: true"
        
        if ($result.obs_enabled) {
            Write-Host "OBS上传: $($result.obs_uploaded)/$($result.success) 成功"
        }
    }
} catch {
    Write-Host "错误: $_" -ForegroundColor Red
}

Write-Host ""
Write-Host ""

# 4. 测试场景4：大规模批量导出（10个节点）
Write-Host "测试场景4: 大规模批量导出（10个节点）" -ForegroundColor Yellow
Write-Host "----------------------------------------"

$nodeIds = @()
for ($i = 0; $i -lt 10; $i++) {
    $nodeIds += "4-$($300 + $i)"
}

$body4 = @{
    command = "export_nodes_as_images"
    params = @{
        nodeIds = $nodeIds -join ","
        scale = 1
        ignore_text = $false
    }
} | ConvertTo-Json

Write-Host "请求体:" -ForegroundColor Green
Write-Host "节点数量: $($nodeIds.Count)"
Write-Host "节点IDs: $($nodeIds -join ', ')"
Write-Host ""

try {
    $stopwatch = [System.Diagnostics.Stopwatch]::StartNew()
    
    $response4 = Invoke-RestMethod -Uri "$baseUrl/mcp-api/tool-call" `
        -Method POST `
        -ContentType "application/json" `
        -Body $body4
    
    $stopwatch.Stop()
    
    Write-Host "响应摘要:" -ForegroundColor Green
    Write-Host "耗时: $($stopwatch.ElapsedMilliseconds) ms"
    Write-Host ""
    
    if ($response4.result.content[0].text) {
        $result = $response4.result.content[0].text | ConvertFrom-Json
        
        Write-Host "导出统计:"
        Write-Host "  总数: $($result.total)"
        Write-Host "  成功: $($result.success)"
        Write-Host "  失败: $($result.failed)"
        Write-Host "  平均耗时: $([math]::Round($stopwatch.ElapsedMilliseconds / $result.total, 2)) ms/张"
        
        if ($result.obs_enabled) {
            Write-Host ""
            Write-Host "OBS上传统计:"
            Write-Host "  已上传: $($result.obs_uploaded)"
            Write-Host "  失败: $($result.obs_failed)"
            
            if ($result.obs_urls) {
                $totalSize = 0
                $result.obs_urls.PSObject.Properties | ForEach-Object {
                    # 这里只是示例，实际无法从URL获取文件大小
                    Write-Host "  ✓ $($_.Name)"
                }
            }
        }
        
        if ($result.errors) {
            Write-Host ""
            Write-Host "导出错误:" -ForegroundColor Red
            $result.errors.PSObject.Properties | ForEach-Object {
                Write-Host "  $($_.Name): $($_.Value)"
            }
        }
        
        if ($result.obs_errors) {
            Write-Host ""
            Write-Host "OBS上传错误:" -ForegroundColor Red
            $result.obs_errors.PSObject.Properties | ForEach-Object {
                Write-Host "  $($_.Name): $($_.Value)"
            }
        }
    }
} catch {
    Write-Host "错误: $_" -ForegroundColor Red
}

Write-Host ""
Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "测试完成！" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "提示：" -ForegroundColor Yellow
Write-Host "1. 请确保Figma插件已连接"
Write-Host "2. 请确保OBS服务已正确配置"
Write-Host "3. 使用真实的节点ID进行测试"
Write-Host "4. 可以在 Web 界面查看详细的图片和OBS URL"
Write-Host ""

