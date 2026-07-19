package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"yjedu-reading-club-server/internal/models"
)

// PostRepository 封装帖子、帖子媒体、点赞的数据库操作。
type PostRepository struct{}

// PostRepo 是全局可用的帖子仓储单例。
var PostRepo = PostRepository{}

// PostDetail 是帖子详情查询结果，包含作者、圈子、媒体。
type PostDetail struct {
	models.Post
	Author  *UserBrief         `gorm:"foreignKey:AuthorID" json:"author"`
	Circle  *models.Circle     `gorm:"foreignKey:CircleID" json:"circle"`
	Topic   *models.Topic      `gorm:"foreignKey:TopicID" json:"topic"`
	Medias  []models.PostMedia `gorm:"foreignKey:PostID" json:"medias"`
	IsLiked bool               `gorm:"-" json:"isLiked"`
}

// PostListItem 是帖子列表项，包含作者、圈子、媒体。
type PostListItem struct {
	models.Post
	Author  *UserBrief         `gorm:"foreignKey:AuthorID" json:"author"`
	Circle  *models.Circle     `gorm:"foreignKey:CircleID" json:"circle"`
	Topic   *models.Topic      `gorm:"foreignKey:TopicID" json:"topic"`
	Medias  []models.PostMedia `gorm:"foreignKey:PostID" json:"medias"`
	IsLiked bool               `gorm:"-" json:"isLiked"`
}

