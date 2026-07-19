package models

import "time"

// Post 对应 posts 表。
// 字段类型与 Prisma migration SQL 对齐。
type Post struct {
	ID            int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	AuthorID      int64     `gorm:"column:author_id" json:"authorId"`
	CircleID      *int64    `gorm:"column:circle_id" json:"circleId"`
	TopicID       *int64    `gorm:"column:topic_id" json:"topicId"`
	Type          string    `gorm:"column:type;type:varchar(20);default:TEXT" json:"type"`
	Title         *string   `gorm:"column:title;size:200" json:"title"`
	Content       string    `gorm:"column:content;type:text" json:"content"`
	LinkURL       *string   `gorm:"column:link_url;type:text" json:"linkUrl"`
	LikeCount     int       `gorm:"column:like_count" json:"likeCount"`
	CommentCount  int       `gorm:"column:comment_count" json:"commentCount"`
	ShareCount    int       `gorm:"column:share_count" json:"shareCount"`
	IsPinned      bool      `gorm:"column:is_pinned" json:"isPinned"`
	IsEssence     bool      `gorm:"column:is_essence" json:"isEssence"`
	Status        int       `gorm:"column:status" json:"status"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName 指定 posts 表名。
func (Post) TableName() string { return "posts" }

// PostMedia 对应 post_medias 表，存储帖子关联的图片/视频资源。
type PostMedia struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	PostID    int64     `gorm:"column:post_id" json:"postId"`
	Type      string    `gorm:"column:type;size:20" json:"type"`
	URL       string    `gorm:"column:url;type:text" json:"url"`
	Sort      int       `gorm:"column:sort" json:"sort"`
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
}

// TableName 指定 post_medias 表名。
func (PostMedia) TableName() string { return "post_medias" }
