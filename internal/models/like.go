package models

import "time"

// Like 对应 likes 表，记录对帖子或评论的点赞。
// 字段类型与 Prisma migration SQL 对齐，target_type 用 varchar(20) 支持唯一索引。
type Like struct {
	ID         int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID     int64     `gorm:"column:user_id;uniqueIndex:idx_like_target" json:"userId"`
	TargetType string    `gorm:"column:target_type;size:20;uniqueIndex:idx_like_target" json:"targetType"`
	TargetID   int64     `gorm:"column:target_id;uniqueIndex:idx_like_target" json:"targetId"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"createdAt"`
}

// TableName 指定 likes 表名。
func (Like) TableName() string { return "likes" }
