# Unity自动日志收集上报系统使用指南

## 📋 概述

`LogFileReporter` 是一个Unity自动日志收集和上报系统，专门用于调试版本的日志采集和问题排查。该系统能够自动捕获Unity运行时的所有日志信息，并在满足特定条件时自动上传到服务器。

### 主要特性

- ✅ **自动收集** - 自动捕获所有Unity运行时日志
- 🎯 **智能触发** - 基于日志数量和错误类型智能上报
- 📊 **统计分析** - 自动统计各类型日志数量
- 🚀 **异步上传** - 不阻塞游戏运行
- 🔒 **调试专用** - 仅在DEBUG模式下启用
- 🌍 **多平台支持** - 自动识别运行平台
- 📝 **格式化输出** - 生成可读的文本日志

## 🎮 快速开始

### 1. 集成到项目

1. 将 `LogFileReporter.cs` 脚本放入项目的 `Scripts/Debug` 目录
2. 脚本会自动在场景加载后启动（无需手动挂载）
3. 确保项目定义了 `DEBUG_CODE` 宏（在 Player Settings → Scripting Define Symbols）

```
项目结构示例：
Assets/
  └── Scripts/
      └── Debug/
          └── LogFileReporter.cs
```

### 2. 配置服务器

修改脚本顶部的配置参数：

```csharp
private static string serverHost = "figma-deliver.wekoi.cc"; // 服务器地址
private static string basePath = "mycoria"; // 基础路径（项目名称）
```

### 3. 构建调试版本

确保在构建设置中启用了 `DEBUG_CODE` 宏：

```
Unity → Edit → Project Settings → Player → Other Settings
→ Scripting Define Symbols
→ 添加 "DEBUG_CODE"
```

## 🔧 工作原理

### 自动启动机制

```csharp
[RuntimeInitializeOnLoadMethod(RuntimeInitializeLoadType.AfterSceneLoad)]
public static void Load()
```

- 在场景加载后自动初始化
- 无需手动创建GameObject或挂载脚本
- 全局单例模式，确保只有一个实例

### 日志收集

系统监听Unity的日志回调：

```csharp
Application.logMessageReceivedThreaded += OnLogMessageReceive;
```

收集的日志类型：
- ✅ `Log` - 普通信息日志
- ⚠️ `Warning` - 警告日志
- ❌ `Error` - 错误日志
- 🔥 `Exception` - 异常日志
- 🛑 `Assert` - 断言日志

### 触发上报条件

日志会在以下情况下自动上报：

| 条件 | 触发阈值 | 说明 |
|------|---------|------|
| **日志总数达到上限** | 1000条 | 防止内存占用过大 |
| **错误/异常较多** | 100条（包含错误/异常） | 及时发现严重问题 |

触发后会：
1. 格式化所有日志为文本文件
2. 保存到本地 `Application.persistentDataPath/Logs/` 目录
3. 异步上传到服务器
4. 清空内存中的日志缓存

## 📝 日志格式

### 文件命名

```
unitylog_YYYY.MM.DD_HH.mm.ss_(序号)-原因.txt
```

示例：
- `unitylog_2025.11.25_14.30.00_(1)-Max.txt` - 日志数量达到上限
- `unitylog_2025.11.25_15.45.30_(2)-Error.txt` - 错误日志触发

### 文件内容

```
设备：iPhone 14 Pro
平台：IPhonePlayer
打包时间：2025-11-25 10:00:00
启动时间：[2025-11-25 14:30:00:1)]
上报时间：[2025-11-25 14:35:20] 
   总计 :1000 Error:5 Warn:20 Info:970 Exception:3 Assert:2

0.[Info] Game started
1.[Warn] Audio clip not found: bgm_battle
2.[Error] NullReferenceException: Object reference not set
UnityEngine.GameObject.GetComponent(...)
...
```

## 🌐 上传路径规则

日志文件会上传到以下路径：

```
logs/mycoria/{平台名称}/文件名.txt
```

### 平台映射表

