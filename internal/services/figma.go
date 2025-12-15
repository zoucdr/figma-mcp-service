package services

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/figma-deliver/internal/models"
	"golang.org/x/net/proxy"
)

// FigmaService Figma API服务
type FigmaService struct {
	Token string
}

// figmaImagesAPIRateLimiter 单个 Token 的 Figma Images API 速率限制器
type figmaImagesAPIRateLimiter struct {
	mu           sync.Mutex
	lastCallTime time.Time
	minInterval  time.Duration
	token        string // 标识哪个 token
}

// figmaImagesAPIRateLimiterManager 管理所有 token 的速率限制器
type figmaImagesAPIRateLimiterManager struct {
	mu          sync.RWMutex
	limiters    map[string]*figmaImagesAPIRateLimiter
	minInterval time.Duration
}

// 全局速率限制器管理器
var globalFigmaImagesAPILimiterManager = &figmaImagesAPIRateLimiterManager{
	limiters:    make(map[string]*figmaImagesAPIRateLimiter),
	minInterval: getFigmaAPIRateLimit(),
}

// 全局 OBS 服务实例（由 main.go 设置）
var globalOBSService *OBSService

// SetGlobalOBSService 设置全局 OBS 服务实例
func SetGlobalOBSService(obs *OBSService) {
	globalOBSService = obs
}

// getFigmaAPIRateLimit 从环境变量或配置文件获取速率限制配置（单位：秒）
func getFigmaAPIRateLimit() time.Duration {
	rateLimitStr := os.Getenv("FIGMA_API_RATE_LIMIT")
	if rateLimitStr == "" {
		// 默认值：10秒
		return 10 * time.Second
	}

	rateLimit := 10 // 默认值
	if val, err := strconv.Atoi(rateLimitStr); err == nil && val > 0 {
		rateLimit = val
	}

	return time.Duration(rateLimit) * time.Second
}

// getOrCreateLimiter 获取或创建指定 token 的速率限制器
func (m *figmaImagesAPIRateLimiterManager) getOrCreateLimiter(token string) *figmaImagesAPIRateLimiter {
	// 生成 token 的简短标识（用于日志）
	tokenID := token
	if len(token) > 10 {
		tokenID = token[:10] + "***"
	}

	// 先尝试读锁获取
	m.mu.RLock()
	limiter, exists := m.limiters[token]
	m.mu.RUnlock()

	if exists {
		return limiter
	}

	// 不存在则创建新的限制器
	m.mu.Lock()
	defer m.mu.Unlock()

	// 双重检查，防止并发创建
	limiter, exists = m.limiters[token]
	if exists {
		return limiter
	}

	// 创建新的限制器
	limiter = &figmaImagesAPIRateLimiter{
		minInterval: m.minInterval,
		token:       tokenID,
	}
	m.limiters[token] = limiter

	fmt.Printf("🔑 为 Token [%s] 创建独立速率限制器，间隔=%s\n", tokenID, m.minInterval)

	return limiter
}

// Wait 等待直到允许调用 API（如果需要排队则阻塞）
func (rl *figmaImagesAPIRateLimiter) Wait() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	timeSinceLastCall := now.Sub(rl.lastCallTime)

	if timeSinceLastCall < rl.minInterval {
		// 需要等待
		waitDuration := rl.minInterval - timeSinceLastCall
		fmt.Printf("⏱️ [Token:%s] Figma Images API 速率限制: 距离上次调用仅 %.1f 秒，需要等待 %.1f 秒\n",
			rl.token, timeSinceLastCall.Seconds(), waitDuration.Seconds())
		time.Sleep(waitDuration)
	}

	// 更新最后调用时间
	rl.lastCallTime = time.Now()
	fmt.Printf("✅ [Token:%s] Figma Images API 调用已允许，下次可调用时间: %s\n",
		rl.token, rl.lastCallTime.Add(rl.minInterval).Format("15:04:05"))
}

// CreateProxyFunc 根据代理URL创建代理函数（支持HTTP/HTTPS/SOCKS5）
func CreateProxyFunc(proxyURL string) (func(*http.Request) (*url.URL, error), error) {
	if proxyURL == "" {
		return nil, nil
	}

	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("无效的代理URL: %v", err)
	}

	// 对于HTTP/HTTPS代理，使用标准的ProxyURL
	if parsedURL.Scheme == "http" || parsedURL.Scheme == "https" {
		return http.ProxyURL(parsedURL), nil
	}

	// SOCKS5代理不能通过ProxyURL设置，需要在Transport层面处理
	// 这里返回nil，让CreateHTTPClientWithToken特殊处理
	return nil, nil
}

// CreateHTTPClientWithToken 根据token查找用户并创建支持代理的HTTP客户端（支持HTTP/HTTPS/SOCKS5）
func CreateHTTPClientWithToken(token string) *http.Client {
	// 查找使用该token的用户
	var user models.User
	err := models.DB.Where("figma_token = ?", token).First(&user).Error

	transport := &http.Transport{}

	// 如果找到用户且启用了代理，配置代理
	if err == nil && user.ProxyEnabled && user.ProxyURL != "" {
		parsedURL, parseErr := url.Parse(user.ProxyURL)
		if parseErr != nil {
			fmt.Printf("⚠️ 用户 %s 代理URL解析失败，使用直连: %v\n", user.Username, parseErr)
		} else {
			// 根据协议类型选择不同的代理方式
			if parsedURL.Scheme == "socks5" {
				// SOCKS5代理
				dialer, dialErr := proxy.SOCKS5("tcp", parsedURL.Host, nil, proxy.Direct)
				if dialErr != nil {
					fmt.Printf("⚠️ 用户 %s SOCKS5代理配置失败，使用直连: %v\n", user.Username, dialErr)
				} else {
					// 使用 DialContext 支持 context 和超时控制
					transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
						return dialer.Dial(network, addr)
					}
					fmt.Printf("✅ 用户 %s 使用SOCKS5代理: %s\n", user.Username, user.ProxyURL)
				}
			} else if parsedURL.Scheme == "http" || parsedURL.Scheme == "https" {
				// HTTP/HTTPS代理
				transport.Proxy = http.ProxyURL(parsedURL)
				fmt.Printf("✅ 用户 %s 使用HTTP代理: %s\n", user.Username, user.ProxyURL)
			} else {
				fmt.Printf("⚠️ 用户 %s 不支持的代理类型: %s，使用直连\n", user.Username, parsedURL.Scheme)
			}
		}
	}

	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}
}

// CreateHTTPClient 根据userID创建支持代理的HTTP客户端（支持HTTP/HTTPS/SOCKS5）
func CreateHTTPClient(userID uint) *http.Client {
	// 获取用户配置
	var user models.User
	models.DB.First(&user, userID)

	transport := &http.Transport{}

	// 如果启用了代理，配置代理
	if user.ProxyEnabled && user.ProxyURL != "" {
		parsedURL, parseErr := url.Parse(user.ProxyURL)
		if parseErr != nil {
			fmt.Printf("⚠️ 用户 %s 代理URL解析失败，使用直连: %v\n", user.Username, parseErr)
		} else {
			// 根据协议类型选择不同的代理方式
			if parsedURL.Scheme == "socks5" {
				// SOCKS5代理
				dialer, dialErr := proxy.SOCKS5("tcp", parsedURL.Host, nil, proxy.Direct)
				if dialErr != nil {
					fmt.Printf("⚠️ 用户 %s SOCKS5代理配置失败，使用直连: %v\n", user.Username, dialErr)
				} else {
					// 使用 DialContext 支持 context 和超时控制
					transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
						return dialer.Dial(network, addr)
					}
					fmt.Printf("✅ 用户 %s 使用SOCKS5代理: %s\n", user.Username, user.ProxyURL)
				}
			} else if parsedURL.Scheme == "http" || parsedURL.Scheme == "https" {
				// HTTP/HTTPS代理
				transport.Proxy = http.ProxyURL(parsedURL)
				fmt.Printf("✅ 用户 %s 使用HTTP代理: %s\n", user.Username, user.ProxyURL)
			} else {
				fmt.Printf("⚠️ 用户 %s 不支持的代理类型: %s，使用直连\n", user.Username, parsedURL.Scheme)
			}
		}
	}

	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}
}

// formatScaleForFilename 格式化缩放等级用于文件名
// 如果是整数（如1.0, 2.0），去掉".0"，使用整数部分（如"1", "2"）
// 如果是非整数（如0.5, 1.5），保持小数点格式（如"0.5", "1.5"）
func formatScaleForFilename(scale float64) string {
	// 如果是整数（如1.0, 2.0），去掉".0"，使用整数部分
	if scale == float64(int64(scale)) {
		return fmt.Sprintf("%.0f", scale)
	}
	// 如果是非整数，保持小数点格式
	return fmt.Sprintf("%.1f", scale)
}

// printDetailedHTTPError 打印详细的HTTP错误信息，包括响应头和响应体
func printDetailedHTTPError(resp *http.Response, responseBody []byte) {
	fmt.Println("========== HTTP 错误详情 ==========")
	fmt.Printf("状态码: %d %s\n", resp.StatusCode, resp.Status)

	// 打印响应头
	fmt.Println("\n--- 响应头 (Response Headers) ---")
	for key, values := range resp.Header {
		for _, value := range values {
			fmt.Printf("  %s: %s\n", key, value)
		}
	}

	// 打印响应体
	fmt.Println("\n--- 响应体 (Response Body) ---")
	if len(responseBody) > 0 {
		fmt.Printf("%s\n", string(responseBody))

		// 尝试解析JSON以获取更多信息
		var jsonData map[string]interface{}
		if err := json.Unmarshal(responseBody, &jsonData); err == nil {
			if errMsg, ok := jsonData["err"].(string); ok {
				fmt.Printf("\n  错误消息: %s\n", errMsg)
			}
			if status, ok := jsonData["status"].(float64); ok {
				fmt.Printf("  状态: %.0f\n", status)
			}
		}
	} else {
		fmt.Println("  (响应体为空)")
	}

	// 特别处理429错误
	if resp.StatusCode == http.StatusTooManyRequests {
		fmt.Println("\n⚠️  已达到 Figma API 请求速率限制")
		if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
			fmt.Printf("⏱️  建议等待时间: %s 秒\n", retryAfter)
		}
		if xRateLimit := resp.Header.Get("X-RateLimit-Limit"); xRateLimit != "" {
			fmt.Printf("📊  速率限制: %s 请求/分钟\n", xRateLimit)
		}
		if xRateLimitRemaining := resp.Header.Get("X-RateLimit-Remaining"); xRateLimitRemaining != "" {
			fmt.Printf("📊  剩余配额: %s\n", xRateLimitRemaining)
		}
		if xRateLimitReset := resp.Header.Get("X-RateLimit-Reset"); xRateLimitReset != "" {
			fmt.Printf("🔄  配额重置时间: %s\n", xRateLimitReset)
		}
	}

	fmt.Println("===================================")
}

// ParseFigmaLink 解析Figma链接
func ParseFigmaLink(link string) (fileKey, nodeId string, err error) {
	// 检查链接是否为空
	if link == "" {
		return "", "", errors.New("链接不能为空")
	}

	// 检查是否是Figma链接
	if !strings.Contains(link, "figma.com") {
		return "", "", errors.New("不是有效的Figma链接")
	}

	// 提取文件ID
	fileKeyRegex := regexp.MustCompile(`figma\.com\/(?:file|design)\/([^\/\?]+)`)
	fileKeyMatches := fileKeyRegex.FindStringSubmatch(link)
	if len(fileKeyMatches) < 2 {
		return "", "", errors.New("无法提取Figma文件ID")
	}
	fileKey = fileKeyMatches[1]

	// 提取节点ID
	nodeIdRegex := regexp.MustCompile(`node-id=([^&]+)`)
	nodeIdMatches := nodeIdRegex.FindStringSubmatch(link)
	if len(nodeIdMatches) >= 2 {
		// 替换URL编码的字符
		nodeId = strings.ReplaceAll(nodeIdMatches[1], "%3A", ":")
	}

	return fileKey, nodeId, nil
}

