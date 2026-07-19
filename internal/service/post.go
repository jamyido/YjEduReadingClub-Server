// Package service 封装跨多个仓储的领域业务逻辑。
// 当前包含发帖领域服务，它在单个事务中完成帖子创建与打卡副作用。
package service

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"yjedu-reading-club-server/internal/database"
	"yjedu-reading-club-server/internal/models"
	"yjedu-reading-club-server/internal/pkg/businessdate"
	"yjedu-reading-club-server/internal/repository"
)

// maxTransactionAttempts 是并发冲突时事务最大尝试次数。
const maxTransactionAttempts = 3

// PostServiceMediaInput 是帖子媒体输入。
type PostServiceMediaInput struct {
	Type string
	URL  string
	Sort int
}

// CreatePostServiceInput 是发帖事务输入。
type CreatePostServiceInput struct {
	AuthorID          int64
	AuthorRole        string // 作者角色：USER / ADMIN，用于校验 AdminOnly 话题权限
	CircleID          *int64
	TopicID           *int64
	Type              string
	Title             *string
	Content           string
	LinkURL           *string
	Medias            []PostServiceMediaInput
	RequireNewCheckIn bool
}

// CreatePostServiceResult 是发帖事务结果。
type CreatePostServiceResult struct {
	Post       *models.Post
	CheckIn    *models.CheckIn
	StreakDays int
}

// PostServiceError 是发帖事务的可预期领域错误。
// API 层可依据 Code/StatusCode 返回稳定错误，而不是转换为 500。
type PostServiceError struct {
	Code       string
	Message    string
	StatusCode int
}

// Error 实现 error 接口。
func (e *PostServiceError) Error() string { return e.Message }

// 发帖事务领域错误代码。
const (
	CodeTopicNotFound       = "TOPIC_NOT_FOUND"
	CodeDefaultTopicMissing = "DEFAULT_TOPIC_MISSING"
	CodeCircleNotFound      = "CIRCLE_NOT_FOUND"
	CodeNotCircleMember     = "NOT_CIRCLE_MEMBER"
	CodeCheckInTopicReq     = "CHECK_IN_TOPIC_REQUIRED"
	CodeAlreadyCheckedIn    = "ALREADY_CHECKED_IN"
	CodeTopicAdminOnly      = "TOPIC_ADMIN_ONLY"
)

// PostService 是发帖领域服务。
type PostService struct{}

// PostSvc 是全局可用的发帖服务单例。
var PostSvc = PostService{}

// isRetryableError 判断错误是否适合重试。
// PostgreSQL SQLSTATE：23505（唯一键冲突）、40P01（序列化失败/死锁）、55P03（锁等待超时）均触发重试。
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	// gorm 翻译后的唯一键冲突。
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	// 底层 PostgreSQL 错误码判断（pgconn.PgError.Code 即 SQLSTATE）。
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505", "40P01", "55P03":
			return true
		}
	}
	return false
}

// serializeCheckInImages 将帖子图片整理为 CheckIn.images 使用的 JSON 字符串。
// 没有图片时返回空字符串。
func serializeCheckInImages(medias []PostServiceMediaInput) string {
	if len(medias) == 0 {
		return ""
	}
	urls := make([]string, 0, len(medias))
	for i := range medias {
		if medias[i].Type != "video" {
			urls = append(urls, medias[i].URL)
		}
	}
	if len(urls) == 0 {
		return ""
	}
	// 手工拼接 JSON 数组字符串，避免引入 encoding/json 依赖。
	var b strings.Builder
	b.WriteByte('[')
	for i, u := range urls {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('"')
		b.WriteString(u)
		b.WriteByte('"')
	}
	b.WriteByte(']')
	return b.String()
}

