package models

import "time"

// Comment 对应 comments 表，支持一级评论与二级回复。
// 字段类型与 Prisma migration SQL 对齐。
type Comment struct {
	ID         int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	PostID     int64     `gorm:"column:post_id" json:"postId"`
	AuthorID   int64     `gorm:"column:author_id" json:"authorId"`
	ParentID   *int64    `gorm:"column:parent_id" json:"parentId"`
	ReplyToID  *int64    `gorm:"column:reply_to_id" json:"replyToId"`
	Content    string    `gorm:"column:content;type:text" json:"content"`
	LikeCount  int       `gorm:"column:like_count" json:"likeCount"`
	Status     int       `gorm:"column:status" json:"status"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt  time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName 指定 comments 表名。
func (Comment) TableName() string { return "comments" }
