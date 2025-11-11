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
		figmaNode, err := CreateFigmaNode(projectID, node["id"].(string), node["name"].(string), node["type"].(string), node["parentId"])
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
		documentCopy["parentId"] = nil
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
			if node, ok := nodeData.(map[string]interface{})["document"].(map[string]interface{}); ok {
				// 添加当前节点 - 直接使用完整节点数据
				nodeCopy := make(map[string]interface{})
				for k, v := range node {
					nodeCopy[k] = v
				}
				nodeCopy["id"] = nodeID
				nodeCopy["parentId"] = nil // 根节点没有父节点
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
	nodeCopy["parentId"] = parentID
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
		"name":     name,
		"type":     nodeType,
		"parentId": parentID,
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
	// 使用默认参数
	return DownloadPreviewFigmaImageWithOptions(token, fileKey, nodeID, "png", 1.0)
}

// DownloadPreviewFigmaImageWithOptions 下载Figma图片（带选项）
func DownloadPreviewFigmaImageWithOptions(token, fileKey, nodeID, imageFormat string, imageScale float64) (string, error) {
	// 打印调试信息
	fmt.Printf("开始下载Figma图片: fileKey=%s, nodeID=%s, format=%s, scale=%.1f\n",
		fileKey, nodeID, imageFormat, imageScale)

	// 首先检查是否已经有该节点的图片文件
	// 创建临时文件夹
	tempDir := filepath.Join("temp", fileKey, "previews")
	err := os.MkdirAll(tempDir, os.ModePerm)
	if err != nil {
		fmt.Printf("创建临时文件夹失败: %v\n", err)
		return "", fmt.Errorf("创建临时文件夹失败: %v", err)
	}

	// 生成文件名 - 替换特殊字符为下划线
	safeNodeID := strings.NewReplacer(":", "_", ";", "_", "/", "_", "\\", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_").Replace(nodeID)

	// 如果有，直接使用最新的文件，避免重复下载
	existingFile := findLatestNodeImage(tempDir, safeNodeID)
	if existingFile != "" {
		// 不再检查过期时间，直接使用缓存的图片文件
		fmt.Printf("找到节点 %s 的缓存图片文件，直接使用: %s\n", nodeID, existingFile)
		return existingFile, nil
	}

	// 第一步：通过Figma API获取图片URL
	// 构建API URL
	apiUrl := fmt.Sprintf("https://api.figma.com/v1/images/%s?ids=%s&format=%s&scale=%.1f&contentOnly=true",
		fileKey, nodeID, imageFormat, imageScale)
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

	imageURL, ok := images[nodeID].(string)
	if !ok || imageURL == "" {
		fmt.Printf("获取图片URL失败，nodeID=%s 不存在或为空\n", nodeID)
		return "", fmt.Errorf("获取图片URL失败，节点ID=%s 不存在或URL为空", nodeID)
	}
	fmt.Printf("获取到图片URL: %s\n", imageURL)

	// 下载图片
	return downloadPreviewImageFile(imageURL, fileKey, nodeID)
}

// 查找节点的最新图片文件
func findLatestNodeImage(tempDir, safeNodeID string) string {
	// 查找同一节点的所有图片文件（支持多种格式，不包括临时文件）
	supportedExts := []string{".png", ".svg", ".jpg", ".pdf"}
	var allFiles []string

	for _, ext := range supportedExts {
		pattern := filepath.Join(tempDir, safeNodeID+"-*"+ext)
		files, err := filepath.Glob(pattern)
		if err == nil && len(files) > 0 {
			allFiles = append(allFiles, files...)
		}
	}

	if len(allFiles) == 0 {
		return ""
	}

	// 过滤掉临时文件
	var validFiles []string
	for _, f := range allFiles {
		// 检查是否是临时文件（包含-temp.扩展名）
		if !strings.Contains(f, "-temp.") {
			validFiles = append(validFiles, f)
		}
	}

	if len(validFiles) == 0 {
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
		return fileInfos[0].path
	}

	return ""
}

// 清理节点的旧图片文件
func cleanupOldNodeImages(tempDir, safeNodeID string, keepCount int) {
	// 查找同一节点的所有图片文件（支持多种格式，不包括临时文件）
	supportedExts := []string{".png", ".svg", ".jpg", ".pdf"}
	var allFiles []string

	for _, ext := range supportedExts {
		pattern := filepath.Join(tempDir, safeNodeID+"-*"+ext)
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
	// 需要清理的目录列表
	cacheDirs := []string{
		filepath.Join("temp", fileKey, "images"),
		filepath.Join("temp", fileKey, "previews"),
		filepath.Join("temp", fileKey, "fpreviews"),
	}

	totalFiles := 0
	var allErrors []string

	for _, tempDir := range cacheDirs {
		fmt.Printf("检查缓存目录: %s\n", tempDir)

		// 检查目录是否存在
		if _, err := os.Stat(tempDir); os.IsNotExist(err) {
			fmt.Printf("目录不存在，跳过: %s\n", tempDir)
			continue
		}

		// 删除所有图片文件
		pattern := filepath.Join(tempDir, "*.png")
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
			}
		}

		totalFiles += len(files)
		fmt.Printf("已清除目录 %s 中的 %d 个文件\n", tempDir, len(files))
	}

	fmt.Printf("已清除项目 %s 的所有缓存图片，共 %d 个文件\n", fileKey, totalFiles)

	// 如果有错误，返回合并的错误信息
	if len(allErrors) > 0 {
		return fmt.Errorf("清理过程中发生错误: %s", strings.Join(allErrors, "; "))
	}

	return nil
}

