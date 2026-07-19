package models

import "time"

// Message 对应 messages 表，记录用户间私信。
// 字段类型与 Prisma migration SQL 对齐。
type Message struct {
	ID         int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SenderID   int64     `gorm:"column:sender_id" json:"senderId"`
	ReceiverID int64     `gorm:"column:receiver_id" json:"receiverId"`
	Type       string    `gorm:"column:type;type:varchar(20);default:TEXT" json:"type"`
	Content    string    `gorm:"column:content;type:text" json:"content"`
	IsRead     bool      `gorm:"column:is_read" json:"isRead"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"createdAt"`
}

// TableName 指定 messages 表名。
func (Message) TableName() string { return "messages" }
