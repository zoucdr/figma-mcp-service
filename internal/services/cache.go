package services

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/figma-deliver/internal/models"
)

// CacheService 缓存服务
type CacheService struct {
	FileAPICooldown   uint32 // 文件API冷却时间（秒）
	ImagesAPICooldown uint32 // 图片API冷却时间（秒）
	OBSExpiresDays    uint32 // OBS文件过期时间（天）
}

// NewCacheService 创建缓存服务
func NewCacheService(fileAPICooldown, imagesAPICooldown, obsExpiresDays uint32) *CacheService {
	return &CacheService{
		FileAPICooldown:   fileAPICooldown,
		ImagesAPICooldown: imagesAPICooldown,
		OBSExpiresDays:    obsExpiresDays,
	}
}

// ===================== Token 冷却管理 =====================

// CheckFileAPICooldown 检查文件API是否可以调用
// 返回: (是否可以调用, 剩余等待时间秒数, 错误)
func (cs *CacheService) CheckFileAPICooldown(token string) (bool, uint32, error) {
	canCall, remaining := models.CheckFileAPICooldown(token, cs.FileAPICooldown)

	if !canCall {
		log.Printf("⏰ [Token:%s...] File API 冷却中，还需等待 %d 秒", token[:10], remaining)
	} else {
		log.Printf("✅ [Token:%s...] File API 冷却完成，可以调用", token[:10])
	}

	return canCall, remaining, nil
}

// CheckImageAPICooldown 检查图片API是否可以调用
// 返回: (是否可以调用, 下次可用时间戳, 错误)
func (cs *CacheService) CheckImageAPICooldown(token string) (bool, uint32, error) {
	canCall, remaining := models.CheckImageAPICooldown(token, cs.ImagesAPICooldown)

	if !canCall {
		log.Printf("⏰ [Token:%s...] Images API 冷却中，还需等待 %d 秒", token[:10], remaining)
	} else {
		log.Printf("✅ [Token:%s...] Images API 冷却完成，可以调用", token[:10])
	}

	// 返回下次可用时间戳
	nextAvailableTime := uint32(time.Now().Unix()) + remaining
	return canCall, nextAvailableTime, nil
}

// GetTokenCooldownRemaining 获取 Token 冷却剩余时间（秒）
// 用于计算等待时间，取 File API 和 Images API 中较大的冷却时间
func (cs *CacheService) GetTokenCooldownRemaining(token string) (uint32, error) {
	// 获取 Token 冷却记录
	cooldown, err := models.GetOrCreateTokenCooldown(token)
	if err != nil {
		log.Printf("❌ [GetTokenCooldownRemaining] 获取冷却记录失败: %v", err)
		return 0, err
	}

	// 获取 File API 冷却剩余时间
	_, fileRemaining := models.CheckFileAPICooldown(token, cs.FileAPICooldown)

	// 获取 Images API 冷却剩余时间
	_, imageRemaining := models.CheckImageAPICooldown(token, cs.ImagesAPICooldown)

	// 返回较大的冷却时间
	maxRemaining := fileRemaining
	if imageRemaining > maxRemaining {
		maxRemaining = imageRemaining
	}

	log.Printf("🔍 [GetTokenCooldownRemaining] Token:%s...", token[:10])
	log.Printf("   - LastFileRequestTime: %d", cooldown.LastFileRequestTime)
	log.Printf("   - LastImageRequestTime: %d", cooldown.LastImageRequestTime)
	log.Printf("   - FileAPICooldown: %d 秒", cs.FileAPICooldown)
	log.Printf("   - ImagesAPICooldown: %d 秒", cs.ImagesAPICooldown)
	log.Printf("   - File API 剩余: %d 秒", fileRemaining)
	log.Printf("   - Images API 剩余: %d 秒", imageRemaining)
	log.Printf("   - 返回最大剩余: %d 秒", maxRemaining)

	return maxRemaining, nil
}

// UpdateFileRequestTime 更新文件请求时间
func (cs *CacheService) UpdateFileRequestTime(token string) error {
	err := models.UpdateFileRequestTime(token)
	if err == nil {
		log.Printf("📝 [Token:%s...] 已更新 File API 请求时间", token[:10])
	}
	return err
}

