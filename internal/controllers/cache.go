package controllers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/figma-deliver/internal/models"
	"github.com/figma-deliver/internal/services"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// CacheController 缓存控制器
type CacheController struct {
	cacheService *services.CacheService
	queueService *services.QueueService
	batchService *services.RenderBatchService
	obsService   *services.OBSService
}

// NewCacheController 创建缓存控制器
func NewCacheController(cacheService *services.CacheService, queueService *services.QueueService, batchService *services.RenderBatchService, obsService *services.OBSService) *CacheController {
	return &CacheController{
		cacheService: cacheService,
		queueService: queueService,
		batchService: batchService,
		obsService:   obsService,
	}
}

// ===================== 节点过滤工具函数 =====================

// shouldExcludeNodeForRender 判断节点是否应该在渲染时被排除
func shouldExcludeNodeForRender(nodeID string, nodeData map[string]interface{}, modifys map[string]interface{}) bool {
	// 1. 检查节点的visible属性，如果为false则排除
	if nodeData != nil {
		if visible, hasVisible := nodeData["visible"]; hasVisible {
			if visibleBool, ok := visible.(bool); ok && !visibleBool {
				log.Printf("🚫 [节点过滤] 排除节点 %s，因为其visible为false", nodeID)
				return true
			}
		}
	}

	// 2. 检查modifys中的ignore标记，如果为true则排除
	if modifys != nil {
		if ignore, ok := modifys["ignore"].(bool); ok && ignore {
			log.Printf("🚫 [节点过滤] 排除节点 %s，因为其被标记为忽略", nodeID)
			return true
		}
	}

	return false
}

// filterValidNodeIDs 过滤掉无效节点ID（visible=false或ignore=true）
func filterValidNodeIDs(projectID uint, fileKey, rootNodeID string, nodeIDs []string) ([]string, error) {
	if len(nodeIDs) == 0 {
		return nodeIDs, nil
	}

	// 获取节点的修改信息
	var figmaNodes []models.FigmaNode
	if err := models.DB.Where("project_id = ?", projectID).Find(&figmaNodes).Error; err != nil {
		log.Printf("⚠️ [节点过滤] 获取节点修改信息失败: %v", err)
		return nodeIDs, nil // 发生错误时不过滤，返回原始列表
	}

	// 构建节点修改信息映射
	nodeModifys := make(map[string]map[string]interface{})
	for _, node := range figmaNodes {
		if node.Modifys != "" {
			var modifyData map[string]interface{}
			if err := json.Unmarshal([]byte(node.Modifys), &modifyData); err == nil {
				nodeModifys[node.NodeID] = modifyData
			}
		}
	}

	// 获取文件缓存数据以检查visible属性
	fileCache, err := models.GetFileCache(fileKey, rootNodeID)
	var nodeDataMap map[string]map[string]interface{}
	if err == nil && fileCache != nil && fileCache.FileData != "" {
		var fileData map[string]interface{}
		if err := json.Unmarshal([]byte(fileCache.FileData), &fileData); err == nil {
			// 递归构建节点数据映射
			nodeDataMap = make(map[string]map[string]interface{})
			if document, ok := fileData["document"].(map[string]interface{}); ok {
				buildNodeDataMap(document, nodeDataMap)
			}
		}
	}

	// 过滤节点ID
	validNodeIDs := make([]string, 0, len(nodeIDs))
	excludedCount := 0
	for _, nodeID := range nodeIDs {
		nodeData := nodeDataMap[nodeID]
		modifys := nodeModifys[nodeID]

		if !shouldExcludeNodeForRender(nodeID, nodeData, modifys) {
			validNodeIDs = append(validNodeIDs, nodeID)
		} else {
			excludedCount++
		}
	}

	if excludedCount > 0 {
		log.Printf("✅ [节点过滤] 过滤了 %d 个无效节点，剩余 %d 个有效节点", excludedCount, len(validNodeIDs))
	}

	return validNodeIDs, nil
}

// buildNodeDataMap 递归构建节点数据映射
func buildNodeDataMap(node map[string]interface{}, nodeMap map[string]map[string]interface{}) {
	if nodeID, ok := node["id"].(string); ok {
		nodeMap[nodeID] = node
	}

	if children, ok := node["children"].([]interface{}); ok {
		for _, child := range children {
			if childNode, ok := child.(map[string]interface{}); ok {
				buildNodeDataMap(childNode, nodeMap)
			}
		}
	}
}

