package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/figma-deliver/internal/models"
	"github.com/gin-gonic/gin"
)

// SwiftExportConfig Swift导出配置
type SwiftExportConfig struct {
	ImageFormat string   `json:"imageFormat"`
	ImageScale  float64  `json:"imageScale"`
	CodeStyle   string   `json:"codeStyle"`
	Options     []string `json:"options"`
}

// SwiftCodeGenerator Swift代码生成器
type SwiftCodeGenerator struct {
	project     *models.FigmaProject
	rootNode    gin.H
	nodeImages  map[string]string
	classNames  map[string]string
	constraints []string
	config      SwiftExportConfig
}

// SwiftUIComponent Swift UI组件定义
type SwiftUIComponent struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Frame      SwiftFrame             `json:"frame"`
	Properties map[string]interface{} `json:"properties"`
	Children   []SwiftUIComponent     `json:"children,omitempty"`
	ImagePath  string                 `json:"imagePath,omitempty"`
}

// SwiftFrame 坐标和尺寸信息
type SwiftFrame struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// ProcessSwiftExportJob 处理Swift代码导出任务
func ProcessSwiftExportJob(jobID uint, config SwiftExportConfig) {
	// 获取任务信息
	job, err := models.GetExportJob(jobID)
	if err != nil {
		return
	}

	// 更新任务状态为处理中
	models.UpdateExportJobStatus(jobID, "processing", 10, job.FilePath, "")

	// 获取项目信息
	project, err := models.GetProjectByID(job.ProjectID)
	if err != nil {
		models.UpdateExportJobStatus(jobID, "failed", 0, "", "获取项目信息失败: "+err.Error())
		return
	}

	// 获取用户信息
	var user models.User
	result := models.DB.First(&user, project.UserID)
	if result.Error != nil {
		models.UpdateExportJobStatus(jobID, "failed", 0, "", "获取用户信息失败: "+result.Error.Error())
		return
	}

	// 创建临时目录
	tempDir := filepath.Join("temp", fmt.Sprintf("swift_export_%d_%d", project.ID, time.Now().Unix()))
	err = os.MkdirAll(tempDir, os.ModePerm)
	if err != nil {
		models.UpdateExportJobStatus(jobID, "failed", 0, "", "创建临时目录失败: "+err.Error())
		return
	}
	defer os.RemoveAll(tempDir)

	// 获取优化后的节点数据（复用现有逻辑）
	rootNode, nodeImages, err := getOptimizedNodesForSwift(project, user.FigmaToken, user.ID)
	if err != nil {
		models.UpdateExportJobStatus(jobID, "failed", 0, "", "获取节点数据失败: "+err.Error())
		return
	}

	// 更新进度
	models.UpdateExportJobStatus(jobID, "processing", 30, job.FilePath, "")

	// 下载图片资源（复用现有的图片下载逻辑）
	err = downloadImagesForSwift(user.FigmaToken, project, nodeImages, tempDir, jobID, config.ImageFormat, config.ImageScale)
	if err != nil {
		models.UpdateExportJobStatus(jobID, "failed", 0, "", "下载图片失败: "+err.Error())
		return
	}

	// 更新进度
	models.UpdateExportJobStatus(jobID, "processing", 60, job.FilePath, "")

	// 生成Swift代码
	generator := &SwiftCodeGenerator{
		project:    project,
		rootNode:   rootNode,
		nodeImages: nodeImages,
		classNames: make(map[string]string),
		config:     config,
	}

	swiftFiles, err := generator.GenerateSwiftCode()
	if err != nil {
		models.UpdateExportJobStatus(jobID, "failed", 0, "", "生成Swift代码失败: "+err.Error())
		return
	}

	// 更新进度
	models.UpdateExportJobStatus(jobID, "processing", 80, job.FilePath, "")

	// 保存Swift文件
	err = saveSwiftFiles(tempDir, swiftFiles, project)
	if err != nil {
		models.UpdateExportJobStatus(jobID, "failed", 0, "", "保存Swift文件失败: "+err.Error())
		return
	}

	// 创建ZIP文件
	exportDir := filepath.Join("temp", "exports", fmt.Sprintf("%d", project.ID))
	err = os.MkdirAll(exportDir, os.ModePerm)
	if err != nil {
		models.UpdateExportJobStatus(jobID, "failed", 0, "", "创建导出目录失败: "+err.Error())
		return
	}

	zipPath := filepath.Join(exportDir, fmt.Sprintf("%s_swift.zip", sanitizeFileName(project.Name)))
	err = createZipArchiveFromDir(tempDir, zipPath)
	if err != nil {
		models.UpdateExportJobStatus(jobID, "failed", 0, "", "创建ZIP文件失败: "+err.Error())
		return
	}

	// 更新任务状态为完成
	models.UpdateExportJobStatus(jobID, "completed", 100, zipPath, "")
}

