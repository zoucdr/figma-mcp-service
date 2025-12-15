package services

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/figma-deliver/internal/models"
)

// CustomCodeExportConfig 自定义代码导出配置
type CustomCodeExportConfig struct {
	ImageFormat string  `json:"imageFormat"` // 图片格式
	ImageScale  float64 `json:"imageScale"`  // 图片缩放比例
}

// ImageRenderInfo 图片渲染信息
type ImageRenderInfo struct {
	NodeID    string // 节点ID
	ImageURL  string // 图片URL
	LocalPath string // 本地路径
}

// CustomCodeExportContext JS脚本执行上下文
type CustomCodeExportContext struct {
	NodeTree  interface{}            `json:"nodeTree"`  // 节点树数据
	ImageDict map[string]interface{} `json:"imageDict"` // 图片字典
	Config    CustomCodeExportConfig `json:"config"`    // 导出配置
	ProjectID uint                   `json:"projectId"` // 项目ID
	Project   *models.FigmaProject   `json:"project"`   // 项目信息
}

// GenerateCustomCodePreview 生成自定义代码预览
func GenerateCustomCodePreview(project *models.FigmaProject, config CustomCodeExportConfig) (map[string]string, error) {
	// 获取用户信息
	var user models.User
	result := models.DB.First(&user, project.UserID)
	if result.Error != nil {
		return nil, fmt.Errorf("获取用户信息失败: %v", result.Error)
	}

	// 检查用户是否配置了UI脚本
	if user.UICreater == "" {
		return nil, fmt.Errorf("用户未配置UI代码生成脚本，请在设置页面配置")
	}

	// 获取节点树和图片字典
	context, err := buildExportContext(project, &user, config)
	if err != nil {
		return nil, fmt.Errorf("构建导出上下文失败: %v", err)
	}

	// 初始化文件映射
	files := make(map[string]string)

	// 添加 context.json 文件，显示完整的上下文数据
	contextJSON, err := json.MarshalIndent(context, "", "  ")
	if err != nil {
		log.Printf("序列化context失败: %v", err)
	} else {
		files["context.json"] = string(contextJSON)
	}

	// 执行用户的JS脚本，调用预览函数
	userFiles, err := executeUserScript(user.UICreater, context, "generatePreview")
	if err != nil {
		return nil, fmt.Errorf("执行预览脚本失败: %v", err)
	}

	// 合并用户生成的文件和 context.json
	for filename, content := range userFiles {
		files[filename] = content
	}

	return files, nil
}

// ExportCustomCode 导出自定义代码
func ExportCustomCode(project *models.FigmaProject, config CustomCodeExportConfig) (map[string]string, error) {
	// 获取用户信息
	var user models.User
	result := models.DB.First(&user, project.UserID)
	if result.Error != nil {
		return nil, fmt.Errorf("获取用户信息失败: %v", result.Error)
	}

	// 检查用户是否配置了UI脚本
	if user.UICreater == "" {
		return nil, fmt.Errorf("用户未配置UI代码生成脚本，请在设置页面配置")
	}

	// 获取节点树和图片字典
	context, err := buildExportContext(project, &user, config)
	if err != nil {
		return nil, fmt.Errorf("构建导出上下文失败: %v", err)
	}

	// 初始化文件映射
	files := make(map[string]string)

	// 添加 context.json 文件，显示完整的上下文数据
	contextJSON, err := json.MarshalIndent(context, "", "  ")
	if err != nil {
		log.Printf("序列化context失败: %v", err)
	} else {
		files["context.json"] = string(contextJSON)
	}

	// 执行用户的JS脚本，调用导出函数
	userFiles, err := executeUserScript(user.UICreater, context, "generateExport")
	if err != nil {
		return nil, fmt.Errorf("执行导出脚本失败: %v", err)
	}

	// 合并用户生成的文件和 context.json
	for filename, content := range userFiles {
		files[filename] = content
	}

	return files, nil
}

