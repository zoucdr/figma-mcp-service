package services

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/figma-deliver/internal/models"
)

// QueueService 队列服务
type QueueService struct {
	MaxNodesPerRequest uint // 每次请求最大节点数
	cacheService       *CacheService
	scheduler          SchedulerNotifier // 调度器接口（用于通知新队列创建）
	batchService       BatchUpdater      // 批次服务接口（用于更新批次进度）
}

// SchedulerNotifier 调度器通知接口
type SchedulerNotifier interface {
	NotifyNewQueue()
}

// BatchUpdater 批次更新接口
type BatchUpdater interface {
	OnQueueStatusChange(queueID uint) error
}

// NewQueueService 创建队列服务
func NewQueueService(maxNodesPerRequest uint, cacheService *CacheService) *QueueService {
	return &QueueService{
		MaxNodesPerRequest: maxNodesPerRequest,
		cacheService:       cacheService,
		scheduler:          nil, // 会在 scheduler 创建时通过 SetScheduler 设置
		batchService:       nil, // 会在 batchService 创建后通过 SetBatchService 设置
	}
}

// SetScheduler 设置调度器（用于通知新队列创建）
func (qs *QueueService) SetScheduler(scheduler SchedulerNotifier) {
	qs.scheduler = scheduler
}

// SetBatchService 设置批次服务（用于更新批次进度）
func (qs *QueueService) SetBatchService(batchService BatchUpdater) {
	qs.batchService = batchService
}

// ===================== 渲染队列管理 =====================

// CreateRenderQueue 创建渲染队列
// nodeIDs: 需要渲染的节点ID列表
// projectIDs: 关联的项目ID列表（可选）
// 返回: 创建的队列ID列表（可能拆分为多个队列）
func (qs *QueueService) CreateRenderQueue(token, fileKey string, nodeIDs []string, format string, scale float64, projectIDs ...uint) ([]uint, error) {
	if len(nodeIDs) == 0 {
		return nil, fmt.Errorf("节点列表为空")
	}

	// 检查是否有相同 fileKey + token 的 waiting 状态队列
	existingQueues, err := models.GetRenderQueuesByToken(token, []string{"waiting"})
	if err != nil {
		log.Printf("❌ [Queue] 查询已有队列失败: %v", err)
		return nil, err
	}

	// 查找匹配的队列（相同 fileKey, format, scale）
	var matchedQueue *models.FigmaRenderQueue
	for i := range existingQueues {
		if existingQueues[i].FileKey == fileKey &&
			existingQueues[i].Format == format &&
			existingQueues[i].Scale == scale {
			matchedQueue = &existingQueues[i]
			break
		}
	}

	// 提取实际的 projectIDs（如果有）
	var actualProjectIDs []uint
	if len(projectIDs) > 0 {
		actualProjectIDs = projectIDs
	}

	if matchedQueue != nil {
		// 合并到已有队列（同时合并 projectIDs）
		return qs.mergeToExistingQueue(matchedQueue, nodeIDs, actualProjectIDs)
	}

	// 创建新队列（按1000个节点拆分）
	return qs.createNewQueues(token, fileKey, nodeIDs, format, scale, actualProjectIDs)
}