// GenerateSwiftPreview 生成Swift代码预览
func GenerateSwiftPreview(project *models.FigmaProject, config SwiftExportConfig) (string, error) {
	// 获取用户信息
	var user models.User
	result := models.DB.First(&user, project.UserID)
	if result.Error != nil {
		return "", fmt.Errorf("获取用户信息失败: %v", result.Error)
	}

	// 获取优化后的节点数据
	rootNode, nodeImages, err := getOptimizedNodesForSwift(project, user.FigmaToken, user.ID)
	if err != nil {
		return "", fmt.Errorf("获取节点数据失败: %v", err)
	}

	// 生成Swift代码
	generator := &SwiftCodeGenerator{
		project:    project,
		rootNode:   rootNode,
		nodeImages: nodeImages,
		classNames: make(map[string]string),
		config:     config,
	}

	// 生成主视图控制器预览
	preview := generator.generateMainViewController()
	return preview, nil
}

// GenerateSwiftFullPreview 生成完整的Swift代码预览（所有文件）
func GenerateSwiftFullPreview(project *models.FigmaProject, config SwiftExportConfig) (map[string]string, error) {
	// 获取用户信息
	var user models.User
	result := models.DB.First(&user, project.UserID)
	if result.Error != nil {
		return nil, fmt.Errorf("获取用户信息失败: %v", result.Error)
	}

	// 获取优化后的节点数据
	rootNode, nodeImages, err := getOptimizedNodesForSwift(project, user.FigmaToken, user.ID)
	if err != nil {
		return nil, fmt.Errorf("获取节点数据失败: %v", err)
	}

	// 生成Swift代码
	generator := &SwiftCodeGenerator{
		project:    project,
		rootNode:   rootNode,
		nodeImages: nodeImages,
		classNames: make(map[string]string),
		config:     config,
	}

	// 生成所有Swift文件
	swiftFiles, err := generator.GenerateSwiftCode()
	if err != nil {
		return nil, fmt.Errorf("生成Swift代码失败: %v", err)
	}

	return swiftFiles, nil
}

// GenerateSwiftCode 生成Swift代码
func (g *SwiftCodeGenerator) GenerateSwiftCode() (map[string]string, error) {
	swiftFiles := make(map[string]string)

	// 生成主视图控制器
	mainViewController := g.generateMainViewController()
	swiftFiles["ViewController.swift"] = mainViewController

	// 生成视图组件
	components := g.extractComponents(g.rootNode)
	for _, component := range components {
		componentCode := g.generateComponentCode(component)
		fileName := fmt.Sprintf("%sView.swift", component.Name)
		swiftFiles[fileName] = componentCode
	}

	// 根据配置生成可选文件
	for _, option := range g.config.Options {
		switch option {
		case "generateExtensions":
			extensionCode := g.generateExtensions()
			swiftFiles["UIView+Extensions.swift"] = extensionCode
		case "generateResourceManager":
			resourceCode := g.generateResourceManager()
			swiftFiles["ResourceManager.swift"] = resourceCode
		case "generateConstraintHelpers":
			constraintCode := g.generateConstraintHelpers()
			swiftFiles["ConstraintHelpers.swift"] = constraintCode
		}
	}

	return swiftFiles, nil
}

