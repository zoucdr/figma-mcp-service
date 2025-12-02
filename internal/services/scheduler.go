package services

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/figma-deliver/internal/models"
)

// SchedulerService 调度服务
type SchedulerService struct {
	cacheService         *CacheService
	queueService         *QueueService
	obsService           *OBSService
	renderQueueInterval  int // 渲染队列处理间隔（秒）
	queueCleanupInterval int // 队列清理间隔（小时）
	stopChan             chan bool
	queueNotifyChan      chan bool // 队列创建通知 channel
	hasActiveQueues      bool      // 是否有活跃队列（优化：避免无效查询）
	isRunning            bool
}

// NewSchedulerService 创建调度服务
func NewSchedulerService(
	cacheService *CacheService,
	queueService *QueueService,
	obsService *OBSService,
	renderQueueInterval int,
	queueCleanupInterval int,
) *SchedulerService {
	// 确保间隔值为正数
	if renderQueueInterval <= 0 {
		log.Printf("⚠️ [调度器] 渲染队列间隔无效 (%d)，使用默认值 5 秒", renderQueueInterval)
		renderQueueInterval = 5
	}
	if queueCleanupInterval <= 0 {
		log.Printf("⚠️ [调度器] 队列清理间隔无效 (%d)，使用默认值 1 小时", queueCleanupInterval)
		queueCleanupInterval = 1
	}

	s := &SchedulerService{
		cacheService:         cacheService,
		queueService:         queueService,
		obsService:           obsService,
		renderQueueInterval:  renderQueueInterval,
		queueCleanupInterval: queueCleanupInterval,
		stopChan:             make(chan bool),
		queueNotifyChan:      make(chan bool, 100), // 缓冲 channel，避免阻塞
		hasActiveQueues:      true,                 // 初始假设有队列，会在第一次检查时更新
		isRunning:            false,
	}

	// 将 scheduler 注入到 queueService，以便创建队列时通知
	queueService.SetScheduler(s)

	return s
}

// Start 启动调度器
func (s *SchedulerService) Start() {
	if s.isRunning {
		log.Printf("⚠️ [调度器] 调度器已经在运行")
		return
	}

	s.isRunning = true
	log.Printf("🚀 [调度器] 启动成功")
	log.Printf("   - 渲染队列处理间隔: %d 秒", s.renderQueueInterval)
	log.Printf("   - 队列清理间隔: %d 小时", s.queueCleanupInterval)

	// 启动渲染队列处理器
	go s.runRenderQueueProcessor()

	// 启动文件缓存队列处理器
	go s.runFileCacheQueueProcessor()

	// 启动队列清理器
	go s.runQueueCleanup()
}

// Stop 停止调度器
func (s *SchedulerService) Stop() {
	if !s.isRunning {
		return
	}

	log.Printf("🛑 [调度器] 停止中...")
	s.stopChan <- true
	s.stopChan <- true
	s.stopChan <- true // 发送三次，停止三个 goroutine
	s.isRunning = false
	log.Printf("✅ [调度器] 已停止")
}

// NotifyNewQueue 通知有新队列创建（由 QueueService 调用）
func (s *SchedulerService) NotifyNewQueue() {
	if !s.isRunning {
		return
	}

	// 非阻塞发送通知
	select {
	case s.queueNotifyChan <- true:
		// 通知成功
	default:
		// channel 已满，忽略（定时器会处理）
	}
}

// ===================== 渲染队列处理器 =====================

// runRenderQueueProcessor 运行渲染队列处理器
func (s *SchedulerService) runRenderQueueProcessor() {
	ticker := time.NewTicker(time.Duration(s.renderQueueInterval) * time.Second)
	defer ticker.Stop()

	log.Printf("✅ [渲染队列处理器] 已启动（优化模式：无队列时自动休眠）")

	for {
		select {
		case <-ticker.C:
			// 定时检查（仅当标志位为 true 时才查询数据库）
			if s.hasActiveQueues {
				s.processRenderQueue()
			}

		case <-s.queueNotifyChan:
			// 收到新队列通知，立即处理
			log.Printf("📢 [渲染队列处理器] 收到新队列通知，立即处理")
			s.hasActiveQueues = true
			s.processRenderQueue()

		case <-s.stopChan:
			log.Printf("🛑 [渲染队列处理器] 已停止")
			return
		}
	}
}

