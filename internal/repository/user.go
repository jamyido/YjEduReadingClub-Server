// Package repository 封装所有数据库访问逻辑。
// 每个领域对应一个无状态结构体，方法统一接受 *gorm.DB 作为首参，
// 以便在普通调用时传入 database.Get()，在事务中传入 tx。
package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"yjedu-reading-club-server/internal/models"
)

// UserRepository 封装用户表的数据库操作。
type UserRepository struct{}

// UserRepo 是全局可用的用户仓储单例。
var UserRepo = UserRepository{}

// UserBrief 是关联上下文中的用户精简信息，仅暴露公开字段。
// 用于帖子作者、评论作者、圈子成员等关联场景，避免泄露 phone、birthday、
// role、status、lastCheckInAt 等敏感或私有字段。
type UserBrief struct {
	ID       int64   `json:"id"`
	Nickname string  `json:"nickname"`
	Avatar   *string `json:"avatar"`
}

// LoadUserBrief 根据用户 ID 查询精简用户信息。
// 仅 SELECT id/nickname/avatar 三列，避免加载敏感字段。
// 用户不存在时返回 nil。
func LoadUserBrief(db *gorm.DB, userID int64) *UserBrief {
	var u models.User
	if err := db.Select("id, nickname, avatar").First(&u, userID).Error; err != nil {
		return nil
	}
	return &UserBrief{
		ID:       u.ID,
		Nickname: u.Nickname,
		Avatar:   u.Avatar,
	}
}

