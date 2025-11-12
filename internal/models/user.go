package models

import (
	"errors"
	"fmt"
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
	MCPToken     string         `gorm:"size:255" json:"mcp_token"`   // MCP Token
	CompTypes    string         `gorm:"size:1000" json:"comp_types"`     // 控件类型列表，逗号分隔
	Prompts      string         `gorm:"type:text" json:"prompts"`        // 修饰提示词，用于MCP节点修饰
	CodePrompts  string         `gorm:"type:text" json:"code_prompts"`   // 代码生成提示词，用于Swift等代码生成
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

// FindUserByID 通过用户ID查找用户
func FindUserByID(id uint) (*User, error) {
	var user User
	result := DB.First(&user, id)
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
func (u *User) UpdateProfile(username, figmaToken, compTypes string) error {
	// 更新用户名
	if err := u.UpdateUsername(username); err != nil {
		return err
	}

	// 仅当figmaToken不为空时，才更新Figma Token
	if figmaToken != "" {
		u.FigmaToken = figmaToken
	}

	// 更新控件类型列表
	u.CompTypes = compTypes

	// 保存所有更新
	result := DB.Save(u)
	return result.Error
}

// UpdateProfileWithPrompts 更新用户资料（包含提示词）
func (u *User) UpdateProfileWithPrompts(username, figmaToken, compTypes, prompts, codePrompts string) error {
	// 更新用户名
	if err := u.UpdateUsername(username); err != nil {
		return err
	}

	// 仅当figmaToken不为空时，才更新Figma Token
	if figmaToken != "" {
		u.FigmaToken = figmaToken
	}

	// 更新控件类型列表
	u.CompTypes = compTypes

	// 更新修饰提示词
	u.Prompts = prompts

	// 更新代码生成提示词
	u.CodePrompts = codePrompts

	// 保存所有更新
	result := DB.Save(u)
	return result.Error
}

// UpdateMCPToken 更新用户的MCP Token
func (u *User) UpdateMCPToken(token string) error {
	u.MCPToken = token
	result := DB.Save(u)
	return result.Error
}

// GenerateNewMCPToken 生成新的MCP Token
func (u *User) GenerateNewMCPToken() (string, error) {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	tokenLength := 32
	result := make([]byte, tokenLength)

	for i := 0; i < tokenLength; i++ {
		result[i] = chars[time.Now().UnixNano()%int64(len(chars))]
		time.Sleep(time.Nanosecond)
	}

	newToken := fmt.Sprintf("mcp_%s", string(result))

	// 更新用户的MCP Token
	if err := u.UpdateMCPToken(newToken); err != nil {
		return "", err
	}

	return newToken, nil
}

// EnsureUserExists 确保用户存在，如果不存在则创建一个临时用户
func EnsureUserExists(userID uint) (*User, error) {
	var user User
	result := DB.First(&user, userID)
	if result.Error == nil {
		return &user, nil
	}

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		// 创建临时用户
		tempUsername := "temp_user_" + time.Now().Format("20060102150405")
		tempPassword := "temp_password_" + time.Now().Format("20060102150405")

		user := User{
			ID:       userID, // 指定ID
			Username: tempUsername,
		}

		// 设置密码
		if err := user.SetPassword(tempPassword); err != nil {
			return nil, err
		}

		// 创建用户
		result = DB.Create(&user)
		if result.Error != nil {
			return nil, result.Error
		}

		return &user, nil
	}

	return nil, result.Error
}