// processRenderQueue 处理渲染队列
func (s *SchedulerService) processRenderQueue() {
	// 获取所有等待处理的队列
	queues, err := s.queueService.GetWaitingQueues()
	if err != nil {
		log.Printf("❌ [渲染队列处理器] 获取队列失败: %v", err)
		return
	}

	if len(queues) == 0 {
		// 没有待处理队列，更新标志位，下次定时器触发时将跳过数据库查询
		if s.hasActiveQueues {
			log.Printf("💤 [渲染队列处理器] 无待处理队列，进入休眠模式（减少数据库查询）")
			s.hasActiveQueues = false
		}
		return
	}

	// 有队列需要处理
	log.Printf("📦 [渲染队列处理器] 发现 %d 个待处理队列", len(queues))

	// 逐个处理队列
	for _, queue := range queues {
		s.processQueue(&queue)
	}
}

// processQueue 处理单个队列
func (s *SchedulerService) processQueue(queue *models.FigmaRenderQueue) {
	// ============ 关键检查：只处理 waiting 状态的队列 ============
	if queue.Status != "waiting" {
		log.Printf("⏭️ [队列 #%d] 跳过处理（状态: %s，只处理 waiting 状态）", queue.ID, queue.Status)
		return
	}

	log.Printf("🔄 [队列 #%d] 开始处理", queue.ID)
	log.Printf("   - 文件: %s", queue.FileKey)
	log.Printf("   - Token: %s...", queue.FigmaToken[:10])
	log.Printf("   - 节点数: %d", len(strings.Split(queue.NodeIDs, ",")))
	log.Printf("   - 当前状态: %s", queue.Status)

	// 检查 Token 冷却
	now := uint32(time.Now().Unix())
	canRequest, nextTime, err := s.cacheService.CheckImageAPICooldown(queue.FigmaToken)
	if err != nil {
		log.Printf("❌ [队列 #%d] 检查冷却失败: %v", queue.ID, err)
		return
	}

	if !canRequest {
		waitSeconds := int64(nextTime) - int64(now)
		log.Printf("⏳ [队列 #%d] Token 冷却中，需等待 %d 秒", queue.ID, waitSeconds)
		// 保持队列状态为 waiting，由 Token 冷却表统一管理等待时间
		return
	}

	// 立即将该 Token 的冷却时间更新为当前时间，防止下一轮循环时其它 waiting 队列（同 Token）被并发处理
	if err := s.cacheService.UpdateImageRequestTime(queue.FigmaToken); err != nil {
		log.Printf("⚠️ [队列 #%d] 更新 Token 冷却时间失败: %v", queue.ID, err)
	}

	// ============ 原子更新：从 waiting 更新为 processing ============
	// 使用数据库的原子操作，防止并发问题
	result := models.DB.Model(&models.FigmaRenderQueue{}).
		Where("id = ? AND status = ?", queue.ID, "waiting").
		Updates(map[string]interface{}{
			"status":     "processing",
			"started_at": now,
			"updated_at": now,
		})

	if result.Error != nil {
		log.Printf("❌ [队列 #%d] 更新状态失败: %v", queue.ID, result.Error)
		return
	}

	if result.RowsAffected == 0 {
		log.Printf("⚠️ [队列 #%d] 状态已被其他进程修改，跳过处理", queue.ID)
		return
	}

	log.Printf("🚀 [队列 #%d] 状态已更新为 processing，开始调用 Figma API", queue.ID)

	// 调用 Figma Images API
	imageURLs, err := s.callFigmaImagesAPI(queue.FileKey, queue.NodeIDs, queue.Format, queue.Scale, queue.FigmaToken)
	if err != nil {
		// 立即更新冷却时间
		_ = s.cacheService.UpdateImageRequestTime(queue.FigmaToken)
		log.Printf("❌ [队列 #%d] Figma API 调用失败: %v", queue.ID, err)

		// 检查是否是 429 速率限制错误
		if strings.Contains(err.Error(), "429") {
			// 429 错误，重新排队等待（Token 冷却由 figma_token_cooldowns 表管理）
			log.Printf("⏳ [队列 #%d] 遇到 429 速率限制，重新排队等待", queue.ID)

			// HandleRetryAfter 已经在 callFigmaImagesAPI 中更新了冷却时间
			now := uint32(time.Now().Unix())
			updateErr := models.DB.Model(&models.FigmaRenderQueue{}).
				Where("id = ?", queue.ID).
				Updates(map[string]interface{}{
					"status":        "waiting",
					"error_message": fmt.Sprintf("429 速率限制: %v", err),
					"updated_at":    now,
				}).Error

			if updateErr != nil {
				log.Printf("❌ [队列 #%d] 更新队列状态失败: %v", queue.ID, updateErr)
			} else {
				log.Printf("⏰ [队列 #%d] 已重新排队，等待 Token 冷却", queue.ID)
			}

			// 标记为有活跃队列
			s.hasActiveQueues = true
		} else {
			// ============ 非 429 错误，标记为 failed ============
			log.Printf("❌ [队列 #%d] API 调用失败（非速率限制错误），标记为 failed", queue.ID)

			now := uint32(time.Now().Unix())
			updateErr := models.DB.Model(&models.FigmaRenderQueue{}).
				Where("id = ?", queue.ID).
				Updates(map[string]interface{}{
					"status":        "failed",
					"error_message": fmt.Sprintf("Figma API 错误: %v", err),
					"completed_at":  now,
					"updated_at":    now,
				}).Error

			if updateErr != nil {
				log.Printf("❌ [队列 #%d] 更新失败状态失败: %v", queue.ID, updateErr)
			} else {
				log.Printf("✅ [队列 #%d] 已标记为 failed", queue.ID)
			}
		}
		return
	}

	// 更新 Token 请求时间
	err = s.cacheService.UpdateImageRequestTime(queue.FigmaToken)
	if err != nil {
		log.Printf("⚠️ [队列 #%d] 更新请求时间失败: %v", queue.ID, err)
	}

	log.Printf("✅ [队列 #%d] 获取到 %d 个图片 URL", queue.ID, len(imageURLs))

	// 更新进度：获取到 URL（processed=30%, failed=0）
	err = s.queueService.UpdateQueueProgress(queue.ID, 30, 0)
	if err != nil {
		log.Printf("❌ [队列 #%d] 更新进度失败: %v", queue.ID, err)
	}

	// 保存图片 URL 到数据库并上传到 OBS
	successCount := 0
	failedCount := 0
	totalCount := len(imageURLs)

	for nodeID, figmaURL := range imageURLs {
		// 验证 URL 是否有效
		if figmaURL == "" || figmaURL == "null" {
			log.Printf("⚠️ [队列 #%d] 节点 %s 的图片 URL 为空，跳过", queue.ID, nodeID)
			failedCount++
			// 更新进度
			err = s.queueService.UpdateQueueProgress(queue.ID, uint(successCount), uint(failedCount))
			if err != nil {
				log.Printf("❌ [队列 #%d] 更新进度失败: %v", queue.ID, err)
			}
			continue
		}

		// 创建或更新节点图片记录
		nodeImage, exists, err := s.cacheService.GetNodeImage(queue.FileKey, nodeID, queue.Format, queue.Scale, false) // 默认不忽略文本
		if err != nil || !exists || nodeImage == nil {
			// 创建新记录
			nodeImage, err = s.cacheService.CreateNodeImage(
				queue.FileKey,
				nodeID,
				queue.Format,
				queue.Scale,
				figmaURL,
				false, // 默认不忽略文本
			)
			if err != nil {
				log.Printf("⚠️ [队列 #%d] 创建节点图片记录失败 (%s): %v", queue.ID, nodeID, err)
				failedCount++
				// 更新进度
				err = s.queueService.UpdateQueueProgress(queue.ID, uint(successCount), uint(failedCount))
				if err != nil {
					log.Printf("❌ [队列 #%d] 更新进度失败: %v", queue.ID, err)
				}
				continue
			}
		} else {
			// 更新 Figma CDN URL
			nodeImage.FigmaCDNURL = figmaURL
			nodeImage.Status = "pending"
			err = models.UpdateNodeImage(nodeImage)
			if err != nil {
				log.Printf("⚠️ [队列 #%d] 更新节点图片记录失败 (%s): %v", queue.ID, nodeID, err)
				failedCount++
				// 更新进度
				err = s.queueService.UpdateQueueProgress(queue.ID, uint(successCount), uint(failedCount))
				if err != nil {
					log.Printf("❌ [队列 #%d] 更新进度失败: %v", queue.ID, err)
				}
				continue
			}
		}

		// 如果 OBS 启用，上传到 OBS
		if s.obsService.IsEnabled() {
			_, obsKey, fileSize, err := s.obsService.UploadImageFromURL(
				figmaURL,
				queue.FileKey,
				nodeID,
				queue.Format,
				queue.Scale,
			)

			if err != nil {
				log.Printf("⚠️ [队列 #%d] 上传 OBS 失败 (%s): %v", queue.ID, nodeID, err)
				// 即使 OBS 上传失败，也保留 Figma CDN URL
				nodeImage.Status = "figma_cdn"
				nodeImage.ErrorMessage = fmt.Sprintf("OBS上传失败: %v", err)
			} else {
				// 更新 OBS 信息（width和height暂时设为0，因为Figma API不直接返回这些信息）
				err = s.cacheService.UpdateNodeImageOBS(nodeImage.ID, obsKey, uint64(fileSize), 0, 0)
				if err != nil {
					log.Printf("⚠️ [队列 #%d] 更新 OBS 信息失败 (%s): %v", queue.ID, nodeID, err)
					nodeImage.Status = "figma_cdn"
				} else {
					nodeImage.Status = "obs_synced"
					log.Printf("✅ [队列 #%d] 节点 %s 已上传到OBS: %s", queue.ID, nodeID, obsKey)
				}
			}
		} else {
			// OBS 未启用，只保存 Figma CDN URL
			nodeImage.Status = "figma_cdn"
		}

		// 更新节点图片状态
		err = models.UpdateNodeImage(nodeImage)
		if err != nil {
			log.Printf("⚠️ [队列 #%d] 更新节点图片状态失败 (%s): %v", queue.ID, nodeID, err)
			failedCount++
		} else {
			successCount++
		}

		// 更新进度（processed和failed的计数）
		err = s.queueService.UpdateQueueProgress(queue.ID, uint(successCount), uint(failedCount))
		if err != nil {
			log.Printf("❌ [队列 #%d] 更新进度失败: %v", queue.ID, err)
		}
	}

	// 完成处理
	if successCount == totalCount {
		err = s.queueService.UpdateQueueStatus(queue.ID, "completed", 100)
		log.Printf("✅ [队列 #%d] 处理完成: %d/%d", queue.ID, successCount, totalCount)
	} else {
		progress := uint(successCount * 100 / totalCount)
		err = s.queueService.UpdateQueueStatus(queue.ID, "partial", progress)
		log.Printf("⚠️ [队列 #%d] 部分完成: %d/%d", queue.ID, successCount, totalCount)
	}

	if err != nil {
		log.Printf("❌ [队列 #%d] 更新最终状态失败: %v", queue.ID, err)
	}
}