// FindByID 根据主键查询用户。
func (UserRepository) FindByID(db *gorm.DB, id int64) (*models.User, error) {
	var user models.User
	if err := db.First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// FindByPhone 根据手机号查询用户。
func (UserRepository) FindByPhone(db *gorm.DB, phone string) (*models.User, error) {
	var user models.User
	if err := db.Where("phone = ?", phone).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// FindByWeappOpenID 根据微信 openid 查询用户。
func (UserRepository) FindByWeappOpenID(db *gorm.DB, openID string) (*models.User, error) {
	var user models.User
	if err := db.Where("weapp_open_id = ?", openID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// CreateUserInput 是创建用户的输入参数。
type CreateUserInput struct {
	Phone       string
	Password    *string
	WeappOpenID *string
	UnionID     *string
	Nickname    string
	Avatar      *string
}

// Create 创建用户。
func (UserRepository) Create(db *gorm.DB, input CreateUserInput) (*models.User, error) {
	user := models.User{
		Phone:       input.Phone,
		Password:    input.Password,
		WeappOpenID: input.WeappOpenID,
		UnionID:     input.UnionID,
		Nickname:    input.Nickname,
		Avatar:      input.Avatar,
		Gender:      models.GenderUnknown,
		Role:        models.RoleUser,
		Status:      models.UserStatusActive,
	}
	if err := db.Create(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// UpdatePassword 更新用户密码。
func (UserRepository) UpdatePassword(db *gorm.DB, userID int64, hashedPassword string) error {
	return db.Model(&models.User{}).Where("id = ?", userID).
		Update("password", hashedPassword).Error
}

// UpdatePhone 更新用户手机号。
func (UserRepository) UpdatePhone(db *gorm.DB, userID int64, phone string) error {
	return db.Model(&models.User{}).Where("id = ?", userID).
		Update("phone", phone).Error
}

// BindWeappOpenID 为已有用户绑定微信 openid 与 unionid。
func (UserRepository) BindWeappOpenID(db *gorm.DB, userID int64, openID, unionID string) error {
	updates := map[string]interface{}{"weapp_open_id": openID}
	if unionID != "" {
		updates["union_id"] = unionID
	}
	return db.Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error
}

// UpdateUserInput 是更新用户资料的输入参数，仅更新非 nil 字段。
type UpdateUserInput struct {
	Nickname *string
	Avatar   *string
	Bio      *string
	Gender   *string
	Birthday *time.Time
}

// Update 更新用户资料，仅更新实际传入字段。
func (UserRepository) Update(db *gorm.DB, userID int64, input UpdateUserInput) (*models.User, error) {
	updates := map[string]interface{}{}
	if input.Nickname != nil {
		updates["nickname"] = *input.Nickname
	}
	if input.Avatar != nil {
		updates["avatar"] = *input.Avatar
	}
	if input.Bio != nil {
		updates["bio"] = *input.Bio
	}
	if input.Gender != nil {
		updates["gender"] = *input.Gender
	}
	if input.Birthday != nil {
		updates["birthday"] = *input.Birthday
	}
	if len(updates) == 0 {
		return UserRepo.FindByID(db, userID)
	}
	if err := db.Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		return nil, err
	}
	return UserRepo.FindByID(db, userID)
}

// UpdateFollowCounts 调整用户关注/粉丝计数。
// followingDelta 与 followerDelta 可为负数。
func (UserRepository) UpdateFollowCounts(db *gorm.DB, userID int64, followingDelta, followerDelta int) error {
	updates := map[string]interface{}{}
	if followingDelta != 0 {
		updates["following_count"] = gorm.Expr("following_count + ?", followingDelta)
	}
	if followerDelta != 0 {
		updates["follower_count"] = gorm.Expr("follower_count + ?", followerDelta)
	}
	if len(updates) == 0 {
		return nil
	}
	return db.Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error
}

// UpdateCheckInStreak 更新用户连续打卡天数与最后打卡时间。
func (UserRepository) UpdateCheckInStreak(db *gorm.DB, userID int64, streakDays int, lastCheckInAt interface{}) error {
	return db.Model(&models.User{}).Where("id = ?", userID).
		Updates(map[string]interface{}{
			"streak_days":     streakDays,
			"last_check_in_at": lastCheckInAt,
		}).Error
}

// FindAllActiveIDs 查询所有未封禁用户的 ID 列表。
func (UserRepository) FindAllActiveIDs(db *gorm.DB) ([]int64, error) {
	var ids []int64
	if err := db.Model(&models.User{}).Where("status = ?", models.UserStatusActive).
		Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// AIAssistantPhone 是 AI伴读 系统用户保留的手机号。
// 该号码不是合法中国移动号码（第二位为 0），不会与真实用户冲突。
const AIAssistantPhone = "10000000000"

// AIAssistantNickname 是 AI伴读 系统用户的固定昵称。
const AIAssistantNickname = "AI伴读"

// FindOrCreateAIAssistant 幂等获取或创建 AI伴读 系统用户并返回其 ID。
// 该用户作为定时总结帖（日/周/月精华）的统一作者身份出现。
// 不参与圈子成员关系，仅作为帖子作者展示。
func (UserRepository) FindOrCreateAIAssistant(db *gorm.DB) (int64, error) {
	var user models.User
	err := db.Where("phone = ?", AIAssistantPhone).First(&user).Error
	if err == nil {
		return user.ID, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}
	bio := "嘉阅圈官方 AI 共读助手，负责生成每日共性问题总结与周/月精华。"
	user = models.User{
		Phone:    AIAssistantPhone,
		Nickname: AIAssistantNickname,
		Bio:      &bio,
		Gender:   models.GenderUnknown,
		Role:     models.RoleUser,
		Status:   models.UserStatusActive,
	}
	if err := db.Create(&user).Error; err != nil {
		// 并发场景下可能因唯一索引冲突，再次查询。
		var existing models.User
		if err2 := db.Where("phone = ?", AIAssistantPhone).First(&existing).Error; err2 == nil {
			return existing.ID, nil
		}
		return 0, err
	}
	return user.ID, nil
}

// FindActiveIDs 查询指定 ID 列表中所有未封禁用户的 ID。
func (UserRepository) FindActiveIDs(db *gorm.DB, ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var result []int64
	if err := db.Model(&models.User{}).Where("id IN ? AND status = ?", ids, models.UserStatusActive).
		Pluck("id", &result).Error; err != nil {
		return nil, err
	}
	return result, nil
}
