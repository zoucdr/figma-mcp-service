@echo off
echo 开始下载库文件...

REM 创建必要的目录
echo 创建目录结构...
mkdir "web\static\lib\vue" 2>nul
mkdir "web\static\lib\element-ui" 2>nul
mkdir "web\static\lib\element-ui\theme-chalk" 2>nul
mkdir "web\static\lib\element-ui\theme-chalk\fonts" 2>nul
mkdir "web\static\lib\axios" 2>nul

REM 设置下载文件列表
echo 开始下载文件...

REM 下载 Vue.js
echo 下载 Vue.js...
powershell -Command "& {(New-Object System.Net.WebClient).DownloadFile('https://cdn.bootcdn.net/ajax/libs/vue/2.6.14/vue.min.js', 'web\static\lib\vue\vue.min.js')}"
if %ERRORLEVEL% EQU 0 (
    echo [成功] Vue.js 已下载
) else (
    echo [失败] Vue.js 下载失败
)

REM 下载 Element UI JavaScript
echo 下载 Element UI JavaScript...
powershell -Command "& {(New-Object System.Net.WebClient).DownloadFile('https://cdn.bootcdn.net/ajax/libs/element-ui/2.15.14/index.min.js', 'web\static\lib\element-ui\index.min.js')}"
if %ERRORLEVEL% EQU 0 (
    echo [成功] Element UI JavaScript 已下载
) else (
    echo [失败] Element UI JavaScript 下载失败
)

REM 下载 Element UI CSS
echo 下载 Element UI CSS...
powershell -Command "& {(New-Object System.Net.WebClient).DownloadFile('https://cdn.bootcdn.net/ajax/libs/element-ui/2.15.14/theme-chalk/index.min.css', 'web\static\lib\element-ui\theme-chalk\index.min.css')}"
if %ERRORLEVEL% EQU 0 (
    echo [成功] Element UI CSS 已下载
) else (
    echo [失败] Element UI CSS 下载失败
)

REM 下载 Element UI 字体 (WOFF)
echo 下载 Element UI 字体 (WOFF)...
powershell -Command "& {(New-Object System.Net.WebClient).DownloadFile('https://cdn.bootcdn.net/ajax/libs/element-ui/2.15.14/theme-chalk/fonts/element-icons.woff', 'web\static\lib\element-ui\theme-chalk\fonts\element-icons.woff')}"
if %ERRORLEVEL% EQU 0 (
    echo [成功] Element UI 字体 (WOFF) 已下载
) else (
    echo [失败] Element UI 字体 (WOFF) 下载失败
)

REM 下载 Element UI 字体 (TTF)
echo 下载 Element UI 字体 (TTF)...
powershell -Command "& {(New-Object System.Net.WebClient).DownloadFile('https://cdn.bootcdn.net/ajax/libs/element-ui/2.15.14/theme-chalk/fonts/element-icons.ttf', 'web\static\lib\element-ui\theme-chalk\fonts\element-icons.ttf')}"
if %ERRORLEVEL% EQU 0 (
    echo [成功] Element UI 字体 (TTF) 已下载
) else (
    echo [失败] Element UI 字体 (TTF) 下载失败
)

REM 下载 Axios
echo 下载 Axios...
powershell -Command "& {(New-Object System.Net.WebClient).DownloadFile('https://cdn.bootcdn.net/ajax/libs/axios/0.21.1/axios.min.js', 'web\static\lib\axios\axios.min.js')}"
if %ERRORLEVEL% EQU 0 (
    echo [成功] Axios 已下载
) else (
    echo [失败] Axios 下载失败
)

echo.
echo 所有文件下载完成！
echo 现在可以修改 layout.html 文件，使用本地资源替代CDN资源。
echo.

pause
