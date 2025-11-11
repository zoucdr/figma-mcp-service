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

	"github.com/figma-deliver/internal/models"
	"github.com/gin-gonic/gin"
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
			fmt.Printf("加载节点修改信息: nodeID=%s, modifys=%v\n", node.NodeID, settings)
		} else {
			fmt.Printf("解析节点修改信息失败: nodeID=%s, error=%v\n", node.NodeID, err)
		}
	}
	fmt.Printf("总共加载了 %d 个节点的修改信息\n", len(nodeSettings))

	// 获取完整的Figma节点树数据
	figmaNodes, err := GetFigmaNodes(user.FigmaToken, project.FileKey, project.RootNodeID)
	if err != nil {
		models.UpdateExportJobStatus(jobID, "failed", 0, "", "获取节点树数据失败: "+err.Error())
		return
	}

	// 下载所有可见节点的图片
	// 首先收集所有需要下载图片的节点ID（包括树中的所有节点）
	allNodeIDs := make(map[string]bool)
	for _, node := range figmaNodes {
		if nodeID, ok := node["id"].(string); ok {
			allNodeIDs[nodeID] = true
		}
	}

	// 下载图片
	totalNodes := len(allNodeIDs)
	processedCount := 0
	for nodeID := range allNodeIDs {
		// 获取节点设置
		setting := nodeSettings[nodeID]

		// 获取图片格式
		format := job.Format // 使用任务中指定的默认格式
		// 如果节点有自定义格式，优先使用节点的自定义格式
		if downloadType, ok := setting["img_ext"].(string); ok && downloadType != "" {
			format = downloadType
		}

		// 下载图片，使用任务中指定的缩放比例
		// 这会返回previews目录中按照hash处理后的图片路径
		imagePath, err := DownloadPreviewFigmaImageWithOptions(user.FigmaToken, project.FileKey, nodeID, format, job.Scale, true)
		if err == nil && imagePath != "" {
			// 检查图片文件是否存在
			if _, err := os.Stat(imagePath); err == nil {
				// 从imagePath提取原始图片文件名（包含hash等信息的完整文件名）
				imageFileName := filepath.Base(imagePath)

				// 复制到临时目录，使用从imagePath提取的原始图片名
				destPath := filepath.Join(tempDir, imageFileName)
				if err := copyFile(imagePath, destPath); err == nil {
					// 复制成功，记录相对路径（用于metadata.json中的img_path）
					nodeImages[nodeID] = imageFileName
					fmt.Printf("成功复制图片: %s -> %s\n", imagePath, destPath)
				} else {
					fmt.Printf("复制图片失败: %s -> %s, 错误: %v\n", imagePath, destPath, err)
				}
			} else {
				fmt.Printf("图片文件不存在: %s, 错误: %v\n", imagePath, err)
			}
		}

		processedCount++
		// 更新进度
		progress := 20 + int(float64(processedCount)/float64(totalNodes)*60)
		models.UpdateExportJobStatus(jobID, "processing", progress, job.FilePath, "")
	}

	// 构建树结构（参考 getOptimizedProjectData 的逻辑，使用0级简化）
	// 首先过滤掉需要移除的节点
	filteredNodes := filterVisibleAndNonIgnoredNodesForExport(figmaNodes, nodeSettings)

	// 构建优化后的节点数据（0级简化：保留所有原数据）
	optimizedNodes := make([]gin.H, 0)
	for _, node := range filteredNodes {
		nodeID := node["id"].(string)

		// 创建优化节点，包含所有Figma原始数据（0级简化：保留所有字段，包括空值）
		optimizedNode := gin.H{}

		// 复制所有Figma原始属性（跳过children字段，稍后重新构建）
		for key, value := range node {
			if key == "children" {
				continue
			}
			// 0级简化：保留所有字段，包括空值
			optimizedNode[key] = value
		}

		// 应用修改信息（如果存在）
		if modifys, exists := nodeSettings[nodeID]; exists {
			fmt.Printf("应用修改信息到节点: nodeID=%s, modifys=%v\n", nodeID, modifys)
			applyModificationsToNodeForExport(optimizedNode, modifys)
			fmt.Printf("应用后的节点: nodeID=%s, name=%v, horizontal=%v, vertical=%v, res_mode=%v\n",
				nodeID, optimizedNode["name"], optimizedNode["horizontal"], optimizedNode["vertical"], optimizedNode["res_mode"])
		} else {
			fmt.Printf("节点没有修改信息: nodeID=%s\n", nodeID)
		}

		// 如果有图片，添加img_path字段
		if imagePath, hasImage := nodeImages[nodeID]; hasImage {
			optimizedNode["img_path"] = imagePath
		}

		optimizedNodes = append(optimizedNodes, optimizedNode)
	}

	// 根据res_mode处理节点结构（需要在构建树结构之前处理）
	optimizedNodes = processNodesByResModeForExport(optimizedNodes, nodeSettings)

	// 构建节点映射和子节点映射
	nodeMap := make(map[string]gin.H)
	childrenMap := make(map[string][]gin.H)

	// 先建立所有节点的映射
	for _, node := range optimizedNodes {
		nodeID := node["id"].(string)
		nodeMap[nodeID] = node
	}

	// 构建父子关系
	for _, node := range optimizedNodes {
		parentID := node["parent_id"]
		if parentID != nil && parentID != project.RootNodeID {
			if parentIDStr, ok := parentID.(string); ok {
				childrenMap[parentIDStr] = append(childrenMap[parentIDStr], node)
			}
		}
	}

	// 创建处理后节点的ID集合，用于快速查找
	validNodeIDs := make(map[string]bool)
	for _, node := range optimizedNodes {
		nodeID := node["id"].(string)
		validNodeIDs[nodeID] = true
	}

	// 为每个节点添加子节点信息
	for i := range optimizedNodes {
		nodeID := optimizedNodes[i]["id"].(string)

		// 清除节点原有的children字段，使用我们重新构建的children
		delete(optimizedNodes[i], "children")

		if children, hasChildren := childrenMap[nodeID]; hasChildren && len(children) > 0 {
			// 只保留仍然存在的子节点
			validChildren := make([]gin.H, 0)
			for _, child := range children {
				childID := child["id"].(string)
				if validNodeIDs[childID] {
					validChildren = append(validChildren, child)
				}
			}

			if len(validChildren) > 0 {
				// 添加有效的子节点
				optimizedNodes[i]["children"] = validChildren
			}
		}
	}

	// 找到根节点（从根节点开始的树形结构）
	var rootNode gin.H
	for i, node := range optimizedNodes {
		parentID := node["parent_id"]
		if parentID == nil || parentID == project.RootNodeID {
			// 0级简化：不砍任何原数据，保留所有字段（包括空值）
			rootNode = optimizedNodes[i]
			break // 只取第一个根节点
		}
	}

	// parent_id是用来调整层级的，调整好后不需要返回给客户端，递归删除所有节点中的parent_id字段
	if rootNode != nil {
		removeParentIDFromTree(rootNode)
	}

	// 如果找不到根节点，使用错误处理
	if rootNode == nil {
		models.UpdateExportJobStatus(jobID, "failed", 0, "", "未找到根节点")
		return
	}

	// 创建元数据文件（使用树结构，参考 APIGetOptimizedNodes 的格式）
	metadata := map[string]interface{}{
		"project": map[string]interface{}{
			"id":           project.ID,
			"name":         project.Name,
			"file_key":     project.FileKey,
			"root_node_id": project.RootNodeID,
		},
		"node": rootNode, // 单个根节点，包含完整的子树（0级简化，保留所有原数据）
		"export_config": map[string]interface{}{
			"format":      job.Format,
			"scale":       job.Scale,
			"exported_at": time.Now(),
		},
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

// filterVisibleAndNonIgnoredNodesForExport 过滤掉不可见和被忽略的节点及其子树
func filterVisibleAndNonIgnoredNodesForExport(nodes []map[string]interface{}, nodeModifys map[string]map[string]interface{}) []map[string]interface{} {
	// 构建节点ID到节点的映射
	nodeMap := make(map[string]map[string]interface{})
	for _, node := range nodes {
		nodeID := node["id"].(string)
		nodeMap[nodeID] = node
	}

	// 构建父子关系映射
	childrenMap := make(map[string][]string)
	for _, node := range nodes {
		nodeID := node["id"].(string)
		if parentID, ok := node["parent_id"]; ok && parentID != nil {
			if parentIDStr, ok := parentID.(string); ok {
				childrenMap[parentIDStr] = append(childrenMap[parentIDStr], nodeID)
			}
		}
	}

	// 收集需要排除的节点ID（包括其子树）
	excludedNodeIDs := make(map[string]bool)

	for _, node := range nodes {
		nodeID := node["id"].(string)

		// 检查是否应该排除此节点
		if shouldExcludeNodeAndSubtreeForExport(node, nodeModifys[nodeID]) {
			// 标记此节点及其所有子节点为排除
			markNodeAndChildrenAsExcludedForExport(nodeID, childrenMap, excludedNodeIDs)
		}
	}

	// 构建过滤后的节点列表
	var filteredNodes []map[string]interface{}
	for _, node := range nodes {
		nodeID := node["id"].(string)
		if !excludedNodeIDs[nodeID] {
			filteredNodes = append(filteredNodes, node)
		}
	}

	return filteredNodes
}

// shouldExcludeNodeAndSubtreeForExport 检查节点是否应该被排除（参考 api.go 中的逻辑）
func shouldExcludeNodeAndSubtreeForExport(node map[string]interface{}, modifys map[string]interface{}) bool {
	nodeID := node["id"].(string)

	// 1. 检查visible属性，如果为false则排除整个子树
	if visible, hasVisible := node["visible"]; hasVisible {
		if visibleBool, ok := visible.(bool); ok && !visibleBool {
			fmt.Printf("排除节点 %s 及其子树，因为其visible为false\n", nodeID)
			return true
		}
	}

	// 2. 检查modifys中的ignore标记，如果为true则排除整个子树
	if modifys != nil {
		if ignore, ok := modifys["ignore"].(bool); ok && ignore {
			fmt.Printf("排除节点 %s 及其子树，因为其被标记为忽略\n", nodeID)
			return true
		}
	}

	return false
}

// markNodeAndChildrenAsExcludedForExport 递归标记节点及其所有子节点为排除
func markNodeAndChildrenAsExcludedForExport(nodeID string, childrenMap map[string][]string, excludedNodeIDs map[string]bool) {
	if excludedNodeIDs[nodeID] {
		return // 已经标记过，避免重复处理
	}
	excludedNodeIDs[nodeID] = true

	// 递归标记所有子节点
	if children, hasChildren := childrenMap[nodeID]; hasChildren {
		for _, childID := range children {
			markNodeAndChildrenAsExcludedForExport(childID, childrenMap, excludedNodeIDs)
		}
	}
}

// isEmptyValueForExport 判断值是否为空（用于导出功能）
func isEmptyValueForExport(value interface{}) bool {
	if value == nil {
		return true
	}

	switch v := value.(type) {
	case string:
		return v == ""
	case []interface{}:
		return len(v) == 0
	case map[string]interface{}:
		return len(v) == 0
	case []gin.H:
		return len(v) == 0
	case gin.H:
		// 节点对象不应该被认为是空值，即使字段数为0
		return false
	case []string:
		return len(v) == 0
	case []int:
		return len(v) == 0
	case []float64:
		return len(v) == 0
	default:
		// 对于其他类型（如bool、int、float64等），不认为是空值
		return false
	}
}

// applyModificationsToNodeForExport 将修改信息应用到节点中
func applyModificationsToNodeForExport(node gin.H, modifys map[string]interface{}) {
	fmt.Printf("开始应用修改信息: modifys keys=%v\n", getMapKeysForExport(modifys))

	// 1. rename不为空时，替换节点name
	if rename, ok := modifys["rename"].(string); ok && !isEmptyValueForExport(rename) {
		fmt.Printf("应用 rename: %s -> %s\n", node["name"], rename)
		node["name"] = rename
	}

	// 2. horizontal节点中增加字段
	if horizontal, ok := modifys["horizontal"].(string); ok && !isEmptyValueForExport(horizontal) {
		fmt.Printf("应用 horizontal: %s\n", horizontal)
		node["horizontal"] = horizontal
	}

	// 3. vertical节点中增加字段
	if vertical, ok := modifys["vertical"].(string); ok && !isEmptyValueForExport(vertical) {
		fmt.Printf("应用 vertical: %s\n", vertical)
		node["vertical"] = vertical
	}

	// 4. components不为空时，增加字段
	if components, ok := modifys["components"]; ok && !isEmptyValueForExport(components) {
		fmt.Printf("应用 components: %v\n", components)
		node["components"] = components
	}

	// 5. img_ext不为空时，增加字段
	if img_ext, ok := modifys["img_ext"].(string); ok && !isEmptyValueForExport(img_ext) {
		fmt.Printf("应用 img_ext: %s\n", img_ext)
		node["img_ext"] = img_ext
	}

	// 6. img_name不为空时，增加字段
	if imgName, ok := modifys["img_name"].(string); ok && !isEmptyValueForExport(imgName) {
		fmt.Printf("应用 img_name: %s\n", imgName)
		node["img_name"] = imgName
	}

	// 7. res_mode不为空时，增加字段
	if resMode, ok := modifys["res_mode"].(string); ok && !isEmptyValueForExport(resMode) {
		fmt.Printf("应用 res_mode: %s\n", resMode)
		node["res_mode"] = resMode
	}

	// 8. parent_id不为空时，切换子树的父节点到parent_id
	if parentID, ok := modifys["parent_id"].(string); ok && !isEmptyValueForExport(parentID) {
		fmt.Printf("应用 parent_id: %s\n", parentID)
		node["parent_id"] = parentID
	}

	// 9. customName不为空时，增加字段
	if customName, ok := modifys["customName"].(string); ok && !isEmptyValueForExport(customName) {
		fmt.Printf("应用 customName: %s\n", customName)
		node["customName"] = customName
	}

	// 10. visible不为空时，更新visible字段（注意：modify中的visible字段名可能是visible）
	if visible, ok := modifys["visible"].(bool); ok {
		fmt.Printf("应用 visible: %v\n", visible)
		node["visible"] = visible
	}

	// 11. ignore不为空时，增加字段
	if ignore, ok := modifys["ignore"].(bool); ok {
		fmt.Printf("应用 ignore: %v\n", ignore)
		node["ignore"] = ignore
	}

	fmt.Printf("修改信息应用完成，节点当前字段: %v\n", getMapKeysForExport(node))
}

// getMapKeysForExport 获取map的所有键（用于调试）
func getMapKeysForExport(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// processNodesByResModeForExport 根据res_mode处理节点结构
func processNodesByResModeForExport(nodes []gin.H, nodeModifys map[string]map[string]interface{}) []gin.H {
	// 构建节点ID到节点的映射
	nodeMap := make(map[string]*gin.H)
	for i := range nodes {
		nodeID := nodes[i]["id"].(string)
		nodeMap[nodeID] = &nodes[i]
	}

	// 构建父子关系映射
	childrenMap := make(map[string][]*gin.H)
	for i := range nodes {
		node := &nodes[i]
		parentID := (*node)["parent_id"]
		if parentID != nil {
			if parentIDStr, ok := parentID.(string); ok {
				childrenMap[parentIDStr] = append(childrenMap[parentIDStr], node)
			}
		}
	}

	// 标记需要移除的节点
	nodesToRemove := make(map[string]bool)
	nodesToPromote := make(map[string][]*gin.H) // 需要提升的子节点

	// 递归处理所有节点的资源模式
	processNodeResModeForExport(nodes, childrenMap, nodeModifys, nodesToRemove, nodesToPromote)

	// 应用需要提升的子节点
	for parentID, childrenToPromote := range nodesToPromote {
		if _, exists := nodeMap[parentID]; exists {
			// 将需要提升的子节点添加到父节点的子节点列表中
			for _, child := range childrenToPromote {
				// 更新子节点的parent_id
				(*child)["parent_id"] = parentID
				// 添加到childrenMap中
				childrenMap[parentID] = append(childrenMap[parentID], child)
			}
		}
	}

	// 构建新的节点列表，排除被标记移除的节点
	var result []gin.H
	for _, node := range nodes {
		nodeID := node["id"].(string)
		if !nodesToRemove[nodeID] {
			result = append(result, node)
		}
	}

	return result
}

// processNodeResModeForExport 递归处理所有节点的资源模式
func processNodeResModeForExport(nodes []gin.H, childrenMap map[string][]*gin.H, nodeModifys map[string]map[string]interface{}, nodesToRemove map[string]bool, nodesToPromote map[string][]*gin.H) {
	for i := range nodes {
		node := &nodes[i]
		nodeID := (*node)["id"].(string)

		// 如果节点已经被标记为删除，跳过处理
		if nodesToRemove[nodeID] {
			continue
		}

		// 检查是否有res_mode修改
		if modifys, exists := nodeModifys[nodeID]; exists {
			if resMode, ok := modifys["res_mode"].(string); ok && resMode != "" {
				switch resMode {
				case "sprite", "slice", "texture":
					// 图片相关模式：清除子节点中不包含图片或文字节点的部分
					if children, hasChildren := childrenMap[nodeID]; hasChildren {
						for _, child := range children {
							childID := (*child)["id"].(string)
							// 检查子节点及其子树是否包含图片或文字节点，如果不包含则可以删除
							if !hasImageNodeInSubtreeForExport(childID, childrenMap, nodeModifys) {
								markNodeAndDescendantsForRemovalForExport(childID, childrenMap, nodesToRemove)
							}
						}
					}
				case "cutout":
					// 层级剔除模式：移除当前节点，子节点上移
					nodesToRemove[nodeID] = true
					if children, hasChildren := childrenMap[nodeID]; hasChildren {
						currentParentID := (*node)["parent_id"]
						if currentParentID != nil {
							if parentIDStr, ok := currentParentID.(string); ok {
								// 将子节点的父节点设置为当前节点的父节点
								for _, child := range children {
									(*child)["parent_id"] = parentIDStr
								}
								// 记录需要提升的子节点
								nodesToPromote[parentIDStr] = append(nodesToPromote[parentIDStr], children...)
							}
						} else {
							// 如果当前节点没有父节点，子节点也设为根节点
							for _, child := range children {
								(*child)["parent_id"] = nil
							}
							// 对于根节点的子节点，需要特殊处理
							// 这里暂时不处理，因为根节点的子节点提升逻辑比较复杂
						}
					}
				}
			}
		}

		// 递归处理子节点（只处理未被删除的子节点）
		if children, hasChildren := childrenMap[nodeID]; hasChildren {
			for _, child := range children {
				childID := (*child)["id"].(string)
				if !nodesToRemove[childID] {
					// 递归处理子节点
					processNodeResModeForExport([]gin.H{*child}, childrenMap, nodeModifys, nodesToRemove, nodesToPromote)
				}
			}
		}
	}
}

// hasImageNodeInSubtreeForExport 检查子树中是否包含图片或文字节点
func hasImageNodeInSubtreeForExport(nodeID string, childrenMap map[string][]*gin.H, nodeModifys map[string]map[string]interface{}) bool {
	// 检查当前节点是否有图片
	if modifys, exists := nodeModifys[nodeID]; exists {
		if resMode, ok := modifys["res_mode"].(string); ok {
			if resMode == "sprite" || resMode == "slice" || resMode == "texture" {
				return true
			}
		}
	}

	// 递归检查子节点
	if children, hasChildren := childrenMap[nodeID]; hasChildren {
		for _, child := range children {
			childID := (*child)["id"].(string)
			if hasImageNodeInSubtreeForExport(childID, childrenMap, nodeModifys) {
				return true
			}
		}
	}

	return false
}

// markNodeAndDescendantsForRemovalForExport 递归标记节点及其所有后代为删除
func markNodeAndDescendantsForRemovalForExport(nodeID string, childrenMap map[string][]*gin.H, nodesToRemove map[string]bool) {
	if nodesToRemove[nodeID] {
		return // 已经标记过，避免重复处理
	}
	nodesToRemove[nodeID] = true

	// 递归标记所有子节点
	if children, hasChildren := childrenMap[nodeID]; hasChildren {
		for _, child := range children {
			childID := (*child)["id"].(string)
			markNodeAndDescendantsForRemovalForExport(childID, childrenMap, nodesToRemove)
		}
	}
}

// removeParentIDFromTree 递归删除树中所有节点的parent_id字段（parent_id用来调整层级，调整好后不需要返回）
func removeParentIDFromTree(node gin.H) {
	// 删除当前节点的parent_id字段
	delete(node, "parent_id")

	// 递归处理子节点
	if children, ok := node["children"].([]gin.H); ok && len(children) > 0 {
		for _, child := range children {
			removeParentIDFromTree(child)
		}
	}
}
