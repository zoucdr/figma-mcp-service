package controllers

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/figma-deliver/internal/middleware"
	"github.com/figma-deliver/internal/models"
	"github.com/figma-deliver/internal/services"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// imageRequest 表示一个图片请求的结果
type imageRequest struct {
	imagePath    string
	err          error
	done         chan struct{}
	waitingCount int // 等待此请求的数量
}

// imageRequestQueue 图片请求队列管理器
type imageRequestQueue struct {
	mu       sync.Mutex
	requests map[string]*imageRequest
}

// 全局图片请求队列
var globalImageQueue = &imageRequestQueue{
	requests: make(map[string]*imageRequest),
}

// 全局 OBS 服务实例（由 main.go 初始化）
var globalOBSService *services.OBSService

// SetGlobalOBSService 设置全局 OBS 服务实例
func SetGlobalOBSService(obs *services.OBSService) {
	globalOBSService = obs
}

// generateRequestKey 生成请求的唯一标识
func generateRequestKey(fileKey, nodeID, format string, scale float64, excludeModified string, ignoreTexts bool, projectID uint64) string {
	key := fmt.Sprintf("%s:%s:%s:%.1f:%s:%v:%d", fileKey, nodeID, format, scale, excludeModified, ignoreTexts, projectID)
	hash := md5.Sum([]byte(key))
	return hex.EncodeToString(hash[:])
}

// getOrWaitForImage 获取图片或等待正在进行的请求
func (q *imageRequestQueue) getOrWaitForImage(
	requestKey string,
	fetchFunc func() (string, error),
) (string, error) {
	q.mu.Lock()

	// 检查是否已有相同的请求在进行
	if req, exists := q.requests[requestKey]; exists {
		// 已有请求在进行，增加等待计数
		req.waitingCount++
		currentWaiting := req.waitingCount
		q.mu.Unlock()

		fmt.Printf("🔄 ImageQueue: 发现重复请求(第%d个等待者)，等待已有请求完成 [key=%s]\n", currentWaiting, requestKey)
		<-req.done // 等待请求完成
		fmt.Printf("✅ ImageQueue: 等待完成，共享结果 [key=%s, path=%s]\n", requestKey, req.imagePath)
		return req.imagePath, req.err
	}

	// 创建新的请求
	req := &imageRequest{
		done:         make(chan struct{}),
		waitingCount: 0,
	}
	q.requests[requestKey] = req
	q.mu.Unlock()

	fmt.Printf("🆕 ImageQueue: 首次请求，开始获取图片 [key=%s]\n", requestKey)

	// 执行实际的获取操作
	imagePath, err := fetchFunc()

	// 更新请求结果
	req.imagePath = imagePath
	req.err = err

	// 从队列中移除并通知所有等待的请求
	q.mu.Lock()
	waitingCount := req.waitingCount
	delete(q.requests, requestKey)
	q.mu.Unlock()

	close(req.done) // 通知所有等待的请求

	if err != nil {
		fmt.Printf("❌ ImageQueue: 请求失败 [key=%s, error=%v]\n", requestKey, err)
	} else {
		if waitingCount > 0 {
			fmt.Printf("✅ ImageQueue: 请求完成，通知 %d 个等待请求 [key=%s, path=%s]\n", waitingCount, requestKey, imagePath)
		} else {
			fmt.Printf("✅ ImageQueue: 请求完成(无等待者) [key=%s, path=%s]\n", requestKey, imagePath)
		}
	}

	return imagePath, err
}

// ParseFigmaLink 解析Figma链接
func ParseFigmaLink(c *gin.Context) {
	link := c.PostForm("link")
	name := c.PostForm("name")
	groupName := c.PostForm("group_name") // 添加分组名称参数

	if link == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Figma链接不能为空",
		})
		return
	}

	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目名称不能为空",
		})
		return
	}

	fileKey, nodeId, err := services.ParseFigmaLink(link)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "解析Figma链接失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"fileKey":   fileKey,
		"nodeId":    nodeId,
		"name":      name,
		"groupName": groupName, // 返回分组名称
	})
}

// GetFigmaNode 获取Figma节点
func GetFigmaNode(c *gin.Context) {
	userID := middleware.GetUserID(c)
	fileKey := c.Param("file_key")
	nodeID := c.Param("node_id")

	if fileKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Figma文件Key不能为空",
		})
		return
	}

	// 获取项目名称，默认为"Figma项目 - {fileKey}"
	projectName := c.Query("name")
	if projectName == "" {
		projectName = fmt.Sprintf("Figma项目 - %s", fileKey)
	}

	// 获取Figma URL参数
	figmaURL := c.Query("figma_url")

	// 获取分组名称参数
	groupName := c.Query("group_name")

	// 创建或获取项目，只保存到figma_projects表
	project, err := services.GetOrCreateFigmaProjectWithGroup(userID, fileKey, nodeID, projectName, figmaURL, groupName)
	if err != nil {
		// 检查是否是用户不存在的错误
		if strings.Contains(err.Error(), "用户不存在") {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "用户不存在，请确保您已正确登录",
			})
			return
		}

		// 检查是否是外键约束错误
		if strings.Contains(err.Error(), "foreign key constraint fails") {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "外键约束错误：用户ID不存在，请确保您已正确登录",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取Figma项目失败: " + err.Error(),
		})
		return
	}

	// 不再保存节点数据到数据库，直接返回项目信息
	c.JSON(http.StatusOK, gin.H{
		"project": project,
		"message": "项目创建成功，节点信息将在需要时通过API获取",
	})
}

