package services

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/figma-deliver/internal/models"
)

// HandleRetryAfter 处理 Figma API 响应中的 Retry-After 头
// 如果存在 Retry-After，更新 token 的冷却时间
func HandleRetryAfter(token string, resp *http.Response) {
	if resp == nil {
		return
	}

	retryAfter := resp.Header.Get("Retry-After")
	if retryAfter == "" {
		return
	}

	// Retry-After 可能是秒数或 HTTP 日期
	// 先尝试解析为秒数
	if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds > 0 {
		now := uint32(time.Now().Unix())
		untilTimestamp := now + uint32(seconds)

		err := models.SetCooldownUntil(token, untilTimestamp)
		if err != nil {
			log.Printf("❌ [Retry-After] 设置冷却时间失败: %v", err)
		} else {
			log.Printf("⚠️ [Retry-After] Figma API 要求等待 %d 秒", seconds)
			log.Printf("   - Token: %s...", token[:10])
			log.Printf("   - 冷却至: %s", time.Unix(int64(untilTimestamp), 0).Format("2006-01-02 15:04:05"))
		}
		return
	}

	// 尝试解析为 HTTP 日期格式
	if t, err := http.ParseTime(retryAfter); err == nil {
		untilTimestamp := uint32(t.Unix())
		now := uint32(time.Now().Unix())

		if untilTimestamp > now {
			err := models.SetCooldownUntil(token, untilTimestamp)
			if err != nil {
				log.Printf("❌ [Retry-After] 设置冷却时间失败: %v", err)
			} else {
				waitSeconds := untilTimestamp - now
				log.Printf("⚠️ [Retry-After] Figma API 要求等待到 %s (约 %d 秒)", t.Format("15:04:05"), waitSeconds)
				log.Printf("   - Token: %s...", token[:10])
			}
		}
		return
	}

	log.Printf("⚠️ [Retry-After] 无法解析 Retry-After 头: %s", retryAfter)
}

// CheckAndHandleRateLimitResponse 检查响应状态码并处理速率限制
// 返回 true 表示遇到速率限制，false 表示正常
func CheckAndHandleRateLimitResponse(token string, resp *http.Response) bool {
	if resp == nil {
		return false
	}

	// 429 Too Many Requests
	if resp.StatusCode == 429 {
		log.Printf("🚫 [Rate Limit] Figma API 返回 429 Too Many Requests")
		log.Printf("   - Token: %s...", token[:10])

		// 处理 Retry-After
		HandleRetryAfter(token, resp)

		// 如果没有 Retry-After，使用默认冷却时间（60秒）
		retryAfter := resp.Header.Get("Retry-After")
		if retryAfter == "" {
			now := uint32(time.Now().Unix())
			defaultCooldown := uint32(60) // 默认冷却60秒
			untilTimestamp := now + defaultCooldown

			err := models.SetCooldownUntil(token, untilTimestamp)
			if err != nil {
				log.Printf("❌ [Rate Limit] 设置默认冷却时间失败: %v", err)
			} else {
				log.Printf("⚠️ [Rate Limit] 未提供 Retry-After，使用默认冷却时间: %d 秒", defaultCooldown)
			}
		}

		return true
	}

	// 403 可能也包含 Retry-After
	if resp.StatusCode == 403 {
		retryAfter := resp.Header.Get("Retry-After")
		if retryAfter != "" {
			log.Printf("🚫 [Forbidden] Figma API 返回 403 Forbidden with Retry-After")
			log.Printf("   - Token: %s...", token[:10])
			HandleRetryAfter(token, resp)
			return true
		}
	}

	return false
}

// FormatRetryError 格式化速率限制错误信息
func FormatRetryError(resp *http.Response) error {
	retryAfter := resp.Header.Get("Retry-After")
	if retryAfter == "" {
		return fmt.Errorf("Figma API 速率限制 (状态码: %d)", resp.StatusCode)
	}

	// 尝试解析为秒数
	if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds > 0 {
		return fmt.Errorf("Figma API 速率限制: 请等待 %d 秒 (状态码: %d)", seconds, resp.StatusCode)
	}

	// 尝试解析为 HTTP 日期
	if t, err := http.ParseTime(retryAfter); err == nil {
		waitSeconds := int(t.Unix() - time.Now().Unix())
		if waitSeconds > 0 {
			return fmt.Errorf("Figma API 速率限制: 请等待到 %s (约 %d 秒)", t.Format("15:04:05"), waitSeconds)
		}
	}

	return fmt.Errorf("Figma API 速率限制: Retry-After=%s (状态码: %d)", retryAfter, resp.StatusCode)
}
