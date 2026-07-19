package repository

import (
	"errors"

	"gorm.io/gorm"

	"yjedu-reading-club-server/internal/models"
)

// CheckInRepository 封装打卡记录表的数据库操作。
type CheckInRepository struct{}

// CheckInRepo 是全局可用的打卡仓储单例。
var CheckInRepo = CheckInRepository{}

// FindByUserAndDate 查询用户某业务日期的打卡记录。
func (CheckInRepository) FindByUserAndDate(db *gorm.DB, userID int64, checkInDate string) (*models.CheckIn, error) {
	var checkIn models.CheckIn
	err := db.Where("user_id = ? AND check_in_date = ?", userID, checkInDate).First(&checkIn).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &checkIn, nil
}

// CreateCheckInInput 是创建打卡记录的输入参数。
type CreateCheckInInput struct {
	UserID      int64
	PostID      *int64
	CheckInDate string
	CircleID    *int64
	Content     *string
	Images      *string
}

// Create 创建打卡记录。
func (CheckInRepository) Create(db *gorm.DB, input CreateCheckInInput) (*models.CheckIn, error) {
	checkIn := models.CheckIn{
		UserID:      input.UserID,
		PostID:      input.PostID,
		CheckInDate: &input.CheckInDate,
		CircleID:    input.CircleID,
		Content:     input.Content,
		Images:      input.Images,
	}
	if err := db.Create(&checkIn).Error; err != nil {
		return nil, err
	}
	return &checkIn, nil
}

// CheckInListOptions 是打卡记录列表查询选项。
type CheckInListOptions struct {
	Page     int
	PageSize int
	UserID   int64
	CircleID int64
}

// CheckInWithUser 是打卡记录列表项，附带用户精简信息。
type CheckInWithUser struct {
	models.CheckIn
	User *UserBrief `gorm:"foreignKey:UserID" json:"user"`
}

// FindMany 分页查询打卡记录，附带用户精简信息。
func (CheckInRepository) FindMany(db *gorm.DB, opts CheckInListOptions) ([]CheckInWithUser, int64, error) {
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PageSize < 1 {
		opts.PageSize = 20
	}
	offset := (opts.Page - 1) * opts.PageSize
	query := db.Model(&models.CheckIn{})
	if opts.UserID != 0 {
		query = query.Where("user_id = ?", opts.UserID)
	}
	if opts.CircleID != 0 {
		query = query.Where("circle_id = ?", opts.CircleID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var records []models.CheckIn
	if err := query.Order("created_at DESC").Offset(offset).Limit(opts.PageSize).Find(&records).Error; err != nil {
		return nil, 0, err
	}
	list := make([]CheckInWithUser, 0, len(records))
	for i := range records {
		item := CheckInWithUser{CheckIn: records[i]}
		item.User = LoadUserBrief(db, records[i].UserID)
		list = append(list, item)
	}
	return list, total, nil
}
