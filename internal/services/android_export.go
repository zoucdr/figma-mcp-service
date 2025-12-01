package services

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/figma-deliver/internal/models"
	"github.com/gin-gonic/gin"
)

// AndroidExportConfig Android导出配置
type AndroidExportConfig struct {
	ImageFormat string   `json:"imageFormat"`
	ImageScale  float64  `json:"imageScale"`
	CodeStyle   string   `json:"codeStyle"`
	Options     []string `json:"options"`
}

// AndroidCodeGenerator Android代码生成器
type AndroidCodeGenerator struct {
	project    *models.FigmaProject
	rootNode   gin.H
	nodeImages map[string]string
	classNames map[string]string
	resources  []string
	config     AndroidExportConfig
}

// AndroidUIComponent Android UI组件定义
type AndroidUIComponent struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Frame      AndroidFrame           `json:"frame"`
	Properties map[string]interface{} `json:"properties"`
	Children   []AndroidUIComponent   `json:"children,omitempty"`
	ImagePath  string                 `json:"imagePath,omitempty"`
}

// AndroidFrame 坐标和尺寸信息
type AndroidFrame struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// ProcessAndroidExportJob 处理Android代码导出任务
func ProcessAndroidExportJob(jobID uint, config AndroidExportConfig) {
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
	tempDir := filepath.Join("temp", fmt.Sprintf("android_export_%d_%d", project.ID, time.Now().Unix()))
	err = os.MkdirAll(tempDir, os.ModePerm)
	if err != nil {
		models.UpdateExportJobStatus(jobID, "failed", 0, "", "创建临时目录失败: "+err.Error())
		return
	}
	defer os.RemoveAll(tempDir)

	// 获取优化后的节点数据（复用现有逻辑）
	rootNode, nodeImages, err := getOptimizedNodesForAndroid(project, user.FigmaToken, user.ID)
	if err != nil {
		models.UpdateExportJobStatus(jobID, "failed", 0, "", "获取节点数据失败: "+err.Error())
		return
	}

	// 更新进度
	models.UpdateExportJobStatus(jobID, "processing", 30, job.FilePath, "")

	// 下载图片资源（复用现有的图片下载逻辑）
	err = downloadImagesForAndroid(user.FigmaToken, project, nodeImages, tempDir, jobID, config.ImageFormat, config.ImageScale)
	if err != nil {
		models.UpdateExportJobStatus(jobID, "failed", 0, "", "下载图片失败: "+err.Error())
		return
	}

	// 更新进度
	models.UpdateExportJobStatus(jobID, "processing", 60, job.FilePath, "")

	// 生成Android代码
	generator := &AndroidCodeGenerator{
		project:    project,
		rootNode:   rootNode,
		nodeImages: nodeImages,
		classNames: make(map[string]string),
		config:     config,
	}

	androidFiles, err := generator.GenerateAndroidCode()
	if err != nil {
		models.UpdateExportJobStatus(jobID, "failed", 0, "", "生成Android代码失败: "+err.Error())
		return
	}

	// 更新进度
	models.UpdateExportJobStatus(jobID, "processing", 80, job.FilePath, "")

	// 保存Android文件
	err = saveAndroidFiles(tempDir, androidFiles, project)
	if err != nil {
		models.UpdateExportJobStatus(jobID, "failed", 0, "", "保存Android文件失败: "+err.Error())
		return
	}

	// 创建ZIP文件
	exportDir := filepath.Join("temp", "exports", fmt.Sprintf("%d", project.ID))
	err = os.MkdirAll(exportDir, os.ModePerm)
	if err != nil {
		models.UpdateExportJobStatus(jobID, "failed", 0, "", "创建导出目录失败: "+err.Error())
		return
	}

	zipPath := filepath.Join(exportDir, fmt.Sprintf("%s_android.zip", sanitizeFileName(project.Name)))
	err = createZipArchiveFromDir(tempDir, zipPath)
	if err != nil {
		models.UpdateExportJobStatus(jobID, "failed", 0, "", "创建ZIP文件失败: "+err.Error())
		return
	}

	// 更新任务状态为完成
	models.UpdateExportJobStatus(jobID, "completed", 100, zipPath, "")
}

