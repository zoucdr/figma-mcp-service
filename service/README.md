# Figma Bridge

Figma Bridge是一个用于连接Figma设计和开发工具的应用程序。它允许用户导入Figma设计，查看节点树，并对节点进行修改。

## 新增功能

### Figma节点树加载

现在，当访问`http://127.0.0.1:8080/dashboard?project=1`时，应用程序会：

1. 从Figma API获取对应项目的节点树
2. 从数据库加载该项目的所有节点修改信息
3. 将修改信息叠加到节点树上
4. 在界面左侧显示完整的节点树

### API端点

新增了两个API端点：

- `GET /figma/project/:project_id/node-tree`：获取Figma节点树
- `GET /figma/project/:project_id/node-modifys`：获取项目的节点修改信息

### 界面更新

- 树形节点显示更加直观，包括图标和标签
- 已修改的节点会以不同的样式显示
- 添加了刷新按钮，可以重新加载节点树
- 预览区域显示当前选中节点的名称和类型

## 使用方法

1. 启动服务器：`go run cmd/main.go`
2. 访问：`http://localhost:8080`
3. 登录后，创建或选择一个Figma项目
4. 在项目编辑页面，左侧会显示完整的节点树
5. 点击节点可以查看和修改节点属性

## 技术实现

- 前端：Vue.js + Element UI
- 后端：Go + Gin + GORM
- 数据存储：MySQL
- 外部API：Figma API

## 数据流程

1. 用户访问项目编辑页面
2. 前端调用`/figma/project/:project_id/node-tree`获取节点树
3. 前端调用`/figma/project/:project_id/node-modifys`获取修改信息
4. 前端将修改信息叠加到节点树上
5. 用户选择节点时，显示节点属性和预览
6. 用户修改节点属性，保存到数据库