// GetFigmaNodeTree 获取Figma节点树
func GetFigmaNodeTree(c *gin.Context) {
	userID := middleware.GetUserID(c)
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
	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "您没有权限访问此项目",
		})
		return
	}

	// 标准化节点ID格式：如果 RootNodeID 包含 "-"，转换为 ":"
	// 例如：1-123 → 1:123
	rootNodeID := project.RootNodeID
	if strings.Contains(rootNodeID, "-") {
		rootNodeID = strings.ReplaceAll(rootNodeID, "-", ":")
		log.Printf("🔄 [GetFigmaNodeTree] 转换节点ID格式: %s → %s", project.RootNodeID, rootNodeID)
	}

	// 获取节点树，查找顺序：
	// 1. 文件缓存（最快）
	// 2. 数据库缓存
	// 3. WebSocket 插件（兜底，需要活跃连接）
	var nodes []map[string]interface{}
	var cacheSource string
	var needWebSocketFallback bool = false

	// 1. 先尝试从文件缓存获取（temp/{fileKey}/documents/{nodeID}.json）
	fileNodes, err := services.LoadCachedNodesFromFile(project.FileKey, rootNodeID)
	if err == nil && fileNodes != nil && len(fileNodes) > 0 {
		nodes = fileNodes
		cacheSource = "file"
		log.Printf("✅ [GetFigmaNodeTree] 从文件缓存获取节点树 (FileKey=%s, NodeID=%s)",
			project.FileKey, rootNodeID)
	} else {
		// 2. 文件缓存未找到，尝试从数据库缓存获取
		log.Printf("⚠️ [GetFigmaNodeTree] 文件缓存未找到，尝试数据库缓存 (FileKey=%s, NodeID=%s)",
			project.FileKey, rootNodeID)

		cache, err := models.GetFileCache(project.FileKey, rootNodeID)
		if err != nil || cache == nil {
			// 尝试查找包含该节点的父缓存树
			cache, err = models.GetFileCacheContainingNode(project.FileKey, rootNodeID)
		}

		if err != nil || cache == nil || cache.FileData == "" {
			// 数据库缓存也未找到，标记需要 WebSocket 插件兜底
			log.Printf("⚠️ [GetFigmaNodeTree] 数据库缓存未找到，尝试 WebSocket 插件兜底 (FileKey=%s, NodeID=%s)",
				project.FileKey, rootNodeID)
			needWebSocketFallback = true
		} else {
			// 使用与 LoadCachedNodesFromFile 相同的解析逻辑
			var fileDataToUse string

			// 如果需要提取子树
			if cache.RootNodeID != rootNodeID {
				// 找到的是父缓存树，需要提取子树
				subtreeData, err := models.ExtractSubtree(cache.FileData, rootNodeID)
				if err != nil {
					log.Printf("❌ [GetFigmaNodeTree] 提取子树失败: %v", err)
					needWebSocketFallback = true
				} else {
					fileDataToUse = subtreeData
				}
			} else {
				fileDataToUse = cache.FileData
			}

			// 只有成功获取数据才继续解析
			if !needWebSocketFallback {
				// 解析 JSON 数据
				var result map[string]interface{}
				if err := json.Unmarshal([]byte(fileDataToUse), &result); err != nil {
					log.Printf("❌ [GetFigmaNodeTree] 解析数据库缓存JSON失败: %v", err)
					needWebSocketFallback = true
				} else {
					// 使用与 LoadCachedNodesFromFile 相同的 parseFigmaNodes 函数
					dbNodes, err := services.ParseFigmaNodesFromData(result, rootNodeID)
					if err != nil {
						log.Printf("❌ [GetFigmaNodeTree] 解析节点数据失败: %v", err)
						needWebSocketFallback = true
					} else if len(dbNodes) == 0 {
						log.Printf("⚠️ [GetFigmaNodeTree] 数据库缓存解析后无有效节点")
						needWebSocketFallback = true
					} else {
						nodes = dbNodes
						cacheSource = "database"
						log.Printf("✅ [GetFigmaNodeTree] 从数据库缓存获取节点树 (FileKey=%s, NodeID=%s, 节点数=%d)",
							project.FileKey, rootNodeID, len(nodes))
					}
				}
			}
		}
	}

	// 3. 如果缓存都未找到，尝试通过 WebSocket 请求 Figma 插件获取
	if needWebSocketFallback {
		// 获取 MCP 服务
		mcpService := services.GetMCPService()

		// 查找用户的 MCP 连接
		// 需要先找到用户的 connection_id
		connection, exists := mcpService.GetUserConnection(userID)

		if exists && connection != nil && mcpService.HasActiveWebSocketConnection(connection.ConnectionID) {
			log.Printf("🔌 [GetFigmaNodeTree] 发现活跃 WebSocket 连接 (ConnectionID=%s)，尝试请求插件获取数据", connection.ConnectionID)

			// 构造请求ID和消息（使用与 mcp_tools.go 相同的格式）
			requestID := fmt.Sprintf("req_nodetree_%d", time.Now().UnixNano())

			// 创建响应通道
			responseChan := make(chan interface{}, 1)
			errorChan := make(chan error, 1)

			// 注册请求等待响应
			mcpService.RegisterPendingRequest(requestID, responseChan, errorChan)
			defer mcpService.UnregisterPendingRequest(requestID)

			message := map[string]interface{}{
				"id":      requestID,
				"command": "read_my_design",
				"params": map[string]interface{}{
					"nodeId": rootNodeID,
				},
			}

			// 获取频道
			channel := mcpService.GetUserChannel(connection.ConnectionID)
			if channel == "" {
				channel = "figma-bridge"
				_ = mcpService.JoinChannel(connection.ConnectionID, channel)
			}

			// 广播消息到频道
			if err := mcpService.BroadcastToChannel(connection.ConnectionID, channel, message); err != nil {
				log.Printf("❌ [GetFigmaNodeTree] 发送插件请求失败: %v", err)
				c.JSON(http.StatusServiceUnavailable, gin.H{
					"error": "发送请求到 Figma 插件失败: " + err.Error(),
				})
				return
			}

			log.Printf("✅ [GetFigmaNodeTree] 已发送请求到频道 %s，等待响应...", channel)

			// 等待响应（30秒超时）
			var response interface{}
			select {
			case response = <-responseChan:
				log.Printf("✅ [GetFigmaNodeTree] 收到响应 (ID=%s)", requestID)
			case respErr := <-errorChan:
				log.Printf("❌ [GetFigmaNodeTree] 收到错误响应 (ID=%s): %v", requestID, respErr)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": fmt.Sprintf("通过 Figma 插件获取数据失败: %v", respErr),
				})
				return
			case <-time.After(30 * time.Second):
				log.Printf("⏰ [GetFigmaNodeTree] 请求超时 (ID=%s)", requestID)
				c.JSON(http.StatusRequestTimeout, gin.H{
					"error": "请求超时，未在 30 秒内收到插件响应",
				})
				return
			}

			log.Printf("✅ [GetFigmaNodeTree] 收到插件响应，解析节点数据")

			// 解析响应数据
			responseData, ok := response.(map[string]interface{})
			if !ok {
				log.Printf("❌ [GetFigmaNodeTree] 响应数据格式错误")
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "插件响应数据格式错误",
				})
				return
			}

			// 将响应数据保存到本地缓存和数据库
			responseJSON, err := json.Marshal(responseData)
			if err != nil {
				log.Printf("⚠️ [GetFigmaNodeTree] 序列化响应数据失败: %v", err)
			} else {
				// 保存到本地缓存（异步）
				go func() {
					if err := services.SaveCachedNodesToFile(project.FileKey, rootNodeID, responseJSON); err != nil {
						log.Printf("⚠️ [GetFigmaNodeTree] 保存到本地缓存失败: %v", err)
					} else {
						log.Printf("✅ [GetFigmaNodeTree] 已保存到本地缓存")
					}
				}()

				// 保存到数据库缓存（异步）
				go func() {
					// 提取所有可见节点ID（排除 visible=false 的节点）
					nodeIDList, err := models.ExtractVisibleNodeIDs(string(responseJSON))
					if err != nil {
						log.Printf("⚠️ [GetFigmaNodeTree] 提取节点ID失败: %v", err)
						nodeIDList = []string{}
					}
					nodeIDsStr := strings.Join(nodeIDList, ",")
					now := uint32(time.Now().Unix())

					// 先尝试获取现有缓存
					existingCache, err := models.GetFileCache(project.FileKey, rootNodeID)
					if err == nil && existingCache != nil {
						// 更新现有缓存
						existingCache.FileData = string(responseJSON)
						existingCache.NodeIDs = nodeIDsStr
						existingCache.UpdatedAt = now
						if err := models.UpdateFileCache(existingCache); err != nil {
							log.Printf("⚠️ [GetFigmaNodeTree] 更新数据库缓存失败: %v", err)
						} else {
							log.Printf("✅ [GetFigmaNodeTree] 已更新数据库缓存 (节点数=%d)", len(nodeIDList))
						}
					} else {
						// 创建新缓存
						newCache := &models.FigmaFileCache{
							FileKey:    project.FileKey,
							RootNodeID: rootNodeID,
							FileData:   string(responseJSON),
							NodeIDs:    nodeIDsStr,
							CreatedAt:  now,
							UpdatedAt:  now,
						}
						if err := models.CreateFileCache(newCache); err != nil {
							log.Printf("⚠️ [GetFigmaNodeTree] 创建数据库缓存失败: %v", err)
						} else {
							log.Printf("✅ [GetFigmaNodeTree] 已创建数据库缓存 (节点数=%d)", len(nodeIDList))
						}
					}
				}()
			}

			// 解析节点数据
			pluginNodes, err := services.ParseFigmaNodesFromData(responseData, rootNodeID)
			if err != nil {
				log.Printf("❌ [GetFigmaNodeTree] 解析节点数据失败: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": fmt.Sprintf("解析节点数据失败: %v", err),
				})
				return
			}

			if len(pluginNodes) == 0 {
				log.Printf("⚠️ [GetFigmaNodeTree] 插件返回的数据解析后无有效节点")
				c.JSON(http.StatusNotFound, gin.H{
					"error": "未找到有效的节点数据",
				})
				return
			}

			nodes = pluginNodes
			cacheSource = "plugin"
			log.Printf("✅ [GetFigmaNodeTree] 从插件获取节点树 (FileKey=%s, NodeID=%s, 节点数=%d)",
				project.FileKey, rootNodeID, len(nodes))

		} else {
			log.Printf("⚠️ [GetFigmaNodeTree] 未找到活跃 WebSocket 连接，无法获取数据")
			c.JSON(http.StatusNotFound, gin.H{
				"error": "节点树缓存不存在，且未检测到活跃的 Figma 插件连接。请打开 Figma 插件并连接后重试。",
			})
			return
		}
	}

	// 获取所有节点的修改信息
	var figmaNodes []models.FigmaNode
	dbResult := models.DB.Where("project_id = ?", projectID).Find(&figmaNodes)
	if dbResult.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取节点修改信息失败: " + dbResult.Error.Error(),
		})
		return
	}

	// 构建节点ID到修改信息的映射
	nodeModifys := make(map[string]map[string]interface{})
	for _, node := range figmaNodes {
		if node.Modifys != "" {
			var modifyData map[string]interface{}
			if err := json.Unmarshal([]byte(node.Modifys), &modifyData); err == nil {
				nodeModifys[node.NodeID] = modifyData
			}
		}
	}

	// 应用修改信息到节点数据（保持扁平结构用于前端兼容）
	processedNodes := applyModificationsToFlatNodes(nodes, nodeModifys)

	// 构建响应
	response := gin.H{
		"nodes":        processedNodes,
		"cache_source": cacheSource, // "file", "database" 或 "api"
	}

	// 添加数据来源提示
	switch cacheSource {
	case "file":
		response["message"] = "数据来自文件缓存"
	case "database":
		response["from_database"] = true
		response["message"] = "数据来自数据库缓存"
	case "api":
		response["from_api"] = true
		response["message"] = "数据来自 Figma API（已自动缓存）"
	}

	c.JSON(http.StatusOK, response)
}

// applyModificationsToFlatNodes 将修改信息应用到扁平节点数据（用于前端兼容）
func applyModificationsToFlatNodes(nodes []map[string]interface{}, nodeModifys map[string]map[string]interface{}) []map[string]interface{} {
	processedNodes := make([]map[string]interface{}, 0)

	for _, node := range nodes {
		nodeID, ok := node["id"].(string)
		if !ok {
			continue
		}

		// 复制节点数据
		processedNode := make(map[string]interface{})
		for key, value := range node {
			processedNode[key] = value
		}

		// 保存原始名称（在应用修改信息之前）
		originalName, _ := node["name"].(string)
		if originalName == "" {
			originalName, _ = node["id"].(string)
		}
		processedNode["originalName"] = originalName

		// 应用修改信息
		if modifys, exists := nodeModifys[nodeID]; exists {
			// 处理rename - 直接替换name字段
			if rename, ok := modifys["rename"].(string); ok && rename != "" {
				processedNode["name"] = rename
			}

			// 将所有修改信息添加到modifys字段（保持前端兼容性）
			processedNode["modifys"] = modifys
		}

		processedNodes = append(processedNodes, processedNode)
	}

	return processedNodes
}

// RefreshFigmaNodeTree 刷新Figma节点树
func RefreshFigmaNodeTree(c *gin.Context) {
	// 直接调用GetFigmaNodeTree，但添加refresh=true参数
	// 在查询参数中添加refresh=true
	c.Request.URL.RawQuery = "refresh=true"
	// 调用GetFigmaNodeTree处理函数
	GetFigmaNodeTree(c)
}

// GetProjectNodeModifys 获取项目的节点修改信息
func GetProjectNodeModifys(c *gin.Context) {
	userID := middleware.GetUserID(c)
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
	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "您没有权限访问此项目",
		})
		return
	}

	// 获取项目的所有节点修改信息
	var nodes []models.FigmaNode
	result := models.DB.Where("project_id = ?", projectID).Find(&nodes)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取节点修改信息失败: " + result.Error.Error(),
		})
		return
	}

	// 构建节点ID到修改信息的映射
	nodeModifys := make(map[string]interface{})
	for _, node := range nodes {
		if node.Modifys != "" {
			var modifyData interface{}
			if err := json.Unmarshal([]byte(node.Modifys), &modifyData); err == nil {
				nodeModifys[node.NodeID] = modifyData
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"modifys": nodeModifys,
	})
}

// UpdateNodeSettings 更新节点设置
func UpdateNodeSettings(c *gin.Context) {
	// 解析请求体为通用JSON
	var requestData map[string]interface{}
	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// 获取节点ID
	nodeID := c.Param("node_id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "节点ID不能为空",
		})
		return
	}

	// 获取项目ID
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID无效",
		})
		return
	}

	// 将设置转换为JSON字符串
	settingsJSON, err := json.Marshal(requestData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "处理设置数据失败: " + err.Error(),
		})
		return
	}

	// 创建或更新节点的Modifys字段
	err = models.SaveNodeModifys(uint(projectID), nodeID, string(settingsJSON))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "保存节点设置失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "节点设置更新成功",
		"settings": requestData,
	})
}

// GetNodeSettings 获取节点设置
func GetNodeSettings(c *gin.Context) {
	// 获取项目ID
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID无效",
		})
		return
	}

	nodeID := c.Param("node_id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "节点ID不能为空",
		})
		return
	}

	// 获取节点修改信息
	modifys, err := models.GetNodeModifys(uint(projectID), nodeID)
	if err != nil {
		// 如果节点不存在，返回空设置
		if err.Error() == "record not found" {
			c.JSON(http.StatusOK, gin.H{
				"settings": map[string]interface{}{},
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取节点设置失败: " + err.Error(),
		})
		return
	}

	// 解析JSON字符串为map
	var settings map[string]interface{}
	if modifys != "" {
		if err := json.Unmarshal([]byte(modifys), &settings); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "解析节点设置失败: " + err.Error(),
			})
			return
		}
	} else {
		settings = map[string]interface{}{}
	}

	c.JSON(http.StatusOK, gin.H{
		"settings": settings,
	})
}

