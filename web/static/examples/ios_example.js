// iOS (Swift + UIKit) UI代码生成脚本示例
// context 对象包含:
// - nodeTree: Figma节点树（递归结构，包含children）
// - imageDict: 图片字典 { "节点ID": "图片文件名" }，例如 { "123:456": "images/123_456.png" }
// - config: 导出配置 { imageFormat, imageScale }
// - projectId: 项目ID
// - project: 项目信息

// 预览函数 - 返回代码文件的字典 { "文件名": "文件内容" }
function generatePreview(context) {
    const files = {};
    
    // 生成主视图控制器
    files["ViewController.swift"] = generateViewController(context.nodeTree, context.imageDict);
    
    // 生成自定义视图
    files["CustomView.swift"] = generateCustomView(context.nodeTree, context.imageDict);
    
    // 生成资源清单
    files["Resources.json"] = generateResourceManifest(context);
    
    return files;
}

// 导出函数 - 返回代码文件的字典 { "文件名": "文件内容" }
function generateExport(context) {
    const files = generatePreview(context);
    
    // 导出时生成额外的文件
    files["README.md"] = generateReadme(context);
    files["Extensions.swift"] = generateExtensions();
    files["Constants.swift"] = generateConstants(context.nodeTree);
    
    return files;
}

// 生成ViewController
function generateViewController(nodeTree, imageDict) {
    let code = "import UIKit\n\n";
    code += "class ViewController: UIViewController {\n\n";
    code += "    // MARK: - Properties\n";
    code += "    private let customView = CustomView()\n\n";
    code += "    // MARK: - Lifecycle\n";
    code += "    override func viewDidLoad() {\n";
    code += "        super.viewDidLoad()\n";
    code += "        setupUI()\n";
    code += "        setupConstraints()\n";
    code += "    }\n\n";
    code += "    // MARK: - Setup\n";
    code += "    private func setupUI() {\n";
    code += "        view.backgroundColor = .white\n";
    code += "        view.addSubview(customView)\n";
    code += "    }\n\n";
    code += "    private func setupConstraints() {\n";
    code += "        customView.translatesAutoresizingMaskIntoConstraints = false\n";
    code += "        NSLayoutConstraint.activate([\n";
    code += "            customView.topAnchor.constraint(equalTo: view.safeAreaLayoutGuide.topAnchor),\n";
    code += "            customView.leadingAnchor.constraint(equalTo: view.leadingAnchor),\n";
    code += "            customView.trailingAnchor.constraint(equalTo: view.trailingAnchor),\n";
    code += "            customView.bottomAnchor.constraint(equalTo: view.bottomAnchor)\n";
    code += "        ])\n";
    code += "    }\n";
    code += "}\n";
    return code;
}

