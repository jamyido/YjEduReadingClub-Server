package models

import "time"

// Topic 对应 topics 表，每个新帖子必须关联一个有效话题。
// AdminOnly=true 的话题仅管理员可选择（如本周精读、本月精读），普通用户不可选（如打卡挑战为 false）。
type Topic struct {
	ID          int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Slug        string    `gorm:"column:slug;size:100;uniqueIndex" json:"slug"`
	Title       string    `gorm:"column:title;size:200;uniqueIndex" json:"title"`
	Description *string   `gorm:"column:description;type:text" json:"description"`
	Status      int       `gorm:"column:status" json:"status"`
	Sort        int       `gorm:"column:sort" json:"sort"`
	AdminOnly   bool      `gorm:"column:admin_only;default:false" json:"adminOnly"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName 指定 topics 表名。
func (Topic) TableName() string { return "topics" }
