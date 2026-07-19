package repository

import (
	"errors"

	"gorm.io/gorm"

	"yjedu-reading-club-server/internal/models"
)

// CourseRepository 封装课程及学习进度的数据库操作。
type CourseRepository struct{}

// CourseRepo 是全局可用的课程仓储单例。
var CourseRepo = CourseRepository{}

// CourseDetail 是课程详情查询结果，包含章节、创建者与所属圈子。
type CourseDetail struct {
	models.Course
	Creator  *UserBrief             `gorm:"foreignKey:CreatorID" json:"creator"`
	Circle   *models.Circle         `gorm:"foreignKey:CircleID" json:"circle"`
	Chapters []models.CourseChapter `gorm:"foreignKey:CourseID" json:"chapters"`
}

// CourseListOptions 是课程列表查询选项。
type CourseListOptions struct {
	Page      int
	PageSize  int
	CircleID  int64
	CreatorID int64
}

// FindMany 分页查询课程列表。
func (CourseRepository) FindMany(db *gorm.DB, opts CourseListOptions) ([]models.Course, int64, error) {
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PageSize < 1 {
		opts.PageSize = 20
	}
	offset := (opts.Page - 1) * opts.PageSize
	query := db.Model(&models.Course{})
	if opts.CircleID != 0 {
		query = query.Where("circle_id = ?", opts.CircleID)
	}
	if opts.CreatorID != 0 {
		query = query.Where("creator_id = ?", opts.CreatorID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []models.Course
	if err := query.Order("created_at DESC").Offset(offset).Limit(opts.PageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// FindByID 查询课程详情，包含章节列表、创建者与所属圈子。
func (CourseRepository) FindByID(db *gorm.DB, id int64) (*CourseDetail, error) {
	var course models.Course
	if err := db.First(&course, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	detail := &CourseDetail{Course: course}
	// 创建者仅取公开字段，避免泄露敏感信息。
	detail.Creator = LoadUserBrief(db, course.CreatorID)
	if course.CircleID != nil {
		var circle models.Circle
		if err := db.Select("id, name").First(&circle, *course.CircleID).Error; err == nil {
			detail.Circle = &circle
		}
	}
	var chapters []models.CourseChapter
	if err := db.Where("course_id = ?", id).Order("sort ASC").Find(&chapters).Error; err == nil {
		detail.Chapters = chapters
	}
	return detail, nil
}

// FindProgress 查询用户在某课程的学习进度。
func (CourseRepository) FindProgress(db *gorm.DB, userID, courseID int64) (*models.CourseProgress, error) {
	var progress models.CourseProgress
	err := db.Where("user_id = ? AND course_id = ?", userID, courseID).First(&progress).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &progress, nil
}

// UpsertProgressInput 是更新学习进度的输入参数。
type UpsertProgressInput struct {
	CurrentChapterID    *int64
	CompletedChapterIDs *string
	Progress            *int
	IsCompleted         *bool
}

// UpsertProgress 创建或更新用户在某课程的学习进度。
func (CourseRepository) UpsertProgress(db *gorm.DB, userID, courseID int64, input UpsertProgressInput) (*models.CourseProgress, error) {
	progress, err := CourseRepo.FindProgress(db, userID, courseID)
	if err != nil {
		return nil, err
	}
	if progress == nil {
		// 新建进度记录。
		newProgress := models.CourseProgress{
			UserID:   userID,
			CourseID: courseID,
		}
		if input.CurrentChapterID != nil {
			newProgress.CurrentChapterID = input.CurrentChapterID
		}
		if input.CompletedChapterIDs != nil {
			newProgress.CompletedChapterIDs = input.CompletedChapterIDs
		}
		if input.Progress != nil {
			newProgress.Progress = *input.Progress
		}
		if input.IsCompleted != nil {
			newProgress.IsCompleted = *input.IsCompleted
		}
		if err := db.Create(&newProgress).Error; err != nil {
			return nil, err
		}
		return &newProgress, nil
	}
	// 更新已存在的进度。
	updates := map[string]interface{}{}
	if input.CurrentChapterID != nil {
		updates["current_chapter_id"] = *input.CurrentChapterID
	}
	if input.CompletedChapterIDs != nil {
		updates["completed_chapter_ids"] = *input.CompletedChapterIDs
	}
	if input.Progress != nil {
		updates["progress"] = *input.Progress
	}
	if input.IsCompleted != nil {
		updates["is_completed"] = *input.IsCompleted
	}
	if len(updates) > 0 {
		if err := db.Model(&models.CourseProgress{}).
			Where("user_id = ? AND course_id = ?", userID, courseID).
			Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return CourseRepo.FindProgress(db, userID, courseID)
}