// DeleteNodeSettings 删除节点设置
// 支持查询参数 subtree=true 来删除节点及其子树的设置
func DeleteNodeSettings(c *gin.Context) {
	userID := middleware.GetUserID(c)

	// 获取项目ID
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID无效",
		})
		return
	}

	nodeID := c.Param("node_id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "节点ID不能为空",
		})
		return
	}

	// 检查是否需要删除子树
	subtree := c.Query("subtree") == "true"

	// 验证项目权限
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在",
		})
		return
	}

	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "您没有权限访问此项目",
		})
		return
	}

	if subtree {
		// 获取用户信息以获取Figma Token
		user, err := models.EnsureUserExists(userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "获取用户信息失败: " + err.Error(),
			})
			return
		}

		if user.FigmaToken == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "请先在个人资料页面配置 Figma Token",
			})
			return
		}

		// 如果节点ID为空或等于根节点ID，重置整个项目
		var subtreeNodeIDs []string
		if nodeID == "" || nodeID == project.RootNodeID {
			// 重置根节点及其子树的修改
			subtreeNodeIDs, err = services.GetNodeSubtreeIDs(user.FigmaToken, project.FileKey, project.RootNodeID, "", user.ID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "获取根节点子树失败: " + err.Error(),
				})
				return
			}
		} else {
			// 重置指定节点及其子树的修改
			subtreeNodeIDs, err = services.GetNodeSubtreeIDs(user.FigmaToken, project.FileKey, project.RootNodeID, nodeID, user.ID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "获取节点子树失败: " + err.Error(),
				})
				return
			}
		}

		// 删除节点及其子树的修改信息，并获取实际被删除的节点ID列表
		actualDeletedNodeIDs, err := models.DeleteNodeSubtreeModifysAndReturnDeleted(uint(projectID), subtreeNodeIDs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "删除节点子树设置失败: " + err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":        fmt.Sprintf("节点及其子树设置删除成功，共删除 %d 个节点的设置", len(actualDeletedNodeIDs)),
			"deleted_count":  len(actualDeletedNodeIDs),
			"subtree":        true,
			"reset_node_ids": actualDeletedNodeIDs,
		})
	} else {
		// 只删除单个节点的修改信息，先检查节点是否有修改
		actualDeletedNodeIDs, err := models.DeleteNodeSubtreeModifysAndReturnDeleted(uint(projectID), []string{nodeID})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "删除节点设置失败: " + err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":        "节点设置删除成功",
			"deleted_count":  len(actualDeletedNodeIDs),
			"subtree":        false,
			"reset_node_ids": actualDeletedNodeIDs,
		})
	}
}

// DeleteProjectNodeSettings 重置项目根节点及其子树的修改
func DeleteProjectNodeSettings(c *gin.Context) {
	userID := middleware.GetUserID(c)

	// 获取项目ID
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID无效",
		})
		return
	}

	// 验证项目是否属于当前用户
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在",
		})
		return
	}

	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "您没有权限访问此项目",
		})
		return
	}

	// 获取用户信息以获取Figma Token
	user, err := models.EnsureUserExists(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取用户信息失败: " + err.Error(),
		})
		return
	}

	if user.FigmaToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请先在个人资料页面配置 Figma Token",
		})
		return
	}

	// 重置根节点及其子树的修改
	subtreeNodeIDs, err := services.GetNodeSubtreeIDs(user.FigmaToken, project.FileKey, project.RootNodeID, "", user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取根节点子树失败: " + err.Error(),
		})
		return
	}

	// 删除根节点及其子树的所有修改，并获取实际被删除的节点ID列表
	actualDeletedNodeIDs, err := models.DeleteNodeSubtreeModifysAndReturnDeleted(uint(projectID), subtreeNodeIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "重置根节点子树修改失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"message":        fmt.Sprintf("项目根节点及其子树修改已全部重置，共重置 %d 个节点", len(actualDeletedNodeIDs)),
		"deleted_count":  len(actualDeletedNodeIDs),
		"reset_node_ids": actualDeletedNodeIDs,
	})
}

// DeleteProjectModifications 删除项目的所有节点修改
func DeleteProjectModifications(c *gin.Context) {
	userID := middleware.GetUserID(c)

	// 获取项目ID
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "项目ID无效",
		})
		return
	}

	// 验证项目是否属于当前用户
	var project models.FigmaProject
	result := models.DB.First(&project, projectID)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "项目不存在",
		})
		return
	}

	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "无权限访问此项目",
		})
		return
	}

	// 删除项目的所有节点修改信息
	err = models.DeleteAllProjectNodeModifys(uint(projectID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "删除节点修改失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "项目节点修改已全部重置",
	})
}

// ExportFigmaDesign 导出Figma设计
func ExportFigmaDesign(c *gin.Context) {
	userID := middleware.GetUserID(c)
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID无效",
		})
		return
	}

	// 解析请求体中的参数
	var requestBody struct {
		Format string  `json:"format"`
		Scale  float64 `json:"scale"`
	}
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		// 如果解析失败，使用默认格式和缩放
		requestBody.Format = "png"
		requestBody.Scale = 1.0
	}

	// 从项目设置获取默认格式和缩放
	var defaultFormat string = "png"
	var defaultScale float64 = 1.0
	project, err := models.GetProjectByID(uint(projectID))
	if err == nil && project.Settings != "" {
		// 尝试将 Settings（JSON字符串）解析为 map[string]interface{}
		var settingsMap map[string]interface{}
		if err := json.Unmarshal([]byte(project.Settings), &settingsMap); err == nil {
			if v, ok := settingsMap["imageFormat"].(string); ok && v != "" {
				defaultFormat = v
			}
			if v, ok := settingsMap["imageScale"].(float64); ok && v > 0 {
				defaultScale = v
			} else if vInt, ok := settingsMap["imageScale"].(int); ok && vInt > 0 {
				defaultScale = float64(vInt)
			}
		}
	}

	// 验证格式是否有效
	format := requestBody.Format
	if format == "" {
		format = defaultFormat // 从项目设置获取默认格式
	}
	if format != "png" && format != "jpg" && format != "svg" {
		format = "png" // 如果不可用则强制为png
	}

	// 验证缩放比例是否有效
	scale := requestBody.Scale
	if scale <= 0 || scale > 4.0 {
		scale = defaultScale // 从项目设置获取默认缩放
	}

	// 先取消该项目的所有进行中的导出任务
	err = models.CancelExportJobsByProject(uint(projectID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "取消之前的导出任务失败: " + err.Error(),
		})
		return
	}

	// 创建导出任务，不需要前端提供路径，使用默认路径
	job := models.ExportJob{
		UserID:    userID,
		ProjectID: uint(projectID),
		Status:    "pending",
		Progress:  0,
		FilePath:  "", // 路径由服务端生成
		Format:    format,
		Scale:     scale,
	}

	result := models.DB.Create(&job)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "创建导出任务失败: " + result.Error.Error(),
		})
		return
	}

	// 启动导出任务（异步）
	go services.ProcessExportJob(job.ID)

	c.JSON(http.StatusOK, gin.H{
		"message": "导出任务已创建",
		"job_id":  job.ID,
	})
}

// GetExportStatus 获取导出状态
func GetExportStatus(c *gin.Context) {
	jobID, err := strconv.ParseUint(c.Param("job_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "任务ID无效",
		})
		return
	}

	// 获取任务状态
	job, err := models.GetExportJob(uint(jobID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "导出任务不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    job.Status,
		"progress":  job.Progress,
		"error":     job.Error,
		"file_path": job.FilePath,
	})
}

// GetProjectActiveExportJob 获取项目的活跃导出任务
func GetProjectActiveExportJob(c *gin.Context) {
	userID := middleware.GetUserID(c)
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID无效",
		})
		return
	}

	// 验证项目是否属于当前用户
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在",
		})
		return
	}

	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "无权访问此项目",
		})
		return
	}

	// 获取项目的活跃导出任务
	jobs, err := models.GetActiveExportJobsByProject(uint(projectID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取导出任务失败",
		})
		return
	}

	// 如果有活跃任务，返回最新的一个
	if len(jobs) > 0 {
		latestJob := jobs[len(jobs)-1] // 获取最新的任务
		c.JSON(http.StatusOK, gin.H{
			"has_active_job": true,
			"job_id":         latestJob.ID,
			"status":         latestJob.Status,
			"progress":       latestJob.Progress,
			"error":          latestJob.Error,
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"has_active_job": false,
		})
	}
}

// DownloadExport 下载导出文件
func DownloadExport(c *gin.Context) {
	jobID, err := strconv.ParseUint(c.Param("job_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "任务ID无效",
		})
		return
	}

	// 获取任务信息
	job, err := models.GetExportJob(uint(jobID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "导出任务不存在",
		})
		return
	}

	// 检查任务是否完成
	if job.Status != "completed" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "导出任务尚未完成",
		})
		return
	}

	// 检查文件是否存在
	if _, err := os.Stat(job.FilePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "导出文件不存在",
		})
		return
	}

	// 获取文件名
	fileName := filepath.Base(job.FilePath)

	// 设置下载头
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", "attachment; filename="+fileName)
	c.Header("Content-Type", "application/octet-stream")
	c.File(job.FilePath)
}