// GetFigmaProjectNodes 获取Figma项目节点
func GetFigmaProjectNodes(userID, projectID uint, fileKey, nodeID string) ([]models.FigmaNode, error) {
	// 获取用户信息
	var user models.User
	result := models.DB.First(&user, userID)
	if result.Error != nil {
		return nil, result.Error
	}

	// 如果没有Figma Token，返回错误
	if user.FigmaToken == "" {
		return nil, errors.New("请先在个人资料页面配置 Figma Token")
	}

	// 获取节点信息（支持 WebSocket 兜底）
	nodes, err := GetFigmaNodesWithUser(user.FigmaToken, fileKey, nodeID, user.ID)
	if err != nil {
		return nil, err
	}

	// 保存节点信息
	var savedNodes []models.FigmaNode
	for _, node := range nodes {
		figmaNode, err := CreateFigmaNode(projectID, node["id"].(string), node["name"].(string), node["type"].(string), node["parent_id"])
		if err != nil {
			continue
		}
		savedNodes = append(savedNodes, *figmaNode)
	}

	return savedNodes, nil
}

// GetFigmaNodesBatch 批量获取Figma节点（支持多个节点ID，一次API调用）
func GetFigmaNodesBatch(token, fileKey string, nodeIDs []string) (map[string][]map[string]interface{}, error) {
	if len(nodeIDs) == 0 {
		return nil, fmt.Errorf("节点ID列表为空")
	}

	// 拼接所有节点ID（用逗号分隔）
	idsParam := strings.Join(nodeIDs, ",")
	fmt.Printf("📡 批量调用 Figma API 获取节点树: fileKey=%s, nodeIDs=%v\n", fileKey, nodeIDs)

	// 构建API URL
	url := fmt.Sprintf("https://api.figma.com/v1/files/%s/nodes?ids=%s", fileKey, idsParam)

	// 创建请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// 设置请求头
	req.Header.Set("X-Figma-Token", token)

	// 发送请求（使用支持代理的客户端）
	client := CreateHTTPClientWithToken(token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 先读取响应体（无论状态码如何都需要读取）
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		// 处理速率限制和 Retry-After
		if CheckAndHandleRateLimitResponse(token, resp) {
			return nil, FormatRetryError(resp)
		}

		// 打印详细的错误信息
		printDetailedHTTPError(resp, respBody)
		return nil, fmt.Errorf("API请求失败: %s", resp.Status)
	}

	// 即使成功，也检查是否有 Retry-After（预防性处理）
	HandleRetryAfter(token, resp)

	// 解析响应
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	// 解析每个节点的数据
	allNodesData := make(map[string][]map[string]interface{})

	// Figma API 返回格式：{"nodes": {"nodeID1": {...}, "nodeID2": {...}}}
	nodesMap, ok := result["nodes"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("响应格式错误：缺少 nodes 字段")
	}

	// 遍历每个节点
	for _, nodeID := range nodeIDs {
		nodeData, exists := nodesMap[nodeID]
		if !exists {
			fmt.Printf("⚠️ 节点 %s 不存在于响应中\n", nodeID)
			continue
		}

		// 构造单个节点的响应格式（与 GetFigmaNodesNoCache 一致）
		singleNodeResult := map[string]interface{}{
			"nodes": map[string]interface{}{
				nodeID: nodeData,
			},
		}

		// 解析节点数据
		nodes := parseFigmaNodes(singleNodeResult, nodeID)
		allNodesData[nodeID] = nodes

		// 缓存单个节点数据到本地文件
		singleNodeJSON, err := json.Marshal(singleNodeResult)
		if err != nil {
			fmt.Printf("⚠️ 序列化节点 %s 数据失败: %v\n", nodeID, err)
			continue
		}

		if err := cacheNodes(fileKey, nodeID, singleNodeJSON); err != nil {
			fmt.Printf("⚠️ 缓存节点 %s 到本地文件失败: %v\n", nodeID, err)
		} else {
			fmt.Printf("✅ 节点 %s 已缓存到本地文件\n", nodeID)
		}

		// 异步保存到数据库
		go func(nID string, nData []byte) {
			// 提取所有可见节点ID（排除 visible=false 的节点）
			nodeIDList, err := models.ExtractVisibleNodeIDs(string(nData))
			if err != nil {
				fmt.Printf("⚠️ 提取节点 %s 的ID失败: %v\n", nID, err)
				nodeIDList = []string{}
			}
			nodeIDsStr := strings.Join(nodeIDList, ",")

			now := uint32(time.Now().Unix())

			// 检查数据库中是否已有记录
			existingCache, err := models.GetFileCache(fileKey, nID)
			if err != nil || existingCache == nil {
				// 创建新记录
				cache := &models.FigmaFileCache{
					FileKey:     fileKey,
					RootNodeID:  nID,
					FileData:    string(nData),
					NodeIDs:     nodeIDsStr,
					FileVersion: "",
					HitCount:    0,
					CreatedAt:   now,
					UpdatedAt:   now,
				}

				if err := models.CreateFileCache(cache); err != nil {
					fmt.Printf("❌ 保存节点 %s 到数据库失败: %v\n", nID, err)
				} else {
					fmt.Printf("✅ 节点 %s 已保存到数据库: token=%s..., 节点数=%d\n", nID, token[:10], len(nodeIDList))
				}
			} else {
				// 更新已有记录
				existingCache.FileData = string(nData)
				existingCache.NodeIDs = nodeIDsStr
				existingCache.UpdatedAt = now

				if err := models.UpdateFileCache(existingCache); err != nil {
					fmt.Printf("❌ 更新节点 %s 到数据库失败: %v\n", nID, err)
				} else {
					fmt.Printf("✅ 节点 %s 已更新到数据库\n", nID)
				}
			}
		}(nodeID, singleNodeJSON)
	}

	fmt.Printf("✅ 批量获取节点成功: 共 %d 个节点\n", len(allNodesData))
	return allNodesData, nil
}

// GetFigmaNodesNoCache 获取Figma节点（不使用缓存，直接调用API并保存到缓存）
func GetFigmaNodesNoCache(token, fileKey, nodeID string) ([]map[string]interface{}, error) {
	fmt.Printf("📡 调用 Figma API 获取节点树（跳过缓存）: fileKey=%s, nodeID=%s\n", fileKey, nodeID)

	// 构建API URL
	url := fmt.Sprintf("https://api.figma.com/v1/files/%s", fileKey)
	if nodeID != "" {
		url = fmt.Sprintf("%s/nodes?ids=%s", url, nodeID)
	}

	// 创建请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// 设置请求头
	req.Header.Set("X-Figma-Token", token)

	// 发送请求（使用支持代理的客户端）
	client := CreateHTTPClientWithToken(token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 先读取响应体（无论状态码如何都需要读取）
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		// 处理速率限制和 Retry-After
		if CheckAndHandleRateLimitResponse(token, resp) {
			return nil, FormatRetryError(resp)
		}

		// 打印详细的错误信息
		printDetailedHTTPError(resp, respBody)
		return nil, fmt.Errorf("API请求失败: %s", resp.Status)
	}

	// 即使成功，也检查是否有 Retry-After（预防性处理）
	HandleRetryAfter(token, resp)

	// 解析响应
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	// 处理节点数据
	nodes := parseFigmaNodes(result, nodeID)

	// 缓存节点数据到本地文件
	if err := cacheNodes(fileKey, nodeID, respBody); err != nil {
		fmt.Printf("⚠️ 缓存节点树到本地文件失败: %v\n", err)
	} else {
		fmt.Printf("✅ 节点树已缓存到本地文件\n")
	}

	// 异步保存到数据库
	go func() {
		// 提取所有可见节点ID（排除 visible=false 的节点）
		nodeIDList, err := models.ExtractVisibleNodeIDs(string(respBody))
		if err != nil {
			fmt.Printf("⚠️ 提取节点ID失败: %v\n", err)
			nodeIDList = []string{}
		}
		nodeIDsStr := strings.Join(nodeIDList, ",")

		now := uint32(time.Now().Unix())

		// 检查数据库中是否已有记录
		existingCache, err := models.GetFileCache(fileKey, nodeID)
		if err != nil || existingCache == nil {
			// 创建新记录
			cache := &models.FigmaFileCache{
				FileKey:     fileKey,
				RootNodeID:  nodeID,
				FileData:    string(respBody),
				NodeIDs:     nodeIDsStr,
				FileVersion: "",
				HitCount:    0,
				CreatedAt:   now,
				UpdatedAt:   now,
			}

			if err := models.CreateFileCache(cache); err != nil {
				fmt.Printf("❌ 保存节点树到数据库失败: %v\n", err)
			} else {
				fmt.Printf("✅ 节点树已保存到数据库: token=%s..., 节点数=%d\n", token[:10], len(nodeIDList))
			}
		} else {
			// 更新已有记录
			existingCache.FileData = string(respBody)
			existingCache.NodeIDs = nodeIDsStr
			existingCache.UpdatedAt = now

			if err := models.UpdateFileCache(existingCache); err != nil {
				fmt.Printf("❌ 更新数据库节点树失败: %v\n", err)
			} else {
				fmt.Printf("✅ 数据库节点树已更新: 节点数=%d\n", len(nodeIDList))
			}
		}
	}()

	return nodes, nil
}

// GetFigmaNodes 获取Figma节点，支持缓存
// 为了保持向后兼容，提供无 userID 参数的版本
func GetFigmaNodes(token, fileKey, nodeID string) ([]map[string]interface{}, error) {
	return GetFigmaNodesWithUser(token, fileKey, nodeID, 0)
}

