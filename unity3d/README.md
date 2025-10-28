# Figma2UGUI

Figma2UGUI 是一个用于将 Figma 设计导入到 Unity UGUI 系统的工具。它可以解析 JSON 数据和图片，自动生成对应的 UGUI 界面。

## 功能特点

- 从 JSON 数据和图片文件夹生成 UGUI 界面
- 支持文本、图片、按钮等多种 UI 元素
- 支持圆角矩形、颜色填充等特殊效果
- 支持字体映射和图片文件夹配置
- 支持图片批量处理和重复图片合并
- 支持 TextMeshPro 文本组件
- 异步处理大型界面，不阻塞编辑器

## 安装方法

1. 将整个项目导入到 Unity 工程中
2. 确保项目中包含 TextMeshPro 包（如果需要使用 TMP 文本）

## 使用方法

### 配置设置

1. 打开 Unity 编辑器
2. 进入 `Edit > Project Settings > Figma2UGUI`
3. 配置字体映射、图片文件夹和其他设置

### 导入界面

1. 打开 `Window > Figma2UGUI > 导入界面`
2. 选择 JSON 文件路径
3. 选择纹理文件夹路径
4. 设置输出文件夹和其他选项
5. 点击 "导入界面" 按钮

## JSON 数据格式

JSON 数据应该是一个树形结构，包含以下关键字段：

```json
{
  "name": "节点名称",
  "type": "节点类型",
  "rect": {
    "x": 0,
    "y": 0,
    "width": 100,
    "height": 100
  },
  "visible": true,
  "anchorMin": [0.5, 0.5],
  "anchorMax": [0.5, 0.5],
  "pivot": [0.5, 0.5],
  "sizeDelta": [100, 100],
  "anchoredPosition": [0, 0],
  "imagePath": "图片路径",
  "hasImage": false,
  "backgroundColor": [1, 1, 1, 1],
  "cornerRadius": 0,
  "isText": false,
  "text": "文本内容",
  "fontFamily": "字体名称",
  "fontSize": 14,
  "textAlignment": 4,
  "fontStyle": 0,
  "textColor": [0, 0, 0, 1],
  "richText": false,
  "lineSpacing": 1.0,
  "componentType": 0,
  "componentProperties": {},
  "children": []
}
```

## 文件夹结构

```
Assets/
  ├── Editor/
  │   ├── Ctrl/
  │   │   └── UGUIGenerater.cs
  │   ├── GUI/
  │   │   ├── UIImportSettingProvider.cs
  │   │   └── UIImportWindow.cs
  │   └── Tools/
  │       ├── AssetManager.cs
  │       └── ImageProcessor.cs
  └── Runtime/
      └── Model/
          ├── UIImportSetting.cs
          └── UINode.cs
```

## 注意事项

- 确保 JSON 数据格式正确，否则可能无法正确解析
- 图片文件应该是 PNG 或 JPG 格式
- 字体映射需要在设置中正确配置
- 如果使用 TextMeshPro，需要在设置中启用并配置 TMP 字体

## 扩展开发

如需扩展功能，可以修改以下关键类：

- `UINode`: 修改节点数据结构
- `UGUIGenerater`: 修改 UI 生成逻辑
- `ImageProcessor`: 修改图片处理逻辑
- `UIImportSettingProvider`: 修改设置界面

## 许可证

MIT License