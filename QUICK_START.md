# Figma Deliver 快速开始指南

## 🚀 5分钟快速体验

### 步骤 1: 启动服务器
服务器已经在运行！访问：**http://localhost:8080**

### 步骤 2: 创建账户
1. 在浏览器中打开 http://localhost:8080
2. 点击"注册"选项卡
3. 填写用户名、密码和 Figma Token
4. 点击"注册"按钮

> **获取 Figma Token**: 
> 1. 登录 Figma 网页版
> 2. 进入 Settings > Account > Personal Access Tokens
> 3. 点击 "Create new token"
> 4. 复制生成的 token

### 步骤 3: 安装 Figma Plugin

#### 🔧 如果遇到安装问题，请查看：[详细安装指南](./FIGMA_PLUGIN_INSTALL_GUIDE.md)

#### 方法一：Figma 桌面版
1. 打开 Figma 桌面应用并登录
2. 确保开发者模式已启用（Settings > Developer）
3. 进入 `Plugins` > `Development` > `Import plugin from manifest...`
4. 选择项目中的 `figma-plugin/manifest.json` 文件
5. 插件安装完成！

#### 方法二：Figma 网页版（推荐）
1. 访问 https://www.figma.com 并登录
2. 打开任意设计文件
3. 右键点击画布 > `Plugins` > `Development` > `Import plugin from manifest...`
4. 选择 `figma-plugin/manifest.json` 文件
5. 插件安装完成！

#### 方法三：使用打包文件
```bash
cd figma-plugin
.\package.bat
```
然后在 Figma 中导入生成的 zip 文件

### 步骤 4: 配置插件
1. 在 Figma 中打开任意设计文件
2. 打开插件：`Plugins` > `Figma Deliver` > `Settings`
3. 服务器地址填写：`http://localhost:8080`
4. 输入刚才注册的用户名和密码
5. 点击"登录"

### 步骤 5: 体验功能

#### 🎨 导出设计稿
1. 在 Figma 中选择任意设计元素
2. 打开插件：`Plugins` > `Figma Deliver` > `Export to Figma Deliver`
3. 设置项目名称，点击"导出到服务器"
4. 等待导出完成

#### 👀 查看预览
1. 回到浏览器，访问 Dashboard：http://localhost:8080/dashboard
2. 点击刚才创建的项目
3. 查看设计预览 - **这里就是新功能！**
   - 如果在 Figma 环境中，会使用 Plugin API 快速预览
   - 否则会自动使用后端 API 预览

## 🔥 新功能亮点

### 预览系统
- **过滤预览功能**：必须在 Figma 中使用 Plugin API，速度超快 ⚡
- **单个节点预览**：智能切换，Figma 中用 Plugin API，浏览器中用后端 API 🔄
- **明确提示**：系统会明确告知用户预览功能的使用要求 💡

### 使用场景
1. **设计师**：在 Figma 中使用过滤预览，批量查看设计元素
2. **开发者**：在浏览器中查看单个节点预览，无需 Figma
3. **产品经理**：根据环境选择合适的预览方式

## 🛠️ 故障排除

### 问题：插件无法连接服务器
**解决**：确认服务器地址是 `http://localhost:8080`，检查网络连接

### 问题：预览图片加载失败
**解决**：
- 在 Figma 中：确保插件已正确安装和认证
- 在浏览器中：刷新页面，清除缓存

### 问题：导出失败
**解决**：确保选择了有效的设计元素，检查 Figma Token 是否正确

## 📚 更多功能

查看完整使用指南：[USAGE_GUIDE.md](./USAGE_GUIDE.md)

## 🎯 下一步

1. 尝试批量导出多个设计元素
2. 体验不同的预览格式（PNG/JPG）
3. 测试缩放功能
4. 邀请团队成员一起使用

---

**提示**：如果遇到任何问题，请查看服务器控制台日志或 Figma 开发者工具中的错误信息。
