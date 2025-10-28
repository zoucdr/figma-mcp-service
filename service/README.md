# Figma Bridge

Figma Bridge 是一个基于Go语言后端、MySQL数据库和H5前端的应用，用于对Figma界面进行二次定义。

## 功能特点

- 用户管理：支持用户注册、登录、退出，数据隔离保护
- Figma链接解析：输入链接，自动解析filekey和node
- 节点树预览：左侧展示树型预览图，直观查看节点结构
- 节点预览：中间展示节点选中预览图，所见即所得
- 节点属性编辑：右侧属性面板，灵活配置节点属性
  - 节点类型选择
  - 图片下载方式
  - 元素可视选择
  - 界面布局锚点调整
  - 节点图片文件重命名
  - 图片排除节点列表
- 界面导出：一键导出优化后的界面元数据和图片

## 技术栈

- 后端：Go + Gin框架
- 数据库：MySQL + GORM
- 前端：HTML5 + Vue.js + Element UI
- API集成：Figma API

## 项目结构

```
figma-bridge/
├── cmd/                    # 命令行入口
│   └── main.go             # 主程序入口
├── configs/                # 配置文件
│   └── config.example.env  # 环境变量示例
├── internal/               # 内部包
│   ├── controllers/        # 控制器
│   ├── middleware/         # 中间件
│   ├── models/             # 数据模型
│   └── services/           # 业务逻辑服务
└── web/                    # Web资源
    ├── static/             # 静态资源
    │   ├── css/            # CSS样式
    │   ├── js/             # JavaScript脚本
    │   └── img/            # 图片资源
    └── templates/          # HTML模板
```

## 安装与运行

### 前提条件

- Go 1.16+
- MySQL 5.7+
- Node.js 14+ (用于前端开发)

### 安装步骤

1. 克隆仓库

```bash
git clone https://github.com/yourusername/figma-bridge.git
cd figma-bridge
```

2. 复制环境配置文件

```bash
cp configs/config.example.env .env
```

3. 修改环境配置

编辑 `.env` 文件，设置数据库连接信息和其他配置。

4. 初始化数据库

```bash
# 创建数据库
mysql -u root -p -e "CREATE DATABASE figma_bridge CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
```

5. 构建并运行

```bash
go build -o figma-bridge ./cmd
./figma-bridge
```

6. 访问应用

打开浏览器，访问 http://localhost:8080

## 使用说明

1. 注册/登录：使用用户名和Figma Private Token注册
2. 添加项目：在仪表盘页面，点击"添加项目"，输入Figma链接
3. 编辑节点：在项目页面，左侧选择节点，右侧编辑属性
4. 导出设计：点击底部"导出设计"按钮，下载优化后的界面

## 贡献指南

欢迎贡献代码！请遵循以下步骤：

1. Fork 仓库
2. 创建功能分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add some amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 创建 Pull Request

## 许可证

本项目采用 MIT 许可证。详情请参阅 [LICENSE](LICENSE) 文件。