// GetFigmaNodesWithUser 获取Figma节点，支持缓存和 WebSocket 兜底
// userID: 用户ID，如果为 0 则不尝试 WebSocket 兜底
func GetFigmaNodesWithUser(token, fileKey, nodeID string, userID uint) ([]map[string]interface{}, error) {
	// 如果 nodeID 含有 "-"，转换为 "_"
	if strings.Contains(nodeID, "-") {
		nodeID = strings.ReplaceAll(nodeID, "-", ":")
	}
	// 1. 检查本地文件缓存
	cachedNodes, err := loadCachedNodes(fileKey, nodeID)
	if err == nil && cachedNodes != nil {
		fmt.Printf("✅ 使用本地缓存的节点树数据: fileKey=%s, nodeID=%s\n", fileKey, nodeID)
		return cachedNodes, nil
	}
	fmt.Printf("⚠️ 本地缓存未找到: %v\n", err)

	// 2. 从数据库查询节点树缓存
	fmt.Printf("🔍 查询数据库缓存: fileKey=%s, nodeID=%s\n", fileKey, nodeID)
	dbCache, err := models.GetFileCache(fileKey, nodeID)
	if err == nil && dbCache != nil && dbCache.FileData != "" {
		fmt.Printf("✅ 从数据库缓存获取节点树: fileKey=%s, nodeID=%s, 数据大小=%d bytes\n",
			fileKey, nodeID, len(dbCache.FileData))

		// 解析数据库中的 JSON 数据
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(dbCache.FileData), &result); err == nil {
			// 调试：打印数据结构
			if nodesMap, ok := result["nodes"].(map[string]interface{}); ok {
				fmt.Printf("📊 [DB缓存] 包含 %d 个节点键\n", len(nodesMap))
				for nid, ndata := range nodesMap {
					if ndataMap, ok := ndata.(map[string]interface{}); ok {
						if doc, ok := ndataMap["document"].(map[string]interface{}); ok {
							if children, ok := doc["children"].([]interface{}); ok {
								fmt.Printf("📊 [DB缓存] 节点 %s 有 %d 个子节点\n", nid, len(children))
							} else {
								fmt.Printf("📊 [DB缓存] 节点 %s 没有 children 字段\n", nid)
							}
						}
					}
				}
			} else if doc, ok := result["document"].(map[string]interface{}); ok {
				if children, ok := doc["children"].([]interface{}); ok {
					fmt.Printf("📊 [DB缓存] document 格式，有 %d 个子节点\n", len(children))
				}
			}

			nodes := parseFigmaNodes(result, nodeID)
			fmt.Printf("📊 [DB缓存] 解析后返回 %d 个节点\n", len(nodes))

			// 同时保存到本地文件缓存，加速下次访问
			go func() {
				if err := cacheNodes(fileKey, nodeID, []byte(dbCache.FileData)); err != nil {
					fmt.Printf("⚠️ 保存到本地文件缓存失败: %v\n", err)
				}
			}()

			return nodes, nil
		} else {
			fmt.Printf("⚠️ 解析数据库缓存数据失败: %v\n", err)
		}
	} else {
		fmt.Printf("⚠️ 数据库缓存未找到或状态异常\n")
	}

	// 3. 如果提供了有效的 userID，尝试通过 WebSocket 请求 Figma 插件
	if userID > 0 {
		mcpService := GetMCPService()
		if mcpService == nil {
			fmt.Printf("⚠️ [GetFigmaNodesWithUser] MCP 服务未初始化\n")
			return nil, fmt.Errorf("节点数据缓存未找到，且 MCP 服务不可用")
		}

		connection, exists := mcpService.GetUserConnection(userID)
		if exists && connection != nil && mcpService.HasActiveWebSocketConnection(connection.ConnectionID) {
			fmt.Printf("🔌 [GetFigmaNodesWithUser] 发现活跃 WebSocket 连接 (UserID=%d, ConnectionID=%s)，发送插件请求并等待响应\n",
				userID, connection.ConnectionID)

			// 构造 read_my_design 工具调用（使用与 mcp_tools.go 相同的格式）
			requestID := fmt.Sprintf("req_service_%d_%s", time.Now().UnixNano(), strings.ReplaceAll(nodeID, ":", "_"))

			// 创建响应通道
			responseChan := make(chan interface{}, 1)
			errorChan := make(chan error, 1)

			// 注册请求等待响应
			mcpService.RegisterPendingRequest(requestID, responseChan, errorChan)
			defer mcpService.UnregisterPendingRequest(requestID)

			// 构建发送到 Figma 插件的消息（与 mcp_tools.go 相同的格式）
			message := map[string]interface{}{
				"id":      requestID,
				"command": "read_my_design",
				"params": map[string]interface{}{
					"nodeId": nodeID,
				},
			}

			// 获取当前用户的频道
			channel := mcpService.GetUserChannel(connection.ConnectionID)
			if channel == "" {
				// 如果没有加入频道，默认使用 figma-bridge
				channel = "figma-bridge"
				// 自动加入默认频道
				_ = mcpService.JoinChannel(connection.ConnectionID, channel)
			}

			// 广播消息到频道
			err := mcpService.BroadcastToChannel(connection.ConnectionID, channel, message)
			if err != nil {
				fmt.Printf("❌ [GetFigmaNodesWithUser] 发送消息到频道失败: %v\n", err)
				return nil, fmt.Errorf("发送消息到 Figma 插件失败: %v", err)
			}

			fmt.Printf("✅ [GetFigmaNodesWithUser] 已发送请求到频道 %s，等待响应...\n", channel)

			// 等待响应（30秒超时）
			var response interface{}
			select {
			case response = <-responseChan:
				fmt.Printf("✅ [GetFigmaNodesWithUser] 收到响应 (ID=%s)\n", requestID)
			case respErr := <-errorChan:
				fmt.Printf("❌ [GetFigmaNodesWithUser] 收到错误响应 (ID=%s): %v\n", requestID, respErr)
				return nil, fmt.Errorf("通过 Figma 插件获取数据失败: %v", respErr)
			case <-time.After(30 * time.Second):
				fmt.Printf("⏰ [GetFigmaNodesWithUser] 请求超时 (ID=%s)\n", requestID)
				return nil, fmt.Errorf("请求超时，未在 30 秒内收到插件响应")
			}

			fmt.Printf("✅ [GetFigmaNodesWithUser] 收到插件响应，解析节点数据\n")

			// 解析响应数据
			// 插件响应格式应该是节点树的 JSON 数据
			responseData, ok := response.(map[string]interface{})
			if !ok {
				fmt.Printf("❌ [GetFigmaNodesWithUser] 响应数据格式错误\n")
				return nil, fmt.Errorf("插件响应数据格式错误")
			}

			// 将响应数据保存到本地缓存和数据库
			responseJSON, err := json.Marshal(responseData)
			if err != nil {
				fmt.Printf("⚠️ [GetFigmaNodesWithUser] 序列化响应数据失败: %v\n", err)
			} else {
				// 保存到本地缓存
				go func() {
					if err := cacheNodes(fileKey, nodeID, responseJSON); err != nil {
						fmt.Printf("⚠️ [GetFigmaNodesWithUser] 保存到本地缓存失败: %v\n", err)
					} else {
						fmt.Printf("✅ [GetFigmaNodesWithUser] 已保存到本地缓存\n")
					}
				}()

				// 保存到数据库缓存
				go func() {
					// 提取所有可见节点ID（排除 visible=false 的节点）
					nodeIDList, err := models.ExtractVisibleNodeIDs(string(responseJSON))
					if err != nil {
						fmt.Printf("⚠️ [GetFigmaNodesWithUser] 提取节点ID失败: %v\n", err)
						nodeIDList = []string{}
					}
					nodeIDsStr := strings.Join(nodeIDList, ",")
					now := uint32(time.Now().Unix())

					// 先尝试获取现有缓存
					existingCache, err := models.GetFileCache(fileKey, nodeID)
					if err == nil && existingCache != nil {
						// 更新现有缓存
						existingCache.FileData = string(responseJSON)
						existingCache.NodeIDs = nodeIDsStr
						existingCache.UpdatedAt = now
						if err := models.UpdateFileCache(existingCache); err != nil {
							fmt.Printf("⚠️ [GetFigmaNodesWithUser] 更新数据库缓存失败: %v\n", err)
						} else {
							fmt.Printf("✅ [GetFigmaNodesWithUser] 已更新数据库缓存 (节点数=%d)\n", len(nodeIDList))
						}
					} else {
						// 创建新缓存
						newCache := &models.FigmaFileCache{
							FileKey:    fileKey,
							RootNodeID: nodeID,
							FileData:   string(responseJSON),
							NodeIDs:    nodeIDsStr,
							CreatedAt:  now,
							UpdatedAt:  now,
						}
						if err := models.CreateFileCache(newCache); err != nil {
							fmt.Printf("⚠️ [GetFigmaNodesWithUser] 创建数据库缓存失败: %v\n", err)
						} else {
							fmt.Printf("✅ [GetFigmaNodesWithUser] 已创建数据库缓存 (节点数=%d)\n", len(nodeIDList))
						}
					}
				}()
			}

			// 解析节点数据
			nodes := parseFigmaNodes(responseData, nodeID)
			fmt.Printf("✅ [GetFigmaNodesWithUser] 从 WebSocket 获取并解析了 %d 个节点\n", len(nodes))
			return nodes, nil
		}

		fmt.Printf("⚠️ [GetFigmaNodesWithUser] 未找到活跃 WebSocket 连接 (UserID=%d)\n", userID)
	}

	// 4. 本地缓存和数据库都没有，且无法使用 WebSocket，返回错误
	return nil, fmt.Errorf("节点数据缓存未找到")
}

// LoadCachedNodesFromFile 加载文件缓存的节点树数据（导出给 controller 使用）
func LoadCachedNodesFromFile(fileKey, nodeID string) ([]map[string]interface{}, error) {
	return loadCachedNodes(fileKey, nodeID)
}

// ParseFigmaNodesFromData 解析Figma节点数据（导出给 controller 使用）
// 这个函数提供与 LoadCachedNodesFromFile 相同的数据格式转换
func ParseFigmaNodesFromData(data map[string]interface{}, rootNodeID string) ([]map[string]interface{}, error) {
	nodes := parseFigmaNodes(data, rootNodeID)
	if len(nodes) == 0 {
		return nil, fmt.Errorf("未能解析出有效的节点数据")
	}
	return nodes, nil
}

// loadCachedNodes 加载缓存的节点树数据
func loadCachedNodes(fileKey, nodeID string) ([]map[string]interface{}, error) {
	// 使用共享的文件读取函数
	data, _, err := LoadCachedFileData(fileKey, nodeID)
	if err != nil {
		return nil, err
	}

	// 解析JSON数据
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	// 处理节点数据
	return parseFigmaNodes(result, nodeID), nil
}

// cacheNodes 缓存节点树数据（内部使用）
func cacheNodes(fileKey, nodeID string, data []byte) error {
	// 创建安全的文件名
	safeNodeID := strings.NewReplacer("-", "_", ":", "_", ";", "_", "/", "_", "\\", "_", "?", "_", "*", "_", "\"", "_", "<", "_", ">", "_", "|", "_").Replace(nodeID)
	cacheDir := filepath.Join("temp", fileKey, "documents")

	// 确保目录存在
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}

	// 写入缓存文件
	cacheFile := filepath.Join(cacheDir, safeNodeID+".json")
	return os.WriteFile(cacheFile, data, 0644)
}

// SaveCachedNodesToFile 保存节点树数据到文件缓存（导出给 controller 使用）
func SaveCachedNodesToFile(fileKey, nodeID string, data []byte) error {
	return cacheNodes(fileKey, nodeID, data)
}

// parseFigmaNodes 解析Figma节点数据
func parseFigmaNodes(data map[string]interface{}, _ /*rootNodeID*/ string) []map[string]interface{} {
	var nodes []map[string]interface{}

	// 如果是节点查询（插件返回的格式：{ "document": {...}, "nodeId": "xxx" }）
	if document, ok := data["document"].(map[string]interface{}); ok {
		// 处理根节点 - 直接使用完整节点数据
		documentCopy := make(map[string]interface{})
		for k, v := range document {
			documentCopy[k] = v
		}

		// 优先使用 document 中的 id，如果不存在则使用外层的 nodeId
		// 这样可以保留原始节点ID（如 "46:1384"）
		var rootNodeID string
		if docID, hasID := document["id"].(string); hasID && docID != "" {
			rootNodeID = docID
		} else if nodeID, hasNodeID := data["nodeId"].(string); hasNodeID && nodeID != "" {
			rootNodeID = nodeID
			documentCopy["id"] = nodeID // 补充 id 字段
		} else {
			rootNodeID = "0:0" // 兼容旧数据：如果都没有则使用默认值
			documentCopy["id"] = "0:0"
		}

		documentCopy["parent_id"] = nil
		nodes = append(nodes, documentCopy)

		// 处理子节点
		if children, ok := document["children"].([]interface{}); ok {
			for _, child := range children {
				childNodes := parseNodeRecursive(child.(map[string]interface{}), rootNodeID)
				nodes = append(nodes, childNodes...)
			}
		}
	} else if nodeMap, ok := data["nodes"].(map[string]interface{}); ok {
		// 处理指定节点查询
		for nodeID, nodeData := range nodeMap {
			// 检查 nodeData 是否为 nil
			if nodeData == nil {
				fmt.Printf("跳过节点 %s，因为 nodeData 为 nil\n", nodeID)
				continue
			}

			// 检查 nodeData 是否为 map[string]interface{} 类型
			nodeDataMap, ok := nodeData.(map[string]interface{})
			if !ok {
				fmt.Printf("跳过节点 %s，因为 nodeData 不是 map 类型: %T\n", nodeID, nodeData)
				continue
			}

			// 检查是否有 document 字段
			documentInterface, hasDocument := nodeDataMap["document"]
			if !hasDocument || documentInterface == nil {
				fmt.Printf("跳过节点 %s，因为没有 document 字段或 document 为 nil\n", nodeID)
				continue
			}

			// 检查 document 是否为 map[string]interface{} 类型
			if node, ok := documentInterface.(map[string]interface{}); ok {
				// 添加当前节点 - 直接使用完整节点数据
				nodeCopy := make(map[string]interface{})
				for k, v := range node {
					nodeCopy[k] = v
				}
				nodeCopy["id"] = nodeID
				nodeCopy["parent_id"] = nil // 根节点没有父节点
				nodes = append(nodes, nodeCopy)

				// 处理子节点
				if children, ok := node["children"].([]interface{}); ok {
					for _, child := range children {
						childNodes := parseNodeRecursive(child.(map[string]interface{}), nodeID)
						nodes = append(nodes, childNodes...)
					}
				}
			}
		}
	}

	return nodes
}

