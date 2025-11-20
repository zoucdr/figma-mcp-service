package models

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"
)

// FigmaTokenCooldown Token冷却管理表
type FigmaTokenCooldown struct {
	ID                   uint   `gorm:"primaryKey" json:"id"`
	FigmaToken           string `gorm:"size:255;uniqueIndex:idx_figma_token;not null" json:"figma_token"`
	LastFileRequestTime  uint32 `gorm:"default:0" json:"last_file_request_time"`  // Unix时间戳（秒）
	LastImageRequestTime uint32 `gorm:"default:0" json:"last_image_request_time"` // Unix时间戳（秒）
	CreatedAt            uint32 `gorm:"not null" json:"created_at"`               // Unix时间戳（秒）
	UpdatedAt            uint32 `gorm:"not null" json:"updated_at"`               // Unix时间戳（秒）
}

// TableName 指定表名
func (FigmaTokenCooldown) TableName() string {
	return "figma_token_cooldowns"
}

// BeforeCreate GORM钩子：创建前设置时间戳
func (t *FigmaTokenCooldown) BeforeCreate(db interface{}) error {
	now := uint32(time.Now().Unix())
	t.CreatedAt = now
	t.UpdatedAt = now
	return nil
}

// BeforeUpdate GORM钩子：更新前设置时间戳
func (t *FigmaTokenCooldown) BeforeUpdate(db interface{}) error {
	t.UpdatedAt = uint32(time.Now().Unix())
	return nil
}

// FigmaFileCache 节点树缓存表
type FigmaFileCache struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	FigmaToken   string `gorm:"size:255;not null;index:idx_figma_token" json:"figma_token"` // 必须使用此token请求Figma API
	FileKey      string `gorm:"size:100;not null;uniqueIndex:idx_file_root" json:"file_key"`
	RootNodeID   string `gorm:"size:100;not null;uniqueIndex:idx_file_root" json:"root_node_id"`
	NodeIDs      string `gorm:"type:text" json:"node_ids"`         // 逗号分隔的节点ID列表
	FileData     string `gorm:"type:mediumtext;not null" json:"-"` // JSON数据，最大16MB
	FileVersion  string `gorm:"size:100" json:"file_version"`
	Status       string `gorm:"size:20;not null;default:'waiting';index:idx_status" json:"status"` // waiting, loading, loaded, error
	ErrorMessage string `gorm:"type:text" json:"error_message"`
	HitCount     uint   `gorm:"default:0" json:"hit_count"`
	CreatedAt    uint32 `gorm:"not null" json:"created_at"` // Unix时间戳（秒）
	UpdatedAt    uint32 `gorm:"not null" json:"updated_at"` // Unix时间戳（秒）
}

// TableName 指定表名
func (FigmaFileCache) TableName() string {
	return "figma_file_caches"
}

// BeforeCreate GORM钩子：创建前设置时间戳
func (f *FigmaFileCache) BeforeCreate(db interface{}) error {
	now := uint32(time.Now().Unix())
	f.CreatedAt = now
	f.UpdatedAt = now
	return nil
}

// BeforeUpdate GORM钩子：更新前设置时间戳
func (f *FigmaFileCache) BeforeUpdate(db interface{}) error {
	f.UpdatedAt = uint32(time.Now().Unix())
	return nil
}

// FigmaRenderQueue 渲染队列表
type FigmaRenderQueue struct {
	ID             uint    `gorm:"primaryKey" json:"id"`
	FigmaToken     string  `gorm:"size:255;not null;index:idx_figma_token;index:idx_token_status" json:"figma_token"`
	FileKey        string  `gorm:"size:100;not null;index:idx_file_key" json:"file_key"`
	NodeIDs        string  `gorm:"type:text;not null" json:"node_ids"` // 逗号分隔，最多1000个
	ProjectIDs     string  `gorm:"type:text" json:"project_ids"`       // 关联的项目ID列表，JSON格式存储
	Format         string  `gorm:"size:10;not null;default:'png'" json:"format"`
	Scale          float64 `gorm:"type:decimal(3,1);not null;default:1.0" json:"scale"`
	Status         string  `gorm:"size:20;not null;default:'waiting';index:idx_status;index:idx_token_status" json:"status"` // waiting, processing, completed, error, cancelled
	Progress       uint    `gorm:"default:0" json:"progress"`
	TotalNodes     uint    `gorm:"default:0" json:"total_nodes"`
	ProcessedNodes uint    `gorm:"default:0" json:"processed_nodes"`
	FailedNodes    uint    `gorm:"default:0" json:"failed_nodes"`
	ErrorMessage   string  `gorm:"type:text" json:"error_message"`
	StartedAt      uint32  `gorm:"default:0" json:"started_at"`   // Unix时间戳（秒）
	CompletedAt    uint32  `gorm:"default:0" json:"completed_at"` // Unix时间戳（秒）
	CreatedAt      uint32  `gorm:"not null" json:"created_at"`    // Unix时间戳（秒）
	UpdatedAt      uint32  `gorm:"not null" json:"updated_at"`    // Unix时间戳（秒）
}

