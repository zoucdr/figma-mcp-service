package services

import (
	"fmt"
	"log"
	"time"

	"github.com/figma-deliver/internal/models"
)

// RenderBatchService 渲染批次服务
type RenderBatchService struct {
	queueService *QueueService
}

// NewRenderBatchService 创建渲染批次服务实例
func NewRenderBatchService(queueService *QueueService) *RenderBatchService {
	return &RenderBatchService{
		queueService: queueService,
	}
}

// CreateRenderBatch 创建渲染批次（支持智能复用）
func (s *RenderBatchService) CreateRenderBatch(
	userID uint,
	projectIDs []uint,
	fileKey string,
	nodeIDs []string,
	format string,
	scale float64,
	token string,
) (*models.FigmaRenderBatch, error) {

	log.Printf("📦 [RenderBatch] 创建渲染批次: UserID=%d, ProjectIDs=%v, FileKey=%s, Nodes=%d, Format=%s, Scale=%.1f",
		userID, projectIDs, fileKey, len(nodeIDs), format, scale)

	var batch *models.FigmaRenderBatch
	var isNewBatch bool

	// 0. 检查第一个项目是否已有活跃的渲染批次（waiting/processing）
	// 如果有，则追加队列到该批次；如果没有，创建新批次
	if len(projectIDs) > 0 {
		var project models.FigmaProject
		if err := models.DB.First(&project, projectIDs[0]).Error; err == nil {
			if project.RenderID > 0 {
				// 项目已有活跃批次，检查批次状态
				var existingBatch models.FigmaRenderBatch
				if err := models.DB.First(&existingBatch, project.RenderID).Error; err == nil {
					if existingBatch.Status == "waiting" || existingBatch.Status == "processing" {
						log.Printf("🔄 [RenderBatch] 项目 %d 已有活跃渲染批次 #%d (状态: %s)，追加新队列到该批次",
							projectIDs[0], existingBatch.ID, existingBatch.Status)

						batch = &existingBatch
						isNewBatch = false

						// 确保所有项目都加入这个批次
						existingProjectIDs, _ := batch.GetProjectIDs()
						mergedProjectIDs := mergeProjectIDs(existingProjectIDs, projectIDs)

						if err := batch.SetProjectIDs(mergedProjectIDs); err != nil {
							log.Printf("⚠️ [RenderBatch] 更新批次项目列表失败: %v", err)
						}

						// 更新所有项目的 render_id
						for _, pid := range projectIDs {
							models.DB.Model(&models.FigmaProject{}).
								Where("id = ?", pid).
								Update("render_id", batch.ID)
						}
					}
				}
			}
		}
	}

	// 1. 如果没有找到活跃批次，创建新批次记录
	if batch == nil {
		batch = &models.FigmaRenderBatch{
			UserID:     userID,
			FileKey:    fileKey,
			Status:     "waiting",
			TotalNodes: 0, // 稍后累加
		}

		if err := batch.SetProjectIDs(projectIDs); err != nil {
			return nil, fmt.Errorf("设置项目ID失败: %v", err)
		}

		if err := models.DB.Create(batch).Error; err != nil {
			return nil, fmt.Errorf("创建批次记录失败: %v", err)
		}

		isNewBatch = true
		log.Printf("✅ [RenderBatch] 新批次记录已创建: BatchID=%d", batch.ID)

		// 更新所有项目的 render_id
		for _, projectID := range projectIDs {
			models.DB.Model(&models.FigmaProject{}).
				Where("id = ?", projectID).
				Update("render_id", batch.ID)
		}
	}

	// 2. 获取批次中已有的队列，进行节点去重检查
	existingQueueIDs, err := models.GetQueuesByBatch(batch.ID)
	var existingQueues []models.FigmaRenderQueue

	if err == nil && len(existingQueueIDs) > 0 {
		// 查询批次中所有队列的详情
		models.DB.Where("id IN ?", existingQueueIDs).Find(&existingQueues)
	}

	// 过滤掉已存在的节点（相同 nodeID + format + scale）
	uniqueNodeIDs := filterUniqueNodes(nodeIDs, existingQueues, format, scale)

	if len(uniqueNodeIDs) == 0 {
		log.Printf("ℹ️ [RenderBatch] 所有节点已存在于批次 #%d 的队列中，无需创建新队列", batch.ID)
		return batch, nil
	}

	log.Printf("🔍 [RenderBatch] 去重完成: 原节点数=%d, 已存在=%d, 待添加=%d",
		len(nodeIDs), len(nodeIDs)-len(uniqueNodeIDs), len(uniqueNodeIDs))

	// 3. 创建多个渲染队列（按配置的上限拆分，比如80个节点/批）
	var firstProjectID uint
	if len(projectIDs) > 0 {
		firstProjectID = projectIDs[0]
	}

	queueIDs, err := s.queueService.CreateRenderQueue(
		token, fileKey, uniqueNodeIDs, format, scale, firstProjectID,
	)
	if err != nil {
		// 创建队列失败
		if isNewBatch {
			models.DB.Delete(batch) // 只删除新创建的批次
		}
		return nil, fmt.Errorf("创建渲染队列失败: %v", err)
	}

	log.Printf("📋 [RenderBatch] 创建了 %d 个渲染队列: %v", len(queueIDs), queueIDs)

	// 4. 建立队列-批次关联
	for _, queueID := range queueIDs {
		if err := models.CreateBatchQueueRelation(queueID, batch.ID); err != nil {
			log.Printf("⚠️ [RenderBatch] 创建队列-批次关联失败: QueueID=%d, Error=%v", queueID, err)
		}
	}

	// 5. 更新批次统计（累加）
	batch.TotalQueues += len(queueIDs)
	batch.TotalNodes += len(uniqueNodeIDs)

	if err := models.DB.Save(batch).Error; err != nil {
		log.Printf("⚠️ [RenderBatch] 保存批次统计失败: %v", err)
	}

	log.Printf("🔗 [RenderBatch] 队列-批次关联已建立")

	if isNewBatch {
		log.Printf("✅ [RenderBatch] 新批次创建完成: BatchID=%d, TotalQueues=%d, TotalNodes=%d",
			batch.ID, batch.TotalQueues, batch.TotalNodes)
	} else {
		log.Printf("✅ [RenderBatch] 队列已追加到批次: BatchID=%d, 新增队列=%d, 新增节点=%d, 累计队列=%d, 累计节点=%d",
			batch.ID, len(queueIDs), len(uniqueNodeIDs), batch.TotalQueues, batch.TotalNodes)
	}

	return batch, nil
}