// generateMainViewController 生成主视图控制器
func (g *SwiftCodeGenerator) generateMainViewController() string {
	className := g.sanitizeClassName(g.project.Name + "ViewController")

	code := fmt.Sprintf(`//
//  %s.swift
//  Generated from Figma Design
//  Created on %s
//

import UIKit

class %s: UIViewController {
    
    // MARK: - UI Components
%s
    
    // MARK: - Lifecycle
    override func viewDidLoad() {
        super.viewDidLoad()
        setupUI()
        setupConstraints()
    }
    
    // MARK: - Setup Methods
    private func setupUI() {
        view.backgroundColor = UIColor.systemBackground
%s
    }
    
    private func setupConstraints() {
%s
    }
}
`, className, time.Now().Format("2006-01-02"), className,
		g.generatePropertyDeclarations(g.rootNode),
		g.generateUISetup(g.rootNode),
		g.generateConstraints(g.rootNode))

	return code
}

// generatePropertyDeclarations 生成属性声明
func (g *SwiftCodeGenerator) generatePropertyDeclarations(node gin.H) string {
	var declarations []string

	// 递归处理所有子节点
	if children, ok := node["children"].([]gin.H); ok {
		for _, child := range children {
			component := g.convertNodeToSwiftComponent(child)
			declarations = append(declarations, fmt.Sprintf("    private let %s = %s()",
				g.camelCase(component.Name), component.Type))

			// 递归处理子组件
			childDeclarations := g.generatePropertyDeclarations(child)
			if childDeclarations != "" {
				declarations = append(declarations, childDeclarations)
			}
		}
	}

	return strings.Join(declarations, "\n")
}

// generateUISetup 生成UI设置代码
func (g *SwiftCodeGenerator) generateUISetup(node gin.H) string {
	var setupCode []string

	if children, ok := node["children"].([]gin.H); ok {
		for _, child := range children {
			component := g.convertNodeToSwiftComponent(child)
			varName := g.camelCase(component.Name)

			// 设置基本属性
			if component.Type == "UILabel" {
				if text, ok := component.Properties["text"].(string); ok {
					setupCode = append(setupCode, fmt.Sprintf("        %s.text = \"%s\"", varName, text))
				}
				if textColor, ok := component.Properties["textColor"].(string); ok {
					setupCode = append(setupCode, fmt.Sprintf("        %s.textColor = %s", varName, textColor))
				}
				if fontSize, ok := component.Properties["fontSize"].(float64); ok {
					setupCode = append(setupCode, fmt.Sprintf("        %s.font = UIFont.systemFont(ofSize: %.1f)", varName, fontSize))
				}
			} else if component.Type == "UIImageView" {
				if imageName, ok := component.Properties["imageName"].(string); ok {
					setupCode = append(setupCode, fmt.Sprintf("        %s.image = UIImage(named: \"%s\")", varName, imageName))
				}
				if contentMode, ok := component.Properties["contentMode"].(string); ok {
					setupCode = append(setupCode, fmt.Sprintf("        %s.contentMode = .%s", varName, contentMode))
				}
			}

			// 设置背景色
			if backgroundColor, ok := component.Properties["backgroundColor"].(string); ok {
				setupCode = append(setupCode, fmt.Sprintf("        %s.backgroundColor = %s", varName, backgroundColor))
			}

			// 设置圆角
			if cornerRadius, ok := component.Properties["cornerRadius"].(float64); ok {
				setupCode = append(setupCode, fmt.Sprintf("        %s.layer.cornerRadius = %.1f", varName, cornerRadius))
			}

			// 添加到父视图
			setupCode = append(setupCode, fmt.Sprintf("        view.addSubview(%s)", varName))
			setupCode = append(setupCode, "")

			// 递归处理子组件
			childSetup := g.generateUISetup(child)
			if childSetup != "" {
				setupCode = append(setupCode, childSetup)
			}
		}
	}

	return strings.Join(setupCode, "\n")
}