| Unity平台 | 上传路径 | 示例 |
|-----------|---------|------|
| `WindowsPlayer` | `logs/mycoria/WindowsPlayer/` | PC客户端 |
| `Android` | `logs/mycoria/Android/` | 安卓手机 |
| `IPhonePlayer` | `logs/mycoria/IPhonePlayer/` | iOS设备 |
| `WebGLPlayer` | `logs/mycoria/WebGLPlayer/` | Web浏览器 |
| `OSXPlayer` | `logs/mycoria/OSXPlayer/` | Mac客户端 |

## 🔍 日志管理系统完整功能

### Web界面访问

#### 主页面

```
https://figma-deliver.wekoi.cc/tools/logs
```

这是日志管理系统的根目录，可以看到所有项目的日志文件夹。

#### 直接访问子目录

```
https://figma-deliver.wekoi.cc/tools/logs/{路径}
```

示例：
- **根目录**: `https://figma-deliver.wekoi.cc/tools/logs`
- **项目目录**: `https://figma-deliver.wekoi.cc/tools/logs/mycoria`
- **平台目录**: `https://figma-deliver.wekoi.cc/tools/logs/mycoria/WindowsPlayer`
- **子目录**: `https://figma-deliver.wekoi.cc/tools/logs/mycoria/Android/2025-11`

### 🎨 界面功能详解

#### 1. 📂 文件浏览器

**目录展示**
```
┌─────────────────────────────────────────┐
│ logs > mycoria > WindowsPlayer          │ ← 面包屑导航
├─────────────────────────────────────────┤
│ 名称                 │ 大小  │ 修改时间  │
├─────────────────────────────────────────┤
│ 📁 2025-11          │   -   │    -      │ ← 文件夹
│ 📄 test.log         │ 1.2KB │ 2025-11-25│ ← 文件
│ 📄 error.txt        │ 856B  │ 2025-11-24│
└─────────────────────────────────────────┘
```

**特性：**
- 🎯 点击文件夹进入子目录
- 🔙 面包屑导航快速返回上级
- 📊 自动排序（按名称/大小/时间）
- 🎨 图标区分文件和文件夹
- ⚡ 实时加载，无需刷新

#### 2. 📤 手动上传日志

**步骤：**
1. 点击右上角"上传日志"按钮
2. 选择标签页：
   - **网页上传** - 直接选择文件上传
   - **API调用** - 查看API文档和示例代码

**网页上传界面：**
```
┌──────────────────────────────────────┐
│ [网页上传] [API调用]                 │
├──────────────────────────────────────┤
│ 当前路径: mycoria/WindowsPlayer      │
│                                      │
│ 选择文件: [选取文件]                 │
│           支持.txt, .log, .json等    │
└──────────────────────────────────────┘
         [取消] [确定上传]
```

**支持的文件格式：**
- `.txt` - 文本文件
- `.log` - 日志文件
- `.json` - JSON格式日志
- `.xml` - XML格式日志
- `.html` - HTML格式日志
- 所有文本类型文件

#### 3. 📖 API调用文档

点击"API调用"标签页，查看完整的API文档：

**包含内容：**
- 📡 接口地址（可一键复制）
- 📋 请求参数表格
- 💻 多语言调用示例
  - cURL（命令行）
  - PowerShell（Windows脚本）
  - C#（Unity开发）
  - Python（自动化脚本）
- 📄 响应格式示例
- ℹ️ 注意事项说明

**示例展示：**
```
┌──────────────────────────────────────┐
│ 📡 API上传接口                       │
│                                      │
│ 接口地址                    [复制]   │
│ POST .../tools/api/logs/upload       │
│                                      │
│ 请求参数                             │
│ ┌────────────────────────────────┐  │
│ │ file   │ 文件   │ 日志文件    │  │
│ │ path   │ 字符串 │ 相对路径    │  │
│ └────────────────────────────────┘  │
│                                      │
│ [cURL] [PowerShell] [C#] [Python]   │
│ ┌────────────────────────────────┐  │
│ │ curl -X POST ...       [复制]  │  │
│ └────────────────────────────────┘  │
└──────────────────────────────────────┘
```

