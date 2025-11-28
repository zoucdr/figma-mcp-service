package services

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/figma-deliver/huaweicloud/obs"
)

// OBSService 华为云OBS服务
type OBSService struct {
	client            *obs.ObsClient
	bucketName        string
	region            string
	endpoint          string
	pathPrefix        string
	uploadConcurrency int
	uploadTimeout     time.Duration
	enabled           bool
}

// OBSConfig OBS配置
type OBSConfig struct {
	Enabled              bool
	Endpoint             string
	AccessKeyID          string
	SecretAccessKey      string
	BucketName           string
	Region               string
	PathPrefix           string
	UploadConcurrency    int
	UploadTimeoutSeconds int
}

// NewOBSService 创建OBS服务
func NewOBSService(config OBSConfig) (*OBSService, error) {
	if !config.Enabled {
		log.Printf("📦 [OBS] OBS服务未启用")
		return &OBSService{enabled: false}, nil
	}

	// 初始化OBS客户端
	client, err := obs.New(config.AccessKeyID, config.SecretAccessKey, config.Endpoint)
	if err != nil {
		log.Printf("❌ [OBS] 初始化失败: %v", err)
		return nil, fmt.Errorf("初始化OBS客户端失败: %w", err)
	}

	service := &OBSService{
		client:            client,
		bucketName:        config.BucketName,
		region:            config.Region,
		endpoint:          config.Endpoint,
		pathPrefix:        config.PathPrefix,
		uploadConcurrency: config.UploadConcurrency,
		uploadTimeout:     time.Duration(config.UploadTimeoutSeconds) * time.Second,
		enabled:           true,
	}

	log.Printf("✅ [OBS] 初始化成功")
	log.Printf("   - Endpoint: %s", config.Endpoint)
	log.Printf("   - Bucket: %s", config.BucketName)
	log.Printf("   - Region: %s", config.Region)
	log.Printf("   - PathPrefix: %s", config.PathPrefix)

	return service, nil
}

// IsEnabled 检查OBS服务是否启用
func (s *OBSService) IsEnabled() bool {
	return s.enabled
}

// Close 关闭OBS客户端
func (s *OBSService) Close() {
	if s.client != nil {
		s.client.Close()
		log.Printf("🔒 [OBS] 客户端已关闭")
	}
}

// ===================== 上传功能 =====================

// UploadImageFromURL 从URL下载图片并上传到OBS
// 返回: (OBS URL, OBS Key, 文件大小, 错误)
func (s *OBSService) UploadImageFromURL(imageURL, fileKey, nodeID, format string, scale float64) (string, string, int64, error) {
	if !s.enabled {
		return "", "", 0, fmt.Errorf("OBS服务未启用")
	}

	// 下载图片
	log.Printf("📥 [OBS] 开始下载图片: %s", imageURL)
	imageData, contentType, err := s.downloadImage(imageURL)
	if err != nil {
		log.Printf("❌ [OBS] 下载图片失败: %v", err)
		return "", "", 0, fmt.Errorf("下载图片失败: %w", err)
	}

	fileSize := int64(len(imageData))
	log.Printf("✅ [OBS] 图片下载成功，大小: %d bytes", fileSize)

	// 生成OBS存储路径
	obsKey := s.generateOBSKey(fileKey, nodeID, format, scale)

	// 上传到OBS
	err = s.uploadToOBS(obsKey, imageData, contentType)
	if err != nil {
		log.Printf("❌ [OBS] 上传失败: %v", err)
		return "", "", 0, fmt.Errorf("上传到OBS失败: %w", err)
	}

	// 生成访问URL
	obsURL := s.generateOBSURL(obsKey)

	log.Printf("✅ [OBS] 上传成功")
	log.Printf("   - OBS Key: %s", obsKey)
	log.Printf("   - OBS URL: %s", obsURL)
	log.Printf("   - 文件大小: %d bytes", fileSize)

	return obsURL, obsKey, fileSize, nil
}