// generateConstraints 生成约束代码
func (g *SwiftCodeGenerator) generateConstraints(node gin.H) string {
	var constraints []string
	var activateConstraints []string

	if children, ok := node["children"].([]gin.H); ok {
		for _, child := range children {
			component := g.convertNodeToSwiftComponent(child)
			varName := g.camelCase(component.Name)

			// 禁用自动约束
			constraints = append(constraints, fmt.Sprintf("        %s.translatesAutoresizingMaskIntoConstraints = false", varName))

			// 根据代码风格生成约束
			if g.config.CodeStyle == "uikit-autolayout" {
				// Auto Layout约束
				activateConstraints = append(activateConstraints,
					fmt.Sprintf("            // %s", component.Name))
				activateConstraints = append(activateConstraints,
					fmt.Sprintf("            %s.leadingAnchor.constraint(equalTo: view.leadingAnchor, constant: %.1f),", varName, component.Frame.X))
				activateConstraints = append(activateConstraints,
					fmt.Sprintf("            %s.topAnchor.constraint(equalTo: view.topAnchor, constant: %.1f),", varName, component.Frame.Y))
				activateConstraints = append(activateConstraints,
					fmt.Sprintf("            %s.widthAnchor.constraint(equalToConstant: %.1f),", varName, component.Frame.Width))
				activateConstraints = append(activateConstraints,
					fmt.Sprintf("            %s.heightAnchor.constraint(equalToConstant: %.1f),", varName, component.Frame.Height))
				activateConstraints = append(activateConstraints, "")
			}
		}
	}

	if len(activateConstraints) > 0 {
		constraints = append(constraints, "")
		constraints = append(constraints, "        NSLayoutConstraint.activate([")
		constraints = append(constraints, strings.Join(activateConstraints, "\n"))
		constraints = append(constraints, "        ])")
	}

	return strings.Join(constraints, "\n")
}

// convertNodeToSwiftComponent 节点类型转换为Swift UI组件
func (g *SwiftCodeGenerator) convertNodeToSwiftComponent(node gin.H) SwiftUIComponent {
	nodeType, _ := node["type"].(string)
	nodeName, _ := node["name"].(string)
	nodeID, _ := node["id"].(string)

	// 获取边界框信息
	frame := g.extractFrame(node)

	// 根据节点类型生成对应的Swift组件
	switch nodeType {
	case "TEXT":
		return g.createTextComponent(node, nodeName, frame)
	case "RECTANGLE", "ELLIPSE", "VECTOR":
		return g.createImageComponent(node, nodeName, frame, nodeID)
	case "FRAME", "GROUP", "INSTANCE":
		return g.createContainerComponent(node, nodeName, frame)
	default:
		return g.createGenericComponent(node, nodeName, frame)
	}
}

// createTextComponent 创建文本组件
func (g *SwiftCodeGenerator) createTextComponent(node gin.H, name string, frame SwiftFrame) SwiftUIComponent {
	properties := make(map[string]interface{})

	// 提取文本内容
	if characters, ok := node["characters"].(string); ok {
		properties["text"] = characters
	}

	// 提取字体样式
	if style, ok := node["style"].(map[string]interface{}); ok {
		if fontSize, ok := style["fontSize"].(float64); ok {
			properties["fontSize"] = fontSize
		}
		if fontFamily, ok := style["fontFamily"].(string); ok {
			properties["fontFamily"] = fontFamily
		}
		if fontWeight, ok := style["fontWeight"].(float64); ok {
			properties["fontWeight"] = fontWeight
		}
	}

	// 提取颜色信息
	if fills, ok := node["fills"].([]interface{}); ok && len(fills) > 0 {
		if fill, ok := fills[0].(map[string]interface{}); ok {
			if color, ok := fill["color"].(map[string]interface{}); ok {
				properties["textColor"] = g.convertColor(color)
			}
		}
	}

	return SwiftUIComponent{
		Name:       g.sanitizeClassName(name),
		Type:       "UILabel",
		Frame:      frame,
		Properties: properties,
	}
}

// createImageComponent 创建图片组件
func (g *SwiftCodeGenerator) createImageComponent(node gin.H, name string, frame SwiftFrame, nodeID string) SwiftUIComponent {
	properties := make(map[string]interface{})

	// 获取图片路径
	var imagePath string
	if imgPath, exists := g.nodeImages[nodeID]; exists {
		imagePath = imgPath
		properties["imageName"] = strings.TrimSuffix(imgPath, filepath.Ext(imgPath))
	}

	// 设置内容模式
	properties["contentMode"] = "scaleAspectFit"

	// 提取背景色
	if fills, ok := node["fills"].([]interface{}); ok && len(fills) > 0 {
		if fill, ok := fills[0].(map[string]interface{}); ok {
			if fillType, ok := fill["type"].(string); ok && fillType == "SOLID" {
				if color, ok := fill["color"].(map[string]interface{}); ok {
					properties["backgroundColor"] = g.convertColor(color)
				}
			}
		}
	}

	return SwiftUIComponent{
		Name:       g.sanitizeClassName(name),
		Type:       "UIImageView",
		Frame:      frame,
		Properties: properties,
		ImagePath:  imagePath,
	}
}

