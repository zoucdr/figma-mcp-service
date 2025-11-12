package models

import (
	"encoding/json"
	"fmt"
	"time"
)

// FigmaProject Figma项目模型
type FigmaProject struct {
	ID                   uint        `gorm:"primaryKey" json:"id"`
	UserID               uint        `gorm:"index" json:"user_id"`
	FileKey              string      `gorm:"size:100;not null" json:"file_key"`
	RootNodeID           string      `gorm:"size:100" json:"root_node_id"`
	Name                 string      `gorm:"size:255" json:"name"`
	GroupName            string      `gorm:"size:255" json:"group_name"`             // 分组名称
	FigmaURL             string      `gorm:"size:500" json:"figma_url"`              // Figma设计图地址
	RefNodes             string      `gorm:"type:text" json:"ref_nodes"`             // 依赖节点ID列表，JSON格式存储
	Settings             string      `gorm:"type:text" json:"settings"`              // 项目配置信息，JSON格式存储
	InterfaceDescription string      `gorm:"type:text" json:"interface_description"` // 界面描述信息，用于MCP提示词
	CreatedAt            time.Time   `json:"created_at"`
	UpdatedAt            time.Time   `json:"updated_at"`
	Nodes                []FigmaNode `gorm:"foreignKey:ProjectID" json:"nodes,omitempty"`
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

// ExportJob 导出任务模型
type ExportJob struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index" json:"user_id"`
	ProjectID uint      `gorm:"index" json:"project_id"`
	Status    string    `gorm:"size:50;default:'pending'" json:"status"` // pending, processing, completed, failed
	Progress  int       `gorm:"default:0" json:"progress"`               // 0-100
	FilePath  string    `gorm:"size:255" json:"file_path"`               // 导出文件路径
	Format    string    `gorm:"size:10;default:'png'" json:"format"`     // 导出图片格式：png, jpg, svg, pdf
	Scale     float64   `gorm:"default:1.0" json:"scale"`                // 图片缩放比例：0.5, 1.0, 2.0, 3.0, 4.0
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

// removeDefaultValues 移除默认值字段，减少存储空间
func removeDefaultValues(modifys map[string]interface{}) map[string]interface{} {
	cleaned := make(map[string]interface{})

	for key, value := range modifys {
		shouldRemove := false

		switch key {
		case "ignore":
			if boolVal, ok := value.(bool); ok && !boolVal {
				shouldRemove = true
			}
		case "components":
			// 处理不同类型的空数组
			if arr, ok := value.([]interface{}); ok && len(arr) == 0 {
				shouldRemove = true
			} else if arr, ok := value.([]string); ok && len(arr) == 0 {
				shouldRemove = true
			} else if value == nil {
				shouldRemove = true
			}
		case "horizontal":
			if strVal, ok := value.(string); ok && strVal == "CENTER" {
				shouldRemove = true
			}
		case "vertical":
			if strVal, ok := value.(string); ok && strVal == "CENTER" {
				shouldRemove = true
			}
		case "img_ext":
			if strVal, ok := value.(string); ok && strVal == "png" {
				shouldRemove = true
			}
		case "img_name":
			if strVal, ok := value.(string); ok && strVal == "" {
				shouldRemove = true
			}
		case "rename":
			if strVal, ok := value.(string); ok && strVal == "" {
				shouldRemove = true
			}
		case "res_mode":
			if strVal, ok := value.(string); ok && strVal == "attach" {
				shouldRemove = true
			}
		}

		if !shouldRemove {
			cleaned[key] = value
		}
	}

	return cleaned
}

// SaveNodeModifys 保存节点修改信息
func SaveNodeModifys(projectID uint, nodeID string, modifys string) error {
	// 解析JSON字符串
	var modifysMap map[string]interface{}
	if modifys != "" {
		if err := json.Unmarshal([]byte(modifys), &modifysMap); err != nil {
			// 如果解析失败，直接保存原始字符串
			modifysMap = nil
		} else {
			// 清理默认值
			modifysMap = removeDefaultValues(modifysMap)
			// 如果清理后为空，则删除节点记录
			if len(modifysMap) == 0 {
				return DeleteNodeModifys(projectID, nodeID)
			}
			// 重新序列化
			if cleanedJSON, err := json.Marshal(modifysMap); err == nil {
				modifys = string(cleanedJSON)
			}
		}
	} else {
		// 如果 modifys 为空字符串，删除节点记录
		return DeleteNodeModifys(projectID, nodeID)
	}

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

// SaveNodeModifysWithMerge 保存节点修改信息（支持数据合并）
func SaveNodeModifysWithMerge(projectID uint, nodeID string, newModifys map[string]interface{}) error {
	var node FigmaNode
	result := DB.Where("project_id = ? AND node_id = ?", projectID, nodeID).First(&node)

	var finalModifys map[string]interface{}

	// 如果 newModifys 为 nil，初始化为空 map
	if newModifys == nil {
		newModifys = make(map[string]interface{})
	}

	if result.Error != nil {
		// 节点不存在，直接使用新的修改信息
		finalModifys = newModifys
	} else {
		// 节点存在，需要合并现有数据
		var existingModifys map[string]interface{}

		if node.Modifys != "" {
			// 解析现有的修改信息
			if err := json.Unmarshal([]byte(node.Modifys), &existingModifys); err != nil {
				// 如果解析失败，使用新的修改信息
				finalModifys = newModifys
			} else {
				// 合并数据：新数据覆盖现有数据
				finalModifys = make(map[string]interface{})

				// 先复制现有数据
				for key, value := range existingModifys {
					finalModifys[key] = value
				}

				// 再覆盖新数据
				for key, value := range newModifys {
					finalModifys[key] = value
				}
			}
		} else {
			// 现有节点没有修改信息，直接使用新数据
			finalModifys = newModifys
		}
	}

	// 清理默认值
	finalModifys = removeDefaultValues(finalModifys)

	// 如果清理后为空，则删除节点记录
	if len(finalModifys) == 0 {
		if result.Error != nil {
			// 节点不存在，无需删除
			return nil
		}
		// 删除现有节点记录
		return DeleteNodeModifys(projectID, nodeID)
	}

	// 将合并后的数据转换为JSON字符串
	modifysJSON, err := json.Marshal(finalModifys)
	if err != nil {
		return fmt.Errorf("序列化修改信息失败: %v", err)
	}

	if result.Error != nil {
		// 创建新节点
		node = FigmaNode{
			ProjectID: projectID,
			NodeID:    nodeID,
			Modifys:   string(modifysJSON),
		}
		return DB.Create(&node).Error
	} else {
		// 更新现有节点
		node.Modifys = string(modifysJSON)
		return DB.Save(&node).Error
	}
}

// DeleteNodeModifys 删除节点修改信息
func DeleteNodeModifys(projectID uint, nodeID string) error {
	return DB.Where("project_id = ? AND node_id = ?", projectID, nodeID).Delete(&FigmaNode{}).Error
}

// DeleteAllProjectNodeModifys 删除项目的所有节点修改信息
func DeleteAllProjectNodeModifys(projectID uint) error {
	return DB.Where("project_id = ?", projectID).Delete(&FigmaNode{}).Error
}

// DeleteNodeSubtreeModifys 删除指定节点及其子树的修改信息
// nodeIDs: 要删除的节点ID列表（包括子节点）
func DeleteNodeSubtreeModifys(projectID uint, nodeIDs []string) error {
	if len(nodeIDs) == 0 {
		return nil
	}

	// 使用IN查询删除多个节点的修改信息
	return DB.Where("project_id = ? AND node_id IN ?", projectID, nodeIDs).Delete(&FigmaNode{}).Error
}

// DeleteNodeSubtreeModifysAndReturnDeleted 删除指定节点及其子树的修改信息，并返回实际被删除的节点ID列表
// nodeIDs: 要删除的节点ID列表（包括子节点）
// 返回: 实际被删除的节点ID列表（只包含有修改记录的节点）
func DeleteNodeSubtreeModifysAndReturnDeleted(projectID uint, nodeIDs []string) ([]string, error) {
	if len(nodeIDs) == 0 {
		return []string{}, nil
	}

	// 先查询哪些节点实际有修改记录
	var existingNodes []FigmaNode
	result := DB.Where("project_id = ? AND node_id IN ?", projectID, nodeIDs).Find(&existingNodes)
	if result.Error != nil {
		return nil, result.Error
	}

	// 提取实际存在的节点ID
	var deletedNodeIDs []string
	for _, node := range existingNodes {
		deletedNodeIDs = append(deletedNodeIDs, node.NodeID)
	}

	// 如果没有需要删除的节点，直接返回
	if len(deletedNodeIDs) == 0 {
		return []string{}, nil
	}

	// 删除这些节点的修改信息
	err := DB.Where("project_id = ? AND node_id IN ?", projectID, deletedNodeIDs).Delete(&FigmaNode{}).Error
	if err != nil {
		return nil, err
	}

	return deletedNodeIDs, nil
}

// CreateExportJob 创建导出任务
func CreateExportJob(userID, projectID uint, path, format string) (*ExportJob, error) {
	// 先取消该项目的所有进行中的导出任务
	err := CancelExportJobsByProject(projectID)
	if err != nil {
		return nil, fmt.Errorf("取消之前的导出任务失败: %v", err)
	}

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

// CancelExportJobsByProject 取消项目的所有进行中的导出任务
func CancelExportJobsByProject(projectID uint) error {
	return DB.Model(&ExportJob{}).
		Where("project_id = ? AND status IN (?)", projectID, []string{"pending", "processing"}).
		Updates(map[string]interface{}{
			"status": "cancelled",
			"error":  "任务被新的导出任务取消",
		}).Error
}

// GetActiveExportJobsByProject 获取项目的所有活跃导出任务
func GetActiveExportJobsByProject(projectID uint) ([]ExportJob, error) {
	var jobs []ExportJob
	result := DB.Where("project_id = ? AND status IN (?)", projectID, []string{"pending", "processing"}).Find(&jobs)
	if result.Error != nil {
		return nil, result.Error
	}
	return jobs, nil
}

// AddRefNode 添加依赖节点
func AddRefNode(projectID uint, nodeID string) error {
	project, err := GetProjectByID(projectID)
	if err != nil {
		return err
	}

	var refNodes []string
	if project.RefNodes != "" {
		if err := json.Unmarshal([]byte(project.RefNodes), &refNodes); err != nil {
			return fmt.Errorf("解析依赖节点列表失败: %v", err)
		}
	}

	// 检查节点是否已存在
	for _, existingNodeID := range refNodes {
		if existingNodeID == nodeID {
			return fmt.Errorf("依赖节点 %s 已存在", nodeID)
		}
	}

	// 添加新节点
	refNodes = append(refNodes, nodeID)

	// 序列化并保存
	refNodesJSON, err := json.Marshal(refNodes)
	if err != nil {
		return fmt.Errorf("序列化依赖节点列表失败: %v", err)
	}

	return DB.Model(&FigmaProject{}).Where("id = ?", projectID).Update("ref_nodes", string(refNodesJSON)).Error
}

// RemoveRefNode 移除依赖节点
func RemoveRefNode(projectID uint, nodeID string) error {
	project, err := GetProjectByID(projectID)
	if err != nil {
		return err
	}

	var refNodes []string
	if project.RefNodes != "" {
		if err := json.Unmarshal([]byte(project.RefNodes), &refNodes); err != nil {
			return fmt.Errorf("解析依赖节点列表失败: %v", err)
		}
	}

	// 查找并移除节点
	found := false
	for i, existingNodeID := range refNodes {
		if existingNodeID == nodeID {
			refNodes = append(refNodes[:i], refNodes[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("依赖节点 %s 不存在", nodeID)
	}

	// 序列化并保存
	refNodesJSON, err := json.Marshal(refNodes)
	if err != nil {
		return fmt.Errorf("序列化依赖节点列表失败: %v", err)
	}

	return DB.Model(&FigmaProject{}).Where("id = ?", projectID).Update("ref_nodes", string(refNodesJSON)).Error
}

// GetRefNodes 获取依赖节点列表
func GetRefNodes(projectID uint) ([]string, error) {
	project, err := GetProjectByID(projectID)
	if err != nil {
		return nil, err
	}

	var refNodes []string
	if project.RefNodes != "" {
		if err := json.Unmarshal([]byte(project.RefNodes), &refNodes); err != nil {
			return nil, fmt.Errorf("解析依赖节点列表失败: %v", err)
		}
	}

	return refNodes, nil
}
