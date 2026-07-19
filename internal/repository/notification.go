package repository

import (
	"gorm.io/gorm"

	"yjedu-reading-club-server/internal/models"
)

// NotificationRepository 封装通知表的数据库操作。
type NotificationRepository struct{}

// NotificationRepo 是全局可用的通知仓储单例。
var NotificationRepo = NotificationRepository{}

// NotificationListOptions 是通知列表查询选项。
type NotificationListOptions struct {
	Page       int
	PageSize   int
	OnlyUnread bool
	Category   string
}

// FindMany 分页查询当前用户的通知列表。
// category 可取 like/reply/system/task，分别映射到不同通知类型。
func (NotificationRepository) FindMany(db *gorm.DB, userID int64, opts NotificationListOptions) ([]models.Notification, int64, error) {
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PageSize < 1 {
		opts.PageSize = 20
	}
	offset := (opts.Page - 1) * opts.PageSize
	query := db.Model(&models.Notification{}).Where("user_id = ?", userID)
	if opts.OnlyUnread {
		query = query.Where("is_read = ?", false)
	}
	switch opts.Category {
	case "like":
		query = query.Where("type = ?", models.NotificationTypeLike)
	case "reply":
		query = query.Where("type = ?", models.NotificationTypeComment)
	case "system":
		query = query.Where("type = ?", models.NotificationTypeSystem)
	case "task":
		query = query.Where("type = ?", models.NotificationTypeTask)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []models.Notification
	if err := query.Order("created_at DESC").Offset(offset).Limit(opts.PageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// CreateNotificationInput 是创建单条通知的输入参数。
type CreateNotificationInput struct {
	UserID     int64
	Type       string
	ActorID    *int64
	TargetType *string
	TargetID   *int64
	Title      string
	Content    *string
}

// Create 创建单条通知。
func (NotificationRepository) Create(db *gorm.DB, input CreateNotificationInput) (*models.Notification, error) {
	notification := models.Notification{
		UserID:     input.UserID,
		Type:       input.Type,
		ActorID:    input.ActorID,
		TargetType: input.TargetType,
		TargetID:   input.TargetID,
		Title:      input.Title,
		Content:    input.Content,
	}
	if err := db.Create(&notification).Error; err != nil {
		return nil, err
	}
	return &notification, nil
}

// CreateMany 批量派发通知给多个用户。
func (NotificationRepository) CreateMany(db *gorm.DB, userIDs []int64, input CreateNotificationInput) error {
	if len(userIDs) == 0 {
		return nil
	}
	notifications := make([]models.Notification, 0, len(userIDs))
	for i := range userIDs {
		notifications = append(notifications, models.Notification{
			UserID:     userIDs[i],
			Type:       input.Type,
			ActorID:    input.ActorID,
			TargetType: input.TargetType,
			TargetID:   input.TargetID,
			Title:      input.Title,
			Content:    input.Content,
		})
	}
	return db.CreateInBatches(notifications, 500).Error
}

// MarkRead 标记单条通知为已读，仅当通知属于该用户时生效。
func (NotificationRepository) MarkRead(db *gorm.DB, notificationID, userID int64) (*models.Notification, error) {
	result := db.Model(&models.Notification{}).
		Where("id = ? AND user_id = ?", notificationID, userID).
		Update("is_read", true)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	var notification models.Notification
	if err := db.First(&notification, notificationID).Error; err != nil {
		return nil, err
	}
	return &notification, nil
}

// MarkAllRead 标记当前用户所有未读通知为已读，返回受影响行数。
func (NotificationRepository) MarkAllRead(db *gorm.DB, userID int64) (int64, error) {
	result := db.Model(&models.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Update("is_read", true)
	return result.RowsAffected, result.Error
}

// CountAll 统计当前用户通知总数。
func (NotificationRepository) CountAll(db *gorm.DB, userID int64) (int64, error) {
	var count int64
	err := db.Model(&models.Notification{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

// CountUnread 统计当前用户未读通知数。
func (NotificationRepository) CountUnread(db *gorm.DB, userID int64) (int64, error) {
	var count int64
	err := db.Model(&models.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).Count(&count).Error
	return count, err
}

// UnreadCountByType 按通知类型统计未读数。
// 返回 type -> 未读数的映射。
func (NotificationRepository) CountUnreadByType(db *gorm.DB, userID int64) (map[string]int64, error) {
	type row struct {
		Type  string `gorm:"column:type"`
		Count int64  `gorm:"column:count"`
	}
	var rows []row
	err := db.Model(&models.Notification{}).
		Select("type, COUNT(*) AS count").
		Where("user_id = ? AND is_read = ?", userID, false).
		Group("type").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]int64, len(rows))
	for i := range rows {
		result[rows[i].Type] = rows[i].Count
	}
	return result, nil
}