#### 4. 👁️ 在线查看日志

**操作方式：**
1. 在文件列表中找到目标日志
2. 点击"查看"按钮
3. 在弹出窗口中查看内容

**查看器功能：**
```
┌──────────────────────────────────────┐
│ 查看文件: unitylog_2025.11.25...txt │
├──────────────────────────────────────┤
│ 设备：iPhone 14 Pro                  │
│ 平台：IPhonePlayer                   │
│ 启动时间：[2025-11-25 14:30:00]     │
│ 总计:1000 Error:5 Warn:20           │
│                                      │
│ 0.[Info] Game started                │
│ 1.[Error] NullReference...           │
│ ...                                  │
└──────────────────────────────────────┘
         [关闭] [复制内容]
```

**特性：**
- 📜 滚动查看完整内容
- 🔍 浏览器内查找（Ctrl+F）
- 📋 一键复制全部内容
- 🎨 代码高亮显示
- 📱 响应式布局（支持移动端）

#### 5. 💾 下载日志文件

**操作方式：**
1. 点击文件的"下载"按钮
2. 自动下载到本地电脑
3. 保持原始文件名

**用途：**
- 离线分析
- 长期存档
- 使用专业工具分析
- 与团队分享

#### 6. 🔄 刷新列表

**刷新按钮：**
- 位置：右上角
- 功能：重新加载当前目录的文件列表
- 用途：查看新上传的文件

### 🎯 使用场景演示

#### 场景1：玩家报告Bug

1. **玩家反馈**：游戏在某场景崩溃
2. **查看日志**：
   ```
   访问: .../tools/logs/mycoria/Android
   按时间排序，找到玩家设备的最新日志
   ```
3. **在线查看**：点击"查看"按钮快速预览
4. **定位问题**：Ctrl+F 搜索 "Exception" 或 "Error"
5. **下载分析**：下载日志进行详细分析

#### 场景2：批量问题排查

1. **收集日志**：多个玩家同时报告问题
2. **浏览目录**：
   ```
   .../tools/logs/mycoria/WindowsPlayer/
   查看所有玩家的日志文件
   ```
3. **批量下载**：下载所有相关日志
4. **对比分析**：使用工具对比不同日志的共同点

#### 场景3：开发者手动测试

1. **运行测试**：本地运行Debug版本
2. **触发上报**：手动调用 `Report("TestCase1")`
3. **立即查看**：
   ```
   刷新Web页面
   查看刚上传的日志
   ```
4. **验证结果**：在线查看日志内容，确认测试结果

#### 场景4：远程协助玩家

1. **指导玩家**：让玩家运行Debug版本
2. **等待上报**：玩家操作后日志自动上传
3. **远程查看**：
   ```
   不需要玩家发送任何文件
   直接在Web界面查看日志
   ```
4. **快速定位**：实时查看问题现场

### 🔐 访问权限

**当前配置：**
- ✅ **无需登录** - 所有人都可以访问
- ✅ **无需Token** - 不需要任何认证
- ✅ **公开访问** - 适合内部团队使用

**安全建议：**
- 🔒 生产环境建议配置IP白名单
- 🔑 或添加基础认证（HTTP Basic Auth）
- 🌐 使用HTTPS保护数据传输

### 📱 移动端支持

Web界面完全支持移动设备：
- 📱 响应式布局自适应屏幕
- 👆 触摸操作友好
- 🔄 下拉刷新
- 📲 可以在手机上查看日志

## 🎯 使用场景

### 1. 内测阶段问题排查

```csharp
// 脚本会自动工作，无需额外代码
// 当玩家遇到问题时，日志会自动上传
```

### 2. 崩溃前日志收集

```csharp
// 在Application.quitting事件中手动触发上报
void OnApplicationQuit()
{
    LogFileReporter.Report("Quit").Wait();
}
```

### 3. 关键节点日志上报