// DownloadFilteredPreviewFigmaImage 下载过滤后的Figma预览图片（使用过滤后的节点ID列表）
func DownloadFilteredPreviewFigmaImage(token, fileKey, nodeID, imageFormat string, imageScale float64, projectID uint) (string, error) {
	// 打印调试信息
	fmt.Printf("开始下载过滤后的Figma预览图片: fileKey=%s, nodeID=%s, format=%s, scale=%.1f, projectID=%d\n",
		fileKey, nodeID, imageFormat, imageScale, projectID)

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

	// 检查是否有现有文件
	existingFile := findLatestNodeImage(tempDir, safeFileName)
	if existingFile != "" {
		fmt.Printf("找到过滤预览图片缓存文件，直接使用: %s\n", existingFile)
		return existingFile, nil
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
	return downloadFilteredPreviewImageFileByNodeIDs(imageURL, fileKey, safeFileName, includedNodeIDs)
}

// downloadPreviewImageFile 下载图片文件
func downloadPreviewImageFile(url, fileKey, nodeID string) (string, error) {
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

	// 清理同一节点的过多旧文件
	cleanupOldNodeImages(tempDir, safeNodeID, 10)

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

	// 使用哈希值构建最终文件名
	fileName := fmt.Sprintf("%s-%s.png", safeNodeID, hash)
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
func downloadFilteredPreviewImageFileByNodeIDs(url, fileKey, safeFileName string, includedNodeIDs []string) (string, error) {
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

	// 清理同一类型的过多旧文件
	cleanupOldNodeImages(tempDir, safeFileName, 10)

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

	// 使用哈希值和正确的扩展名构建最终文件名
	fileName := fmt.Sprintf("%s-%s%s", safeFileName, hash, finalFileExt)
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
		filePath, err := downloadSingleFilteredPreviewImage(imageURL, fileKey, nodeID, fileName, tempDir)
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
func downloadSingleFilteredPreviewImage(imageURL, _ /*fileKey*/, nodeID, fileName, tempDir string) (string, error) {
	fmt.Printf("开始下载单个图片: nodeID=%s, fileName=%s, url=%s\n", nodeID, fileName, imageURL)

	// 检查是否已经有缓存的图片文件
	existingFile := findLatestNodeImage(tempDir, fileName)
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

	// 使用哈希值和正确的扩展名构建最终文件名
	finalFileName := fmt.Sprintf("%s-%s%s", fileName, hash, resultFileExt)
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

	// 清理同一节点的过多旧文件
	cleanupOldNodeImages(tempDir, fileName, 10)

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
		if parentID, ok := node["parentId"]; ok && parentID != nil {
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
