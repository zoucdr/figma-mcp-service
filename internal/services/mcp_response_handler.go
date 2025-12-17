package services

import (
	"encoding/json"
	"log"
)

// HandleFigmaPluginResponse 处理来自Figma插件的响应
func (s *MCPService) HandleFigmaPluginResponse(message []byte) error {
	var msg struct {
		Type    string                 `json:"type"`
		Message map[string]interface{} `json:"message"`
		Channel string                 `json:"channel"`
		ID      string                 `json:"id"`
	}

	if err := json.Unmarshal(message, &msg); err != nil {
		log.Printf("解析Figma插件响应失败: %v", err)
		return err
	}

	// 处理broadcast类型的响应（来自Figma插件的实际响应）
	// 注意：消息类型可能是 "message" 或 "broadcast"
	// 客户端日志显示类型是 "message"
	if (msg.Type == "broadcast" || msg.Type == "message") && msg.Message != nil {
		if requestID, ok := msg.Message["id"].(string); ok {
			// 检查是否有结果
			if result, hasResult := msg.Message["result"]; hasResult {
				// 响应pending请求
				if s.RespondToPendingRequest(requestID, result, nil) {
					log.Printf("✓ 收到Figma响应并处理，requestID: %s", requestID)
					return nil
				} else {
					log.Printf("⚠️ 收到Figma响应但未找到对应请求(可能已超时)，requestID: %s", requestID)
				}
			}

			// 检查是否有错误
			if errorMsg, hasError := msg.Message["error"]; hasError {
				var errStr string
				if errMap, ok := errorMsg.(map[string]interface{}); ok {
					if message, ok := errMap["message"].(string); ok {
						errStr = message
					} else {
						errStr = "Unknown error"
					}
				} else if str, ok := errorMsg.(string); ok {
					errStr = str
				} else {
					errStr = "Unknown error format"
				}

				if s.RespondToPendingRequest(requestID, nil, &MCPError{
					Code:    -32603,
					Message: errStr,
				}) {
					log.Printf("✗ Figma返回错误并处理，requestID: %s, error: %s", requestID, errStr)
					return nil
				}
			}
		}
	}

	// 处理进度更新
	if msg.Type == "progress_update" {
		log.Printf("📊 进度更新: %v", msg.Message)
		// 可以在这里添加进度更新的处理逻辑
		// 例如：通过Server-Sent Events推送给前端
	}

	return nil
}

// MCPError MCP错误结构
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *MCPError) Error() string {
	return e.Message
}