// TableName 指定表名
func (FigmaRenderQueue) TableName() string {
	return "figma_render_queues"
}

// BeforeCreate GORM钩子：创建前设置时间戳
func (r *FigmaRenderQueue) BeforeCreate(db interface{}) error {
	now := uint32(time.Now().Unix())
	r.CreatedAt = now
	r.UpdatedAt = now
	return nil
}

// BeforeUpdate GORM钩子：更新前设置时间戳
func (r *FigmaRenderQueue) BeforeUpdate(db interface{}) error {
	r.UpdatedAt = uint32(time.Now().Unix())
	return nil
}

// FigmaNodeImage 节点图片缓存表
type FigmaNodeImage struct {
	ID             uint    `gorm:"primaryKey" json:"id"`
	FileKey        string  `gorm:"size:100;not null;uniqueIndex:idx_node_format_scale" json:"file_key"`
	NodeID         string  `gorm:"size:100;not null;uniqueIndex:idx_node_format_scale" json:"node_id"`
	Format         string  `gorm:"size:10;not null;uniqueIndex:idx_node_format_scale" json:"format"`
	Scale          float64 `gorm:"type:decimal(3,1);not null;uniqueIndex:idx_node_format_scale" json:"scale"`
	FigmaCDNURL    string  `gorm:"size:1000" json:"figma_cdn_url"`
	OBSKey         string  `gorm:"size:500;index:idx_obs_key" json:"obs_key"`             // OBS对象Key（通过它可以动态生成URL）
	OBSExpiresAt   uint32  `gorm:"default:0;index:idx_obs_expires" json:"obs_expires_at"` // Unix时间戳（秒），1个月
	FileSize       uint64  `gorm:"default:0" json:"file_size"`
	Width          uint    `gorm:"default:0" json:"width"`
	Height         uint    `gorm:"default:0" json:"height"`
	Status         string  `gorm:"size:20;not null;default:'pending';index:idx_status" json:"status"` // pending, figma_cdn, obs_synced, error
	ErrorMessage   string  `gorm:"type:text" json:"error_message"`
	HitCount       uint    `gorm:"default:0" json:"hit_count"`
	LastAccessedAt uint32  `gorm:"default:0;index:idx_last_accessed" json:"last_accessed_at"` // Unix时间戳（秒）
	CreatedAt      uint32  `gorm:"not null" json:"created_at"`                                // Unix时间戳（秒）
	UpdatedAt      uint32  `gorm:"not null" json:"updated_at"`                                // Unix时间戳（秒）
}

// TableName 指定表名
func (FigmaNodeImage) TableName() string {
	return "figma_node_images"
}

// BeforeCreate GORM钩子：创建前设置时间戳
func (n *FigmaNodeImage) BeforeCreate(db interface{}) error {
	now := uint32(time.Now().Unix())
	n.CreatedAt = now
	n.UpdatedAt = now
	return nil
}

// BeforeUpdate GORM钩子：更新前设置时间戳
func (n *FigmaNodeImage) BeforeUpdate(db interface{}) error {
	n.UpdatedAt = uint32(time.Now().Unix())
	return nil
}

// ===================== Token冷却管理相关方法 =====================

// GetOrCreateTokenCooldown 获取或创建Token冷却记录
func GetOrCreateTokenCooldown(token string) (*FigmaTokenCooldown, error) {
	var cooldown FigmaTokenCooldown
	result := DB.Where("figma_token = ?", token).First(&cooldown)

	if result.Error != nil {
		// 如果不存在，创建新记录
		now := uint32(time.Now().Unix())
		cooldown = FigmaTokenCooldown{
			FigmaToken:           token,
			LastFileRequestTime:  0,
			LastImageRequestTime: 0,
			CreatedAt:            now,
			UpdatedAt:            now,
		}
		if err := DB.Create(&cooldown).Error; err != nil {
			return nil, err
		}
	}

	return &cooldown, nil
}

