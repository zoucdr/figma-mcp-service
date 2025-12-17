package controllers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/figma-deliver/internal/middleware"
	"github.com/figma-deliver/internal/models"
	"github.com/gin-gonic/gin"
)

// SharePage 分享页面
func SharePage(c *gin.Context) {
	// 获取时间戳
	timestamp, exists := c.Get("timestamp")
	if !exists {
		timestamp = "1"
	}

	data := gin.H{
		"title":     "提示词分享",
		"timestamp": timestamp,
	}

	// 如果用户已登录，添加用户信息和 CSRF Token
	userID := middleware.GetUserIDOptional(c)
	log.Printf("SharePage - 获取用户ID: %d", userID)

	if userID != 0 {
		user, err := models.FindUserByID(userID)
		if err == nil {
			data["username"] = user.Username
			data["user_id"] = user.ID
			log.Printf("SharePage - 用户已登录: ID=%d, 用户名=%s", user.ID, user.Username)
		} else {
			log.Printf("SharePage - 查找用户失败: %v", err)
		}

		// 添加 CSRF Token
		csrfToken, exists := c.Get("csrf_token")
		if exists {
			data["csrf_token"] = csrfToken
		}
	} else {
		log.Printf("SharePage - 用户未登录")
	}

	c.HTML(http.StatusOK, "share.html", data)
}

// GetPromptShares 获取所有提示词分享
func GetPromptShares(c *gin.Context) {
	// 获取分页参数
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	// 获取提示词分享列表
	shares, err := models.GetAllPromptShares(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取分享列表失败: " + err.Error(),
		})
		return
	}

	// 获取总数
	total, err := models.GetPromptSharesCount()
	if err != nil {
		total = 0
	}

	// 清除关联用户数据，减少响应大小
	for i := range shares {
		shares[i].User = nil
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"shares": shares,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		},
	})
}

// CreatePromptShare 创建提示词分享
func CreatePromptShare(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req struct {
		Title       string `json:"title" binding:"required"`
		Prompt      string `json:"prompt" binding:"required"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数错误: " + err.Error(),
		})
		return
	}

	// 获取用户信息
	user, err := models.FindUserByID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取用户信息失败",
		})
		return
	}

	// 创建分享
	share := &models.PromptShare{
		UserID:      userID,
		Username:    user.Username,
		Title:       req.Title,
		Prompt:      req.Prompt,
		Description: req.Description,
		LikeCount:   0,
	}

	if err := models.CreatePromptShare(share); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "创建分享失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    share,
		"message": "分享成功",
	})
}

// DeletePromptShare 删除提示词分享
func DeletePromptShare(c *gin.Context) {
	userID := middleware.GetUserID(c)
	shareID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的分享ID",
		})
		return
	}

	// 获取分享信息
	share, err := models.GetPromptShareByID(uint(shareID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "分享不存在",
		})
		return
	}

	// 检查是否是分享者本人
	if share.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "无权删除此分享",
		})
		return
	}

	// 删除分享
	if err := models.DeletePromptShare(uint(shareID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "删除分享失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "删除成功",
	})
}

// LikePromptShare 点赞提示词分享
func LikePromptShare(c *gin.Context) {
	userID := middleware.GetUserID(c)
	shareID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的分享ID",
		})
		return
	}

	// 获取分享信息
	share, err := models.GetPromptShareByID(uint(shareID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "分享不存在",
		})
		return
	}

	// 检查是否是自己的分享
	if share.UserID == userID {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "不能为自己的分享点赞",
		})
		return
	}

	// 点赞
	if err := models.LikePromptShare(uint(shareID), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "点赞失败: " + err.Error(),
		})
		return
	}

	// 获取最新的点赞数
	share, _ = models.GetPromptShareByID(uint(shareID))

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"like_count": share.LikeCount,
		"message":    "点赞成功",
	})
}

// UnlikePromptShare 取消点赞
func UnlikePromptShare(c *gin.Context) {
	userID := middleware.GetUserID(c)
	shareID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的分享ID",
		})
		return
	}

	// 取消点赞
	if err := models.UnlikePromptShare(uint(shareID), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "取消点赞失败: " + err.Error(),
		})
		return
	}

	// 获取最新的点赞数
	share, _ := models.GetPromptShareByID(uint(shareID))

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"like_count": share.LikeCount,
		"message":    "取消点赞成功",
	})
}

// CheckLikeStatus 检查点赞状态
func CheckLikeStatus(c *gin.Context) {
	userID := middleware.GetUserID(c)
	shareID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的分享ID",
		})
		return
	}

	liked, err := models.HasUserLiked(uint(shareID), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "检查点赞状态失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"liked":   liked,
	})
}

// GetMyShares 获取我的分享
func GetMyShares(c *gin.Context) {
	userID := middleware.GetUserID(c)

	shares, err := models.GetPromptSharesByUserID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取分享列表失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    shares,
	})
}
