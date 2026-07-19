package models

import "time"

// Circle 对应 circles 表。
// 字段类型与 Prisma migration SQL 对齐。
type Circle struct {
	ID          int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"column:name;size:100" json:"name"`
	Description *string   `gorm:"column:description;type:text" json:"description"`
	Cover       *string   `gorm:"column:cover;type:text" json:"cover"`
	ThemeColor  *string   `gorm:"column:theme_color;size:20" json:"themeColor"`
	IsPublic    bool      `gorm:"column:is_public" json:"isPublic"`
	MemberCount int       `gorm:"column:member_count" json:"memberCount"`
	PostCount   int       `gorm:"column:post_count" json:"postCount"`
	OwnerID     int64     `gorm:"column:owner_id" json:"ownerId"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName 指定 circles 表名。
func (Circle) TableName() string { return "circles" }

// CircleMember 对应 circle_members 表，记录用户与圈子的从属关系及角色。
type CircleMember struct {
	ID       int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID   int64     `gorm:"column:user_id;uniqueIndex:idx_user_circle" json:"userId"`
	CircleID int64     `gorm:"column:circle_id;uniqueIndex:idx_user_circle" json:"circleId"`
	Role     string    `gorm:"column:role;type:varchar(20);default:MEMBER" json:"role"`
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName 指定 circle_members 表名。
func (CircleMember) TableName() string { return "circle_members" }