// GenerateAndroidPreview 生成Android代码预览
func GenerateAndroidPreview(project *models.FigmaProject, config AndroidExportConfig) (string, error) {
	// 获取用户信息
	var user models.User
	result := models.DB.First(&user, project.UserID)
	if result.Error != nil {
		return "", fmt.Errorf("获取用户信息失败: %v", result.Error)
	}

	// 获取优化后的节点数据
	rootNode, nodeImages, err := getOptimizedNodesForAndroid(project, user.FigmaToken, user.ID)
	if err != nil {
		return "", fmt.Errorf("获取节点数据失败: %v", err)
	}

	// 生成Android代码
	generator := &AndroidCodeGenerator{
		project:    project,
		rootNode:   rootNode,
		nodeImages: nodeImages,
		classNames: make(map[string]string),
		config:     config,
	}

	// 生成主Activity预览
	preview := generator.generateMainActivity()
	return preview, nil
}

// GenerateAndroidFullPreview 生成完整的Android代码预览（所有文件）
func GenerateAndroidFullPreview(project *models.FigmaProject, config AndroidExportConfig) (map[string]string, error) {
	// 获取用户信息
	var user models.User
	result := models.DB.First(&user, project.UserID)
	if result.Error != nil {
		return nil, fmt.Errorf("获取用户信息失败: %v", result.Error)
	}

	// 获取优化后的节点数据
	rootNode, nodeImages, err := getOptimizedNodesForAndroid(project, user.FigmaToken, user.ID)
	if err != nil {
		return nil, fmt.Errorf("获取节点数据失败: %v", err)
	}

	// 生成Android代码
	generator := &AndroidCodeGenerator{
		project:    project,
		rootNode:   rootNode,
		nodeImages: nodeImages,
		classNames: make(map[string]string),
		config:     config,
	}

	// 生成所有Android文件
	androidFiles, err := generator.GenerateAndroidCode()
	if err != nil {
		return nil, fmt.Errorf("生成Android代码失败: %v", err)
	}

	return androidFiles, nil
}

// GenerateAndroidCode 生成Android代码
func (g *AndroidCodeGenerator) GenerateAndroidCode() (map[string]string, error) {
	androidFiles := make(map[string]string)

	// 生成主Activity
	mainActivity := g.generateMainActivity()
	androidFiles["MainActivity.kt"] = mainActivity

	// 生成布局文件
	layoutXML := g.generateLayoutXML()
	androidFiles["activity_main.xml"] = layoutXML

	// 生成视图组件
	components := g.extractComponents(g.rootNode)
	for _, component := range components {
		componentCode := g.generateComponentCode(component)
		fileName := fmt.Sprintf("%sView.kt", component.Name)
		androidFiles[fileName] = componentCode
	}

	// 根据配置生成可选文件
	for _, option := range g.config.Options {
		switch option {
		case "generateExtensions":
			extensionCode := g.generateExtensions()
			androidFiles["ViewExtensions.kt"] = extensionCode
		case "generateResourceManager":
			resourceCode := g.generateResourceManager()
			androidFiles["ResourceManager.kt"] = resourceCode
		case "generateStyles":
			stylesCode := g.generateStyles()
			androidFiles["styles.xml"] = stylesCode
		case "generateColors":
			colorsCode := g.generateColors()
			androidFiles["colors.xml"] = colorsCode
		case "generateDimens":
			dimensCode := g.generateDimens()
			androidFiles["dimens.xml"] = dimensCode
		}
	}

	return androidFiles, nil
}

// generateMainActivity 生成主Activity
func (g *AndroidCodeGenerator) generateMainActivity() string {
	className := sanitizeClassName(g.project.Name) + "Activity"

	code := fmt.Sprintf(`package com.example.%s

import android.os.Bundle
import androidx.appcompat.app.AppCompatActivity
import androidx.constraintlayout.widget.ConstraintLayout
import android.widget.*
import androidx.core.content.ContextCompat

class %s : AppCompatActivity() {
    
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)
        
        setupViews()
    }
    
    private fun setupViews() {
        // 初始化视图组件
%s
    }
%s
}`,
		strings.ToLower(sanitizeClassName(g.project.Name)),
		className,
		g.generateViewInitialization(),
		g.generateViewMethods())

	return code
}