// ProcessCustomCodeExportJob 处理自定义代码导出任务（参考图文导出实现）
func ProcessCustomCodeExportJob(jobID uint, config CustomCodeExportConfig) {
	// 获取任务信息
	job, err := models.GetExportJob(jobID)
	if err != nil {
		return
	}

	// 更新任务状态为处理中 - 10%
	models.UpdateExportJobStatus(jobID, "processing", 10, "", "")

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

	// 检查用户是否配置了UI脚本
	if user.UICreater == "" {
		models.UpdateExportJobStatus(jobID, "failed", 0, "", "用户未配置UI代码生成脚本")
		return
	}

	// 更新进度 - 20%
	models.UpdateExportJobStatus(jobID, "processing", 20, "", "正在生成代码...")

	// 执行代码生成
	codeFiles, err := ExportCustomCode(project, config)
	if err != nil {
		models.UpdateExportJobStatus(jobID, "failed", 0, "", "生成代码失败: "+err.Error())
		return
	}

	// 更新进度 - 40%
	models.UpdateExportJobStatus(jobID, "processing", 40, "", "正在下载图片资源...")

	// 获取节点数据（包含图片信息）
	_, nodeImages, err := getOptimizedNodesForCustomExport(project, user.FigmaToken, user.ID)
	if err != nil {
		models.UpdateExportJobStatus(jobID, "failed", 0, "", "获取节点数据失败: "+err.Error())
		return
	}

	// 下载所有图片资源
	imageFiles := make(map[string]string)
	if len(nodeImages) > 0 {
		totalImages := len(nodeImages)
		processedImages := 0

		// 创建临时目录用于下载图片
		tempImageDir := filepath.Join("temp", fmt.Sprintf("custom_export_images_%d_%d", project.ID, time.Now().Unix()))
		err = os.MkdirAll(tempImageDir, os.ModePerm)
		if err != nil {
			models.UpdateExportJobStatus(jobID, "failed", 0, "", "创建临时目录失败: "+err.Error())
			return
		}
		defer os.RemoveAll(tempImageDir)

		for nodeID := range nodeImages {
			// 下载图片
			imagePath, err := DownloadFigmaImageForExport(
				user.FigmaToken,
				project.FileKey,
				nodeID,
				config.ImageFormat,
				config.ImageScale,
				false,      // ignoreTexts
				[]string{}, // ignoreNodes
				user.ID,
			)

			if err != nil {
				log.Printf("下载图片失败 nodeID=%s: %v", nodeID, err)
				continue
			}

			// 读取图片文件内容
			imageData, err := os.ReadFile(imagePath)
			if err != nil {
				log.Printf("读取图片文件失败 nodeID=%s: %v", nodeID, err)
				continue
			}

			// 确定图片文件名和扩展名
			ext := config.ImageFormat
			if ext == "jpg" {
				ext = "jpeg"
			}
			imageFileName := fmt.Sprintf("images/%s.%s", sanitizeNodeID(nodeID), ext)

			// 将图片内容存储（用于ZIP）
			imageFiles[imageFileName] = string(imageData)

			processedImages++
			progress := 40 + int(float64(processedImages)/float64(totalImages)*40)
			models.UpdateExportJobStatus(jobID, "processing", progress, "", fmt.Sprintf("正在下载图片 %d/%d", processedImages, totalImages))
		}
	}

	// 更新进度 - 80%
	models.UpdateExportJobStatus(jobID, "processing", 80, "", "正在创建压缩包...")

	// 合并代码文件和图片文件
	allFiles := make(map[string]string)
	for filename, content := range codeFiles {
		allFiles[filename] = content
	}
	for filename, content := range imageFiles {
		allFiles[filename] = content
	}

	// 创建ZIP压缩包
	zipPath, err := CreateExportArchive(allFiles, fmt.Sprintf("custom_code_%d", project.ID))
	if err != nil {
		models.UpdateExportJobStatus(jobID, "failed", 0, "", "创建压缩包失败: "+err.Error())
		return
	}

	// 更新任务状态为完成 - 100%
	models.UpdateExportJobStatus(jobID, "completed", 100, zipPath, "")
}

// sanitizeNodeID 清理节点ID中的特殊字符，使其适合作为文件名
func sanitizeNodeID(nodeID string) string {
	return strings.NewReplacer(
		":", "_",
		";", "_",
		"/", "_",
		"\\", "_",
		"?", "_",
		"*", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	).Replace(nodeID)
}