// parseNodeRecursive 递归解析节点
func parseNodeRecursive(node map[string]interface{}, parentID string) []map[string]interface{} {
	var nodes []map[string]interface{}

	// 添加当前节点 - 直接使用完整节点数据
	nodeCopy := make(map[string]interface{})
	for k, v := range node {
		nodeCopy[k] = v
	}
	nodeCopy["parent_id"] = parentID
	nodes = append(nodes, nodeCopy)

	// 处理子节点
	if children, ok := node["children"].([]interface{}); ok {
		for _, child := range children {
			childNodes := parseNodeRecursive(child.(map[string]interface{}), node["id"].(string))
			nodes = append(nodes, childNodes...)
		}
	}

	return nodes
}

// CreateFigmaNode 创建Figma节点
func CreateFigmaNode(projectID uint, nodeID, name, nodeType string, parentID interface{}) (*models.FigmaNode, error) {
	// 先检查节点是否已存在
	existingNode, err := models.FindNodeByProjectAndNodeID(projectID, nodeID)
	if err == nil && existingNode != nil {
		// 节点已存在，保留现有的 modifys，不覆盖用户的修改信息
		// 只更新基本的节点信息（如果需要的话，这里可以选择性更新）
		return existingNode, nil
	}

	// 节点不存在，创建新节点
	// 创建节点记录，将节点信息存储在modifys字段中
	nodeInfo := map[string]interface{}{
		"name":      name,
		"type":      nodeType,
		"parent_id": parentID,
	}

	// 转换为JSON字符串
	modifysJSON, err := json.Marshal(nodeInfo)
	if err != nil {
		return nil, err
	}

	figmaNode := &models.FigmaNode{
		ProjectID: projectID,
		NodeID:    nodeID,
		Modifys:   string(modifysJSON),
	}

	err = models.SaveNode(figmaNode)
	if err != nil {
		return nil, err
	}

	return figmaNode, nil
}

// GetOrCreateFigmaProject 获取或创建Figma项目
func GetOrCreateFigmaProject(userID uint, fileKey, rootNodeID, name, figmaURL string) (*models.FigmaProject, error) {
	return GetOrCreateFigmaProjectWithGroup(userID, fileKey, rootNodeID, name, figmaURL, "")
}

// GetOrCreateFigmaProjectWithGroup 获取或创建Figma项目（支持分组名称）
func GetOrCreateFigmaProjectWithGroup(userID uint, fileKey, rootNodeID, name, figmaURL, groupName string) (*models.FigmaProject, error) {
	// 确保用户存在
	_, err := models.EnsureUserExists(userID)
	if err != nil {
		return nil, fmt.Errorf("确保用户存在失败: %w", err)
	}

	// 如果提供了rootNodeID，先尝试通过fileKey和rootNodeID精确查找
	if rootNodeID != "" {
		project, err := models.GetProjectByUserFileKeyAndRootNodeID(userID, fileKey, rootNodeID)
		if err == nil {
			// 找到了精确匹配的项目，更新URL和分组名称（如果提供了新的值）
			needSave := false
			if figmaURL != "" && project.FigmaURL != figmaURL {
				project.FigmaURL = figmaURL
				needSave = true
			}
			if groupName != "" && project.GroupName != groupName {
				project.GroupName = groupName
				needSave = true
			}
			if needSave {
				models.DB.Save(project)
			}
			return project, nil
		}
	}

	// 如果没有提供rootNodeID或者找不到精确匹配，尝试只通过fileKey查找
	if rootNodeID == "" {
		project, err := models.GetProjectByUserAndFileKey(userID, fileKey)
		if err == nil {
			// 找到了项目，但检查是否需要更新rootNodeID、URL和分组名称
			needSave := false
			if project.RootNodeID == "" && rootNodeID != "" {
				project.RootNodeID = rootNodeID
				needSave = true
			}
			if figmaURL != "" && project.FigmaURL != figmaURL {
				project.FigmaURL = figmaURL
				needSave = true
			}
			if groupName != "" && project.GroupName != groupName {
				project.GroupName = groupName
				needSave = true
			}
			if needSave {
				models.DB.Save(project)
			}
			return project, nil
		}
	}

	// 创建新项目
	project, err := models.CreateOrUpdateProjectWithGroup(userID, fileKey, rootNodeID, name, figmaURL, groupName)
	if err != nil {
		return nil, err
	}

	return project, nil
}

// DownloadPreviewFigmaImageWithOptions 获取Figma图片（从缓存、数据库、OBS或Figma CDN）
// 注意：此函数不会直接调用 Figma API 下载图片，请使用统一渲染接口
// userID: 用户ID，用于 WebSocket 兜底，为 0 则不尝试 WebSocket
func DownloadPreviewFigmaImageWithOptions(token, fileKey, nodeID, imageFormat string, imageScale float64, useCache bool, userID uint, ignoreTexts bool) (string, error) {
	// 打印调试信息
	fmt.Printf("📖 获取Figma图片: fileKey=%s, nodeID=%s, format=%s, scale=%.1f, useCache=%v, userID=%d, ignoreTexts=%v\n",
		fileKey, nodeID, imageFormat, imageScale, useCache, userID, ignoreTexts)

	// 1. 检查本地缓存文件
	tempDir := filepath.Join("temp", fileKey, "previews")
	fmt.Printf("缓存目录路径: %s\n", tempDir)
	err := os.MkdirAll(tempDir, os.ModePerm)
	if err != nil {
		fmt.Printf("创建临时文件夹失败: %v\n", err)
		return getFallbackImage()
	}

	// 生成文件名 - 替换特殊字符为下划线
	safeNodeID := strings.NewReplacer(":", "_", ";", "_", "/", "_", "\\", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_").Replace(nodeID)
	fmt.Printf("原始节点ID: %s, 安全节点ID: %s\n", nodeID, safeNodeID)

	// 如果使用缓存，检查是否已经有该节点的图片文件
	if useCache {
		existingFile := findLatestNodeImage(tempDir, safeNodeID, imageScale)
		if existingFile != "" {
			fmt.Printf("✅ 找到本地缓存图片: %s\n", existingFile)
			return existingFile, nil
		}
		fmt.Printf("⚠️ 本地缓存未找到\n")
	} else {
		fmt.Printf("跳过本地缓存检查\n")
	}

	// 2. 查询数据库中的图片记录
	fmt.Printf("🔍 查询数据库: fileKey=%s, nodeID=%s, format=%s, scale=%.1f, ignoreTexts=%v\n", fileKey, nodeID, imageFormat, imageScale, ignoreTexts)
	image, err := models.GetNodeImage(fileKey, nodeID, imageFormat, imageScale, ignoreTexts)
	if err != nil {
		fmt.Printf("⚠️ 数据库查询失败: %v\n", err)
		// 不直接返回裂图，先尝试 WebSocket
	} else if image != nil {
		// 3. 优先使用 OBS Key（如果存在）
		if image.OBSKey != "" {
			fmt.Printf("📦 找到 OBS Key: %s\n", image.OBSKey)

			// 尝试从 OBS 下载到本地缓存
			localPath, err := downloadImageFromOBS(image.OBSKey, tempDir, safeNodeID, imageFormat, imageScale)
			if err == nil && localPath != "" {
				fmt.Printf("✅ 成功从 OBS 下载图片到本地: %s\n", localPath)
				return localPath, nil
			}
			fmt.Printf("⚠️ 从 OBS 下载失败: %v，尝试 Figma CDN\n", err)
		}

		// 4. 使用 Figma CDN URL（如果存在）
		if image.FigmaCDNURL != "" {
			fmt.Printf("🌐 找到 Figma CDN URL: %s\n", image.FigmaCDNURL)

			// 尝试从 Figma CDN 下载到本地缓存
			localPath, err := downloadImageFromURL(image.FigmaCDNURL, tempDir, safeNodeID, imageFormat, imageScale)
			if err == nil && localPath != "" {
				fmt.Printf("✅ 成功从 Figma CDN 下载图片到本地: %s\n", localPath)
				return localPath, nil
			}
			fmt.Printf("⚠️ 从 Figma CDN 下载失败: %v\n", err)
		}
	} else {
		fmt.Printf("⚠️ 数据库中未找到图片记录\n")
	}

	// 5. 如果提供了有效的 userID，尝试通过 WebSocket 从 Figma 插件获取图片
	if userID > 0 {
		fmt.Printf("🔌 [DownloadPreviewFigmaImageWithOptions] 尝试通过 WebSocket 获取图片 (UserID=%d)\n", userID)
		localPath, err := tryGetImageViaWebSocket(fileKey, nodeID, imageFormat, imageScale, userID, tempDir, safeNodeID)
		if err == nil && localPath != "" {
			fmt.Printf("✅ [DownloadPreviewFigmaImageWithOptions] 成功通过 WebSocket 获取图片: %s\n", localPath)
			return localPath, nil
		}
		fmt.Printf("⚠️ [DownloadPreviewFigmaImageWithOptions] WebSocket 获取失败: %v\n", err)
	} else {
		fmt.Printf("⚠️ [DownloadPreviewFigmaImageWithOptions] 未提供 userID，跳过 WebSocket 兜底\n")
	}

	// 6. 都失败了，返回裂图
	fmt.Printf("❌ 所有获取途径都失败，返回裂图\n")
	return getFallbackImage()
}

// tryGetImageViaWebSocket 尝试通过 WebSocket 从 Figma 插件获取图片
func tryGetImageViaWebSocket(fileKey, nodeID, imageFormat string, imageScale float64, userID uint, tempDir, safeNodeID string) (string, error) {
	return TryGetImageViaWebSocket(fileKey, nodeID, imageFormat, imageScale, userID, tempDir, safeNodeID, false, []string{})
}

// TryGetImageViaWebSocket 尝试通过 WebSocket 从 Figma 插件获取图片（公共函数，支持ignoreTexts和ignoreNodes参数）
func TryGetImageViaWebSocket(fileKey, nodeID, imageFormat string, imageScale float64, userID uint, tempDir, safeNodeID string, ignoreTexts bool, ignoreNodes []string) (string, error) {
	mcpService := GetMCPService()
	if mcpService == nil {
		return "", fmt.Errorf("MCP 服务未初始化")
	}

	connection, exists := mcpService.GetUserConnection(userID)
	if !exists || connection == nil || !mcpService.HasActiveWebSocketConnection(connection.ConnectionID) {
		return "", fmt.Errorf("用户没有活跃的 WebSocket 连接")
	}

	fmt.Printf("🔌 [TryGetImageViaWebSocket] 发现活跃 WebSocket 连接 (UserID=%d, ConnectionID=%s)\n",
		userID, connection.ConnectionID)

	// 构造 export_node_as_image 工具调用
	requestID := fmt.Sprintf("req_img_%d_%s", time.Now().UnixNano(), strings.ReplaceAll(nodeID, ":", "_"))

	// 创建响应通道
	responseChan := make(chan interface{}, 1)
	errorChan := make(chan error, 1)

	// 注册请求等待响应
	mcpService.RegisterPendingRequest(requestID, responseChan, errorChan)
	defer mcpService.UnregisterPendingRequest(requestID)

	// 构建发送到 Figma 插件的消息
	params := map[string]interface{}{
		"nodeId":      nodeID,
		"format":      strings.ToUpper(imageFormat), // PNG, JPG, SVG
		"scale":       imageScale,
		"ignore_text": ignoreTexts, // 是否忽略文字
	}

	// 添加 ignore_nodes 参数（如果提供）
	if len(ignoreNodes) > 0 {
		params["ignore_nodes"] = ignoreNodes
	}

	message := map[string]interface{}{
		"id":      requestID,
		"command": "export_node_as_image",
		"params":  params,
	}

	// 获取当前用户的频道
	channel := mcpService.GetUserChannel(connection.ConnectionID)
	if channel == "" {
		// 如果没有加入频道，默认使用 figma-bridge
		channel = "figma-bridge"
		// 自动加入默认频道
		_ = mcpService.JoinChannel(connection.ConnectionID, channel)
	}

	// 广播消息到频道
	err := mcpService.BroadcastToChannel(connection.ConnectionID, channel, message)
	if err != nil {
		return "", fmt.Errorf("发送消息到频道失败: %v", err)
	}

	fmt.Printf("✅ [TryGetImageViaWebSocket] 已发送请求到频道 %s，等待响应...\n", channel)

	// 等待响应（30秒超时）
	var response interface{}
	select {
	case response = <-responseChan:
		fmt.Printf("✅ [tryGetImageViaWebSocket] 收到响应 (ID=%s)\n", requestID)
	case respErr := <-errorChan:
		return "", fmt.Errorf("收到错误响应: %v", respErr)
	case <-time.After(30 * time.Second):
		return "", fmt.Errorf("请求超时，未在 30 秒内收到插件响应")
	}

	// 解析响应数据
	responseData, ok := response.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("响应数据格式错误")
	}

	// 提取 base64 图片数据
	imageDataB64, ok := responseData["imageData"].(string)
	if !ok || imageDataB64 == "" {
		return "", fmt.Errorf("响应中没有图片数据")
	}

	fmt.Printf("✅ [TryGetImageViaWebSocket] 收到 base64 图片数据，大小: %d bytes\n", len(imageDataB64))

	// 调用处理函数保存图片
	return handleImageDataFromPlugin(fileKey, nodeID, imageFormat, imageScale, imageDataB64, responseData, tempDir, safeNodeID, ignoreTexts, ignoreNodes)
}