// generateLayoutXML 生成布局XML文件
func (g *AndroidCodeGenerator) generateLayoutXML() string {
	xml := `<?xml version="1.0" encoding="utf-8"?>
<androidx.constraintlayout.widget.ConstraintLayout 
    xmlns:android="http://schemas.android.com/apk/res/android"
    xmlns:app="http://schemas.android.com/apk/res-auto"
    xmlns:tools="http://schemas.android.com/tools"
    android:layout_width="match_parent"
    android:layout_height="match_parent"
    tools:context=".MainActivity">

`

	// 生成子视图
	xml += g.generateChildViews(g.rootNode, 0)

	xml += `
</androidx.constraintlayout.widget.ConstraintLayout>`

	return xml
}

// generateChildViews 生成子视图XML
func (g *AndroidCodeGenerator) generateChildViews(node gin.H, depth int) string {
	var xml strings.Builder
	indent := strings.Repeat("    ", depth+1)

	if children, ok := node["children"].([]interface{}); ok {
		for _, child := range children {
			if childNode, ok := child.(gin.H); ok {
				component := g.convertNodeToAndroidComponent(childNode)
				if component != nil {
					xml.WriteString(g.generateComponentXML(component, indent))
				}
			}
		}
	}

	return xml.String()
}

// convertNodeToAndroidComponent 将Figma节点转换为Android组件
func (g *AndroidCodeGenerator) convertNodeToAndroidComponent(node gin.H) *AndroidUIComponent {
	nodeType, _ := node["type"].(string)
	nodeName, _ := node["name"].(string)

	component := &AndroidUIComponent{
		Name:       sanitizeClassName(nodeName),
		Properties: make(map[string]interface{}),
	}

	// 提取坐标和尺寸
	if absoluteBoundingBox, ok := node["absoluteBoundingBox"].(gin.H); ok {
		component.Frame = AndroidFrame{
			X:      getFloat64(absoluteBoundingBox, "x"),
			Y:      getFloat64(absoluteBoundingBox, "y"),
			Width:  getFloat64(absoluteBoundingBox, "width"),
			Height: getFloat64(absoluteBoundingBox, "height"),
		}
	}

	switch nodeType {
	case "TEXT":
		component.Type = "TextView"
		g.extractTextProperties(node, component)
	case "RECTANGLE", "ELLIPSE":
		if g.hasImageFill(node) {
			component.Type = "ImageView"
			g.extractImageProperties(node, component)
		} else {
			component.Type = "View"
			g.extractViewProperties(node, component)
		}
	case "FRAME", "GROUP":
		component.Type = "LinearLayout"
		g.extractContainerProperties(node, component)
	default:
		component.Type = "View"
		g.extractViewProperties(node, component)
	}

	// 处理子节点
	if children, ok := node["children"].([]interface{}); ok {
		for _, child := range children {
			if childNode, ok := child.(gin.H); ok {
				childComponent := g.convertNodeToAndroidComponent(childNode)
				if childComponent != nil {
					component.Children = append(component.Children, *childComponent)
				}
			}
		}
	}

	return component
}

