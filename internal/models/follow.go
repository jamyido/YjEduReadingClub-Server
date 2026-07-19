package models

import "time"

// Follow 对应 follows 表，记录用户关注关系。
type Follow struct {
	ID          int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	FollowerID  int64     `gorm:"column:follower_id;uniqueIndex:idx_follow_pair" json:"followerId"`
	FollowingID int64     `gorm:"column:following_id;uniqueIndex:idx_follow_pair" json:"followingId"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"createdAt"`
}

// TableName 指定 follows 表名。
func (Follow) TableName() string { return "follows" }