// handleImageDataFromPlugin 处理从 Figma 插件接收到的图片数据
func handleImageDataFromPlugin(fileKey, nodeID, imageFormat string, imageScale float64, imageDataB64 string, responseData map[string]interface{}, tempDir, safeNodeID string, ignoreTexts bool, ignoreNodes []string) (string, error) {
	fmt.Printf("🎨 [handleImageDataFromPlugin] 开始处理插件图片: fileKey=%s, nodeID=%s, format=%s, scale=%.1f, ignoreTexts=%v, ignoreNodes=%v\n",
		fileKey, nodeID, imageFormat, imageScale, ignoreTexts, ignoreNodes)

	// 1. 解码 base64 图片数据
	decodedBytes, err := base64.StdEncoding.DecodeString(imageDataB64)
	if err != nil {
		return "", fmt.Errorf("解码 base64 失败: %v", err)
	}

	fmt.Printf("✅ [handleImageDataFromPlugin] 图片解码成功，大小: %d bytes\n", len(decodedBytes))

	// 2. 确定 MIME 类型和文件扩展名
	mimeType := "image/png"
	if mt, ok := responseData["mimeType"].(string); ok && mt != "" {
		mimeType = mt
	}

	fileExt := "." + imageFormat
	if imageFormat == "" {
		fileExt = ".png"
	}

	// 3. 保存到本地缓存
	scaleStr := formatScaleForFilename(imageScale)

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
		ignoreNodesHash := fmt.Sprintf("%x", hash)[:8] // 使用前8位哈希值
		suffix = suffix + "i" + ignoreNodesHash
	}

	// 使用固定的文件名格式，如果文件已存在则直接替换
	fileName := fmt.Sprintf("%s-%s%s%s", safeNodeID, scaleStr, suffix, fileExt)
	localPath := filepath.Join(tempDir, fileName)

	// 写入文件
	err = os.WriteFile(localPath, decodedBytes, 0644)
	if err != nil {
		return "", fmt.Errorf("写入本地文件失败: %v", err)
	}

	fmt.Printf("✅ [handleImageDataFromPlugin] 图片已保存到本地: %s\n", localPath)

	// 4. 异步上传到华为云 OBS
	go func() {
		if globalOBSService == nil || !globalOBSService.IsEnabled() {
			fmt.Printf("⚠️ [handleImageDataFromPlugin] OBS 服务未启用，跳过上传\n")
			return
		}

		fmt.Printf("📤 [handleImageDataFromPlugin] 开始上传到 OBS\n")
		obsURL, obsKey, fileSize, err := globalOBSService.UploadImageFromBytes(decodedBytes, fileKey, nodeID, imageFormat, imageScale, mimeType)
		if err != nil {
			fmt.Printf("❌ [handleImageDataFromPlugin] 上传到 OBS 失败: %v\n", err)
			return
		}

		fmt.Printf("✅ [handleImageDataFromPlugin] 成功上传到 OBS: %s (URL: %s)\n", obsKey, obsURL)

		// 5. 保存或更新数据库记录
		now := uint32(time.Now().Unix())
		expiresAt := uint32(time.Now().AddDate(0, 0, 7).Unix()) // 7天过期

		existingImage, err := models.GetNodeImage(fileKey, nodeID, imageFormat, imageScale, ignoreTexts)
		if err == nil && existingImage != nil {
			// 更新已有记录
			existingImage.OBSKey = obsKey
			existingImage.UpdatedAt = now
			existingImage.OBSExpiresAt = expiresAt
			existingImage.FileSize = uint64(fileSize)
			existingImage.Status = "obs_synced"
			if err := models.UpdateNodeImage(existingImage); err != nil {
				fmt.Printf("❌ [handleImageDataFromPlugin] 更新数据库失败: %v\n", err)
			} else {
				fmt.Printf("✅ [handleImageDataFromPlugin] 已更新数据库记录\n")
			}
		} else {
			// 创建新记录
			newImage := &models.FigmaNodeImage{
				FileKey:      fileKey,
				NodeID:       nodeID,
				Format:       imageFormat,
				Scale:        imageScale,
				OBSKey:       obsKey,
				FigmaCDNURL:  "",
				FileSize:     uint64(fileSize),
				Status:       "obs_synced",
				CreatedAt:    now,
				UpdatedAt:    now,
				OBSExpiresAt: expiresAt,
			}
			if err := models.CreateNodeImage(newImage); err != nil {
				fmt.Printf("❌ [handleImageDataFromPlugin] 创建数据库记录失败: %v\n", err)
			} else {
				fmt.Printf("✅ [handleImageDataFromPlugin] 已创建数据库记录\n")
			}
		}
	}()

	return localPath, nil
}

// getFallbackImage 获取裂图路径
func getFallbackImage() (string, error) {
	fallbackPath := filepath.Join("web", "static", "img", "break.jpg")
	if _, err := os.Stat(fallbackPath); os.IsNotExist(err) {
		return "", fmt.Errorf("裂图文件不存在: %s", fallbackPath)
	}
	fmt.Printf("🖼️ 返回裂图: %s\n", fallbackPath)
	return fallbackPath, nil
}

// downloadImageFromOBS 从 OBS 下载图片到本地缓存
func downloadImageFromOBS(obsKey, tempDir, safeNodeID, imageFormat string, imageScale float64) (string, error) {
	return DownloadImageFromOBS(obsKey, tempDir, safeNodeID, imageFormat, imageScale, false)
}

// DownloadImageFromOBS 从 OBS 下载图片到本地缓存（公共函数，支持ignoreTexts参数）
func DownloadImageFromOBS(obsKey, tempDir, safeNodeID, imageFormat string, imageScale float64, ignoreTexts bool) (string, error) {
	// 检查 OBS 服务是否可用
	if globalOBSService == nil || !globalOBSService.IsEnabled() {
		return "", fmt.Errorf("OBS 服务未启用")
	}

	fmt.Printf("📥 [OBS] 开始下载图片: %s\n", obsKey)

	// 从 OBS 下载图片数据
	imageData, contentType, err := globalOBSService.DownloadImage(obsKey)
	if err != nil {
		return "", fmt.Errorf("从 OBS 下载失败: %w", err)
	}

	fmt.Printf("✅ [OBS] 图片下载成功，大小: %d bytes, Content-Type: %s\n", len(imageData), contentType)

	// 确定文件扩展名
	fileExt := "." + imageFormat
	if imageFormat == "" {
		fileExt = ".png"
	}

	// 生成本地文件路径，根据ignoreTexts决定文件名后缀
	scaleStr := formatScaleForFilename(imageScale)
	suffix := "x" // 默认带文字
	if ignoreTexts {
		suffix = "p" // 不带文字
	}

	// 使用固定的文件名格式，如果文件已存在则直接替换
	fileName := fmt.Sprintf("%s-%s%s%s", safeNodeID, scaleStr, suffix, fileExt)
	localPath := filepath.Join(tempDir, fileName)

	// 写入本地文件
	err = os.WriteFile(localPath, imageData, 0644)
	if err != nil {
		return "", fmt.Errorf("写入本地文件失败: %w", err)
	}

	fmt.Printf("✅ [OBS] 图片已保存到本地: %s\n", localPath)
	return localPath, nil
}

// downloadImageFromURL 从指定 URL 下载图片到本地缓存
func downloadImageFromURL(imageURL, tempDir, safeNodeID, imageFormat string, imageScale float64) (string, error) {
	fmt.Printf("开始从 URL 下载图片: %s\n", imageURL)

	// 创建带超时的HTTP客户端
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// 创建请求
	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建下载请求失败: %v", err)
	}

	// 添加请求头
	req.Header.Set("User-Agent", "Mozilla/5.0 FigmaDeliver/1.0")

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载失败，状态码: %d", resp.StatusCode)
	}

	// 确定文件扩展名
	fileExt := "." + imageFormat
	if imageFormat == "" {
		fileExt = ".png"
	}

	// 生成文件路径
	scaleStr := formatScaleForFilename(imageScale)
	fileName := fmt.Sprintf("%s-%s%s", safeNodeID, scaleStr, fileExt)
	filePath := filepath.Join(tempDir, fileName)

	// 创建临时文件用于下载
	downloadTempPath := filePath + ".tmp"
	file, err := os.Create(downloadTempPath)
	if err != nil {
		return "", fmt.Errorf("创建临时文件失败: %v", err)
	}

	// 直接写入文件
	bytes, err := io.Copy(file, resp.Body)
	if err != nil {
		file.Close()
		os.Remove(downloadTempPath)
		return "", fmt.Errorf("写入临时文件失败: %v", err)
	}

	// 关闭文件
	if err := file.Close(); err != nil {
		os.Remove(downloadTempPath)
		return "", fmt.Errorf("关闭临时文件失败: %v", err)
	}

	// 检查文件大小
	if bytes == 0 {
		os.Remove(downloadTempPath)
		return "", errors.New("下载的图片大小为0字节")
	}

	// 移动到最终位置（如果目标文件已存在则替换）
	if err := os.Rename(downloadTempPath, filePath); err != nil {
		os.Remove(downloadTempPath)
		return "", fmt.Errorf("重命名临时文件失败: %v", err)
	}

	fmt.Printf("图片下载完成: %s (%d 字节)\n", filePath, bytes)
	return filePath, nil
}

