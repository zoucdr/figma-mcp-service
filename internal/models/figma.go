package models

import (
	"time"
)

// FigmaProject Figma项目模型
type FigmaProject struct {
	ID         uint        `gorm:"primaryKey" json:"id"`
	UserID     uint        `gorm:"index" json:"user_id"`
	FileKey    string      `gorm:"size:100;not null" json:"file_key"`
	RootNodeID string      `gorm:"size:100" json:"root_node_id"`
	Name       string      `gorm:"size:255" json:"name"`
	FigmaURL   string      `gorm:"size:500" json:"figma_url"` // Figma设计图地址
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
	Nodes      []FigmaNode `gorm:"foreignKey:ProjectID" json:"nodes,omitempty"`
}

// 为FileKey、RootNodeID和UserID创建联合唯一索引，允许每个用户的每个文件有多个项目（不同RootNodeID）
func (FigmaProject) TableName() string {
	return "figma_projects"
}

func (FigmaProject) Indexes() [][]interface{} {
	return [][]interface{}{
		{"idx_user_file_node", "user_id", "file_key", "root_node_id"},
	}
}

// FigmaNode Figma节点模型
type FigmaNode struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ProjectID uint      `gorm:"index" json:"project_id"`
	NodeID    string    `gorm:"size:100;index;not null" json:"node_id"`
	Modifys   string    `gorm:"type:text" json:"modifys"` // 存储修改信息的大JSON
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NodeSetting 节点设置模型 - 不再使用，保留代码以供参考
// 节点的所有设置现在都存储在FigmaNode的Modifys字段中
/*
type NodeSetting struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	NodeID             string    `gorm:"size:100;uniqueIndex;not null" json:"node_id"`
	NodeType           string    `gorm:"size:50;default:'default'" json:"node_type"`       // 节点类型：default, button, scroll, etc.
	ImageDownloadType  string    `gorm:"size:50;default:'png'" json:"image_download_type"` // 图片下载类型：png, jpg, svg
	IsVisible          bool      `gorm:"default:true" json:"is_visible"`                   // 是否可见
	AnchorPoint        string    `gorm:"size:50;default:'center'" json:"anchor_point"`     // 锚点位置
	CustomName         string    `gorm:"size:255" json:"custom_name"`                      // 自定义节点名称
	ExcludedChildNodes string    `gorm:"type:text" json:"excluded_child_nodes"`            // 排除的子节点，以JSON数组存储
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}
*/

// ExportJob 导出任务模型
type ExportJob struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index" json:"user_id"`
	ProjectID uint      `gorm:"index" json:"project_id"`
	Status    string    `gorm:"size:50;default:'pending'" json:"status"` // pending, processing, completed, failed
	Progress  int       `gorm:"default:0" json:"progress"`               // 0-100
	FilePath  string    `gorm:"size:255" json:"file_path"`               // 导出文件路径
	Format    string    `gorm:"size:10;default:'png'" json:"format"`     // 导出图片格式：png, jpg, svg, pdf
	Error     string    `gorm:"type:text" json:"error"`                  // 错误信息
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GetProjectByFileKey 通过FileKey获取项目
// GetProjectByFileKey 通过文件键获取项目
func GetProjectByFileKey(fileKey string) (*FigmaProject, error) {
	var project FigmaProject
	result := DB.Where("file_key = ?", fileKey).First(&project)
	if result.Error != nil {
		return nil, result.Error
	}
	return &project, nil
}

// GetProjectByUserAndFileKey 通过用户ID和文件键获取项目
func GetProjectByUserAndFileKey(userID uint, fileKey string) (*FigmaProject, error) {
	var project FigmaProject
	result := DB.Where("user_id = ? AND file_key = ?", userID, fileKey).First(&project)
	if result.Error != nil {
		return nil, result.Error
	}
	return &project, nil
}

// GetProjectByUserFileKeyAndRootNodeID 通过用户ID、文件键和根节点ID获取项目
func GetProjectByUserFileKeyAndRootNodeID(userID uint, fileKey string, rootNodeID string) (*FigmaProject, error) {
	var project FigmaProject
	result := DB.Where("user_id = ? AND file_key = ? AND root_node_id = ?", userID, fileKey, rootNodeID).First(&project)
	if result.Error != nil {
		return nil, result.Error
	}
	return &project, nil
}