```csharp
// 在重要游戏节点手动触发
public async void OnBossDefeated()
{
    await LogFileReporter.Report("BossDefeated");
}
```

## ⚙️ 高级配置

### 修改上报阈值

```csharp
// 在 OnLogMessageReceive 方法中修改
if (logInfos.Count > 1000) // 修改此数值
{
    Report("Max");
}
else if (logInfos.Count > 100 && ...) // 修改此数值
{
    Report(type.ToString());
}
```

### 自定义基础路径

```csharp
// 为不同项目设置不同的路径
private static string basePath = "your_project_name";
```

### 修改服务器地址

```csharp
// 切换到其他服务器
private static string serverHost = "your-server.com";
```

### 添加更多设备信息

```csharp
private string ConvertLogsToText()
{
    StringBuilder sb = new StringBuilder();
    sb.AppendLine($"设备：{deviceModel}");
    sb.AppendLine($"平台：{platform}");
    
    // 添加更多信息
    sb.AppendLine($"Unity版本：{Application.unityVersion}");
    sb.AppendLine($"游戏版本：{Application.version}");
    sb.AppendLine($"系统语言：{Application.systemLanguage}");
    sb.AppendLine($"内存大小：{SystemInfo.systemMemorySize}MB");
    
    // ... 其余代码
}
```

## 🔐 安全性说明

### DEBUG模式限制

```csharp
#if DEBUG_CODE
// 整个脚本只在定义了 DEBUG_CODE 宏时编译
#endif
```

- ✅ 发布版本不会包含此功能
- ✅ 不会收集或上传任何数据
- ✅ 不影响正式版本的性能

### 数据隐私

- 📝 仅收集Unity日志信息
- 🔒 不收集用户个人信息
- 🌐 通过HTTP上传（建议在生产环境使用HTTPS）
- 📁 存储在开发者控制的服务器上

## 📊 性能影响

### 内存占用

- 每条日志约占 50-200 字节
- 缓存上限 1000 条 ≈ 50KB-200KB
- 触发上报后立即清空缓存

### CPU占用

- 日志收集：极低（异步回调）
- 文件写入：中等（同步IO）
- 网络上传：低（异步HTTP）

### 建议

- ✅ 仅在内测/测试版本启用
- ✅ 正式版本移除 `DEBUG_CODE` 宏
- ✅ 监控上传频率，避免过于频繁

## 🐛 故障排查

### 问题1：日志没有上传

**检查项：**
1. 是否定义了 `DEBUG_CODE` 宏？
2. 服务器地址是否正确？
3. 设备是否有网络连接？
4. 查看Unity Console是否有上传错误日志

**解决方案：**
```csharp
// 在Unity Editor中测试
#if UNITY_EDITOR
Debug.Log("LogFileReporter已启动");
#endif
```

### 问题2：找不到本地日志文件

**本地文件位置：**
- **Android**: `/storage/emulated/0/Android/data/{包名}/files/Logs/`
- **iOS**: `~/Library/Caches/{包名}/Logs/`
- **Windows**: `%AppData%\..\LocalLow\{公司名}\{游戏名}\Logs\`
- **Mac**: `~/Library/Logs/{公司名}/{游戏名}/Logs/`

### 问题3：上传失败

**检查网络请求：**
```csharp
Debug.LogError($"状态码: {(int)response.StatusCode}");
Debug.LogError($"响应: {body}");
```

**常见错误：**
- `404` - 服务器地址错误
- `500` - 服务器内部错误
- `Timeout` - 网络超时（增加超时时间）

## 🔄 完整工作流程（图文版）

### 流程1：自动上报 → Web查看

```
Unity游戏运行
     │
     ├─→ 捕获日志
     │   (Debug.Log, Error等)
     │
     ├─→ 缓存在内存
     │   (最多1000条)
     │
     ├─→ 触发条件满足
     │   (>=1000条 或 >=100错误)
     │
     ├─→ 格式化为文本
     │   unitylog_2025.11.25_14.30.00_(1)-Max.txt
     │
     ├─→ 保存到本地
     │   {persistentDataPath}/Logs/
     │
     ├─→ 异步上传到服务器
     │   POST /tools/api/logs/upload
     │
     └─→ 存储在华为云OBS
         logs/mycoria/WindowsPlayer/unitylog...txt