// GetFigmaImage 获取Figma图片
func GetFigmaImage(c *gin.Context) {
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID无效",
		})
		return
	}

	nodeID := c.Param("node_id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "节点ID不能为空",
		})
		return
	}

	// 获取查询参数
	format := c.DefaultQuery("format", "png")                // 默认png格式
	scaleStr := c.DefaultQuery("scale", "1.0")               // 默认1.0标准清晰度
	excludeModified := c.Query("excludeModified")            // 是否排除修改的节点（过滤预览）
	useCacheStr := c.DefaultQuery("useCache", "true")        // 是否使用缓存，默认为true
	ignoreTextsStr := c.DefaultQuery("ignoreTexts", "false") // 是否忽略文本节点
	ignoreNodesStr := c.Query("ignoreNodes")                 // 要忽略的节点ID列表（逗号分隔）

	// 解析缩放比例
	scale, err := strconv.ParseFloat(scaleStr, 64)
	if err != nil || scale <= 0 || scale > 4.0 {
		scale = 1.0 // 如果解析失败或超出范围，使用默认值
		fmt.Printf("Controller: 缩放比例参数无效，使用默认值: 1.0\n")
	}

	// 验证格式
	validFormats := map[string]bool{"png": true, "jpg": true, "svg": true}
	if !validFormats[format] {
		format = "png" // 如果格式无效，使用默认值
		fmt.Printf("Controller: 图片格式参数无效，使用默认值: png\n")
	}

	// 解析是否使用缓存
	useCache := useCacheStr == "true" || useCacheStr == "1"

	// 解析是否忽略文本节点
	ignoreTexts := ignoreTextsStr == "true" || ignoreTextsStr == "1"

	// 解析要忽略的节点ID列表
	var ignoreNodes []string
	if ignoreNodesStr != "" {
		// 按逗号分隔字符串，并去除空格
		for _, nodeID := range strings.Split(ignoreNodesStr, ",") {
			nodeID = strings.TrimSpace(nodeID)
			if nodeID != "" {
				ignoreNodes = append(ignoreNodes, nodeID)
			}
		}
	}

	fmt.Printf("Controller: 图片参数 - 格式: %s, 缩放比例: %.1f, 排除修改: %s, 使用缓存: %v, 忽略文本: %v, 忽略节点: %v\n", format, scale, excludeModified, useCache, ignoreTexts, ignoreNodes)

	// 获取项目信息
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在",
		})
		return
	}

	// ⚡ 立即生成请求唯一标识并检查队列（在任何耗时操作之前）
	requestKey := generateRequestKey(project.FileKey, nodeID, format, scale, excludeModified, ignoreTexts, projectID)

	// 使用队列管理器获取图片，防止重复请求
	// 将所有耗时操作（包括数据库查询和API调用）都放在队列的 fetchFunc 中
	imagePath, err := globalImageQueue.getOrWaitForImage(requestKey, func() (string, error) {
		// 在队列保护下获取用户信息
		user, err := models.EnsureUserExists(project.UserID)
		if err != nil {
			return "", fmt.Errorf("获取用户信息失败: %v", err)
		}

		var imagePath string

		// 1. 优先检查本地文件缓存
		if useCache {
			safeNodeID := strings.NewReplacer(":", "_", ";", "_", "/", "_", "\\", "_", "?", "_", "*", "_", "\"", "_", "<", "_", ">", "_", "|", "_").Replace(nodeID)
			tempDir := filepath.Join("temp", project.FileKey, "previews")

			// 根据ignoreTexts决定文件名后缀：x表示带文字，p表示不带文字
			suffix := "x"
			if ignoreTexts {
				suffix = "p"
			}

			// 如果有忽略节点，添加到后缀中
			if len(ignoreNodes) > 0 {
				// 对忽略节点列表进行排序以确保一致性
				sortedIgnoreNodes := make([]string, len(ignoreNodes))
				copy(sortedIgnoreNodes, ignoreNodes)
				sort.Strings(sortedIgnoreNodes)

				// 生成忽略节点的哈希值作为文件名的一部分
				ignoreNodesStr := strings.Join(sortedIgnoreNodes, ",")
				hash := md5.Sum([]byte(ignoreNodesStr))
				ignoreNodesHash := fmt.Sprintf("%x", hash)[:8]
				suffix = suffix + "i" + ignoreNodesHash
			}

			scaleStr := fmt.Sprintf("%.1fx", scale)
			ext := format

			// 尝试新格式的文件名: nodeID-scale[x|p]-hash.format
			if files, err := ioutil.ReadDir(tempDir); err == nil {
				for _, file := range files {
					fileName := file.Name()
					// 匹配格式: nodeID-scale[x|p][i-hash]*.format
					if strings.HasPrefix(fileName, safeNodeID+"-"+scaleStr+suffix) && strings.HasSuffix(fileName, "."+format) {
						localFilePath := filepath.Join(tempDir, fileName)
						log.Printf("✅ [GetFigmaImage] 从本地缓存返回图片(新格式): %s", localFilePath)
						return localFilePath, nil
					}
				}
			}

			// 回退到旧格式的文件名: nodeID_scalex.format
			localFilePath := filepath.Join(tempDir, fmt.Sprintf("%s_%.1fx.%s", safeNodeID, scale, ext))
			if _, err := os.Stat(localFilePath); err == nil {
				log.Printf("✅ [GetFigmaImage] 从本地缓存返回图片(旧格式): %s", localFilePath)
				return localFilePath, nil
			}
			log.Printf("⚠️ [GetFigmaImage] 本地缓存未找到: %s", localFilePath)
		}

		// 2. 本地缓存未找到，尝试通过 WebSocket 请求 Figma 插件获取实时图片
		mcpService := services.GetMCPService()
		if mcpService != nil {
			connection, exists := mcpService.GetUserConnection(user.ID)

			if exists && connection != nil && mcpService.HasActiveWebSocketConnection(connection.ConnectionID) {
				log.Printf("🔌 [GetFigmaImage] 本地缓存未找到，通过 WebSocket 请求插件获取实时图片")

				// 构造请求ID和消息
				requestID := fmt.Sprintf("req_image_%d", time.Now().UnixNano())

				// 创建响应通道
				responseChan := make(chan interface{}, 1)
				errorChan := make(chan error, 1)

				// 注册请求等待响应
				mcpService.RegisterPendingRequest(requestID, responseChan, errorChan)
				defer mcpService.UnregisterPendingRequest(requestID)

				// 构造参数
				params := map[string]interface{}{
					"nodeId":      nodeID,
					"format":      strings.ToUpper(format),
					"scale":       scale,
					"ignore_text": ignoreTexts,
				}

				// 如果有忽略节点列表，添加到参数中
				if len(ignoreNodes) > 0 {
					params["ignore_nodes"] = ignoreNodes
				}

				message := map[string]interface{}{
					"id":      requestID,
					"command": "export_node_as_image",
					"params":  params,
				}

				// 获取频道
				channel := mcpService.GetUserChannel(connection.ConnectionID)
				if channel == "" {
					channel = "figma-bridge"
					_ = mcpService.JoinChannel(connection.ConnectionID, channel)
				}

				// 广播消息到频道
				if err := mcpService.BroadcastToChannel(connection.ConnectionID, channel, message); err != nil {
					log.Printf("❌ [GetFigmaImage] 发送插件请求失败: %v", err)
				} else {
					log.Printf("✅ [GetFigmaImage] 已发送请求到频道 %s，等待响应...", channel)

					// 等待响应（30秒超时）
					select {
					case response := <-responseChan:
						log.Printf("✅ [GetFigmaImage] 收到插件响应")

						// 解析响应数据
						if responseData, ok := response.(map[string]interface{}); ok {
							// 提取图片数据
							if imageDataB64, hasImage := responseData["imageData"].(string); hasImage {
								// 处理图片：保存本地并上传到 OBS
								savedPath, handleErr := handleImageFromPlugin(project.FileKey, nodeID, format, scale, imageDataB64, responseData, ignoreTexts, ignoreNodes)
								if handleErr == nil && savedPath != "" {
									log.Printf("✅ [GetFigmaImage] 插件图片处理成功: %s", savedPath)
									return savedPath, nil
								}
								log.Printf("⚠️ [GetFigmaImage] 插件图片处理失败: %v", handleErr)
							}
						}

					case respErr := <-errorChan:
						log.Printf("❌ [GetFigmaImage] 收到错误响应: %v", respErr)

					case <-time.After(30 * time.Second):
						log.Printf("⏰ [GetFigmaImage] 请求超时")
					}
				}
			}
		}

		// 3. WebSocket不可用或失败，检查数据库缓存（OBS URL 或 Figma CDN URL）
		if useCache {
			nodeImage, err := models.GetNodeImage(project.FileKey, nodeID, format, scale, ignoreTexts)
			if err == nil && nodeImage != nil {
				// 3.1 优先从 OBS 下载
				if nodeImage.OBSKey != "" && globalOBSService != nil && globalOBSService.IsEnabled() {
					log.Printf("🔍 [GetFigmaImage] 发现数据库中的 OBS Key: %s", nodeImage.OBSKey)

					// 准备本地保存路径
					safeNodeID := strings.NewReplacer(":", "_", ";", "_", "/", "_", "\\", "_", "?", "_", "*", "_", "\"", "_", "<", "_", ">", "_", "|", "_").Replace(nodeID)
					tempDir := filepath.Join("temp", project.FileKey, "previews")
					os.MkdirAll(tempDir, os.ModePerm)

					scaleStr := fmt.Sprintf("%.1fx", scale)
					suffix := "x"
					if ignoreTexts {
						suffix = "p"
					}

					localFilePath := filepath.Join(tempDir, fmt.Sprintf("%s-%s%s.%s", safeNodeID, scaleStr, suffix, format))

					// 从 OBS 下载到本地
					imageData, _, err := globalOBSService.DownloadImage(nodeImage.OBSKey)
					if err == nil {
						// 保存到本地
						if saveErr := ioutil.WriteFile(localFilePath, imageData, 0644); saveErr == nil {
							log.Printf("✅ [GetFigmaImage] 从 OBS 下载并缓存到本地: %s", localFilePath)
							return localFilePath, nil
						}
					}
					log.Printf("⚠️ [GetFigmaImage] 从 OBS 下载失败: %v", err)
				}

				// 3.2 从 Figma CDN 下载
				if nodeImage.FigmaCDNURL != "" {
					log.Printf("🔍 [GetFigmaImage] 尝试从 Figma CDN 下载: %s", nodeImage.FigmaCDNURL)

					resp, err := http.Get(nodeImage.FigmaCDNURL)
					if err == nil && resp.StatusCode == http.StatusOK {
						defer resp.Body.Close()
						imageData, err := ioutil.ReadAll(resp.Body)
						if err == nil {
							// 保存到本地
							safeNodeID := strings.NewReplacer(":", "_", ";", "_", "/", "_", "\\", "_", "?", "_", "*", "_", "\"", "_", "<", "_", ">", "_", "|", "_").Replace(nodeID)
							tempDir := filepath.Join("temp", project.FileKey, "previews")
							os.MkdirAll(tempDir, os.ModePerm)

							scaleStr := fmt.Sprintf("%.1fx", scale)
							suffix := "x"
							if ignoreTexts {
								suffix = "p"
							}

							localFilePath := filepath.Join(tempDir, fmt.Sprintf("%s-%s%s.%s", safeNodeID, scaleStr, suffix, format))
							if saveErr := ioutil.WriteFile(localFilePath, imageData, 0644); saveErr == nil {
								log.Printf("✅ [GetFigmaImage] 从 Figma CDN 下载并缓存到本地: %s", localFilePath)
								return localFilePath, nil
							}
						}
					}
					log.Printf("⚠️ [GetFigmaImage] 从 Figma CDN 下载失败: %v", err)
				}
			}
		}

		// 4. 所有缓存都失败，使用 Figma API 兜底
		// TODO: 过滤预览功能暂时禁用，等待 DownloadFilteredPreviewFigmaImage 函数实现
		// if excludeModified == "true" {
		// 	// 过滤预览：排除修改的节点，保存到fpreviews文件夹
		// 	fmt.Printf("Controller: 开始获取过滤后的Figma图片, project_id=%d, node_id=%s, format=%s, scale=%.1f, useCache=%v\n",
		// 		projectID, nodeID, format, scale, useCache)
		// 	imagePath, imgErr = services.DownloadFilteredPreviewFigmaImage(user.FigmaToken, project.FileKey, nodeID, format, scale, uint(projectID), useCache)
		// } else {
		// 普通预览：保存到previews文件夹
		fmt.Printf("Controller: 开始获取普通Figma图片（支持 WebSocket 兜底）, project_id=%d, node_id=%s, format=%s, scale=%.1f, useCache=%v\n",
			projectID, nodeID, format, scale, useCache)
		imagePath, imgErr := services.DownloadPreviewFigmaImageWithOptions(user.FigmaToken, project.FileKey, nodeID, format, scale, useCache, user.ID, ignoreTexts)
		// }

		return imagePath, imgErr
	})

	if err != nil {
		errMsg := fmt.Sprintf("获取图片失败: %v", err)
		fmt.Println("Controller: " + errMsg)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": errMsg,
		})
		return
	}
	fmt.Printf("Controller: 成功获取Figma图片, path=%s\n", imagePath)
	// 如果 imagePath 是破图（固定位置），无需记录数据库等后续操作，直接返回
	if imagePath != "web/static/img/break.jpg" && imagePath != "web\\static\\img\\break.jpg" {
		// 后台检查并上传到 OBS（不阻塞返回）
		go checkAndUploadToOBS(project.FileKey, nodeID, format, scale, imagePath)
	}

	// 返回图片
	fmt.Printf("Controller: 返回图片文件: %s\n", imagePath)
	c.File(imagePath)
}