// GetBatchProgress 获取批次进度（聚合所有队列）
func (s *RenderBatchService) GetBatchProgress(batchID uint) (*models.FigmaRenderBatch, error) {
	var batch models.FigmaRenderBatch
	if err := models.DB.First(&batch, batchID).Error; err != nil {
		return nil, err
	}

	// 查询关联的所有队列ID
	queueIDs, err := models.GetQueuesByBatch(batchID)
	if err != nil {
		return nil, fmt.Errorf("获取队列列表失败: %v", err)
	}

	if len(queueIDs) == 0 {
		return &batch, nil
	}

	// 查询所有队列的状态
	var queues []models.FigmaRenderQueue
	if err := models.DB.Where("id IN ?", queueIDs).Find(&queues).Error; err != nil {
		return nil, fmt.Errorf("查询队列状态失败: %v", err)
	}

	// 聚合队列进度
	totalProcessed := 0
	totalFailed := 0
	completedCount := 0
	hasProcessing := false
	hasWaiting := false
	hasFailed := false

	for _, queue := range queues {
		totalProcessed += int(queue.ProcessedNodes)
		totalFailed += int(queue.FailedNodes)

		switch queue.Status {
		case "completed", "partial":
			completedCount++
		case "processing":
			hasProcessing = true
		case "waiting":
			hasWaiting = true
		case "failed", "error":
			hasFailed = true
			completedCount++ // 失败的队列也算作已完成的队列
		}
	}

	// 更新批次统计
	batch.ProcessedNodes = totalProcessed
	batch.FailedNodes = totalFailed
	batch.CompletedQueues = completedCount

	// 计算进度
	batch.CalculateProgress()

	// 更新状态
	if hasProcessing || (hasWaiting && completedCount > 0) {
		batch.Status = "processing"
		if batch.StartedAt == nil {
			now := time.Now()
			batch.StartedAt = &now
		}
	} else {
		batch.UpdateStatus()
	}

	// 如果有失败的队列，记录到 ErrorMessage 中
	if hasFailed {
		failedQueueCount := 0
		for _, queue := range queues {
			if queue.Status == "failed" || queue.Status == "error" {
				failedQueueCount++
			}
		}
		if batch.ErrorMessage == "" {
			batch.ErrorMessage = fmt.Sprintf("有 %d 个渲染队列失败", failedQueueCount)
		}
		log.Printf("⚠️ [RenderBatch] 批次 #%d 包含失败队列: %d/%d", batch.ID, failedQueueCount, len(queues))
	}

	// 保存更新
	if err := models.DB.Save(&batch).Error; err != nil {
		log.Printf("⚠️ [RenderBatch] 保存批次进度失败: %v", err)
	}

	return &batch, nil
}