// runCreateTransaction 在单个事务中创建帖子并完成所有计数与打卡副作用。
func (PostService) runCreateTransaction(input CreatePostServiceInput) (*CreatePostServiceResult, error) {
	db := database.Get()
	result := &CreatePostServiceResult{}

	err := db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()

		// 解析话题：指定 topicId 则按 ID 查询，否则取默认打卡话题。
		var topic *models.Topic
		var topicErr error
		if input.TopicID != nil {
			topic, topicErr = repository.TopicRepo.FindByID(tx, *input.TopicID)
		} else {
			topic, topicErr = repository.TopicRepo.FindBySlug(tx, businessdate.CheckInTopicSlug)
		}
		if topicErr != nil {
			return topicErr
		}
		if topic == nil || topic.Status != models.TopicStatusEnabled {
			if input.TopicID != nil {
				return &PostServiceError{Code: CodeTopicNotFound, Message: "话题不存在或已停用", StatusCode: 400}
			}
			return &PostServiceError{Code: CodeDefaultTopicMissing, Message: "系统默认打卡话题尚未配置", StatusCode: 500}
		}

		// 旧打卡接口要求必须使用打卡挑战话题。
		if input.RequireNewCheckIn && topic.Slug != businessdate.CheckInTopicSlug {
			return &PostServiceError{Code: CodeCheckInTopicReq, Message: "打卡必须使用打卡挑战话题", StatusCode: 400}
		}

		// 圈子存在性与成员校验，同时记录是否为该圈子圈主（圈主可使用 AdminOnly 话题）。
		isCircleOwner := false
		if input.CircleID != nil {
			circle, err := repository.CircleRepo.FindByID(tx, *input.CircleID)
			if err != nil {
				return err
			}
			if circle == nil {
				return &PostServiceError{Code: CodeCircleNotFound, Message: "圈子不存在", StatusCode: 404}
			}
			membership, err := repository.CircleRepo.FindMembership(tx, input.AuthorID, *input.CircleID)
			if err != nil {
				return err
			}
			if membership == nil {
				return &PostServiceError{Code: CodeNotCircleMember, Message: "加入圈子后才能在该圈子发帖", StatusCode: 403}
			}
			if membership.Role == models.CircleRoleOwner {
				isCircleOwner = true
			}
		}

		// AdminOnly 话题（本周精读、本月精读等）仅管理员或所发帖圈子的圈主可选，普通用户禁止使用。
		if topic.AdminOnly && input.AuthorRole != models.RoleAdmin && !isCircleOwner {
			return &PostServiceError{Code: CodeTopicAdminOnly, Message: "该话题仅管理员或圈主可发布", StatusCode: 403}
		}

		businessDate := businessdate.ToShanghaiDateKey(now)
		previousBusinessDate := businessdate.GetPreviousShanghaiDateKey(now)

		// 旧打卡接口要求当天首次打卡。
		if input.RequireNewCheckIn {
			existingCheckIn, err := repository.CheckInRepo.FindByUserAndDate(tx, input.AuthorID, businessDate)
			if err != nil {
				return err
			}
			currentUser, err := repository.UserRepo.FindByID(tx, input.AuthorID)
			if err != nil {
				return err
			}
			lastDate := ""
			if currentUser != nil && currentUser.LastCheckInAt != nil {
				lastDate = businessdate.ToShanghaiDateKey(*currentUser.LastCheckInAt)
			}
			if existingCheckIn != nil || lastDate == businessDate {
				return &PostServiceError{Code: CodeAlreadyCheckedIn, Message: "今日已打卡，请明天再来", StatusCode: 409}
			}
		}

		// 创建帖子及媒体。
		medias := make([]repository.CreatePostMediaInput, 0, len(input.Medias))
		for i := range input.Medias {
			medias = append(medias, repository.CreatePostMediaInput{
				Type: input.Medias[i].Type,
				URL:  input.Medias[i].URL,
				Sort: input.Medias[i].Sort,
			})
		}
		post, err := repository.PostRepo.Create(tx, repository.CreatePostInput{
			AuthorID: input.AuthorID,
			CircleID: input.CircleID,
			TopicID:  topic.ID,
			Type:     input.Type,
			Title:    input.Title,
			Content:  input.Content,
			LinkURL:  input.LinkURL,
			Medias:   medias,
		})
		if err != nil {
			return err
		}
		result.Post = post

		// 圈子帖子计数 +1。
		if input.CircleID != nil {
			if err := repository.CircleRepo.IncrementPostCount(tx, *input.CircleID, 1); err != nil {
				return err
			}
		}

		// 打卡挑战话题：同步写入打卡记录并更新连续天数。
		if topic.Slug == businessdate.CheckInTopicSlug {
			currentUser, err := repository.UserRepo.FindByID(tx, input.AuthorID)
			if err != nil {
				return err
			}
			existingCheckIn, err := repository.CheckInRepo.FindByUserAndDate(tx, input.AuthorID, businessDate)
			if err != nil {
				return err
			}
			lastDate := ""
			if currentUser != nil && currentUser.LastCheckInAt != nil {
				lastDate = businessdate.ToShanghaiDateKey(*currentUser.LastCheckInAt)
			}
			// lastCheckInAt 的同日判断兼容迁移当天尚无 checkInDate 的旧打卡记录。
			if existingCheckIn == nil && lastDate != businessDate {
				streakDays := 1
				if lastDate == previousBusinessDate && currentUser != nil {
					streakDays = currentUser.StreakDays + 1
				}
				images := serializeCheckInImages(input.Medias)
				var imagesPtr *string
				if images != "" {
					imagesPtr = &images
				}
				contentPtr := &input.Content
				checkIn, err := repository.CheckInRepo.Create(tx, repository.CreateCheckInInput{
					UserID:      input.AuthorID,
					PostID:      &post.ID,
					CheckInDate: businessDate,
					CircleID:    input.CircleID,
					Content:     contentPtr,
					Images:      imagesPtr,
				})
				if err != nil {
					return err
				}
				result.CheckIn = checkIn
				result.StreakDays = streakDays

				if err := repository.UserRepo.UpdateCheckInStreak(tx, input.AuthorID, streakDays, checkIn.CreatedAt); err != nil {
					return err
				}
			}
		}

		return nil
	}, &sql.TxOptions{
		Isolation: sql.LevelSerializable,
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}

// Create 创建帖子；遇到同日打卡唯一键竞争或事务死锁时自动重试整个事务。
func (PostService) Create(input CreatePostServiceInput) (*CreatePostServiceResult, error) {
	for attempt := 0; attempt < maxTransactionAttempts; attempt++ {
		result, err := PostSvc.runCreateTransaction(input)
		if err == nil {
			return result, nil
		}
		// 可预期领域错误不重试。
		var serviceErr *PostServiceError
		if errors.As(err, &serviceErr) {
			return nil, err
		}
		if !isRetryableError(err) || attempt >= maxTransactionAttempts-1 {
			return nil, err
		}
	}
	return nil, errors.New("创建帖子事务重试失败")
}
