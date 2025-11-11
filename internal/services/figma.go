package services

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/figma-bridge/internal/models"
)

// FigmaService Figma API服务
type FigmaService struct {
	Token string
}

// formatScaleForFilename 格式化缩放等级用于文件名
// 如果是整数（如1.0, 2.0），去掉".0"，使用整数部分（如"1", "2"）
// 如果是非整数（如0.5, 1.5），保持小数点格式（如"0.5", "1.5"）
func formatScaleForFilename(scale float64) string {
	// 如果是整数（如1.0, 2.0），去掉".0"，使用整数部分
	if scale == float64(int64(scale)) {
		return fmt.Sprintf("%.0f", scale)
	}
	// 如果是非整数，保持小数点格式
	return fmt.Sprintf("%.1f", scale)
}

// ParseFigmaLink 解析Figma链接
func ParseFigmaLink(link string) (fileKey, nodeId string, err error) {
	// 检查链接是否为空
	if link == "" {
		return "", "", errors.New("链接不能为空")
	}

	// 检查是否是Figma链接
	if !strings.Contains(link, "figma.com") {
		return "", "", errors.New("不是有效的Figma链接")
	}

	// 提取文件ID
	fileKeyRegex := regexp.MustCompile(`figma\.com\/(?:file|design)\/([^\/\?]+)`)
	fileKeyMatches := fileKeyRegex.FindStringSubmatch(link)
	if len(fileKeyMatches) < 2 {
		return "", "", errors.New("无法提取Figma文件ID")
	}
	fileKey = fileKeyMatches[1]

	// 提取节点ID
	nodeIdRegex := regexp.MustCompile(`node-id=([^&]+)`)
	nodeIdMatches := nodeIdRegex.FindStringSubmatch(link)
	if len(nodeIdMatches) >= 2 {
		// 替换URL编码的字符
		nodeId = strings.ReplaceAll(nodeIdMatches[1], "%3A", ":")
	}

	return fileKey, nodeId, nil
}

// GetFigmaProjectNodes 获取Figma项目节点
func GetFigmaProjectNodes(userID, projectID uint, fileKey, nodeID string) ([]models.FigmaNode, error) {
	// 获取用户信息
	var user models.User
	result := models.DB.First(&user, userID)
	if result.Error != nil {
		return nil, result.Error
	}

	// 如果没有Figma Token，返回错误
	if user.FigmaToken == "" {
		return nil, errors.New("用户未设置Figma Token")
	}

	// 获取节点信息
	nodes, err := GetFigmaNodes(user.FigmaToken, fileKey, nodeID)
	if err != nil {
		return nil, err
	}

	// 保存节点信息
	var savedNodes []models.FigmaNode
	for _, node := range nodes {
		figmaNode, err := CreateFigmaNode(projectID, node["id"].(string), node["name"].(string), node["type"].(string), node["parent_id"])
		if err != nil {
			continue
		}
		savedNodes = append(savedNodes, *figmaNode)
	}

	return savedNodes, nil
}

// GetFigmaNodes 获取Figma节点，支持缓存
func GetFigmaNodes(token, fileKey, nodeID string) ([]map[string]interface{}, error) {
	// 检查是否存在缓存
	cachedNodes, err := loadCachedNodes(fileKey, nodeID)
	if err == nil && cachedNodes != nil {
		fmt.Printf("使用缓存的节点树数据: fileKey=%s, nodeID=%s\n", fileKey, nodeID)
		return cachedNodes, nil
	}

	// 构建API URL
	url := fmt.Sprintf("https://api.figma.com/v1/files/%s", fileKey)
	if nodeID != "" {
		url = fmt.Sprintf("%s/nodes?ids=%s", url, nodeID)
	}

	// 创建请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// 设置请求头
	req.Header.Set("X-Figma-Token", token)

	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API请求失败: %s", resp.Status)
	}

	// 读取响应体
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 解析响应
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	// 处理节点数据
	nodes := parseFigmaNodes(result, nodeID)

	// 缓存节点数据
	if err := cacheNodes(fileKey, nodeID, respBody); err != nil {
		fmt.Printf("缓存节点树数据失败: %v\n", err)
	}

	return nodes, nil
}

// loadCachedNodes 加载缓存的节点树数据
func loadCachedNodes(fileKey, nodeID string) ([]map[string]interface{}, error) {
	// 创建安全的文件名
	safeNodeID := strings.NewReplacer(":", "_", ";", "_", "/", "_", "\\", "_", "?", "_", "*", "_", "\"", "_", "<", "_", ">", "_", "|", "_").Replace(nodeID)
	cacheDir := filepath.Join("temp", fileKey, "documents")
	cacheFile := filepath.Join(cacheDir, safeNodeID+".json")

	// 检查文件是否存在
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("缓存文件不存在: %s", cacheFile)
	}

	// 读取缓存文件
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil, err
	}

	// 解析JSON数据
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	// 处理节点数据
	return parseFigmaNodes(result, nodeID), nil
}

// cacheNodes 缓存节点树数据
func cacheNodes(fileKey, nodeID string, data []byte) error {
	// 创建安全的文件名
	safeNodeID := strings.NewReplacer(":", "_", ";", "_", "/", "_", "\\", "_", "?", "_", "*", "_", "\"", "_", "<", "_", ">", "_", "|", "_").Replace(nodeID)
	cacheDir := filepath.Join("temp", fileKey, "documents")

	// 确保目录存在
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}

	// 写入缓存文件
	cacheFile := filepath.Join(cacheDir, safeNodeID+".json")
	return os.WriteFile(cacheFile, data, 0644)
}

// parseFigmaNodes 解析Figma节点数据
func parseFigmaNodes(data map[string]interface{}, _ /*rootNodeID*/ string) []map[string]interface{} {
	var nodes []map[string]interface{}

	// 如果是节点查询
	if document, ok := data["document"].(map[string]interface{}); ok {
		// 处理根节点 - 直接使用完整节点数据
		documentCopy := make(map[string]interface{})
		for k, v := range document {
			documentCopy[k] = v
		}
		documentCopy["id"] = "0:0" // 设置根节点ID
		documentCopy["parent_id"] = nil
		nodes = append(nodes, documentCopy)

		// 处理子节点
		if children, ok := document["children"].([]interface{}); ok {
			for _, child := range children {
				childNodes := parseNodeRecursive(child.(map[string]interface{}), "0:0")
				nodes = append(nodes, childNodes...)
			}
		}
	} else if nodeMap, ok := data["nodes"].(map[string]interface{}); ok {
		// 处理指定节点查询
		for nodeID, nodeData := range nodeMap {
			// 检查 nodeData 是否为 nil
			if nodeData == nil {
				fmt.Printf("跳过节点 %s，因为 nodeData 为 nil\n", nodeID)
				continue
			}

			// 检查 nodeData 是否为 map[string]interface{} 类型
			nodeDataMap, ok := nodeData.(map[string]interface{})
			if !ok {
				fmt.Printf("跳过节点 %s，因为 nodeData 不是 map 类型: %T\n", nodeID, nodeData)
				continue
			}

			// 检查是否有 document 字段
			documentInterface, hasDocument := nodeDataMap["document"]
			if !hasDocument || documentInterface == nil {
				fmt.Printf("跳过节点 %s，因为没有 document 字段或 document 为 nil\n", nodeID)
				continue
			}

			// 检查 document 是否为 map[string]interface{} 类型
			if node, ok := documentInterface.(map[string]interface{}); ok {
				// 添加当前节点 - 直接使用完整节点数据
				nodeCopy := make(map[string]interface{})
				for k, v := range node {
					nodeCopy[k] = v
				}
				nodeCopy["id"] = nodeID
				nodeCopy["parent_id"] = nil // 根节点没有父节点
				nodes = append(nodes, nodeCopy)

				// 处理子节点
				if children, ok := node["children"].([]interface{}); ok {
					for _, child := range children {
						childNodes := parseNodeRecursive(child.(map[string]interface{}), nodeID)
						nodes = append(nodes, childNodes...)
					}
				}
			}
		}
	}

	return nodes
}

// parseNodeRecursive 递归解析节点
func parseNodeRecursive(node map[string]interface{}, parentID string) []map[string]interface{} {
	var nodes []map[string]interface{}

	// 添加当前节点 - 直接使用完整节点数据
	nodeCopy := make(map[string]interface{})
	for k, v := range node {
		nodeCopy[k] = v
	}
	nodeCopy["parent_id"] = parentID
	nodes = append(nodes, nodeCopy)

	// 处理子节点
	if children, ok := node["children"].([]interface{}); ok {
		for _, child := range children {
			childNodes := parseNodeRecursive(child.(map[string]interface{}), node["id"].(string))
			nodes = append(nodes, childNodes...)
		}
	}

	return nodes
}