开发者查看
     │
     ├─→ 访问Web界面
     │   https://figma-deliver.wekoi.cc/tools/logs
     │
     ├─→ 导航到目录
     │   mycoria → WindowsPlayer
     │
     ├─→ 查看文件列表
     │   按时间排序
     │
     ├─→ 点击"查看"按钮
     │   在线预览日志内容
     │
     ├─→ 搜索关键词
     │   Ctrl+F 查找 "Error"
     │
     └─→ 下载到本地
         详细分析问题
```

### 流程2：手动上传 → 快速分析

```
本地有日志文件
     │
     ├─→ 访问Web界面
     │   https://figma-deliver.wekoi.cc/tools/logs
     │
     ├─→ 导航到目标目录
     │   mycoria → Android
     │
     ├─→ 点击"上传日志"
     │   
     ├─→ 选择"网页上传"
     │   选取本地文件
     │
     ├─→ 点击"确定上传"
     │   自动上传到当前目录
     │
     └─→ 立即查看
         刷新列表 → 点击查看
```

### 流程3：API集成 → 自动化

```
自动化脚本/CI系统
     │
     ├─→ 收集测试日志
     │   test_results.log
     │
     ├─→ 调用上传API
     │   curl -X POST ... -F "file=@test.log"
     │
     ├─→ 获取上传结果
     │   {success: true, url: "..."}
     │
     └─→ Web界面查看
         自动化测试报告
```

## 💡 高级使用技巧

### 技巧1：使用浏览器书签快速访问

为常用平台创建书签：
```
Windows日志: .../tools/logs/mycoria/WindowsPlayer
Android日志: .../tools/logs/mycoria/Android
iOS日志: .../tools/logs/mycoria/IPhonePlayer
```

### 技巧2：使用浏览器的"在页面中查找"

1. 打开日志文件
2. 按 `Ctrl+F` (Windows) 或 `Cmd+F` (Mac)
3. 输入关键词：
   - `Error` - 查找所有错误
   - `Exception` - 查找异常
   - `NullReference` - 查找空引用
   - 玩家ID - 查找特定玩家日志

### 技巧3：按时间过滤日志

在文件列表中：
1. 点击"修改时间"列标题排序
2. 最新的日志会排在最前面
3. 快速定位最近的问题

### 技巧4：使用目录组织日志

建议的目录结构：
```
logs/
└── mycoria/
    ├── WindowsPlayer/
    │   ├── 2025-11/        ← 按月份归档
    │   │   ├── crash/      ← 按类型分类
    │   │   ├── performance/
    │   │   └── general/
    │   └── 2025-10/
    ├── Android/
    └── IPhonePlayer/
```

在上传时指定path参数：
```csharp
string relativePath = $"{basePath}/{platform}/2025-11/crash";
```

### 技巧5：导出日志到分析工具

1. 批量下载日志文件
2. 使用日志分析工具：
   - **Splunk** - 企业级日志分析
   - **ELK Stack** - 开源日志分析
   - **LogParser** - 微软日志解析工具
   - **grep/awk** - Linux命令行工具

### 技巧6：团队协作

分享日志链接：
```
发送给同事：
https://figma-deliver.wekoi.cc/tools/logs/mycoria/Android