// CreateOrUpdateProject 创建或更新项目
func CreateOrUpdateProject(userID uint, fileKey, rootNodeID, name, figmaURL string) (*FigmaProject, error) {
	var project FigmaProject

	// 如果提供了rootNodeID，先尝试精确匹配
	if rootNodeID != "" {
		result := DB.Where("user_id = ? AND file_key = ? AND root_node_id = ?", userID, fileKey, rootNodeID).First(&project)
		if result.Error == nil {
			// 找到精确匹配，更新名称和URL
			project.Name = name
			project.FigmaURL = figmaURL
			DB.Save(&project)
			return &project, nil
		}
	}

	// 如果没有找到精确匹配，则创建新项目
	project = FigmaProject{
		UserID:     userID,
		FileKey:    fileKey,
		RootNodeID: rootNodeID,
		Name:       name,
		FigmaURL:   figmaURL,
	}
	result := DB.Create(&project)
	if result.Error != nil {
		return nil, result.Error
	}

	return &project, nil
}

// SaveNode 保存节点
func SaveNode(node *FigmaNode) error {
	var existingNode FigmaNode
	result := DB.Where("project_id = ? AND node_id = ?", node.ProjectID, node.NodeID).First(&existingNode)

	if result.Error != nil {
		// 创建新节点
		return DB.Create(node).Error
	}

	// 更新现有节点
	existingNode.Modifys = node.Modifys
	return DB.Save(&existingNode).Error
}

// FindNodeByProjectAndNodeID 通过项目ID和节点ID查找节点
func FindNodeByProjectAndNodeID(projectID uint, nodeID string) (*FigmaNode, error) {
	var node FigmaNode
	result := DB.Where("project_id = ? AND node_id = ?", projectID, nodeID).First(&node)
	if result.Error != nil {
		return nil, result.Error
	}
	return &node, nil
}

// GetNodeModifys 获取节点修改信息
func GetNodeModifys(projectID uint, nodeID string) (string, error) {
	var node FigmaNode
	result := DB.Where("project_id = ? AND node_id = ?", projectID, nodeID).First(&node)
	if result.Error != nil {
		return "", result.Error
	}
	return node.Modifys, nil
}

// GetProjectByID 通过ID获取项目
func GetProjectByID(projectID uint) (*FigmaProject, error) {
	var project FigmaProject
	result := DB.First(&project, projectID)
	if result.Error != nil {
		return nil, result.Error
	}
	return &project, nil
}

// DeleteProject 删除项目
func DeleteProject(projectID uint) error {
	// 删除项目关联的节点
	if err := DB.Where("project_id = ?", projectID).Delete(&FigmaNode{}).Error; err != nil {
		return err
	}

	// 删除项目
	return DB.Delete(&FigmaProject{}, projectID).Error
}

// GetExportJob 获取导出任务
func GetExportJob(jobID uint) (*ExportJob, error) {
	var job ExportJob
	result := DB.First(&job, jobID)
	if result.Error != nil {
		return nil, result.Error
	}
	return &job, nil
}

// SaveNodeModifys 保存节点修改信息
func SaveNodeModifys(projectID uint, nodeID string, modifys string) error {
	var node FigmaNode
	result := DB.Where("project_id = ? AND node_id = ?", projectID, nodeID).First(&node)

	if result.Error != nil {
		// 创建新节点
		node = FigmaNode{
			ProjectID: projectID,
			NodeID:    nodeID,
			Modifys:   modifys,
		}
		return DB.Create(&node).Error
	}

	// 更新现有节点
	node.Modifys = modifys
	return DB.Save(&node).Error
}

// DeleteNodeModifys 删除节点修改信息
func DeleteNodeModifys(projectID uint, nodeID string) error {
	return DB.Where("project_id = ? AND node_id = ?", projectID, nodeID).Delete(&FigmaNode{}).Error
}

// CreateExportJob 创建导出任务
func CreateExportJob(userID, projectID uint, path, format string) (*ExportJob, error) {
	job := ExportJob{
		UserID:    userID,
		ProjectID: projectID,
		Status:    "pending",
		Progress:  0,
	}

	result := DB.Create(&job)
	if result.Error != nil {
		return nil, result.Error
	}

	return &job, nil
}

// UpdateExportJobStatus 更新导出任务状态
func UpdateExportJobStatus(jobID uint, status string, progress int, filePath string, errorMsg string) error {
	return DB.Model(&ExportJob{}).Where("id = ?", jobID).Updates(map[string]interface{}{
		"status":    status,
		"progress":  progress,
		"file_path": filePath,
		"error":     errorMsg,
	}).Error
}