// UpdateImageRequestTime 更新图片请求时间
func (cs *CacheService) UpdateImageRequestTime(token string) error {
	err := models.UpdateImageRequestTime(token)
	if err == nil {
		log.Printf("📝 [Token:%s...] 已更新 Images API 请求时间", token[:10])
	}
	return err
}

// ===================== 文件缓存管理 =====================

// GetFileCache 获取文件缓存
// 返回: (缓存对象, 是否命中, 错误)
func (cs *CacheService) GetFileCache(fileKey, rootNodeID string) (*models.FigmaFileCache, bool, error) {
	cache, err := models.GetFileCache(fileKey, rootNodeID)

	if err != nil {
		// 缓存未命中
		log.Printf("❌ [FileCache] 未命中缓存 fileKey=%s, rootNodeID=%s", fileKey, rootNodeID)
		return nil, false, nil
	}

	// 检查缓存状态
	if cache.Status != "loaded" {
		log.Printf("⏳ [FileCache] 缓存状态=%s, fileKey=%s, rootNodeID=%s", cache.Status, fileKey, rootNodeID)
		return cache, false, nil
	}

	// 增加命中次数
	_ = models.IncrementFileCacheHit(cache.ID)

	log.Printf("✅ [FileCache] 命中缓存 fileKey=%s, rootNodeID=%s, hitCount=%d",
		fileKey, rootNodeID, cache.HitCount+1)

	return cache, true, nil
}