他们可以直接访问查看，无需登录
```

## 📚 API参考

### 公共方法

#### Upload(string filePath)

上传指定的日志文件。

**参数：**
- `filePath` - 本地文件完整路径

**返回：**
- `Task<bool>` - 上传是否成功

**示例：**
```csharp
bool success = await LogFileReporter.Upload("/path/to/log.txt");
```

#### UploadWithCustomName(string filePath, string customFileName)

使用自定义文件名上传日志文件。

**参数：**
- `filePath` - 本地文件完整路径
- `customFileName` - 自定义文件名

**返回：**
- `Task<bool>` - 上传是否成功

**示例：**
```csharp
bool success = await LogFileReporter.UploadWithCustomName(
    "/path/to/log.txt", 
    "custom_name.txt"
);
```

#### Report(string reason)

手动触发日志上报。

**参数：**
- `reason` - 上报原因（会显示在文件名中）

**返回：**
- `Task` - 异步任务

**示例：**
```csharp
await LogFileReporter.Report("ManualTrigger");
```

## 🔄 更新日志

### v1.0 (2025-11-25)

- ✅ 初始版本
- ✅ 支持自动日志收集
- ✅ 支持智能触发上报
- ✅ 支持多平台自动识别
- ✅ 支持异步上传
- ✅ 支持日志格式化

## 📞 技术支持

如有问题，请：
1. 查看Unity Console的错误日志
2. 检查服务器日志管理页面
3. 联系开发团队

---

## 附录A：Web界面操作指南

### 主界面布局

```
┌────────────────────────────────────────────────────┐
│ Figma Deliver    日志查看工具                      │
│                           [上传日志] [刷新]        │
├────────────────────────────────────────────────────┤
│ logs > mycoria > WindowsPlayer                     │ ← 面包屑
├────────────────────────────────────────────────────┤
│ 名称            │ 大小   │ 修改时间      │ 操作   │
├────────────────────────────────────────────────────┤
│ 📁 2025-11      │   -    │      -        │        │
│ 📄 unity...txt  │ 1.2KB  │ 2025-11-25... │[查看][下载]│
│ 📄 error.log    │ 856B   │ 2025-11-24... │[查看][下载]│
└────────────────────────────────────────────────────┘
```

### 常用操作快捷键

| 操作 | 快捷键 | 说明 |
|------|--------|------|
| 页面内搜索 | `Ctrl+F` / `Cmd+F` | 查找文件名或内容 |
| 刷新页面 | `F5` | 重新加载文件列表 |
| 返回上级 | 点击面包屑 | 快速导航 |
| 复制内容 | 查看窗口中点击"复制" | 复制日志内容 |

### 文件类型图标

| 图标 | 类型 | 说明 |
|------|------|------|
| 📁 | 文件夹 | 可以点击进入 |
| 📄 | 文件 | 可以查看和下载 |

### 上传对话框标签页

| 标签页 | 功能 | 适用场景 |
|--------|------|---------|
| 网页上传 | 选择文件直接上传 | 手动上传单个文件 |
| API调用 | 查看API文档和示例 | 程序化集成 |

## 附录B：完整系统架构

```
┌─────────────────────────────────────────┐
│  Unity游戏客户端                        │
│  ├─ LogFileReporter.cs (收集)          │
│  └─ HttpClient (上传)                   │
└─────────────────┬───────────────────────┘
                  │ POST /tools/api/logs/upload
                  │ multipart/form-data
                  ▼
┌─────────────────────────────────────────┐
│  Figma Deliver服务器                    │
│  ├─ Gin Web框架                         │
│  ├─ LogsController (处理上传)          │
│  └─ OBSService (对接华为云)            │
└─────────────────┬───────────────────────┘
                  │ PutObject API
                  ▼
┌─────────────────────────────────────────┐
│  华为云OBS对象存储                      │
│  Bucket: figma-deliver-cache            │
│  Path: logs/mycoria/{platform}/...      │
└─────────────────┬───────────────────────┘
                  │ ListObjects API
                  │ GetObject API
                  ▼
┌─────────────────────────────────────────┐
│  Web管理界面                            │
│  ├─ Vue.js (前端框架)                   │
│  ├─ Element UI (UI组件)                │
│  └─ Axios (HTTP请求)                    │
└─────────────────────────────────────────┘
         │
         ├─→ 文件浏览
         ├─→ 在线查看
         ├─→ 下载文件
         └─→ 上传文件