// mergeToExistingQueue 合并节点和项目ID到已有队列
func (qs *QueueService) mergeToExistingQueue(queue *models.FigmaRenderQueue, newNodeIDs []string, newProjectIDs []uint) ([]uint, error) {
	// 解析已有节点ID
	existingNodeIDs := splitNodeIDsFromQueue(queue.NodeIDs)

	// 去重合并节点ID
	mergedNodeIDs := mergeAndDeduplicate(existingNodeIDs, newNodeIDs)

	// 检查是否有新节点加入
	addedCount := len(mergedNodeIDs) - len(existingNodeIDs)
	if addedCount == 0 {
		// 所有节点都已存在，但可能需要更新 projectIDs
		log.Printf("⚠️ [Queue] 所有节点已在队列中 id=%d, 无新增节点", queue.ID)
	} else {
		log.Printf("📝 [Queue] 准备合并节点到队列 id=%d, 原节点数=%d, 新增节点数=%d, 合并后总数=%d",
			queue.ID, len(existingNodeIDs), addedCount, len(mergedNodeIDs))
	}

	// 合并 projectIDs（去重）
	var existingProjectIDs []uint
	if queue.ProjectIDs != "" {
		if err := json.Unmarshal([]byte(queue.ProjectIDs), &existingProjectIDs); err != nil {
			log.Printf("⚠️ [Queue] 解析现有 ProjectIDs 失败: %v", err)
			existingProjectIDs = []uint{}
		}
	}

	// 去重合并 projectIDs
	projectIDMap := make(map[uint]bool)
	for _, pid := range existingProjectIDs {
		projectIDMap[pid] = true
	}

	addedProjectCount := 0
	for _, pid := range newProjectIDs {
		if !projectIDMap[pid] {
			existingProjectIDs = append(existingProjectIDs, pid)
			projectIDMap[pid] = true
			addedProjectCount++
		}
	}

	// 序列化合并后的 projectIDs
	var projectIDsJSON string
	if len(existingProjectIDs) > 0 {
		jsonBytes, err := json.Marshal(existingProjectIDs)
		if err != nil {
			log.Printf("❌ [Queue] 序列化 projectIDs 失败: %v", err)
			projectIDsJSON = queue.ProjectIDs // 保持原值
		} else {
			projectIDsJSON = string(jsonBytes)
		}
	}

	if addedProjectCount > 0 {
		log.Printf("📎 [Queue] 合并 ProjectIDs: 原有=%v, 新增=%d, 合并后=%v",
			existingProjectIDs[:len(existingProjectIDs)-addedProjectCount], addedProjectCount, existingProjectIDs)
	}

	// 如果超过1000个，需要拆分
	if len(mergedNodeIDs) <= int(qs.MaxNodesPerRequest) {
		// 更新现有队列
		queue.NodeIDs = joinNodeIDsForQueue(mergedNodeIDs)
		queue.TotalNodes = uint(len(mergedNodeIDs))
		queue.ProjectIDs = projectIDsJSON
		queue.UpdatedAt = uint32(time.Now().Unix())

		err := models.UpdateRenderQueue(queue)
		if err != nil {
			log.Printf("❌ [Queue] 更新队列失败 id=%d: %v", queue.ID, err)
			return nil, err
		}

		log.Printf("✅ [Queue] 已合并到现有队列 id=%d, 新增节点数=%d, 总节点数=%d, ProjectIDs=%v",
			queue.ID, addedCount, queue.TotalNodes, existingProjectIDs)

		// 通知调度器有新队列创建（合并也算）
		if qs.scheduler != nil {
			qs.scheduler.NotifyNewQueue()
		}

		return []uint{queue.ID}, nil
	}

	// 需要拆分：保留原队列的前1000个，其余创建新队列
	queue.NodeIDs = joinNodeIDsForQueue(mergedNodeIDs[:qs.MaxNodesPerRequest])
	queue.TotalNodes = uint(qs.MaxNodesPerRequest)
	queue.ProjectIDs = projectIDsJSON
	queue.UpdatedAt = uint32(time.Now().Unix())

	err := models.UpdateRenderQueue(queue)
	if err != nil {
		log.Printf("❌ [Queue] 更新队列失败 id=%d: %v", queue.ID, err)
		return nil, err
	}

	// 创建新队列存储剩余节点（不传递 projectIDs，因为已经在第一个队列中关联）
	remainingNodes := mergedNodeIDs[qs.MaxNodesPerRequest:]
	newQueueIDs, err := qs.createNewQueues(queue.FigmaToken, queue.FileKey, remainingNodes, queue.Format, queue.Scale, nil)
	if err != nil {
		return nil, err
	}

	allQueueIDs := append([]uint{queue.ID}, newQueueIDs...)

	log.Printf("✅ [Queue] 已合并并拆分队列，共 %d 个队列，总节点数=%d, ProjectIDs=%v",
		len(allQueueIDs), len(mergedNodeIDs), existingProjectIDs)

	return allQueueIDs, nil
}

