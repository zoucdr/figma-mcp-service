// Android UI代码生成脚本示例
// context 对象包含:
// - nodeTree: Figma节点树（递归结构，包含children）
// - imageDict: 图片字典 { "节点ID": "图片文件名" }，例如 { "123:456": "images/123_456.png" }
// - config: 导出配置 { imageFormat, imageScale }
// - projectId: 项目ID
// - project: 项目信息

// 预览函数 - 返回代码文件的字典 { "文件名": "文件内容" }
function generatePreview(context) {
    const files = {};
    
    // 生成主布局文件
    files["activity_main.xml"] = generateAndroidLayout(context.nodeTree, context.imageDict);
    
    // 生成Activity代码
    files["MainActivity.kt"] = generateActivityCode(context);
    
    // 生成资源清单
    files["Resources.json"] = generateResourceManifest(context);
    
    return files;
}

// 导出函数 - 返回代码文件的字典 { "文件名": "文件内容" }
function generateExport(context) {
    const files = generatePreview(context);
    
    // 导出时生成额外的文件
    files["README.md"] = generateReadme(context);
    files["colors.xml"] = generateColorsXml(context.nodeTree);
    files["dimens.xml"] = generateDimensXml(context.nodeTree);
    
    return files;
}

// 生成Android布局XML
function generateAndroidLayout(nodeTree, imageDict) {
    let xml = '<?xml version="1.0" encoding="utf-8"?>\n';
    xml += '<androidx.constraintlayout.widget.ConstraintLayout\n';
    xml += '    xmlns:android="http://schemas.android.com/apk/res/android"\n';
    xml += '    xmlns:app="http://schemas.android.com/apk/res-auto"\n';
    xml += '    xmlns:tools="http://schemas.android.com/tools"\n';
    xml += '    android:layout_width="match_parent"\n';
    xml += '    android:layout_height="match_parent"\n';
    xml += '    tools:context=".MainActivity">\n\n';
    
    // 递归处理节点树
    function processNode(node, depth = 1) {
        if (!node) return;
        const indent = "    ".repeat(depth);
        const nodeId = (node.id || "unknown").replace(/[^a-zA-Z0-9]/g, "_");
        const nodeName = (node.name || "unknown").replace(/[^a-zA-Z0-9]/g, "_");
        
        xml += indent + "<!-- " + node.name + " (" + node.type + ") -->\n";
        
        if (node.type === "TEXT") {
            xml += indent + "<TextView\n";
            xml += indent + "    android:id=\"@+id/" + nodeName + "\"\n";
            xml += indent + "    android:layout_width=\"wrap_content\"\n";
            xml += indent + "    android:layout_height=\"wrap_content\"\n";
            xml += indent + "    android:text=\"" + (node.characters || node.name) + "\"\n";
            xml += indent + "    />\n\n";
        } else if (node.type === "RECTANGLE" || node.type === "FRAME") {
            // 检查是否有图片
            if (imageDict && imageDict[node.id]) {
                xml += indent + "<ImageView\n";
                xml += indent + "    android:id=\"@+id/" + nodeName + "\"\n";
                xml += indent + "    android:layout_width=\"wrap_content\"\n";
                xml += indent + "    android:layout_height=\"wrap_content\"\n";
                xml += indent + "    android:src=\"@drawable/" + nodeId + "\"\n";
                xml += indent + "    android:contentDescription=\"" + node.name + "\"\n";
                xml += indent + "    />\n\n";
            } else {
                xml += indent + "<View\n";
                xml += indent + "    android:id=\"@+id/" + nodeName + "\"\n";
                xml += indent + "    android:layout_width=\"wrap_content\"\n";
                xml += indent + "    android:layout_height=\"wrap_content\"\n";
                xml += indent + "    />\n\n";
            }
        }
        
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
    
    xml += "</androidx.constraintlayout.widget.ConstraintLayout>\n";
    return xml;
}

// 生成Activity代码
function generateActivityCode(context) {
    let code = "package com.example.app\n\n";
    code += "import android.os.Bundle\n";
    code += "import androidx.appcompat.app.AppCompatActivity\n\n";
    code += "class MainActivity : AppCompatActivity() {\n\n";
    code += "    override fun onCreate(savedInstanceState: Bundle?) {\n";
    code += "        super.onCreate(savedInstanceState)\n";
    code += "        setContentView(R.layout.activity_main)\n";
    code += "        \n";
    code += "        // TODO: Initialize views\n";
    code += "    }\n";
    code += "}\n";
    return code;
}

// 生成colors.xml
function generateColorsXml(nodeTree) {
    let xml = '<?xml version="1.0" encoding="utf-8"?>\n';
    xml += '<resources>\n';
    xml += '    <color name="purple_200">#FFBB86FC</color>\n';
    xml += '    <color name="purple_500">#FF6200EE</color>\n';
    xml += '    <color name="purple_700">#FF3700B3</color>\n';
    xml += '</resources>\n';
    return xml;
}

// 生成dimens.xml
function generateDimensXml(nodeTree) {
    let xml = '<?xml version="1.0" encoding="utf-8"?>\n';
    xml += '<resources>\n';
    xml += '    <dimen name="padding_small">8dp</dimen>\n';
    xml += '    <dimen name="padding_medium">16dp</dimen>\n';
    xml += '    <dimen name="padding_large">24dp</dimen>\n';
    xml += '</resources>\n';
    return xml;
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
        platform: "Android",
        totalResources: resources.length,
        resources: resources
    }, null, 2);
}

// 生成README
function generateReadme(context) {
    return "# " + (context.project?.name || "Android UI Export") + "\n\n" +
           "## 项目信息\n\n" +
           "- 平台: Android\n" +
           "- 项目ID: " + context.projectId + "\n" +
           "- 图片格式: " + context.config.imageFormat + "\n" +
           "- 缩放比例: " + context.config.imageScale + "x\n" +
           "- 导出时间: " + new Date().toLocaleString() + "\n\n" +
           "## 使用说明\n\n" +
           "1. 将布局文件复制到 `res/layout/` 目录\n" +
           "2. 将资源文件复制到 `res/` 对应目录\n" +
           "3. 将图片文件复制到 `res/drawable/` 目录\n";
}