// generateComponentXML 生成组件XML
func (g *AndroidCodeGenerator) generateComponentXML(component *AndroidUIComponent, indent string) string {
	var xml strings.Builder

	xml.WriteString(fmt.Sprintf("%s<%s\n", indent, component.Type))
	xml.WriteString(fmt.Sprintf("%s    android:id=\"@+id/%s\"\n", indent, strings.ToLower(component.Name)))
	xml.WriteString(fmt.Sprintf("%s    android:layout_width=\"%.0fdp\"\n", indent, component.Frame.Width))
	xml.WriteString(fmt.Sprintf("%s    android:layout_height=\"%.0fdp\"\n", indent, component.Frame.Height))

	// 添加约束
	xml.WriteString(fmt.Sprintf("%s    app:layout_constraintStart_toStartOf=\"parent\"\n", indent))
	xml.WriteString(fmt.Sprintf("%s    app:layout_constraintTop_toTopOf=\"parent\"\n", indent))
	xml.WriteString(fmt.Sprintf("%s    app:layout_constraintLeft_toLeftOf=\"parent\"\n", indent))
	xml.WriteString(fmt.Sprintf("%s    app:layout_constraintTop_toTopOf=\"parent\"\n", indent))

	// 添加边距
	if component.Frame.X > 0 {
		xml.WriteString(fmt.Sprintf("%s    android:layout_marginStart=\"%.0fdp\"\n", indent, component.Frame.X))
	}
	if component.Frame.Y > 0 {
		xml.WriteString(fmt.Sprintf("%s    android:layout_marginTop=\"%.0fdp\"\n", indent, component.Frame.Y))
	}

	// 添加特定属性
	for key, value := range component.Properties {
		xml.WriteString(fmt.Sprintf("%s    %s=\"%v\"\n", indent, key, value))
	}

	if len(component.Children) > 0 {
		xml.WriteString(fmt.Sprintf("%s>\n", indent))

		// 生成子视图
		for _, child := range component.Children {
			xml.WriteString(g.generateComponentXML(&child, indent+"    "))
		}

		xml.WriteString(fmt.Sprintf("%s</%s>\n", indent, component.Type))
	} else {
		xml.WriteString(fmt.Sprintf("%s />\n", indent))
	}

	return xml.String()
}

// extractTextProperties 提取文本属性
func (g *AndroidCodeGenerator) extractTextProperties(node gin.H, component *AndroidUIComponent) {
	if style, ok := node["style"].(gin.H); ok {
		// 文本内容
		if characters, ok := node["characters"].(string); ok {
			component.Properties["android:text"] = characters
		}

		// 字体大小
		if fontSize, ok := style["fontSize"].(float64); ok {
			component.Properties["android:textSize"] = fmt.Sprintf("%.0fsp", fontSize)
		}

		// 文本颜色
		if fills, ok := style["fills"].([]interface{}); ok && len(fills) > 0 {
			if fill, ok := fills[0].(gin.H); ok {
				if color, ok := fill["color"].(gin.H); ok {
					colorHex := g.convertColor(color)
					component.Properties["android:textColor"] = colorHex
				}
			}
		}

		// 文本对齐
		if textAlignHorizontal, ok := style["textAlignHorizontal"].(string); ok {
			switch textAlignHorizontal {
			case "LEFT":
				component.Properties["android:gravity"] = "start"
			case "CENTER":
				component.Properties["android:gravity"] = "center"
			case "RIGHT":
				component.Properties["android:gravity"] = "end"
			}
		}
	}
}

// extractImageProperties 提取图片属性
func (g *AndroidCodeGenerator) extractImageProperties(node gin.H, component *AndroidUIComponent) {
	component.Properties["android:scaleType"] = "centerCrop"

	// 如果有对应的图片资源
	nodeID, _ := node["id"].(string)
	if imagePath, exists := g.nodeImages[nodeID]; exists {
		component.ImagePath = imagePath
		// 生成drawable资源引用
		imageName := strings.TrimSuffix(filepath.Base(imagePath), filepath.Ext(imagePath))
		component.Properties["android:src"] = fmt.Sprintf("@drawable/%s", imageName)
	}
}

// extractViewProperties 提取视图属性
func (g *AndroidCodeGenerator) extractViewProperties(node gin.H, component *AndroidUIComponent) {
	// 背景颜色
	if fills, ok := node["fills"].([]interface{}); ok && len(fills) > 0 {
		if fill, ok := fills[0].(gin.H); ok {
			if color, ok := fill["color"].(gin.H); ok {
				colorHex := g.convertColor(color)
				component.Properties["android:background"] = colorHex
			}
		}
	}

	// 圆角
	if cornerRadius, ok := node["cornerRadius"].(float64); ok && cornerRadius > 0 {
		// 需要创建drawable资源
		component.Properties["android:background"] = fmt.Sprintf("@drawable/bg_%s", strings.ToLower(component.Name))
	}
}

// extractContainerProperties 提取容器属性
func (g *AndroidCodeGenerator) extractContainerProperties(node gin.H, component *AndroidUIComponent) {
	component.Properties["android:orientation"] = "vertical"
	g.extractViewProperties(node, component)
}

