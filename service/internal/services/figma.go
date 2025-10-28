package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/figma-bridge/internal/models"
)

// FigmaService Figma API服务
type FigmaService struct {
	Token string
}

// FigmaNodeResponse Figma节点API响应
type FigmaNodeResponse struct {
	Name     string                 `json:"name"`
	Document map[string]interface{} `json:"document"`
	Error    bool                   `json:"err,omitempty"`
	Message  string                 `json:"message,omitempty"`
}

// FigmaImageResponse Figma图片API响应
type FigmaImageResponse struct {
	Err     bool                       `json:"err,omitempty"`
	Message string                     `json:"message,omitempty"`
	Images  map[string]string          `json:"images,omitempty"`
	Meta    map[string]json.RawMessage `json:"meta,omitempty"`
}

// NewFigmaService 创建Figma服务实例
func NewFigmaService(token string) *FigmaService {
	return &FigmaService{
		Token: token,
	}
}

// ParseFigmaLink 解析Figma链接
func ParseFigmaLink(link string) (string, string, error) {
	// 支持的格式:
	// https://www.figma.com/file/{fileKey}/{title}?node-id={nodeId}
	// https://www.figma.com/proto/{fileKey}/{title}?node-id={nodeId}

	if !strings.Contains(link, "figma.com") {
		return "", "", errors.New("无效的Figma链接")
	}

	fileKeyStart := strings.Index(link, "/file/")
	if fileKeyStart == -1 {
		fileKeyStart = strings.Index(link, "/proto/")
		if fileKeyStart == -1 {
			return "", "", errors.New("无法解析Figma链接中的fileKey")
		}
		fileKeyStart += 7 // "/proto/" 长度
	} else {
		fileKeyStart += 6 // "/file/" 长度
	}

	fileKeyEnd := strings.Index(link[fileKeyStart:], "/")
	if fileKeyEnd == -1 {
		return "", "", errors.New("无法解析Figma链接中的fileKey")
	}

	fileKey := link[fileKeyStart : fileKeyStart+fileKeyEnd]

	nodeIdParam := "node-id="
	nodeIdStart := strings.Index(link, nodeIdParam)
	if nodeIdStart == -1 {
		return fileKey, "", nil // 没有指定节点ID
	}

	nodeIdStart += len(nodeIdParam)
	nodeIdEnd := strings.Index(link[nodeIdStart:], "&")

	var nodeId string
	if nodeIdEnd == -1 {
		nodeId = link[nodeIdStart:]
	} else {
		nodeId = link[nodeIdStart : nodeIdStart+nodeIdEnd]
	}

	// 将节点ID中的"-"替换为":"
	nodeId = strings.ReplaceAll(nodeId, "-", ":")

	return fileKey, nodeId, nil
}

// GetFigmaNode 获取Figma节点数据
func (s *FigmaService) GetFigmaNode(fileKey, nodeId string) (*FigmaNodeResponse, error) {
	url := fmt.Sprintf("https://api.figma.com/v1/files/%s", fileKey)
	if nodeId != "" {
		url = fmt.Sprintf("%s/nodes?ids=%s", url, nodeId)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Figma-Token", s.Token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Figma API错误: %s", resp.Status)
	}

	var response FigmaNodeResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return nil, err
	}

	if response.Error {
		return nil, fmt.Errorf("Figma API错误: %s", response.Message)
	}

	return &response, nil
}

// GetFigmaImages 获取Figma节点图片
func (s *FigmaService) GetFigmaImages(fileKey string, nodeIds []string, format string, scale float64) (*FigmaImageResponse, error) {
	if format == "" {
		format = "png"
	}

	if scale <= 0 {
		scale = 1.0
	}

	nodeIdsStr := strings.Join(nodeIds, ",")
	url := fmt.Sprintf("https://api.figma.com/v1/images/%s?ids=%s&format=%s&scale=%f", fileKey, nodeIdsStr, format, scale)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Figma-Token", s.Token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Figma API错误: %s", resp.Status)
	}

	var response FigmaImageResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return nil, err
	}

	if response.Err {
		return nil, fmt.Errorf("Figma API错误: %s", response.Message)
	}

	return &response, nil
}

// DownloadImage 下载图片
func (s *FigmaService) DownloadImage(imageURL, outputPath string) error {
	// 创建目录
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 下载图片
	resp, err := http.Get(imageURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载图片错误: %s", resp.Status)
	}

	// 创建输出文件
	out, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer out.Close()

	// 复制内容
	_, err = io.Copy(out, resp.Body)
	return err
}

