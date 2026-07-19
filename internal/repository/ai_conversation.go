package repository

import (
	"errors"

	"gorm.io/gorm"

	"yjedu-reading-club-server/internal/models"
)

// AIConversationRepository 封装 AI 共读会话与消息的数据库操作。
type AIConversationRepository struct{}

// AIConversationRepo 是全局可用的 AI 会话仓储单例。
var AIConversationRepo = AIConversationRepository{}

// AIConversationListOptions 是会话列表查询选项。
type AIConversationListOptions struct {
	Page     int
	PageSize int
}

// AIConversationListItem 是会话列表项，附带帖子摘要。
type AIConversationListItem struct {
	models.AIConversation
	PostTitle string `gorm:"column:post_title" json:"postTitle"`
}

// FindOrCreateByUserPost 查询或创建当前用户对指定帖子的 AI 共读会话。
// 若已存在则返回已有会话，否则创建新会话。
func (AIConversationRepository) FindOrCreateByUserPost(db *gorm.DB, userID, postID int64, title string) (*models.AIConversation, bool, error) {
	var conv models.AIConversation
	err := db.Where("user_id = ? AND post_id = ?", userID, postID).First(&conv).Error
	if err == nil {
		return &conv, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}

	// 创建新会话。
	conv = models.AIConversation{
		UserID: userID,
		PostID: postID,
		Title:  title,
		Status: models.AIConversationStatusNormal,
	}
	if err := db.Create(&conv).Error; err != nil {
		// 并发场景下可能因唯一索引冲突而创建失败，再次查询。
		var existing models.AIConversation
		if err2 := db.Where("user_id = ? AND post_id = ?", userID, postID).First(&existing).Error; err2 == nil {
			return &existing, false, nil
		}
		return nil, false, err
	}
	return &conv, true, nil
}

// FindByID 根据主键查询会话。
func (AIConversationRepository) FindByID(db *gorm.DB, id int64) (*models.AIConversation, error) {
	var conv models.AIConversation
	if err := db.First(&conv, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &conv, nil
}

// FindMany 分页查询当前用户的会话列表，附带帖子标题。
func (AIConversationRepository) FindMany(db *gorm.DB, userID int64, opts AIConversationListOptions) ([]AIConversationListItem, int64, error) {
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PageSize < 1 {
		opts.PageSize = 20
	}
	offset := (opts.Page - 1) * opts.PageSize

	query := db.Table("ai_conversations").
		Select("ai_conversations.*, posts.title AS post_title").
		Joins("LEFT JOIN posts ON posts.id = ai_conversations.post_id").
		Where("ai_conversations.user_id = ?", userID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []AIConversationListItem
	if err := query.Order("ai_conversations.updated_at DESC").
		Offset(offset).Limit(opts.PageSize).
		Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// FindMessages 查询会话内全部消息，按时间升序。
func (AIConversationRepository) FindMessages(db *gorm.DB, conversationID int64) ([]models.AIMessage, error) {
	var messages []models.AIMessage
	err := db.Where("conversation_id = ?", conversationID).
		Order("created_at ASC").
		Find(&messages).Error
	if err != nil {
		return nil, err
	}
	return messages, nil
}

// CreateMessage 创建单条 AI 消息。
func (AIConversationRepository) CreateMessage(db *gorm.DB, conversationID int64, role, content string) (*models.AIMessage, error) {
	msg := models.AIMessage{
		ConversationID: conversationID,
		Role:           role,
		Content:        content,
	}
	if err := db.Create(&msg).Error; err != nil {
		return nil, err
	}
	// 更新会话的 updated_at，便于列表按最近活跃排序。
	db.Model(&models.AIConversation{}).Where("id = ?", conversationID).
		UpdateColumn("updated_at", gorm.Expr("CURRENT_TIMESTAMP"))
	return &msg, nil
}
