# 下载库文件的PowerShell脚本

# 创建必要的目录
$directories = @(
    "web\static\lib\vue",
    "web\static\lib\element-ui",
    "web\static\lib\element-ui\theme-chalk",
    "web\static\lib\element-ui\theme-chalk\fonts",
    "web\static\lib\axios"
)

foreach ($dir in $directories) {
    if (!(Test-Path -Path $dir)) {
        Write-Host "创建目录: $dir"
        New-Item -ItemType Directory -Force -Path $dir | Out-Null
    }
}

# 定义要下载的文件
$files = @(
    @{
        Url = "https://cdn.bootcdn.net/ajax/libs/vue/2.6.14/vue.min.js"
        Destination = "web\static\lib\vue\vue.min.js"
        Name = "Vue.js"
    },
    @{
        Url = "https://cdn.bootcdn.net/ajax/libs/element-ui/2.15.14/index.min.js"
        Destination = "web\static\lib\element-ui\index.min.js"
        Name = "Element UI JavaScript"
    },
    @{
        Url = "https://cdn.bootcdn.net/ajax/libs/element-ui/2.15.14/theme-chalk/index.min.css"
        Destination = "web\static\lib\element-ui\theme-chalk\index.min.css"
        Name = "Element UI CSS"
    },
    @{
        Url = "https://cdn.bootcdn.net/ajax/libs/element-ui/2.15.14/theme-chalk/fonts/element-icons.woff"
        Destination = "web\static\lib\element-ui\theme-chalk\fonts\element-icons.woff"
        Name = "Element UI 字体 (WOFF)"
    },
    @{
        Url = "https://cdn.bootcdn.net/ajax/libs/element-ui/2.15.14/theme-chalk/fonts/element-icons.ttf"
        Destination = "web\static\lib\element-ui\theme-chalk\fonts\element-icons.ttf"
        Name = "Element UI 字体 (TTF)"
    },
    @{
        Url = "https://cdn.bootcdn.net/ajax/libs/axios/0.21.1/axios.min.js"
        Destination = "web\static\lib\axios\axios.min.js"
        Name = "Axios"
    }
)

# 下载文件
foreach ($file in $files) {
    Write-Host "正在下载 $($file.Name) 到 $($file.Destination)..."
    try {
        # 创建WebClient对象
        $webClient = New-Object System.Net.WebClient
        # 下载文件
        $webClient.DownloadFile($file.Url, $file.Destination)
        Write-Host "✓ 下载成功: $($file.Name)" -ForegroundColor Green
    }
    catch {
        Write-Host "✗ 下载失败: $($file.Name) - $($_.Exception.Message)" -ForegroundColor Red
    }
}

Write-Host "`n所有文件下载完成！" -ForegroundColor Cyan
Write-Host "现在可以修改 layout.html 文件，使用本地资源替代CDN资源。" -ForegroundColor Cyan