// ===================== 节点树刷新 =====================

// RefreshFileRequest 节点树刷新请求
type RefreshFileRequest struct {
	FileKey    string `json:"file_key" binding:"required"`
	RootNodeID string `json:"root_node_id"` // 可选，空则刷新整个文件
}

// RefreshFileResponse 节点树刷新响应
type RefreshFileResponse struct {
	CacheID              uint   `json:"cache_id"`
	Status               string `json:"status"` // waiting, loading, loaded
	EstimatedWaitSeconds uint32 `json:"estimated_wait_seconds"`
}

// RefreshFile 节点树刷新请求（强制排队刷新）
// POST /api/figma/file/refresh
func (cc *CacheController) RefreshFile(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	// 获取用户信息
	user, err := models.FindUserByID(userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户信息失败"})
		return
	}

	if user.FigmaToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未配置 Figma Token"})
		return
	}

	var req RefreshFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	// 检查是否已有缓存记录
	existingCache, hit, err := cc.cacheService.GetFileCache(req.FileKey, req.RootNodeID)
	if err == nil && existingCache != nil {
		// 已有缓存记录，根据状态返回
		if existingCache.Status == "loading" {
			// 正在加载中
			c.JSON(http.StatusOK, gin.H{
				"code":    0,
				"message": "文件正在加载中",
				"data": RefreshFileResponse{
					CacheID:              existingCache.ID,
					Status:               existingCache.Status,
					EstimatedWaitSeconds: 0,
				},
			})
			return
		} else if existingCache.Status == "loaded" && hit {
			// 已加载成功，返回缓存ID（前端可以查询状态获取数据）
			c.JSON(http.StatusOK, gin.H{
				"code":    0,
				"message": "文件已缓存",
				"data": RefreshFileResponse{
					CacheID:              existingCache.ID,
					Status:               existingCache.Status,
					EstimatedWaitSeconds: 0,
				},
			})
			return
		}
	}

	// 检查 Token 冷却时间，用于返回预计等待时间
	_, remaining, err := cc.cacheService.CheckFileAPICooldown(user.FigmaToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "检查冷却时间失败"})
		return
	}

	// 创建或更新缓存记录
	var cache *models.FigmaFileCache
	if existingCache != nil {
		// 更新状态为 waiting
		now := uint32(time.Now().Unix())
		existingCache.Status = "waiting"
		existingCache.UpdatedAt = now
		if err := cc.cacheService.UpdateFileCacheStatus(existingCache.ID, "waiting", ""); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新缓存状态失败"})
			return
		}
		cache = existingCache
	} else {
		// 创建新缓存记录（需要获取用户的 token）
		// 从项目中获取用户 ID 和 token
		var project models.FigmaProject
		err = models.DB.Where("file_key = ? AND root_node_id = ?", req.FileKey, req.RootNodeID).
			First(&project).Error
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "未找到关联的项目"})
			return
		}

		var user models.User
		err = models.DB.First(&user, project.UserID).Error
		if err != nil || user.FigmaToken == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户 Token 失败"})
			return
		}

		cache, err = cc.cacheService.CreateFileCache(req.FileKey, req.RootNodeID, user.FigmaToken, []string{})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建缓存记录失败"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "已加入刷新队列",
		"data": RefreshFileResponse{
			CacheID:              cache.ID,
			Status:               "waiting",
			EstimatedWaitSeconds: remaining,
		},
	})
}

// ===================== 获取节点树请求状态 =====================

// GetFileCacheStatus 获取节点树请求状态
// GET /api/figma/file/cache/status/:cache_id
func (cc *CacheController) GetFileCacheStatus(c *gin.Context) {
	cacheIDStr := c.Param("cache_id")
	cacheID, err := strconv.ParseUint(cacheIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的缓存ID"})
		return
	}

	// 直接查询数据库
	var cache models.FigmaFileCache
	if err := models.DB.First(&cache, uint(cacheID)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "缓存记录不存在"})
		return
	}

	response := gin.H{
		"cache_id":      cache.ID,
		"status":        cache.Status,
		"progress":      0,
		"error_message": cache.ErrorMessage,
		"created_at":    cache.CreatedAt,
		"updated_at":    cache.UpdatedAt,
	}

	// 如果状态是 loaded，返回文件数据
	if cache.Status == "loaded" {
		response["file_data"] = cache.FileData
		response["progress"] = 100
	} else if cache.Status == "loading" {
		response["progress"] = 50
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    response,
	})
}

