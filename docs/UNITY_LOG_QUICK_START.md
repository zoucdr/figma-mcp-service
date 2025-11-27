# Unity日志自动上报 - 快速开始

## 🚀 5分钟快速集成

### 第1步：添加脚本

将 `LogFileReporter.cs` 放入项目的 `Assets/Scripts/Debug/` 目录

### 第2步：配置宏定义

Unity → `Edit` → `Project Settings` → `Player` → `Other Settings`

在 `Scripting Define Symbols` 中添加：
```
DEBUG_CODE
```

### 第3步：修改配置（可选）

打开 `LogFileReporter.cs`，修改服务器地址和项目名称：

```csharp
private static string serverHost = "figma-deliver.wekoi.cc"; // 服务器地址
private static string basePath = "mycoria"; // 你的项目名称
```

### 第4步：构建并运行

构建Debug版本，脚本会自动工作！

## ✨ 自动触发条件

| 条件 | 阈值 | 说明 |
|------|------|------|
| 日志总数 | ≥ 1000条 | 自动上报并清空 |
| 错误日志 | ≥ 100条 | 包含Error/Exception时触发 |

## 📂 日志管理系统功能

### Web界面访问

```
https://figma-deliver.wekoi.cc/tools/logs
```

直接访问子目录：
```
https://figma-deliver.wekoi.cc/tools/logs/mycoria/{平台}
```

平台示例：
- **Windows**: `.../tools/logs/mycoria/WindowsPlayer`
- **Android**: `.../tools/logs/mycoria/Android`  
- **iOS**: `.../tools/logs/mycoria/IPhonePlayer`

### 📋 功能列表

#### 1. 📂 文件浏览
- 目录树形展示
- 面包屑导航
- 点击目录进入子目录
- 实时显示文件大小和修改时间

#### 2. 📤 手动上传日志
- 点击右上角"上传日志"按钮
- 选择文件上传到当前目录
- 支持所有文本格式（.txt, .log, .json等）

#### 3. 👁️ 在线查看
- 点击文件的"查看"按钮
- 在线预览日志内容
- 支持复制内容到剪贴板

#### 4. 💾 下载日志
- 点击文件的"下载"按钮
- 下载到本地电脑

#### 5. 🔄 刷新列表
- 点击右上角"刷新"按钮
- 重新加载文件列表

## 📝 日志文件格式

```
设备：iPhone 14 Pro
平台：IPhonePlayer
启动时间：[2025-11-25 14:30:00]
上报时间：[2025-11-25 14:35:20]
总计:1000 Error:5 Warn:20 Info:970 Exception:3

0.[Info] Game started
1.[Error] NullReferenceException: ...
...
```

## 🎯 手动触发上报

### Unity代码中触发

```csharp
// 在需要的地方调用
await LogFileReporter.Report("自定义原因");
```

### API手动上传

如果需要从其他地方上传日志，可以使用API：

```bash
# cURL示例
curl -X POST https://figma-deliver.wekoi.cc/tools/api/logs/upload \
  -F "file=@/path/to/logfile.txt" \
  -F "path=mycoria/WindowsPlayer"
```

```powershell
# PowerShell示例
$form = @{
    path = "mycoria/WindowsPlayer"
    file = Get-Item -Path "C:\logs\test.log"
}
Invoke-RestMethod -Uri "https://figma-deliver.wekoi.cc/tools/api/logs/upload" `
  -Method Post -Form $form
```

## 💡 使用技巧

### 1. 快速定位问题
在Web界面中：
1. 进入对应平台目录（如 `mycoria/Android`）
2. 按修改时间排序，查看最新日志
3. 点击"查看"在线浏览日志内容
4. 使用浏览器的查找功能（Ctrl+F）搜索关键词

### 2. 批量下载日志
可以下载多个日志文件到本地进行分析：
1. 点击每个日志的"下载"按钮
2. 使用文本编辑器批量查看
3. 或使用日志分析工具处理

### 3. 查看API调用示例
点击"上传日志"按钮，切换到"API调用"标签页：
- 查看完整API文档
- 提供cURL、PowerShell、C#、Python示例
- 一键复制代码

## ⚠️ 注意事项

- ✅ 仅在DEBUG模式下工作
- ✅ 发布版本记得移除 `DEBUG_CODE` 宏
- ✅ 需要网络连接才能上传
- ✅ 无需任何登录或Token
- 📌 Web界面支持所有现代浏览器
- 🔒 上传的日志存储在华为云OBS（安全可靠）

## 📚 完整文档

详细信息请查看：[完整使用指南](./UNITY_LOG_REPORTER_GUIDE.md)

---

**3步完成集成，让问题无处遁形！** 🎉