// UpdateFileRequestTime 更新文件请求时间
// 只有当新值大于当前值时才更新，避免时间戳回退
func UpdateFileRequestTime(token string) error {
	now := uint32(time.Now().Unix())

	// 获取当前冷却信息
	cooldown, err := GetOrCreateTokenCooldown(token)
	if err != nil {
		return err
	}

	// 只有在新值大于旧值时才更新
	if now <= cooldown.LastFileRequestTime {
		// 时间戳没有前进，跳过更新
		return nil
	}

	return DB.Model(&FigmaTokenCooldown{}).
		Where("figma_token = ?", token).
		Updates(map[string]interface{}{
			"last_file_request_time": now,
			"updated_at":             now,
		}).Error
}

// UpdateImageRequestTime 更新图片请求时间
// 只有当新值大于当前值时才更新，避免时间戳回退
func UpdateImageRequestTime(token string) error {
	now := uint32(time.Now().Unix())

	// 获取当前冷却信息
	cooldown, err := GetOrCreateTokenCooldown(token)
	if err != nil {
		return err
	}

	// 只有在新值大于旧值时才更新
	if now <= cooldown.LastImageRequestTime {
		// 时间戳没有前进，跳过更新
		return nil
	}

	log.Printf("✅ [Cache Time] 已更新 last_image_request_time: %d", now)
	return DB.Model(&FigmaTokenCooldown{}).
		Where("figma_token = ?", token).
		Updates(map[string]interface{}{
			"last_image_request_time": now,
			"updated_at":              now,
		}).Error
}

// SetCooldownUntil 设置冷却时间直到指定时间戳（用于处理 Retry-After）
// 同时更新文件和图片请求时间，确保期间无法调用任何 Figma API
// 只有当新值大于当前值时才更新，避免时间戳回退
func SetCooldownUntil(token string, untilTimestamp uint32) error {
	now := uint32(time.Now().Unix())

	// 获取当前冷却信息
	cooldown, err := GetOrCreateTokenCooldown(token)
	if err != nil {
		return err
	}

	// 计算需要更新的字段
	updates := make(map[string]interface{})
	updates["updated_at"] = now

	// 只有在新值大于旧值时才更新 last_file_request_time
	if untilTimestamp > cooldown.LastFileRequestTime {
		updates["last_file_request_time"] = untilTimestamp
	}

	// 只有在新值大于旧值时才更新 last_image_request_time
	if untilTimestamp > cooldown.LastImageRequestTime {
		updates["last_image_request_time"] = untilTimestamp
	}

	// 如果没有需要更新的时间字段，直接返回
	if len(updates) == 1 { // 只有 updated_at
		return nil
	}

	return DB.Model(&FigmaTokenCooldown{}).
		Where("figma_token = ?", token).
		Updates(updates).Error
}

// CheckFileAPICooldown 检查文件API冷却时间（30秒）
func CheckFileAPICooldown(token string, cooldownSeconds uint32) (bool, uint32) {
	cooldown, err := GetOrCreateTokenCooldown(token)
	if err != nil {
		return false, 0
	}

	now := uint32(time.Now().Unix())

	// Figma API 速率限制是全局的，需要检查两个时间戳中较大的那个
	lastRequestTime := cooldown.LastFileRequestTime
	if cooldown.LastImageRequestTime > lastRequestTime {
		lastRequestTime = cooldown.LastImageRequestTime
	}

	if lastRequestTime == 0 {
		return true, 0 // 从未请求过，可以请求
	}

	// 支持未来时间：如果 lastRequestTime 是未来时间（Retry-After 设置）
	// 则直接计算剩余时间
	if lastRequestTime > now {
		// lastRequestTime 是冷却结束时间，直接返回剩余秒数
		remaining := lastRequestTime - now
		return false, remaining
	}

	// 正常情况：lastRequestTime 是过去的时间
	elapsed := now - lastRequestTime
	if elapsed >= cooldownSeconds {
		return true, 0 // 冷却时间已过
	}

	remaining := cooldownSeconds - elapsed
	return false, remaining // 还需要等待
}