// 查找节点的最新图片文件（根据缩放等级）
func findLatestNodeImage(tempDir, safeNodeID string, imageScale float64) string {
	// 查找同一节点和缩放等级的所有图片文件（支持多种格式，不包括临时文件）
	fmt.Printf("查找缓存图片: tempDir=%s, safeNodeID=%s, scale=%.1f\n", tempDir, safeNodeID, imageScale)

	// 构建缩放等级字符串（用于匹配文件名）
	scaleStr := formatScaleForFilename(imageScale)

	supportedExts := []string{".png", ".svg", ".jpg"}
	var allFiles []string

	for _, ext := range supportedExts {
		// 匹配格式：节点-缩放等级-*.*
		pattern := filepath.Join(tempDir, safeNodeID+"-"+scaleStr+"-*"+ext)
		fmt.Printf("查找模式: %s\n", pattern)
		files, err := filepath.Glob(pattern)
		if err != nil {
			fmt.Printf("Glob查找出错: %v\n", err)
		} else {
			fmt.Printf("找到 %d 个文件匹配模式 %s\n", len(files), pattern)
			if len(files) > 0 {
				allFiles = append(allFiles, files...)
			}
		}
	}

	if len(allFiles) == 0 {
		fmt.Printf("未找到任何匹配的缓存文件\n")
		return ""
	}
	fmt.Printf("总共找到 %d 个文件\n", len(allFiles))

	// 过滤掉临时文件
	var validFiles []string
	for _, f := range allFiles {
		// 检查是否是临时文件（包含-temp.扩展名）
		if !strings.Contains(f, "-temp.") {
			validFiles = append(validFiles, f)
			fmt.Printf("有效缓存文件: %s\n", f)
		} else {
			fmt.Printf("跳过临时文件: %s\n", f)
		}
	}

	if len(validFiles) == 0 {
		fmt.Printf("过滤后没有有效的缓存文件\n")
		return ""
	}

	// 按修改时间排序
	type fileInfo struct {
		path    string
		modTime time.Time
	}
	fileInfos := make([]fileInfo, 0, len(validFiles))

	for _, f := range validFiles {
		if info, err := os.Stat(f); err == nil {
			fileInfos = append(fileInfos, fileInfo{path: f, modTime: info.ModTime()})
		}
	}

	// 按修改时间降序排序
	sort.Slice(fileInfos, func(i, j int) bool {
		return fileInfos[i].modTime.After(fileInfos[j].modTime)
	})

	// 返回最新的文件
	if len(fileInfos) > 0 {
		fmt.Printf("返回最新的缓存文件: %s (修改时间: %v)\n", fileInfos[0].path, fileInfos[0].modTime)
		return fileInfos[0].path
	}

	fmt.Printf("没有有效的文件信息\n")
	return ""
}

// 清理节点的旧图片文件（根据缩放等级）
func cleanupOldNodeImages(tempDir, safeNodeID string, imageScale float64, keepCount int) {
	// 构建缩放等级字符串
	scaleStr := formatScaleForFilename(imageScale)

	// 查找同一节点和缩放等级的所有图片文件（支持多种格式，不包括临时文件）
	supportedExts := []string{".png", ".svg", ".jpg"}
	var allFiles []string

	for _, ext := range supportedExts {
		// 匹配格式：节点-缩放等级-*.*
		pattern := filepath.Join(tempDir, safeNodeID+"-"+scaleStr+"-*"+ext)
		files, err := filepath.Glob(pattern)
		if err == nil && len(files) > 0 {
			allFiles = append(allFiles, files...)
		}
	}

	if len(allFiles) == 0 {
		return
	}

	// 过滤掉临时文件
	var validFiles []string
	for _, f := range allFiles {
		// 检查是否是临时文件（包含-temp.扩展名）
		if !strings.Contains(f, "-temp.") {
			validFiles = append(validFiles, f)
		}
	}

	if len(validFiles) <= keepCount {
		return
	}

	fmt.Printf("找到节点的 %d 个图片文件，将保留最新的 %d 个\n", len(validFiles), keepCount)

	// 按修改时间排序
	type fileInfo struct {
		path    string
		modTime time.Time
	}
	fileInfos := make([]fileInfo, 0, len(validFiles))

	for _, f := range validFiles {
		if info, err := os.Stat(f); err == nil {
			fileInfos = append(fileInfos, fileInfo{path: f, modTime: info.ModTime()})
		}
	}

	// 按修改时间降序排序
	sort.Slice(fileInfos, func(i, j int) bool {
		return fileInfos[i].modTime.After(fileInfos[j].modTime)
	})

	// 删除旧文件
	for i := keepCount; i < len(fileInfos); i++ {
		fmt.Printf("删除旧图片文件: %s\n", fileInfos[i].path)
		os.Remove(fileInfos[i].path)
	}
}

// 清除项目的所有图片缓存
func ClearProjectImageCache(fileKey string) error {
	return ClearProjectImageCacheByNodeIDs(fileKey, nil)
}

// ClearProjectExportCache 清除项目的导出缓存
func ClearProjectExportCache(projectID uint) (int, error) {
	exportDir := filepath.Join("temp", "exports", fmt.Sprintf("%d", projectID))

	fmt.Printf("清除项目 %d 的导出缓存目录: %s\n", projectID, exportDir)

	// 检查目录是否存在
	if _, err := os.Stat(exportDir); os.IsNotExist(err) {
		fmt.Printf("导出目录不存在，跳过: %s\n", exportDir)
		return 0, nil
	}

	// 计算要删除的文件数量
	var fileCount int
	err := filepath.Walk(exportDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			fileCount++
		}
		return nil
	})

	if err != nil {
		return 0, fmt.Errorf("统计导出文件失败: %v", err)
	}

	// 删除整个导出目录
	err = os.RemoveAll(exportDir)
	if err != nil {
		return 0, fmt.Errorf("删除导出目录失败: %v", err)
	}

	fmt.Printf("成功删除项目 %d 的导出目录，清除了 %d 个文件\n", projectID, fileCount)
	return fileCount, nil
}

// ClearProjectImageCacheByNodeIDs 根据节点ID列表清除项目的图片缓存
// 如果nodeIDs为nil或空，则清除所有缓存文件
func ClearProjectImageCacheByNodeIDs(fileKey string, nodeIDs []string) error {
	// 需要清理的目录列表
	cacheDirs := []string{
		filepath.Join("temp", fileKey, "images"),
		filepath.Join("temp", fileKey, "previews"),
		filepath.Join("temp", fileKey, "fpreviews"),
	}

	totalFiles := 0
	var allErrors []string

	// 如果没有指定节点ID，则清除所有缓存文件（原有行为）
	if nodeIDs == nil || len(nodeIDs) == 0 {
		fmt.Printf("清除项目 %s 的所有缓存文件\n", fileKey)

		for _, tempDir := range cacheDirs {
			fmt.Printf("检查缓存目录: %s\n", tempDir)

			// 检查目录是否存在
			if _, err := os.Stat(tempDir); os.IsNotExist(err) {
				fmt.Printf("目录不存在，跳过: %s\n", tempDir)
				continue
			}

			// 删除所有图片文件（支持多种格式）
			supportedExts := []string{"*.png", "*.jpg", "*.jpeg", "*.svg"}
			for _, ext := range supportedExts {
				pattern := filepath.Join(tempDir, ext)
				files, err := filepath.Glob(pattern)
				if err != nil {
					errMsg := fmt.Sprintf("查找缓存文件失败 %s: %v", tempDir, err)
					fmt.Println(errMsg)
					allErrors = append(allErrors, errMsg)
					continue
				}

				// 删除找到的所有文件
				for _, f := range files {
					fmt.Printf("删除缓存图片文件: %s\n", f)
					if err := os.Remove(f); err != nil {
						errMsg := fmt.Sprintf("删除文件失败: %s, 错误: %v", f, err)
						fmt.Println(errMsg)
						allErrors = append(allErrors, errMsg)
					} else {
						totalFiles++
					}
				}
			}

			// 同时清理所有 .tmp 临时文件
			tmpPattern := filepath.Join(tempDir, "*.tmp")
			tmpFiles, err := filepath.Glob(tmpPattern)
			if err != nil {
				errMsg := fmt.Sprintf("查找临时文件失败 %s: %v", tempDir, err)
				fmt.Println(errMsg)
				allErrors = append(allErrors, errMsg)
			} else {
				for _, tmpFile := range tmpFiles {
					fmt.Printf("删除临时文件: %s\n", tmpFile)
					if err := os.Remove(tmpFile); err != nil {
						errMsg := fmt.Sprintf("删除临时文件失败: %s, 错误: %v", tmpFile, err)
						fmt.Println(errMsg)
						allErrors = append(allErrors, errMsg)
					} else {
						totalFiles++
					}
				}
			}

			fmt.Printf("已清除目录 %s 中的文件\n", tempDir)
		}
	} else {
		// 按节点ID清除特定缓存文件
		fmt.Printf("清除项目 %s 中 %d 个节点的缓存文件\n", fileKey, len(nodeIDs))

		// 将节点ID转换为安全的文件名前缀
		safeNodeIDs := make([]string, len(nodeIDs))
		for i, nodeID := range nodeIDs {
			safeNodeIDs[i] = strings.NewReplacer(
				":", "_", ";", "_", "/", "_", "\\", "_",
				"*", "_", "?", "_", "\"", "_", "<", "_",
				">", "_", "|", "_", ",", "_",
			).Replace(nodeID)
		}

		for _, tempDir := range cacheDirs {
			fmt.Printf("检查缓存目录: %s\n", tempDir)

			// 检查目录是否存在
			if _, err := os.Stat(tempDir); os.IsNotExist(err) {
				fmt.Printf("目录不存在，跳过: %s\n", tempDir)
				continue
			}

			dirCleared := 0

			// 为每个节点ID查找并删除对应的缓存文件
			for i, safeNodeID := range safeNodeIDs {
				originalNodeID := nodeIDs[i]

				// 支持多种图片格式
				supportedExts := []string{".png", ".jpg", ".jpeg", ".svg"}

				for _, ext := range supportedExts {
					// 查找以节点ID为前缀的文件
					pattern := filepath.Join(tempDir, safeNodeID+"-*"+ext)
					files, err := filepath.Glob(pattern)
					if err != nil {
						errMsg := fmt.Sprintf("查找节点 %s 的缓存文件失败: %v", originalNodeID, err)
						fmt.Println(errMsg)
						allErrors = append(allErrors, errMsg)
						continue
					}

					// 删除找到的文件
					for _, file := range files {
						fmt.Printf("删除节点 %s 的缓存文件: %s\n", originalNodeID, file)
						if err := os.Remove(file); err != nil {
							errMsg := fmt.Sprintf("删除文件失败: %s, 错误: %v", file, err)
							fmt.Println(errMsg)
							allErrors = append(allErrors, errMsg)
						} else {
							dirCleared++
							totalFiles++
						}
					}
				}
			}

			// 无论是否按节点ID清除，都要清理所有 .tmp 临时文件
			tmpPattern := filepath.Join(tempDir, "*.tmp")
			tmpFiles, err := filepath.Glob(tmpPattern)
			if err != nil {
				errMsg := fmt.Sprintf("查找临时文件失败 %s: %v", tempDir, err)
				fmt.Println(errMsg)
				allErrors = append(allErrors, errMsg)
			} else {
				for _, tmpFile := range tmpFiles {
					fmt.Printf("删除临时文件: %s\n", tmpFile)
					if err := os.Remove(tmpFile); err != nil {
						errMsg := fmt.Sprintf("删除临时文件失败: %s, 错误: %v", tmpFile, err)
						fmt.Println(errMsg)
						allErrors = append(allErrors, errMsg)
					} else {
						dirCleared++
						totalFiles++
					}
				}
			}

			fmt.Printf("目录 %s 中清除了 %d 个文件\n", tempDir, dirCleared)
		}
	}

	fmt.Printf("已清除项目 %s 的缓存图片，共 %d 个文件\n", fileKey, totalFiles)

	// 如果有错误，返回合并的错误信息
	if len(allErrors) > 0 {
		return fmt.Errorf("清理过程中发生错误: %s", strings.Join(allErrors, "; "))
	}

	return nil
}

