package models

import "time"

// Course 对应 courses 表。
// 字段类型与 Prisma migration SQL 对齐。
type Course struct {
	ID          int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Title       string    `gorm:"column:title;size:200" json:"title"`
	Description *string   `gorm:"column:description;type:text" json:"description"`
	Cover       *string   `gorm:"column:cover;type:text" json:"cover"`
	CircleID    *int64    `gorm:"column:circle_id" json:"circleId"`
	CreatorID   int64     `gorm:"column:creator_id" json:"creatorId"`
	Status      int       `gorm:"column:status" json:"status"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updatedAt"`
	// Progress 为非持久化字段，仅在课程详情接口中携带当前登录用户的学习进度。
	Progress *CourseProgress `gorm:"-" json:"progress,omitempty"`
}

// TableName 指定 courses 表名。
func (Course) TableName() string { return "courses" }

// CourseChapter 对应 course_chapters 表，课程章节。
// 字段类型与 Prisma migration SQL 对齐。
type CourseChapter struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	CourseID  int64     `gorm:"column:course_id" json:"courseId"`
	Title     string    `gorm:"column:title;size:200" json:"title"`
	Content   *string   `gorm:"column:content;type:text" json:"content"`
	VideoURL  *string   `gorm:"column:video_url;type:text" json:"videoUrl"`
	Sort      int       `gorm:"column:sort" json:"sort"`
	Duration  int       `gorm:"column:duration" json:"duration"`
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName 指定 course_chapters 表名。
func (CourseChapter) TableName() string { return "course_chapters" }

// CourseProgress 对应 course_progresses 表，学习进度。
type CourseProgress struct {
	ID                 int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID             int64     `gorm:"column:user_id;uniqueIndex:idx_user_course" json:"userId"`
	CourseID           int64     `gorm:"column:course_id;uniqueIndex:idx_user_course" json:"courseId"`
	CurrentChapterID   *int64    `gorm:"column:current_chapter_id" json:"currentChapterId"`
	CompletedChapterIDs *string  `gorm:"column:completed_chapter_ids;type:text" json:"completedChapterIds"`
	Progress           int       `gorm:"column:progress" json:"progress"`
	IsCompleted        bool      `gorm:"column:is_completed" json:"isCompleted"`
	CreatedAt          time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt          time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName 指定 course_progresses 表名。
func (CourseProgress) TableName() string { return "course_progresses" }
