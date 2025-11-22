package models

import (
	"encoding/json"
	"time"
)

// FigmaRenderBatch 渲染任务组模型（批次不限制具体渲染参数，由队列决定）
type FigmaRenderBatch struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	UserID          uint       `gorm:"index" json:"user_id"`
	ProjectIDs      string     `gorm:"type:text" json:"project_ids"` // JSON数组: [1,2,3]
	FileKey         string     `gorm:"size:100;index" json:"file_key"`
	Status          string     `gorm:"size:20;index;default:'waiting'" json:"status"`
	TotalQueues     int        `gorm:"default:0" json:"total_queues"`
	CompletedQueues int        `gorm:"default:0" json:"completed_queues"`
	TotalNodes      int        `gorm:"default:0" json:"total_nodes"`
	ProcessedNodes  int        `gorm:"default:0" json:"processed_nodes"`
	FailedNodes     int        `gorm:"default:0" json:"failed_nodes"`
	Progress        int        `gorm:"default:0" json:"progress"` // 0-100
	ErrorMessage    string     `gorm:"type:text" json:"error_message"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
}

// QueueBatchRelation 队列-批次关联模型
type QueueBatchRelation struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	QueueID   uint      `gorm:"index;not null" json:"queue_id"`
	BatchID   uint      `gorm:"index;not null" json:"batch_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (FigmaRenderBatch) TableName() string {
	return "figma_render_batches"
}

func (QueueBatchRelation) TableName() string {
	return "figma_queue_batch_relations"
}

// GetProjectIDs 解析项目ID列表
func (b *FigmaRenderBatch) GetProjectIDs() ([]uint, error) {
	var ids []uint
	if b.ProjectIDs == "" || b.ProjectIDs == "[]" {
		return ids, nil
	}
	if err := json.Unmarshal([]byte(b.ProjectIDs), &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

// SetProjectIDs 设置项目ID列表
func (b *FigmaRenderBatch) SetProjectIDs(ids []uint) error {
	data, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	b.ProjectIDs = string(data)
	return nil
}

// CalculateProgress 计算进度（包含失败节点）
func (b *FigmaRenderBatch) CalculateProgress() {
	if b.TotalNodes > 0 {
		// 进度 = (成功节点 + 失败节点) / 总节点 * 100
		// 失败的节点也算作已处理
		completedNodes := b.ProcessedNodes + b.FailedNodes
		b.Progress = (completedNodes * 100) / b.TotalNodes
		if b.Progress > 100 {
			b.Progress = 100
		}
	} else {
		b.Progress = 0
	}
}

// UpdateStatus 更新状态（根据队列完成情况）
func (b *FigmaRenderBatch) UpdateStatus() {
	if b.CompletedQueues == 0 {
		if b.Status != "cancelled" && b.Status != "failed" {
			b.Status = "waiting"
		}
	} else if b.CompletedQueues < b.TotalQueues {
		b.Status = "processing"
	} else {
		// 所有队列都完成了
		if b.FailedNodes == 0 {
			b.Status = "completed"
			now := time.Now()
			b.CompletedAt = &now
		} else if b.ProcessedNodes > 0 {
			b.Status = "partial"
			now := time.Now()
			b.CompletedAt = &now
		} else {
			b.Status = "failed"
			now := time.Now()
			b.CompletedAt = &now
		}
	}
}

// GetBatchByID 根据ID获取批次
func GetBatchByID(id uint) (*FigmaRenderBatch, error) {
	var batch FigmaRenderBatch
	if err := DB.First(&batch, id).Error; err != nil {
		return nil, err
	}
	return &batch, nil
}

// GetActiveBatchByProject 获取项目的活跃批次
func GetActiveBatchByProject(projectID uint) (*FigmaRenderBatch, error) {
	var project FigmaProject
	if err := DB.First(&project, projectID).Error; err != nil {
		return nil, err
	}

	if project.RenderID == 0 {
		return nil, nil
	}

	return GetBatchByID(project.RenderID)
}

// GetQueuesByBatch 获取批次关联的所有队列ID
func GetQueuesByBatch(batchID uint) ([]uint, error) {
	var relations []QueueBatchRelation
	if err := DB.Where("batch_id = ?", batchID).Find(&relations).Error; err != nil {
		return nil, err
	}

	queueIDs := make([]uint, len(relations))
	for i, rel := range relations {
		queueIDs[i] = rel.QueueID
	}
	return queueIDs, nil
}

// GetBatchByQueue 根据队列ID获取关联的批次
func GetBatchByQueue(queueID uint) (*FigmaRenderBatch, error) {
	var relation QueueBatchRelation
	if err := DB.Where("queue_id = ?", queueID).First(&relation).Error; err != nil {
		return nil, err
	}

	return GetBatchByID(relation.BatchID)
}

// CreateBatchQueueRelation 创建队列-批次关联
func CreateBatchQueueRelation(queueID, batchID uint) error {
	relation := &QueueBatchRelation{
		QueueID: queueID,
		BatchID: batchID,
	}
	return DB.Create(relation).Error
}
