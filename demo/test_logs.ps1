# 测试日志管理功能的PowerShell脚本

$baseUrl = "http://localhost:8080"

Write-Host "===== 测试日志管理功能 =====" -ForegroundColor Cyan
Write-Host ""

# 1. 测试上传日志文件
Write-Host "1. 测试上传日志文件..." -ForegroundColor Yellow

# 创建测试日志文件
$testContent = "Test log content`nTimestamp: $(Get-Date)`nThis is a test log file."
$testFilePath = "$env:TEMP\test_log_$(Get-Date -Format 'yyyyMMdd_HHmmss').txt"
Set-Content -Path $testFilePath -Value $testContent -Encoding UTF8

Write-Host "   创建测试文件: $testFilePath" -ForegroundColor Gray

try {
    # 准备表单数据
    $form = @{
        path = "mateai/test"
        file = Get-Item -Path $testFilePath
    }
    
    # 上传文件
    $uploadResponse = Invoke-RestMethod -Uri "$baseUrl/tools/api/logs/upload" -Method Post -Form $form
    
    if ($uploadResponse.success) {
        Write-Host "   ✅ 上传成功" -ForegroundColor Green
        Write-Host "   对象键: $($uploadResponse.object_key)" -ForegroundColor Gray
        Write-Host "   URL: $($uploadResponse.url)" -ForegroundColor Gray
        Write-Host "   大小: $($uploadResponse.size) bytes" -ForegroundColor Gray
    } else {
        Write-Host "   ❌ 上传失败: $($uploadResponse.error)" -ForegroundColor Red
        exit 1
    }
} catch {
    Write-Host "   ❌ 上传请求失败: $_" -ForegroundColor Red
    exit 1
} finally {
    # 清理测试文件
    Remove-Item -Path $testFilePath -ErrorAction SilentlyContinue
}

Write-Host ""

# 2. 测试列出根目录日志
Write-Host "2. 测试列出根目录日志..." -ForegroundColor Yellow

try {
    $listResponse = Invoke-RestMethod -Uri "$baseUrl/tools/api/logs/list" -Method Get
    
    if ($listResponse.success) {
        Write-Host "   ✅ 列出成功" -ForegroundColor Green
        Write-Host "   路径: $($listResponse.path)" -ForegroundColor Gray
        Write-Host "   对象数量: $($listResponse.objects.Count)" -ForegroundColor Gray
        
        if ($listResponse.objects.Count -gt 0) {
            Write-Host "   对象列表:" -ForegroundColor Gray
            foreach ($obj in $listResponse.objects) {
                $type = if ($obj.is_dir) { "目录" } else { "文件" }
                Write-Host "     - $($obj.name) ($type)" -ForegroundColor Gray
            }
        }
    } else {
        Write-Host "   ❌ 列出失败: $($listResponse.error)" -ForegroundColor Red
    }
} catch {
    Write-Host "   ❌ 列出请求失败: $_" -ForegroundColor Red
}

Write-Host ""

# 3. 测试列出子目录日志
Write-Host "3. 测试列出子目录日志 (mateai)..." -ForegroundColor Yellow

try {
    $listResponse = Invoke-RestMethod -Uri "$baseUrl/tools/api/logs/list?path=mateai" -Method Get
    
    if ($listResponse.success) {
        Write-Host "   ✅ 列出成功" -ForegroundColor Green
        Write-Host "   路径: $($listResponse.path)" -ForegroundColor Gray
        Write-Host "   对象数量: $($listResponse.objects.Count)" -ForegroundColor Gray
        
        if ($listResponse.objects.Count -gt 0) {
            Write-Host "   对象列表:" -ForegroundColor Gray
            foreach ($obj in $listResponse.objects) {
                $type = if ($obj.is_dir) { "目录" } else { "文件" }
                Write-Host "     - $($obj.name) ($type)" -ForegroundColor Gray
            }
        }
    } else {
        Write-Host "   ❌ 列出失败: $($listResponse.error)" -ForegroundColor Red
    }
} catch {
    Write-Host "   ❌ 列出请求失败: $_" -ForegroundColor Red
}

Write-Host ""

# 4. 测试访问页面
Write-Host "4. 测试访问日志页面..." -ForegroundColor Yellow

try {
    $pageResponse = Invoke-WebRequest -Uri "$baseUrl/tools/logs" -Method Get
    
    if ($pageResponse.StatusCode -eq 200) {
        Write-Host "   ✅ 页面访问成功 (状态码: $($pageResponse.StatusCode))" -ForegroundColor Green
    } else {
        Write-Host "   ⚠️ 页面访问返回状态码: $($pageResponse.StatusCode)" -ForegroundColor Yellow
    }
} catch {
    Write-Host "   ❌ 页面访问失败: $_" -ForegroundColor Red
}

Write-Host ""

# 5. 测试访问子路径页面
Write-Host "5. 测试访问子路径页面 (/tools/logs/mateai)..." -ForegroundColor Yellow

try {
    $pageResponse = Invoke-WebRequest -Uri "$baseUrl/tools/logs/mateai" -Method Get
    
    if ($pageResponse.StatusCode -eq 200) {
        Write-Host "   ✅ 页面访问成功 (状态码: $($pageResponse.StatusCode))" -ForegroundColor Green
    } else {
        Write-Host "   ⚠️ 页面访问返回状态码: $($pageResponse.StatusCode)" -ForegroundColor Yellow
    }
} catch {
    Write-Host "   ❌ 页面访问失败: $_" -ForegroundColor Red
}

Write-Host ""
Write-Host "===== 测试完成 =====" -ForegroundColor Cyan
Write-Host ""
Write-Host "提示: 可以在浏览器中访问 $baseUrl/tools/logs 查看日志管理界面" -ForegroundColor Green