// ProcessNode 处理节点，提取节点信息并保存到数据库
func (s *FigmaService) ProcessNode(projectID uint, node map[string]interface{}, parentID *string) (*models.FigmaNode, error) {
	nodeID, ok := node["id"].(string)
	if !ok {
		return nil, errors.New("无效的节点ID")
	}

	name, _ := node["name"].(string)
	nodeType, _ := node["type"].(string)

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

	// 处理子节点
	children, ok := node["children"].([]interface{})
	if ok {
		for _, child := range children {
			childNode, ok := child.(map[string]interface{})
			if ok {
				_, err := s.ProcessNode(projectID, childNode, &nodeID)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	return figmaNode, nil
}

// ExportDesign 导出设计
func (s *FigmaService) ExportDesign(job *models.ExportJob, outputDir string) error {
	// 获取项目信息
	var project models.FigmaProject
	result := models.DB.First(&project, job.ProjectID)
	if result.Error != nil {
		return result.Error
	}

	// 更新任务状态
	models.UpdateExportJobStatus(job.ID, "processing", 10, "", "")

	// 获取项目的所有节点
	var nodes []models.FigmaNode
	result = models.DB.Where("project_id = ?", project.ID).Find(&nodes)
	if result.Error != nil {
		return result.Error
	}

	// 解析所有节点的修改信息
	nodeSettings := make(map[string]map[string]interface{})
	for _, node := range nodes {
		// 解析Modifys字段
		var settings map[string]interface{}
		if err := json.Unmarshal([]byte(node.Modifys), &settings); err != nil {
			// 如果解析失败，创建一个空的设置
			settings = make(map[string]interface{})
		}
		nodeSettings[node.NodeID] = settings
	}

	// 创建输出目录
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	// 创建图片目录
	imagesDir := filepath.Join(outputDir, "images")
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		return err
	}

	// 更新任务状态
	models.UpdateExportJobStatus(job.ID, "processing", 30, "", "")

	// 获取需要下载的节点ID列表
	var nodeIDs []string
	for _, node := range nodes {
		setting, exists := nodeSettings[node.NodeID]
		if exists && setting.IsVisible {
			nodeIDs = append(nodeIDs, node.NodeID)
		}
	}

	// 批量获取图片URL
	imageResponse, err := s.GetFigmaImages(project.FileKey, nodeIDs, "png", 1.0)
	if err != nil {
		models.UpdateExportJobStatus(job.ID, "failed", 0, "", err.Error())
		return err
	}

	// 更新任务状态
	models.UpdateExportJobStatus(job.ID, "processing", 50, "", "")

	// 下载图片
	totalImages := len(imageResponse.Images)
	downloadedImages := 0

	for nodeID, imageURL := range imageResponse.Images {
		// 获取节点设置
		setting, exists := nodeSettings[nodeID]
		if !exists || !setting.IsVisible {
			continue
		}

		// 确定图片文件名
		var fileName string
		// 获取节点设置
		settings, exists := nodeSettings[nodeID]
		if exists {
			// 尝试获取自定义名称
			if customName, ok := settings["customName"].(string); ok && customName != "" {
				fileName = customName
			} else if name, ok := settings["name"].(string); ok {
				// 使用节点名称
				fileName = name
			}
		}

		// 确保文件名有效
		fileName = strings.ReplaceAll(fileName, " ", "_")
		fileName = strings.ReplaceAll(fileName, "/", "_")
		fileName = strings.ReplaceAll(fileName, "\\", "_")
		fileName = strings.ReplaceAll(fileName, ":", "_")

		// 下载图片
		imagePath := filepath.Join(imagesDir, fileName+".png")
		err := s.DownloadImage(imageURL, imagePath)
		if err != nil {
			// 记录错误但继续处理其他图片
			fmt.Printf("下载图片错误 %s: %v\n", nodeID, err)
			continue
		}

		downloadedImages++
		progress := 50 + int(float64(downloadedImages)/float64(totalImages)*40)
		models.UpdateExportJobStatus(job.ID, "processing", progress, "", "")
	}

	// 生成元数据JSON文件
	metadata := map[string]interface{}{
		"project": map[string]interface{}{
			"id":      project.ID,
			"fileKey": project.FileKey,
			"name":    project.Name,
		},
		"nodes": make([]map[string]interface{}, 0),
	}

	for _, node := range nodes {
		settings, exists := nodeSettings[node.NodeID]
		if !exists {
			continue
		}

		// 创建节点数据，包含基本信息
		nodeData := map[string]interface{}{
			"id": node.NodeID,
		}

		// 将所有设置添加到节点数据中
		for key, value := range settings {
			nodeData[key] = value
		}

		// 处理excludedChildNodes字段
		if excludedNodesStr, ok := settings["excludedChildNodes"].(string); ok && excludedNodesStr != "" {
			var excludedNodes []string
			json.Unmarshal([]byte(excludedNodesStr), &excludedNodes)
			nodeData["excludedChildNodes"] = excludedNodes
		}

		metadata["nodes"] = append(metadata["nodes"].([]map[string]interface{}), nodeData)
	}

	// 写入元数据文件
	metadataBytes, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		models.UpdateExportJobStatus(job.ID, "failed", 0, "", err.Error())
		return err
	}

	metadataPath := filepath.Join(outputDir, "metadata.json")
	err = os.WriteFile(metadataPath, metadataBytes, 0644)
	if err != nil {
		models.UpdateExportJobStatus(job.ID, "failed", 0, "", err.Error())
		return err
	}

	// 创建ZIP文件
	zipPath := filepath.Join(os.TempDir(), fmt.Sprintf("figma-export-%d.zip", job.ID))
	err = createZipArchive(outputDir, zipPath)
	if err != nil {
		models.UpdateExportJobStatus(job.ID, "failed", 0, "", err.Error())
		return err
	}

	// 更新任务状态为完成
	models.UpdateExportJobStatus(job.ID, "completed", 100, zipPath, "")

	return nil
}

// createZipArchive 创建ZIP归档文件
func createZipArchive(sourceDir, targetFile string) error {
	// 这里实现ZIP压缩逻辑
	// 为简化示例，这里只是一个占位函数
	// 实际实现可以使用Go的archive/zip包或执行系统命令

	// 模拟压缩过程
	time.Sleep(2 * time.Second)

	// 创建一个空文件作为示例
	f, err := os.Create(targetFile)
	if err != nil {
		return err
	}
	defer f.Close()

	// 写入一些内容表示这是一个ZIP文件
	_, err = f.Write([]byte("ZIP ARCHIVE"))
	return err
}