// downloadPreviewImageFile 下载图片文件
func downloadPreviewImageFile(url, fileKey, nodeID string, imageScale float64) (string, error) {
	fmt.Printf("第二步：开始下载图片文件: url=%s\n", url)

	// 创建临时文件夹
	tempDir := filepath.Join("temp", fileKey, "previews")
	fmt.Printf("临时文件夹路径: %s\n", tempDir)
	err := os.MkdirAll(tempDir, os.ModePerm)
	if err != nil {
		fmt.Printf("创建临时文件夹失败: %v\n", err)
		return "", fmt.Errorf("创建临时文件夹失败: %v", err)
	}

	// 生成文件名 - 替换特殊字符为下划线
	safeNodeID := strings.NewReplacer(":", "_", ";", "_", "/", "_", "\\", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_").Replace(nodeID)

	// 清理同一节点和缩放等级的过多旧文件
	cleanupOldNodeImages(tempDir, safeNodeID, imageScale, 10)

	// 下载图片
	fmt.Printf("开始下载图片: %s\n", url)

	// 创建带超时的HTTP客户端
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// 创建请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("创建图片下载请求失败: %v\n", err)
		return "", fmt.Errorf("创建图片下载请求失败: %v", err)
	}

	// 添加请求头
	req.Header.Set("User-Agent", "Mozilla/5.0 FigmaDeliver/1.0")

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("下载图片请求失败: %v\n", err)
		return "", fmt.Errorf("下载图片请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	fmt.Printf("图片下载响应状态码: %d\n", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("下载图片失败: %s", resp.Status)
		fmt.Println(errMsg)
		return "", fmt.Errorf(errMsg)
	}

	// 检查Content-Type
	contentType := resp.Header.Get("Content-Type")
	fmt.Printf("图片Content-Type: %s\n", contentType)
	if !strings.HasPrefix(contentType, "image/") {
		fmt.Printf("警告：响应Content-Type不是图片类型: %s\n", contentType)
	}

	// 生成文件名
	scaleStr := formatScaleForFilename(imageScale)
	fileName := fmt.Sprintf("%s-%s.png", safeNodeID, scaleStr)
	filePath := filepath.Join(tempDir, fileName)

	// 创建临时文件，避免直接写入目标文件可能导致的损坏
	downloadTempPath := filePath + ".tmp"
	fmt.Printf("创建临时文件: %s\n", downloadTempPath)
	file, err := os.Create(downloadTempPath)
	if err != nil {
		fmt.Printf("创建临时文件失败: %v\n", err)
		return "", fmt.Errorf("创建临时文件失败: %v", err)
	}

	// 将响应内容写入临时文件
	fmt.Printf("将响应内容写入临时文件...\n")
	bytes, err := io.Copy(file, resp.Body)
	if err != nil {
		file.Close()
		os.Remove(downloadTempPath) // 清理临时文件
		fmt.Printf("写入临时文件失败: %v\n", err)
		return "", fmt.Errorf("写入临时文件失败: %v", err)
	}

	// 关闭文件
	if err := file.Close(); err != nil {
		os.Remove(downloadTempPath) // 清理临时文件
		fmt.Printf("关闭临时文件失败: %v\n", err)
		return "", fmt.Errorf("关闭临时文件失败: %v", err)
	}

	// 检查文件大小
	if bytes == 0 {
		os.Remove(downloadTempPath) // 清理临时文件
		fmt.Printf("错误：下载的图片大小为0字节\n")
		return "", errors.New("下载的图片大小为0字节")
	}

	fmt.Printf("成功写入 %d 字节到临时文件\n", bytes)

	// 将临时文件重命名为最终文件（如果目标文件已存在则替换）
	if err := os.Rename(downloadTempPath, filePath); err != nil {
		os.Remove(downloadTempPath) // 清理临时文件
		fmt.Printf("重命名临时文件失败: %v\n", err)
		return "", fmt.Errorf("重命名临时文件失败: %v", err)
	}

	fmt.Printf("图片下载完成: %s (%d 字节)\n", filePath, bytes)
	return filePath, nil
}

// generateNodeIDsHash 基于节点ID列表生成哈希值
func generateNodeIDsHash(nodeIDs []string) string {
	// 对节点ID列表进行排序，确保相同的节点集合产生相同的哈希
	sortedNodeIDs := make([]string, len(nodeIDs))
	copy(sortedNodeIDs, nodeIDs)
	sort.Strings(sortedNodeIDs)

	// 将排序后的节点ID连接成字符串
	nodeIDsStr := strings.Join(sortedNodeIDs, ",")

	// 计算MD5哈希值并取前8位
	hasher := md5.New()
	hasher.Write([]byte(nodeIDsStr))
	hash := hex.EncodeToString(hasher.Sum(nil))[0:8]

	fmt.Printf("节点ID列表哈希: %s (基于 %d 个节点)\n", hash, len(nodeIDs))
	return hash
}

// downloadFilteredPreviewImageFileByNodeIDs 基于节点ID列表下载过滤后的预览图片文件
func downloadFilteredPreviewImageFileByNodeIDs(url, fileKey, safeFileName string, includedNodeIDs []string, imageScale float64) (string, error) {
	fmt.Printf("开始下载过滤后的预览图片文件: url=%s, 包含 %d 个节点\n", url, len(includedNodeIDs))

	// 创建临时文件夹
	tempDir := filepath.Join("temp", fileKey, "fpreviews")
	fmt.Printf("临时文件夹路径: %s\n", tempDir)
	err := os.MkdirAll(tempDir, os.ModePerm)
	if err != nil {
		fmt.Printf("创建临时文件夹失败: %v\n", err)
		return "", fmt.Errorf("创建临时文件夹失败: %v", err)
	}

	// 清理同一类型和缩放等级的过多旧文件
	cleanupOldNodeImages(tempDir, safeFileName, imageScale, 10)

	// 下载图片
	fmt.Printf("开始下载过滤后的预览图片: %s\n", url)

	// 创建带超时的HTTP客户端
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// 创建请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("创建图片下载请求失败: %v\n", err)
		return "", fmt.Errorf("创建图片下载请求失败: %v", err)
	}

	// 添加请求头
	req.Header.Set("User-Agent", "Mozilla/5.0 FigmaDeliver/1.0")

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("下载图片请求失败: %v\n", err)
		return "", fmt.Errorf("下载图片请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	fmt.Printf("图片下载响应状态码: %d\n", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("下载图片失败: %s", resp.Status)
		fmt.Println(errMsg)
		return "", fmt.Errorf(errMsg)
	}

	// 检查Content-Type
	contentType := resp.Header.Get("Content-Type")
	fmt.Printf("图片Content-Type: %s\n", contentType)
	if !strings.HasPrefix(contentType, "image/") {
		fmt.Printf("警告：响应Content-Type不是图片类型: %s\n", contentType)
	}

	// 根据Content-Type确定文件扩展名
	fileExt := ".png" // 默认为png
	if strings.HasPrefix(contentType, "image/") {
		switch contentType {
		case "image/svg+xml":
			fileExt = ".svg"
		case "image/jpeg":
			fileExt = ".jpg"
		case "image/png":
			fileExt = ".png"
		}
	}

	// 构建文件名（包含缩放等级）
	scaleStr := formatScaleForFilename(imageScale)
	fileName := fmt.Sprintf("%s-%s%s", safeFileName, scaleStr, fileExt)
	filePath := filepath.Join(tempDir, fileName)
	fmt.Printf("文件路径: %s (Content-Type: %s)\n", filePath, contentType)

	// 创建临时文件，避免直接写入目标文件可能导致的损坏
	downloadTempPath := filePath + ".tmp"
	fmt.Printf("创建临时文件: %s\n", downloadTempPath)
	file, err := os.Create(downloadTempPath)
	if err != nil {
		fmt.Printf("创建临时文件失败: %v\n", err)
		return "", fmt.Errorf("创建临时文件失败: %v", err)
	}

	// 将响应内容写入临时文件
	fmt.Printf("将响应内容写入临时文件...\n")
	bytes, err := io.Copy(file, resp.Body)
	if err != nil {
		file.Close()
		os.Remove(downloadTempPath) // 清理临时文件
		fmt.Printf("写入临时文件失败: %v\n", err)
		return "", fmt.Errorf("写入临时文件失败: %v", err)
	}

	// 关闭文件
	if err := file.Close(); err != nil {
		os.Remove(downloadTempPath) // 清理临时文件
		fmt.Printf("关闭临时文件失败: %v\n", err)
		return "", fmt.Errorf("关闭临时文件失败: %v", err)
	}

	// 检查文件大小
	if bytes == 0 {
		os.Remove(downloadTempPath) // 清理临时文件
		fmt.Printf("错误：下载的图片大小为0字节\n")
		return "", errors.New("下载的图片大小为0字节")
	}

	fmt.Printf("成功写入 %d 字节到临时文件\n", bytes)

	// 将临时文件重命名为最终文件（如果目标文件已存在则替换）
	if err := os.Rename(downloadTempPath, filePath); err != nil {
		os.Remove(downloadTempPath) // 清理临时文件
		fmt.Printf("重命名临时文件失败: %v\n", err)
		return "", fmt.Errorf("重命名临时文件失败: %v", err)
	}

	fmt.Printf("过滤后的预览图片下载完成: %s (%d 字节)\n", filePath, bytes)
	return filePath, nil
}

// generateNodeFileName 为节点生成文件名
func generateNodeFileName(nodeID string, nodeInfo map[string]interface{}, nodeModifys map[string]interface{}) string {
	var nodeName string

	// 优先使用修改信息中的rename
	if nodeModifys != nil {
		if rename, ok := nodeModifys["rename"].(string); ok && rename != "" {
			nodeName = rename
		}
	}

	// 如果没有rename，使用节点原始名称
	if nodeName == "" && nodeInfo != nil {
		if name, ok := nodeInfo["name"].(string); ok {
			nodeName = name
		}
	}

	// 如果都没有，使用nodeID作为后备
	if nodeName == "" {
		nodeName = nodeID
	}

	// 移除特殊字符
	safeNodeName := strings.NewReplacer(":", "_", ";", "_", "/", "_", "\\", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_", " ", "_").Replace(nodeName)

	return safeNodeName
}

// downloadSingleFilteredPreviewImage 下载单个过滤预览图片
func downloadSingleFilteredPreviewImage(imageURL, _ /*fileKey*/, nodeID, fileName, tempDir string, imageScale float64) (string, error) {
	fmt.Printf("开始下载单个图片: nodeID=%s, fileName=%s, url=%s, scale=%.1f\n", nodeID, fileName, imageURL, imageScale)

	// 检查是否已经有缓存的图片文件
	existingFile := findLatestNodeImage(tempDir, fileName, imageScale)
	if existingFile != "" {
		fmt.Printf("找到节点 %s 的缓存图片文件，直接使用: %s\n", nodeID, existingFile)
		return existingFile, nil
	}

	// 创建带超时的HTTP客户端
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// 创建请求
	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		fmt.Printf("创建图片下载请求失败: %v\n", err)
		return "", fmt.Errorf("创建图片下载请求失败: %v", err)
	}

	// 添加请求头
	req.Header.Set("User-Agent", "Mozilla/5.0 FigmaDeliver/1.0")

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("下载图片请求失败: %v\n", err)
		return "", fmt.Errorf("下载图片请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	fmt.Printf("图片下载响应状态码: %d\n", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("下载图片失败: %s", resp.Status)
		fmt.Println(errMsg)
		return "", fmt.Errorf(errMsg)
	}

	// 检查Content-Type
	contentType := resp.Header.Get("Content-Type")
	fmt.Printf("图片Content-Type: %s\n", contentType)
	if !strings.HasPrefix(contentType, "image/") {
		fmt.Printf("警告：响应Content-Type不是图片类型: %s\n", contentType)
	}

	// 根据Content-Type确定文件扩展名
	fileExt := ".png" // 默认为png
	if strings.HasPrefix(contentType, "image/") {
		switch contentType {
		case "image/svg+xml":
			fileExt = ".svg"
		case "image/jpeg":
			fileExt = ".jpg"
		case "image/png":
			fileExt = ".png"
		}
	}

	// 构建文件名（包含缩放等级）
	scaleStr := formatScaleForFilename(imageScale)
	finalFileName := fmt.Sprintf("%s-%s%s", fileName, scaleStr, fileExt)
	filePath := filepath.Join(tempDir, finalFileName)
	fmt.Printf("文件路径: %s (Content-Type: %s)\n", filePath, contentType)

	// 创建临时文件，避免直接写入目标文件可能导致的损坏
	downloadTempPath := filePath + ".tmp"
	fmt.Printf("创建临时文件: %s\n", downloadTempPath)
	file, err := os.Create(downloadTempPath)
	if err != nil {
		fmt.Printf("创建临时文件失败: %v\n", err)
		return "", fmt.Errorf("创建临时文件失败: %v", err)
	}

	// 将响应内容写入临时文件
	fmt.Printf("将响应内容写入临时文件...\n")
	bytes, err := io.Copy(file, resp.Body)
	if err != nil {
		file.Close()
		os.Remove(downloadTempPath) // 清理临时文件
		fmt.Printf("写入临时文件失败: %v\n", err)
		return "", fmt.Errorf("写入临时文件失败: %v", err)
	}

	// 关闭文件
	if err := file.Close(); err != nil {
		os.Remove(downloadTempPath) // 清理临时文件
		fmt.Printf("关闭临时文件失败: %v\n", err)
		return "", fmt.Errorf("关闭临时文件失败: %v", err)
	}

	// 检查文件大小
	if bytes == 0 {
		os.Remove(downloadTempPath) // 清理临时文件
		fmt.Printf("错误：下载的图片大小为0字节\n")
		return "", errors.New("下载的图片大小为0字节")
	}

	fmt.Printf("成功写入 %d 字节到临时文件\n", bytes)

	// 将临时文件重命名为最终文件（如果目标文件已存在则替换）
	if err := os.Rename(downloadTempPath, filePath); err != nil {
		os.Remove(downloadTempPath) // 清理临时文件
		fmt.Printf("重命名临时文件失败: %v\n", err)
		return "", fmt.Errorf("重命名临时文件失败: %v", err)
	}

	// 清理同一节点和缩放等级的过多旧文件
	cleanupOldNodeImages(tempDir, fileName, imageScale, 10)

	fmt.Printf("单个图片下载完成: %s (%d 字节)\n", filePath, bytes)
	return filePath, nil
}