// FindByID 根据主键查询帖子。
func (PostRepository) FindByID(db *gorm.DB, id int64) (*models.Post, error) {
	var post models.Post
	if err := db.First(&post, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &post, nil
}

// FindDetailByID 查询帖子详情，预加载作者、圈子、话题与媒体。
func (PostRepository) FindDetailByID(db *gorm.DB, id int64) (*PostDetail, error) {
	var post models.Post
	if err := db.First(&post, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	detail := &PostDetail{Post: post}
	// 作者仅取公开字段，避免泄露 phone 等敏感信息。
	detail.Author = LoadUserBrief(db, post.AuthorID)
	if post.CircleID != nil {
		var circle models.Circle
		if err := db.Select("id, name, cover").First(&circle, *post.CircleID).Error; err == nil {
			detail.Circle = &circle
		}
	}
	if post.TopicID != nil {
		var topic models.Topic
		if err := db.Select("id, slug, title").First(&topic, *post.TopicID).Error; err == nil {
			detail.Topic = &topic
		}
	}
	var medias []models.PostMedia
	if err := db.Where("post_id = ?", id).Order("sort ASC").Find(&medias).Error; err == nil {
		detail.Medias = medias
	}
	return detail, nil
}

// CreatePostMediaInput 是帖子媒体输入。
type CreatePostMediaInput struct {
	Type string
	URL  string
	Sort int
}

// CreatePostInput 是创建帖子的输入参数。
type CreatePostInput struct {
	AuthorID int64
	CircleID *int64
	TopicID  int64
	Type     string
	Title    *string
	Content  string
	LinkURL  *string
	Medias   []CreatePostMediaInput
}

// Create 在单个事务中创建帖子及其媒体资源。
func (PostRepository) Create(db *gorm.DB, input CreatePostInput) (*models.Post, error) {
	post := models.Post{
		AuthorID: input.AuthorID,
		CircleID: input.CircleID,
		TopicID:  &input.TopicID,
		Type:     input.Type,
		Title:    input.Title,
		Content:  input.Content,
		LinkURL:  input.LinkURL,
		Status:   models.PostStatusNormal,
	}
	if post.Type == "" {
		post.Type = models.PostTypeText
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&post).Error; err != nil {
			return err
		}
		for i, m := range input.Medias {
			media := models.PostMedia{
				PostID: post.ID,
				Type:   m.Type,
				URL:    m.URL,
				Sort:   m.Sort,
			}
			if media.Sort == 0 {
				media.Sort = i
			}
			if err := tx.Create(&media).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &post, nil
}

// SoftDelete 软删除帖子，将状态置为 1。
func (PostRepository) SoftDelete(db *gorm.DB, postID int64) error {
	return db.Model(&models.Post{}).Where("id = ?", postID).
		Update("status", models.PostStatusDeleted).Error
}

// UpdatePostInput 是更新帖子的输入参数。
// 仅允许修改标题、正文、链接、类型与媒体列表；作者、圈子、话题均不可变更，
// 以避免触发打卡副作用与圈子帖子计数错乱。
type UpdatePostInput struct {
	Title   *string
	Content string
	LinkURL *string
	Type    string
	Medias  []CreatePostMediaInput
}

// Update 在单个事务中更新帖子字段并全量替换媒体资源。
// 旧媒体先按 post_id 物理删除，再按 input.Medias 顺序重新写入，保证排序与数量一致。
// 返回最新帖子记录（不含媒体），调用方如需详情应再调用 FindDetailByID。
func (PostRepository) Update(db *gorm.DB, postID int64, input UpdatePostInput) (*models.Post, error) {
	postType := input.Type
	if postType == "" {
		postType = models.PostTypeText
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		// 仅更新业务字段，避免覆盖 status / 计数 / created_at 等敏感列。
		updates := map[string]interface{}{
			"title":   input.Title,
			"content": input.Content,
			"link_url": input.LinkURL,
			"type":    postType,
		}
		if err := tx.Model(&models.Post{}).Where("id = ?", postID).
			Updates(updates).Error; err != nil {
			return err
		}
		// 全量替换媒体：先删旧后建新，与 Create 行为保持一致。
		if err := tx.Where("post_id = ?", postID).
			Delete(&models.PostMedia{}).Error; err != nil {
			return err
		}
		for i, m := range input.Medias {
			media := models.PostMedia{
				PostID: postID,
				Type:   m.Type,
				URL:    m.URL,
				Sort:   m.Sort,
			}
			if media.Sort == 0 {
				media.Sort = i
			}
			if err := tx.Create(&media).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// 重新读取最新记录，确保返回值与数据库一致。
	var updated models.Post
	if err := db.First(&updated, postID).Error; err != nil {
		return nil, err
	}
	return &updated, nil
}

// PostListOptions 是帖子列表查询选项。
type PostListOptions struct {
	Page     int
	PageSize int
	CircleID int64
	AuthorID int64
	TopicID  int64
	Type     string
	Status   int
}

// FindMany 分页查询帖子列表，按置顶与创建时间排序。
func (PostRepository) FindMany(db *gorm.DB, opts PostListOptions) ([]PostListItem, int64, error) {
	if opts.Status == 0 {
		opts.Status = models.PostStatusNormal
	}
	query := db.Model(&models.Post{}).Where("status = ?", opts.Status)
	if opts.CircleID != 0 {
		query = query.Where("circle_id = ?", opts.CircleID)
	}
	if opts.AuthorID != 0 {
		query = query.Where("author_id = ?", opts.AuthorID)
	}
	if opts.TopicID != 0 {
		query = query.Where("topic_id = ?", opts.TopicID)
	}
	if opts.Type != "" {
		query = query.Where("type = ?", opts.Type)
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
	var posts []models.Post
	if err := query.Order("is_pinned DESC, created_at DESC").
		Offset(offset).Limit(opts.PageSize).Find(&posts).Error; err != nil {
		return nil, 0, err
	}
	list := make([]PostListItem, 0, len(posts))
	for i := range posts {
		item := PostListItem{Post: posts[i]}
		item.Author = LoadUserBrief(db, posts[i].AuthorID)
		if posts[i].CircleID != nil {
			var circle models.Circle
			if err := db.Select("id, name, cover").First(&circle, *posts[i].CircleID).Error; err == nil {
				item.Circle = &circle
			}
		}
		if posts[i].TopicID != nil {
			var topic models.Topic
			if err := db.Select("id, slug, title").First(&topic, *posts[i].TopicID).Error; err == nil {
				item.Topic = &topic
			}
		}
		var medias []models.PostMedia
		if err := db.Where("post_id = ?", posts[i].ID).Order("sort ASC").Find(&medias).Error; err == nil {
			item.Medias = medias
		}
		list = append(list, item)
	}
	return list, total, nil
}

// PostDaysRankingRecord 是圈子累计发帖天数排行榜记录。
type PostDaysRankingRecord struct {
	UserID             int64  `gorm:"column:user_id" json:"userId"`
	Nickname           string `gorm:"column:nickname" json:"nickname"`
	Avatar             *string `gorm:"column:avatar" json:"avatar"`
	CumulativePostDays int64  `gorm:"column:cumulative_post_days" json:"cumulativePostDays"`
	PostCount          int64  `gorm:"column:post_count" json:"postCount"`
	LastPostAt         string `gorm:"column:last_post_at" json:"lastPostAt"`
}

// FindCirclePostDaysRanking 分页查询圈子累计发帖天数排行榜。
// 正常帖子按 Asia/Shanghai 自然日去重，同一天发布多帖只累计一天；
// 同分时依次按最近发帖时间降序、用户 ID 升序形成稳定名次。
func (PostRepository) FindCirclePostDaysRanking(db *gorm.DB, circleID int64, page, pageSize int) ([]PostDaysRankingRecord, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize

	rankingSQL := `
		SELECT
			users.id AS user_id,
			users.nickname AS nickname,
			users.avatar AS avatar,
			ranking.cumulative_post_days AS cumulative_post_days,
			ranking.post_count AS post_count,
			to_char(ranking.last_post_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS last_post_at
		FROM (
			SELECT
				posts.author_id AS author_id,
				COUNT(DISTINCT DATE(posts.created_at AT TIME ZONE 'Asia/Shanghai')) AS cumulative_post_days,
				COUNT(*) AS post_count,
				MAX(posts.created_at) AS last_post_at
			FROM posts
			WHERE posts.circle_id = ? AND posts.status = 0
			GROUP BY posts.author_id
		) AS ranking
		INNER JOIN users ON users.id = ranking.author_id
		ORDER BY ranking.cumulative_post_days DESC, ranking.last_post_at DESC, users.id ASC
		LIMIT ? OFFSET ?`
	var list []PostDaysRankingRecord
	if err := db.Raw(rankingSQL, circleID, pageSize, offset).Scan(&list).Error; err != nil {
		return nil, 0, err
	}

	var total int64
	countSQL := `SELECT COUNT(DISTINCT posts.author_id) FROM posts WHERE posts.circle_id = ? AND posts.status = 0`
	if err := db.Raw(countSQL, circleID).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// IncrementLikeCount 调整帖子点赞数，delta 可为负。
func (PostRepository) IncrementLikeCount(db *gorm.DB, postID int64, delta int) error {
	return db.Model(&models.Post{}).Where("id = ?", postID).
		UpdateColumn("like_count", gorm.Expr("like_count + ?", delta)).Error
}

// IncrementCommentCount 调整帖子评论数，delta 可为负。
func (PostRepository) IncrementCommentCount(db *gorm.DB, postID int64, delta int) error {
	return db.Model(&models.Post{}).Where("id = ?", postID).
		UpdateColumn("comment_count", gorm.Expr("comment_count + ?", delta)).Error
}

// IncrementShareCount 调整帖子转发计数，delta 可为负。
func (PostRepository) IncrementShareCount(db *gorm.DB, postID int64, delta int) error {
	return db.Model(&models.Post{}).Where("id = ?", postID).
		UpdateColumn("share_count", gorm.Expr("share_count + ?", delta)).Error
}

// FindLikedPostIDs 批量查询用户已点赞的帖子 ID。
func (PostRepository) FindLikedPostIDs(db *gorm.DB, userID int64, postIDs []int64) ([]int64, error) {
	if len(postIDs) == 0 {
		return nil, nil
	}
	var ids []int64
	err := db.Model(&models.Like{}).
		Where("user_id = ? AND target_type = ? AND target_id IN ?", userID, models.LikeTargetPost, postIDs).
		Pluck("target_id", &ids).Error
	return ids, err
}

// FindLike 查询用户是否点赞指定目标。
func (PostRepository) FindLike(db *gorm.DB, userID int64, targetType string, targetID int64) (*models.Like, error) {
	var like models.Like
	err := db.Where("user_id = ? AND target_type = ? AND target_id = ?", userID, targetType, targetID).
		First(&like).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &like, nil
}

// CreateLike 创建点赞记录。
func (PostRepository) CreateLike(db *gorm.DB, userID int64, targetType string, targetID int64) error {
	like := models.Like{
		UserID:     userID,
		TargetType: targetType,
		TargetID:   targetID,
	}
	return db.Create(&like).Error
}

// DeleteLike 删除点赞记录。
func (PostRepository) DeleteLike(db *gorm.DB, userID int64, targetType string, targetID int64) error {
	return db.Where("user_id = ? AND target_type = ? AND target_id = ?", userID, targetType, targetID).
		Delete(&models.Like{}).Error
}

// CirclePostDigestItem 是圈子时段总结帖查询的扁平结果。
// 仅包含 AI 总结所需的字段：作者昵称、帖子标题与正文片段、创建时间。
type CirclePostDigestItem struct {
	ID        int64     `gorm:"column:id" json:"id"`
	AuthorID  int64     `gorm:"column:author_id" json:"authorId"`
	Nickname  string    `gorm:"column:nickname" json:"nickname"`
	Title     *string   `gorm:"column:title" json:"title"`
	Content   string    `gorm:"column:content" json:"content"`
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
}

// FindCirclePostsByTimeRange 查询指定圈子在某时段内的正常帖子，排除指定作者（用于排除 AI伴读 自身发的总结帖）。
// 按创建时间升序返回，最多返回 maxItems 条，供 AI 总结使用。
func (PostRepository) FindCirclePostsByTimeRange(db *gorm.DB, circleID int64, startTime, endTime time.Time, excludeAuthorID int64, maxItems int) ([]CirclePostDigestItem, error) {
	if maxItems <= 0 {
		maxItems = 50
	}
	query := db.Table("posts").
		Select("posts.id, posts.author_id, users.nickname, posts.title, posts.content, posts.created_at").
		Joins("INNER JOIN users ON users.id = posts.author_id").
		Where("posts.circle_id = ? AND posts.status = ? AND posts.created_at >= ? AND posts.created_at < ?",
			circleID, models.PostStatusNormal, startTime, endTime)
	if excludeAuthorID > 0 {
		query = query.Where("posts.author_id != ?", excludeAuthorID)
	}
	var items []CirclePostDigestItem
	if err := query.Order("posts.created_at ASC").Limit(maxItems).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// UnpinPostsByCircleAndTopic 取消指定圈子中指定话题下所有已置顶帖子的置顶状态。
// 用于在创建新的 AI 总结帖前，移除旧 AI 总结帖的置顶，保证每个圈子每个话题仅有最新一条置顶。
func (PostRepository) UnpinPostsByCircleAndTopic(db *gorm.DB, circleID int64, topicID int64) error {
	return db.Model(&models.Post{}).
		Where("circle_id = ? AND topic_id = ? AND is_pinned = ?", circleID, topicID, true).
		Update("is_pinned", false).Error
}

// CreatePinnedPostInput 是创建置顶帖的输入参数。
// 供 AI伴读 系统用户发布日/周/月精华总结帖使用，绕过 PostService 的成员与 AdminOnly 校验。
type CreatePinnedPostInput struct {
	AuthorID int64
	CircleID int64
	TopicID  int64
	Title    string
	Content  string
}

// CreatePinnedPost 创建一条置顶帖子。
// 调用方应事先调用 UnpinPostsByCircleAndTopic 取消同圈子同话题的旧置顶帖。
func (PostRepository) CreatePinnedPost(db *gorm.DB, input CreatePinnedPostInput) (*models.Post, error) {
	post := models.Post{
		AuthorID: input.AuthorID,
		CircleID: &input.CircleID,
		TopicID:  &input.TopicID,
		Type:     models.PostTypeText,
		Title:    &input.Title,
		Content:  input.Content,
		Status:   models.PostStatusNormal,
		IsPinned: true,
	}
	if err := db.Create(&post).Error; err != nil {
		return nil, err
	}
	return &post, nil
}