// hasImageFill 检查是否有图片填充
func (g *AndroidCodeGenerator) hasImageFill(node gin.H) bool {
	if fills, ok := node["fills"].([]interface{}); ok {
		for _, fill := range fills {
			if fillMap, ok := fill.(gin.H); ok {
				if fillType, ok := fillMap["type"].(string); ok && fillType == "IMAGE" {
					return true
				}
			}
		}
	}
	return false
}

// convertColor 转换颜色
func (g *AndroidCodeGenerator) convertColor(color gin.H) string {
	r := int(getFloat64(color, "r") * 255)
	green := int(getFloat64(color, "g") * 255)
	b := int(getFloat64(color, "b") * 255)
	a := int(getFloat64(color, "a") * 255)

	if a < 255 {
		return fmt.Sprintf("#%02X%02X%02X%02X", a, r, green, b)
	}
	return fmt.Sprintf("#%02X%02X%02X", r, green, b)
}

// extractComponents 提取组件
func (g *AndroidCodeGenerator) extractComponents(node gin.H) []AndroidUIComponent {
	var components []AndroidUIComponent

	if children, ok := node["children"].([]interface{}); ok {
		for _, child := range children {
			if childNode, ok := child.(gin.H); ok {
				component := g.convertNodeToAndroidComponent(childNode)
				if component != nil {
					components = append(components, *component)
				}
				// 递归提取子组件
				subComponents := g.extractComponents(childNode)
				components = append(components, subComponents...)
			}
		}
	}

	return components
}

// generateComponentCode 生成组件代码
func (g *AndroidCodeGenerator) generateComponentCode(component AndroidUIComponent) string {
	className := component.Name + "View"

	code := fmt.Sprintf(`package com.example.%s.views

import android.content.Context
import android.util.AttributeSet
import android.widget.*
import androidx.constraintlayout.widget.ConstraintLayout

class %s @JvmOverloads constructor(
    context: Context,
    attrs: AttributeSet? = null,
    defStyleAttr: Int = 0
) : %s(context, attrs, defStyleAttr) {
    
    init {
        setupView()
    }
    
    private fun setupView() {
        // 设置视图属性
%s
    }
}`,
		strings.ToLower(sanitizeClassName(g.project.Name)),
		className,
		component.Type,
		g.generateViewSetup(component))

	return code
}

// generateViewSetup 生成视图设置代码
func (g *AndroidCodeGenerator) generateViewSetup(component AndroidUIComponent) string {
	var setup strings.Builder

	for key, value := range component.Properties {
		switch key {
		case "android:text":
			setup.WriteString(fmt.Sprintf("        text = \"%v\"\n", value))
		case "android:textSize":
			setup.WriteString(fmt.Sprintf("        textSize = %v\n", strings.TrimSuffix(value.(string), "sp")))
		case "android:textColor":
			setup.WriteString(fmt.Sprintf("        setTextColor(Color.parseColor(\"%v\"))\n", value))
		case "android:background":
			setup.WriteString(fmt.Sprintf("        setBackgroundColor(Color.parseColor(\"%v\"))\n", value))
		}
	}

	return setup.String()
}

// generateViewInitialization 生成视图初始化代码
func (g *AndroidCodeGenerator) generateViewInitialization() string {
	var init strings.Builder

	components := g.extractComponents(g.rootNode)
	for _, component := range components {
		viewId := strings.ToLower(component.Name)
		init.WriteString(fmt.Sprintf("        val %s = findViewById<%s>(R.id.%s)\n",
			viewId, component.Type, viewId))
	}

	return init.String()
}

// generateViewMethods 生成视图方法
func (g *AndroidCodeGenerator) generateViewMethods() string {
	return `
    // 添加自定义方法
    private fun setupClickListeners() {
        // 设置点击事件监听器
    }
    
    private fun updateUI() {
        // 更新UI状态
    }`
}

