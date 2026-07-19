package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	"yjedu-reading-club-server/internal/database"
	"yjedu-reading-club-server/internal/middleware"
	"yjedu-reading-club-server/internal/models"
	"yjedu-reading-club-server/internal/pkg/mediaurl"
	"yjedu-reading-club-server/internal/pkg/response"
	"yjedu-reading-club-server/internal/repository"
	"yjedu-reading-club-server/internal/service"
)

// normalizeImageUrls 将旧打卡接口的 images 字段归一化为 URL 数组。
// 兼容单个 URL、URL 数组和历史 JSON 数组字符串。
func normalizeImageUrls(raw interface{}) []string {
	if raw == nil {
		return nil
	}
	switch value := raw.(type) {
	case []interface{}:
		result := make([]string, 0, len(value))
		for i := range value {
			if str, ok := value[i].(string); ok {
				trimmed := strings.TrimSpace(str)
				if trimmed != "" {
					result = append(result, trimmed)
				}
			}
		}
		return result
	case string:
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return nil
		}
		if strings.HasPrefix(trimmed, "[") {
			// 历史数据可能保存为 JSON 数组字符串。
			// 这里不引入 encoding/json，简单提取引号包裹的字符串。
			return parseJsonStringArray(trimmed)
		}
		return []string{trimmed}
	}
	return nil
}

// parseJsonStringArray 简易解析形如 ["a","b"] 的 JSON 字符串数组。
// 避免引入 encoding/json 依赖，且对非法格式返回空数组。
func parseJsonStringArray(value string) []string {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
		return nil
	}
	body := strings.TrimSuffix(strings.TrimPrefix(value, "["), "]")
	body = strings.TrimSpace(body)
	if body == "" {
		return nil
	}
	parts := strings.Split(body, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if len(part) >= 2 && strings.HasPrefix(part, "\"") && strings.HasSuffix(part, "\"") {
			inner := part[1 : len(part)-1]
			if inner != "" {
				result = append(result, inner)
			}
		}
	}
	return result
}

// ListCheckIns 处理 GET /api/checkin。
// 分页查询打卡记录。
// @Summary      打卡记录列表
// @Description  公开接口。分页查询打卡记录，可按用户 ID 与圈子 ID 过滤。
// @Tags         打卡
// @Accept       json
// @Produce      json
// @Param        userId    query     int64   false  "用户 ID"
// @Param        circleId  query     int64   false  "圈子 ID"
// @Param        page      query     int     false  "页码，默认 1"    default(1)
// @Param        pageSize  query     int     false  "每页数量，默认 20"  default(20)
// @Success      200  {object}  response.ApiResponse
// @Failure      500  {object}  response.ApiResponse  "服务器内部错误"
// @Router       /checkin [get]
func ListCheckIns(c *gin.Context) {
	page, pageSize := parsePagination(c)
	opts := repository.CheckInListOptions{
		Page:     page,
		PageSize: pageSize,
		UserID:   parseInt64Query(c, "userId"),
		CircleID: parseInt64Query(c, "circleId"),
	}
	list, total, err := repository.CheckInRepo.FindMany(database.Get(), opts)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	response.SendPaginated(c, list, total, opts.Page, opts.PageSize)
}

// CreateCheckIn 处理 POST /api/checkin。
// 兼容旧客户端入口，委托 PostService 创建打卡挑战帖子，并在同一事务计算连续天数。
// @Summary      创建打卡
// @Description  需登录。兼容旧客户端入口，委托 PostService 创建打卡挑战帖子，并在同一事务计算连续天数；同一用户同一圈子每日仅可打卡一次。
// @Tags         打卡
// @Accept       json
// @Produce      json
// @Param        body  body      object  true  "打卡请求"  Example({"circleId":1,"content":"完成今日打卡","images":["https://cdn.example.com/a.jpg"]})
// @Success      201  {object}  response.ApiResponse
// @Failure      400  {object}  response.ApiResponse  "请求参数格式错误或圈子 ID 不合法"
// @Failure      401  {object}  response.ApiResponse  "未登录或登录已过期"
// @Failure      409  {object}  response.ApiResponse  "今日已打卡"
// @Failure      500  {object}  response.ApiResponse  "打卡记录创建失败"
// @Router       /checkin [post]
func CreateCheckIn(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	var body struct {
		CircleID interface{} `json:"circleId"`
		Content  string      `json:"content"`
		Images   interface{} `json:"images"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.SendBadRequest(c, "请求参数格式错误")
		return
	}

	// circleId 可选；非空时必须为正整数。
	var circleID *int64
	switch value := body.CircleID.(type) {
	case nil:
	case string:
		if strings.TrimSpace(value) == "" {
			break
		}
		response.SendError(c, 400, "INVALID_CIRCLE_ID", "圈子 ID 格式不正确")
		return
	case float64:
		if value <= 0 || value != float64(int64(value)) {
			response.SendError(c, 400, "INVALID_CIRCLE_ID", "圈子 ID 格式不正确")
			return
		}
		id := int64(value)
		circleID = &id
	default:
		response.SendError(c, 400, "INVALID_CIRCLE_ID", "圈子 ID 格式不正确")
		return
	}

	content := strings.TrimSpace(body.Content)
	if content == "" {
		content = "完成今日打卡"
	}

	imageUrls := normalizeImageUrls(body.Images)
	if len(imageUrls) > 9 {
		response.SendError(c, 400, "INVALID_MEDIAS", "打卡图片最多 9 张")
		return
	}

	medias := make([]service.PostServiceMediaInput, 0, len(imageUrls))
	for i := range imageUrls {
		url, err := mediaurl.ValidatePersistedImageUrlErr(imageUrls[i], user.ID, "")
		if err != nil {
			response.SendBadRequest(c, err.Error())
			return
		}
		medias = append(medias, service.PostServiceMediaInput{
			Type: "image",
			URL:  url,
			Sort: i,
		})
	}

	postType := models.PostTypeText
	if len(medias) > 0 {
		postType = models.PostTypeImage
	}

	result, err := service.PostSvc.Create(service.CreatePostServiceInput{
		AuthorID:          user.ID,
		CircleID:          circleID,
		Type:              postType,
		Content:           content,
		Medias:            medias,
		RequireNewCheckIn: true,
	})
	if err != nil {
		handlePostServiceError(c, err)
		return
	}
	if result.CheckIn == nil {
		response.SendInternalError(c, "打卡记录创建失败")
		return
	}
	response.SendCreated(c, gin.H{
		"checkIn": result.CheckIn,
		"streak":  result.StreakDays,
	}, "打卡成功")
}