// handleImageFromPlugin 处理从 Figma 插件接收到的图片数据
// 参数：fileKey, nodeID, format, scale, imageDataB64(base64编码的图片), responseData(插件响应)
// 返回：本地保存路径, 错误
func handleImageFromPlugin(fileKey, nodeID, format string, scale float64, imageDataB64 string, responseData map[string]interface{}, ignoreTexts bool, ignoreNodes []string) (string, error) {
	log.Printf("🎨 [handleImageFromPlugin] 开始处理插件图片: fileKey=%s, nodeID=%s, format=%s, scale=%.1f, ignoreTexts=%v, ignoreNodes=%v", fileKey, nodeID, format, scale, ignoreTexts, ignoreNodes)

	// 1. 解码 base64 图片数据
	imageBytes, err := base64.StdEncoding.DecodeString(imageDataB64)
	if err != nil {
		return "", fmt.Errorf("解码 base64 失败: %v", err)
	}

	log.Printf("✅ [handleImageFromPlugin] 图片解码成功，大小: %d bytes", len(imageBytes))

	// 2. 确定 MIME 类型
	mimeType := "image/png"
	if mt, ok := responseData["mimeType"].(string); ok && mt != "" {
		mimeType = mt
	}

	// 3. 保存到本地缓存
	safeNodeID := strings.NewReplacer(":", "_", ";", "_", "/", "_", "\\", "_", "?", "_", "*", "_", "\"", "_", "<", "_", ">", "_", "|", "_").Replace(nodeID)
	previewsDir := filepath.Join("temp", fileKey, "previews")
	if err := os.MkdirAll(previewsDir, 0755); err != nil {
		return "", fmt.Errorf("创建预览目录失败: %v", err)
	}

	// 确定文件扩展名
	ext := strings.ToLower(format)
	if ext == "jpg" {
		ext = "jpeg"
	}

	// 根据ignoreTexts决定文件名后缀：x表示带文字，p表示不带文字
	scaleStr := fmt.Sprintf("%.1fx", scale)
	suffix := "x"
	if ignoreTexts {
		suffix = "p"
	}

	// 如果有忽略节点，添加到后缀中
	if len(ignoreNodes) > 0 {
		// 对忽略节点列表进行排序以确保一致性
		sortedIgnoreNodes := make([]string, len(ignoreNodes))
		copy(sortedIgnoreNodes, ignoreNodes)
		sort.Strings(sortedIgnoreNodes)

		// 生成忽略节点的哈希值作为文件名的一部分
		ignoreNodesStr := strings.Join(sortedIgnoreNodes, ",")
		hash := md5.Sum([]byte(ignoreNodesStr))
		ignoreNodesHash := fmt.Sprintf("%x", hash)[:8]
		suffix = suffix + "i" + ignoreNodesHash
	}

	// 使用新的文件名格式：nodeID-scale[x|p][i-hash].format
	localFilePath := filepath.Join(previewsDir, fmt.Sprintf("%s-%s%s.%s", safeNodeID, scaleStr, suffix, ext))

	// 保存文件
	if err := os.WriteFile(localFilePath, imageBytes, 0644); err != nil {
		return "", fmt.Errorf("保存本地文件失败: %v", err)
	}

	log.Printf("✅ [handleImageFromPlugin] 图片已保存到本地: %s", localFilePath)

	// 4. 异步上传到华为云 OBS
	if globalOBSService != nil && globalOBSService.IsEnabled() {
		go func() {
			log.Printf("☁️ [handleImageFromPlugin] 开始上传到华为云 OBS...")

			obsURL, obsKey, fileSize, err := globalOBSService.UploadImageFromBytes(
				imageBytes,
				fileKey,
				nodeID,
				format,
				scale,
				mimeType,
			)

			if err != nil {
				log.Printf("❌ [handleImageFromPlugin] 上传到 OBS 失败: %v", err)
				return
			}

			log.Printf("✅ [handleImageFromPlugin] 上传到 OBS 成功: %s (%d bytes)", obsKey, fileSize)

			// 5. 保存到数据库缓存
			now := uint32(time.Now().Unix())
			expiresAt := now + (7 * 24 * 3600) // 7天过期

			// 检查是否已存在缓存记录
			existingImage, err := models.GetNodeImage(fileKey, nodeID, format, scale, ignoreTexts)
			if err == nil && existingImage != nil {
				// 更新现有记录
				existingImage.OBSKey = obsKey
				existingImage.OBSExpiresAt = expiresAt
				existingImage.FileSize = uint64(fileSize)
				existingImage.Status = "obs_synced"
				existingImage.UpdatedAt = now

				if err := models.UpdateNodeImage(existingImage); err != nil {
					log.Printf("⚠️ [handleImageFromPlugin] 更新数据库缓存失败: %v", err)
				} else {
					log.Printf("✅ [handleImageFromPlugin] 已更新数据库缓存 (ID=%d)", existingImage.ID)
				}
			} else {
				// 创建新记录
				newImage := &models.FigmaNodeImage{
					FileKey:      fileKey,
					NodeID:       nodeID,
					Format:       format,
					Scale:        scale,
					IgnoreTexts:  ignoreTexts,
					FigmaCDNURL:  obsURL, // 使用 OBS URL 作为备用
					OBSKey:       obsKey,
					OBSExpiresAt: expiresAt,
					FileSize:     uint64(fileSize),
					Status:       "obs_synced",
					CreatedAt:    now,
					UpdatedAt:    now,
				}

				if err := models.CreateNodeImage(newImage); err != nil {
					log.Printf("⚠️ [handleImageFromPlugin] 创建数据库缓存失败: %v", err)
				} else {
					log.Printf("✅ [handleImageFromPlugin] 已创建数据库缓存 (ID=%d)", newImage.ID)
				}
			}
		}()
	} else {
		log.Printf("⚠️ [handleImageFromPlugin] OBS 服务未启用，跳过云端上传")
	}

	return localFilePath, nil
}

// checkAndUploadToOBS 检查本地图片是否已上传到OBS，如果没有则上传
func checkAndUploadToOBS(fileKey, nodeID, format string, scale float64, localFilePath string) {
	// 1. 检查数据库中是否已有 OBS Key
	image, err := models.GetNodeImage(fileKey, nodeID, format, scale, false) // 默认不忽略文本
	if err == nil && image != nil && image.OBSKey != "" {
		// 已有 OBS Key，无需重复上传
		fmt.Printf("📦 OBS Key 已存在，跳过上传: fileKey=%s, nodeID=%s, obsKey=%s\n", fileKey, nodeID, image.OBSKey)
		return
	}

	// 2. 检查本地文件是否存在
	_, err = os.Stat(localFilePath)
	if err != nil {
		fmt.Printf("❌ 本地文件不存在，无法上传: %s, error=%v\n", localFilePath, err)
		return
	}

	// 3. 读取本地文件
	fileData, err := os.ReadFile(localFilePath)
	if err != nil {
		fmt.Printf("❌ 读取本地文件失败: %s, error=%v\n", localFilePath, err)
		return
	}

	// 4. 尝试上传到 OBS（使用全局 OBS 服务实例）
	if globalOBSService == nil || !globalOBSService.IsEnabled() {
		fmt.Printf("⚠️ OBS 服务未启用，跳过上传\n")
		return
	}

	// 确定 Content-Type
	contentType := "image/png"
	if format == "jpg" || format == "jpeg" {
		contentType = "image/jpeg"
	} else if format == "svg" {
		contentType = "image/svg+xml"
	}

	_, obsKey, fileSize, err := globalOBSService.UploadImageFromBytes(fileData, fileKey, nodeID, format, scale, contentType)
	if err != nil {
		fmt.Printf("❌ 上传到 OBS 失败: error=%v\n", err)
		return
	}

	fmt.Printf("✅ 成功上传到 OBS: %s → Key=%s\n", localFilePath, obsKey)

	// 5. 更新数据库记录（只保存 OBS Key，不保存 URL）
	if image != nil {
		// 更新现有记录
		image.OBSKey = obsKey
		image.FileSize = uint64(fileSize)
		image.UpdatedAt = uint32(time.Now().Unix())
		err = models.UpdateNodeImage(image)
		if err != nil {
			fmt.Printf("❌ 更新数据库记录失败: %v\n", err)
		} else {
			fmt.Printf("💾 已更新数据库: nodeID=%s, OBSKey=%s\n", nodeID, obsKey)
		}
	} else {
		// 创建新记录
		newImage := &models.FigmaNodeImage{
			FileKey:     fileKey,
			NodeID:      nodeID,
			Format:      format,
			Scale:       scale,
			FigmaCDNURL: "", // 从本地缓存上传的没有 Figma CDN URL
			OBSKey:      obsKey,
			FileSize:    uint64(fileSize),
			Width:       0, // 暂时不解析图片尺寸
			Height:      0,
			CreatedAt:   uint32(time.Now().Unix()),
			UpdatedAt:   uint32(time.Now().Unix()),
		}
		err = models.CreateNodeImage(newImage)
		if err != nil {
			// 创建失败，可能是并发创建导致的唯一键冲突
			// 尝试重新获取记录并更新
			fmt.Printf("⚠️ 创建数据库记录失败（可能已存在）: %v\n", err)
			existingImage, getErr := models.GetNodeImage(fileKey, nodeID, format, scale, false) // 默认不忽略文本
			if getErr == nil && existingImage != nil {
				// 成功获取到记录，更新它
				existingImage.OBSKey = obsKey
				existingImage.FileSize = uint64(fileSize)
				existingImage.UpdatedAt = uint32(time.Now().Unix())
				updateErr := models.UpdateNodeImage(existingImage)
				if updateErr != nil {
					fmt.Printf("❌ 重试更新数据库记录失败: %v\n", updateErr)
				} else {
					fmt.Printf("💾 重试更新数据库成功: nodeID=%s, OBSKey=%s\n", nodeID, obsKey)
				}
			} else {
				fmt.Printf("❌ 无法获取现有记录: %v\n", getErr)
			}
		} else {
			fmt.Printf("💾 已创建数据库记录: nodeID=%s, OBSKey=%s\n", nodeID, obsKey)
		}
	}
}