// ===================== 项目渲染 =====================

// ===================== 项目渲染（已弃用，请使用 ManualRender） =====================

// RenderProjectRequest 项目渲染请求
// DEPRECATED: 已弃用，请使用 ManualRender 接口
type RenderProjectRequest struct {
	Format          string  `json:"format" binding:"required,oneof=png jpg svg"`
	Scale           float64 `json:"scale" binding:"required,gt=0"` // 只要求大于0即可
	IncludeRefNodes bool    `json:"include_ref_nodes"`             // 是否包含依赖节点
}

// RenderProjectResponse 项目渲染响应
// DEPRECATED: 已弃用，请使用 ManualRender 接口
type RenderProjectResponse struct {
	QueueIDs                 []uint `json:"queue_ids"`
	TotalNodes               int    `json:"total_nodes"`
	EstimatedDurationSeconds int    `json:"estimated_duration_seconds"`
}

// RenderProject 项目渲染请求
// POST /api/figma/project/render
// DEPRECATED: 此方法已弃用，统一使用 ManualRender 接口，提供更灵活的节点筛选功能
// 保留此方法仅用于向后兼容，建议迁移到 ManualRender
/*
func (cc *CacheController) RenderProject(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	// 从 URL 路径参数获取 project_id
	projectIDStr := c.Param("project_id")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的项目ID"})
		return
	}

	// 获取用户信息
	user, err := models.FindUserByID(userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户信息失败"})
		return
	}

	if user.FigmaToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未配置 Figma Token"})
		return
	}

	var req RenderProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	// 获取项目信息
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	// 验证项目是否属于当前用户
	if project.UserID != userID.(uint) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该项目"})
		return
	}

	// 1. 从缓存中提取项目节点树的所有节点ID（应用 ignore 和 visible 过滤）
	log.Printf("📋 [RenderProject] 开始提取项目节点: ProjectID=%d, FileKey=%s, RootNodeID=%s",
		projectID, project.FileKey, project.RootNodeID)

	nodeIDs, _, err := cc.cacheService.GetProjectNodeIDs(uint(projectID), project.FileKey, project.RootNodeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// 2. 如果包含依赖节点，添加 RefNodes
	if req.IncludeRefNodes {
		refNodes, err := models.GetRefNodes(uint(projectID))
		if err == nil && len(refNodes) > 0 {
			log.Printf("📎 [RenderProject] 添加依赖节点: %d 个", len(refNodes))

			// 使用 map 去重
			nodeIDMap := make(map[string]bool)
			for _, nodeID := range nodeIDs {
				nodeIDMap[nodeID] = true
			}

			// 添加依赖节点（去重）
			addedCount := 0
			for _, refNode := range refNodes {
				if !nodeIDMap[refNode] {
					nodeIDs = append(nodeIDs, refNode)
					nodeIDMap[refNode] = true
					addedCount++
				}
			}
			log.Printf("✅ [RenderProject] 添加了 %d 个新的依赖节点（去重后）", addedCount)
		}
	}

	// 3. 过滤掉无效节点（visible=false或ignore=true）
	originalCount := len(nodeIDs)
	nodeIDs, err = filterValidNodeIDs(uint(projectID), project.FileKey, project.RootNodeID, nodeIDs)
	if err != nil {
		log.Printf("⚠️ [RenderProject] 过滤节点失败: %v，继续使用原始节点列表", err)
	} else if len(nodeIDs) < originalCount {
		log.Printf("🔍 [RenderProject] 过滤前: %d 个节点，过滤后: %d 个节点", originalCount, len(nodeIDs))
	}

	// 4. 如果没有节点，返回错误
	if len(nodeIDs) == 0 {
		log.Printf("❌ [RenderProject] 没有可渲染的节点")
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目没有需要渲染的节点（所有节点都被过滤）"})
		return
	}

	log.Printf("✅ [RenderProject] 总共需要渲染 %d 个节点", len(nodeIDs))

	// 5. 检查并清理旧的渲染队列
	oldRenderID := project.RenderID
	if oldRenderID > 0 {
		log.Printf("🔍 [RenderProject] 检查旧的渲染队列: RenderID=%d", oldRenderID)

		// 查询旧渲染队列的状态
		var oldQueue models.FigmaRenderQueue
		err := models.DB.Where("id = ?", oldRenderID).First(&oldQueue).Error
		if err == nil {
			// 找到旧队列，检查状态
			if oldQueue.Status == "processing" {
				// 正在处理中，不允许重新渲染
				log.Printf("⚠️ [RenderProject] 旧渲染队列正在处理中，不允许重新渲染: RenderID=%d, Status=%s", oldRenderID, oldQueue.Status)
				c.JSON(http.StatusConflict, gin.H{
					"error": "当前渲染任务正在处理中，请等待完成后再重新渲染",
					"data": gin.H{
						"render_id": oldRenderID,
						"status":    oldQueue.Status,
					},
				})
				return
			} else {
				// 非处理中状态（waiting/completed/failed/partial/cancelled），删除旧队列
				log.Printf("🗑️ [RenderProject] 删除旧的非处理中渲染队列: RenderID=%d, Status=%s", oldRenderID, oldQueue.Status)
				if err := models.DB.Delete(&oldQueue).Error; err != nil {
					log.Printf("⚠️ [RenderProject] 删除旧渲染队列失败: %v", err)
				} else {
					log.Printf("✅ [RenderProject] 已删除旧渲染队列")
				}
			}
		} else {
			// 旧队列不存在或查询失败
			log.Printf("⚠️ [RenderProject] 旧渲染队列不存在或查询失败: RenderID=%d, Error=%v", oldRenderID, err)
		}
	}

	// 6. 创建渲染队列（传入项目ID）
	queueIDs, err := cc.queueService.CreateRenderQueue(
		user.FigmaToken,
		project.FileKey,
		nodeIDs,
		req.Format,
		req.Scale,
		uint(projectID), // 传入当前项目ID
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建渲染队列失败: " + err.Error()})
		return
	}

	// 7. 更新项目的 render_id（使用第一个队列ID）
	if len(queueIDs) > 0 {
		newRenderID := queueIDs[0]
		err := models.DB.Model(&models.FigmaProject{}).
			Where("id = ?", projectID).
			Update("render_id", newRenderID).Error
		if err != nil {
			log.Printf("❌ [RenderProject] 更新项目 %d 的 render_id 失败: %v", projectID, err)
		} else {
			log.Printf("✅ [RenderProject] 已更新项目 render_id: %d -> %d", oldRenderID, newRenderID)
		}
	}

	// 估算处理时间（每个节点约 2 秒，加上API冷却时间）
	estimatedSeconds := (len(nodeIDs) * 2) + (len(queueIDs) * 30)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "已加入渲染队列",
		"data": RenderProjectResponse{
			QueueIDs:                 queueIDs,
			TotalNodes:               len(nodeIDs),
			EstimatedDurationSeconds: estimatedSeconds,
		},
	})
}
*/