// createContainerComponent 创建容器组件
func (g *SwiftCodeGenerator) createContainerComponent(node gin.H, name string, frame SwiftFrame) SwiftUIComponent {
	properties := make(map[string]interface{})

	// 提取背景色
	if fills, ok := node["fills"].([]interface{}); ok && len(fills) > 0 {
		if fill, ok := fills[0].(map[string]interface{}); ok {
			if fillType, ok := fill["type"].(string); ok && fillType == "SOLID" {
				if color, ok := fill["color"].(map[string]interface{}); ok {
					properties["backgroundColor"] = g.convertColor(color)
				}
			}
		}
	}

	// 提取圆角信息
	if cornerRadius, ok := node["cornerRadius"].(float64); ok {
		properties["cornerRadius"] = cornerRadius
	}

	// 处理子组件
	var children []SwiftUIComponent
	if childrenData, ok := node["children"].([]gin.H); ok {
		for _, child := range childrenData {
			childComponent := g.convertNodeToSwiftComponent(child)
			children = append(children, childComponent)
		}
	}

	return SwiftUIComponent{
		Name:       g.sanitizeClassName(name),
		Type:       "UIView",
		Frame:      frame,
		Properties: properties,
		Children:   children,
	}
}

// createGenericComponent 创建通用组件
func (g *SwiftCodeGenerator) createGenericComponent(node gin.H, name string, frame SwiftFrame) SwiftUIComponent {
	return SwiftUIComponent{
		Name:       g.sanitizeClassName(name),
		Type:       "UIView",
		Frame:      frame,
		Properties: make(map[string]interface{}),
	}
}

// 辅助方法
func (g *SwiftCodeGenerator) extractFrame(node gin.H) SwiftFrame {
	frame := SwiftFrame{}

	if absoluteBoundingBox, ok := node["absoluteBoundingBox"].(map[string]interface{}); ok {
		if x, ok := absoluteBoundingBox["x"].(float64); ok {
			frame.X = x
		}
		if y, ok := absoluteBoundingBox["y"].(float64); ok {
			frame.Y = y
		}
		if width, ok := absoluteBoundingBox["width"].(float64); ok {
			frame.Width = width
		}
		if height, ok := absoluteBoundingBox["height"].(float64); ok {
			frame.Height = height
		}
	}

	return frame
}

func (g *SwiftCodeGenerator) convertColor(color map[string]interface{}) string {
	r, _ := color["r"].(float64)
	green, _ := color["g"].(float64)
	b, _ := color["b"].(float64)
	a := 1.0
	if alpha, ok := color["a"].(float64); ok {
		a = alpha
	}

	return fmt.Sprintf("UIColor(red: %.3f, green: %.3f, blue: %.3f, alpha: %.3f)", r, green, b, a)
}

func (g *SwiftCodeGenerator) sanitizeClassName(name string) string {
	// 清理类名，移除特殊字符，确保符合Swift命名规范
	cleaned := strings.ReplaceAll(name, " ", "")
	cleaned = strings.ReplaceAll(cleaned, "-", "")
	cleaned = strings.ReplaceAll(cleaned, "_", "")

	// 确保首字母大写
	if len(cleaned) > 0 {
		cleaned = strings.ToUpper(cleaned[:1]) + cleaned[1:]
	}

	return cleaned
}

func (g *SwiftCodeGenerator) camelCase(name string) string {
	// 转换为驼峰命名
	cleaned := g.sanitizeClassName(name)
	if len(cleaned) > 0 {
		cleaned = strings.ToLower(cleaned[:1]) + cleaned[1:]
	}
	return cleaned
}