// CheckImageAPICooldown 检查图片API冷却时间（30秒）
func CheckImageAPICooldown(token string, cooldownSeconds uint32) (bool, uint32) {
	cooldown, err := GetOrCreateTokenCooldown(token)
	if err != nil {
		return false, 0
	}

	now := uint32(time.Now().Unix())

	// Figma API 速率限制是全局的，需要检查两个时间戳中较大的那个
	lastRequestTime := cooldown.LastImageRequestTime
	if cooldown.LastFileRequestTime > lastRequestTime {
		lastRequestTime = cooldown.LastFileRequestTime
	}

	if lastRequestTime == 0 {
		return true, 0 // 从未请求过，可以请求
	}

	// 支持未来时间：如果 lastRequestTime 是未来时间（Retry-After 设置）
	// 则直接计算剩余时间
	if lastRequestTime > now {
		// lastRequestTime 是冷却结束时间，直接返回剩余秒数
		remaining := lastRequestTime - now
		return false, remaining
	}

	// 正常情况：lastRequestTime 是过去的时间
	elapsed := now - lastRequestTime
	if elapsed >= cooldownSeconds {
		return true, 0 // 冷却时间已过
	}

	remaining := cooldownSeconds - elapsed
	return false, remaining // 还需要等待
}

// ===================== 文件缓存相关方法 =====================

// GetFileCache 获取文件缓存
func GetFileCache(fileKey, rootNodeID string) (*FigmaFileCache, error) {
	var cache FigmaFileCache
	result := DB.Where("file_key = ? AND root_node_id = ?", fileKey, rootNodeID).First(&cache)
	if result.Error != nil {
		return nil, result.Error
	}
	return &cache, nil
}

// CreateFileCache 创建文件缓存
func CreateFileCache(cache *FigmaFileCache) error {
	return DB.Create(cache).Error
}

// UpdateFileCache 更新文件缓存
func UpdateFileCache(cache *FigmaFileCache) error {
	return DB.Save(cache).Error
}

// UpdateFileCacheStatus 更新文件缓存状态
func UpdateFileCacheStatus(id uint, status string, errorMsg string) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": uint32(time.Now().Unix()),
	}
	if errorMsg != "" {
		updates["error_message"] = errorMsg
	}
	return DB.Model(&FigmaFileCache{}).Where("id = ?", id).Updates(updates).Error
}

// UpdateFileCacheNodeIDs 更新文件缓存的节点ID列表（只更新 node_ids 字段）
func UpdateFileCacheNodeIDs(id uint, nodeIDs string) error {
	updates := map[string]interface{}{
		"node_ids":   nodeIDs,
		"updated_at": uint32(time.Now().Unix()),
	}
	return DB.Model(&FigmaFileCache{}).Where("id = ?", id).Updates(updates).Error
}

// IncrementFileCacheHit 增加缓存命中次数
func IncrementFileCacheHit(id uint) error {
	return DB.Model(&FigmaFileCache{}).Where("id = ?", id).
		UpdateColumn("hit_count", DB.Raw("hit_count + 1")).Error
}

// ===================== 渲染队列相关方法 =====================

// CreateRenderQueue 创建渲染队列
func CreateRenderQueue(queue *FigmaRenderQueue) error {
	return DB.Create(queue).Error
}

// GetRenderQueue 获取渲染队列
func GetRenderQueue(id uint) (*FigmaRenderQueue, error) {
	var queue FigmaRenderQueue
	result := DB.First(&queue, id)
	if result.Error != nil {
		return nil, result.Error
	}
	return &queue, nil
}

// GetWaitingRenderQueues 获取等待中的渲染队列
// Token 冷却时间由 figma_token_cooldowns 表统一管理，这里只查询 waiting 状态
func GetWaitingRenderQueues() ([]FigmaRenderQueue, error) {
	var queues []FigmaRenderQueue
	result := DB.Where("status = ?", "waiting").
		Order("created_at ASC").
		Find(&queues)
	if result.Error != nil {
		return nil, result.Error
	}
	return queues, nil
}

// GetRenderQueuesByToken 获取指定Token的渲染队列
func GetRenderQueuesByToken(token string, statuses []string) ([]FigmaRenderQueue, error) {
	var queues []FigmaRenderQueue
	query := DB.Where("figma_token = ?", token)
	if len(statuses) > 0 {
		query = query.Where("status IN ?", statuses)
	}
	result := query.Order("created_at DESC").Find(&queues)
	if result.Error != nil {
		return nil, result.Error
	}
	return queues, nil
}