// 生成自定义视图
function generateCustomView(nodeTree, imageDict) {
    let code = "import UIKit\n\n";
    code += "class CustomView: UIView {\n\n";
    code += "    // MARK: - UI Components\n";
    
    // 递归处理节点树，生成属性声明
    function processNodeProperties(node, depth = 1) {
        if (!node) return;
        const indent = "    ".repeat(depth);
        const name = (node.name || "unknown").replace(/[^a-zA-Z0-9]/g, "_");
        
        if (node.type === "TEXT") {
            code += indent + "private let " + name + ": UILabel = {\n";
            code += indent + "    let label = UILabel()\n";
            code += indent + "    label.text = \"" + (node.characters || node.name) + "\"\n";
            code += indent + "    label.translatesAutoresizingMaskIntoConstraints = false\n";
            code += indent + "    return label\n";
            code += indent + "}()\n\n";
        } else if (node.type === "RECTANGLE" || node.type === "FRAME") {
            if (imageDict && imageDict[node.id]) {
                code += indent + "private let " + name + ": UIImageView = {\n";
                code += indent + "    let imageView = UIImageView()\n";
                code += indent + "    imageView.contentMode = .scaleAspectFit\n";
                code += indent + "    imageView.translatesAutoresizingMaskIntoConstraints = false\n";
                code += indent + "    // Image: " + imageDict[node.id] + "\n";
                code += indent + "    return imageView\n";
                code += indent + "}()\n\n";
            } else {
                code += indent + "private let " + name + ": UIView = {\n";
                code += indent + "    let view = UIView()\n";
                code += indent + "    view.translatesAutoresizingMaskIntoConstraints = false\n";
                code += indent + "    return view\n";
                code += indent + "}()\n\n";
            }
        }
        
        // 处理子节点
        if (node.children && node.children.length > 0) {
            for (let child of node.children) {
                processNodeProperties(child, depth);
            }
        }
    }
    
    if (nodeTree) {
        processNodeProperties(nodeTree);
    }
    
    code += "    // MARK: - Initialization\n";
    code += "    override init(frame: CGRect) {\n";
    code += "        super.init(frame: frame)\n";
    code += "        setupUI()\n";
    code += "    }\n\n";
    code += "    required init?(coder: NSCoder) {\n";
    code += "        fatalError(\"init(coder:) has not been implemented\")\n";
    code += "    }\n\n";
    code += "    // MARK: - Setup\n";
    code += "    private func setupUI() {\n";
    code += "        backgroundColor = .white\n";
    code += "        // TODO: Add subviews and setup constraints\n";
    code += "    }\n";
    code += "}\n";
    return code;
}

// 生成扩展
function generateExtensions() {
    let code = "import UIKit\n\n";
    code += "// MARK: - UIColor Extensions\n";
    code += "extension UIColor {\n";
    code += "    static func hex(_ hexString: String) -> UIColor {\n";
    code += "        let hex = hexString.trimmingCharacters(in: CharacterSet.alphanumerics.inverted)\n";
    code += "        var int: UInt64 = 0\n";
    code += "        Scanner(string: hex).scanHexInt64(&int)\n";
    code += "        let a, r, g, b: UInt64\n";
    code += "        switch hex.count {\n";
    code += "        case 6: (a, r, g, b) = (255, int >> 16, int >> 8 & 0xFF, int & 0xFF)\n";
    code += "        case 8: (a, r, g, b) = (int >> 24, int >> 16 & 0xFF, int >> 8 & 0xFF, int & 0xFF)\n";
    code += "        default: (a, r, g, b) = (255, 0, 0, 0)\n";
    code += "        }\n";
    code += "        return UIColor(red: CGFloat(r) / 255, green: CGFloat(g) / 255, blue: CGFloat(b) / 255, alpha: CGFloat(a) / 255)\n";
    code += "    }\n";
    code += "}\n";
    return code;
}

// 生成常量
function generateConstants(nodeTree) {
    let code = "import UIKit\n\n";
    code += "enum Constants {\n";
    code += "    enum Spacing {\n";
    code += "        static let small: CGFloat = 8\n";
    code += "        static let medium: CGFloat = 16\n";
    code += "        static let large: CGFloat = 24\n";
    code += "    }\n\n";
    code += "    enum CornerRadius {\n";
    code += "        static let small: CGFloat = 4\n";
    code += "        static let medium: CGFloat = 8\n";
    code += "        static let large: CGFloat = 12\n";
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
        platform: "iOS",
        totalResources: resources.length,
        resources: resources
    }, null, 2);
}

// 生成README
function generateReadme(context) {
    return "# " + (context.project?.name || "iOS UI Export") + "\n\n" +
           "## 项目信息\n\n" +
           "- 平台: iOS (Swift + UIKit)\n" +
           "- 项目ID: " + context.projectId + "\n" +
           "- 图片格式: " + context.config.imageFormat + "\n" +
           "- 缩放比例: " + context.config.imageScale + "x\n" +
           "- 导出时间: " + new Date().toLocaleString() + "\n\n" +
           "## 使用说明\n\n" +
           "1. 将Swift文件添加到Xcode项目中\n" +
           "2. 将图片资源添加到Assets.xcassets\n" +
           "3. 根据需要调整约束和样式\n";
}