// 生成可选文件的方法
func (g *SwiftCodeGenerator) generateExtensions() string {
	return `//
//  UIView+Extensions.swift
//  Generated from Figma Design
//  Created on ` + time.Now().Format("2006-01-02") + `
//

import UIKit

extension UIView {
    
    // MARK: - Corner Radius
    func setCornerRadius(_ radius: CGFloat) {
        layer.cornerRadius = radius
        layer.masksToBounds = true
    }
    
    // MARK: - Shadow
    func addShadow(color: UIColor = .black, opacity: Float = 0.2, offset: CGSize = CGSize(width: 0, height: 2), radius: CGFloat = 4) {
        layer.shadowColor = color.cgColor
        layer.shadowOpacity = opacity
        layer.shadowOffset = offset
        layer.shadowRadius = radius
        layer.masksToBounds = false
    }
    
    // MARK: - Border
    func setBorder(width: CGFloat, color: UIColor) {
        layer.borderWidth = width
        layer.borderColor = color.cgColor
    }
}

extension UIColor {
    
    // MARK: - Hex Color
    convenience init(hex: String) {
        let hex = hex.trimmingCharacters(in: CharacterSet.alphanumerics.inverted)
        var int: UInt64 = 0
        Scanner(string: hex).scanHexInt64(&int)
        let a, r, g, b: UInt64
        switch hex.count {
        case 3: // RGB (12-bit)
            (a, r, g, b) = (255, (int >> 8) * 17, (int >> 4 & 0xF) * 17, (int & 0xF) * 17)
        case 6: // RGB (24-bit)
            (a, r, g, b) = (255, int >> 16, int >> 8 & 0xFF, int & 0xFF)
        case 8: // ARGB (32-bit)
            (a, r, g, b) = (int >> 24, int >> 16 & 0xFF, int >> 8 & 0xFF, int & 0xFF)
        default:
            (a, r, g, b) = (1, 1, 1, 0)
        }
        
        self.init(
            red: CGFloat(r) / 255,
            green: CGFloat(g) / 255,
            blue: CGFloat(b) / 255,
            alpha: CGFloat(a) / 255
        )
    }
}`
}

func (g *SwiftCodeGenerator) generateResourceManager() string {
	return `//
//  ResourceManager.swift
//  Generated from Figma Design
//  Created on ` + time.Now().Format("2006-01-02") + `
//

import UIKit

class ResourceManager {
    
    static let shared = ResourceManager()
    
    private init() {}
    
    // MARK: - Images
    func image(named name: String) -> UIImage? {
        return UIImage(named: name)
    }
    
    // MARK: - Colors
    struct Colors {
        static let primary = UIColor(red: 0.247, green: 0.318, blue: 0.710, alpha: 1.0)
        static let secondary = UIColor(red: 0.5, green: 0.5, blue: 0.5, alpha: 1.0)
        static let background = UIColor.systemBackground
        static let text = UIColor.label
    }
    
    // MARK: - Fonts
    struct Fonts {
        static let title = UIFont.systemFont(ofSize: 24, weight: .bold)
        static let subtitle = UIFont.systemFont(ofSize: 18, weight: .medium)
        static let body = UIFont.systemFont(ofSize: 16, weight: .regular)
        static let caption = UIFont.systemFont(ofSize: 14, weight: .regular)
    }
}`
}

func (g *SwiftCodeGenerator) generateConstraintHelpers() string {
	return `//
//  ConstraintHelpers.swift
//  Generated from Figma Design
//  Created on ` + time.Now().Format("2006-01-02") + `
//

import UIKit

extension UIView {
    
    // MARK: - Constraint Helpers
    func pinToSuperview(insets: UIEdgeInsets = .zero) {
        guard let superview = superview else { return }
        translatesAutoresizingMaskIntoConstraints = false
        NSLayoutConstraint.activate([
            topAnchor.constraint(equalTo: superview.topAnchor, constant: insets.top),
            leadingAnchor.constraint(equalTo: superview.leadingAnchor, constant: insets.left),
            trailingAnchor.constraint(equalTo: superview.trailingAnchor, constant: -insets.right),
            bottomAnchor.constraint(equalTo: superview.bottomAnchor, constant: -insets.bottom)
        ])
    }
    
    func centerInSuperview() {
        guard let superview = superview else { return }
        translatesAutoresizingMaskIntoConstraints = false
        NSLayoutConstraint.activate([
            centerXAnchor.constraint(equalTo: superview.centerXAnchor),
            centerYAnchor.constraint(equalTo: superview.centerYAnchor)
        ])
    }
    
    func setSize(width: CGFloat, height: CGFloat) {
        translatesAutoresizingMaskIntoConstraints = false
        NSLayoutConstraint.activate([
            widthAnchor.constraint(equalToConstant: width),
            heightAnchor.constraint(equalToConstant: height)
        ])
    }
}`
}