// ===================== 获取渲染队列信息 =====================

// GetRenderQueue 获取渲染队列信息
// GET /api/figma/render/queue?status=processing,waiting
func (cc *CacheController) GetRenderQueue(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	// 获取用户信息
	user, err := models.FindUserByID(userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户信息失败"})
		return
	}

	if user.FigmaToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未配置 Figma Token"})
		return
	}

	// 获取状态参数
	statusParam := c.DefaultQuery("status", "waiting,processing")
	statuses := strings.Split(statusParam, ",")

	// 查询队列
	queues, err := cc.queueService.GetQueuesByToken(user.FigmaToken, statuses)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询队列失败"})
		return
	}

	// 转换为响应格式
	queueList := make([]gin.H, 0, len(queues))
	for _, queue := range queues {
		queueList = append(queueList, gin.H{
			"id":              queue.ID,
			"file_key":        queue.FileKey,
			"format":          queue.Format,
			"scale":           queue.Scale,
			"status":          queue.Status,
			"progress":        queue.Progress,
			"total_nodes":     queue.TotalNodes,
			"processed_nodes": queue.ProcessedNodes,
			"failed_nodes":    queue.FailedNodes,
			"created_at":      queue.CreatedAt,
			"updated_at":      queue.UpdatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"queues": queueList,
			"total":  len(queueList),
		},
	})
}

