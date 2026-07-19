package models

import "time"

// AIConversation 对应 ai_conversations 表，记录用户围绕某篇帖子发起的 AI 共读会话。
// 一个用户对一篇帖子只会有一个会话（唯一索引 user_id+post_id），
// 会话内包含多条 AIMessage（用户提问 + AI 回复）。
type AIConversation struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID    int64     `gorm:"column:user_id;index:idx_ai_conv_user_post,unique" json:"userId"`
	PostID    int64     `gorm:"column:post_id;index:idx_ai_conv_user_post,unique" json:"postId"`
	Title     string    `gorm:"column:title;size:200" json:"title"`
	Status    int       `gorm:"column:status;default:0" json:"status"` // 0 正常 1 已归档
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName 指定 ai_conversations 表名。
func (AIConversation) TableName() string { return "ai_conversations" }

// AIMessage 对应 ai_messages 表，记录 AI 共读会话内的单条消息。
// Role 取值为 user（用户提问）或 assistant（AI 回复）。
type AIMessage struct {
	ID             int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ConversationID int64     `gorm:"column:conversation_id;index:idx_ai_msg_conv" json:"conversationId"`
	Role           string    `gorm:"column:role;type:varchar(20);default:user" json:"role"`
	Content        string    `gorm:"column:content;type:text" json:"content"`
	CreatedAt      time.Time `gorm:"column:created_at" json:"createdAt"`
}

// TableName 指定 ai_messages 表名。
func (AIMessage) TableName() string { return "ai_messages" }

// AI 会话消息角色枚举。
const (
	AIMessageRoleUser      = "user"
	AIMessageRoleAssistant = "assistant"
)

// AI 会话状态枚举。
const (
	AIConversationStatusNormal  = 0
	AIConversationStatusArchived = 1
)