// CreateFileCache 创建文件缓存（排队状态）
// 参数 token: 必须指定用于请求 Figma API 的 token，不允许使用其他 token
func (cs *CacheService) CreateFileCache(fileKey, rootNodeID, token string, nodeIDs []string) (*models.FigmaFileCache, error) {
	if token == "" {
		return nil, fmt.Errorf("figma_token 不能为空")
	}

	now := uint32(time.Now().Unix())

	cache := &models.FigmaFileCache{
		FigmaToken: token,
		FileKey:    fileKey,
		RootNodeID: rootNodeID,
		NodeIDs:    joinNodeIDs(nodeIDs),
		FileData:   "",
		Status:     "waiting",
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	err := models.CreateFileCache(cache)
	if err != nil {
		log.Printf("❌ [FileCache] 创建失败: %v", err)
		return nil, err
	}

	log.Printf("🆕 [FileCache] 已创建缓存记录 id=%d, fileKey=%s, rootNodeID=%s, token=%s..., status=waiting",
		cache.ID, fileKey, rootNodeID, token[:10])

	return cache, nil
}

// UpdateFileCacheData 更新文件缓存数据（加载成功）
func (cs *CacheService) UpdateFileCacheData(cacheID uint, fileData interface{}, version string) error {
	// 将数据转换为JSON字符串
	jsonData, err := json.Marshal(fileData)
	if err != nil {
		return fmt.Errorf("序列化文件数据失败: %w", err)
	}

	now := uint32(time.Now().Unix())

	cache := &models.FigmaFileCache{
		ID:          cacheID,
		FileData:    string(jsonData),
		FileVersion: version,
		Status:      "loaded",
		UpdatedAt:   now,
	}

	err = models.UpdateFileCache(cache)
	if err != nil {
		log.Printf("❌ [FileCache] 更新数据失败 id=%d: %v", cacheID, err)
		return err
	}

	log.Printf("✅ [FileCache] 已更新缓存数据 id=%d, version=%s, size=%d bytes",
		cacheID, version, len(jsonData))

	return nil
}

// UpdateFileCacheStatus 更新文件缓存状态
func (cs *CacheService) UpdateFileCacheStatus(cacheID uint, status string, errorMsg string) error {
	err := models.UpdateFileCacheStatus(cacheID, status, errorMsg)
	if err != nil {
		log.Printf("❌ [FileCache] 更新状态失败 id=%d: %v", cacheID, err)
		return err
	}

	if errorMsg != "" {
		log.Printf("⚠️ [FileCache] 状态更新 id=%d, status=%s, error=%s", cacheID, status, errorMsg)
	} else {
		log.Printf("📝 [FileCache] 状态更新 id=%d, status=%s", cacheID, status)
	}

	return nil
}

// ===================== 节点图片缓存管理 =====================

// GetNodeImage 获取节点图片缓存
// 返回: (图片缓存对象, 是否有效, 错误)
func (cs *CacheService) GetNodeImage(fileKey, nodeID, format string, scale float64) (*models.FigmaNodeImage, bool, error) {
	image, err := models.GetNodeImage(fileKey, nodeID, format, scale)

	if err != nil {
		// 缓存未命中
		return nil, false, nil
	}

	now := uint32(time.Now().Unix())

	// 检查 OBS Key 是否存在且未过期
	if image.OBSKey != "" && image.OBSExpiresAt > 0 {
		if image.OBSExpiresAt > now {
			// OBS 缓存有效
			_ = models.IncrementNodeImageHit(image.ID)
			log.Printf("✅ [NodeImage] OBS缓存命中 nodeID=%s, format=%s, scale=%.1f",
				nodeID, format, scale)
			return image, true, nil
		} else {
			log.Printf("⏰ [NodeImage] OBS缓存已过期 nodeID=%s, expiresAt=%d, now=%d",
				nodeID, image.OBSExpiresAt, now)
		}
	}

	// 检查 Figma CDN URL 是否存在
	if image.FigmaCDNURL != "" {
		// Figma CDN URL 存在但可能过期，仍然返回（可以尝试使用）
		_ = models.IncrementNodeImageHit(image.ID)
		log.Printf("⚠️ [NodeImage] Figma CDN缓存命中（可能过期） nodeID=%s", nodeID)
		return image, true, nil
	}

	// 缓存存在但无可用URL
	log.Printf("❌ [NodeImage] 缓存无可用URL nodeID=%s, status=%s", nodeID, image.Status)
	return image, false, nil
}

// CreateNodeImage 创建节点图片缓存记录
func (cs *CacheService) CreateNodeImage(fileKey, nodeID, format string, scale float64, figmaCDNURL string) (*models.FigmaNodeImage, error) {
	now := uint32(time.Now().Unix())
	expiresAt := now + (cs.OBSExpiresDays * 24 * 3600) // 转换为秒

	image := &models.FigmaNodeImage{
		FileKey:      fileKey,
		NodeID:       nodeID,
		Format:       format,
		Scale:        scale,
		FigmaCDNURL:  figmaCDNURL,
		OBSKey:       "",
		OBSExpiresAt: expiresAt,
		Status:       "figma_cdn",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	err := models.CreateNodeImage(image)
	if err != nil {
		log.Printf("❌ [NodeImage] 创建失败: %v", err)
		return nil, err
	}

	log.Printf("🆕 [NodeImage] 已创建图片缓存 id=%d, nodeID=%s, format=%s, scale=%.1f",
		image.ID, nodeID, format, scale)

	return image, nil
}

// UpdateNodeImageOBS 更新节点图片的OBS信息
func (cs *CacheService) UpdateNodeImageOBS(imageID uint, obsKey string, fileSize uint64, width, height uint) error {
	now := uint32(time.Now().Unix())
	expiresAt := now + (cs.OBSExpiresDays * 24 * 3600)

	// 使用专门的方法只更新 OBS 相关字段
	err := models.UpdateNodeImageOBSInfo(imageID, obsKey, expiresAt, fileSize, "obs_synced")
	if err != nil {
		log.Printf("❌ [NodeImage] 更新OBS信息失败 id=%d: %v", imageID, err)
		return err
	}

	log.Printf("✅ [NodeImage] 已更新OBS信息 id=%d, obsKey=%s, expiresAt=%d, size=%d bytes",
		imageID, obsKey, expiresAt, fileSize)

	return nil
}

// BatchCreateNodeImages 批量创建节点图片缓存
func (cs *CacheService) BatchCreateNodeImages(fileKey string, nodeImageMap map[string]string, format string, scale float64) error {
	if len(nodeImageMap) == 0 {
		return nil
	}

	now := uint32(time.Now().Unix())
	expiresAt := now + (cs.OBSExpiresDays * 24 * 3600)

	images := make([]models.FigmaNodeImage, 0, len(nodeImageMap))

	for nodeID, figmaCDNURL := range nodeImageMap {
		images = append(images, models.FigmaNodeImage{
			FileKey:      fileKey,
			NodeID:       nodeID,
			Format:       format,
			Scale:        scale,
			FigmaCDNURL:  figmaCDNURL,
			OBSKey:       "",
			OBSExpiresAt: expiresAt,
			Status:       "figma_cdn",
			CreatedAt:    now,
			UpdatedAt:    now,
		})
	}

	err := models.BatchCreateNodeImages(images)
	if err != nil {
		log.Printf("❌ [NodeImage] 批量创建失败: %v", err)
		return err
	}

	log.Printf("✅ [NodeImage] 批量创建成功，共 %d 个图片缓存", len(images))

	return nil
}

// ===================== 辅助函数 =====================

// joinNodeIDs 将节点ID列表转换为逗号分隔的字符串
func joinNodeIDs(nodeIDs []string) string {
	if len(nodeIDs) == 0 {
		return ""
	}

	result := ""
	for i, id := range nodeIDs {
		if i > 0 {
			result += ","
		}
		result += id
	}
	return result
}

// splitNodeIDs 将逗号分隔的字符串转换为节点ID列表
func splitNodeIDs(nodeIDsStr string) []string {
	if nodeIDsStr == "" {
		return []string{}
	}

	result := []string{}
	current := ""

	for i := 0; i < len(nodeIDsStr); i++ {
		if nodeIDsStr[i] == ',' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(nodeIDsStr[i])
		}
	}

	if current != "" {
		result = append(result, current)
	}

	return result
}

// ===================== 子树提取服务方法 =====================

// GetFileCacheWithSubtreeExtraction 获取文件缓存，支持子树提取
// 这是核心优化方法：先精确匹配，再尝试从父树提取子树
func (cs *CacheService) GetFileCacheWithSubtreeExtraction(fileKey, nodeID string) (*models.FigmaFileCache, string, error) {
	// 查找包含该节点的缓存（精确匹配或父树）
	cache, err := models.GetFileCacheContainingNode(fileKey, nodeID)
	if err != nil {
		return nil, "", err
	}

	// 更新命中次数
	cache.HitCount++
	err = models.UpdateFileCache(cache)
	if err != nil {
		log.Printf("⚠️ [Cache] 更新命中次数失败: %v", err)
	}

	// 如果是精确匹配，直接返回
	if cache.RootNodeID == nodeID {
		log.Printf("✅ [Cache] 精确匹配命中: file=%s, node=%s", fileKey, nodeID)
		return cache, cache.FileData, nil
	}

	// 需要提取子树
	log.Printf("🔍 [Cache] 从父树提取子树: file=%s, node=%s, parent_root=%s",
		fileKey, nodeID, cache.RootNodeID)

	subtreeData, err := models.ExtractSubtree(cache.FileData, nodeID)
	if err != nil {
		log.Printf("❌ [Cache] 子树提取失败: %v", err)
		return nil, "", err
	}

	log.Printf("✅ [Cache] 子树提取成功: %d bytes", len(subtreeData))

	// 可选：异步保存提取的子树为新的缓存记录（加速后续查询）
	go cs.saveExtractedSubtreeCache(fileKey, nodeID, subtreeData)

	return cache, subtreeData, nil
}

// saveExtractedSubtreeCache 保存提取的子树为新缓存（可选，异步执行）
func (cs *CacheService) saveExtractedSubtreeCache(fileKey, nodeID, subtreeData string) {
	// 提取子树的所有可见节点ID（排除 visible=false 的节点）
	nodeIDs, err := models.ExtractVisibleNodeIDs(subtreeData)
	if err != nil {
		log.Printf("⚠️ [Cache] 提取节点ID失败: %v", err)
		return
	}

	// 检查是否已存在（避免重复保存）
	existing, err := models.GetFileCacheContainingNode(fileKey, nodeID)
	if err == nil && existing.RootNodeID == nodeID {
		// 已存在精确匹配的缓存，无需保存
		return
	}

	// 创建新的缓存记录
	// 注意：这里无法获取 token，因为是从已有缓存中提取的子树数据
	// 子树缓存应该继承父缓存的 token，但这里暂时跳过保存
	// TODO: 需要重构为继承父缓存的 token
	log.Printf("⚠️ [Cache] 子树缓存保存已跳过（需要 token 信息）: node=%s, node_count=%d", nodeID, len(nodeIDs))
}

// UpdateFileCacheWithNodeIDs 更新文件缓存的 node_ids 字段
// 在从 Figma API 获取数据后调用此方法
func (cs *CacheService) UpdateFileCacheWithNodeIDs(cacheID uint, fileData string) error {
	// 提取所有可见节点ID（排除 visible=false 的节点）
	nodeIDs, err := models.ExtractVisibleNodeIDs(fileData)
	if err != nil {
		log.Printf("⚠️ [Cache] 提取节点ID失败: %v", err)
		// 即使提取失败也不影响主流程，只是失去子树查找优化
		return nil
	}

	// 更新缓存记录
	cache := &models.FigmaFileCache{
		ID:      cacheID,
		NodeIDs: strings.Join(nodeIDs, ","),
	}

	err = models.DB.Model(&models.FigmaFileCache{}).
		Where("id = ?", cacheID).
		Update("node_ids", cache.NodeIDs).Error

	if err != nil {
		log.Printf("❌ [Cache] 更新 node_ids 失败: %v", err)
		return err
	}

	log.Printf("✅ [Cache] node_ids 已更新: cache_id=%d, node_count=%d", cacheID, len(nodeIDs))
	return nil
}

// tryLoadNodeIDsFromFile 尝试从文件系统缓存中加载节点ID列表
// 返回: (节点ID列表, 文件数据JSON字符串, 错误)
func (cs *CacheService) tryLoadNodeIDsFromFile(projectID uint, fileKey, rootNodeID string) ([]string, string, error) {
	// 使用共享的文件读取函数
	data, fileInfo, err := LoadCachedFileData(fileKey, rootNodeID)
	if err != nil {
		return nil, "", err
	}

	// 解析JSON数据并提取有效节点ID（排除 visible=false 和 ignore=true 的节点及其子树）
	nodeIDs, err := models.ExtractValidNodeIDs(string(data), projectID)
	if err != nil {
		return nil, "", fmt.Errorf("从缓存文件提取节点ID失败: %v", err)
	}

	log.Printf("📁 [FileCache] 从文件系统加载节点ID: fileKey=%s, rootNodeID=%s (大小=%d bytes, 节点数=%d, 修改时间=%v)",
		fileKey, rootNodeID, len(data), len(nodeIDs), fileInfo.ModTime())

	return nodeIDs, string(data), nil
}

// LoadCachedFileData 从文件系统加载缓存的节点树JSON数据（共享函数）
// 返回: (文件数据, 文件信息, 错误)
func LoadCachedFileData(fileKey, nodeID string) ([]byte, os.FileInfo, error) {
	// 创建安全的文件名
	safeNodeID := strings.NewReplacer(":", "_", ";", "_", "/", "_", "\\", "_", "?", "_", "*", "_", "\"", "_", "<", "_", ">", "_", "|", "_").Replace(nodeID)
	cacheDir := filepath.Join("temp", fileKey, "documents")
	cacheFile := filepath.Join(cacheDir, safeNodeID+".json")

	// 检查文件是否存在
	fileInfo, err := os.Stat(cacheFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("缓存文件不存在: %s", cacheFile)
		}
		return nil, nil, fmt.Errorf("检查文件失败: %v", err)
	}

	// 读取缓存文件
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil, nil, fmt.Errorf("读取缓存文件失败: %v", err)
	}

	return data, fileInfo, nil
}

