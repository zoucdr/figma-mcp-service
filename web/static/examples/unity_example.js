// Unity AppUI UI代码生成脚本示例
// context 对象包含:
// - nodeTree: Figma节点树（递归结构，包含children）
// - imageDict: 图片字典 { "节点ID": "图片文件名" }，例如 { "123:456": "images/123_456.png" }
// - config: 导出配置 { imageFormat, imageScale }
// - projectId: 项目ID
// - project: 项目信息

// 预览函数 - 返回代码文件的字典 { "文件名": "文件内容" }
function generatePreview(context) {
    const files = {};
    
    // 生成主UI脚本
    files["UIManager.cs"] = generateUIManager(context.nodeTree, context.imageDict);
    
    // 生成UI元素脚本
    files["UIElements.cs"] = generateUIElements(context.nodeTree, context.imageDict);
    
    // 生成UXML
    files["MainUI.uxml"] = generateUXML(context.nodeTree, context.imageDict);
    
    // 生成USS样式表
    files["MainUI.uss"] = generateUSS(context.nodeTree);
    
    // 生成资源清单
    files["Resources.json"] = generateResourceManifest(context);
    
    return files;
}

// 导出函数 - 返回代码文件的字典 { "文件名": "文件内容" }
function generateExport(context) {
    const files = generatePreview(context);
    
    // 导出时生成额外的文件
    files["README.md"] = generateReadme(context);
    files["UIConstants.cs"] = generateConstants();
    
    return files;
}

// 生成UIManager
function generateUIManager(nodeTree, imageDict) {
    let code = "using UnityEngine;\n";
    code += "using UnityEngine.UIElements;\n\n";
    code += "public class UIManager : MonoBehaviour\n";
    code += "{\n";
    code += "    [SerializeField] private UIDocument uiDocument;\n\n";
    code += "    // UI Elements\n";
    
    // 递归处理节点树，生成字段声明
    function processNodeFields(node, depth = 1) {
        if (!node) return;
        const indent = "    ".repeat(depth);
        const name = (node.name || "unknown").replace(/[^a-zA-Z0-9]/g, "_");
        
        if (node.type === "TEXT") {
            code += indent + "private Label " + name + ";\n";
        } else if (node.type === "RECTANGLE" || node.type === "FRAME") {
            if (imageDict && imageDict[node.id]) {
                code += indent + "private VisualElement " + name + ";\n";
            } else {
                code += indent + "private VisualElement " + name + ";\n";
            }
        }
        
        // 处理子节点
        if (node.children && node.children.length > 0) {
            for (let child of node.children) {
                processNodeFields(child, depth);
            }
        }
    }
    
    if (nodeTree) {
        processNodeFields(nodeTree);
    }
    
    code += "\n    private void OnEnable()\n";
    code += "    {\n";
    code += "        var root = uiDocument.rootVisualElement;\n";
    code += "        InitializeElements(root);\n";
    code += "        BindEvents();\n";
    code += "    }\n\n";
    code += "    private void InitializeElements(VisualElement root)\n";
    code += "    {\n";
    code += "        // Query UI elements by name\n";
    
    // 生成元素初始化代码
    function processNodeInit(node, depth = 2) {
        if (!node) return;
        const indent = "    ".repeat(depth);
        const name = (node.name || "unknown").replace(/[^a-zA-Z0-9]/g, "_");
        
        if (node.type === "TEXT") {
            code += indent + name + " = root.Q<Label>(\"" + name + "\");\n";
        } else if (node.type === "RECTANGLE" || node.type === "FRAME") {
            code += indent + name + " = root.Q<VisualElement>(\"" + name + "\");\n";
        }
        
        if (node.children && node.children.length > 0) {
            for (let child of node.children) {
                processNodeInit(child, depth);
            }
        }
    }
    
    if (nodeTree) {
        processNodeInit(nodeTree);
    }
    
    code += "    }\n\n";
    code += "    private void BindEvents()\n";
    code += "    {\n";
    code += "        // Bind button clicks and other events here\n";
    code += "    }\n";
    code += "}\n";
    return code;
}

// 生成UIElements
function generateUIElements(nodeTree, imageDict) {
    let code = "using UnityEngine;\n";
    code += "using UnityEngine.UIElements;\n\n";
    code += "// Custom UI Element classes\n\n";
    code += "public class CustomButton : Button\n";
    code += "{\n";
    code += "    public new class UxmlFactory : UxmlFactory<CustomButton, UxmlTraits> { }\n\n";
    code += "    public CustomButton()\n";
    code += "    {\n";
    code += "        // Initialize custom button\n";
    code += "    }\n";
    code += "}\n";
    return code;
}