```

## 附录C：存储结构示例

```
logs/                              ← 日志根目录
└── mycoria/                       ← 项目名称
    ├── WindowsPlayer/             ← PC平台
    │   ├── unitylog_2025.11.25_14.30.00_(1)-Max.txt
    │   ├── unitylog_2025.11.25_15.20.30_(2)-Error.txt
    │   └── 2025-11/               ← 月份归档
    │       └── crash/             ← 类型分类
    ├── Android/                   ← 安卓平台
    │   ├── unitylog_2025.11.24_10.15.00_(1)-Max.txt
    │   └── unitylog_2025.11.25_16.45.00_(2)-Exception.txt
    └── IPhonePlayer/              ← iOS平台
        └── unitylog_2025.11.25_12.00.00_(1)-Max.txt
```

## 附录D：HTTP API完整说明

### 上传日志文件

**请求：**
```http
POST /tools/api/logs/upload HTTP/1.1
Host: figma-deliver.wekoi.cc
Content-Type: multipart/form-data; boundary=----WebKitFormBoundary

------WebKitFormBoundary
Content-Disposition: form-data; name="path"

mycoria/WindowsPlayer
------WebKitFormBoundary
Content-Disposition: form-data; name="file"; filename="test.log"
Content-Type: application/octet-stream

[文件内容]
------WebKitFormBoundary--
```

**响应：**
```json
{
    "success": true,
    "object_key": "logs/mycoria/WindowsPlayer/test.log",
    "url": "https://obs.cn-north-4.myhuaweicloud.com/figma-deliver-cache/logs/mycoria/WindowsPlayer/test.log",
    "size": 1024
}
```

### 列出日志文件

**请求：**
```http
GET /tools/api/logs/list?path=mycoria/WindowsPlayer HTTP/1.1
Host: figma-deliver.wekoi.cc
```

**响应：**
```json
{
    "success": true,
    "path": "mycoria/WindowsPlayer",
    "objects": [
        {
            "key": "logs/mycoria/WindowsPlayer/2025-11/",
            "name": "2025-11",
            "is_dir": true
        },
        {
            "key": "logs/mycoria/WindowsPlayer/test.log",
            "name": "test.log",
            "size": 1024,
            "last_modified": "2025-11-25T10:30:00Z",
            "is_dir": false
        }
    ]
}
```

### 查看日志内容

**请求：**
```http
GET /tools/api/logs/view?key=logs/mycoria/WindowsPlayer/test.log HTTP/1.1
Host: figma-deliver.wekoi.cc
```

**响应：**
```json
{
    "success": true,
    "content": "设备：Windows PC\n平台：WindowsPlayer\n..."
}
```

### 下载日志文件

**请求：**
```http
GET /tools/api/logs/download?key=logs/mycoria/WindowsPlayer/test.log HTTP/1.1
Host: figma-deliver.wekoi.cc
```

**响应：**
```
HTTP/1.1 200 OK
Content-Type: text/plain; charset=utf-8
Content-Disposition: attachment; filename=test.log

[文件内容]
```

## 附录E：最佳实践清单

### ✅ 开发阶段

- [ ] 定义 `DEBUG_CODE` 宏
- [ ] 配置服务器地址
- [ ] 设置项目基础路径
- [ ] 测试日志上传功能
- [ ] 验证Web界面访问

### ✅ 测试阶段

- [ ] 监控日志上传频率
- [ ] 检查日志文件大小
- [ ] 定期清理旧日志
- [ ] 分析常见错误类型
- [ ] 优化上报阈值

### ✅ 发布阶段

- [ ] 移除 `DEBUG_CODE` 宏
- [ ] 验证发布版本无日志收集
- [ ] 确认性能无影响
- [ ] 归档测试期间的日志

### ✅ 维护阶段

- [ ] 定期检查存储空间
- [ ] 清理过期日志（建议30天）
- [ ] 更新服务器地址（如需要）
- [ ] 优化目录结构

**让问题排查变得简单！** 🎉

