package models

import "time"

// CheckIn 对应 check_ins 表，记录每日打卡。
// 字段类型与 Prisma migration SQL 对齐，check_in_date 用 varchar(20) 支持唯一索引。
type CheckIn struct {
	ID          int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID      int64     `gorm:"column:user_id;uniqueIndex:idx_user_checkin_date" json:"userId"`
	PostID      *int64    `gorm:"column:post_id;uniqueIndex" json:"postId"`
	CheckInDate *string   `gorm:"column:check_in_date;size:20;uniqueIndex:idx_user_checkin_date" json:"checkInDate"`
	CircleID    *int64    `gorm:"column:circle_id" json:"circleId"`
	Content     *string   `gorm:"column:content;type:text" json:"content"`
	Images      *string   `gorm:"column:images;type:text" json:"images"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"createdAt"`
}

// TableName 指定 check_ins 表名。
func (CheckIn) TableName() string { return "check_ins" }