// callFigmaImagesAPI 调用 Figma Images API
func (s *SchedulerService) callFigmaImagesAPI(fileKey, nodeIDs, format string, scale float64, token string) (map[string]string, error) {
	// 无论成功或失败，都记录此次 API 调用时间
	defer func() {
		err := models.UpdateImageRequestTime(token)
		if err != nil {
			log.Printf("⚠️ [Figma API] 更新 last_image_request_time 失败: %v", err)
		}
	}()

	// 构建 URL
	url := fmt.Sprintf("https://api.figma.com/v1/images/%s", fileKey)

	// 添加查询参数
	url += fmt.Sprintf("?ids=%s", nodeIDs)
	url += fmt.Sprintf("&format=%s", format)
	url += fmt.Sprintf("&scale=%.1f", scale)

	log.Printf("🌐 [Figma API] 请求 URL: %s", url)
	log.Printf("🌐 [Figma API] Token: %s...", token[:10])
	// 创建请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置 Header
	req.Header.Set("X-Figma-Token", token)

	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 检查状态码
	if resp.StatusCode != 200 {
		// 打印完整的响应头信息（用于调试）
		log.Printf("❌ [Figma API] 响应状态码: %d", resp.StatusCode)
		log.Printf("📋 [Figma API] 响应头信息:")
		for key, values := range resp.Header {
			for _, value := range values {
				log.Printf("    %s: %s", key, value)
			}
		}
		log.Printf("📄 [Figma API] 响应体: %s", string(body))

		// 处理 429 速率限制错误
		if resp.StatusCode == http.StatusTooManyRequests {
			// 检查并处理 Retry-After 头
			HandleRetryAfter(token, resp)

			// 获取 Retry-After 信息用于错误消息
			retryAfter := resp.Header.Get("Retry-After")
			if retryAfter != "" {
				return nil, fmt.Errorf("Figma API 速率限制 (状态码 429, Retry-After: %s): %s", retryAfter, string(body))
			}
		}

		return nil, fmt.Errorf("Figma API 错误 (状态码 %d): %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var result struct {
		Err    string            `json:"err"`
		Status int               `json:"status"`
		Images map[string]string `json:"images"`
	}

	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if result.Err != "" {
		return nil, fmt.Errorf("Figma API 返回错误: %s", result.Err)
	}

	if len(result.Images) == 0 {
		return nil, fmt.Errorf("未获取到任何图片 URL")
	}

	// 即使成功，也检查是否有 Retry-After（预防性处理）
	HandleRetryAfter(token, resp)

	return result.Images, nil
}

// ===================== 队列清理 =====================

// runQueueCleanup 运行队列清理器
func (s *SchedulerService) runQueueCleanup() {
	ticker := time.NewTicker(time.Duration(s.queueCleanupInterval) * time.Hour)
	defer ticker.Stop()

	log.Printf("✅ [队列清理器] 已启动")

	for {
		select {
		case <-ticker.C:
			s.cleanupQueues()
		case <-s.stopChan:
			log.Printf("🛑 [队列清理器] 已停止")
			return
		}
	}
}

// cleanupQueues 清理过期的队列记录
func (s *SchedulerService) cleanupQueues() {
	log.Printf("🧹 [队列清理器] 开始清理过期队列...")

	// 清理逻辑：
	// 1. 删除 24 小时前已完成的队列
	// 2. 删除 7 天前失败的队列
	// 3. 重置长时间卡在 processing 状态的队列
	// 4. 清理不再使用的 token 对应的队列和缓存

	cutoffCompleted := uint32(time.Now().Unix()) - 24*3600 // 24小时前
	cutoffFailed := uint32(time.Now().Unix()) - 7*24*3600  // 7天前
	cutoffProcessing := uint32(time.Now().Unix()) - 3600   // 1小时前

	db := models.DB

	// 删除已完成的队列
	result := db.Where("status = ? AND updated_at < ?", "completed", cutoffCompleted).
		Delete(&models.FigmaRenderQueue{})
	if result.Error != nil {
		log.Printf("❌ [队列清理器] 清理已完成队列失败: %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("🗑️ [队列清理器] 清理了 %d 个已完成队列", result.RowsAffected)
	}

	// 删除失败的队列
	result = db.Where("status = ? AND updated_at < ?", "failed", cutoffFailed).
		Delete(&models.FigmaRenderQueue{})
	if result.Error != nil {
		log.Printf("❌ [队列清理器] 清理失败队列失败: %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("🗑️ [队列清理器] 清理了 %d 个失败队列", result.RowsAffected)
	}

	// 重置卡住的处理中队列
	result = db.Model(&models.FigmaRenderQueue{}).
		Where("status = ? AND updated_at < ?", "processing", cutoffProcessing).
		Updates(map[string]interface{}{
			"status":        "waiting",
			"error_message": "重新排队（处理超时）",
			"updated_at":    uint32(time.Now().Unix()),
		})
	if result.Error != nil {
		log.Printf("❌ [队列清理器] 重置卡住队列失败: %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("🔄 [队列清理器] 重置了 %d 个卡住队列", result.RowsAffected)
	}

	// 清理不再使用的 token 对应的数据
	s.cleanupOrphanedTokenData()

	log.Printf("✅ [队列清理器] 清理完成")
}

// cleanupOrphanedTokenData 清理不再使用的 token 对应的队列和缓存
func (s *SchedulerService) cleanupOrphanedTokenData() {
	db := models.DB

	// 1. 获取所有用户的有效 token
	var validTokens []string
	err := db.Table("users").
		Where("figma_token != ?", "").
		Pluck("figma_token", &validTokens).Error

	if err != nil {
		log.Printf("❌ [清理孤立数据] 查询有效token失败: %v", err)
		return
	}

	if len(validTokens) == 0 {
		log.Printf("⚠️ [清理孤立数据] 没有有效的token，跳过清理")
		return
	}

	log.Printf("📊 [清理孤立数据] 发现 %d 个有效token", len(validTokens))

	// 2. 清理 figma_render_queues 中不再使用的 token 数据
	result := db.Where("figma_token NOT IN ?", validTokens).
		Delete(&models.FigmaRenderQueue{})
	if result.Error != nil {
		log.Printf("❌ [清理孤立数据] 清理渲染队列失败: %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("🗑️ [清理孤立数据] 清理了 %d 条孤立的渲染队列记录", result.RowsAffected)
	}

	// 3. 清理 figma_file_caches 中不再使用的 token 数据
	result = db.Where("figma_token NOT IN ?", validTokens).
		Delete(&models.FigmaFileCache{})
	if result.Error != nil {
		log.Printf("❌ [清理孤立数据] 清理文件缓存失败: %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("🗑️ [清理孤立数据] 清理了 %d 条孤立的文件缓存记录", result.RowsAffected)
	}

	// 4. 清理 figma_token_cooldowns 中不再使用的 token 数据
	result = db.Where("figma_token NOT IN ?", validTokens).
		Delete(&models.FigmaTokenCooldown{})
	if result.Error != nil {
		log.Printf("❌ [清理孤立数据] 清理token冷却记录失败: %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("🗑️ [清理孤立数据] 清理了 %d 条孤立的token冷却记录", result.RowsAffected)
	}
}

// runFileCacheQueueProcessor 文件缓存队列处理器
func (s *SchedulerService) runFileCacheQueueProcessor() {
	ticker := time.NewTicker(time.Duration(s.renderQueueInterval) * time.Second)
	defer ticker.Stop()

	log.Printf("📁 [文件缓存处理器] 已启动")

	for {
		select {
		case <-s.stopChan:
			log.Printf("🛑 [文件缓存处理器] 停止")
			return
		case <-ticker.C:
			s.processFileCacheQueue()
		}
	}
}

// processFileCacheQueue 处理文件获取队列
func (s *SchedulerService) processFileCacheQueue() {
	// 查询等待中的文件获取请求（status = 'waiting'）
	// Token 冷却时间由 figma_token_cooldowns 表统一管理
	queues, err := models.GetWaitingFileFetchQueues()

	if err != nil {
		log.Printf("❌ [文件获取处理器] 查询队列失败: %v", err)
		return
	}

	if len(queues) == 0 {
		return // 没有待处理的队列
	}

	// 限制每次处理数量
	if len(queues) > 10 {
		queues = queues[:10]
	}

	log.Printf("📁 [文件获取处理器] 找到 %d 个待处理的请求", len(queues))

	for _, queue := range queues {
		// ============ 关键：检查 FigmaToken 字段 ============
		if queue.FigmaToken == "" {
			log.Printf("❌ [文件获取处理器] 队列记录缺少 figma_token 字段 (QueueID=%d)", queue.ID)
			models.UpdateFileFetchQueueStatus(queue.ID, "error", "缺少 figma_token 字段")
			continue
		}

		log.Printf("🔍 [文件获取处理器] 处理队列记录 (QueueID=%d, Token=%s..., FileKey=%s)",
			queue.ID, queue.FigmaToken[:10], queue.FileKey)

		// 更新状态为 loading
		if err := models.UpdateFileFetchQueueStatus(queue.ID, "loading", ""); err != nil {
			log.Printf("⚠️ [文件获取处理器] 更新状态失败: %v", err)
			continue
		}

		// ============ 必须使用队列记录中的 figma_token ============
		// 禁止切换到其他冷却好的 token
		token := queue.FigmaToken

		// 检查 Token 冷却状态
		canRequest, waitSeconds, err := s.cacheService.CheckFileAPICooldown(token)
		if err != nil {
			log.Printf("⚠️ [文件获取处理器] 检查冷却状态失败: %v", err)
			models.UpdateFileFetchQueueStatus(queue.ID, "error", "检查冷却状态失败")
			continue
		}

		if !canRequest {
			// 还在冷却中，保持 waiting 状态，等待下次轮询
			// Token 冷却时间由 figma_token_cooldowns 表统一管理
			models.DB.Model(&models.FigmaFileFetchQueue{}).
				Where("id = ?", queue.ID).
				Updates(map[string]interface{}{
					"status":     "waiting",
					"updated_at": uint32(time.Now().Unix()),
				})
			log.Printf("⏰ [文件获取处理器] Token 冷却中，延迟处理 (QueueID=%d, Token=%s..., WaitSeconds=%d)",
				queue.ID, token[:10], waitSeconds)
			continue
		}

		// Token 可用，调用 Figma API 获取节点树
		log.Printf("🔄 [文件获取处理器] 开始获取节点树 (QueueID=%d, Token=%s..., FileKey=%s, RootNodeID=%s)",
			queue.ID, token[:10], queue.FileKey, queue.RootNodeID)

		// 更新冷却时间
		if err := s.cacheService.UpdateFileRequestTime(token); err != nil {
			log.Printf("⚠️ [文件获取处理器] 更新冷却时间失败: %v", err)
		}

		// 调用 Figma API（GetFigmaNodesNoCache 会保存到数据库）
		nodes, err := GetFigmaNodesNoCache(token, queue.FileKey, queue.RootNodeID)
		if err != nil {
			log.Printf("❌ [文件获取处理器] 获取失败 (QueueID=%d): %v", queue.ID, err)
			models.UpdateFileFetchQueueStatus(queue.ID, "error", err.Error())
			continue
		}

		// 更新队列状态为 loaded
		if err := models.UpdateFileFetchQueueStatus(queue.ID, "loaded", ""); err != nil {
			log.Printf("⚠️ [文件获取处理器] 更新队列状态失败: %v", err)
		}

		log.Printf("✅ [文件获取处理器] 获取成功 (QueueID=%d, Token=%s..., NodeCount=%d)",
			queue.ID, token[:10], len(nodes))
	}
}