// UpdateRenderQueueStatus 更新渲染队列状态
func UpdateRenderQueueStatus(id uint, status string, progress uint) error {
	updates := map[string]interface{}{
		"status":     status,
		"progress":   progress,
		"updated_at": uint32(time.Now().Unix()),
	}

	now := uint32(time.Now().Unix())
	if status == "processing" && progress == 0 {
		updates["started_at"] = now
	} else if status == "completed" {
		updates["completed_at"] = now
	}

	return DB.Model(&FigmaRenderQueue{}).Where("id = ?", id).Updates(updates).Error
}

// UpdateRenderQueueProgress 更新渲染队列进度
func UpdateRenderQueueProgress(id uint, processed, failed uint) error {
	updates := map[string]interface{}{
		"processed_nodes": processed,
		"failed_nodes":    failed,
		"updated_at":      uint32(time.Now().Unix()),
	}
	return DB.Model(&FigmaRenderQueue{}).Where("id = ?", id).Updates(updates).Error
}

// ===================== 节点图片缓存相关方法 =====================

// GetNodeImage 获取节点图片缓存
func GetNodeImage(fileKey, nodeID, format string, scale float64) (*FigmaNodeImage, error) {
	var image FigmaNodeImage
	result := DB.Where("file_key = ? AND node_id = ? AND format = ? AND scale = ?",
		fileKey, nodeID, format, scale).First(&image)
	if result.Error != nil {
		return nil, result.Error
	}
	return &image, nil
}

// CreateNodeImage 创建节点图片缓存
// 如果记录已存在（根据 file_key + node_id + format + scale 唯一索引），则更新记录
func CreateNodeImage(image *FigmaNodeImage) error {
	// 使用 Clauses 实现 ON DUPLICATE KEY UPDATE
	// 当唯一键冲突时，更新除了 ID 和 CreatedAt 之外的所有字段
	result := DB.Exec(`
		INSERT INTO figma_node_images (
			file_key, node_id, format, scale, figma_cdn_url, obs_key, obs_expires_at,
			file_size, width, height, status, error_message, hit_count, last_accessed_at,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			figma_cdn_url = VALUES(figma_cdn_url),
			obs_key = VALUES(obs_key),
			obs_expires_at = VALUES(obs_expires_at),
			file_size = VALUES(file_size),
			width = VALUES(width),
			height = VALUES(height),
			status = VALUES(status),
			error_message = VALUES(error_message),
			updated_at = VALUES(updated_at)
	`,
		image.FileKey, image.NodeID, image.Format, image.Scale, image.FigmaCDNURL,
		image.OBSKey, image.OBSExpiresAt, image.FileSize, image.Width, image.Height,
		image.Status, image.ErrorMessage, image.HitCount, image.LastAccessedAt,
		image.CreatedAt, image.UpdatedAt,
	)

	return result.Error
}

// UpdateNodeImage 更新节点图片缓存
func UpdateNodeImage(image *FigmaNodeImage) error {
	// 使用 Updates 方法，只更新非零值字段
	// 但需要特别处理 UpdatedAt，确保它总是被更新
	return DB.Model(&FigmaNodeImage{}).
		Where("id = ?", image.ID).
		Updates(image).Error
}