// UploadImageFromBytes 从字节数组上传图片到OBS
func (s *OBSService) UploadImageFromBytes(imageData []byte, fileKey, nodeID, format string, scale float64, contentType string) (string, string, int64, error) {
	if !s.enabled {
		return "", "", 0, fmt.Errorf("OBS服务未启用")
	}

	fileSize := int64(len(imageData))

	// 生成OBS存储路径
	obsKey := s.generateOBSKey(fileKey, nodeID, format, scale)

	// 上传到OBS
	err := s.uploadToOBS(obsKey, imageData, contentType)
	if err != nil {
		log.Printf("❌ [OBS] 上传失败: %v", err)
		return "", "", 0, fmt.Errorf("上传到OBS失败: %w", err)
	}

	// 生成访问URL
	obsURL := s.generateOBSURL(obsKey)

	log.Printf("✅ [OBS] 上传成功: %s (%d bytes)", obsKey, fileSize)

	return obsURL, obsKey, fileSize, nil
}

// downloadImage 从URL下载图片
func (s *OBSService) downloadImage(imageURL string) ([]byte, string, error) {
	client := &http.Client{
		Timeout: s.uploadTimeout,
	}

	resp, err := client.Get(imageURL)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP状态码: %d", resp.StatusCode)
	}

	// 读取图片数据
	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/png" // 默认值
	}

	return imageData, contentType, nil
}

// uploadToOBS 上传数据到OBS
func (s *OBSService) uploadToOBS(obsKey string, data []byte, contentType string) error {
	input := &obs.PutObjectInput{}
	input.Bucket = s.bucketName
	input.Key = obsKey
	input.Body = bytes.NewReader(data)
	input.ContentType = contentType

	// 设置ACL为公开读（根据需求调整）
	// input.ACL = obs.AclPublicRead

	output, err := s.client.PutObject(input)
	if err != nil {
		return err
	}

	if output.StatusCode != 200 {
		return fmt.Errorf("上传失败，状态码: %d", output.StatusCode)
	}

	return nil
}

// ===================== 下载功能 =====================

// DownloadImage 从OBS下载图片
func (s *OBSService) DownloadImage(obsKey string) ([]byte, string, error) {
	if !s.enabled {
		return nil, "", fmt.Errorf("OBS服务未启用")
	}

	input := &obs.GetObjectInput{}
	input.Bucket = s.bucketName
	input.Key = obsKey

	output, err := s.client.GetObject(input)
	if err != nil {
		log.Printf("❌ [OBS] 下载失败: %v", err)
		return nil, "", fmt.Errorf("从OBS下载失败: %w", err)
	}
	defer output.Body.Close()

	if output.StatusCode != 200 {
		return nil, "", fmt.Errorf("下载失败，状态码: %d", output.StatusCode)
	}

	// 读取数据
	data, err := io.ReadAll(output.Body)
	if err != nil {
		return nil, "", err
	}

	contentType := output.ContentType
	if contentType == "" {
		contentType = "image/png"
	}

	log.Printf("✅ [OBS] 下载成功: %s (%d bytes)", obsKey, len(data))

	return data, contentType, nil
}

// ===================== URL生成 =====================

// GenerateSignedURL 生成带签名的临时访问URL（有效期1小时）
func (s *OBSService) GenerateSignedURL(obsKey string, expiresInSeconds int) (string, error) {
	if !s.enabled {
		return "", fmt.Errorf("OBS服务未启用")
	}

	input := &obs.CreateSignedUrlInput{}
	input.Method = obs.HttpMethodGet
	input.Bucket = s.bucketName
	input.Key = obsKey
	input.Expires = expiresInSeconds // 秒

	output, err := s.client.CreateSignedUrl(input)
	if err != nil {
		log.Printf("❌ [OBS] 生成签名URL失败: %v", err)
		return "", fmt.Errorf("生成签名URL失败: %w", err)
	}

	log.Printf("🔗 [OBS] 生成签名URL: %s (有效期: %d 秒)", output.SignedUrl, expiresInSeconds)

	return output.SignedUrl, nil
}

// generateOBSURL 生成OBS公开访问URL
func (s *OBSService) generateOBSURL(obsKey string) string {
	// 格式: https://{bucket}.{endpoint}/{key}
	// 或者: https://{endpoint}/{bucket}/{key}

	// 使用endpoint直接访问（推荐）
	return fmt.Sprintf("https://%s/%s/%s", s.endpoint, s.bucketName, obsKey)
}

// GenerateOBSURL 根据 OBS Key 生成访问 URL（公开方法）
func (s *OBSService) GenerateOBSURL(obsKey string) string {
	if !s.enabled || obsKey == "" {
		return ""
	}
	return s.generateOBSURL(obsKey)
}

