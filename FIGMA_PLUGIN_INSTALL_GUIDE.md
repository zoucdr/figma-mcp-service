# Figma 插件安装完整指南

## 🚨 常见问题及解决方案

### 问题1：找不到 "Development" 选项

**原因**：开发者模式未启用或 Figma 版本过旧

**解决方案**：
1. **更新 Figma**：
   - 桌面版：帮助菜单 → 检查更新
   - 网页版：刷新页面获取最新版本

2. **启用开发者模式**：
   - 打开 Figma
   - 点击右上角头像 → Settings
   - 左侧菜单找到 "Developer" 或 "开发者"
   - 确保相关选项已启用

### 问题2：无法导入 manifest.json

**原因**：文件路径错误或文件格式问题

**解决方案**：
1. **检查文件路径**：
   ```
   D:\figma-deliver\service\figma-plugin\manifest.json
   ```

2. **验证文件内容**：
   确保 manifest.json 格式正确：
   ```json
   {
     "name": "Figma Deliver",
     "id": "figma-deliver-plugin",
     "api": "1.0.0",
     "main": "src/code.js",
     "ui": "ui/ui.html"
   }
   ```

### 问题3：插件安装后无法运行

**原因**：文件路径问题或代码错误

**解决方案**：
1. **检查文件结构**：
   ```
   figma-plugin/
   ├── manifest.json
   ├── src/
   │   └── code.js
   └── ui/
       ├── ui.html
       └── service.js
   ```

2. **检查控制台错误**：
   - 在 Figma 中按 F12 打开开发者工具
   - 查看 Console 面板的错误信息

## 📋 详细安装步骤

### 方法一：Figma 桌面版安装

1. **下载并安装 Figma 桌面版**：
   - 访问 https://www.figma.com/downloads/
   - 下载适合你系统的版本
   - 安装并登录账户

2. **启用开发者功能**：
   - 打开 Figma
   - 菜单栏：Help → Developer Tools（或按 F12）
   - 在设置中启用开发者模式

3. **导入插件**：
   - 打开任意设计文件
   - 菜单：Plugins → Development → Import plugin from manifest...
   - 选择：`D:\figma-deliver\service\figma-plugin\manifest.json`

### 方法二：Figma 网页版安装

1. **打开 Figma 网页版**：
   - 访问 https://www.figma.com
   - 登录你的账户

2. **创建或打开设计文件**：
   - 点击 "New design file" 或打开现有文件

3. **导入插件**：
   - 右键点击画布 → Plugins → Development → Import plugin from manifest...
   - 或者：菜单栏 → Plugins → Development → Import plugin from manifest...
   - 选择 manifest.json 文件

### 方法三：使用打包文件安装

如果直接导入有问题，可以使用打包文件：

1. **生成插件包**：
   ```bash
   cd D:\figma-deliver\service\figma-plugin
   .\package.bat
   ```

2. **安装插件包**：
   - 在 Figma 中：Plugins → Development → Import plugin from manifest...
   - 选择生成的 `figma-deliver-plugin.zip` 文件

## 🔍 故障排除

### 检查清单

- [ ] Figma 已更新到最新版本
- [ ] 已登录 Figma 账户
- [ ] 开发者模式已启用
- [ ] manifest.json 文件路径正确
- [ ] 文件结构完整
- [ ] 没有语法错误

### 常见错误信息

1. **"Cannot read manifest"**：
   - 检查 manifest.json 文件是否存在
   - 验证 JSON 格式是否正确

2. **"Plugin failed to load"**：
   - 检查 code.js 和 ui.html 文件是否存在
   - 查看控制台错误信息

3. **"Permission denied"**：
   - 确保文件夹有读取权限
   - 尝试以管理员身份运行 Figma

### 替代方案

如果仍然无法安装，可以考虑：

1. **使用 Figma 社区版本**：
   - 将插件发布到 Figma 社区
   - 通过社区安装

2. **使用在线版本**：
   - 部署插件到在线服务
   - 通过 URL 安装

3. **联系支持**：
   - Figma 官方支持：https://help.figma.com
   - 社区论坛：https://forum.figma.com

## 🎯 验证安装

安装成功后，验证插件是否正常工作：

1. **查看插件列表**：
   - Plugins 菜单中应该出现 "Figma Deliver"

2. **测试基本功能**：
   - 运行插件：Plugins → Figma Deliver → Export to Figma Deliver
   - 检查是否显示插件界面

3. **测试连接**：
   - 在插件中输入服务器地址：`http://localhost:8080`
   - 尝试登录验证连接

## 📞 获取帮助

如果问题仍然存在：

1. **检查系统要求**：
   - Windows 10+ 或 macOS 10.14+
   - 最新版本的 Chrome/Edge（网页版）

2. **收集错误信息**：
   - 截图错误消息
   - 复制控制台错误日志
   - 记录操作步骤

3. **寻求帮助**：
   - 查看 Figma 官方文档
   - 在开发者社区提问
   - 联系技术支持

## 💰 账户要求说明

### ✅ 好消息：完全免费！

**Figma 插件功能对所有用户免费开放**，包括：
- 插件开发和安装
- API 访问和调用
- 设计资源导出
- 本项目的所有功能

### 📋 免费 vs 付费账户

| 功能 | 免费账户 | 付费账户 |
|------|----------|----------|
| 插件开发 | ✅ | ✅ |
| 插件使用 | ✅ | ✅ |
| API 访问 | ✅ | ✅ |
| 团队协作 | 限制 | ✅ |
| 版本历史 | 30天 | 无限 |

### 🎯 使用建议

1. **个人开发**：免费账户完全够用
2. **团队项目**：考虑升级到 Professional
3. **企业使用**：可能需要 Organization 账户

---

**提示**：如果你是企业用户，可能需要 IT 管理员权限来安装插件。请联系你的 IT 部门获取帮助。