// GetProjectRenderProgress 获取项目渲染进度（使用批次）
// GET /api/figma/project/:project_id/render/progress
func (cc *CacheController) GetProjectRenderProgress(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	// 获取项目ID
	projectIDStr := c.Param("project_id")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的项目ID"})
		return
	}

	// 获取项目信息
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	// 检查项目是否属于当前用户
	if project.UserID != userID.(uint) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该项目"})
		return
	}

	// 如果没有活跃的渲染批次，返回空进度
	if project.RenderID == 0 {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data": gin.H{
				"has_render":      false,
				"status":          "idle",
				"progress":        0,
				"total_nodes":     0,
				"processed_nodes": 0,
				"failed_nodes":    0,
			},
		})
		return
	}

	// 获取批次进度
	batch, err := cc.batchService.GetBatchProgress(project.RenderID)
	if err != nil {
		log.Printf("❌ [GetProjectRenderProgress] 获取批次进度失败: %v", err)
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data": gin.H{
				"has_render": false,
			},
		})
		return
	}

	// 如果状态是 waiting，查询冷却时间
	cooldownSeconds := 0
	if batch.Status == "waiting" {
		// 查询第一个队列的冷却时间
		queueIDs, _ := models.GetQueuesByBatch(batch.ID)
		if len(queueIDs) > 0 {
			queue, err := models.GetRenderQueue(queueIDs[0])
			if err == nil && queue.FigmaToken != "" {
				cooldownRemaining, err := cc.cacheService.GetTokenCooldownRemaining(queue.FigmaToken)
				if err == nil {
					cooldownSeconds = int(cooldownRemaining)
					log.Printf("✅ [GetProjectRenderProgress] 冷却剩余时间: %d 秒", cooldownSeconds)
				}
			}
		}
	}

	// 准备响应数据
	responseData := gin.H{
		"has_render":         true,
		"batch_id":           batch.ID,
		"status":             batch.Status,
		"progress":           batch.Progress,
		"total_nodes":        batch.TotalNodes,
		"processed_nodes":    batch.ProcessedNodes,
		"failed_nodes":       batch.FailedNodes,
		"total_queues":       batch.TotalQueues,
		"completed_queues":   batch.CompletedQueues,
		"cooldown_remaining": cooldownSeconds,
		"error_message":      batch.ErrorMessage,
		"started_at":         batch.StartedAt,
		"completed_at":       batch.CompletedAt,
		"created_at":         batch.CreatedAt,
		"updated_at":         batch.UpdatedAt,
	}

	// 返回渲染进度
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    responseData,
	})
}

// CancelProjectRender 取消项目渲染
// POST /api/figma/project/:project_id/render/cancel
func (cc *CacheController) CancelProjectRender(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	// 获取项目ID
	projectIDStr := c.Param("project_id")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的项目ID"})
		return
	}

	// 获取项目信息
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	// 检查项目是否属于当前用户
	if project.UserID != userID.(uint) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该项目"})
		return
	}

	// 如果没有活跃的渲染批次
	if project.RenderID == 0 {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "没有活跃的渲染任务",
		})
		return
	}

	// 取消渲染批次
	err = cc.batchService.CancelBatchForProject(uint(projectID))
	if err != nil {
		log.Printf("❌ [CancelProjectRender] 取消渲染批次失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "取消渲染失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "渲染任务已取消",
	})
}

// GetQueueStatistics 获取队列统计信息
// GET /api/figma/render/queue/stats
func (cc *CacheController) GetQueueStatistics(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	// 获取用户信息
	user, err := models.FindUserByID(userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户信息失败"})
		return
	}

	if user.FigmaToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未配置 Figma Token"})
		return
	}

	// 获取统计信息
	stats, err := cc.queueService.GetQueueStatistics(user.FigmaToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取统计信息失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// ===================== 获取节点图片 =====================

// GetNodeImage 获取节点图片（直接返回图片文件）
// GET /api/figma/node/image?file_key=xxx&node_id=xxx&format=png&scale=2.0
func (cc *CacheController) GetNodeImage(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		// 未登录，返回备用图片
		cc.returnFallbackImage(c)
		return
	}

	// 获取用户信息
	user, err := models.FindUserByID(userID.(uint))
	if err != nil {
		// 获取用户失败，返回备用图片
		cc.returnFallbackImage(c)
		return
	}

	if user.FigmaToken == "" {
		// 未配置 Token，返回备用图片
		cc.returnFallbackImage(c)
		return
	}

	// 获取参数
	fileKey := c.Query("file_key")
	nodeID := c.Query("node_id")
	format := c.DefaultQuery("format", "png")
	scaleStr := c.DefaultQuery("scale", "1.0")

	if fileKey == "" || nodeID == "" {
		// 参数错误，返回备用图片
		cc.returnFallbackImage(c)
		return
	}

	scale, err := strconv.ParseFloat(scaleStr, 64)
	if err != nil {
		// 参数错误，返回备用图片
		cc.returnFallbackImage(c)
		return
	}

	// 查询缓存
	image, valid, err := cc.cacheService.GetNodeImage(fileKey, nodeID, format, scale)
	if err != nil {
		// 查询缓存失败，返回备用图片
		cc.returnFallbackImage(c)
		return
	}

	if valid && image != nil {
		// 有缓存，重定向到缓存URL
		if image.OBSKey != "" {
			// 优先使用 OBS（通过 Key 动态生成 URL）
			obsURL := cc.obsService.GenerateOBSURL(image.OBSKey)
			if obsURL != "" {
				c.Redirect(http.StatusFound, obsURL)
				return
			}
		}

		if image.FigmaCDNURL != "" {
			// 使用 Figma CDN URL
			c.Redirect(http.StatusFound, image.FigmaCDNURL)
			return
		}
	}

	// 没有缓存，加入渲染队列（后台处理）
	_, err = cc.queueService.CreateRenderQueue(
		user.FigmaToken,
		fileKey,
		[]string{nodeID},
		format,
		scale,
	)
	if err != nil {
		// 创建队列失败，返回备用图片
		cc.returnFallbackImage(c)
		return
	}

	// 图片正在生成中，返回备用图片作为占位
	cc.returnFallbackImage(c)
}

