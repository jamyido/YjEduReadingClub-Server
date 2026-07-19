package repository

import (
	"gorm.io/gorm"

	"yjedu-reading-club-server/internal/models"
)

// MessageRepository 封装私信表的数据库操作。
type MessageRepository struct{}

// MessageRepo 是全局可用的私信仓储单例。
var MessageRepo = MessageRepository{}

// ConversationItem 是私信会话列表项。
type ConversationItem struct {
	OtherUserID   int64   `gorm:"column:other_user_id" json:"otherUserId"`
	OtherNickname string  `gorm:"column:other_nickname" json:"otherNickname"`
	OtherAvatar   *string `gorm:"column:other_avatar" json:"otherAvatar"`
	LastContent   string  `gorm:"column:last_content" json:"lastContent"`
	LastTime      string  `gorm:"column:last_time" json:"lastTime"`
	UnreadCount   int64   `gorm:"column:unread_count" json:"unreadCount"`
}

// FindMany 分页查询当前用户的私信会话列表。
// onlyUnread 为 true 时仅返回含未读消息的会话。
func (MessageRepository) FindMany(db *gorm.DB, userID int64, page, pageSize int, onlyUnread bool) ([]ConversationItem, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	// 以对方用户为维度聚合会话。
	baseQuery := db.Table("messages").
		Where("sender_id = ? OR receiver_id = ?", userID, userID)
	if onlyUnread {
		baseQuery = baseQuery.Where("receiver_id = ? AND is_read = ?", userID, false)
	}

	// 子查询：每个会话对方的 user_id。
	otherUserExpr := db.Table("messages").
		Select("CASE WHEN sender_id = ? THEN receiver_id ELSE sender_id END AS other_user_id", userID).
		Where("sender_id = ? OR receiver_id = ?", userID, userID)

	var total int64
	countSQL := `SELECT COUNT(DISTINCT other_user_id) FROM (?) AS t`
	if err := db.Raw(countSQL, otherUserExpr).Scan(&total).Error; err != nil {
		return nil, 0, err
	}

	conversationSQL := `
		SELECT
			t.other_user_id AS other_user_id,
			u.nickname AS other_nickname,
			u.avatar AS other_avatar,
			(SELECT content FROM messages WHERE sender_id = t.other_user_id OR receiver_id = t.other_user_id ORDER BY created_at DESC LIMIT 1) AS last_content,
			to_char((SELECT created_at FROM messages WHERE sender_id = t.other_user_id OR receiver_id = t.other_user_id ORDER BY created_at DESC LIMIT 1) AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS last_time,
			(SELECT COUNT(*) FROM messages WHERE receiver_id = ? AND sender_id = t.other_user_id AND is_read = ?) AS unread_count
		FROM (
			SELECT CASE WHEN sender_id = ? THEN receiver_id ELSE sender_id END AS other_user_id
			FROM messages
			WHERE sender_id = ? OR receiver_id = ?
			GROUP BY other_user_id
		) AS t
		INNER JOIN users u ON u.id = t.other_user_id
		ORDER BY last_time DESC
		LIMIT ? OFFSET ?`
	var list []ConversationItem
	if err := db.Raw(conversationSQL, userID, false, userID, userID, userID, pageSize, offset).Scan(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// FindConversation 分页查询与指定用户的私聊记录。
func (MessageRepository) FindConversation(db *gorm.DB, userID, otherUserID int64, page, pageSize int) ([]models.Message, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	query := db.Model(&models.Message{}).
		Where("(sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?)",
			userID, otherUserID, otherUserID, userID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []models.Message
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// MarkConversationRead 将对方发来的未读消息标记为已读。
func (MessageRepository) MarkConversationRead(db *gorm.DB, userID, otherUserID int64) error {
	return db.Model(&models.Message{}).
		Where("receiver_id = ? AND sender_id = ? AND is_read = ?", userID, otherUserID, false).
		Update("is_read", true).Error
}

// MarkAllRead 将当前用户所有未读私信标记为已读，返回受影响行数。
func (MessageRepository) MarkAllRead(db *gorm.DB, userID int64) (int64, error) {
	result := db.Model(&models.Message{}).
		Where("receiver_id = ? AND is_read = ?", userID, false).
		Update("is_read", true)
	return result.RowsAffected, result.Error
}

// CountUnread 统计当前用户未读私信数。
func (MessageRepository) CountUnread(db *gorm.DB, userID int64) (int64, error) {
	var count int64
	err := db.Model(&models.Message{}).
		Where("receiver_id = ? AND is_read = ?", userID, false).
		Count(&count).Error
	return count, err
}

// Send 发送私信。
func (MessageRepository) Send(db *gorm.DB, senderID, receiverID int64, messageType, content string) (*models.Message, error) {
	message := models.Message{
		SenderID:   senderID,
		ReceiverID: receiverID,
		Type:       messageType,
		Content:    content,
	}
	if err := db.Create(&message).Error; err != nil {
		return nil, err
	}
	return &message, nil
}