// GetFigmaImages 批量获取Figma过滤预览图片（返回节点ID到图片路径的映射）
func GetFigmaImages(c *gin.Context) {
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID无效",
		})
		return
	}

	nodeID := c.Param("node_id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "节点ID不能为空",
		})
		return
	}

	// 获取查询参数
	format := c.DefaultQuery("format", "png")  // 默认png格式
	scaleStr := c.DefaultQuery("scale", "1.0") // 默认1.0标准清晰度

	// 解析缩放比例
	scale, err := strconv.ParseFloat(scaleStr, 64)
	if err != nil || scale <= 0 || scale > 4.0 {
		scale = 1.0 // 如果解析失败或超出范围，使用默认值
		fmt.Printf("Controller: 缩放比例参数无效，使用默认值: 1.0\n")
	}

	// 验证格式
	validFormats := map[string]bool{"png": true, "jpg": true, "svg": true}
	if !validFormats[format] {
		format = "png" // 如果格式无效，使用默认值
		fmt.Printf("Controller: 图片格式参数无效，使用默认值: png\n")
	}

	fmt.Printf("Controller: 批量图片参数 - 格式: %s, 缩放比例: %.1f\n", format, scale)

	// 获取项目信息
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在",
		})
		return
	}

	// 获取用户信息，确保用户存在
	_, err = models.EnsureUserExists(project.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取用户信息失败: " + err.Error(),
		})
		return
	}

	// 批量获取过滤预览图片
	// TODO: 批量过滤预览功能暂时禁用，等待 DownloadFilteredPreviewFigmaImages 函数实现
	fmt.Printf("Controller: 批量过滤预览功能暂时不可用 - projectID=%d, nodeID=%s, format=%s, scale=%.1f\n",
		projectID, nodeID, format, scale)

	// imagePathMap, err := services.DownloadFilteredPreviewFigmaImages(user.FigmaToken, project.FileKey, nodeID, format, scale, uint(projectID))
	var imagePathMap map[string]string
	err = fmt.Errorf("批量过滤预览功能暂时不可用")
	if err != nil {
		errMsg := fmt.Sprintf("批量获取图片失败: %v", err)
		fmt.Println("Controller: " + errMsg)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": errMsg,
		})
		return
	}

	fmt.Printf("Controller: 成功批量获取Figma图片，共 %d 个图片\n", len(imagePathMap))

	// 返回节点ID到图片路径的映射
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"images":  imagePathMap,
		"count":   len(imagePathMap),
	})
}

// GetFigmaNodeInfo 获取项目节点列表（从缓存或数据库获取，用于渲染）
func GetFigmaRenderNodeInfo(c *gin.Context) {
	// 获取当前用户ID
	userID := middleware.GetUserID(c)

	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "项目ID无效",
		})
		return
	}

	// 获取项目信息
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "项目不存在",
		})
		return
	}

	// 检查项目是否属于当前用户
	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": "无权访问该项目",
		})
		return
	}

	// 1. 尝试从本地缓存获取节点树
	nodes, err := getNodesWithTypeFromLocalCache(project.ID, project.FileKey, project.RootNodeID)
	if err == nil && len(nodes) > 0 {
		fmt.Printf("✅ 从本地缓存获取到 %d 个节点\n", len(nodes))

		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "获取渲染节点列表成功（从本地缓存）",
			"data": gin.H{
				"file_key": project.FileKey,
				"nodes":    nodes,
				"count":    len(nodes),
			},
		})
		return
	}
	fmt.Printf("⚠️ 本地缓存未找到节点树: %v\n", err)

	// 2. 尝试从数据库 figma_file_caches 获取
	fileCache, err := models.GetFileCache(project.FileKey, project.RootNodeID)
	if err == nil && fileCache != nil && fileCache.FileData != "" {
		// 从 file_data 提取节点信息（包含类型）
		fmt.Printf("📦 从数据库缓存的 file_data 提取节点信息（包含类型，应用 ignore 过滤）...\n")
		extractedNodes, err := extractNodesWithTypeFromFileData(project.ID, fileCache.FileData, project.RootNodeID)
		if err == nil && len(extractedNodes) > 0 {
			fmt.Printf("✅ 成功从 file_data 提取 %d 个节点信息（已应用 ignore 和 visible 过滤）\n", len(extractedNodes))

			c.JSON(http.StatusOK, gin.H{
				"code":    0,
				"message": "获取渲染节点列表成功（从数据库缓存）",
				"data": gin.H{
					"file_key": project.FileKey,
					"nodes":    extractedNodes,
					"count":    len(extractedNodes),
				},
			})
			return
		}
		fmt.Printf("⚠️ 从 file_data 提取节点信息失败: %v\n", err)
	} else {
		if err != nil {
			fmt.Printf("⚠️ 数据库中未找到文件缓存: %v\n", err)
		} else if fileCache != nil && fileCache.FileData == "" {
			fmt.Printf("⚠️ 数据库缓存存在但 file_data 为空\n")
		}
	}

	// 3. 如果缓存都没有，回退到从 figma_nodes 表获取（兼容旧数据）
	fmt.Printf("⚠️ 所有缓存途径都失败，从 figma_nodes 表获取\n")
	var figmaNodes []models.FigmaNode
	if err := models.DB.Where("project_id = ?", projectID).Find(&figmaNodes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取节点列表失败: " + err.Error(),
		})
		return
	}

	// 构建节点信息列表，排除标记为 ignore 的节点
	nodes = make([]models.NodeInfo, 0)
	nodeIDSet := make(map[string]bool) // 用于去重

	for _, node := range figmaNodes {
		// 解析修改信息，检查是否被标记为 ignore
		if node.Modifys != "" {
			var modifyData map[string]interface{}
			if err := json.Unmarshal([]byte(node.Modifys), &modifyData); err == nil {
				// 检查是否有 ignore 标记
				if ignore, ok := modifyData["ignore"].(bool); ok && ignore {
					// 跳过标记为 ignore 的节点
					continue
				}
			}
		}

		nodes = append(nodes, models.NodeInfo{
			ID:   node.NodeID,
			Type: "", // figma_nodes 表中没有类型信息
			Name: "", // figma_nodes 表中没有名称信息
		})
		nodeIDSet[node.NodeID] = true
	}

	// 获取依赖节点列表，添加到节点列表中（排除重复）
	refNodeIDs, err := models.GetRefNodes(uint(projectID))
	if err == nil && len(refNodeIDs) > 0 {
		for _, refNodeID := range refNodeIDs {
			if !nodeIDSet[refNodeID] {
				nodes = append(nodes, models.NodeInfo{
					ID:   refNodeID,
					Type: "",
					Name: "",
				})
				nodeIDSet[refNodeID] = true
			}
		}
	}

	if len(nodes) == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "未找到节点列表，请先刷新节点树",
		})
		return
	}

	// 返回简化的格式（返回节点信息列表）
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "获取渲染节点列表成功（从 figma_nodes 表）",
		"data": gin.H{
			"file_key": project.FileKey,
			"nodes":    nodes,
			"count":    len(nodes),
		},
	})
}

// filterValidRenderNodeIDs 过滤掉无效节点ID（visible=false或ignore=true）
func filterValidRenderNodeIDs(projectID uint, fileKey, rootNodeID string, nodeIDs []string) ([]string, error) {
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
				buildNodeDataMapForFilter(document, nodeDataMap)
			}
		}
	}

	// 过滤节点ID
	validNodeIDs := make([]string, 0, len(nodeIDs))
	excludedCount := 0
	for _, nodeID := range nodeIDs {
		nodeData := nodeDataMap[nodeID]
		modifys := nodeModifys[nodeID]

		// 检查visible属性
		shouldExclude := false
		if nodeData != nil {
			if visible, hasVisible := nodeData["visible"]; hasVisible {
				if visibleBool, ok := visible.(bool); ok && !visibleBool {
					log.Printf("🚫 [节点过滤] 排除节点 %s，因为其visible为false", nodeID)
					shouldExclude = true
				}
			}
		}

		// 检查ignore标记
		if !shouldExclude && modifys != nil {
			if ignore, ok := modifys["ignore"].(bool); ok && ignore {
				log.Printf("🚫 [节点过滤] 排除节点 %s，因为其被标记为忽略", nodeID)
				shouldExclude = true
			}
		}

		if !shouldExclude {
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

// buildNodeDataMapForFilter 递归构建节点数据映射
func buildNodeDataMapForFilter(node map[string]interface{}, nodeMap map[string]map[string]interface{}) {
	if nodeID, ok := node["id"].(string); ok {
		nodeMap[nodeID] = node
	}

	if children, ok := node["children"].([]interface{}); ok {
		for _, child := range children {
			if childNode, ok := child.(map[string]interface{}); ok {
				buildNodeDataMapForFilter(childNode, nodeMap)
			}
		}
	}
}

// getNodeIDsFromLocalCache 从本地缓存文件中获取所有节点ID
func getNodeIDsFromLocalCache(projectID uint, fileKey, rootNodeID string) ([]string, error) {
	// 创建安全的文件名
	safeNodeID := strings.NewReplacer(":", "_", ";", "_", "/", "_", "\\", "_", "?", "_", "*", "_", "\"", "_", "<", "_", ">", "_", "|", "_").Replace(rootNodeID)
	cacheDir := filepath.Join("temp", fileKey, "documents")
	cacheFile := filepath.Join(cacheDir, safeNodeID+".json")

	// 检查文件是否存在
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("本地缓存文件不存在: %s", cacheFile)
	}

	// 读取缓存文件
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil, fmt.Errorf("读取本地缓存文件失败: %v", err)
	}

	// 提取节点ID
	return extractNodeIDsFromFileData(projectID, string(data), rootNodeID)
}

// getNodesWithTypeFromLocalCache 从本地缓存获取节点信息（包含类型）
func getNodesWithTypeFromLocalCache(projectID uint, fileKey, rootNodeID string) ([]models.NodeInfo, error) {
	// 创建安全的文件名
	safeNodeID := strings.NewReplacer(":", "_", ";", "_", "/", "_", "\\", "_", "?", "_", "*", "_", "\"", "_", "<", "_", ">", "_", "|", "_").Replace(rootNodeID)
	cacheDir := filepath.Join("temp", fileKey, "documents")
	cacheFile := filepath.Join(cacheDir, safeNodeID+".json")

	// 检查文件是否存在
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("本地缓存文件不存在: %s", cacheFile)
	}

	// 读取缓存文件
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil, fmt.Errorf("读取本地缓存文件失败: %v", err)
	}

	// 提取节点信息（包含类型）
	return extractNodesWithTypeFromFileData(projectID, string(data), rootNodeID)
}