// returnFallbackImage 返回备用图片（当 Figma API 失败或图片不可用时）
func (cc *CacheController) returnFallbackImage(c *gin.Context) {
	// 返回 break.jpg 作为备用图片
	c.File("web/static/img/break.jpg")
}

// ===================== 手动触发渲染 =====================

// ManualRenderRequest 手动触发渲染请求
type ManualRenderRequest struct {
	FileKey    string   `json:"file_key" binding:"required"`
	NodeIDs    []string `json:"node_ids" binding:"required,min=1"`
	ProjectIDs []uint   `json:"project_ids"` // 关联的项目ID列表，可选
	Format     string   `json:"format" binding:"required,oneof=png jpg svg"`
	Scale      float64  `json:"scale" binding:"required,gt=0"` // 只要求大于0即可
}

// ManualRender 手动触发渲染
// POST /api/figma/manual/render
func (cc *CacheController) ManualRender(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	// 获取用户信息
	user, err := models.FindUserByID(userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户信息失败"})
		return
	}

	if user.FigmaToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未配置 Figma Token"})
		return
	}

	var req ManualRenderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	log.Printf("📋 [ManualRender] 接收到的请求参数: FileKey=%s, NodeIDs数量=%d, ProjectIDs=%v, Format=%s, Scale=%.1f",
		req.FileKey, len(req.NodeIDs), req.ProjectIDs, req.Format, req.Scale)

	// 使用渲染批次服务创建批次
	batch, err := cc.batchService.CreateRenderBatch(
		user.ID,
		req.ProjectIDs,
		req.FileKey,
		req.NodeIDs,
		req.Format,
		req.Scale,
		user.FigmaToken,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建渲染批次失败: " + err.Error(),
		})
		return
	}

	log.Printf("✅ [ManualRender] 渲染批次已创建: BatchID=%d, TotalQueues=%d, TotalNodes=%d",
		batch.ID, batch.TotalQueues, batch.TotalNodes)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "渲染批次已创建",
		"data": gin.H{
			"batch_id":     batch.ID,
			"total_queues": batch.TotalQueues,
			"total_nodes":  batch.TotalNodes,
		},
	})
}

