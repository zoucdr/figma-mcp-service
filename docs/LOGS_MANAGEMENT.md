# 日志管理功能文档

## 概述

日志管理功能允许用户通过API上传日志文件到华为云OBS桶，并通过Web界面查看和管理这些日志文件。该功能无需登录即可访问。

## 功能特性

### ✅ 已实现功能

1. **日志文件上传**
   - 通过API上传日志文件
   - 支持自定义相对路径
   - 自动上传到华为云桶 `figma-deliver-cache` 的 `logs` 目录下
   - 无需登录即可访问

2. **日志文件浏览**
   - 展示文件夹结构（树形目录）
   - 支持目录导航
   - 显示文件大小和修改时间
   - 面包屑导航

3. **日志文件查看**
   - 在线查看日志文件内容
   - 支持内容复制
   - 暗黑主题界面

4. **日志文件下载**
   - 支持下载日志文件到本地

5. **路径直达**
   - 支持通过URL路径直接定位到指定目录
   - 例如：`/tools/logs/mateai` 直接跳转到 `logs/mateai` 目录

## 访问地址

### Web界面

- **根目录**: `http://localhost:8080/tools/logs`
- **子目录**: `http://localhost:8080/tools/logs/{相对路径}`
  - 示例：`http://localhost:8080/tools/logs/mateai`
  - 示例：`http://localhost:8080/tools/logs/mateai/debug`

### API接口

所有API接口都**无需登录**即可访问。

#### 1. 上传日志文件

**接口**: `POST /tools/api/logs/upload`

**参数**:
- `file` (文件): 要上传的日志文件
- `path` (表单字段或查询参数): 相对路径（可选，默认为根目录）

**示例**:

```bash
# cURL 示例
curl -X POST http://localhost:8080/tools/api/logs/upload \
  -F "file=@/path/to/logfile.txt" \
  -F "path=mateai/debug"

# PowerShell 示例
$form = @{
    path = "mateai/debug"
    file = Get-Item -Path "C:\logs\test.log"
}
Invoke-RestMethod -Uri "http://localhost:8080/tools/api/logs/upload" -Method Post -Form $form
```

**响应**:
```json
{
    "success": true,
    "object_key": "logs/mateai/debug/test.log",
    "url": "https://obs.cn-north-4.myhuaweicloud.com/figma-deliver-cache/logs/mateai/debug/test.log",
    "size": 1024
}
```

#### 2. 列出日志文件

**接口**: `GET /tools/api/logs/list`

**参数**:
- `path` (查询参数): 相对路径（可选，默认为根目录）

**示例**:

```bash
# 列出根目录
curl http://localhost:8080/tools/api/logs/list

# 列出子目录
curl http://localhost:8080/tools/api/logs/list?path=mateai

# PowerShell 示例
Invoke-RestMethod -Uri "http://localhost:8080/tools/api/logs/list?path=mateai"
```

**响应**:
```json
{
    "success": true,
    "path": "mateai",
    "objects": [
        {
            "key": "logs/mateai/debug/",
            "name": "debug",
            "size": 0,
            "last_modified": "0001-01-01T00:00:00Z",
            "is_dir": true
        },
        {
            "key": "logs/mateai/test.log",
            "name": "test.log",
            "size": 1024,
            "last_modified": "2025-11-25T10:30:00Z",
            "is_dir": false
        }
    ]
}
```

#### 3. 查看日志文件内容

**接口**: `GET /tools/api/logs/view`

**参数**:
- `key` (查询参数): OBS对象键（从列表接口获取）

**示例**:

```bash
curl "http://localhost:8080/tools/api/logs/view?key=logs/mateai/test.log"

# PowerShell 示例
Invoke-RestMethod -Uri "http://localhost:8080/tools/api/logs/view?key=logs/mateai/test.log"
```

**响应**:
```json
{
    "success": true,
    "content": "日志内容..."
}
```

#### 4. 下载日志文件

**接口**: `GET /tools/api/logs/download`

**参数**:
- `key` (查询参数): OBS对象键（从列表接口获取）

**示例**:

```bash
# 下载文件
curl -O "http://localhost:8080/tools/api/logs/download?key=logs/mateai/test.log"

# PowerShell 示例 - 下载到本地
Invoke-WebRequest -Uri "http://localhost:8080/tools/api/logs/download?key=logs/mateai/test.log" -OutFile "test.log"
```

## 存储结构

所有日志文件都存储在华为云OBS桶 `figma-deliver-cache` 的 `logs` 目录下。

**路径格式**:
```
logs/{相对路径}/{文件名}
```

**示例**:
- 根目录文件: `logs/test.log`
- 子目录文件: `logs/mateai/debug/error.log`
- 深层目录: `logs/project/module/date/app.log`

## 界面特性

### 暗黑主题

日志管理界面采用与其他页面一致的暗黑主题设计，包括：
- 深色背景渐变
- 紫色系主题色
- 卡片式布局
- 平滑动画效果

### 功能操作

1. **上传按钮**: 点击右上角"上传日志"按钮上传文件
2. **刷新按钮**: 点击"刷新"按钮重新加载文件列表
3. **面包屑导航**: 点击路径段快速跳转到父目录
4. **文件列表**:
   - 点击目录进入子目录
   - 点击文件查看内容
   - 使用"查看"按钮在线查看文件内容
   - 使用"下载"按钮下载文件到本地

## 测试脚本

提供了测试脚本来验证功能：

```powershell
# 运行测试脚本
.\demo\test_logs.ps1
```

测试脚本会执行以下操作：
1. 创建并上传测试日志文件
2. 列出根目录日志
3. 列出子目录日志
4. 测试访问页面
5. 测试访问子路径页面

## 技术实现

### 后端

1. **OBS服务扩展** (`internal/services/obs.go`)
   - `ListObjectsInPath`: 列出指定路径下的对象
   - `UploadLogFile`: 上传日志文件
   - `DownloadLogFile`: 下载日志文件

2. **日志控制器** (`internal/controllers/logs.go`)
   - `LogsPage`: 日志查看页面
   - `ListLogs`: 列出日志文件API
   - `UploadLog`: 上传日志文件API
   - `ViewLog`: 查看日志文件内容API
   - `DownloadLog`: 下载日志文件API

3. **路由配置** (`main.go`)
   - 所有日志管理路由都无需登录验证
   - 支持路径参数（通配符路由）

### 前端

1. **HTML模板** (`web/templates/logs.html`)
   - Vue.js 2.6
   - Element UI 2.15
   - 响应式布局

2. **CSS样式** (`web/static/css/logs.css`)
   - 暗黑主题样式
   - 渐变背景
   - 平滑动画

3. **JavaScript** (`web/static/js/logs.js`)
   - Vue.js应用
   - Axios HTTP请求
   - 路径导航
   - 文件操作

## 注意事项

1. **无需登录**: 所有功能都无需登录即可访问，适合公开的日志收集和查看场景
2. **存储位置**: 所有文件存储在华为云OBS桶的 `logs` 目录下
3. **文件类型**: 支持所有文本类型文件（.txt, .log, .json, .xml, .html等）
4. **路径格式**: 相对路径使用斜杠 `/` 分隔，不需要以斜杠开头
5. **浏览器兼容**: 推荐使用现代浏览器（Chrome, Firefox, Edge等）

## 使用场景

1. **应用日志收集**: 各个应用通过API上传运行日志
2. **错误追踪**: 快速查看和下载错误日志
3. **调试辅助**: 开发和测试阶段的日志查看
4. **团队协作**: 团队成员共享和查看日志文件
5. **监控告警**: 配合监控系统上传告警日志

## 未来扩展

可能的功能扩展：
- 日志搜索功能
- 日志文件删除功能
- 日志文件权限控制
- 日志文件分类标签
- 日志文件过期自动清理
- 日志文件预览（前N行）
- 日志文件统计信息

## 相关文档

- [华为云OBS集成指南](./OBS_INTEGRATION_GUIDE.md)
- [OBS集成总结](./OBS_INTEGRATION_SUMMARY.md)

## 联系方式

如有问题或建议，请联系开发团队。