// createNewQueues 创建新队列（按1000个节点拆分）
func (qs *QueueService) createNewQueues(token, fileKey string, nodeIDs []string, format string, scale float64, projectIDs []uint) ([]uint, error) {
	now := uint32(time.Now().Unix())
	queueIDs := []uint{}

	// 序列化 projectIDs 为 JSON 字符串
	var projectIDsJSON string
	if len(projectIDs) > 0 {
		jsonBytes, err := json.Marshal(projectIDs)
		if err != nil {
			log.Printf("❌ [Queue] 序列化 projectIDs 失败: %v", err)
			projectIDsJSON = ""
		} else {
			projectIDsJSON = string(jsonBytes)
		}
	}

	// 按最大节点数拆分
	for i := 0; i < len(nodeIDs); i += int(qs.MaxNodesPerRequest) {
		end := i + int(qs.MaxNodesPerRequest)
		if end > len(nodeIDs) {
			end = len(nodeIDs)
		}

		batch := nodeIDs[i:end]

		queue := &models.FigmaRenderQueue{
			FigmaToken:     token,
			FileKey:        fileKey,
			NodeIDs:        joinNodeIDsForQueue(batch),
			ProjectIDs:     projectIDsJSON,
			Format:         format,
			Scale:          scale,
			Status:         "waiting",
			Progress:       0,
			TotalNodes:     uint(len(batch)),
			ProcessedNodes: 0,
			FailedNodes:    0,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		err := models.CreateRenderQueue(queue)
		if err != nil {
			log.Printf("❌ [Queue] 创建队列失败: %v", err)
			return nil, err
		}

		queueIDs = append(queueIDs, queue.ID)

		log.Printf("🆕 [Queue] 已创建渲染队列 id=%d, nodes=%d, format=%s, scale=%.1f",
			queue.ID, len(batch), format, scale)
	}

	log.Printf("✅ [Queue] 共创建 %d 个渲染队列，总节点数=%d", len(queueIDs), len(nodeIDs))

	// 通知调度器有新队列创建
	if qs.scheduler != nil {
		qs.scheduler.NotifyNewQueue()
	}

	return queueIDs, nil
}

// GetWaitingQueues 获取所有等待中的队列
func (qs *QueueService) GetWaitingQueues() ([]models.FigmaRenderQueue, error) {
	queues, err := models.GetWaitingRenderQueues()
	if err != nil {
		log.Printf("❌ [Queue] 获取等待队列失败: %v", err)
		return nil, err
	}

	if len(queues) > 0 {
		log.Printf("📋 [Queue] 当前有 %d 个等待处理的队列", len(queues))
	}

	return queues, nil
}

// GetQueuesByToken 获取指定Token的队列
func (qs *QueueService) GetQueuesByToken(token string, statuses []string) ([]models.FigmaRenderQueue, error) {
	queues, err := models.GetRenderQueuesByToken(token, statuses)
	if err != nil {
		log.Printf("❌ [Queue] 获取Token队列失败: %v", err)
		return nil, err
	}

	log.Printf("📋 [Queue] Token [%s...] 有 %d 个队列（状态：%v）",
		token[:10], len(queues), statuses)

	return queues, nil
}

// UpdateQueueStatus 更新队列状态
func (qs *QueueService) UpdateQueueStatus(queueID uint, status string, progress uint) error {
	err := models.UpdateRenderQueueStatus(queueID, status, progress)
	if err != nil {
		log.Printf("❌ [Queue] 更新队列状态失败 id=%d: %v", queueID, err)
		return err
	}

	log.Printf("📝 [Queue] 状态更新 id=%d, status=%s, progress=%d%%",
		queueID, status, progress)

	// 通知批次服务更新批次进度
	if qs.batchService != nil {
		if err := qs.batchService.OnQueueStatusChange(queueID); err != nil {
			log.Printf("⚠️ [Queue] 通知批次更新失败 id=%d: %v", queueID, err)
			// 不中断流程，只记录日志
		}
	}

	return nil
}

// UpdateQueueStatusWithError 更新队列状态并设置错误消息
func (qs *QueueService) UpdateQueueStatusWithError(queueID uint, status string, errorMessage string) error {
	now := uint32(time.Now().Unix())

	updates := map[string]interface{}{
		"status":        status,
		"error_message": errorMessage,
		"updated_at":    now,
	}

	// 如果是失败或完成状态，设置完成时间
	if status == "failed" || status == "completed" || status == "partial" {
		updates["completed_at"] = now
	}

	err := models.DB.Model(&models.FigmaRenderQueue{}).
		Where("id = ?", queueID).
		Updates(updates).Error

	if err != nil {
		log.Printf("❌ [Queue] 更新队列状态失败 id=%d: %v", queueID, err)
		return err
	}

	log.Printf("📝 [Queue] 状态更新 id=%d, status=%s, error=%s",
		queueID, status, errorMessage)

	// 通知批次服务更新批次进度
	if qs.batchService != nil {
		if err := qs.batchService.OnQueueStatusChange(queueID); err != nil {
			log.Printf("⚠️ [Queue] 通知批次更新失败 id=%d: %v", queueID, err)
			// 不中断流程，只记录日志
		}
	}

	return nil
}

// UpdateQueueProgress 更新队列进度
func (qs *QueueService) UpdateQueueProgress(queueID uint, processed, failed uint) error {
	err := models.UpdateRenderQueueProgress(queueID, processed, failed)
	if err != nil {
		log.Printf("❌ [Queue] 更新队列进度失败 id=%d: %v", queueID, err)
		return err
	}

	log.Printf("📊 [Queue] 进度更新 id=%d, processed=%d, failed=%d",
		queueID, processed, failed)

	return nil
}

// CalculateQueueProgress 计算队列进度百分比
func (qs *QueueService) CalculateQueueProgress(queue *models.FigmaRenderQueue) uint {
	if queue.TotalNodes == 0 {
		return 0
	}

	progress := (queue.ProcessedNodes * 100) / queue.TotalNodes
	if progress > 100 {
		progress = 100
	}

	return progress
}

// GetQueueStatistics 获取队列统计信息
func (qs *QueueService) GetQueueStatistics(token string) (map[string]interface{}, error) {
	queues, err := models.GetRenderQueuesByToken(token, []string{"waiting", "processing"})
	if err != nil {
		return nil, err
	}

	stats := map[string]interface{}{
		"total_queues":     len(queues),
		"waiting_count":    0,
		"processing_count": 0,
		"total_nodes":      0,
		"processed_nodes":  0,
		"overall_progress": 0,
	}

	totalNodes := 0
	processedNodes := 0

	for _, queue := range queues {
		if queue.Status == "waiting" {
			stats["waiting_count"] = stats["waiting_count"].(int) + 1
		} else if queue.Status == "processing" {
			stats["processing_count"] = stats["processing_count"].(int) + 1
		}

		totalNodes += int(queue.TotalNodes)
		processedNodes += int(queue.ProcessedNodes)
	}

	stats["total_nodes"] = totalNodes
	stats["processed_nodes"] = processedNodes

	if totalNodes > 0 {
		stats["overall_progress"] = (processedNodes * 100) / totalNodes
	}

	return stats, nil
}

// ===================== 辅助函数 =====================

// splitNodeIDsFromQueue 从队列的字符串中分割节点ID
func splitNodeIDsFromQueue(nodeIDsStr string) []string {
	if nodeIDsStr == "" {
		return []string{}
	}
	return strings.Split(nodeIDsStr, ",")
}

// joinNodeIDsForQueue 将节点ID列表合并为字符串
func joinNodeIDsForQueue(nodeIDs []string) string {
	return strings.Join(nodeIDs, ",")
}

// mergeAndDeduplicate 合并并去重节点ID列表
func mergeAndDeduplicate(list1, list2 []string) []string {
	seen := make(map[string]bool)
	result := []string{}

	// 添加第一个列表
	for _, id := range list1 {
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}

	// 添加第二个列表
	for _, id := range list2 {
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}

	return result
}

// UpdateRenderQueue 更新队列信息（完整更新）
func UpdateRenderQueue(queue *models.FigmaRenderQueue) error {
	return models.UpdateRenderQueue(queue)
}

// GetRenderQueue 获取队列详情
func GetRenderQueue(queueID uint) (*models.FigmaRenderQueue, error) {
	return models.GetRenderQueue(queueID)
}