// extractNodeIDsFromFileData 从 Figma 文件数据（JSON）中提取所有节点ID
func extractNodeIDsFromFileData(projectID uint, fileDataJSON string, rootNodeID string) ([]string, error) {
	// 直接使用 models 包中的函数
	return models.ExtractValidNodeIDs(fileDataJSON, projectID)
}

// extractNodesWithTypeFromFileData 从 Figma 文件数据（JSON）中提取节点信息（包含类型）
func extractNodesWithTypeFromFileData(projectID uint, fileDataJSON string, rootNodeID string) ([]models.NodeInfo, error) {
	// 使用 models 包中的新函数
	return models.ExtractValidNodesWithType(fileDataJSON, projectID)
}

// extractAllNodeIDsRecursive 递归提取节点ID（用于从文件数据提取）
// 支持过滤不可见节点(visible=false)和被标记为忽略的节点(ignore=true)及其子树
func extractAllNodeIDsRecursive(node map[string]interface{}, nodeIDs *[]string, nodeIDSet map[string]bool, nodeModifys map[string]map[string]interface{}) {
	// 检查节点是否可见，如果不可见则跳过该节点及其整个子树
	if visible, hasVisible := node["visible"]; hasVisible {
		if visibleBool, ok := visible.(bool); ok && !visibleBool {
			if nodeID, ok := node["id"].(string); ok {
				fmt.Printf("🚫 跳过不可见节点及其子树: %s\n", nodeID)
			}
			return // 跳过整个子树
		}
	}

	// 提取当前节点的ID
	if nodeID, ok := node["id"].(string); ok && nodeID != "" && nodeID != "0:0" {
		// 检查是否被标记为ignore（如果有modify信息）
		if modifys, hasModify := nodeModifys[nodeID]; hasModify {
			if ignore, ok := modifys["ignore"].(bool); ok && ignore {
				fmt.Printf("🚫 跳过被标记为忽略的节点及其子树: %s\n", nodeID)
				return // 跳过整个子树
			}
		}

		if !nodeIDSet[nodeID] {
			*nodeIDs = append(*nodeIDs, nodeID)
			nodeIDSet[nodeID] = true
		}
	}

	// 递归处理子节点
	if children, ok := node["children"].([]interface{}); ok {
		for _, child := range children {
			if childNode, ok := child.(map[string]interface{}); ok {
				extractAllNodeIDsRecursive(childNode, nodeIDs, nodeIDSet, nodeModifys)
			}
		}
	}
}

// UpdateProject 更新项目
func UpdateProject(c *gin.Context) {
	userID := middleware.GetUserID(c)
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID无效",
		})
		return
	}

	// 检查项目是否属于当前用户
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在",
		})
		return
	}

	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "无权更新该项目",
		})
		return
	}

	// 获取新的项目名称
	name := c.PostForm("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目名称不能为空",
		})
		return
	}

	// 获取分组名称（可选）
	groupName := c.PostForm("group_name")

	// 获取Figma URL（可选）
	figmaURL := c.PostForm("figma_url")

	// 更新项目信息
	project.Name = name
	project.GroupName = groupName
	project.FigmaURL = figmaURL
	if err := models.DB.Save(&project).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "更新项目失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "项目更新成功",
		"project": project,
	})
}

// UpdateProjectSettings 更新项目设置
func UpdateProjectSettings(c *gin.Context) {
	userID := middleware.GetUserID(c)
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID无效",
		})
		return
	}

	// 检查项目是否属于当前用户
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在",
		})
		return
	}

	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "无权更新该项目",
		})
		return
	}

	// 解析请求体为JSON
	var settings map[string]interface{}
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的JSON数据: " + err.Error(),
		})
		return
	}

	// 将设置转换为JSON字符串
	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "处理设置数据失败: " + err.Error(),
		})
		return
	}

	// 更新项目设置
	project.Settings = string(settingsJSON)
	if err := models.DB.Save(&project).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "更新项目设置失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "项目设置更新成功",
		"settings": settings,
	})
}

// DeleteProject 删除项目
func DeleteProject(c *gin.Context) {
	userID := middleware.GetUserID(c)
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID无效",
		})
		return
	}

	// 检查项目是否属于当前用户
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在",
		})
		return
	}

	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "无权删除该项目",
		})
		return
	}

	// 删除项目
	err = models.DeleteProject(uint(projectID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "删除项目失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "项目删除成功",
	})
}

// ClearProjectImageCache 清除项目的图片缓存
func ClearProjectImageCache(c *gin.Context) {
	userID := middleware.GetUserID(c)
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID无效",
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

	// 检查项目是否属于当前用户
	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "无权清除该项目的缓存",
		})
		return
	}

	// 检查是否需要按节点ID清理（通过查询参数smart=true启用）
	smartClear := c.Query("smart") == "true"

	var clearedCount int

	if smartClear {
		// 智能清理：根据项目的节点数据进行精确清理
		fmt.Printf("启用智能缓存清理模式\n")

		// 获取用户信息
		user, err := models.EnsureUserExists(userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "获取用户信息失败: " + err.Error(),
			})
			return
		}

		// 获取项目的优化JSON数据以提取所有节点ID
		optimizedData, err := getOptimizedProjectDataForCache(user, project)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "获取项目数据失败: " + err.Error(),
			})
			return
		}

		// 从JSON数据中提取所有节点ID
		nodeIDs, err := extractAllNodeIDsFromData(optimizedData)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "提取节点ID失败: " + err.Error(),
			})
			return
		}

		// 按节点ID清除缓存
		err = services.ClearProjectImageCacheByNodeIDs(project.FileKey, nodeIDs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "智能清除缓存失败: " + err.Error(),
			})
			return
		}

		// 清除项目导出目录
		exportCleared, err := services.ClearProjectExportCache(uint(projectID))
		if err != nil {
			fmt.Printf("清除项目导出缓存失败: %v\n", err)
		}

		clearedCount = len(nodeIDs)

		c.JSON(http.StatusOK, gin.H{
			"message":      fmt.Sprintf("已成功智能清除项目图片缓存，涉及 %d 个节点，清除 %d 个导出文件", clearedCount, exportCleared),
			"smart_clear":  true,
			"node_count":   clearedCount,
			"export_count": exportCleared,
		})
	} else {
		// 传统清理：清除所有缓存文件
		err = services.ClearProjectImageCache(project.FileKey)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "清除缓存失败: " + err.Error(),
			})
			return
		}

		// 清除项目导出目录
		exportCleared, err := services.ClearProjectExportCache(uint(projectID))
		if err != nil {
			fmt.Printf("清除项目导出缓存失败: %v\n", err)
		}

		c.JSON(http.StatusOK, gin.H{
			"message":      fmt.Sprintf("已成功清除项目图片缓存，清除 %d 个导出文件", exportCleared),
			"smart_clear":  false,
			"export_count": exportCleared,
		})
	}
}

// getOptimizedProjectDataForCache 获取项目的优化数据用于缓存清理
func getOptimizedProjectDataForCache(user *models.User, project *models.FigmaProject) (gin.H, error) {
	// 获取节点树数据（支持 WebSocket 兜底，会自动等待响应）
	nodes, err := services.GetFigmaNodesWithUser(user.FigmaToken, project.FileKey, project.RootNodeID, user.ID)
	if err != nil {
		return nil, fmt.Errorf("获取节点树失败: %v", err)
	}

	// 获取所有节点的修改信息
	var figmaNodes []models.FigmaNode
	dbResult := models.DB.Where("project_id = ?", project.ID).Find(&figmaNodes)
	if dbResult.Error != nil {
		return nil, fmt.Errorf("获取节点修改信息失败: %v", dbResult.Error)
	}

	// 构建节点ID到修改信息的映射
	nodeModifys := make(map[string]map[string]interface{})
	for _, node := range figmaNodes {
		if node.Modifys != "" {
			var modifyData map[string]interface{}
			if err := json.Unmarshal([]byte(node.Modifys), &modifyData); err == nil {
				nodeModifys[node.NodeID] = modifyData
			}
		}
	}

	// 构建优化后的节点数据
	optimizedNodes := make([]gin.H, 0)

	for _, node := range nodes {
		nodeID := node["id"].(string)
		nodeName, _ := node["name"].(string)
		nodeType, _ := node["type"].(string)

		// 创建优化节点，包含所有Figma原始数据
		optimizedNode := gin.H{}

		// 复制所有Figma原始属性
		for key, value := range node {
			optimizedNode[key] = value
		}

		// 添加几何信息的简化版本（保持向后兼容）
		if absoluteBoundingBox, ok := node["absoluteBoundingBox"].(map[string]interface{}); ok {
			optimizedNode["bounds"] = gin.H{
				"x":      absoluteBoundingBox["x"],
				"y":      absoluteBoundingBox["y"],
				"width":  absoluteBoundingBox["width"],
				"height": absoluteBoundingBox["height"],
			}
		}

		// 确保可见性信息存在
		if _, ok := node["visible"]; !ok {
			optimizedNode["visible"] = true // 默认可见
		}

		// 添加修改信息（如果存在）
		if modifys, exists := nodeModifys[nodeID]; exists {
			optimizedNode["modifys"] = modifys

			// 如果有重命名，使用重命名后的名称
			if rename, ok := modifys["rename"].(string); ok && rename != "" {
				optimizedNode["displayName"] = rename
			} else {
				optimizedNode["displayName"] = nodeName
			}

			// 添加资源模式信息
			if resMode, ok := modifys["res_mode"].(string); ok {
				optimizedNode["resourceMode"] = resMode
			}

			// 添加忽略标记
			if ignore, ok := modifys["ignore"].(bool); ok {
				optimizedNode["ignored"] = ignore
			}
		} else {
			optimizedNode["displayName"] = nodeName
			optimizedNode["resourceMode"] = "default"
			optimizedNode["ignored"] = false
		}

		// 添加额外的便利属性（基于原始数据）
		if nodeType == "TEXT" {
			if characters, ok := node["characters"].(string); ok {
				optimizedNode["text"] = characters // 便利属性
			}
		}

		optimizedNodes = append(optimizedNodes, optimizedNode)
	}

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

	// 为每个节点添加子节点信息
	for i := range optimizedNodes {
		nodeID := optimizedNodes[i]["id"].(string)
		if children, hasChildren := childrenMap[nodeID]; hasChildren {
			optimizedNodes[i]["children"] = children
		} else {
			optimizedNodes[i]["children"] = []gin.H{}
		}
	}

	// 只返回根节点（从根节点开始的树形结构）
	treeNodes := make([]gin.H, 0)
	for _, node := range optimizedNodes {
		parentID := node["parent_id"]
		if parentID == nil || parentID == project.RootNodeID {
			treeNodes = append(treeNodes, node)
		}
	}

	// 返回优化后的数据结构（只包含nodes，保持树形结构）
	return gin.H{
		"nodes": treeNodes, // 只返回根节点，每个节点包含完整的子树
	}, nil
}