// GetProjectBatchProgress 获取项目的渲染进度
func (s *RenderBatchService) GetProjectBatchProgress(projectID uint) (*models.FigmaRenderBatch, error) {
	var project models.FigmaProject
	if err := models.DB.First(&project, projectID).Error; err != nil {
		return nil, err
	}

	if project.RenderID == 0 {
		return nil, nil // 没有活跃的渲染批次
	}

	return s.GetBatchProgress(project.RenderID)
}

// CancelBatchForProject 取消项目的渲染批次（直接删除批次和队列数据）
func (s *RenderBatchService) CancelBatchForProject(projectID uint) error {
	log.Printf("🚫 [RenderBatch] 取消项目渲染批次: ProjectID=%d", projectID)

	// 1. 获取项目的活跃批次
	var project models.FigmaProject
	if err := models.DB.First(&project, projectID).Error; err != nil {
		return err
	}

	if project.RenderID == 0 {
		log.Printf("ℹ️ [RenderBatch] 项目没有活跃渲染批次: ProjectID=%d", projectID)
		return nil // 没有活跃渲染
	}

	batchID := project.RenderID

	// 2. 获取批次信息
	var batch models.FigmaRenderBatch
	if err := models.DB.First(&batch, batchID).Error; err != nil {
		return err
	}

	log.Printf("🗑️ [RenderBatch] 删除渲染批次及其队列: BatchID=%d, Status=%s", batchID, batch.Status)

	// 3. 获取并删除所有关联的队列
	queueIDs, err := models.GetQueuesByBatch(batchID)
	if err != nil {
		log.Printf("⚠️ [RenderBatch] 获取队列列表失败: %v", err)
	} else {
		log.Printf("📋 [RenderBatch] 批次关联的队列数: %d", len(queueIDs))

		// 删除所有队列
		deletedCount := 0
		for _, queueID := range queueIDs {
			if err := models.DB.Delete(&models.FigmaRenderQueue{}, queueID).Error; err != nil {
				log.Printf("⚠️ [RenderBatch] 删除队列失败: QueueID=%d, Error=%v", queueID, err)
			} else {
				deletedCount++
				log.Printf("🗑️ [RenderBatch] 队列已删除: QueueID=%d", queueID)
			}
		}

		log.Printf("✅ [RenderBatch] 已删除 %d 个队列", deletedCount)
	}

	// 4. 删除队列-批次关联关系
	if err := models.DB.Where("batch_id = ?", batchID).Delete(&models.QueueBatchRelation{}).Error; err != nil {
		log.Printf("⚠️ [RenderBatch] 删除队列-批次关联失败: %v", err)
	} else {
		log.Printf("🗑️ [RenderBatch] 队列-批次关联已删除")
	}

	// 5. 清除批次关联的所有项目的 render_id
	projectIDs, err := batch.GetProjectIDs()
	if err != nil {
		log.Printf("⚠️ [RenderBatch] 解析项目ID列表失败: %v", err)
		// 继续执行，至少清除当前项目
		projectIDs = []uint{projectID}
	}

	log.Printf("🔄 [RenderBatch] 清除 %d 个项目的渲染批次ID", len(projectIDs))

	for _, pid := range projectIDs {
		if err := models.DB.Model(&models.FigmaProject{}).
			Where("id = ?", pid).
			Update("render_id", 0).Error; err != nil {
			log.Printf("⚠️ [RenderBatch] 清除项目渲染批次ID失败: ProjectID=%d, Error=%v", pid, err)
		} else {
			log.Printf("✅ [RenderBatch] 已清除项目 %d 的渲染批次ID", pid)
		}
	}

	// 6. 删除批次记录
	if err := models.DB.Delete(&batch).Error; err != nil {
		log.Printf("⚠️ [RenderBatch] 删除批次记录失败: %v", err)
		return err
	}

	log.Printf("✅ [RenderBatch] 渲染批次已完全删除: BatchID=%d, 影响项目数=%d", batchID, len(projectIDs))

	return nil
}

