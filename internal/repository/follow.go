package repository

import (
	"errors"

	"gorm.io/gorm"

	"yjedu-reading-club-server/internal/models"
)

// FollowRepository 封装关注关系的数据库操作。
type FollowRepository struct{}

// FollowRepo 是全局可用的关注仓储单例。
var FollowRepo = FollowRepository{}

// FindRelation 查询关注关系是否存在。
func (FollowRepository) FindRelation(db *gorm.DB, followerID, followingID int64) (*models.Follow, error) {
	var follow models.Follow
	err := db.Where("follower_id = ? AND following_id = ?", followerID, followingID).First(&follow).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &follow, nil
}

// Follow 创建关注关系。
func (FollowRepository) Follow(db *gorm.DB, followerID, followingID int64) error {
	follow := models.Follow{
		FollowerID:  followerID,
		FollowingID: followingID,
	}
	return db.Create(&follow).Error
}

// Unfollow 删除关注关系。
func (FollowRepository) Unfollow(db *gorm.DB, followerID, followingID int64) error {
	return db.Where("follower_id = ? AND following_id = ?", followerID, followingID).
		Delete(&models.Follow{}).Error
}

// FollowUserItem 是关注/粉丝列表项。
type FollowUserItem struct {
	ID         int64   `gorm:"column:id" json:"id"`
	Nickname   string  `gorm:"column:nickname" json:"nickname"`
	Avatar     *string `gorm:"column:avatar" json:"avatar"`
	Bio        *string `gorm:"column:bio" json:"bio"`
	FollowedAt string  `gorm:"column:followed_at" json:"followedAt"`
}

// FindFollowers 分页查询用户的粉丝列表。
func (FollowRepository) FindFollowers(db *gorm.DB, userID int64, page, pageSize int) ([]FollowUserItem, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	query := db.Table("follows").Where("following_id = ?", userID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []FollowUserItem
	err := db.Table("follows").
		Select(`users.id, users.nickname, users.avatar, users.bio, to_char(follows.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS followed_at`).
		Joins("INNER JOIN users ON users.id = follows.follower_id").
		Where("follows.following_id = ?", userID).
		Order("follows.created_at DESC").
		Offset(offset).Limit(pageSize).
		Find(&list).Error
	return list, total, err
}

// FindFollowing 分页查询用户的关注列表。
func (FollowRepository) FindFollowing(db *gorm.DB, userID int64, page, pageSize int) ([]FollowUserItem, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	query := db.Table("follows").Where("follower_id = ?", userID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []FollowUserItem
	err := db.Table("follows").
		Select(`users.id, users.nickname, users.avatar, users.bio, to_char(follows.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS followed_at`).
		Joins("INNER JOIN users ON users.id = follows.following_id").
		Where("follows.follower_id = ?", userID).
		Order("follows.created_at DESC").
		Offset(offset).Limit(pageSize).
		Find(&list).Error
	return list, total, err
}
