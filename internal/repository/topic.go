package repository

import (
	"errors"

	"gorm.io/gorm"

	"yjedu-reading-club-server/internal/models"
)

// TopicRepository 封装话题表的数据库操作。
type TopicRepository struct{}

// TopicRepo 是全局可用的话题仓储单例。
var TopicRepo = TopicRepository{}

// TopicListItem 是话题列表项，附带帖子数。
type TopicListItem struct {
	models.Topic
	PostCount int64 `gorm:"column:post_count" json:"postCount"`
}

// FindByID 根据主键查询话题。
func (TopicRepository) FindByID(db *gorm.DB, id int64) (*models.Topic, error) {
	var topic models.Topic
	if err := db.First(&topic, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &topic, nil
}

// FindBySlug 根据 slug 查询话题。
func (TopicRepository) FindBySlug(db *gorm.DB, slug string) (*models.Topic, error) {
	var topic models.Topic
	if err := db.Where("slug = ?", slug).First(&topic).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &topic, nil
}

// TopicListOptions 是话题列表查询选项。
type TopicListOptions struct {
	Page     int
	PageSize int
	Query    string
}

// FindMany 分页查询启用状态话题，附带各话题下的帖子数。
func (TopicRepository) FindMany(db *gorm.DB, opts TopicListOptions) ([]TopicListItem, int64, error) {
	query := db.Model(&models.Topic{}).Where("status = ?", models.TopicStatusEnabled)
	if opts.Query != "" {
		like := "%" + opts.Query + "%"
		query = query.Where("title LIKE ? OR description LIKE ?", like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PageSize < 1 {
		opts.PageSize = 20
	}
	offset := (opts.Page - 1) * opts.PageSize
	var list []TopicListItem
	err := db.Table("topics").
		Select("topics.*, (SELECT COUNT(*) FROM posts WHERE posts.topic_id = topics.id AND posts.status = 0) AS post_count").
		Where("topics.status = ?", models.TopicStatusEnabled).
		Order("topics.sort DESC, topics.created_at DESC").
		Offset(offset).Limit(opts.PageSize).
		Find(&list).Error
	return list, total, err
}