// UpdateNodeImageOBSInfo 更新节点图片的OBS信息（只更新指定字段）
func UpdateNodeImageOBSInfo(id uint, obsKey string, obsExpiresAt uint32, fileSize uint64, status string) error {
	updates := map[string]interface{}{
		"obs_key":        obsKey,
		"obs_expires_at": obsExpiresAt,
		"file_size":      fileSize,
		"status":         status,
		"updated_at":     uint32(time.Now().Unix()),
	}

	return DB.Model(&FigmaNodeImage{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// UpdateNodeImageStatus 更新节点图片状态
func UpdateNodeImageStatus(id uint, status string, errorMsg string) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": uint32(time.Now().Unix()),
	}
	if errorMsg != "" {
		updates["error_message"] = errorMsg
	}
	return DB.Model(&FigmaNodeImage{}).Where("id = ?", id).Updates(updates).Error
}

// IncrementNodeImageHit 增加图片访问次数
func IncrementNodeImageHit(id uint) error {
	now := uint32(time.Now().Unix())
	return DB.Model(&FigmaNodeImage{}).Where("id = ?", id).Updates(map[string]interface{}{
		"hit_count":        DB.Raw("hit_count + 1"),
		"last_accessed_at": now,
	}).Error
}

// BatchCreateNodeImages 批量创建节点图片缓存
func BatchCreateNodeImages(images []FigmaNodeImage) error {
	if len(images) == 0 {
		return nil
	}
	return DB.Create(&images).Error
}

// UpdateRenderQueue 更新渲染队列（完整更新）
func UpdateRenderQueue(queue *FigmaRenderQueue) error {
	return DB.Save(queue).Error
}

// ===================== 子树提取辅助方法 =====================

// ExtractAllNodeIDs 从 Figma 文件树 JSON 中提取所有节点ID
func ExtractAllNodeIDs(fileData string) ([]string, error) {
	var data map[string]interface{}
	err := json.Unmarshal([]byte(fileData), &data)
	if err != nil {
		return nil, err
	}

	nodeIDs := make([]string, 0)
	extractNodeIDsRecursive(data, &nodeIDs)
	return nodeIDs, nil
}

// ExtractVisibleNodeIDs 从 Figma 文件树 JSON 中提取所有可见节点ID（排除 visible=false 的节点）
func ExtractVisibleNodeIDs(fileData string) ([]string, error) {
	var data map[string]interface{}
	err := json.Unmarshal([]byte(fileData), &data)
	if err != nil {
		return nil, err
	}

	nodeIDs := make([]string, 0)
	extractVisibleNodeIDsRecursive(data, &nodeIDs)
	return nodeIDs, nil
}

// ExtractValidNodeIDs 从 Figma 文件树 JSON 中提取所有有效节点ID
// 排除 visible=false 的节点及其子树，以及 modify 中标记为 ignore=true 的节点及其子树
func ExtractValidNodeIDs(fileData string, projectID uint) ([]string, error) {
	var data map[string]interface{}
	err := json.Unmarshal([]byte(fileData), &data)
	if err != nil {
		return nil, err
	}

	// 获取节点的modify信息（用于检查ignore标记）
	nodeModifys := make(map[string]map[string]interface{})
	var figmaNodes []FigmaNode
	if err := DB.Where("project_id = ?", projectID).Find(&figmaNodes).Error; err == nil {
		for _, node := range figmaNodes {
			if node.Modifys != "" {
				var modifyData map[string]interface{}
				if err := json.Unmarshal([]byte(node.Modifys), &modifyData); err == nil {
					nodeModifys[node.NodeID] = modifyData
				}
			}
		}
	}

	nodeIDs := make([]string, 0)
	extractValidNodeIDsRecursive(data, &nodeIDs, nodeModifys)
	return nodeIDs, nil
}

// extractNodeIDsRecursive 递归提取节点ID
func extractNodeIDsRecursive(node interface{}, nodeIDs *[]string) {
	nodeMap, ok := node.(map[string]interface{})
	if !ok {
		return
	}

	// 提取当前节点ID
	if id, exists := nodeMap["id"]; exists {
		if idStr, ok := id.(string); ok {
			*nodeIDs = append(*nodeIDs, idStr)
		}
	}
}

// extractVisibleNodeIDsRecursive 递归提取可见节点ID（排除 visible=false）
func extractVisibleNodeIDsRecursive(node interface{}, nodeIDs *[]string) {
	nodeMap, ok := node.(map[string]interface{})
	if !ok {
		return
	}

	// 检查 visible 属性，如果为 false 则跳过该节点及其子树
	if visible, exists := nodeMap["visible"]; exists {
		if visibleBool, ok := visible.(bool); ok && !visibleBool {
			// 节点不可见，跳过该节点及其子树
			return
		}
	}

	// 提取当前节点ID（只有可见节点才添加）
	if id, exists := nodeMap["id"]; exists {
		if idStr, ok := id.(string); ok {
			*nodeIDs = append(*nodeIDs, idStr)
		}
	}

	// 递归处理子节点（标准树形结构）
	if children, exists := nodeMap["children"]; exists {
		if childrenArray, ok := children.([]interface{}); ok {
			for _, child := range childrenArray {
				extractVisibleNodeIDsRecursive(child, nodeIDs)
			}
		}
	}

	// 处理 Figma 文档结构的特殊字段
	if document, exists := nodeMap["document"]; exists {
		extractVisibleNodeIDsRecursive(document, nodeIDs)
	}

	// 处理以 Map 形式存储的节点数据（如 .nodes 或 .data 字段）
	// 这种情况下，节点以 key-value 形式存储，key 是节点ID，value 是节点对象
	if nodes, exists := nodeMap["nodes"]; exists {
		if nodesMap, ok := nodes.(map[string]interface{}); ok {
			for _, nodeData := range nodesMap {
				extractVisibleNodeIDsRecursive(nodeData, nodeIDs)
			}
		}
	}
}

// extractValidNodeIDsRecursive 递归提取有效节点ID（排除 visible=false 和 ignore=true 的节点及其子树）
func extractValidNodeIDsRecursive(node interface{}, nodeIDs *[]string, nodeModifys map[string]map[string]interface{}) {
	nodeMap, ok := node.(map[string]interface{})
	if !ok {
		return
	}

	// 检查 visible 属性，如果为 false 则跳过该节点及其子树
	if visible, exists := nodeMap["visible"]; exists {
		if visibleBool, ok := visible.(bool); ok && !visibleBool {
			// 节点不可见，跳过该节点及其子树
			return
		}
	}

	// 提取当前节点ID并检查 ignore 标记
	var currentNodeID string
	if id, exists := nodeMap["id"]; exists {
		if idStr, ok := id.(string); ok {
			currentNodeID = idStr

			// 检查是否被标记为ignore（如果有modify信息）
			if modifys, hasModify := nodeModifys[currentNodeID]; hasModify {
				if ignore, ok := modifys["ignore"].(bool); ok && ignore {
					// 跳过被标记为忽略的节点及其子树
					return
				}
			}

			*nodeIDs = append(*nodeIDs, currentNodeID)
		}
	}

	// 递归处理子节点（标准树形结构）
	if children, exists := nodeMap["children"]; exists {
		if childrenArray, ok := children.([]interface{}); ok {
			for _, child := range childrenArray {
				extractValidNodeIDsRecursive(child, nodeIDs, nodeModifys)
			}
		}
	}

	// 处理 Figma 文档结构的特殊字段
	if document, exists := nodeMap["document"]; exists {
		extractValidNodeIDsRecursive(document, nodeIDs, nodeModifys)
	}

	// 处理以 Map 形式存储的节点数据（如 .nodes 或 .data 字段）
	if nodes, exists := nodeMap["nodes"]; exists {
		if nodesMap, ok := nodes.(map[string]interface{}); ok {
			for _, nodeData := range nodesMap {
				extractValidNodeIDsRecursive(nodeData, nodeIDs, nodeModifys)
			}
		}
	}

	// 兼容 .data 字段（某些情况下节点也可能存储在这里）
	if data, exists := nodeMap["data"]; exists {
		if dataMap, ok := data.(map[string]interface{}); ok {
			// 检查是否是节点Map结构
			for _, value := range dataMap {
				// 如果 value 是对象且包含节点特征（如包含 id 字段），则递归处理
				if valueMap, ok := value.(map[string]interface{}); ok {
					if _, hasID := valueMap["id"]; hasID {
						extractVisibleNodeIDsRecursive(valueMap, nodeIDs)
					}
				}
			}
		} else {
			// 如果 data 本身是对象而不是 map，递归处理
			extractVisibleNodeIDsRecursive(data, nodeIDs)
		}
	}
}

// ExtractSubtree 从完整树中提取子树
func ExtractSubtree(fileData string, targetNodeID string) (string, error) {
	var data map[string]interface{}
	err := json.Unmarshal([]byte(fileData), &data)
	if err != nil {
		return "", err
	}

	// 查找目标节点
	subtree := findNodeInTree(data, targetNodeID)
	if subtree == nil {
		return "", fmt.Errorf("节点 %s 不存在于树中", targetNodeID)
	}

	// 将子树转换为 JSON
	subtreeJSON, err := json.Marshal(subtree)
	if err != nil {
		return "", err
	}

	return string(subtreeJSON), nil
}

// findNodeInTree 在树中查找指定节点
func findNodeInTree(node interface{}, targetNodeID string) interface{} {
	nodeMap, ok := node.(map[string]interface{})
	if !ok {
		return nil
	}

	// 检查当前节点
	if id, exists := nodeMap["id"]; exists {
		if idStr, ok := id.(string); ok && idStr == targetNodeID {
			return node
		}
	}

	// 在子节点中查找
	if children, exists := nodeMap["children"]; exists {
		if childrenArray, ok := children.([]interface{}); ok {
			for _, child := range childrenArray {
				result := findNodeInTree(child, targetNodeID)
				if result != nil {
					return result
				}
			}
		}
	}

	// 在 document 字段中查找
	if document, exists := nodeMap["document"]; exists {
		return findNodeInTree(document, targetNodeID)
	}

	// 在 nodes map 中查找（支持 Map 结构的缓存）
	if nodes, exists := nodeMap["nodes"]; exists {
		if nodesMap, ok := nodes.(map[string]interface{}); ok {
			// 直接检查 key
			if nodeData, exists := nodesMap[targetNodeID]; exists {
				return nodeData
			}
			// 递归检查每个节点
			for _, nodeData := range nodesMap {
				result := findNodeInTree(nodeData, targetNodeID)
				if result != nil {
					return result
				}
			}
		}
	}

	return nil
}

// ContainsNodeID 检查 node_ids 字符串是否包含指定节点ID
func ContainsNodeID(nodeIDs, targetNodeID string) bool {
	if nodeIDs == "" {
		return false
	}

	// 精确匹配，避免部分匹配问题
	// 例如 "1:123" 不应匹配 "1:1234"
	ids := strings.Split(nodeIDs, ",")
	for _, id := range ids {
		if strings.TrimSpace(id) == targetNodeID {
			return true
		}
	}
	return false
}

// GetFileCacheContainingNode 获取包含指定节点的文件缓存
func GetFileCacheContainingNode(fileKey, nodeID string) (*FigmaFileCache, error) {
	var cache FigmaFileCache

	// 先尝试精确匹配
	err := DB.Where("file_key = ? AND root_node_id = ? AND status = ?",
		fileKey, nodeID, "loaded").First(&cache).Error

	if err == nil {
		log.Printf("✅ [GetFileCacheContainingNode] 精确匹配成功 (FileKey=%s, NodeID=%s, CacheID=%d)",
			fileKey, nodeID, cache.ID)
		return &cache, nil
	}

	// 如果精确匹配失败，查找包含该节点的父树
	log.Printf("🔍 [GetFileCacheContainingNode] 精确匹配失败，查找父树... (FileKey=%s, NodeID=%s)",
		fileKey, nodeID)

	var caches []FigmaFileCache
	err = DB.Where("file_key = ? AND status = ?", fileKey, "loaded").
		Order("LENGTH(node_ids) ASC"). // 优先返回最小的包含树
		Find(&caches).Error

	if err != nil {
		log.Printf("❌ [GetFileCacheContainingNode] 查询缓存失败: %v", err)
		return nil, err
	}

	log.Printf("📊 [GetFileCacheContainingNode] 找到 %d 个候选缓存记录", len(caches))

	// 检查哪个缓存包含目标节点
	for i, c := range caches {
		log.Printf("  [%d/%d] 检查缓存: CacheID=%d, RootNodeID=%s, NodeIDsLength=%d, FileDataLength=%d",
			i+1, len(caches), c.ID, c.RootNodeID, len(c.NodeIDs), len(c.FileData))

		// 1. 先从 node_ids 字段检查（快速路径）
		if c.NodeIDs != "" && ContainsNodeID(c.NodeIDs, nodeID) {
			log.Printf("✅ [GetFileCacheContainingNode] 从 node_ids 字段找到节点 (CacheID=%d)", c.ID)
			return &c, nil
		}

		// 2. 如果 node_ids 为空，从 file_data 中检查（慢速路径）
		if c.NodeIDs == "" && c.FileData != "" {
			log.Printf("  [%d/%d] node_ids 为空，尝试从 file_data 查找...", i+1, len(caches))
			// 尝试从 file_data 中查找该节点
			var data map[string]interface{}
			if err := json.Unmarshal([]byte(c.FileData), &data); err == nil {
				if subtree := findNodeInTree(data, nodeID); subtree != nil {
					log.Printf("✅ [GetFileCacheContainingNode] 从 file_data 中找到节点 (CacheID=%d)", c.ID)
					return &c, nil
				}
			} else {
				log.Printf("  ⚠️ [%d/%d] 解析 file_data 失败: %v", i+1, len(caches), err)
			}
		}
	}

	log.Printf("❌ [GetFileCacheContainingNode] 未找到包含节点的缓存 (FileKey=%s, NodeID=%s)",
		fileKey, nodeID)
	return nil, gorm.ErrRecordNotFound
}