// 生成UXML
function generateUXML(nodeTree, imageDict) {
    let xml = '<?xml version="1.0" encoding="utf-8"?>\n';
    xml += '<ui:UXML\n';
    xml += '    xmlns:ui="UnityEngine.UIElements"\n';
    xml += '    xmlns:uie="UnityEditor.UIElements"\n';
    xml += '    xsi="http://www.w3.org/2001/XMLSchema-instance"\n';
    xml += '    engine="UnityEngine.UIElements"\n';
    xml += '    editor="UnityEditor.UIElements"\n';
    xml += '    noNamespaceSchemaLocation="../../UIElementsSchema/UIElements.xsd"\n';
    xml += '    editor-extension-mode="False">\n\n';
    
    // 递归处理节点树
    function processNode(node, depth = 1) {
        if (!node) return;
        const indent = "    ".repeat(depth);
        const name = (node.name || "unknown").replace(/[^a-zA-Z0-9]/g, "_");
        
        xml += indent + "<!-- " + node.name + " (" + node.type + ") -->\n";
        
        if (node.type === "TEXT") {
            xml += indent + '<ui:Label name="' + name + '" text="' + (node.characters || node.name) + '" />\n';
        } else if (node.type === "RECTANGLE" || node.type === "FRAME") {
            if (imageDict && imageDict[node.id]) {
                xml += indent + '<ui:VisualElement name="' + name + '" class="image-element">\n';
                xml += indent + '    <!-- Image: ' + imageDict[node.id] + ' -->\n';
                xml += indent + '</ui:VisualElement>\n';
            } else {
                xml += indent + '<ui:VisualElement name="' + name + '" />\n';
            }
        }
        
        xml += "\n";
        
        // 处理子节点
        if (node.children && node.children.length > 0) {
            for (let child of node.children) {
                processNode(child, depth);
            }
        }
    }
    
    if (nodeTree) {
        processNode(nodeTree);
    }
    
    xml += '</ui:UXML>\n';
    return xml;
}

// 生成USS样式表
function generateUSS(nodeTree) {
    let css = "/* Unity UI Toolkit Stylesheet */\n\n";
    css += ".container {\n";
    css += "    flex-grow: 1;\n";
    css += "    background-color: rgb(255, 255, 255);\n";
    css += "}\n\n";
    css += ".image-element {\n";
    css += "    width: 100px;\n";
    css += "    height: 100px;\n";
    css += "    background-image: resource('Sprites/placeholder');\n";
    css += "}\n\n";
    css += ".text-element {\n";
    css += "    color: rgb(0, 0, 0);\n";
    css += "    font-size: 14px;\n";
    css += "}\n";
    return css;
}

// 生成常量
function generateConstants() {
    let code = "using UnityEngine;\n\n";
    code += "public static class UIConstants\n";
    code += "{\n";
    code += "    public static class Spacing\n";
    code += "    {\n";
    code += "        public const float Small = 8f;\n";
    code += "        public const float Medium = 16f;\n";
    code += "        public const float Large = 24f;\n";
    code += "    }\n\n";
    code += "    public static class Colors\n";
    code += "    {\n";
    code += "        public static readonly Color Primary = new Color(0.2f, 0.4f, 0.8f);\n";
    code += "        public static readonly Color Secondary = new Color(0.5f, 0.5f, 0.5f);\n";
    code += "        public static readonly Color Background = Color.white;\n";
    code += "    }\n";
    code += "}\n";
    return code;
}

// 生成资源清单
function generateResourceManifest(context) {
    const resources = [];
    
    function collectResources(node) {
        if (!node) return;
        if (context.imageDict && context.imageDict[node.id]) {
            resources.push({
                nodeId: node.id,
                nodeName: node.name,
                imagePath: context.imageDict[node.id]
            });
        }
        if (node.children) {
            node.children.forEach(collectResources);
        }
    }
    
    collectResources(context.nodeTree);
    
    return JSON.stringify({
        platform: "Unity AppUI",
        totalResources: resources.length,
        resources: resources
    }, null, 2);
}

// 生成README
function generateReadme(context) {
    return "# " + (context.project?.name || "Unity AppUI Export") + "\n\n" +
           "## 项目信息\n\n" +
           "- 平台: Unity AppUI (UI Toolkit)\n" +
           "- 项目ID: " + context.projectId + "\n" +
           "- 图片格式: " + context.config.imageFormat + "\n" +
           "- 缩放比例: " + context.config.imageScale + "x\n" +
           "- 导出时间: " + new Date().toLocaleString() + "\n\n" +
           "## 使用说明\n\n" +
           "1. 将C#脚本文件添加到Unity项目的Scripts文件夹\n" +
           "2. 将UXML和USS文件添加到UI文件夹\n" +
           "3. 将图片资源导入到Resources/Sprites文件夹\n" +
           "4. 在场景中创建UI Document组件并关联UXML文件\n" +
           "5. 添加UIManager脚本到GameObject上\n";
}


