# Figma Deliver Plugin

Figma Deliver Plugin 是一个 Figma 插件，用于将 Figma 设计导出到 Figma Deliver 服务。

## 功能

- 直接从 Figma 中导出设计到 Figma Deliver 服务
- 认证与 Figma Deliver 服务的连接
- 选择要导出的设计元素
- 设置项目名称和其他导出选项

## 安装

### 开发模式安装

1. 克隆此仓库
2. 打开 Figma 桌面应用
3. 点击菜单 `插件` > `开发` > `导入插件`
4. 选择此仓库中的 `manifest.json` 文件

### 用户安装

1. 下载最新的发布版本
2. 打开 Figma 桌面应用
3. 点击菜单 `插件` > `管理插件`
4. 点击 `+` 按钮，然后选择 `导入插件`
5. 选择下载的 `.fig` 文件

## 使用方法

### 认证

1. 启动插件后，如果未认证，将显示认证选项卡
2. 输入 Figma Deliver 服务的地址（例如：`http://localhost:8080`）
3. 输入您的用户名和密码
4. 点击 `登录` 按钮

### 导出设计

1. 在 Figma 中选择要导出的设计元素
2. 启动插件
3. 如果需要，可以修改项目名称
4. 点击 `导出到 Figma Deliver` 按钮
5. 等待导出完成

## 开发

### 项目结构

```
figma-plugin/
  ├── manifest.json    # 插件清单文件
  ├── src/
  │   └── code.js      # 插件主代码
  └── ui/
      ├── ui.html      # 插件 UI 界面
      └── service.js   # 服务通信层
```

### 构建

此插件不需要构建步骤，可以直接在 Figma 中加载。

## 配置

插件使用 Figma 的 `clientStorage` API 存储认证令牌和服务器地址。这些数据仅存储在本地，不会上传到任何服务器。

## 注意事项

- 插件需要连接到运行中的 Figma Deliver 服务才能工作
- 确保您有权访问 Figma Deliver 服务，并且已创建账户

## 故障排除

### 认证失败

- 检查服务器地址是否正确
- 确认用户名和密码是否正确
- 确认 Figma Deliver 服务是否正在运行

### 导出失败

- 确保已选择有效的设计元素
- 检查与服务器的连接
- 确认认证令牌是否有效

## 许可证

[MIT License](LICENSE)
