package models

import (
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// User 用户模型
type User struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Username     string         `gorm:"size:50;uniqueIndex;" json:"username"` // 移除not null约束
	PasswordHash string         `gorm:"size:255;not null" json:"-"`
	FigmaToken   string         `gorm:"size:255" json:"-"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	Projects     []FigmaProject `gorm:"foreignKey:UserID" json:"projects,omitempty"`
}

// SetPassword 设置用户密码
func (u *User) SetPassword(password string) error {
	if len(password) < 6 {
		return errors.New("密码长度至少为6位")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	u.PasswordHash = string(hashedPassword)
	return nil
}

// CheckPassword 检查密码是否正确
func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	return err == nil
}

// FindUserByUsername 通过用户名查找用户
func FindUserByUsername(username string) (*User, error) {
	var user User
	result := DB.Where("username = ?", username).First(&user)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, errors.New("用户不存在")
		}
		return nil, result.Error
	}
	return &user, nil
}

// CreateUser 创建新用户
func CreateUser(username, password, figmaToken string) (*User, error) {
	user := User{
		Username:   username,
		FigmaToken: figmaToken,
	}

	if err := user.SetPassword(password); err != nil {
		return nil, err
	}

	result := DB.Create(&user)
	if result.Error != nil {
		return nil, result.Error
	}

	return &user, nil
}

// UpdateFigmaToken 更新用户的Figma Token
func (u *User) UpdateFigmaToken(token string) error {
	u.FigmaToken = token
	result := DB.Save(u)
	return result.Error
}

// UpdateUsername 更新用户名
func (u *User) UpdateUsername(username string) error {
	// 如果用户名没有变化，直接返回成功
	if u.Username == username {
		return nil
	}

	// 检查新用户名是否已存在
	if username != "" {
		var count int64
		DB.Model(&User{}).Where("username = ? AND id != ?", username, u.ID).Count(&count)
		if count > 0 {
			return errors.New("用户名已存在")
		}
	}

	u.Username = username
	result := DB.Save(u)
	return result.Error
}

// UpdateProfile 更新用户资料
func (u *User) UpdateProfile(username, figmaToken string) error {
	// 更新用户名
	if err := u.UpdateUsername(username); err != nil {
		return err
	}

	// 更新Figma Token
	u.FigmaToken = figmaToken
	result := DB.Save(u)
	return result.Error
}