// buildExportContext 构建导出上下文
func buildExportContext(project *models.FigmaProject, user *models.User, config CustomCodeExportConfig) (*CustomCodeExportContext, error) {
	// 获取优化后的节点数据
	rootNode, nodeImages, err := getOptimizedNodesForCustomExport(project, user.FigmaToken, user.ID)
	if err != nil {
		return nil, fmt.Errorf("获取节点数据失败: %v", err)
	}

	// 构建图片字典（只返回文件名）
	imageDict := buildImageDict(nodeImages, config.ImageFormat)

	context := &CustomCodeExportContext{
		NodeTree:  rootNode,
		ImageDict: imageDict,
		Config:    config,
		ProjectID: project.ID,
		Project:   project,
	}

	return context, nil
}

// getOptimizedNodesForCustomExport 获取优化后的节点数据（用于自定义导出）
func getOptimizedNodesForCustomExport(project *models.FigmaProject, figmaToken string, userID uint) (interface{}, map[string]*ImageRenderInfo, error) {
	// 直接从数据库获取所有节点（不触发节点更新，避免覆盖用户的修改信息）
	var nodes []models.FigmaNode
	result := models.DB.Where("project_id = ?", project.ID).Find(&nodes)
	if result.Error != nil {
		return nil, nil, fmt.Errorf("从数据库获取项目节点失败: %v", result.Error)
	}

	// 从文件缓存获取完整的节点树数据
	figmaNodes, err := GetFigmaNodesWithUser(figmaToken, project.FileKey, project.RootNodeID, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("获取Figma节点树失败: %v", err)
	}

	// 构建节点树
	rootNode := buildNodeTree(figmaNodes, project.RootNodeID)

	// 解析所有节点的修改信息
	nodeSettings := make(map[string]map[string]interface{})
	for _, node := range nodes {
		var settings map[string]interface{}
		if err := json.Unmarshal([]byte(node.Modifys), &settings); err == nil {
			nodeSettings[node.NodeID] = settings
		}
	}

	// 获取节点对应的图片渲染信息（仅包含res_mode为图片类型的节点）
	nodeImages := make(map[string]*ImageRenderInfo)
	for _, node := range nodes {
		// 检查该节点是否有图片相关的res_mode
		shouldIncludeImage := false
		if modifys, exists := nodeSettings[node.NodeID]; exists {
			if resMode, ok := modifys["res_mode"].(string); ok {
				// 只有res_mode为sprite、slice、texture时才需要图片
				if resMode == "sprite" || resMode == "slice" || resMode == "texture" {
					shouldIncludeImage = true
				}
			}
		}

		if !shouldIncludeImage {
			continue
		}

		// 查询该节点的图片信息
		var nodeImage models.FigmaNodeImage
		result := models.DB.Where("node_id = ? AND file_key = ?", node.NodeID, project.FileKey).First(&nodeImage)
		if result.Error == nil && (nodeImage.FigmaCDNURL != "" || nodeImage.OBSKey != "") {
			// 构建图片URL
			imageURL := nodeImage.FigmaCDNURL
			if imageURL == "" && nodeImage.OBSKey != "" {
				// 如果没有FigmaCDNURL，使用OBSKey生成URL（需要实现GetOBSURL函数，这里先使用OBSKey）
				imageURL = nodeImage.OBSKey
			}

			nodeImages[node.NodeID] = &ImageRenderInfo{
				NodeID:    node.NodeID,
				ImageURL:  imageURL,
				LocalPath: nodeImage.OBSKey, // 使用OBSKey作为本地路径标识
			}
		}
	}

	return rootNode, nodeImages, nil
}

// buildNodeTree 构建节点树（从Figma节点扁平列表构建树形结构）
func buildNodeTree(nodes []map[string]interface{}, rootNodeID string) interface{} {
	// 创建节点映射
	nodeMap := make(map[string]map[string]interface{})
	for _, node := range nodes {
		if id, ok := node["id"].(string); ok {
			// 复制节点数据
			nodeCopy := make(map[string]interface{})
			for k, v := range node {
				nodeCopy[k] = v
			}
			// 初始化children字段
			if _, hasChildren := nodeCopy["children"]; !hasChildren {
				nodeCopy["children"] = []interface{}{}
			}
			nodeMap[id] = nodeCopy
		}
	}

	// 构建父子关系
	for _, node := range nodes {
		id := node["id"].(string)
		parentID, hasParent := node["parent_id"]

		if hasParent && parentID != nil {
			parentIDStr, ok := parentID.(string)
			if ok && parentIDStr != "" && parentIDStr != rootNodeID {
				if parent, exists := nodeMap[parentIDStr]; exists {
					children := parent["children"].([]interface{})
					children = append(children, nodeMap[id])
					parent["children"] = children
				}
			}
		}
	}

	// 找到根节点
	if root, exists := nodeMap[rootNodeID]; exists {
		return root
	}

	// 如果找不到指定的根节点，返回第一个没有父节点的节点
	for _, node := range nodes {
		parentID, hasParent := node["parent_id"]
		if !hasParent || parentID == nil || parentID == "" || parentID == rootNodeID {
			if id, ok := node["id"].(string); ok {
				if rootNode, exists := nodeMap[id]; exists {
					return rootNode
				}
			}
		}
	}

	return nil
}