// CreateFigmaNode 创建Figma节点
func CreateFigmaNode(projectID uint, nodeID, name, nodeType string, parentID interface{}) (*models.FigmaNode, error) {
	// 创建节点记录，将节点信息存储在modifys字段中
	nodeInfo := map[string]interface{}{
		"name":      name,
		"type":      nodeType,
		"parent_id": parentID,
	}

	// 转换为JSON字符串
	modifysJSON, err := json.Marshal(nodeInfo)
	if err != nil {
		return nil, err
	}

	figmaNode := &models.FigmaNode{
		ProjectID: projectID,
		NodeID:    nodeID,
		Modifys:   string(modifysJSON),
	}

	err = models.SaveNode(figmaNode)
	if err != nil {
		return nil, err
	}

	return figmaNode, nil
}

// GetOrCreateFigmaProject 获取或创建Figma项目
func GetOrCreateFigmaProject(userID uint, fileKey, rootNodeID, name, figmaURL string) (*models.FigmaProject, error) {
	// 确保用户存在
	_, err := models.EnsureUserExists(userID)
	if err != nil {
		return nil, fmt.Errorf("确保用户存在失败: %w", err)
	}

	// 如果提供了rootNodeID，先尝试通过fileKey和rootNodeID精确查找
	if rootNodeID != "" {
		project, err := models.GetProjectByUserFileKeyAndRootNodeID(userID, fileKey, rootNodeID)
		if err == nil {
			// 找到了精确匹配的项目，更新URL（如果提供了新的URL）
			if figmaURL != "" && project.FigmaURL != figmaURL {
				project.FigmaURL = figmaURL
				models.DB.Save(project)
			}
			return project, nil
		}
	}

	// 如果没有提供rootNodeID或者找不到精确匹配，尝试只通过fileKey查找
	if rootNodeID == "" {
		project, err := models.GetProjectByUserAndFileKey(userID, fileKey)
		if err == nil {
			// 找到了项目，但检查是否需要更新rootNodeID和URL
			needSave := false
			if project.RootNodeID == "" && rootNodeID != "" {
				project.RootNodeID = rootNodeID
				needSave = true
			}
			if figmaURL != "" && project.FigmaURL != figmaURL {
				project.FigmaURL = figmaURL
				needSave = true
			}
			if needSave {
				models.DB.Save(project)
			}
			return project, nil
		}
	}

	// 创建新项目
	project, err := models.CreateOrUpdateProject(userID, fileKey, rootNodeID, name, figmaURL)
	if err != nil {
		return nil, err
	}

	return project, nil
}

// DownloadFigmaImage 下载Figma图片
func DownloadFigmaImage(token, fileKey, nodeID string) (string, error) {
	// 使用默认参数，默认使用缓存
	return DownloadPreviewFigmaImageWithOptions(token, fileKey, nodeID, "png", 1.0, true)
}