// RefreshProjectNodeTree 刷新项目节点树（兼容旧API）
// GET /figma/project/:project_id/node-tree/refresh
func (cc *CacheController) RefreshProjectNodeTree(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "未登录",
		})
		return
	}

	// 获取项目ID
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的项目ID",
		})
		return
	}

	// 获取项目信息
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在",
		})
		return
	}

	// 验证项目所有者
	if project.UserID != userID.(uint) {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "您没有权限访问此项目",
		})
		return
	}

	// 获取用户信息
	user, err := models.FindUserByID(userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取用户信息失败",
		})
		return
	}

	if user.FigmaToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "用户未设置Figma Token",
		})
		return
	}

	// 检查冷却状态
	canRequest, waitSeconds, err := cc.cacheService.CheckFileAPICooldown(user.FigmaToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "检查冷却状态失败: " + err.Error(),
		})
		return
	}

	// 如果还在冷却中，创建或更新文件缓存记录到队列，等待调度器处理
	if !canRequest {
		log.Printf("⏰ [RefreshProjectNodeTree] Token 冷却中，加入队列 (ProjectID=%d, WaitSeconds=%d)",
			projectID, waitSeconds)

		// 检查是否已存在缓存记录
		existingCache, err := models.GetFileCache(project.FileKey, project.RootNodeID)

		if err != nil || existingCache == nil {
			// 创建新的缓存记录（状态为 waiting）
			cache := &models.FigmaFileCache{
				FigmaToken: user.FigmaToken,
				FileKey:    project.FileKey,
				RootNodeID: project.RootNodeID,
				NodeIDs:    "",
				FileData:   "",
				Status:     "waiting",
			}
			if err := models.CreateFileCache(cache); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "创建刷新队列失败: " + err.Error(),
				})
				return
			}
			log.Printf("✅ [RefreshProjectNodeTree] 已创建刷新队列 (Token=%s...)", user.FigmaToken[:10])
			log.Printf("✅ [RefreshProjectNodeTree] 已创建刷新队列 (CacheID=%d)",
				cache.ID)
		} else {
			// 更新现有缓存记录的状态
			if err := cc.cacheService.UpdateFileCacheStatus(existingCache.ID, "waiting", ""); err != nil {
				log.Printf("⚠️ [RefreshProjectNodeTree] 更新缓存状态失败: %v", err)
			}
			log.Printf("✅ [RefreshProjectNodeTree] 已更新刷新队列 (CacheID=%d)",
				existingCache.ID)
		}

		c.JSON(http.StatusAccepted, gin.H{
			"success":            true,
			"message":            "请求已加入队列，等待处理",
			"status":             "queued",
			"cooldown_remaining": waitSeconds,
		})
		return
	}

	// Token 冷却完成，可以直接调用 API
	// 更新冷却时间（在调用 API 之前）
	if err := cc.cacheService.UpdateFileRequestTime(user.FigmaToken); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "更新冷却时间失败: " + err.Error(),
		})
		return
	}

	// 直接调用 Figma API 获取节点树（跳过缓存，GetFigmaNodesNoCache 会自动保存到数据库）
	nodes, fetchErr := services.GetFigmaNodesNoCache(user.FigmaToken, project.FileKey, project.RootNodeID)
	if fetchErr != nil {
		// Figma API 请求失败，尝试从数据库恢复数据
		cache, hit, err := cc.cacheService.GetFileCache(project.FileKey, project.RootNodeID)
		if err == nil && hit && cache != nil && cache.FileData != "" {
			// 从数据库恢复成功
			var cachedNodes []map[string]interface{}
			if err := json.Unmarshal([]byte(cache.FileData), &cachedNodes); err == nil {
				c.JSON(http.StatusOK, gin.H{
					"success": true,
					"status":  "cached", // 使用缓存数据
					"message": "Figma API 请求失败，已返回缓存数据",
					"warning": "数据可能不是最新的",
					"nodes":   cachedNodes,
					"error":   fetchErr.Error(),
				})
				return
			}
		}

		// 无法从数据库恢复，返回错误
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"status":  "error",
			"error":   "获取Figma节点树失败: " + fetchErr.Error(),
		})
		return
	}
	// 返回节点数据（兼容旧API格式）
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"status":  "success", // 刷新成功
		"message": "节点树刷新成功",
		"nodes":   nodes,
	})
}

// BatchRefreshNodeTreeRequest 批量刷新节点树请求
type BatchRefreshNodeTreeRequest struct {
	FileKey string   `json:"file_key" binding:"required"`
	NodeIDs []string `json:"node_ids" binding:"required,min=1"`
}

