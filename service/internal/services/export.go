package services

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/figma-bridge/internal/models"
)

// ProcessExportJob 处理导出任务
func ProcessExportJob(jobID uint) {
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

	// 获取项目节点
	var nodes []models.FigmaNode
	models.DB.Where("project_id = ?", project.ID).Find(&nodes)
	if len(nodes) == 0 {
		models.UpdateExportJobStatus(jobID, "failed", 0, "", "项目没有节点数据")
		return
	}

	// 创建临时目录
	tempDir := filepath.Join("temp", fmt.Sprintf("export_%d_%d", project.ID, time.Now().Unix()))
	err = os.MkdirAll(tempDir, os.ModePerm)
	if err != nil {
		models.UpdateExportJobStatus(jobID, "failed", 0, "", "创建临时目录失败: "+err.Error())
		return
	}
	defer os.RemoveAll(tempDir) // 任务完成后清理临时目录

	// 更新进度
	models.UpdateExportJobStatus(jobID, "processing", 20, job.FilePath, "")

	// 下载所有节点图片
	nodeImages := make(map[string]string)
	nodeSettings := make(map[string]map[string]interface{})

	// 解析所有节点的修改信息
	for _, node := range nodes {
		var settings map[string]interface{}
		if err := json.Unmarshal([]byte(node.Modifys), &settings); err == nil {
			nodeSettings[node.NodeID] = settings
		}
	}

	// 筛选可见节点
	var visibleNodes []models.FigmaNode
	for _, node := range nodes {
		setting, exists := nodeSettings[node.NodeID]
		if !exists || setting["isVisible"] != false { // 默认可见或明确设置为可见
			visibleNodes = append(visibleNodes, node)
		}
	}

	// 下载图片
	totalNodes := len(visibleNodes)
	for i, node := range visibleNodes {
		// 获取节点设置
		setting := nodeSettings[node.NodeID]

		// 获取自定义名称
		var fileName string
		if customName, ok := setting["customName"].(string); ok && customName != "" {
			fileName = customName
		} else {
			fileName = node.NodeID
		}

		// 获取图片格式
		format := job.Format // 使用任务中指定的默认格式
		// 如果节点有自定义格式，优先使用节点的自定义格式
		if downloadType, ok := setting["imageDownloadType"].(string); ok && downloadType != "" {
			format = downloadType
		}

		// 下载图片
		imagePath, err := DownloadFigmaImage(user.FigmaToken, project.FileKey, node.NodeID)
		if err == nil {
			// 复制到临时目录
			destPath := filepath.Join(tempDir, fileName+"."+format)
			copyFile(imagePath, destPath)
			nodeImages[node.NodeID] = fileName + "." + format
		}

		// 更新进度
		progress := 20 + int(float64(i+1)/float64(totalNodes)*60)
		models.UpdateExportJobStatus(jobID, "processing", progress, job.FilePath, "")
	}

	// 创建元数据文件
	metadata := map[string]interface{}{
		"project":     project,
		"nodes":       visibleNodes,
		"images":      nodeImages,
		"settings":    nodeSettings,
		"exported_at": time.Now(),
	}

	metadataJSON, _ := json.MarshalIndent(metadata, "", "  ")
	metadataPath := filepath.Join(tempDir, "metadata.json")
	err = os.WriteFile(metadataPath, metadataJSON, 0644)
	if err != nil {
		models.UpdateExportJobStatus(jobID, "failed", 0, "", "创建元数据文件失败: "+err.Error())
		return
	}

	// 更新进度
	models.UpdateExportJobStatus(jobID, "processing", 90, job.FilePath, "")

	// 创建导出目录 {userid}/temp/exports
	exportDir := filepath.Join("temp", "exports", fmt.Sprintf("%d", user.ID))
	err = os.MkdirAll(exportDir, os.ModePerm)
	if err != nil {
		models.UpdateExportJobStatus(jobID, "failed", 0, "", "创建导出目录失败: "+err.Error())
		return
	}

	// 创建ZIP文件名，使用file_key和节点id的组合（排除特殊字符）
	safeFileKey := strings.NewReplacer(":", "_", ";", "_", "/", "_", "\\", "_", "?", "_", "*", "_", "\"", "_", "<", "_", ">", "_", "|", "_").Replace(project.FileKey)
	safeNodeID := strings.NewReplacer(":", "_", ";", "_", "/", "_", "\\", "_", "?", "_", "*", "_", "\"", "_", "<", "_", ">", "_", "|", "_").Replace(project.RootNodeID)
	zipPath := filepath.Join(exportDir, fmt.Sprintf("%s_%s.zip", safeFileKey, safeNodeID))

	err = createZipArchiveFromDir(tempDir, zipPath)
	if err != nil {
		models.UpdateExportJobStatus(jobID, "failed", 0, "", "创建ZIP文件失败: "+err.Error())
		return
	}

	// 更新任务状态为完成
	models.UpdateExportJobStatus(jobID, "completed", 100, zipPath, "")
}

// 复制文件
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	return nil
}

// createZipArchiveFromDir 创建ZIP归档
func createZipArchiveFromDir(sourceDir, zipPath string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	archive := zip.NewWriter(zipFile)
	defer archive.Close()

	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过目录本身
		if path == sourceDir {
			return nil
		}

		// 创建ZIP头
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		// 设置相对路径
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		header.Name = relPath

		// 设置压缩方法
		header.Method = zip.Deflate

		// 创建文件
		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		// 如果是目录，不需要复制内容
		if info.IsDir() {
			return nil
		}

		// 复制文件内容
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})

	return err
}
