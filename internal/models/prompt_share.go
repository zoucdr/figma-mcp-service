package models

import (
	"time"
)

// PromptShare 提示词分享
type PromptShare struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"not null;index" json:"user_id"`
	Username    string    `gorm:"size:255" json:"username"`
	Title       string    `gorm:"size:255;not null" json:"title"`
	Prompt      string    `gorm:"type:text;not null" json:"prompt"`
	Description string    `gorm:"type:text" json:"description"`
	LikeCount   int       `gorm:"default:0" json:"like_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// 关联
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// PromptLike 提示词点赞记录
type PromptLike struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	PromptShareID uint      `gorm:"not null;uniqueIndex:idx_prompt_like_unique" json:"prompt_share_id"`
	UserID        uint      `gorm:"not null;uniqueIndex:idx_prompt_like_unique" json:"user_id"`
	CreatedAt     time.Time `json:"created_at"`

	// 联合唯一索引，防止重复点赞
}

// TableName 指定表名
func (PromptShare) TableName() string {
	return "prompt_shares"
}

// TableName 指定表名
func (PromptLike) TableName() string {
	return "prompt_likes"
}

// CreatePromptShare 创建提示词分享
func CreatePromptShare(share *PromptShare) error {
	return DB.Create(share).Error
}

// GetPromptShareByID 根据ID获取提示词分享
func GetPromptShareByID(id uint) (*PromptShare, error) {
	var share PromptShare
	err := DB.Preload("User").First(&share, id).Error
	return &share, err
}

// GetAllPromptShares 获取所有提示词分享（按点赞数排序）
func GetAllPromptShares(limit, offset int) ([]PromptShare, error) {
	var shares []PromptShare
	err := DB.Preload("User").
		Order("like_count DESC, created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&shares).Error
	return shares, err
}

// GetPromptSharesByUserID 获取用户的提示词分享
func GetPromptSharesByUserID(userID uint) ([]PromptShare, error) {
	var shares []PromptShare
	err := DB.Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&shares).Error
	return shares, err
}

// UpdatePromptShare 更新提示词分享
func UpdatePromptShare(share *PromptShare) error {
	return DB.Save(share).Error
}

// DeletePromptShare 删除提示词分享
func DeletePromptShare(id uint) error {
	// 先删除相关的点赞记录
	DB.Where("prompt_share_id = ?", id).Delete(&PromptLike{})
	// 删除分享
	return DB.Delete(&PromptShare{}, id).Error
}

// LikePromptShare 点赞提示词
func LikePromptShare(promptShareID, userID uint) error {
	// 检查是否已经点赞
	var count int64
	DB.Model(&PromptLike{}).
		Where("prompt_share_id = ? AND user_id = ?", promptShareID, userID).
		Count(&count)

	if count > 0 {
		return nil // 已经点赞过
	}

	// 创建点赞记录
	like := &PromptLike{
		PromptShareID: promptShareID,
		UserID:        userID,
	}

	if err := DB.Create(like).Error; err != nil {
		return err
	}

	// 增加点赞数
	return DB.Model(&PromptShare{}).
		Where("id = ?", promptShareID).
		UpdateColumn("like_count", DB.Raw("like_count + 1")).
		Error
}

// UnlikePromptShare 取消点赞
func UnlikePromptShare(promptShareID, userID uint) error {
	// 删除点赞记录
	result := DB.Where("prompt_share_id = ? AND user_id = ?", promptShareID, userID).
		Delete(&PromptLike{})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected > 0 {
		// 减少点赞数
		return DB.Model(&PromptShare{}).
			Where("id = ?", promptShareID).
			UpdateColumn("like_count", DB.Raw("like_count - 1")).
			Error
	}

	return nil
}

// HasUserLiked 检查用户是否已点赞
func HasUserLiked(promptShareID, userID uint) (bool, error) {
	var count int64
	err := DB.Model(&PromptLike{}).
		Where("prompt_share_id = ? AND user_id = ?", promptShareID, userID).
		Count(&count).Error
	return count > 0, err
}

// GetPromptSharesCount 获取提示词分享总数
func GetPromptSharesCount() (int64, error) {
	var count int64
	err := DB.Model(&PromptShare{}).Count(&count).Error
	return count, err
}
