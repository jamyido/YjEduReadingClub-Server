package models

import "time"

// User 对应 users 表，支持手机号+密码登录与微信小程序登录。
// 字段类型与 Prisma migration SQL 对齐，避免 GORM 默认 longtext 导致唯一索引失败。
type User struct {
	ID             int64      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Phone          string     `gorm:"column:phone;size:20;uniqueIndex" json:"phone"`
	Password       *string    `gorm:"column:password;size:255" json:"-"`
	WeappOpenID    *string    `gorm:"column:weapp_open_id;size:100;uniqueIndex" json:"-"`
	UnionID        *string    `gorm:"column:union_id;size:100;uniqueIndex" json:"-"`
	Nickname       string     `gorm:"column:nickname;size:50" json:"nickname"`
	Avatar         *string    `gorm:"column:avatar;type:text" json:"avatar"`
	Bio            *string    `gorm:"column:bio;size:500" json:"bio"`
	Gender         string     `gorm:"column:gender;type:varchar(10);default:UNKNOWN" json:"gender"`
	Birthday       *time.Time `gorm:"column:birthday" json:"birthday"`
	Role           string     `gorm:"column:role;type:varchar(10);default:USER" json:"role"`
	Status         string     `gorm:"column:status;type:varchar(10);default:ACTIVE" json:"status"`
	StreakDays     int        `gorm:"column:streak_days" json:"streakDays"`
	LastCheckInAt  *time.Time `gorm:"column:last_check_in_at" json:"lastCheckInAt"`
	FollowingCount int        `gorm:"column:following_count" json:"followingCount"`
	FollowerCount  int        `gorm:"column:follower_count" json:"followerCount"`
	CreatedAt      time.Time  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt      time.Time  `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName 指定 users 表名。
func (User) TableName() string { return "users" }