// CleanupCompletedBatches 清理完成的批次（定期任务）
func (s *RenderBatchService) CleanupCompletedBatches(daysOld int) error {
	cutoffTime := time.Now().AddDate(0, 0, -daysOld)

	log.Printf("🧹 [RenderBatch] 清理完成的批次: 早于 %s", cutoffTime.Format("2006-01-02"))

	result := models.DB.Where("status IN ? AND updated_at < ?",
		[]string{"completed", "failed", "cancelled"}, cutoffTime).
		Delete(&models.FigmaRenderBatch{})

	if result.Error != nil {
		return result.Error
	}

	log.Printf("✅ [RenderBatch] 清理了 %d 个旧批次", result.RowsAffected)

	return nil
}

// OnQueueStatusChange 队列状态变化时的回调（用于更新批次进度）
func (s *RenderBatchService) OnQueueStatusChange(queueID uint) error {
	// 查找队列关联的批次
	batch, err := models.GetBatchByQueue(queueID)
	if err != nil {
		// 队列可能不属于任何批次（旧数据）
		return nil
	}

	// 刷新批次进度
	_, err = s.GetBatchProgress(batch.ID)
	return err
}

// ==================== 辅助函数 ====================

// findReusableBatch 查找可复用的批次
// 条件：相同用户、文件、节点、格式、缩放，且状态为 waiting/processing
// mergeProjectIDs 合并项目ID列表（去重）
func mergeProjectIDs(existing []uint, new []uint) []uint {
	idMap := make(map[uint]bool)

	// 添加已有的ID
	for _, id := range existing {
		idMap[id] = true
	}

	// 添加新的ID
	for _, id := range new {
		idMap[id] = true
	}

	// 转换回切片
	result := make([]uint, 0, len(idMap))
	for id := range idMap {
		result = append(result, id)
	}

	return result
}

// filterUniqueNodes 过滤掉已存在于队列中的节点
// 检查条件：nodeID、format、scale 全部相同才算重复
func filterUniqueNodes(nodeIDs []string, existingQueues []models.FigmaRenderQueue, format string, scale float64) []string {
	// 构建已存在节点的集合（key = nodeID:format:scale）
	existingNodeSet := make(map[string]bool)

	for _, queue := range existingQueues {
		// 只检查 waiting 和 processing 状态的队列
		if queue.Status != "waiting" && queue.Status != "processing" {
			continue
		}

		// 检查格式和缩放是否匹配
		if queue.Format != format || queue.Scale != scale {
			continue
		}

		// 解析队列中的节点ID列表
		queueNodeIDs, err := queue.GetNodeIDs()
		if err != nil {
			log.Printf("⚠️ [FilterNodes] 解析队列 #%d 节点列表失败: %v", queue.ID, err)
			continue
		}

		// 添加到已存在集合
		for _, nodeID := range queueNodeIDs {
			key := fmt.Sprintf("%s:%s:%.2f", nodeID, format, scale)
			existingNodeSet[key] = true
		}
	}

	// 过滤出不存在的节点
	uniqueNodes := make([]string, 0, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		key := fmt.Sprintf("%s:%s:%.2f", nodeID, format, scale)
		if !existingNodeSet[key] {
			uniqueNodes = append(uniqueNodes, nodeID)
		}
	}

	return uniqueNodes
}