// extractAllNodeIDsFromData 从优化的JSON数据中提取所有节点ID
func extractAllNodeIDsFromData(optimizedData gin.H) ([]string, error) {
	var nodeIDs []string

	// 从nodes字段中提取节点ID
	if nodes, ok := optimizedData["nodes"].([]gin.H); ok {
		nodeIDs = extractNodeIDsRecursiveFromData(nodes, nodeIDs)
	} else {
		return nil, fmt.Errorf("无法解析nodes数据")
	}

	fmt.Printf("从项目数据中提取到 %d 个节点ID\n", len(nodeIDs))
	return nodeIDs, nil
}

// extractNodeIDsRecursiveFromData 递归提取节点ID（包括子节点）
func extractNodeIDsRecursiveFromData(nodes []gin.H, nodeIDs []string) []string {
	for _, node := range nodes {
		// 提取当前节点的ID
		if id, ok := node["id"].(string); ok {
			nodeIDs = append(nodeIDs, id)
		}

		// 递归处理子节点
		if children, ok := node["children"].([]gin.H); ok && len(children) > 0 {
			nodeIDs = extractNodeIDsRecursiveFromData(children, nodeIDs)
		}
	}
	return nodeIDs
}

// AddRefNode 添加依赖节点
func AddRefNode(c *gin.Context) {
	userID := middleware.GetUserID(c)
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的项目ID",
		})
		return
	}

	// 获取节点ID
	nodeID := c.PostForm("node_id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "节点ID不能为空",
		})
		return
	}

	nodeID = strings.ReplaceAll(nodeID, "-", ":")
	// 获取项目信息并验证权限
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在",
		})
		return
	}

	// 验证项目所有者
	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "您没有权限访问此项目",
		})
		return
	}

	// 添加依赖节点
	err = models.AddRefNode(uint(projectID), nodeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "依赖节点添加成功",
		"data": gin.H{
			"project_id": projectID,
			"node_id":    nodeID,
		},
	})
}

// RemoveRefNode 移除依赖节点
func RemoveRefNode(c *gin.Context) {
	userID := middleware.GetUserID(c)
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的项目ID",
		})
		return
	}

	// 获取节点ID
	nodeID := c.Param("node_id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "节点ID不能为空",
		})
		return
	}

	// 获取项目信息并验证权限
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在",
		})
		return
	}

	// 验证项目所有者
	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "您没有权限访问此项目",
		})
		return
	}

	// 移除依赖节点
	err = models.RemoveRefNode(uint(projectID), nodeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "依赖节点移除成功",
		"data": gin.H{
			"project_id": projectID,
			"node_id":    nodeID,
		},
	})
}

// GetRefNodes 获取依赖节点列表
func GetRefNodes(c *gin.Context) {
	userID := middleware.GetUserID(c)
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的项目ID",
		})
		return
	}

	// 获取项目信息并验证权限
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在",
		})
		return
	}

	// 验证项目所有者
	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "您没有权限访问此项目",
		})
		return
	}

	// 获取依赖节点列表
	refNodes, err := models.GetRefNodes(uint(projectID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取依赖节点失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"project_id": projectID,
			"ref_nodes":  refNodes,
		},
		"message": "获取依赖节点列表成功",
	})
}

// UpdateInterfaceDescription 更新项目界面描述
func UpdateInterfaceDescription(c *gin.Context) {
	userID := middleware.GetUserID(c)
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID无效",
		})
		return
	}

	// 获取请求体中的界面描述
	var requestBody struct {
		InterfaceDescription string `json:"interface_description"`
	}
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求数据格式错误: " + err.Error(),
		})
		return
	}

	// 检查项目是否属于当前用户
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在",
		})
		return
	}

	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "无权更新该项目",
		})
		return
	}

	// 更新界面描述
	project.InterfaceDescription = requestBody.InterfaceDescription
	if err := models.DB.Save(&project).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "更新界面描述失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "界面描述更新成功",
		"data": gin.H{
			"project_id":            projectID,
			"interface_description": requestBody.InterfaceDescription,
		},
	})
}

// GetInterfaceDescription 获取项目界面描述
func GetInterfaceDescription(c *gin.Context) {
	userID := middleware.GetUserID(c)
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID无效",
		})
		return
	}

	// 检查项目是否属于当前用户
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在",
		})
		return
	}

	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "无权访问该项目",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"project_id":            projectID,
			"interface_description": project.InterfaceDescription,
		},
	})
}

// GetRefNodeDetails 获取指定依赖节点的详细信息
func GetRefNodeDetails(c *gin.Context) {
	userID := middleware.GetUserID(c)
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的项目ID",
		})
		return
	}

	// 获取节点ID列表（通过查询参数传递，逗号分隔）
	nodeIDsParam := c.Query("node_ids")
	if nodeIDsParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "节点ID列表不能为空",
		})
		return
	}

	nodeIDs := strings.Split(nodeIDsParam, ",")

	// 获取项目信息并验证权限
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在",
		})
		return
	}

	// 验证项目所有者
	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "您没有权限访问此项目",
		})
		return
	}

	// 获取用户信息
	var user models.User
	result := models.DB.First(&user, userID)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "用户不存在",
		})
		return
	}

	// 如果没有Figma Token，返回错误
	if user.FigmaToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请先在个人资料页面配置 Figma Token",
		})
		return
	}

	// 为每个节点ID获取具体的节点数据
	nodeDetails := make(map[string]interface{})

	for _, nodeID := range nodeIDs {
		nodeID = strings.TrimSpace(nodeID)
		if nodeID == "" {
			continue
		}

		// 使用指定的节点ID获取节点树数据（只获取该节点及其子节点，支持 WebSocket 兜底，会自动等待响应）
		nodeData, err := services.GetFigmaNodesWithUser(user.FigmaToken, project.FileKey, nodeID, user.ID)
		if err != nil {
			// 如果获取失败，记录错误但继续处理其他节点
			fmt.Printf("获取节点 %s 数据失败: %v\n", nodeID, err)

			nodeDetails[nodeID] = gin.H{
				"id":           nodeID,
				"name":         fmt.Sprintf("节点 %s", nodeID),
				"type":         "REFERENCE",
				"error":        true,
				"errorMessage": err.Error(),
			}
			continue
		}

		// 查找根节点（应该是请求的节点ID）
		var rootNode map[string]interface{}
		for _, node := range nodeData {
			if nodeIDStr, ok := node["id"].(string); ok && nodeIDStr == nodeID {
				rootNode = node
				break
			}
		}

		if rootNode != nil {
			// 返回根节点信息和完整的树结构数据
			nodeDetails[nodeID] = gin.H{
				"rootNode": rootNode,
				"treeData": nodeData, // 包含完整的节点树数据
			}
		} else {
			// 如果没有找到对应的节点，返回占位符
			nodeDetails[nodeID] = gin.H{
				"id":       nodeID,
				"name":     fmt.Sprintf("节点 %s", nodeID),
				"type":     "REFERENCE",
				"notFound": true,
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"project_id":   projectID,
			"file_key":     project.FileKey,
			"node_details": nodeDetails,
		},
		"message": "获取依赖节点详细信息成功",
	})
}

// ==================== 自定义代码导出功能 ====================

// GenerateCustomCodePreview 生成自定义代码预览
func GenerateCustomCodePreview(c *gin.Context) {
	userID := middleware.GetUserID(c)
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID无效",
		})
		return
	}

	// 解析请求体中的导出配置
	var config services.CustomCodeExportConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		// 使用默认配置
		config = services.CustomCodeExportConfig{
			ImageFormat: "png",
			ImageScale:  2.0,
		}
	}

	// 验证项目权限
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在",
		})
		return
	}

	// 检查项目是否属于当前用户
	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "无权限访问此项目",
		})
		return
	}

	// 生成自定义代码预览
	files, err := services.GenerateCustomCodePreview(project, config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "生成代码预览失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"files":   files,
		"message": "代码预览生成成功",
	})
}

// ShowCustomCodePreview 显示自定义代码预览页面
func ShowCustomCodePreview(c *gin.Context) {
	userID := middleware.GetUserID(c)
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"title":   "错误",
			"message": "项目ID无效",
		})
		return
	}

	// 验证项目权限
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"title":   "项目不存在",
			"message": "找不到指定的项目",
		})
		return
	}

	// 检查项目是否属于当前用户
	if project.UserID != userID {
		c.HTML(http.StatusForbidden, "error.html", gin.H{
			"title":   "无权限",
			"message": "您无权限访问此项目",
		})
		return
	}

	// 获取导出配置
	exportConfig := services.CustomCodeExportConfig{
		ImageFormat: c.DefaultQuery("imageFormat", "png"),
		ImageScale:  2.0,
	}

	// 解析缩放比例
	if scaleStr := c.Query("imageScale"); scaleStr != "" {
		if scale, err := strconv.ParseFloat(scaleStr, 64); err == nil && scale > 0 && scale <= 4.0 {
			exportConfig.ImageScale = scale
		}
	}

	// 生成自定义代码预览
	files, err := services.GenerateCustomCodePreview(project, exportConfig)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "生成失败",
			"message": "生成代码预览失败: " + err.Error(),
		})
		return
	}

	// 从session获取CSRF令牌（而不是生成新的）
	session := sessions.Default(c)
	csrfToken := session.Get("csrf_token")
	if csrfToken == nil {
		// 如果session中没有，生成一个新的
		token, err := middleware.GenerateRandomString(32)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"title":   "系统错误",
				"message": "生成CSRF令牌失败",
			})
			return
		}
		session.Set("csrf_token", token)
		session.Save()
		csrfToken = token
	}

	c.HTML(http.StatusOK, "custom-code-preview.html", gin.H{
		"title":         "代码预览",
		"project_id":    project.ID,
		"project_name":  project.Name,
		"code_files":    files,
		"export_config": exportConfig,
		"csrf_token":    csrfToken,
		"timestamp":     time.Now().Unix(),
	})
}

// ExportCustomCode 导出自定义代码
func ExportCustomCode(c *gin.Context) {
	userID := middleware.GetUserID(c)
	projectID, err := strconv.ParseUint(c.Param("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "项目ID无效",
		})
		return
	}

	// 解析请求体中的导出配置
	var config services.CustomCodeExportConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		// 使用默认配置
		config = services.CustomCodeExportConfig{
			ImageFormat: "png",
			ImageScale:  2.0,
		}
	}

	// 验证项目权限
	project, err := models.GetProjectByID(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "项目不存在",
		})
		return
	}

	// 检查项目是否属于当前用户
	if project.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "无权限访问此项目",
		})
		return
	}

	// 创建导出任务
	job, err := models.CreateExportJob(userID, uint(projectID), "", config.ImageFormat)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "创建导出任务失败: " + err.Error(),
		})
		return
	}

	// 异步执行导出任务，参考图文导出的实现
	go services.ProcessCustomCodeExportJob(job.ID, config)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"job_id":  job.ID,
		"message": "导出任务已创建，正在处理中...",
	})
}