// generateExtensions 生成扩展函数
func (g *AndroidCodeGenerator) generateExtensions() string {
	return fmt.Sprintf(`package com.example.%s.extensions

import android.view.View
import android.widget.ImageView
import androidx.core.content.ContextCompat

// View扩展函数
fun View.show() {
    visibility = View.VISIBLE
}

fun View.hide() {
    visibility = View.GONE
}

fun View.invisible() {
    visibility = View.INVISIBLE
}

// ImageView扩展函数
fun ImageView.loadImage(resourceId: Int) {
    setImageResource(resourceId)
}

// 尺寸转换扩展
fun Int.dp(context: android.content.Context): Int {
    return (this * context.resources.displayMetrics.density).toInt()
}`, strings.ToLower(sanitizeClassName(g.project.Name)))
}

// generateResourceManager 生成资源管理器
func (g *AndroidCodeGenerator) generateResourceManager() string {
	return fmt.Sprintf(`package com.example.%s.utils

import android.content.Context
import androidx.core.content.ContextCompat

object ResourceManager {
    
    fun getColor(context: Context, colorRes: Int): Int {
        return ContextCompat.getColor(context, colorRes)
    }
    
    fun getDimension(context: Context, dimenRes: Int): Float {
        return context.resources.getDimension(dimenRes)
    }
    
    fun getString(context: Context, stringRes: Int): String {
        return context.getString(stringRes)
    }
    
    fun getDrawable(context: Context, drawableRes: Int) = 
        ContextCompat.getDrawable(context, drawableRes)
}`, strings.ToLower(sanitizeClassName(g.project.Name)))
}

// generateStyles 生成样式文件
func (g *AndroidCodeGenerator) generateStyles() string {
	return `<?xml version="1.0" encoding="utf-8"?>
<resources>
    
    <style name="AppTheme" parent="Theme.AppCompat.Light.DarkActionBar">
        <item name="colorPrimary">@color/colorPrimary</item>
        <item name="colorPrimaryDark">@color/colorPrimaryDark</item>
        <item name="colorAccent">@color/colorAccent</item>
    </style>
    
    <style name="TextStyle">
        <item name="android:textColor">@color/textColor</item>
        <item name="android:textSize">16sp</item>
    </style>
    
    <style name="ButtonStyle">
        <item name="android:background">@color/buttonBackground</item>
        <item name="android:textColor">@color/buttonTextColor</item>
        <item name="android:padding">12dp</item>
    </style>
    
</resources>`
}

// generateColors 生成颜色文件
func (g *AndroidCodeGenerator) generateColors() string {
	var colors strings.Builder
	colors.WriteString(`<?xml version="1.0" encoding="utf-8"?>
<resources>
    
    <color name="colorPrimary">#3F51B5</color>
    <color name="colorPrimaryDark">#303F9F</color>
    <color name="colorAccent">#FF4081</color>
    
    <color name="textColor">#333333</color>
    <color name="backgroundColor">#FFFFFF</color>
    <color name="buttonBackground">#2196F3</color>
    <color name="buttonTextColor">#FFFFFF</color>
`)

	// 从Figma节点中提取颜色
	g.extractColorsFromNode(g.rootNode, &colors)

	colors.WriteString(`
</resources>`)

	return colors.String()
}

// extractColorsFromNode 从节点中提取颜色
func (g *AndroidCodeGenerator) extractColorsFromNode(node gin.H, colors *strings.Builder) {
	// 提取填充颜色
	if fills, ok := node["fills"].([]interface{}); ok {
		for _, fill := range fills {
			if fillMap, ok := fill.(gin.H); ok {
				if color, ok := fillMap["color"].(gin.H); ok {
					colorHex := g.convertColor(color)
					nodeName, _ := node["name"].(string)
					colorName := fmt.Sprintf("color_%s", strings.ToLower(sanitizeClassName(nodeName)))
					colors.WriteString(fmt.Sprintf("    <color name=\"%s\">%s</color>\n", colorName, colorHex))
				}
			}
		}
	}

	// 递归处理子节点
	if children, ok := node["children"].([]interface{}); ok {
		for _, child := range children {
			if childNode, ok := child.(gin.H); ok {
				g.extractColorsFromNode(childNode, colors)
			}
		}
	}
}