// ===================== 辅助方法 =====================

// generateOBSKey 生成OBS存储Key
// 格式: {prefix}/{file_key}/{format}_{scale}x/{node_id}.{ext}
// 例如: figma_images/abc123/png_2.0x/1-123.png
func (s *OBSService) generateOBSKey(fileKey, nodeID, format string, scale float64) string {
	// 处理节点ID中的特殊字符（冒号替换为连字符）
	safeNodeID := strings.ReplaceAll(nodeID, ":", "-")

	// 确定文件扩展名
	ext := format
	if format == "jpg" {
		ext = "jpeg"
	}

	// 构建路径
	// {prefix}/{file_key}/{format}_{scale}x/{node_id}.{ext}
	key := filepath.Join(
		s.pathPrefix,
		fileKey,
		fmt.Sprintf("%s_%.1fx", format, scale),
		fmt.Sprintf("%s.%s", safeNodeID, ext),
	)

	// Windows路径分隔符转换为Unix风格
	key = filepath.ToSlash(key)

	return key
}

// DeleteObject 删除OBS对象
func (s *OBSService) DeleteObject(obsKey string) error {
	if !s.enabled {
		return fmt.Errorf("OBS服务未启用")
	}

	input := &obs.DeleteObjectInput{}
	input.Bucket = s.bucketName
	input.Key = obsKey

	output, err := s.client.DeleteObject(input)
	if err != nil {
		log.Printf("❌ [OBS] 删除失败: %v", err)
		return fmt.Errorf("删除OBS对象失败: %w", err)
	}

	if output.StatusCode != 204 && output.StatusCode != 200 {
		return fmt.Errorf("删除失败，状态码: %d", output.StatusCode)
	}

	log.Printf("🗑️ [OBS] 删除成功: %s", obsKey)

	return nil
}

// CheckObjectExists 检查OBS对象是否存在
func (s *OBSService) CheckObjectExists(obsKey string) (bool, error) {
	if !s.enabled {
		return false, fmt.Errorf("OBS服务未启用")
	}

	input := &obs.GetObjectMetadataInput{}
	input.Bucket = s.bucketName
	input.Key = obsKey

	output, err := s.client.GetObjectMetadata(input)
	if err != nil {
		// 如果是404错误，表示对象不存在
		if obsError, ok := err.(obs.ObsError); ok {
			if obsError.StatusCode == 404 {
				return false, nil
			}
		}
		return false, err
	}

	return output.StatusCode == 200, nil
}

// GetObjectSize 获取OBS对象大小
func (s *OBSService) GetObjectSize(obsKey string) (int64, error) {
	if !s.enabled {
		return 0, fmt.Errorf("OBS服务未启用")
	}

	input := &obs.GetObjectMetadataInput{}
	input.Bucket = s.bucketName
	input.Key = obsKey

	output, err := s.client.GetObjectMetadata(input)
	if err != nil {
		return 0, err
	}

	return output.ContentLength, nil
}

// ===================== 批量操作 =====================

// BatchUploadImages 批量上传图片
type BatchUploadResult struct {
	NodeID   string
	OBSURL   string
	OBSKey   string
	FileSize int64
	Error    error
}

// BatchUploadImagesFromURLs 批量从URL上传图片到OBS
func (s *OBSService) BatchUploadImagesFromURLs(imageMap map[string]string, fileKey, format string, scale float64) []BatchUploadResult {
	if !s.enabled {
		results := make([]BatchUploadResult, 0, len(imageMap))
		for nodeID := range imageMap {
			results = append(results, BatchUploadResult{
				NodeID: nodeID,
				Error:  fmt.Errorf("OBS服务未启用"),
			})
		}
		return results
	}

	results := make([]BatchUploadResult, 0, len(imageMap))

	log.Printf("📦 [OBS] 开始批量上传，共 %d 个图片", len(imageMap))

	// 使用并发控制
	semaphore := make(chan struct{}, s.uploadConcurrency)
	resultChan := make(chan BatchUploadResult, len(imageMap))

	for nodeID, imageURL := range imageMap {
		go func(nid, url string) {
			semaphore <- struct{}{}        // 获取信号量
			defer func() { <-semaphore }() // 释放信号量

			obsURL, obsKey, fileSize, err := s.UploadImageFromURL(url, fileKey, nid, format, scale)
			resultChan <- BatchUploadResult{
				NodeID:   nid,
				OBSURL:   obsURL,
				OBSKey:   obsKey,
				FileSize: fileSize,
				Error:    err,
			}
		}(nodeID, imageURL)
	}

	// 收集结果
	for i := 0; i < len(imageMap); i++ {
		result := <-resultChan
		results = append(results, result)
	}

	// 统计
	successCount := 0
	for _, result := range results {
		if result.Error == nil {
			successCount++
		}
	}

	log.Printf("✅ [OBS] 批量上传完成: 成功 %d/%d", successCount, len(imageMap))

	return results
}