// buildImageDict 构建图片字典（只返回文件名，用于导出）
func buildImageDict(nodeImages map[string]*ImageRenderInfo, imageFormat string) map[string]interface{} {
	imageDict := make(map[string]interface{})

	// 确定文件扩展名
	ext := imageFormat
	if ext == "jpg" {
		ext = "jpeg"
	}

	for nodeID := range nodeImages {
		// 生成图片文件名
		imageFileName := fmt.Sprintf("images/%s.%s", sanitizeNodeID(nodeID), ext)
		imageDict[nodeID] = imageFileName
	}
	return imageDict
}

// executeUserScript 执行用户的JS脚本
func executeUserScript(script string, context *CustomCodeExportContext, functionName string) (map[string]string, error) {
	// 创建JS虚拟机
	vm := goja.New()

	// 注入上下文数据
	contextJSON, err := json.Marshal(context)
	if err != nil {
		return nil, fmt.Errorf("序列化上下文失败: %v", err)
	}

	var contextObj interface{}
	if err := json.Unmarshal(contextJSON, &contextObj); err != nil {
		return nil, fmt.Errorf("反序列化上下文失败: %v", err)
	}

	vm.Set("context", contextObj)

	// 添加console.log支持
	console := vm.NewObject()
	console.Set("log", func(call goja.FunctionCall) goja.Value {
		args := make([]interface{}, len(call.Arguments))
		for i, arg := range call.Arguments {
			args[i] = arg.Export()
		}
		// 使用log.Println输出每个参数
		for _, arg := range args {
			log.Println("[用户脚本]", arg)
		}
		return goja.Undefined()
	})
	vm.Set("console", console)

	// 添加JSON支持
	jsonObj := vm.NewObject()
	jsonObj.Set("stringify", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return vm.ToValue("")
		}
		data, err := json.Marshal(call.Arguments[0].Export())
		if err != nil {
			return vm.ToValue("")
		}
		return vm.ToValue(string(data))
	})
	jsonObj.Set("parse", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return goja.Undefined()
		}
		str := call.Arguments[0].String()
		var result interface{}
		if err := json.Unmarshal([]byte(str), &result); err != nil {
			return goja.Undefined()
		}
		return vm.ToValue(result)
	})
	vm.Set("JSON", jsonObj)

	// 执行用户脚本
	_, err = vm.RunString(script)
	if err != nil {
		return nil, fmt.Errorf("执行脚本失败: %v", err)
	}

	// 调用指定的函数
	var function goja.Callable
	functionValue := vm.Get(functionName)
	if functionValue == nil {
		return nil, fmt.Errorf("脚本中未定义函数: %s", functionName)
	}

	var ok bool
	function, ok = goja.AssertFunction(functionValue)
	if !ok {
		return nil, fmt.Errorf("%s 不是一个函数", functionName)
	}

	// 调用函数
	result, err := function(goja.Undefined(), vm.ToValue(contextObj))
	if err != nil {
		return nil, fmt.Errorf("调用函数 %s 失败: %v", functionName, err)
	}

	// 解析返回结果
	resultExport := result.Export()
	if resultExport == nil {
		return nil, fmt.Errorf("函数 %s 返回了空值", functionName)
	}

	// 转换返回结果为map[string]string
	files := make(map[string]string)

	// 先转换为JSON再解析，确保类型正确
	resultJSON, err := json.Marshal(resultExport)
	if err != nil {
		return nil, fmt.Errorf("序列化函数返回值失败: %v", err)
	}

	if err := json.Unmarshal(resultJSON, &files); err != nil {
		return nil, fmt.Errorf("解析函数返回值失败: %v，期望返回格式为 { \"文件名\": \"文件内容\" }", err)
	}

	return files, nil
}