// BatchRefreshNodeTree 批量刷新节点树（支持多个root节点）
// POST /api/figma/batch-refresh-node-tree
func (cc *CacheController) BatchRefreshNodeTree(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "未登录",
		})
		return
	}

	// 获取用户信息
	user, err := models.FindUserByID(userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取用户信息失败",
		})
		return
	}

	if user.FigmaToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "用户未设置Figma Token",
		})
		return
	}

	// 解析请求参数
	var req BatchRefreshNodeTreeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "参数错误: " + err.Error(),
		})
		return
	}

	log.Printf("📋 [BatchRefreshNodeTree] 批量刷新节点树: FileKey=%s, NodeIDs=%v", req.FileKey, req.NodeIDs)

	// 检查冷却状态
	canRequest, waitSeconds, err := cc.cacheService.CheckFileAPICooldown(user.FigmaToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "检查冷却状态失败: " + err.Error(),
		})
		return
	}
	// 立即记录冷却时间，防止重复调用
	if err := cc.cacheService.UpdateFileRequestTime(user.FigmaToken); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "立即记录冷却时间失败: " + err.Error(),
		})
		return
	}

	// 如果还在冷却中，返回队列状态
	if !canRequest {
		log.Printf("⏰ [BatchRefreshNodeTree] Token 冷却中，等待 %d 秒", waitSeconds)

		// 为每个节点创建或更新缓存记录到队列
		for _, nodeID := range req.NodeIDs {
			existingCache, err := models.GetFileCache(req.FileKey, nodeID)
			if err != nil || existingCache == nil {
				// 创建新的缓存记录（状态为 waiting）
				cache := &models.FigmaFileCache{
					FigmaToken: user.FigmaToken,
					FileKey:    req.FileKey,
					RootNodeID: nodeID,
					NodeIDs:    "",
					FileData:   "",
					Status:     "waiting",
				}
				if err := models.CreateFileCache(cache); err != nil {
					log.Printf("⚠️ [BatchRefreshNodeTree] 创建缓存记录失败: NodeID=%s, Error=%v", nodeID, err)
				} else {
					log.Printf("✅ [BatchRefreshNodeTree] 已创建刷新队列: CacheID=%d, NodeID=%s", cache.ID, nodeID)
				}
			} else {
				// 更新现有缓存记录的状态
				if err := cc.cacheService.UpdateFileCacheStatus(existingCache.ID, "waiting", ""); err != nil {
					log.Printf("⚠️ [BatchRefreshNodeTree] 更新缓存状态失败: CacheID=%d, Error=%v", existingCache.ID, err)
				} else {
					log.Printf("✅ [BatchRefreshNodeTree] 已更新刷新队列: CacheID=%d, NodeID=%s", existingCache.ID, nodeID)
				}
			}
		}

		c.JSON(http.StatusAccepted, gin.H{
			"success":            true,
			"message":            "请求已加入队列，等待处理",
			"status":             "queued",
			"cooldown_remaining": waitSeconds,
			"queued_nodes":       len(req.NodeIDs),
		})
		return
	}

	// Token 冷却完成，可以直接调用 API
	// 更新冷却时间（在调用 API 之前）
	if err := cc.cacheService.UpdateFileRequestTime(user.FigmaToken); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "更新冷却时间失败: " + err.Error(),
		})
		return
	}

	// 批量调用 Figma API 获取节点树（一次请求多个节点）
	log.Printf("🚀 [BatchRefreshNodeTree] 开始调用 Figma API: FileKey=%s, NodeIDs=%v", req.FileKey, req.NodeIDs)

	allNodes, fetchErr := services.GetFigmaNodesBatch(user.FigmaToken, req.FileKey, req.NodeIDs)
	if fetchErr != nil {
		// Figma API 请求失败，尝试从数据库恢复数据
		log.Printf("❌ [BatchRefreshNodeTree] Figma API 请求失败: %v", fetchErr)

		// 尝试返回缓存数据
		cachedResults := make(map[string]interface{})
		hasCache := false

		for _, nodeID := range req.NodeIDs {
			cache, hit, err := cc.cacheService.GetFileCache(req.FileKey, nodeID)
			if err == nil && hit && cache != nil && cache.FileData != "" {
				var cachedNodes []map[string]interface{}
				if err := json.Unmarshal([]byte(cache.FileData), &cachedNodes); err == nil {
					cachedResults[nodeID] = cachedNodes
					hasCache = true
				}
			}
		}

		if hasCache {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"status":  "cached",
				"message": "Figma API 请求失败，已返回缓存数据",
				"warning": "数据可能不是最新的",
				"nodes":   cachedResults,
				"error":   fetchErr.Error(),
			})
			return
		}

		// 无法从数据库恢复，返回错误
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"status":  "error",
			"error":   "获取Figma节点树失败: " + fetchErr.Error(),
		})
		return
	}

	log.Printf("✅ [BatchRefreshNodeTree] 批量刷新成功: 获取了 %d 个节点的数据", len(allNodes))

	// 返回节点数据
	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"status":        "success",
		"message":       "节点树批量刷新成功",
		"nodes":         allNodes,
		"updated_nodes": len(allNodes),
	})
}
