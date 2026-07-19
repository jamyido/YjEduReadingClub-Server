package repository

import (
	"errors"

	"gorm.io/gorm"

	"yjedu-reading-club-server/internal/models"
)

// CommentRepository 封装评论表的数据库操作。
type CommentRepository struct{}

// CommentRepo 是全局可用的评论仓储单例。
var CommentRepo = CommentRepository{}

// CommentWithAuthor 是评论查询结果，包含作者信息。
type CommentWithAuthor struct {
	models.Comment
	Author *UserBrief `gorm:"foreignKey:AuthorID" json:"author"`
}

// FindByID 根据主键查询评论。
func (CommentRepository) FindByID(db *gorm.DB, id int64) (*models.Comment, error) {
	var comment models.Comment
	if err := db.First(&comment, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &comment, nil
}

// FindByPost 分页查询帖子的一级评论，包含作者信息。
func (CommentRepository) FindByPost(db *gorm.DB, postID int64, page, pageSize int) ([]CommentWithAuthor, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	query := db.Model(&models.Comment{}).
		Where("post_id = ? AND parent_id IS NULL AND status = ?", postID, models.CommentStatusNormal)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var comments []models.Comment
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&comments).Error; err != nil {
		return nil, 0, err
	}
	list := make([]CommentWithAuthor, 0, len(comments))
	for i := range comments {
		item := CommentWithAuthor{Comment: comments[i]}
		item.Author = LoadUserBrief(db, comments[i].AuthorID)
		list = append(list, item)
	}
	return list, total, nil
}

// CreateCommentInput 是创建评论的输入参数。
type CreateCommentInput struct {
	PostID     int64
	AuthorID   int64
	Content    string
	ParentID   *int64
	ReplyToID  *int64
}

// Create 创建评论。
func (CommentRepository) Create(db *gorm.DB, input CreateCommentInput) (*models.Comment, error) {
	comment := models.Comment{
		PostID:    input.PostID,
		AuthorID:  input.AuthorID,
		Content:   input.Content,
		ParentID:  input.ParentID,
		ReplyToID: input.ReplyToID,
		Status:    models.CommentStatusNormal,
	}
	if err := db.Create(&comment).Error; err != nil {
		return nil, err
	}
	return &comment, nil
}

// IsActiveThreadParticipant 判断目标用户是否是帖子评论线程的活跃参与者。
// 用于二级回复时校验 replyToId 的合法性。
func (CommentRepository) IsActiveThreadParticipant(db *gorm.DB, postID, replyToUserID int64) (bool, error) {
	var count int64
	err := db.Model(&models.Comment{}).
		Where("post_id = ? AND author_id = ? AND status = ?", postID, replyToUserID, models.CommentStatusNormal).
		Count(&count).Error
	return count > 0, err
}