// generateDimens 生成尺寸文件
func (g *AndroidCodeGenerator) generateDimens() string {
	return `<?xml version="1.0" encoding="utf-8"?>
<resources>
    
    <dimen name="text_size_small">12sp</dimen>
    <dimen name="text_size_medium">16sp</dimen>
    <dimen name="text_size_large">20sp</dimen>
    
    <dimen name="margin_small">8dp</dimen>
    <dimen name="margin_medium">16dp</dimen>
    <dimen name="margin_large">24dp</dimen>
    
    <dimen name="padding_small">8dp</dimen>
    <dimen name="padding_medium">16dp</dimen>
    <dimen name="padding_large">24dp</dimen>
    
</resources>`
}

// getOptimizedNodesForAndroid 获取优化后的节点数据（复用Swift的逻辑）
func getOptimizedNodesForAndroid(project *models.FigmaProject, figmaToken string, userID uint) (gin.H, map[string]string, error) {
	// 复用Swift的逻辑
	return getOptimizedNodesForSwift(project, figmaToken, userID)
}

// downloadImagesForAndroid 下载图片资源（复用Swift的逻辑）
func downloadImagesForAndroid(figmaToken string, project *models.FigmaProject, nodeImages map[string]string,
	tempDir string, jobID uint, format string, scale float64) error {
	// 复用Swift的逻辑
	return downloadImagesForSwift(figmaToken, project, nodeImages, tempDir, jobID, format, scale)
}

// saveAndroidFiles 保存Android文件
func saveAndroidFiles(tempDir string, androidFiles map[string]string, project *models.FigmaProject) error {
	// 创建Android项目结构
	srcDir := filepath.Join(tempDir, "app", "src", "main")
	javaDir := filepath.Join(srcDir, "java", "com", "example", strings.ToLower(sanitizeClassName(project.Name)))
	resDir := filepath.Join(srcDir, "res")
	layoutDir := filepath.Join(resDir, "layout")
	valuesDir := filepath.Join(resDir, "values")
	drawableDir := filepath.Join(resDir, "drawable")

	// 创建目录
	dirs := []string{javaDir, layoutDir, valuesDir, drawableDir,
		filepath.Join(javaDir, "views"), filepath.Join(javaDir, "utils"), filepath.Join(javaDir, "extensions")}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return err
		}
	}

	// 保存文件
	for filename, content := range androidFiles {
		var filePath string

		if strings.HasSuffix(filename, ".kt") {
			if strings.Contains(filename, "View.kt") {
				filePath = filepath.Join(javaDir, "views", filename)
			} else if strings.Contains(filename, "Extensions.kt") {
				filePath = filepath.Join(javaDir, "extensions", filename)
			} else if strings.Contains(filename, "Manager.kt") {
				filePath = filepath.Join(javaDir, "utils", filename)
			} else {
				filePath = filepath.Join(javaDir, filename)
			}
		} else if strings.HasSuffix(filename, ".xml") {
			if strings.HasPrefix(filename, "activity_") {
				filePath = filepath.Join(layoutDir, filename)
			} else {
				filePath = filepath.Join(valuesDir, filename)
			}
		} else {
			filePath = filepath.Join(tempDir, filename)
		}

		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return err
		}
	}

	return nil
}

// 辅助函数

// sanitizeClassName 清理类名，确保符合编程语言命名规范
func sanitizeClassName(name string) string {
	// 移除特殊字符，只保留字母、数字和下划线
	reg := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	cleaned := reg.ReplaceAllString(name, "")

	// 确保以字母开头
	if len(cleaned) > 0 && cleaned[0] >= '0' && cleaned[0] <= '9' {
		cleaned = "Class" + cleaned
	}

	// 如果为空，使用默认名称
	if cleaned == "" {
		cleaned = "DefaultClass"
	}

	// 首字母大写
	if len(cleaned) > 0 {
		cleaned = strings.ToUpper(cleaned[:1]) + cleaned[1:]
	}

	return cleaned
}

// getFloat64 从gin.H中安全获取float64值
func getFloat64(data gin.H, key string) float64 {
	if value, exists := data[key]; exists {
		switch v := value.(type) {
		case float64:
			return v
		case float32:
			return float64(v)
		case int:
			return float64(v)
		case int64:
			return float64(v)
		}
	}
	return 0.0
}
