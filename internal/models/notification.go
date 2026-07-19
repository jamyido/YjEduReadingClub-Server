package models

import "time"

// Notification 对应 notifications 表，记录系统/互动通知。
// 字段类型与 Prisma migration SQL 对齐，type 包含 TASK 以兼容代码中的 NotificationTypeTask。
type Notification struct {
	ID         int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID     int64     `gorm:"column:user_id" json:"userId"`
	Type       string    `gorm:"column:type;type:varchar(20);default:SYSTEM" json:"type"`
	ActorID    *int64    `gorm:"column:actor_id" json:"actorId"`
	TargetType *string   `gorm:"column:target_type;size:20" json:"targetType"`
	TargetID   *int64    `gorm:"column:target_id" json:"targetId"`
	Title      string    `gorm:"column:title;size:200" json:"title"`
	Content    *string   `gorm:"column:content;type:text" json:"content"`
	IsRead     bool      `gorm:"column:is_read" json:"isRead"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"createdAt"`
}

// TableName 指定 notifications 表名。
func (Notification) TableName() string { return "notifications" }