// collectIncludedNodeIDs 使用树型遍历收集所有需要包含的节点ID
// 在遍历过程中直接判断节点是否应该被排除，当节点被排除时，跳过其整个子树
func collectIncludedNodeIDs(nodes []map[string]interface{}, rootNodeID string, nodeModifys map[string]map[string]interface{}) []string {
	fmt.Printf("开始树型遍历收集包含的节点ID，根节点: %s\n", rootNodeID)

	// 构建节点ID到节点的映射，方便查找
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

	var includedNodeIDs []string
	visitedNodes := make(map[string]bool) // 记录已访问的节点，避免重复

	// 直接遍历根节点的子节点，不判断根节点自身
	if children, hasChildren := childrenMap[rootNodeID]; hasChildren {
		fmt.Printf("根节点 %s 有 %d 个子节点，直接遍历子节点\n", rootNodeID, len(children))
		for _, childID := range children {
			traverseNodeTree(childID, nodeMap, childrenMap, nodeModifys, visitedNodes, &includedNodeIDs)
		}
	} else {
		fmt.Printf("根节点 %s 没有子节点\n", rootNodeID)
	}

	fmt.Printf("树型遍历完成，收集到 %d 个包含的节点ID\n", len(includedNodeIDs))
	return includedNodeIDs
}

// shouldExcludeNode 判断节点是否应该被排除
func shouldExcludeNode(node map[string]interface{}, modifys map[string]interface{}) bool {
	nodeID := node["id"].(string)

	// 检查visible属性，如果为false则排除
	if visible, hasVisible := node["visible"]; hasVisible {
		if visibleBool, ok := visible.(bool); ok && !visibleBool {
			fmt.Printf("排除节点 %s，因为其visible为false\n", nodeID)
			return true
		}
	}

	// 检查modifys中的排除条件
	if modifys != nil {
		// 检查是否标记为忽略
		if ignore, ok := modifys["ignore"].(bool); ok && ignore {
			fmt.Printf("排除节点 %s，因为其被标记为忽略\n", nodeID)
			return true
		}

		// 检查是否有res_mode属性，且不是"attach"
		if resMode, ok := modifys["res_mode"].(string); ok && resMode != "attach" {
			fmt.Printf("排除节点 %s，因为其资源模式为 %s\n", nodeID, resMode)
			return true
		}
	}

	// 检查TEXT节点的特殊逻辑：如为TEXT节点，且没有modifys或者modifys中的res_mode不是attach，需要排除
	nodeType, _ := node["type"].(string)
	if nodeType == "TEXT" {
		if modifys == nil {
			fmt.Printf("排除节点 %s，因为其类型为TEXT且没有modifys\n", nodeID)
			return true
		}
		if resMode, ok := modifys["res_mode"].(string); !ok || resMode != "attach" {
			fmt.Printf("排除节点 %s，因为其类型为TEXT且modifys.res_mode不是attach\n", nodeID)
			return true
		}
	}

	return false
}

// traverseNodeTree 递归遍历节点树，在遍历过程中直接判断并收集需要包含的节点ID
func traverseNodeTree(nodeID string, nodeMap map[string]map[string]interface{}, childrenMap map[string][]string, nodeModifys map[string]map[string]interface{}, visitedNodes map[string]bool, includedNodeIDs *[]string) {
	// 检查节点是否存在
	node, exists := nodeMap[nodeID]
	if !exists {
		fmt.Printf("节点 %s 不存在，跳过\n", nodeID)
		return
	}

	// 检查节点是否已经被访问过，避免重复处理
	if visitedNodes[nodeID] {
		fmt.Printf("节点 %s 已经被访问过，跳过\n", nodeID)
		return
	}

	// 标记节点为已访问
	visitedNodes[nodeID] = true

	// 在遍历过程中直接判断当前节点是否应该被排除
	shouldExclude := shouldExcludeNode(node, nodeModifys[nodeID])
	if shouldExclude {
		fmt.Printf("节点 %s 被排除，跳过其整个子树\n", nodeID)
		return // 跳过整个子树
	}

	// 当前节点未被排除，添加到包含列表
	*includedNodeIDs = append(*includedNodeIDs, nodeID)
	fmt.Printf("包含节点: %s\n", nodeID)

	// 递归处理子节点
	if children, hasChildren := childrenMap[nodeID]; hasChildren {
		fmt.Printf("节点 %s 有 %d 个子节点，继续遍历\n", nodeID, len(children))
		for _, childID := range children {
			traverseNodeTree(childID, nodeMap, childrenMap, nodeModifys, visitedNodes, includedNodeIDs)
		}
	} else {
		fmt.Printf("节点 %s 没有子节点\n", nodeID)
	}
}

// GetNodeSubtreeIDs 获取指定节点及其子树的所有节点ID
// 如果nodeID为空或等于根节点ID，则返回所有节点ID
// userID: 可选，用于 WebSocket 兜底，为 0 则不尝试
func GetNodeSubtreeIDs(token, fileKey, rootNodeID, targetNodeID string, userID uint) ([]string, error) {
	// 如果目标节点ID为空，使用根节点ID
	if targetNodeID == "" {
		targetNodeID = rootNodeID
	}

	// 获取完整的节点树（支持 WebSocket 兜底）
	nodes, err := GetFigmaNodesWithUser(token, fileKey, rootNodeID, userID)
	if err != nil {
		return nil, fmt.Errorf("获取节点树失败: %v", err)
	}

	// 如果目标节点就是根节点，返回所有节点ID
	if targetNodeID == rootNodeID {
		var allNodeIDs []string
		for _, node := range nodes {
			if nodeID, ok := node["id"].(string); ok {
				allNodeIDs = append(allNodeIDs, nodeID)
			}
		}
		return allNodeIDs, nil
	}

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

	// 检查目标节点是否存在
	if _, exists := nodeMap[targetNodeID]; !exists {
		return nil, fmt.Errorf("目标节点 %s 不存在", targetNodeID)
	}

	// 收集目标节点及其子树的所有节点ID
	var subtreeNodeIDs []string
	collectSubtreeNodeIDs(targetNodeID, childrenMap, &subtreeNodeIDs)

	return subtreeNodeIDs, nil
}

// collectSubtreeNodeIDs 递归收集节点及其子树的所有节点ID
func collectSubtreeNodeIDs(nodeID string, childrenMap map[string][]string, subtreeNodeIDs *[]string) {
	// 添加当前节点ID
	*subtreeNodeIDs = append(*subtreeNodeIDs, nodeID)

	// 递归处理子节点
	if children, hasChildren := childrenMap[nodeID]; hasChildren {
		for _, childID := range children {
			collectSubtreeNodeIDs(childID, childrenMap, subtreeNodeIDs)
		}
	}
}

// CleanupAllTempFiles 清理指定目录下的临时文件（.tmp文件和JSON缓存文件）
func CleanupAllTempFiles(tempDir string) error {
	fmt.Printf("开始清理目录 %s 中的临时文件\n", tempDir)

	// 检查目录是否存在
	if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		fmt.Printf("目录不存在，跳过清理: %s\n", tempDir)
		return nil
	}

	deletedCount := 0
	var errors []string

	// 清理 .tmp 文件（所有目录）
	tmpPattern := filepath.Join(tempDir, "*.tmp")
	tmpFiles, err := filepath.Glob(tmpPattern)
	if err != nil {
		return fmt.Errorf("查找临时文件失败: %v", err)
	}

	// 删除找到的临时文件
	for _, tmpFile := range tmpFiles {
		fmt.Printf("删除临时文件: %s\n", tmpFile)
		if err := os.Remove(tmpFile); err != nil {
			errMsg := fmt.Sprintf("删除临时文件失败: %s, 错误: %v", tmpFile, err)
			fmt.Println(errMsg)
			errors = append(errors, errMsg)
		} else {
			deletedCount++
		}
	}

	// 如果是 documents 目录，额外清理 JSON 缓存文件
	if strings.Contains(tempDir, "documents") {
		jsonPattern := filepath.Join(tempDir, "*.json")
		jsonFiles, err := filepath.Glob(jsonPattern)
		if err != nil {
			return fmt.Errorf("查找JSON缓存文件失败: %v", err)
		}

		// 删除找到的JSON缓存文件
		for _, jsonFile := range jsonFiles {
			fmt.Printf("删除JSON缓存文件: %s\n", jsonFile)
			if err := os.Remove(jsonFile); err != nil {
				errMsg := fmt.Sprintf("删除JSON缓存文件失败: %s, 错误: %v", jsonFile, err)
				fmt.Println(errMsg)
				errors = append(errors, errMsg)
			} else {
				deletedCount++
			}
		}
	}

	fmt.Printf("清理完成，删除了 %d 个文件\n", deletedCount)

	// 如果有错误，返回合并的错误信息
	if len(errors) > 0 {
		return fmt.Errorf("部分文件清理失败: %s", strings.Join(errors, "; "))
	}

	return nil
}

// CleanupProjectTempFiles 清理特定项目的临时文件
func CleanupProjectTempFiles(fileKey, rootNodeID string) error {
	fmt.Printf("开始清理项目 %s (根节点: %s) 的临时文件\n", fileKey, rootNodeID)

	// 需要清理的目录列表
	cacheDirs := []string{
		filepath.Join("temp", fileKey, "previews"),
		filepath.Join("temp", fileKey, "images"),
		filepath.Join("temp", fileKey, "fpreviews"),
		filepath.Join("temp", fileKey, "documents"),
	}

	totalDeleted := 0
	var allErrors []string

	for _, tempDir := range cacheDirs {
		fmt.Printf("检查目录: %s\n", tempDir)

		// 检查目录是否存在
		if _, err := os.Stat(tempDir); os.IsNotExist(err) {
			fmt.Printf("目录不存在，跳过: %s\n", tempDir)
			continue
		}

		// 清理该目录下的临时文件
		if err := CleanupAllTempFiles(tempDir); err != nil {
			errMsg := fmt.Sprintf("清理目录 %s 失败: %v", tempDir, err)
			fmt.Println(errMsg)
			allErrors = append(allErrors, errMsg)
		} else {
			// 统计删除的文件数
			pattern := filepath.Join(tempDir, "*.tmp")
			tmpFiles, _ := filepath.Glob(pattern)
			totalDeleted += len(tmpFiles)
		}
	}

	fmt.Printf("项目 %s 临时文件清理完成，共处理 %d 个文件\n", fileKey, totalDeleted)

	// 如果有错误，返回合并的错误信息
	if len(allErrors) > 0 {
		return fmt.Errorf("部分目录清理失败: %s", strings.Join(allErrors, "; "))
	}

	return nil
}