// 其他辅助方法
func (g *SwiftCodeGenerator) extractComponents(node gin.H) []SwiftUIComponent {
	var components []SwiftUIComponent

	if children, ok := node["children"].([]gin.H); ok {
		for _, child := range children {
			component := g.convertNodeToSwiftComponent(child)
			components = append(components, component)
		}
	}

	return components
}

func (g *SwiftCodeGenerator) generateComponentCode(component SwiftUIComponent) string {
	className := g.sanitizeClassName(component.Name + "View")

	code := fmt.Sprintf(`//
//  %s.swift
//  Generated from Figma Design
//  Created on %s
//

import UIKit

class %s: UIView {
    
    // MARK: - UI Components
%s
    
    // MARK: - Initialization
    override init(frame: CGRect) {
        super.init(frame: frame)
        setupUI()
        setupConstraints()
    }
    
    required init?(coder: NSCoder) {
        super.init(coder: coder)
        setupUI()
        setupConstraints()
    }
    
    // MARK: - Setup Methods
    private func setupUI() {
%s
    }
    
    private func setupConstraints() {
%s
    }
}
`, className, time.Now().Format("2006-01-02"), className,
		g.generateComponentProperties(component),
		g.generateComponentUISetup(component),
		g.generateComponentConstraints(component))

	return code
}

func (g *SwiftCodeGenerator) generateComponentProperties(component SwiftUIComponent) string {
	var properties []string

	for _, child := range component.Children {
		properties = append(properties, fmt.Sprintf("    private let %s = %s()",
			g.camelCase(child.Name), child.Type))
	}

	return strings.Join(properties, "\n")
}

func (g *SwiftCodeGenerator) generateComponentUISetup(component SwiftUIComponent) string {
	var setupCode []string

	for _, child := range component.Children {
		varName := g.camelCase(child.Name)
		setupCode = append(setupCode, fmt.Sprintf("        addSubview(%s)", varName))
	}

	return strings.Join(setupCode, "\n")
}

func (g *SwiftCodeGenerator) generateComponentConstraints(component SwiftUIComponent) string {
	var constraints []string

	for _, child := range component.Children {
		varName := g.camelCase(child.Name)
		constraints = append(constraints, fmt.Sprintf("        %s.translatesAutoresizingMaskIntoConstraints = false", varName))
	}

	return strings.Join(constraints, "\n")
}