// DownloadPreviewFigmaImageWithOptions 下载Figma图片（带选项）
func DownloadPreviewFigmaImageWithOptions(token, fileKey, nodeID, imageFormat string, imageScale float64, useCache bool) (string, error) {
	// 打印调试信息
	fmt.Printf("开始下载Figma图片: fileKey=%s, nodeID=%s, format=%s, scale=%.1f, useCache=%v\n",
		fileKey, nodeID, imageFormat, imageScale, useCache)

	// 首先检查是否已经有该节点的图片文件
	// 创建临时文件夹
	tempDir := filepath.Join("temp", fileKey, "previews")
	fmt.Printf("缓存目录路径: %s\n", tempDir)
	err := os.MkdirAll(tempDir, os.ModePerm)
	if err != nil {
		fmt.Printf("创建临时文件夹失败: %v\n", err)
		return "", fmt.Errorf("创建临时文件夹失败: %v", err)
	}

	// 生成文件名 - 替换特殊字符为下划线
	safeNodeID := strings.NewReplacer(":", "_", ";", "_", "/", "_", "\\", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_").Replace(nodeID)
	fmt.Printf("原始节点ID: %s, 安全节点ID: %s\n", nodeID, safeNodeID)

	// 如果使用缓存，检查是否已经有该节点的图片文件
	if useCache {
		existingFile := findLatestNodeImage(tempDir, safeNodeID, imageScale)
		if existingFile != "" {
			// 不再检查过期时间，直接使用缓存的图片文件
			fmt.Printf("找到节点 %s 的缓存图片文件，直接使用: %s\n", nodeID, existingFile)
			return existingFile, nil
		}
	} else {
		fmt.Printf("跳过缓存检查，强制重新下载\n")
	}

	// 第一步：通过Figma API获取图片URL
	// 确保节点ID使用冒号格式（Figma API标准格式）
	figmaNodeID := strings.ReplaceAll(nodeID, "-", ":")

	// 构建API URL
	apiUrl := fmt.Sprintf("https://api.figma.com/v1/images/%s?ids=%s&format=%s&scale=%.1f&contentOnly=true",
		fileKey, figmaNodeID, imageFormat, imageScale)
	fmt.Printf("Figma API URL: %s\n", apiUrl)

	// 创建请求
	req, err := http.NewRequest("GET", apiUrl, nil)
	if err != nil {
		fmt.Printf("创建请求失败: %v\n", err)
		return "", fmt.Errorf("创建API请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("X-Figma-Token", token)
	fmt.Printf("请求头已设置: X-Figma-Token=%s...\n", token[:5]+"***")

	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	fmt.Printf("发送请求获取图片URL...\n")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("请求失败: %v\n", err)
		return "", fmt.Errorf("发送API请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	fmt.Printf("响应状态码: %d\n", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("API请求失败: %s", resp.Status)
		fmt.Println(errMsg)
		return "", fmt.Errorf(errMsg)
	}

	// 解析响应
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("读取响应内容失败: %v\n", err)
		return "", fmt.Errorf("读取API响应失败: %v", err)
	}

	fmt.Printf("响应数据: %s\n", string(responseBody))

	var result map[string]interface{}
	if err := json.Unmarshal(responseBody, &result); err != nil {
		fmt.Printf("解析响应JSON失败: %v\n", err)
		return "", fmt.Errorf("解析API响应JSON失败: %v", err)
	}

	// 获取图片URL
	images, ok := result["images"].(map[string]interface{})
	if !ok {
		fmt.Printf("解析图片信息失败，images字段不是map类型: %T\n", result["images"])
		return "", errors.New("解析图片信息失败，未找到images字段或格式不正确")
	}
	fmt.Printf("图片信息: %+v\n", images)
	fmt.Printf("查找图片URL - 原始nodeID: %s, 转换后figmaNodeID: %s\n", nodeID, figmaNodeID)

	// 尝试所有可能的节点ID格式
	var imageURL string
	var found bool

	// 候选节点ID列表
	candidateIDs := []string{
		figmaNodeID, // 转换后的冒号格式
		nodeID,      // 原始格式
		strings.ReplaceAll(figmaNodeID, ":", "-"), // 连字符格式
		strings.ReplaceAll(nodeID, "-", ":"),      // 冒号格式（如果原始是连字符）
	}

	fmt.Printf("尝试查找图片URL，候选ID: %v\n", candidateIDs)

	for _, candidateID := range candidateIDs {
		if url, exists := images[candidateID].(string); exists && url != "" {
			imageURL = url
			found = true
			fmt.Printf("成功找到图片URL，使用节点ID: %s, URL: %s\n", candidateID, url)
			break
		}
		fmt.Printf("节点ID %s 未找到或URL为空\n", candidateID)
	}

	if !found || imageURL == "" {
		fmt.Printf("获取图片URL失败，尝试了所有候选ID: %v\n", candidateIDs)
		return "", fmt.Errorf("获取图片URL失败，节点ID=%s 不存在或URL为空", nodeID)
	}
	fmt.Printf("获取到图片URL: %s\n", imageURL)

	// 下载图片
	return downloadPreviewImageFile(imageURL, fileKey, nodeID, imageScale)
}

// 查找节点的最新图片文件（根据缩放等级）
func findLatestNodeImage(tempDir, safeNodeID string, imageScale float64) string {
	// 查找同一节点和缩放等级的所有图片文件（支持多种格式，不包括临时文件）
	fmt.Printf("查找缓存图片: tempDir=%s, safeNodeID=%s, scale=%.1f\n", tempDir, safeNodeID, imageScale)

	// 构建缩放等级字符串（用于匹配文件名）
	scaleStr := formatScaleForFilename(imageScale)

	supportedExts := []string{".png", ".svg", ".jpg", ".pdf"}
	var allFiles []string

	for _, ext := range supportedExts {
		// 匹配格式：节点-缩放等级-*.*
		pattern := filepath.Join(tempDir, safeNodeID+"-"+scaleStr+"-*"+ext)
		fmt.Printf("查找模式: %s\n", pattern)
		files, err := filepath.Glob(pattern)
		if err != nil {
			fmt.Printf("Glob查找出错: %v\n", err)
		} else {
			fmt.Printf("找到 %d 个文件匹配模式 %s\n", len(files), pattern)
			if len(files) > 0 {
				allFiles = append(allFiles, files...)
			}
		}
	}

	if len(allFiles) == 0 {
		fmt.Printf("未找到任何匹配的缓存文件\n")
		return ""
	}
	fmt.Printf("总共找到 %d 个文件\n", len(allFiles))

	// 过滤掉临时文件
	var validFiles []string
	for _, f := range allFiles {
		// 检查是否是临时文件（包含-temp.扩展名）
		if !strings.Contains(f, "-temp.") {
			validFiles = append(validFiles, f)
			fmt.Printf("有效缓存文件: %s\n", f)
		} else {
			fmt.Printf("跳过临时文件: %s\n", f)
		}
	}

	if len(validFiles) == 0 {
		fmt.Printf("过滤后没有有效的缓存文件\n")
		return ""
	}

	// 按修改时间排序
	type fileInfo struct {
		path    string
		modTime time.Time
	}
	fileInfos := make([]fileInfo, 0, len(validFiles))

	for _, f := range validFiles {
		if info, err := os.Stat(f); err == nil {
			fileInfos = append(fileInfos, fileInfo{path: f, modTime: info.ModTime()})
		}
	}

	// 按修改时间降序排序
	sort.Slice(fileInfos, func(i, j int) bool {
		return fileInfos[i].modTime.After(fileInfos[j].modTime)
	})

	// 返回最新的文件
	if len(fileInfos) > 0 {
		fmt.Printf("返回最新的缓存文件: %s (修改时间: %v)\n", fileInfos[0].path, fileInfos[0].modTime)
		return fileInfos[0].path
	}

	fmt.Printf("没有有效的文件信息\n")
	return ""
}

// 清理节点的旧图片文件（根据缩放等级）
func cleanupOldNodeImages(tempDir, safeNodeID string, imageScale float64, keepCount int) {
	// 构建缩放等级字符串
	scaleStr := formatScaleForFilename(imageScale)

	// 查找同一节点和缩放等级的所有图片文件（支持多种格式，不包括临时文件）
	supportedExts := []string{".png", ".svg", ".jpg", ".pdf"}
	var allFiles []string

	for _, ext := range supportedExts {
		// 匹配格式：节点-缩放等级-*.*
		pattern := filepath.Join(tempDir, safeNodeID+"-"+scaleStr+"-*"+ext)
		files, err := filepath.Glob(pattern)
		if err == nil && len(files) > 0 {
			allFiles = append(allFiles, files...)
		}
	}

	if len(allFiles) == 0 {
		return
	}

	// 过滤掉临时文件
	var validFiles []string
	for _, f := range allFiles {
		// 检查是否是临时文件（包含-temp.扩展名）
		if !strings.Contains(f, "-temp.") {
			validFiles = append(validFiles, f)
		}
	}

	if len(validFiles) <= keepCount {
		return
	}

	fmt.Printf("找到节点的 %d 个图片文件，将保留最新的 %d 个\n", len(validFiles), keepCount)

	// 按修改时间排序
	type fileInfo struct {
		path    string
		modTime time.Time
	}
	fileInfos := make([]fileInfo, 0, len(validFiles))

	for _, f := range validFiles {
		if info, err := os.Stat(f); err == nil {
			fileInfos = append(fileInfos, fileInfo{path: f, modTime: info.ModTime()})
		}
	}

	// 按修改时间降序排序
	sort.Slice(fileInfos, func(i, j int) bool {
		return fileInfos[i].modTime.After(fileInfos[j].modTime)
	})

	// 删除旧文件
	for i := keepCount; i < len(fileInfos); i++ {
		fmt.Printf("删除旧图片文件: %s\n", fileInfos[i].path)
		os.Remove(fileInfos[i].path)
	}
}

// 清除项目的所有图片缓存
func ClearProjectImageCache(fileKey string) error {
	return ClearProjectImageCacheByNodeIDs(fileKey, nil)
}

// ClearProjectImageCacheByNodeIDs 根据节点ID列表清除项目的图片缓存
// 如果nodeIDs为nil或空，则清除所有缓存文件
func ClearProjectImageCacheByNodeIDs(fileKey string, nodeIDs []string) error {
	// 需要清理的目录列表
	cacheDirs := []string{
		filepath.Join("temp", fileKey, "images"),
		filepath.Join("temp", fileKey, "previews"),
		filepath.Join("temp", fileKey, "fpreviews"),
	}

	totalFiles := 0
	var allErrors []string

	// 如果没有指定节点ID，则清除所有缓存文件（原有行为）
	if nodeIDs == nil || len(nodeIDs) == 0 {
		fmt.Printf("清除项目 %s 的所有缓存文件\n", fileKey)

		for _, tempDir := range cacheDirs {
			fmt.Printf("检查缓存目录: %s\n", tempDir)

			// 检查目录是否存在
			if _, err := os.Stat(tempDir); os.IsNotExist(err) {
				fmt.Printf("目录不存在，跳过: %s\n", tempDir)
				continue
			}

			// 删除所有图片文件（支持多种格式）
			supportedExts := []string{"*.png", "*.jpg", "*.jpeg", "*.svg", "*.pdf"}
			for _, ext := range supportedExts {
				pattern := filepath.Join(tempDir, ext)
				files, err := filepath.Glob(pattern)
				if err != nil {
					errMsg := fmt.Sprintf("查找缓存文件失败 %s: %v", tempDir, err)
					fmt.Println(errMsg)
					allErrors = append(allErrors, errMsg)
					continue
				}

				// 删除找到的所有文件
				for _, f := range files {
					fmt.Printf("删除缓存图片文件: %s\n", f)
					if err := os.Remove(f); err != nil {
						errMsg := fmt.Sprintf("删除文件失败: %s, 错误: %v", f, err)
						fmt.Println(errMsg)
						allErrors = append(allErrors, errMsg)
					} else {
						totalFiles++
					}
				}
			}

			// 同时清理所有 .tmp 临时文件
			tmpPattern := filepath.Join(tempDir, "*.tmp")
			tmpFiles, err := filepath.Glob(tmpPattern)
			if err != nil {
				errMsg := fmt.Sprintf("查找临时文件失败 %s: %v", tempDir, err)
				fmt.Println(errMsg)
				allErrors = append(allErrors, errMsg)
			} else {
				for _, tmpFile := range tmpFiles {
					fmt.Printf("删除临时文件: %s\n", tmpFile)
					if err := os.Remove(tmpFile); err != nil {
						errMsg := fmt.Sprintf("删除临时文件失败: %s, 错误: %v", tmpFile, err)
						fmt.Println(errMsg)
						allErrors = append(allErrors, errMsg)
					} else {
						totalFiles++
					}
				}
			}

			fmt.Printf("已清除目录 %s 中的文件\n", tempDir)
		}
	} else {
		// 按节点ID清除特定缓存文件
		fmt.Printf("清除项目 %s 中 %d 个节点的缓存文件\n", fileKey, len(nodeIDs))

		// 将节点ID转换为安全的文件名前缀
		safeNodeIDs := make([]string, len(nodeIDs))
		for i, nodeID := range nodeIDs {
			safeNodeIDs[i] = strings.NewReplacer(
				":", "_", ";", "_", "/", "_", "\\", "_",
				"*", "_", "?", "_", "\"", "_", "<", "_",
				">", "_", "|", "_", ",", "_",
			).Replace(nodeID)
		}

		for _, tempDir := range cacheDirs {
			fmt.Printf("检查缓存目录: %s\n", tempDir)

			// 检查目录是否存在
			if _, err := os.Stat(tempDir); os.IsNotExist(err) {
				fmt.Printf("目录不存在，跳过: %s\n", tempDir)
				continue
			}

			dirCleared := 0

			// 为每个节点ID查找并删除对应的缓存文件
			for i, safeNodeID := range safeNodeIDs {
				originalNodeID := nodeIDs[i]

				// 支持多种图片格式
				supportedExts := []string{".png", ".jpg", ".jpeg", ".svg", ".pdf"}

				for _, ext := range supportedExts {
					// 查找以节点ID为前缀的文件
					pattern := filepath.Join(tempDir, safeNodeID+"-*"+ext)
					files, err := filepath.Glob(pattern)
					if err != nil {
						errMsg := fmt.Sprintf("查找节点 %s 的缓存文件失败: %v", originalNodeID, err)
						fmt.Println(errMsg)
						allErrors = append(allErrors, errMsg)
						continue
					}

					// 删除找到的文件
					for _, file := range files {
						fmt.Printf("删除节点 %s 的缓存文件: %s\n", originalNodeID, file)
						if err := os.Remove(file); err != nil {
							errMsg := fmt.Sprintf("删除文件失败: %s, 错误: %v", file, err)
							fmt.Println(errMsg)
							allErrors = append(allErrors, errMsg)
						} else {
							dirCleared++
							totalFiles++
						}
					}
				}
			}

			// 无论是否按节点ID清除，都要清理所有 .tmp 临时文件
			tmpPattern := filepath.Join(tempDir, "*.tmp")
			tmpFiles, err := filepath.Glob(tmpPattern)
			if err != nil {
				errMsg := fmt.Sprintf("查找临时文件失败 %s: %v", tempDir, err)
				fmt.Println(errMsg)
				allErrors = append(allErrors, errMsg)
			} else {
				for _, tmpFile := range tmpFiles {
					fmt.Printf("删除临时文件: %s\n", tmpFile)
					if err := os.Remove(tmpFile); err != nil {
						errMsg := fmt.Sprintf("删除临时文件失败: %s, 错误: %v", tmpFile, err)
						fmt.Println(errMsg)
						allErrors = append(allErrors, errMsg)
					} else {
						dirCleared++
						totalFiles++
					}
				}
			}

			fmt.Printf("目录 %s 中清除了 %d 个文件\n", tempDir, dirCleared)
		}
	}

	fmt.Printf("已清除项目 %s 的缓存图片，共 %d 个文件\n", fileKey, totalFiles)

	// 如果有错误，返回合并的错误信息
	if len(allErrors) > 0 {
		return fmt.Errorf("清理过程中发生错误: %s", strings.Join(allErrors, "; "))
	}

	return nil
}

// DownloadFilteredPreviewFigmaImage 下载过滤后的Figma预览图片（使用过滤后的节点ID列表）
func DownloadFilteredPreviewFigmaImage(token, fileKey, nodeID, imageFormat string, imageScale float64, projectID uint, useCache bool) (string, error) {
	// 打印调试信息
	fmt.Printf("开始下载过滤后的Figma预览图片: fileKey=%s, nodeID=%s, format=%s, scale=%.1f, projectID=%d, useCache=%v\n",
		fileKey, nodeID, imageFormat, imageScale, projectID, useCache)

	// 获取节点树，用于过滤
	nodes, err := GetFigmaNodes(token, fileKey, nodeID)
	if err != nil {
		fmt.Printf("获取节点树失败: %v\n", err)
		return "", fmt.Errorf("获取节点树失败: %v", err)
	}

	// 获取所有节点的修改信息
	var figmaNodes []models.FigmaNode
	dbResult := models.DB.Where("project_id = ?", projectID).Find(&figmaNodes)
	if dbResult.Error != nil {
		fmt.Printf("获取节点修改信息失败: %v\n", dbResult.Error)
		return "", fmt.Errorf("获取节点修改信息失败: %v", dbResult.Error)
	}

	// 构建节点ID到修改信息的映射
	nodeModifys := make(map[string]map[string]interface{})
	for _, node := range figmaNodes {
		if node.Modifys != "" {
			var modifyData map[string]interface{}
			if err := json.Unmarshal([]byte(node.Modifys), &modifyData); err == nil {
				nodeModifys[node.NodeID] = modifyData
			}
		}
	}

	// 使用树型遍历收集所有需要包含的节点ID（排除被过滤的节点及其子树）
	includedNodeIDs := collectIncludedNodeIDs(nodes, nodeID, nodeModifys)

	// 如果没有需要包含的节点，返回错误
	if len(includedNodeIDs) == 0 {
		fmt.Printf("没有需要包含的节点\n")
		return "", fmt.Errorf("没有需要包含的节点")
	}

	// 格式化节点ID列表为 l11:22,123:33 格式
	nodeIDsParam := "I" + strings.Join(includedNodeIDs, ",")
	fmt.Printf("格式化后的节点ID参数: %s\n", nodeIDsParam)

	// 检查是否已经有缓存的图片文件
	tempDir := filepath.Join("temp", fileKey, "fpreviews")
	err = os.MkdirAll(tempDir, os.ModePerm)
	if err != nil {
		fmt.Printf("创建临时文件夹失败: %v\n", err)
		return "", fmt.Errorf("创建临时文件夹失败: %v", err)
	}

	// 基于过滤后的节点ID列表生成文件名
	// 命名和preview规则一样，用nodeid去掉特殊符号加签
	safeFileName := strings.NewReplacer(":", "_", ";", "_", "/", "_", "\\", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_").Replace(nodeID)
	nodeIDsHash := generateNodeIDsHash(includedNodeIDs)
	safeFileName = fmt.Sprintf("%s-%s", safeFileName, nodeIDsHash)

	// 如果使用缓存，检查是否有现有文件
	if useCache {
		existingFile := findLatestNodeImage(tempDir, safeFileName, imageScale)
		if existingFile != "" {
			fmt.Printf("找到过滤预览图片缓存文件，直接使用: %s\n", existingFile)
			return existingFile, nil
		}
	} else {
		fmt.Printf("跳过缓存检查，强制重新下载过滤预览图片\n")
	}

	// 调用Figma API获取图片URL
	apiUrl := fmt.Sprintf("https://api.figma.com/v1/images/%s?ids=%s&format=%s&scale=%.1f",
		fileKey, nodeIDsParam, imageFormat, imageScale)
	fmt.Printf("Figma API URL: %s\n", apiUrl)

	// 创建请求
	req, err := http.NewRequest("GET", apiUrl, nil)
	if err != nil {
		fmt.Printf("创建请求失败: %v\n", err)
		return "", fmt.Errorf("创建API请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("X-Figma-Token", token)
	fmt.Printf("请求头已设置: X-Figma-Token=%s...\n", token[:5]+"***")

	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	fmt.Printf("发送请求获取图片URL...\n")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("请求失败: %v\n", err)
		return "", fmt.Errorf("发送API请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	fmt.Printf("响应状态码: %d\n", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("API请求失败: %s", resp.Status)
		fmt.Println(errMsg)
		return "", fmt.Errorf(errMsg)
	}

	// 解析响应
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("读取响应内容失败: %v\n", err)
		return "", fmt.Errorf("读取API响应失败: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(responseBody, &result); err != nil {
		fmt.Printf("解析响应JSON失败: %v\n", err)
		return "", fmt.Errorf("解析API响应JSON失败: %v", err)
	}

	// 获取图片URL - Figma API返回的是节点ID到URL的映射
	images, ok := result["images"].(map[string]interface{})
	if !ok {
		fmt.Printf("解析图片信息失败，images字段不是map类型: %T\n", result["images"])
		return "", errors.New("解析图片信息失败，未找到images字段或格式不正确")
	}
	fmt.Printf("图片信息: %+v\n", images)

	// 获取第一个可用的图片URL（通常Figma会返回合成后的图片）
	var imageURL string
	for _, url := range images {
		if urlStr, ok := url.(string); ok && urlStr != "" {
			imageURL = urlStr
			break
		}
	}

	if imageURL == "" {
		fmt.Printf("获取图片URL失败，没有找到有效的图片URL\n")
		return "", fmt.Errorf("获取图片URL失败，没有找到有效的图片URL")
	}
	fmt.Printf("获取到图片URL: %s\n", imageURL)

	// 下载图片
	fmt.Printf("发现 %d 个包含的节点，下载过滤后的预览图片\n", len(includedNodeIDs))
	return downloadFilteredPreviewImageFileByNodeIDs(imageURL, fileKey, safeFileName, includedNodeIDs, imageScale)
}

// downloadPreviewImageFile 下载图片文件
func downloadPreviewImageFile(url, fileKey, nodeID string, imageScale float64) (string, error) {
	fmt.Printf("第二步：开始下载图片文件: url=%s\n", url)

	// 创建临时文件夹
	tempDir := filepath.Join("temp", fileKey, "previews")
	fmt.Printf("临时文件夹路径: %s\n", tempDir)
	err := os.MkdirAll(tempDir, os.ModePerm)
	if err != nil {
		fmt.Printf("创建临时文件夹失败: %v\n", err)
		return "", fmt.Errorf("创建临时文件夹失败: %v", err)
	}

	// 生成文件名 - 替换特殊字符为下划线
	safeNodeID := strings.NewReplacer(":", "_", ";", "_", "/", "_", "\\", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_").Replace(nodeID)

	// 临时文件名（不带哈希值）
	tempFileName := fmt.Sprintf("%s-temp.png", safeNodeID)
	tempFilePath := filepath.Join(tempDir, tempFileName)
	fmt.Printf("临时文件路径: %s (原节点ID: %s)\n", tempFilePath, nodeID)

	// 我们现在使用文件内容的哈希值作为文件名的一部分
	// 这样相同内容的文件会有相同的名称，避免重复下载
	// 但我们仍需要清理过多的旧文件，避免占用过多磁盘空间

	// 清理同一节点和缩放等级的过多旧文件
	cleanupOldNodeImages(tempDir, safeNodeID, imageScale, 10)

	// 下载图片
	fmt.Printf("开始下载图片: %s\n", url)

	// 创建带超时的HTTP客户端
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// 创建请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("创建图片下载请求失败: %v\n", err)
		return "", fmt.Errorf("创建图片下载请求失败: %v", err)
	}

	// 添加请求头
	req.Header.Set("User-Agent", "Mozilla/5.0 FigmaDeliver/1.0")

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("下载图片请求失败: %v\n", err)
		return "", fmt.Errorf("下载图片请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	fmt.Printf("图片下载响应状态码: %d\n", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("下载图片失败: %s", resp.Status)
		fmt.Println(errMsg)
		return "", fmt.Errorf(errMsg)
	}

	// 检查Content-Type
	contentType := resp.Header.Get("Content-Type")
	fmt.Printf("图片Content-Type: %s\n", contentType)
	if !strings.HasPrefix(contentType, "image/") {
		fmt.Printf("警告：响应Content-Type不是图片类型: %s\n", contentType)
	}

	// 创建临时文件，避免直接写入目标文件可能导致的损坏
	downloadTempPath := tempFilePath + ".tmp"
	fmt.Printf("创建临时文件: %s\n", downloadTempPath)
	file, err := os.Create(downloadTempPath)
	if err != nil {
		fmt.Printf("创建临时文件失败: %v\n", err)
		return "", fmt.Errorf("创建临时文件失败: %v", err)
	}

	// 创建一个TeeReader，同时写入文件和计算哈希值
	fmt.Printf("将响应内容写入临时文件并计算哈希值...\n")
	hasher := md5.New()
	reader := io.TeeReader(resp.Body, hasher)

	bytes, err := io.Copy(file, reader)
	if err != nil {
		file.Close()
		os.Remove(downloadTempPath) // 清理临时文件
		fmt.Printf("写入临时文件失败: %v\n", err)
		return "", fmt.Errorf("写入临时文件失败: %v", err)
	}

	// 关闭文件
	if err := file.Close(); err != nil {
		os.Remove(downloadTempPath) // 清理临时文件
		fmt.Printf("关闭临时文件失败: %v\n", err)
		return "", fmt.Errorf("关闭临时文件失败: %v", err)
	}

	// 检查文件大小
	if bytes == 0 {
		os.Remove(downloadTempPath) // 清理临时文件
		fmt.Printf("错误：下载的图片大小为0字节\n")
		return "", errors.New("下载的图片大小为0字节")
	}

	fmt.Printf("成功写入 %d 字节到临时文件\n", bytes)

	// 获取文件数据的哈希值
	hash := hex.EncodeToString(hasher.Sum(nil))[0:6]
	fmt.Printf("文件数据哈希值: %s\n", hash)

	// 使用哈希值构建最终文件名（包含缩放等级）
	// 格式：节点-缩放等级-hash.png，例如：节点-1-hash.png 或 节点-0.5-hash.png
	scaleStr := formatScaleForFilename(imageScale)
	fileName := fmt.Sprintf("%s-%s-%s.png", safeNodeID, scaleStr, hash)
	filePath := filepath.Join(tempDir, fileName)
	fmt.Printf("最终文件路径: %s\n", filePath)

	// 检查同名文件是否已存在（相同内容的文件）
	if _, err := os.Stat(filePath); err == nil {
		// 文件已存在，删除临时文件
		os.Remove(tempFilePath)
		fmt.Printf("文件已存在（相同内容），直接使用: %s\n", filePath)
		return filePath, nil
	}

	// 将临时文件重命名为最终文件
	if err := os.Rename(downloadTempPath, filePath); err != nil {
		os.Remove(downloadTempPath) // 清理临时文件
		fmt.Printf("重命名临时文件失败: %v\n", err)
		return "", fmt.Errorf("重命名临时文件失败: %v", err)
	}

	fmt.Printf("图片下载完成: %s (%d 字节)\n", filePath, bytes)
	return filePath, nil
}

// generateNodeIDsHash 基于节点ID列表生成哈希值
func generateNodeIDsHash(nodeIDs []string) string {
	// 对节点ID列表进行排序，确保相同的节点集合产生相同的哈希
	sortedNodeIDs := make([]string, len(nodeIDs))
	copy(sortedNodeIDs, nodeIDs)
	sort.Strings(sortedNodeIDs)

	// 将排序后的节点ID连接成字符串
	nodeIDsStr := strings.Join(sortedNodeIDs, ",")

	// 计算MD5哈希值并取前8位
	hasher := md5.New()
	hasher.Write([]byte(nodeIDsStr))
	hash := hex.EncodeToString(hasher.Sum(nil))[0:8]

	fmt.Printf("节点ID列表哈希: %s (基于 %d 个节点)\n", hash, len(nodeIDs))
	return hash
}

// downloadFilteredPreviewImageFileByNodeIDs 基于节点ID列表下载过滤后的预览图片文件
func downloadFilteredPreviewImageFileByNodeIDs(url, fileKey, safeFileName string, includedNodeIDs []string, imageScale float64) (string, error) {
	fmt.Printf("开始下载过滤后的预览图片文件: url=%s, 包含 %d 个节点\n", url, len(includedNodeIDs))

	// 创建临时文件夹
	tempDir := filepath.Join("temp", fileKey, "fpreviews")
	fmt.Printf("临时文件夹路径: %s\n", tempDir)
	err := os.MkdirAll(tempDir, os.ModePerm)
	if err != nil {
		fmt.Printf("创建临时文件夹失败: %v\n", err)
		return "", fmt.Errorf("创建临时文件夹失败: %v", err)
	}

	// 临时文件名（不带哈希值），使用默认png格式
	// 注意：此函数没有imageFormat参数，所以我们使用默认扩展名
	// 最终文件名会根据Content-Type调整
	fileExt := ".png" // 默认为png

	tempFileName := fmt.Sprintf("%s-temp%s", safeFileName, fileExt)
	tempFilePath := filepath.Join(tempDir, tempFileName)
	fmt.Printf("临时文件路径: %s\n", tempFilePath)

	// 清理同一类型和缩放等级的过多旧文件
	cleanupOldNodeImages(tempDir, safeFileName, imageScale, 10)

	// 下载图片
	fmt.Printf("开始下载过滤后的预览图片: %s\n", url)

	// 创建带超时的HTTP客户端
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// 创建请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("创建图片下载请求失败: %v\n", err)
		return "", fmt.Errorf("创建图片下载请求失败: %v", err)
	}

	// 添加请求头
	req.Header.Set("User-Agent", "Mozilla/5.0 FigmaDeliver/1.0")

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("下载图片请求失败: %v\n", err)
		return "", fmt.Errorf("下载图片请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	fmt.Printf("图片下载响应状态码: %d\n", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("下载图片失败: %s", resp.Status)
		fmt.Println(errMsg)
		return "", fmt.Errorf(errMsg)
	}

	// 检查Content-Type
	contentType := resp.Header.Get("Content-Type")
	fmt.Printf("图片Content-Type: %s\n", contentType)
	if !strings.HasPrefix(contentType, "image/") {
		fmt.Printf("警告：响应Content-Type不是图片类型: %s\n", contentType)
	}

	// 创建临时文件，避免直接写入目标文件可能导致的损坏
	downloadTempPath := tempFilePath + ".tmp"
	fmt.Printf("创建临时文件: %s\n", downloadTempPath)
	file, err := os.Create(downloadTempPath)
	if err != nil {
		fmt.Printf("创建临时文件失败: %v\n", err)
		return "", fmt.Errorf("创建临时文件失败: %v", err)
	}

	// 创建一个TeeReader，同时写入文件和计算哈希值
	fmt.Printf("将响应内容写入临时文件并计算哈希值...\n")
	hasher := md5.New()
	reader := io.TeeReader(resp.Body, hasher)

	bytes, err := io.Copy(file, reader)
	if err != nil {
		file.Close()
		os.Remove(downloadTempPath) // 清理临时文件
		fmt.Printf("写入临时文件失败: %v\n", err)
		return "", fmt.Errorf("写入临时文件失败: %v", err)
	}

	// 关闭文件
	if err := file.Close(); err != nil {
		os.Remove(downloadTempPath) // 清理临时文件
		fmt.Printf("关闭临时文件失败: %v\n", err)
		return "", fmt.Errorf("关闭临时文件失败: %v", err)
	}

	// 检查文件大小
	if bytes == 0 {
		os.Remove(downloadTempPath) // 清理临时文件
		fmt.Printf("错误：下载的图片大小为0字节\n")
		return "", errors.New("下载的图片大小为0字节")
	}

	fmt.Printf("成功写入 %d 字节到临时文件\n", bytes)

	// 获取文件数据的哈希值
	hash := hex.EncodeToString(hasher.Sum(nil))[0:6]
	fmt.Printf("文件数据哈希值: %s\n", hash)

	// 根据Content-Type确定文件扩展名
	finalFileExt := ".png" // 默认为png
	if strings.HasPrefix(contentType, "image/") {
		switch contentType {
		case "image/svg+xml":
			finalFileExt = ".svg"
		case "image/jpeg":
			finalFileExt = ".jpg"
		case "image/png":
			finalFileExt = ".png"
		case "application/pdf":
			finalFileExt = ".pdf"
		}
	}

	// 使用哈希值和正确的扩展名构建最终文件名（包含缩放等级）
	// 格式：文件名-缩放等级-hash.ext
	scaleStr := formatScaleForFilename(imageScale)
	fileName := fmt.Sprintf("%s-%s-%s%s", safeFileName, scaleStr, hash, finalFileExt)
	filePath := filepath.Join(tempDir, fileName)
	fmt.Printf("最终文件路径: %s (Content-Type: %s)\n", filePath, contentType)

	// 检查同名文件是否已存在（相同内容的文件）
	if _, err := os.Stat(filePath); err == nil {
		// 文件已存在，删除临时文件
		os.Remove(downloadTempPath)
		fmt.Printf("文件已存在（相同内容），直接使用: %s\n", filePath)
		return filePath, nil
	}

	// 将临时文件重命名为最终文件
	if err := os.Rename(downloadTempPath, filePath); err != nil {
		os.Remove(downloadTempPath) // 清理临时文件
		fmt.Printf("重命名临时文件失败: %v\n", err)
		return "", fmt.Errorf("重命名临时文件失败: %v", err)
	}

	fmt.Printf("过滤后的预览图片下载完成: %s (%d 字节)\n", filePath, bytes)
	return filePath, nil
}

// DownloadFilteredPreviewFigmaImages 批量下载过滤后的Figma预览图片（返回节点ID到图片路径的映射）
func DownloadFilteredPreviewFigmaImages(token, fileKey, nodeID, imageFormat string, imageScale float64, projectID uint) (map[string]string, error) {
	// 打印调试信息
	fmt.Printf("开始批量下载过滤后的Figma预览图片: fileKey=%s, nodeID=%s, format=%s, scale=%.1f, projectID=%d\n",
		fileKey, nodeID, imageFormat, imageScale, projectID)

	// 获取节点树，用于过滤
	nodes, err := GetFigmaNodes(token, fileKey, nodeID)
	if err != nil {
		fmt.Printf("获取节点树失败: %v\n", err)
		return nil, fmt.Errorf("获取节点树失败: %v", err)
	}

	// 获取所有节点的修改信息
	var figmaNodes []models.FigmaNode
	dbResult := models.DB.Where("project_id = ?", projectID).Find(&figmaNodes)
	if dbResult.Error != nil {
		fmt.Printf("获取节点修改信息失败: %v\n", dbResult.Error)
		return nil, fmt.Errorf("获取节点修改信息失败: %v", dbResult.Error)
	}

	// 构建节点ID到修改信息的映射
	nodeModifys := make(map[string]map[string]interface{})
	for _, node := range figmaNodes {
		if node.Modifys != "" {
			var modifyData map[string]interface{}
			if err := json.Unmarshal([]byte(node.Modifys), &modifyData); err == nil {
				nodeModifys[node.NodeID] = modifyData
			}
		}
	}

	// 构建节点ID到节点信息的映射
	nodeMap := make(map[string]map[string]interface{})
	for _, node := range nodes {
		nodeID := node["id"].(string)
		nodeMap[nodeID] = node
	}

	// 使用树型遍历收集所有需要包含的节点ID（排除被过滤的节点及其子树）
	includedNodeIDs := collectIncludedNodeIDs(nodes, nodeID, nodeModifys)

	// 如果没有需要包含的节点，返回错误
	if len(includedNodeIDs) == 0 {
		fmt.Printf("没有需要包含的节点\n")
		return nil, fmt.Errorf("没有需要包含的节点")
	}

	// 格式化节点ID列表为 I11:22,123:33 格式
	nodeIDsParam := strings.Join(includedNodeIDs, ",")
	fmt.Printf("格式化后的节点ID参数: %s\n", nodeIDsParam)

	// 检查是否已经有缓存的图片文件
	tempDir := filepath.Join("temp", fileKey, "fpreviews")
	err = os.MkdirAll(tempDir, os.ModePerm)
	if err != nil {
		fmt.Printf("创建临时文件夹失败: %v\n", err)
		return nil, fmt.Errorf("创建临时文件夹失败: %v", err)
	}

	// 调用Figma API获取图片URL
	apiUrl := fmt.Sprintf("https://api.figma.com/v1/images/%s?ids=%s&format=%s&scale=%.1f",
		fileKey, nodeIDsParam, imageFormat, imageScale)
	fmt.Printf("Figma API URL: %s\n", apiUrl)

	// 创建请求
	req, err := http.NewRequest("GET", apiUrl, nil)
	if err != nil {
		fmt.Printf("创建请求失败: %v\n", err)
		return nil, fmt.Errorf("创建API请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("X-Figma-Token", token)
	fmt.Printf("请求头已设置: X-Figma-Token=%s...\n", token[:5]+"***")

	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	fmt.Printf("发送请求获取图片URL...\n")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("请求失败: %v\n", err)
		return nil, fmt.Errorf("发送API请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	fmt.Printf("响应状态码: %d\n", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("API请求失败: %s", resp.Status)
		fmt.Println(errMsg)
		return nil, fmt.Errorf(errMsg)
	}

	// 解析响应
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("读取响应内容失败: %v\n", err)
		return nil, fmt.Errorf("读取API响应失败: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(responseBody, &result); err != nil {
		fmt.Printf("解析响应JSON失败: %v\n", err)
		return nil, fmt.Errorf("解析API响应JSON失败: %v", err)
	}

	// 获取图片URL - Figma API返回的是节点ID到URL的映射
	images, ok := result["images"].(map[string]interface{})
	if !ok {
		fmt.Printf("解析图片信息失败，images字段不是map类型: %T\n", result["images"])
		return nil, errors.New("解析图片信息失败，未找到images字段或格式不正确")
	}
	fmt.Printf("图片信息: %+v\n", images)

	// 批量下载所有图片并建立映射
	imagePathMap := make(map[string]string)

	for _, nodeID := range includedNodeIDs {
		imageURL, ok := images[nodeID].(string)
		if !ok || imageURL == "" {
			fmt.Printf("节点 %s 没有对应的图片URL，跳过\n", nodeID)
			continue
		}

		// 为每个节点生成文件名
		fileName := generateNodeFileName(nodeID, nodeMap[nodeID], nodeModifys[nodeID])

		// 下载单个图片
		filePath, err := downloadSingleFilteredPreviewImage(imageURL, fileKey, nodeID, fileName, tempDir, imageScale)
		if err != nil {
			fmt.Printf("下载节点 %s 的图片失败: %v\n", nodeID, err)
			continue
		}

		imagePathMap[nodeID] = filePath
		fmt.Printf("成功下载节点 %s 的图片: %s\n", nodeID, filePath)
	}

	fmt.Printf("批量下载完成，成功下载 %d 个图片\n", len(imagePathMap))
	return imagePathMap, nil
}

// generateNodeFileName 为节点生成文件名
func generateNodeFileName(nodeID string, nodeInfo map[string]interface{}, nodeModifys map[string]interface{}) string {
	var nodeName string

	// 优先使用修改信息中的rename
	if nodeModifys != nil {
		if rename, ok := nodeModifys["rename"].(string); ok && rename != "" {
			nodeName = rename
		}
	}

	// 如果没有rename，使用节点原始名称
	if nodeName == "" && nodeInfo != nil {
		if name, ok := nodeInfo["name"].(string); ok {
			nodeName = name
		}
	}

	// 如果都没有，使用nodeID作为后备
	if nodeName == "" {
		nodeName = nodeID
	}

	// 移除特殊字符
	safeNodeName := strings.NewReplacer(":", "_", ";", "_", "/", "_", "\\", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_", " ", "_").Replace(nodeName)

	return safeNodeName
}

// downloadSingleFilteredPreviewImage 下载单个过滤预览图片
func downloadSingleFilteredPreviewImage(imageURL, _ /*fileKey*/, nodeID, fileName, tempDir string, imageScale float64) (string, error) {
	fmt.Printf("开始下载单个图片: nodeID=%s, fileName=%s, url=%s, scale=%.1f\n", nodeID, fileName, imageURL, imageScale)

	// 检查是否已经有缓存的图片文件
	existingFile := findLatestNodeImage(tempDir, fileName, imageScale)
	if existingFile != "" {
		fmt.Printf("找到节点 %s 的缓存图片文件，直接使用: %s\n", nodeID, existingFile)
		return existingFile, nil
	}

	// 创建带超时的HTTP客户端
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// 创建请求
	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		fmt.Printf("创建图片下载请求失败: %v\n", err)
		return "", fmt.Errorf("创建图片下载请求失败: %v", err)
	}

	// 添加请求头
	req.Header.Set("User-Agent", "Mozilla/5.0 FigmaDeliver/1.0")

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("下载图片请求失败: %v\n", err)
		return "", fmt.Errorf("下载图片请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	fmt.Printf("图片下载响应状态码: %d\n", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("下载图片失败: %s", resp.Status)
		fmt.Println(errMsg)
		return "", fmt.Errorf(errMsg)
	}

	// 检查Content-Type
	contentType := resp.Header.Get("Content-Type")
	fmt.Printf("图片Content-Type: %s\n", contentType)
	if !strings.HasPrefix(contentType, "image/") {
		fmt.Printf("警告：响应Content-Type不是图片类型: %s\n", contentType)
	}

	// 临时文件名（不带哈希值），根据请求的格式确定扩展名
	tempFileExt := ".png" // 默认为png
	// 这里不能直接使用imageFormat参数，因为这个函数没有该参数
	// 但我们可以从Content-Type推断
	if strings.HasPrefix(contentType, "image/") {
		switch contentType {
		case "image/svg+xml":
			tempFileExt = ".svg"
		case "image/jpeg":
			tempFileExt = ".jpg"
		case "image/png":
			tempFileExt = ".png"
		case "application/pdf":
			tempFileExt = ".pdf"
		}
	}

	tempFileName := fmt.Sprintf("%s-temp%s", fileName, tempFileExt)
	tempFilePath := filepath.Join(tempDir, tempFileName)

	// 创建临时文件，避免直接写入目标文件可能导致的损坏
	downloadTempPath := tempFilePath + ".tmp"
	fmt.Printf("创建临时文件: %s\n", downloadTempPath)
	file, err := os.Create(downloadTempPath)
	if err != nil {
		fmt.Printf("创建临时文件失败: %v\n", err)
		return "", fmt.Errorf("创建临时文件失败: %v", err)
	}

	// 创建一个TeeReader，同时写入文件和计算哈希值
	fmt.Printf("将响应内容写入临时文件并计算哈希值...\n")
	hasher := md5.New()
	reader := io.TeeReader(resp.Body, hasher)

	bytes, err := io.Copy(file, reader)
	if err != nil {
		file.Close()
		os.Remove(downloadTempPath) // 清理临时文件
		fmt.Printf("写入临时文件失败: %v\n", err)
		return "", fmt.Errorf("写入临时文件失败: %v", err)
	}

	// 关闭文件
	if err := file.Close(); err != nil {
		os.Remove(downloadTempPath) // 清理临时文件
		fmt.Printf("关闭临时文件失败: %v\n", err)
		return "", fmt.Errorf("关闭临时文件失败: %v", err)
	}

	// 检查文件大小
	if bytes == 0 {
		os.Remove(downloadTempPath) // 清理临时文件
		fmt.Printf("错误：下载的图片大小为0字节\n")
		return "", errors.New("下载的图片大小为0字节")
	}

	fmt.Printf("成功写入 %d 字节到临时文件\n", bytes)

	// 获取文件数据的哈希值
	hash := hex.EncodeToString(hasher.Sum(nil))[0:6]
	fmt.Printf("文件数据哈希值: %s\n", hash)

	// 根据Content-Type确定文件扩展名
	resultFileExt := ".png" // 默认为png
	if strings.HasPrefix(contentType, "image/") {
		switch contentType {
		case "image/svg+xml":
			resultFileExt = ".svg"
		case "image/jpeg":
			resultFileExt = ".jpg"
		case "image/png":
			resultFileExt = ".png"
		case "application/pdf":
			resultFileExt = ".pdf"
		}
	}

	// 使用哈希值和正确的扩展名构建最终文件名（包含缩放等级）
	// 格式：文件名-缩放等级-hash.ext
	scaleStr := formatScaleForFilename(imageScale)
	finalFileName := fmt.Sprintf("%s-%s-%s%s", fileName, scaleStr, hash, resultFileExt)
	filePath := filepath.Join(tempDir, finalFileName)
	fmt.Printf("最终文件路径: %s (Content-Type: %s)\n", filePath, contentType)

	// 检查同名文件是否已存在（相同内容的文件）
	if _, err := os.Stat(filePath); err == nil {
		// 文件已存在，删除临时文件
		os.Remove(downloadTempPath)
		fmt.Printf("文件已存在（相同内容），直接使用: %s\n", filePath)
		return filePath, nil
	}

	// 将临时文件重命名为最终文件
	if err := os.Rename(downloadTempPath, filePath); err != nil {
		os.Remove(downloadTempPath) // 清理临时文件
		fmt.Printf("重命名临时文件失败: %v\n", err)
		return "", fmt.Errorf("重命名临时文件失败: %v", err)
	}

	// 清理同一节点和缩放等级的过多旧文件
	cleanupOldNodeImages(tempDir, fileName, imageScale, 10)

	fmt.Printf("单个图片下载完成: %s (%d 字节)\n", filePath, bytes)
	return filePath, nil
}

// collectIncludedNodeIDs 使用树型遍历收集所有需要包含的节点ID
// 在遍历过程中直接判断节点是否应该被排除，当节点被排除时，跳过其整个子树
func collectIncludedNodeIDs(nodes []map[string]interface{}, rootNodeID string, nodeModifys map[string]map[string]interface{}) []string {
	fmt.Printf("开始树型遍历收集包含的节点ID，根节点: %s\n", rootNodeID)

	// 构建节点ID到节点的映射，方便查找
	nodeMap := make(map[string]map[string]interface{})
	for _, node := range nodes {
		nodeID := node["id"].(string)
		nodeMap[nodeID] = node
	}

	// 构建父子关系映射
	childrenMap := make(map[string][]string)
	for _, node := range nodes {
		nodeID := node["id"].(string)
		if parentID, ok := node["parent_id"]; ok && parentID != nil {
			if parentIDStr, ok := parentID.(string); ok {
				childrenMap[parentIDStr] = append(childrenMap[parentIDStr], nodeID)
			}
		}
	}

	var includedNodeIDs []string
	visitedNodes := make(map[string]bool) // 记录已访问的节点，避免重复

	// 直接遍历根节点的子节点，不判断根节点自身
	if children, hasChildren := childrenMap[rootNodeID]; hasChildren {
		fmt.Printf("根节点 %s 有 %d 个子节点，直接遍历子节点\n", rootNodeID, len(children))
		for _, childID := range children {
			traverseNodeTree(childID, nodeMap, childrenMap, nodeModifys, visitedNodes, &includedNodeIDs)
		}
	} else {
		fmt.Printf("根节点 %s 没有子节点\n", rootNodeID)
	}

	fmt.Printf("树型遍历完成，收集到 %d 个包含的节点ID\n", len(includedNodeIDs))
	return includedNodeIDs
}

// shouldExcludeNode 判断节点是否应该被排除
func shouldExcludeNode(node map[string]interface{}, modifys map[string]interface{}) bool {
	nodeID := node["id"].(string)

	// 检查visible属性，如果为false则排除
	if visible, hasVisible := node["visible"]; hasVisible {
		if visibleBool, ok := visible.(bool); ok && !visibleBool {
			fmt.Printf("排除节点 %s，因为其visible为false\n", nodeID)
			return true
		}
	}

	// 检查modifys中的排除条件
	if modifys != nil {
		// 检查是否标记为忽略
		if ignore, ok := modifys["ignore"].(bool); ok && ignore {
			fmt.Printf("排除节点 %s，因为其被标记为忽略\n", nodeID)
			return true
		}

		// 检查是否有res_mode属性，且不是"attach"
		if resMode, ok := modifys["res_mode"].(string); ok && resMode != "attach" {
			fmt.Printf("排除节点 %s，因为其资源模式为 %s\n", nodeID, resMode)
			return true
		}
	}

	// 检查TEXT节点的特殊逻辑：如为TEXT节点，且没有modifys或者modifys中的res_mode不是attach，需要排除
	nodeType, _ := node["type"].(string)
	if nodeType == "TEXT" {
		if modifys == nil {
			fmt.Printf("排除节点 %s，因为其类型为TEXT且没有modifys\n", nodeID)
			return true
		}
		if resMode, ok := modifys["res_mode"].(string); !ok || resMode != "attach" {
			fmt.Printf("排除节点 %s，因为其类型为TEXT且modifys.res_mode不是attach\n", nodeID)
			return true
		}
	}

	return false
}

// traverseNodeTree 递归遍历节点树，在遍历过程中直接判断并收集需要包含的节点ID
func traverseNodeTree(nodeID string, nodeMap map[string]map[string]interface{}, childrenMap map[string][]string, nodeModifys map[string]map[string]interface{}, visitedNodes map[string]bool, includedNodeIDs *[]string) {
	// 检查节点是否存在
	node, exists := nodeMap[nodeID]
	if !exists {
		fmt.Printf("节点 %s 不存在，跳过\n", nodeID)
		return
	}

	// 检查节点是否已经被访问过，避免重复处理
	if visitedNodes[nodeID] {
		fmt.Printf("节点 %s 已经被访问过，跳过\n", nodeID)
		return
	}

	// 标记节点为已访问
	visitedNodes[nodeID] = true

	// 在遍历过程中直接判断当前节点是否应该被排除
	shouldExclude := shouldExcludeNode(node, nodeModifys[nodeID])
	if shouldExclude {
		fmt.Printf("节点 %s 被排除，跳过其整个子树\n", nodeID)
		return // 跳过整个子树
	}

	// 当前节点未被排除，添加到包含列表
	*includedNodeIDs = append(*includedNodeIDs, nodeID)
	fmt.Printf("包含节点: %s\n", nodeID)

	// 递归处理子节点
	if children, hasChildren := childrenMap[nodeID]; hasChildren {
		fmt.Printf("节点 %s 有 %d 个子节点，继续遍历\n", nodeID, len(children))
		for _, childID := range children {
			traverseNodeTree(childID, nodeMap, childrenMap, nodeModifys, visitedNodes, includedNodeIDs)
		}
	} else {
		fmt.Printf("节点 %s 没有子节点\n", nodeID)
	}
}

// GetNodeSubtreeIDs 获取指定节点及其子树的所有节点ID
// 如果nodeID为空或等于根节点ID，则返回所有节点ID
func GetNodeSubtreeIDs(token, fileKey, rootNodeID, targetNodeID string) ([]string, error) {
	// 如果目标节点ID为空，使用根节点ID
	if targetNodeID == "" {
		targetNodeID = rootNodeID
	}

	// 获取完整的节点树
	nodes, err := GetFigmaNodes(token, fileKey, rootNodeID)
	if err != nil {
		return nil, fmt.Errorf("获取节点树失败: %v", err)
	}

	// 如果目标节点就是根节点，返回所有节点ID
	if targetNodeID == rootNodeID {
		var allNodeIDs []string
		for _, node := range nodes {
			if nodeID, ok := node["id"].(string); ok {
				allNodeIDs = append(allNodeIDs, nodeID)
			}
		}
		return allNodeIDs, nil
	}

	// 构建节点ID到节点的映射
	nodeMap := make(map[string]map[string]interface{})
	for _, node := range nodes {
		nodeID := node["id"].(string)
		nodeMap[nodeID] = node
	}

	// 构建父子关系映射
	childrenMap := make(map[string][]string)
	for _, node := range nodes {
		nodeID := node["id"].(string)
		if parentID, ok := node["parent_id"]; ok && parentID != nil {
			if parentIDStr, ok := parentID.(string); ok {
				childrenMap[parentIDStr] = append(childrenMap[parentIDStr], nodeID)
			}
		}
	}

	// 检查目标节点是否存在
	if _, exists := nodeMap[targetNodeID]; !exists {
		return nil, fmt.Errorf("目标节点 %s 不存在", targetNodeID)
	}

	// 收集目标节点及其子树的所有节点ID
	var subtreeNodeIDs []string
	collectSubtreeNodeIDs(targetNodeID, childrenMap, &subtreeNodeIDs)

	return subtreeNodeIDs, nil
}

// collectSubtreeNodeIDs 递归收集节点及其子树的所有节点ID
func collectSubtreeNodeIDs(nodeID string, childrenMap map[string][]string, subtreeNodeIDs *[]string) {
	// 添加当前节点ID
	*subtreeNodeIDs = append(*subtreeNodeIDs, nodeID)

	// 递归处理子节点
	if children, hasChildren := childrenMap[nodeID]; hasChildren {
		for _, childID := range children {
			collectSubtreeNodeIDs(childID, childrenMap, subtreeNodeIDs)
		}
	}
}

// CleanupAllTempFiles 清理指定目录下的临时文件（.tmp文件和JSON缓存文件）
func CleanupAllTempFiles(tempDir string) error {
	fmt.Printf("开始清理目录 %s 中的临时文件\n", tempDir)

	// 检查目录是否存在
	if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		fmt.Printf("目录不存在，跳过清理: %s\n", tempDir)
		return nil
	}

	deletedCount := 0
	var errors []string

	// 清理 .tmp 文件（所有目录）
	tmpPattern := filepath.Join(tempDir, "*.tmp")
	tmpFiles, err := filepath.Glob(tmpPattern)
	if err != nil {
		return fmt.Errorf("查找临时文件失败: %v", err)
	}

	// 删除找到的临时文件
	for _, tmpFile := range tmpFiles {
		fmt.Printf("删除临时文件: %s\n", tmpFile)
		if err := os.Remove(tmpFile); err != nil {
			errMsg := fmt.Sprintf("删除临时文件失败: %s, 错误: %v", tmpFile, err)
			fmt.Println(errMsg)
			errors = append(errors, errMsg)
		} else {
			deletedCount++
		}
	}

	// 如果是 documents 目录，额外清理 JSON 缓存文件
	if strings.Contains(tempDir, "documents") {
		jsonPattern := filepath.Join(tempDir, "*.json")
		jsonFiles, err := filepath.Glob(jsonPattern)
		if err != nil {
			return fmt.Errorf("查找JSON缓存文件失败: %v", err)
		}

		// 删除找到的JSON缓存文件
		for _, jsonFile := range jsonFiles {
			fmt.Printf("删除JSON缓存文件: %s\n", jsonFile)
			if err := os.Remove(jsonFile); err != nil {
				errMsg := fmt.Sprintf("删除JSON缓存文件失败: %s, 错误: %v", jsonFile, err)
				fmt.Println(errMsg)
				errors = append(errors, errMsg)
			} else {
				deletedCount++
			}
		}
	}

	fmt.Printf("清理完成，删除了 %d 个文件\n", deletedCount)

	// 如果有错误，返回合并的错误信息
	if len(errors) > 0 {
		return fmt.Errorf("部分文件清理失败: %s", strings.Join(errors, "; "))
	}

	return nil
}

// CleanupProjectTempFiles 清理特定项目的临时文件
func CleanupProjectTempFiles(fileKey, rootNodeID string) error {
	fmt.Printf("开始清理项目 %s (根节点: %s) 的临时文件\n", fileKey, rootNodeID)

	// 需要清理的目录列表
	cacheDirs := []string{
		filepath.Join("temp", fileKey, "previews"),
		filepath.Join("temp", fileKey, "images"),
		filepath.Join("temp", fileKey, "fpreviews"),
		filepath.Join("temp", fileKey, "documents"),
	}

	totalDeleted := 0
	var allErrors []string

	for _, tempDir := range cacheDirs {
		fmt.Printf("检查目录: %s\n", tempDir)

		// 检查目录是否存在
		if _, err := os.Stat(tempDir); os.IsNotExist(err) {
			fmt.Printf("目录不存在，跳过: %s\n", tempDir)
			continue
		}

		// 清理该目录下的临时文件
		if err := CleanupAllTempFiles(tempDir); err != nil {
			errMsg := fmt.Sprintf("清理目录 %s 失败: %v", tempDir, err)
			fmt.Println(errMsg)
			allErrors = append(allErrors, errMsg)
		} else {
			// 统计删除的文件数
			pattern := filepath.Join(tempDir, "*.tmp")
			tmpFiles, _ := filepath.Glob(pattern)
			totalDeleted += len(tmpFiles)
		}
	}

	fmt.Printf("项目 %s 临时文件清理完成，共处理 %d 个文件\n", fileKey, totalDeleted)

	// 如果有错误，返回合并的错误信息
	if len(allErrors) > 0 {
		return fmt.Errorf("部分目录清理失败: %s", strings.Join(allErrors, "; "))
	}

	return nil
}