// GetProjectNodeIDs 获取项目的所有节点ID列表（优先从缓存的 node_ids 字段获取）
// 支持过滤 visible=false 和 ignore=true 的节点及其子树
// 返回: (节点ID列表, 缓存对象, 错误)
func (cs *CacheService) GetProjectNodeIDs(projectID uint, fileKey, rootNodeID string) ([]string, *models.FigmaFileCache, error) {
	// 0. 优先尝试从文件系统缓存中提取节点ID（最快路径）
	fileNodeIDs, fileDataJSON, err := cs.tryLoadNodeIDsFromFile(projectID, fileKey, rootNodeID)
	if err == nil && len(fileNodeIDs) > 0 {
		log.Printf("✅ [GetProjectNodeIDs] 从文件系统缓存提取到 %d 个节点 (已过滤 ignore 和 visible)", len(fileNodeIDs))

		// 构造一个临时的 cache 对象返回（用于兼容现有接口）
		cache := &models.FigmaFileCache{
			FileKey:    fileKey,
			RootNodeID: rootNodeID,
			NodeIDs:    strings.Join(fileNodeIDs, ","),
			FileData:   fileDataJSON,
			Status:     "loaded",
		}

		return fileNodeIDs, cache, nil
	}
	log.Printf("⚠️ [GetProjectNodeIDs] 文件系统缓存未找到或提取失败: %v，继续查询数据库", err)

	// 1. 先尝试精确匹配查询数据库
	cache, err := models.GetFileCache(fileKey, rootNodeID)
	isExactMatch := (err == nil && cache != nil)

	// 2. 如果精确匹配失败，尝试查找包含该节点的父缓存树
	if err != nil {
		log.Printf("⚠️ [GetProjectNodeIDs] 精确匹配缓存失败，尝试查找包含该节点的父树 (FileKey=%s, RootNodeID=%s)",
			fileKey, rootNodeID)

		cache, err = models.GetFileCacheContainingNode(fileKey, rootNodeID)
		if err != nil {
			log.Printf("❌ [GetProjectNodeIDs] 未找到包含该节点的缓存树: %v (FileKey=%s, RootNodeID=%s)",
				err, fileKey, rootNodeID)
			return nil, nil, fmt.Errorf("节点树缓存不存在，请先在 Dashboard 中刷新节点树")
		}

		log.Printf("✅ [GetProjectNodeIDs] 找到父缓存树 (CacheID=%d, ParentRootNodeID=%s, 目标节点=%s)",
			cache.ID, cache.RootNodeID, rootNodeID)
	}

	if cache == nil {
		log.Printf("❌ [GetProjectNodeIDs] 节点树缓存不存在 (FileKey=%s, RootNodeID=%s)",
			fileKey, rootNodeID)
		return nil, nil, fmt.Errorf("节点树缓存不存在，请先在 Dashboard 中刷新节点树")
	}

	// 3. 始终从 file_data 中提取节点（应用 ignore 和 visible 过滤）
	// 不再直接使用 node_ids 字段，因为它无法反映节点的树形关系，无法正确过滤子树
	if cache.FileData == "" {
		log.Printf("❌ [GetProjectNodeIDs] 节点树缓存数据为空 (FileKey=%s, RootNodeID=%s, CacheID=%d, Status=%s)",
			fileKey, rootNodeID, cache.ID, cache.Status)
		return nil, cache, fmt.Errorf("节点树缓存数据为空，请在 Dashboard 中重新刷新节点树")
	}

	var extractedIDs []string

	// 4. 如果不是精确匹配，需要提取子树节点
	if !isExactMatch {
		log.Printf("⚠️ [GetProjectNodeIDs] 从父缓存树中提取子树节点（应用 ignore 和 visible 过滤）(TargetNodeID=%s)", rootNodeID)

		// 提取子树
		subtreeData, err := models.ExtractSubtree(cache.FileData, rootNodeID)
		if err != nil {
			log.Printf("❌ [GetProjectNodeIDs] 提取子树失败: %v (TargetNodeID=%s)", err, rootNodeID)
			return nil, cache, fmt.Errorf("提取子树失败: %v", err)
		}

		// 从子树中提取所有有效节点ID（排除 visible=false 和 ignore=true 的节点及其子树）
		extractedIDs, err = models.ExtractValidNodeIDs(subtreeData, projectID)
		if err != nil {
			log.Printf("❌ [GetProjectNodeIDs] 从子树提取节点ID失败: %v", err)
			return nil, cache, fmt.Errorf("解析子树数据失败: %v", err)
		}

		log.Printf("✅ [GetProjectNodeIDs] 从子树提取到 %d 个有效节点（已过滤 ignore 和 visible）", len(extractedIDs))
	} else {
		// 精确匹配，从 file_data 中提取节点（应用 ignore 和 visible 过滤）
		log.Printf("📦 [GetProjectNodeIDs] 从 file_data 中提取节点（应用 ignore 和 visible 过滤）")

		extractedIDs, err = models.ExtractValidNodeIDs(cache.FileData, projectID)
		if err != nil {
			log.Printf("❌ [GetProjectNodeIDs] 从 file_data 提取节点ID失败: %v", err)
			return nil, cache, fmt.Errorf("解析节点树数据失败: %v", err)
		}

		log.Printf("✅ [GetProjectNodeIDs] 从 file_data 提取到 %d 个有效节点（已过滤 ignore 和 visible）", len(extractedIDs))
	}

	return extractedIDs, cache, nil
}
