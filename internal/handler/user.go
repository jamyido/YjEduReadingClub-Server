package handler

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"yjedu-reading-club-server/internal/database"
	"yjedu-reading-club-server/internal/middleware"
	"yjedu-reading-club-server/internal/pkg/businessdate"
	"yjedu-reading-club-server/internal/pkg/mediaurl"
	"yjedu-reading-club-server/internal/pkg/response"
	"yjedu-reading-club-server/internal/repository"
)

// validGenders 是合法的性别取值集合。
var validGenders = map[string]bool{
	"UNKNOWN": true,
	"MALE":    true,
	"FEMALE":  true,
}

// userProfile 是用户公开资料响应。
type userProfile struct {
	ID             int64   `json:"id"`
	Nickname       string  `json:"nickname"`
	Avatar         *string `json:"avatar"`
	Bio            *string `json:"bio"`
	Gender         string  `json:"gender"`
	StreakDays     int     `json:"streakDays"`
	FollowingCount int     `json:"followingCount"`
	FollowerCount  int     `json:"followerCount"`
	CreatedAt      string  `json:"createdAt"`
	UpdatedAt      string  `json:"updatedAt"`
}

// UpdateProfile 处理 PUT /api/users/profile。
// 更新当前用户资料，仅更新请求体中实际传入的字段。
// @Summary      更新当前用户资料
// @Description  需登录。仅更新请求体中实际传入的字段，支持昵称、头像、简介、性别与生日；头像需通过持久化 URL 归属校验，性别取值 UNKNOWN/MALE/FEMALE，生日支持 RFC3339、ISO 日期或秒级时间戳。
// @Tags         用户
// @Accept       json
// @Produce      json
// @Param        body  body      object  true  "更新资料请求"  Example({"nickname":"小明","avatar":"/uploads/avatar/1.png","bio":"热爱阅读","gender":"MALE","birthday":"2000-01-01"})
// @Success      200   {object}  response.ApiResponse
// @Failure      400   {object}  response.ApiResponse  "请求参数格式错误或性别/生日/头像取值无效"
// @Failure      401   {object}  response.ApiResponse  "未登录或登录已过期"
// @Router       /users/profile [put]
func UpdateProfile(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	var body struct {
		Nickname string      `json:"nickname"`
		Avatar   string      `json:"avatar"`
		Bio      string      `json:"bio"`
		Gender   *string     `json:"gender"`
		Birthday interface{} `json:"birthday"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.SendBadRequest(c, "请求参数格式错误")
		return
	}

	input := repository.UpdateUserInput{}
	if trimmed := strings.TrimSpace(body.Nickname); trimmed != "" {
		input.Nickname = &trimmed
	}
	if body.Avatar != "" {
		existingAvatar := ""
		if user.Avatar != nil {
			existingAvatar = *user.Avatar
		}
		avatarURL, err := mediaurl.ValidatePersistedImageUrlErr(body.Avatar, user.ID, existingAvatar)
		if err != nil {
			response.SendBadRequest(c, err.Error())
			return
		}
		input.Avatar = &avatarURL
	}
	if body.Bio != "" {
		bio := body.Bio
		input.Bio = &bio
	}
	if body.Gender != nil {
		upper := strings.ToUpper(*body.Gender)
		if !validGenders[upper] {
			response.SendError(c, 400, "INVALID_GENDER", "性别取值无效")
			return
		}
		input.Gender = &upper
	}
	if body.Birthday != nil {
		birthday, ok := parseBirthday(body.Birthday)
		if !ok {
			response.SendError(c, 400, "INVALID_BIRTHDAY", "生日格式无效")
			return
		}
		input.Birthday = &birthday
	}

	updated, err := repository.UserRepo.Update(database.Get(), user.ID, input)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	response.SendSuccess(c, middleware.ToPublicUser(updated), "资料更新成功")
}

// parseBirthday 将请求体中的生日字段解析为 time.Time。
// 支持 RFC3339 / ISO 日期字符串与时间戳数字。
func parseBirthday(value interface{}) (time.Time, bool) {
	switch v := value.(type) {
	case string:
		if v == "" {
			return time.Time{}, false
		}
		// 优先按 RFC3339 解析，失败时再尝试仅日期格式。
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return t, true
		}
		if t, err := time.Parse("2006-01-02", v); err == nil {
			return t, true
		}
		return time.Time{}, false
	case float64:
		// 时间戳秒数。
		sec := int64(v)
		if sec <= 0 {
			return time.Time{}, false
		}
		return time.Unix(sec, 0).UTC(), true
	}
	return time.Time{}, false
}

// GetUserProfile 处理 GET /api/users/:id。
// 返回用户的公开资料信息，无需登录鉴权。
// @Summary      获取用户公开资料
// @Description  公开接口。返回指定用户的公开主页，包含昵称、头像、简介、性别、连续打卡天数（按业务日期计算）、关注数与粉丝数。
// @Tags         用户
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "用户 ID"
// @Success      200   {object}  response.ApiResponse
// @Failure      400   {object}  response.ApiResponse  "无效的用户 ID"
// @Failure      404   {object}  response.ApiResponse  "用户不存在"
// @Router       /users/{id} [get]
func GetUserProfile(c *gin.Context) {
	userID, ok := parseInt64Param(c, "id")
	if !ok {
		response.SendError(c, 400, "INVALID_ID", "无效的用户 ID")
		return
	}
	user, err := repository.UserRepo.FindByID(database.Get(), userID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if user == nil {
		response.SendNotFound(c, "用户不存在")
		return
	}
	profile := userProfile{
		ID:             user.ID,
		Nickname:       user.Nickname,
		Avatar:         user.Avatar,
		Bio:            user.Bio,
		Gender:         user.Gender,
		StreakDays:     businessdate.GetEffectiveStreakDays(user.StreakDays, user.LastCheckInAt, time.Now()),
		FollowingCount: user.FollowingCount,
		FollowerCount:  user.FollowerCount,
		CreatedAt:      user.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:      user.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	response.SendSuccess(c, profile)
}

// FollowUser 处理 POST /api/users/:id/follow。
// 关注该用户，校验目标用户存在性、防止自我关注与重复关注。
// @Summary      关注用户
// @Description  需登录。关注指定用户，校验目标用户存在性、防止自我关注与重复关注，并同步更新双方关注/粉丝计数。
// @Tags         用户
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "目标用户 ID"
// @Success      200   {object}  response.ApiResponse
// @Failure      400   {object}  response.ApiResponse  "无效的用户 ID 或不能关注自己"
// @Failure      401   {object}  response.ApiResponse  "未登录或登录已过期"
// @Failure      409   {object}  response.ApiResponse  "已经关注该用户"
// @Router       /users/{id}/follow [post]
func FollowUser(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	targetID, ok := parseInt64Param(c, "id")
	if !ok {
		response.SendError(c, 400, "INVALID_ID", "无效的用户 ID")
		return
	}
	if targetID == user.ID {
		response.SendError(c, 400, "CANNOT_FOLLOW_SELF", "不能关注自己")
		return
	}
	target, err := repository.UserRepo.FindByID(database.Get(), targetID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if target == nil {
		response.SendNotFound(c, "用户不存在")
		return
	}
	existing, err := repository.FollowRepo.FindRelation(database.Get(), user.ID, targetID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if existing != nil {
		response.SendError(c, 409, "ALREADY_FOLLOWING", "已经关注该用户")
		return
	}
	if err := repository.FollowRepo.Follow(database.Get(), user.ID, targetID); err != nil {
		response.SendInternalError(c)
		return
	}
	if err := repository.UserRepo.UpdateFollowCounts(database.Get(), user.ID, 1, 0); err != nil {
		response.SendInternalError(c)
		return
	}
	if err := repository.UserRepo.UpdateFollowCounts(database.Get(), targetID, 0, 1); err != nil {
		response.SendInternalError(c)
		return
	}
	response.SendSuccess(c, gin.H{"following": true}, "关注成功")
}

// UnfollowUser 处理 DELETE /api/users/:id/follow。
// 取消关注，校验关注关系是否存在。
// @Summary      取消关注用户
// @Description  需登录。取消关注指定用户，校验关注关系是否存在，并同步更新双方关注/粉丝计数。
// @Tags         用户
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "目标用户 ID"
// @Success      200   {object}  response.ApiResponse
// @Failure      400   {object}  response.ApiResponse  "无效的用户 ID 或不能关注自己"
// @Failure      401   {object}  response.ApiResponse  "未登录或登录已过期"
// @Failure      409   {object}  response.ApiResponse  "尚未关注该用户"
// @Router       /users/{id}/follow [delete]
func UnfollowUser(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	targetID, ok := parseInt64Param(c, "id")
	if !ok {
		response.SendError(c, 400, "INVALID_ID", "无效的用户 ID")
		return
	}
	if targetID == user.ID {
		response.SendError(c, 400, "CANNOT_FOLLOW_SELF", "不能关注自己")
		return
	}
	target, err := repository.UserRepo.FindByID(database.Get(), targetID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if target == nil {
		response.SendNotFound(c, "用户不存在")
		return
	}
	existing, err := repository.FollowRepo.FindRelation(database.Get(), user.ID, targetID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if existing == nil {
		response.SendError(c, 409, "NOT_FOLLOWING", "尚未关注该用户")
		return
	}
	if err := repository.FollowRepo.Unfollow(database.Get(), user.ID, targetID); err != nil {
		response.SendInternalError(c)
		return
	}
	if err := repository.UserRepo.UpdateFollowCounts(database.Get(), user.ID, -1, 0); err != nil {
		response.SendInternalError(c)
		return
	}
	if err := repository.UserRepo.UpdateFollowCounts(database.Get(), targetID, 0, -1); err != nil {
		response.SendInternalError(c)
		return
	}
	response.SendSuccess(c, gin.H{"following": false}, "已取消关注")
}

// ListFollowers 处理 GET /api/users/:id/followers。
// 分页查询用户的粉丝列表，无需登录鉴权。
// @Summary      查询用户粉丝列表
// @Description  公开接口。分页查询指定用户的粉丝列表，page 默认 1，pageSize 默认 20、上限 100。
// @Tags         用户
// @Accept       json
// @Produce      json
// @Param        id        path   int  true   "用户 ID"
// @Param        page      query  int  false  "页码（默认 1）"
// @Param        pageSize  query  int  false  "每页数量（默认 20，上限 100）"
// @Success      200   {object}  response.ApiResponse
// @Failure      400   {object}  response.ApiResponse  "无效的用户 ID"
// @Router       /users/{id}/followers [get]
func ListFollowers(c *gin.Context) {
	userID, ok := parseInt64Param(c, "id")
	if !ok {
		response.SendError(c, 400, "INVALID_ID", "无效的用户 ID")
		return
	}
	page, pageSize := parsePagination(c)
	list, total, err := repository.FollowRepo.FindFollowers(database.Get(), userID, page, pageSize)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	response.SendPaginated(c, list, total, page, pageSize)
}

// ListFollowing 处理 GET /api/users/:id/following。
// 分页查询用户的关注列表，无需登录鉴权。
// @Summary      查询用户关注列表
// @Description  公开接口。分页查询指定用户的关注列表，page 默认 1，pageSize 默认 20、上限 100。
// @Tags         用户
// @Accept       json
// @Produce      json
// @Param        id        path   int  true   "用户 ID"
// @Param        page      query  int  false  "页码（默认 1）"
// @Param        pageSize  query  int  false  "每页数量（默认 20，上限 100）"
// @Success      200   {object}  response.ApiResponse
// @Failure      400   {object}  response.ApiResponse  "无效的用户 ID"
// @Router       /users/{id}/following [get]
func ListFollowing(c *gin.Context) {
	userID, ok := parseInt64Param(c, "id")
	if !ok {
		response.SendError(c, 400, "INVALID_ID", "无效的用户 ID")
		return
	}
	page, pageSize := parsePagination(c)
	list, total, err := repository.FollowRepo.FindFollowing(database.Get(), userID, page, pageSize)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	response.SendPaginated(c, list, total, page, pageSize)
}