// 获取优化节点数据（复用现有逻辑）
func getOptimizedNodesForSwift(project *models.FigmaProject, token string, userID uint) (gin.H, map[string]string, error) {
	// 获取完整的Figma节点树数据（支持 WebSocket 兜底，会自动等待响应）
	figmaNodes, err := GetFigmaNodesWithUser(token, project.FileKey, project.RootNodeID, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("获取节点树数据失败: %v", err)
	}

	// 获取所有节点的修改信息
	var nodes []models.FigmaNode
	models.DB.Where("project_id = ?", project.ID).Find(&nodes)

	nodeSettings := make(map[string]map[string]interface{})
	for _, node := range nodes {
		var settings map[string]interface{}
		if err := json.Unmarshal([]byte(node.Modifys), &settings); err == nil {
			nodeSettings[node.NodeID] = settings
		}
	}

	// 过滤掉不可见和被忽略的节点
	filteredNodes := filterVisibleAndNonIgnoredNodesForExport(figmaNodes, nodeSettings)

	// 构建优化后的节点数据
	optimizedNodes := make([]gin.H, 0)
	for _, node := range filteredNodes {
		nodeID := node["id"].(string)
		optimizedNode := gin.H{}

		// 复制所有Figma原始属性
		for key, value := range node {
			if key == "children" {
				continue
			}
			optimizedNode[key] = value
		}

		// 应用修改信息
		if modifys, exists := nodeSettings[nodeID]; exists {
			applyModificationsToNodeForExport(optimizedNode, modifys)
		}

		optimizedNodes = append(optimizedNodes, optimizedNode)
	}

	// 构建树结构
	nodeMap := make(map[string]gin.H)
	childrenMap := make(map[string][]gin.H)

	for _, node := range optimizedNodes {
		nodeID := node["id"].(string)
		nodeMap[nodeID] = node
	}

	for _, node := range optimizedNodes {
		parentID := node["parent_id"]
		if parentID != nil && parentID != project.RootNodeID {
			if parentIDStr, ok := parentID.(string); ok {
				childrenMap[parentIDStr] = append(childrenMap[parentIDStr], node)
			}
		}
	}

	// 为每个节点添加子节点信息
	for i := range optimizedNodes {
		nodeID := optimizedNodes[i]["id"].(string)
		delete(optimizedNodes[i], "children")

		if children, hasChildren := childrenMap[nodeID]; hasChildren && len(children) > 0 {
			optimizedNodes[i]["children"] = children
		}
	}

	// 找到根节点
	var rootNode gin.H
	for i, node := range optimizedNodes {
		parentID := node["parent_id"]
		if parentID == nil || parentID == project.RootNodeID {
			rootNode = optimizedNodes[i]
			break
		}
	}

	if rootNode == nil {
		return nil, nil, fmt.Errorf("未找到根节点")
	}

	// 移除parent_id字段
	removeParentIDFromTree(rootNode)

	// 提取需要下载的图片列表
	nodeImages := make(map[string]string)
	// 这里可以根据需要实现图片路径映射逻辑

	return rootNode, nodeImages, nil
}

// 下载图片资源
func downloadImagesForSwift(token string, project *models.FigmaProject, nodeImages map[string]string, tempDir string, jobID uint, format string, scale float64) error {
	// 这里可以实现图片下载逻辑，复用现有的图片下载功能
	// 暂时返回nil，表示成功
	return nil
}

// 保存Swift文件
func saveSwiftFiles(tempDir string, swiftFiles map[string]string, project *models.FigmaProject) error {
	for fileName, content := range swiftFiles {
		filePath := filepath.Join(tempDir, fileName)
		err := os.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			return fmt.Errorf("保存文件 %s 失败: %v", fileName, err)
		}
	}

	// 创建README文件
	readmeContent := fmt.Sprintf(`# %s - Swift UIKit Code

This Swift code was automatically generated from Figma design.

## Files

- ViewController.swift: Main view controller
- *View.swift: Individual UI components
- UIView+Extensions.swift: Useful UIView extensions (optional)
- ResourceManager.swift: Resource management utilities (optional)
- ConstraintHelpers.swift: Auto Layout helper methods (optional)

## Usage

1. Add these files to your Xcode project
2. Import the image assets to your project's asset catalog
3. Update the view controller class name if needed
4. Customize the code as needed for your app

## Generated on: %s
`, project.Name, time.Now().Format("2006-01-02 15:04:05"))

	readmePath := filepath.Join(tempDir, "README.md")
	return os.WriteFile(readmePath, []byte(readmeContent), 0644)
}

// 清理文件名
func sanitizeFileName(name string) string {
	// 移除或替换不安全的文件名字符
	cleaned := strings.ReplaceAll(name, " ", "_")
	cleaned = strings.ReplaceAll(cleaned, "/", "_")
	cleaned = strings.ReplaceAll(cleaned, "\\", "_")
	cleaned = strings.ReplaceAll(cleaned, ":", "_")
	cleaned = strings.ReplaceAll(cleaned, "*", "_")
	cleaned = strings.ReplaceAll(cleaned, "?", "_")
	cleaned = strings.ReplaceAll(cleaned, "\"", "_")
	cleaned = strings.ReplaceAll(cleaned, "<", "_")
	cleaned = strings.ReplaceAll(cleaned, ">", "_")
	cleaned = strings.ReplaceAll(cleaned, "|", "_")
	return cleaned
}