// ===================== 日志文件管理 =====================

// ObjectInfo 对象信息
type ObjectInfo struct {
	Key          string    `json:"key"`
	Name         string    `json:"name"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
	IsDir        bool      `json:"is_dir"`
}

// ListObjectsInPath 列出指定路径下的对象
func (s *OBSService) ListObjectsInPath(prefix string) ([]ObjectInfo, error) {
	if !s.enabled {
		return nil, fmt.Errorf("OBS服务未启用")
	}

	input := &obs.ListObjectsInput{}
	input.Bucket = s.bucketName
	input.Prefix = prefix
	input.Delimiter = "/" // 使用分隔符来模拟目录结构
	input.MaxKeys = 1000  // 每次最多返回1000个对象

	output, err := s.client.ListObjects(input)
	if err != nil {
		log.Printf("❌ [OBS] 列出对象失败: %v", err)
		return nil, fmt.Errorf("列出对象失败: %w", err)
	}

	var objects []ObjectInfo

	// 添加子目录（CommonPrefixes）
	for _, commonPrefix := range output.CommonPrefixes {
		// 移除前缀和末尾的斜杠，获取目录名
		dirName := strings.TrimPrefix(commonPrefix, prefix)
		dirName = strings.TrimSuffix(dirName, "/")

		objects = append(objects, ObjectInfo{
			Key:   commonPrefix,
			Name:  dirName,
			IsDir: true,
		})
	}

	// 添加文件对象
	for _, content := range output.Contents {
		// 跳过目录本身
		if content.Key == prefix {
			continue
		}

		// 获取文件名（移除前缀）
		fileName := strings.TrimPrefix(content.Key, prefix)

		objects = append(objects, ObjectInfo{
			Key:          content.Key,
			Name:         fileName,
			Size:         content.Size,
			LastModified: content.LastModified,
			IsDir:        false,
		})
	}

	log.Printf("✅ [OBS] 列出对象成功: %s (%d 个对象)", prefix, len(objects))

	return objects, nil
}

// ListObjectsInPathWithPagination 带分页的列出对象
func (s *OBSService) ListObjectsInPathWithPagination(prefix string, maxKeys int, marker string) ([]ObjectInfo, string, bool, error) {
	if !s.enabled {
		return nil, "", false, fmt.Errorf("OBS服务未启用")
	}

	// 默认每页50条
	if maxKeys <= 0 {
		maxKeys = 50
	}
	// 最大不超过1000
	if maxKeys > 1000 {
		maxKeys = 1000
	}

	input := &obs.ListObjectsInput{}
	input.Bucket = s.bucketName
	input.Prefix = prefix
	input.Delimiter = "/" // 使用分隔符来模拟目录结构
	input.MaxKeys = maxKeys
	if marker != "" {
		input.Marker = marker
	}

	output, err := s.client.ListObjects(input)
	if err != nil {
		log.Printf("❌ [OBS] 列出对象失败: %v", err)
		return nil, "", false, fmt.Errorf("列出对象失败: %w", err)
	}

	var objects []ObjectInfo

	// 添加子目录（CommonPrefixes）
	for _, commonPrefix := range output.CommonPrefixes {
		// 移除前缀和末尾的斜杠，获取目录名
		dirName := strings.TrimPrefix(commonPrefix, prefix)
		dirName = strings.TrimSuffix(dirName, "/")

		objects = append(objects, ObjectInfo{
			Key:   commonPrefix,
			Name:  dirName,
			IsDir: true,
		})
	}

	// 添加文件对象
	for _, content := range output.Contents {
		// 跳过目录本身
		if content.Key == prefix {
			continue
		}

		// 获取文件名（移除前缀）
		fileName := strings.TrimPrefix(content.Key, prefix)

		objects = append(objects, ObjectInfo{
			Key:          content.Key,
			Name:         fileName,
			Size:         content.Size,
			LastModified: content.LastModified,
			IsDir:        false,
		})
	}

	nextMarker := output.NextMarker
	isTruncated := output.IsTruncated

	log.Printf("✅ [OBS] 列出对象成功: %s (%d 个对象, 是否还有更多: %v)", prefix, len(objects), isTruncated)

	return objects, nextMarker, isTruncated, nil
}

// UploadLogFile 上传日志文件到OBS
func (s *OBSService) UploadLogFile(fileData []byte, relativePath, fileName string) (string, error) {
	if !s.enabled {
		return "", fmt.Errorf("OBS服务未启用")
	}

	// 构建完整的对象键：logs/{relativePath}/{fileName}
	var objectKey string
	if relativePath != "" && relativePath != "/" {
		objectKey = filepath.Join("logs", relativePath, fileName)
	} else {
		objectKey = filepath.Join("logs", fileName)
	}

	// Windows路径分隔符转换为Unix风格
	objectKey = filepath.ToSlash(objectKey)

	// 根据文件扩展名确定Content-Type
	contentType := "application/octet-stream"
	ext := strings.ToLower(filepath.Ext(fileName))
	switch ext {
	case ".txt", ".log":
		contentType = "text/plain; charset=utf-8"
	case ".json":
		contentType = "application/json; charset=utf-8"
	case ".xml":
		contentType = "application/xml; charset=utf-8"
	case ".html":
		contentType = "text/html; charset=utf-8"
	}

	// 上传到OBS
	input := &obs.PutObjectInput{}
	input.Bucket = s.bucketName
	input.Key = objectKey
	input.Body = bytes.NewReader(fileData)
	input.ContentType = contentType

	output, err := s.client.PutObject(input)
	if err != nil {
		log.Printf("❌ [OBS] 上传日志文件失败: %v", err)
		return "", fmt.Errorf("上传日志文件失败: %w", err)
	}

	if output.StatusCode != 200 {
		return "", fmt.Errorf("上传失败，状态码: %d", output.StatusCode)
	}

	log.Printf("✅ [OBS] 日志文件上传成功: %s (%d bytes)", objectKey, len(fileData))

	return objectKey, nil
}

// DownloadLogFile 下载日志文件
func (s *OBSService) DownloadLogFile(objectKey string) ([]byte, string, error) {
	if !s.enabled {
		return nil, "", fmt.Errorf("OBS服务未启用")
	}

	input := &obs.GetObjectInput{}
	input.Bucket = s.bucketName
	input.Key = objectKey

	output, err := s.client.GetObject(input)
	if err != nil {
		log.Printf("❌ [OBS] 下载日志文件失败: %v", err)
		return nil, "", fmt.Errorf("下载日志文件失败: %w", err)
	}
	defer output.Body.Close()

	if output.StatusCode != 200 {
		return nil, "", fmt.Errorf("下载失败，状态码: %d", output.StatusCode)
	}

	// 读取数据
	data, err := io.ReadAll(output.Body)
	if err != nil {
		return nil, "", err
	}

	log.Printf("✅ [OBS] 日志文件下载成功: %s (%d bytes)", objectKey, len(data))

	return data, output.ContentType, nil
}

// DeleteDirectory 删除目录下的所有对象（文件和子目录）
func (s *OBSService) DeleteDirectory(prefix string) (int, error) {
	if !s.enabled {
		return 0, fmt.Errorf("OBS服务未启用")
	}

	log.Printf("🗑️ [OBS] 开始删除目录: %s", prefix)

	// 列出所有对象（包括子目录中的文件）
	input := &obs.ListObjectsInput{}
	input.Bucket = s.bucketName
	input.Prefix = prefix
	input.MaxKeys = 1000 // 一次最多列出1000个

	var allKeys []string

	for {
		output, err := s.client.ListObjects(input)
		if err != nil {
			log.Printf("❌ [OBS] 列出对象失败: %v", err)
			return 0, fmt.Errorf("列出对象失败: %w", err)
		}

		// 收集所有对象的Key
		for _, content := range output.Contents {
			allKeys = append(allKeys, content.Key)
		}

		// 检查是否还有更多对象
		if !output.IsTruncated {
			break
		}

		// 设置下一次查询的起始位置
		input.Marker = output.NextMarker
	}

	if len(allKeys) == 0 {
		log.Printf("⚠️ [OBS] 目录为空，无需删除: %s", prefix)
		return 0, nil
	}

	log.Printf("📋 [OBS] 找到 %d 个对象需要删除", len(allKeys))

	// 批量删除对象
	deleteCount := 0
	batchSize := 1000 // OBS批量删除最多1000个

	for i := 0; i < len(allKeys); i += batchSize {
		end := i + batchSize
		if end > len(allKeys) {
			end = len(allKeys)
		}

		batch := allKeys[i:end]

		// 构造批量删除请求
		deleteInput := &obs.DeleteObjectsInput{}
		deleteInput.Bucket = s.bucketName

		objects := make([]obs.ObjectToDelete, len(batch))
		for j, key := range batch {
			objects[j] = obs.ObjectToDelete{Key: key}
		}
		deleteInput.Objects = objects

		// 执行批量删除
		deleteOutput, err := s.client.DeleteObjects(deleteInput)
		if err != nil {
			log.Printf("❌ [OBS] 批量删除失败: %v", err)
			return deleteCount, fmt.Errorf("批量删除失败: %w", err)
		}

		// 统计成功删除的数量
		deleteCount += len(deleteOutput.Deleteds)

		// 记录删除失败的对象
		for _, deleted := range deleteOutput.Errors {
			log.Printf("⚠️ [OBS] 删除失败: %s (错误: %s)", deleted.Key, deleted.Message)
		}
	}

	log.Printf("✅ [OBS] 目录删除完成: %s (删除了 %d 个对象)", prefix, deleteCount)

	return deleteCount, nil
}

// UploadSharedFile 上传文件到共享目录
func (s *OBSService) UploadSharedFile(fileData []byte, fileName string) (string, error) {
	if !s.enabled {
		return "", fmt.Errorf("OBS服务未启用")
	}

	// 构建完整的对象键：file_share/{fileName}
	objectKey := filepath.Join("file_share", fileName)
	objectKey = filepath.ToSlash(objectKey)

	// 根据文件扩展名确定Content-Type
	contentType := "application/octet-stream"
	ext := strings.ToLower(filepath.Ext(fileName))
	switch ext {
	case ".txt", ".log", ".md":
		contentType = "text/plain; charset=utf-8"
	case ".json":
		contentType = "application/json; charset=utf-8"
	case ".xml":
		contentType = "application/xml; charset=utf-8"
	case ".html":
		contentType = "text/html; charset=utf-8"
	case ".png":
		contentType = "image/png"
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".pdf":
		contentType = "application/pdf"
	case ".zip":
		contentType = "application/zip"
	}

	// 上传到OBS
	input := &obs.PutObjectInput{}
	input.Bucket = s.bucketName
	input.Key = objectKey
	input.Body = bytes.NewReader(fileData)
	input.ContentType = contentType

	output, err := s.client.PutObject(input)
	if err != nil {
		log.Printf("❌ [OBS] 上传共享文件失败: %v", err)
		return "", fmt.Errorf("上传共享文件失败: %w", err)
	}

	if output.StatusCode != 200 {
		return "", fmt.Errorf("上传失败，状态码: %d", output.StatusCode)
	}

	log.Printf("✅ [OBS] 共享文件上传成功: %s (%d bytes)", objectKey, len(fileData))

	return objectKey, nil
}

// DownloadFile 下载OBS文件（通用）
func (s *OBSService) DownloadFile(objectKey string) ([]byte, string, error) {
	if !s.enabled {
		return nil, "", fmt.Errorf("OBS服务未启用")
	}

	input := &obs.GetObjectInput{}
	input.Bucket = s.bucketName
	input.Key = objectKey

	output, err := s.client.GetObject(input)
	if err != nil {
		log.Printf("❌ [OBS] 下载文件失败: %v", err)
		return nil, "", fmt.Errorf("从OBS下载失败: %w", err)
	}
	defer output.Body.Close()

	if output.StatusCode != 200 {
		return nil, "", fmt.Errorf("下载失败，状态码: %d", output.StatusCode)
	}

	// 读取数据
	data, err := io.ReadAll(output.Body)
	if err != nil {
		return nil, "", err
	}

	contentType := output.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	log.Printf("✅ [OBS] 下载成功: %s (%d bytes)", objectKey, len(data))

	return data, contentType, nil
}
