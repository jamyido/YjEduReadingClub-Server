package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	"yjedu-reading-club-server/internal/database"
	"yjedu-reading-club-server/internal/middleware"
	"yjedu-reading-club-server/internal/models"
	"yjedu-reading-club-server/internal/pkg/response"
	"yjedu-reading-club-server/internal/repository"
)

// notificationSummary 是消息页所需的通知总量与分类未读统计。
type notificationSummary struct {
	AllTotal    int64 `json:"allTotal"`
	UnreadTotal int64 `json:"unreadTotal"`
	Like        int64 `json:"like"`
	Reply       int64 `json:"reply"`
	System      int64 `json:"system"`
	Task        int64 `json:"task"`
}

// notificationCategoryMap 将前端查询参数映射到仓库通知分类。
var notificationCategoryMap = map[string]string{
	"like":   "like",
	"reply":  "reply",
	"system": "system",
	"task":   "task",
}

// ListNotifications 处理 GET /api/notifications。
// 分页查询当前用户的通知列表，可按分类与未读状态过滤。
// @Summary      通知列表
// @Description  需登录。分页查询当前用户的通知列表，支持按 category 分类（like/reply/system/task）与 onlyUnread 过滤。
// @Tags         通知
// @Accept       json
// @Produce      json
// @Param        page        query     int     false  "页码，默认 1"               default(1)
// @Param        pageSize    query     int     false  "每页数量，默认 20，最大 100"  default(20)
// @Param        category    query     string  false  "通知分类：like/reply/system/task"
// @Param        onlyUnread  query     bool    false  "仅返回未读通知"             default(false)
// @Success      200  {object}  response.ApiResponse
// @Failure      401  {object}  response.ApiResponse  "未登录或登录已过期"
// @Router       /notifications [get]
func ListNotifications(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	page, pageSize := parsePagination(c)
	category := ""
	if raw := strings.ToLower(strings.TrimSpace(c.Query("category"))); raw != "" {
		if mapped, ok := notificationCategoryMap[raw]; ok {
			category = mapped
		}
	}
	list, total, err := repository.NotificationRepo.FindMany(database.Get(), user.ID, repository.NotificationListOptions{
		Page:       page,
		PageSize:   pageSize,
		OnlyUnread: parseBoolQuery(c, "onlyUnread"),
		Category:   category,
	})
	if err != nil {
		response.SendInternalError(c)
		return
	}
	response.SendPaginated(c, list, total, page, pageSize)
}

// dispatchNotificationInput 是校验后的通知派发参数。
type dispatchNotificationInput struct {
	Type       string
	Audience   string
	UserIDs    []int64
	CircleID   int64
	Title      string
	Content    string
	TargetType string
	TargetID   int64
}

// normalizeUserIds 将 userIds 字段归一化为去重的正整数列表。
func normalizeUserIds(value interface{}) []int64 {
	arr, ok := value.([]interface{})
	if !ok {
		return nil
	}
	seen := make(map[int64]bool, len(arr))
	result := make([]int64, 0, len(arr))
	for i := range arr {
		num, ok := arr[i].(float64)
		if !ok {
			return nil
		}
		id := int64(num)
		if id <= 0 || num != float64(id) {
			return nil
		}
		if seen[id] {
			return nil
		}
		seen[id] = true
		result = append(result, id)
	}
	return result
}

// buildDispatchInput 校验并构造通知派发参数。
// 失败时返回错误信息与错误码，HTTP 状态码默认 400。
func buildDispatchInput(body map[string]interface{}) (*dispatchNotificationInput, string, string, int) {
	rawType := strings.ToUpper(strings.TrimSpace(getString(body, "type")))
	if rawType != "SYSTEM" && rawType != "TASK" {
		return nil, "仅支持派发系统通知或任务通知", "INVALID_NOTIFICATION_TYPE", 400
	}

	rawAudience := strings.ToUpper(strings.TrimSpace(getString(body, "audience")))
	if rawAudience != "ALL" && rawAudience != "USERS" && rawAudience != "CIRCLE" {
		return nil, "通知接收范围无效", "INVALID_AUDIENCE", 400
	}

	title := strings.TrimSpace(getString(body, "title"))
	if title == "" {
		return nil, "通知标题不能为空", "MISSING_TITLE", 400
	}
	if len(title) > 200 {
		return nil, "通知标题不能超过 200 个字符", "TITLE_TOO_LONG", 400
	}

	content := strings.TrimSpace(getString(body, "content"))
	if content != "" && len(content) > 5000 {
		return nil, "通知内容不能超过 5000 个字符", "CONTENT_TOO_LONG", 400
	}
	if rawType == "TASK" && content == "" {
		return nil, "任务通知必须包含任务说明", "MISSING_TASK_CONTENT", 400
	}

	input := &dispatchNotificationInput{
		Type:     rawType,
		Audience: rawAudience,
		Title:    title,
		Content:  content,
	}
	if input.Content == "" {
		input.Content = ""
	}

	// userIds 校验。
	userIDs := normalizeUserIds(body["userIds"])
	if rawAudience == "USERS" {
		rawArr, hasArr := body["userIds"].([]interface{})
		if !hasArr || len(rawArr) == 0 || len(userIDs) == 0 {
			return nil, "请至少指定一个接收用户", "MISSING_RECIPIENTS", 400
		}
		if len(rawArr) > 500 || len(userIDs) > 500 {
			return nil, "单次最多指定 500 个接收用户", "TOO_MANY_RECIPIENTS", 400
		}
		if len(userIDs) != len(rawArr) {
			return nil, "接收用户 ID 必须是互不重复的正整数", "INVALID_RECIPIENTS", 400
		}
	}
	input.UserIDs = userIDs

	// circleId 校验。
	if raw, ok := body["circleId"].(float64); ok {
		if raw > 0 && raw == float64(int64(raw)) {
			input.CircleID = int64(raw)
		}
	}
	if rawAudience == "CIRCLE" && input.CircleID == 0 {
		return nil, "请指定有效的圈子 ID", "INVALID_CIRCLE_ID", 400
	}

	// targetType 校验。
	targetType := strings.TrimSpace(getString(body, "targetType"))
	if len(targetType) > 20 {
		return nil, "关联目标类型不能超过 20 个字符", "TARGET_TYPE_TOO_LONG", 400
	}
	if rawType == "SYSTEM" && strings.ToLower(targetType) == "task" {
		return nil, "任务提醒请使用 TASK 通知类型", "TASK_TYPE_REQUIRED", 400
	}
	if targetType == "" {
		if rawType == "TASK" {
			targetType = "task"
		}
	} else {
		input.TargetType = targetType
	}

	// targetId 校验。
	if raw, exists := body["targetId"]; exists && raw != nil {
		num, ok := raw.(float64)
		if !ok || num <= 0 || num != float64(int64(num)) {
			return nil, "关联目标 ID 必须是正整数", "INVALID_TARGET_ID", 400
		}
		input.TargetID = int64(num)
	}

	// 内部使用 Prisma 枚举字符串。
	if rawType == "TASK" {
		input.Type = models.NotificationTypeTask
	} else {
		input.Type = models.NotificationTypeSystem
	}
	return input, "", "", 0
}

// getString 从 map 中读取字符串字段，非字符串时返回空串。
func getString(body map[string]interface{}, key string) string {
	if v, ok := body[key].(string); ok {
		return v
	}
	return ""
}

// resolveDispatchRecipients 根据通知接收范围解析状态正常的接收用户。
// 失败时返回错误信息与错误码。
func resolveDispatchRecipients(input *dispatchNotificationInput) ([]int64, string, string, int) {
	switch input.Audience {
	case "ALL":
		ids, err := repository.UserRepo.FindAllActiveIDs(database.Get())
		if err != nil {
			return nil, "服务器内部错误", "INTERNAL_ERROR", 500
		}
		if len(ids) == 0 {
			return nil, "当前没有可接收通知的用户", "NO_RECIPIENTS", 409
		}
		return ids, "", "", 0
	case "USERS":
		ids, err := repository.UserRepo.FindActiveIDs(database.Get(), input.UserIDs)
		if err != nil {
			return nil, "服务器内部错误", "INTERNAL_ERROR", 500
		}
		if len(ids) != len(input.UserIDs) {
			return nil, "部分接收用户不存在或已被禁用", "INVALID_RECIPIENTS", 400
		}
		return ids, "", "", 0
	case "CIRCLE":
		// 先校验圈子存在性。
		circle, err := repository.CircleRepo.FindByID(database.Get(), input.CircleID)
		if err != nil {
			return nil, "服务器内部错误", "INTERNAL_ERROR", 500
		}
		if circle == nil {
			return nil, "圈子不存在", "CIRCLE_NOT_FOUND", 404
		}
		candidateIDs, err := repository.CircleRepo.FindNotificationRecipientIDs(database.Get(), input.CircleID)
		if err != nil {
			return nil, "服务器内部错误", "INTERNAL_ERROR", 500
		}
		activeIDs, err := repository.UserRepo.FindActiveIDs(database.Get(), candidateIDs)
		if err != nil {
			return nil, "服务器内部错误", "INTERNAL_ERROR", 500
		}
		if len(activeIDs) == 0 {
			return nil, "圈子内没有可接收通知的用户", "NO_RECIPIENTS", 409
		}
		return activeIDs, "", "", 0
	}
	return nil, "通知接收范围无效", "INVALID_AUDIENCE", 400
}

// DispatchNotification 处理 POST /api/notifications/dispatch。
// 仅平台管理员可派发 SYSTEM 或 TASK 通知。
// @Summary      派发通知
// @Description  需登录 + 需管理员。派发 SYSTEM 或 TASK 通知，支持按 ALL（全体活跃用户）、USERS（指定用户，最多 500 个）、CIRCLE（指定圈子）三种接收范围派发；TASK 类型必须包含任务说明内容。
// @Tags         通知
// @Accept       json
// @Produce      json
// @Param        body  body      object  true  "通知派发请求"  Example({"type":"SYSTEM","audience":"ALL","title":"系统维护通知","content":"今晚 22:00 进行系统维护","userIds":[],"circleId":0,"targetType":"","targetId":0})
// @Success      201  {object}  response.ApiResponse
// @Failure      400  {object}  response.ApiResponse  "参数缺失或校验不通过"
// @Failure      403  {object}  response.ApiResponse  "仅平台管理员可派发系统通知和任务通知"
// @Failure      404  {object}  response.ApiResponse  "圈子不存在"
// @Failure      409  {object}  response.ApiResponse  "当前没有可接收通知的用户"
// @Router       /notifications/dispatch [post]
func DispatchNotification(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if !middleware.IsAdmin(user) {
		response.SendForbidden(c, "仅平台管理员可派发系统通知和任务通知")
		return
	}

	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.SendBadRequest(c, "请求参数格式错误")
		return
	}
	input, msg, code, status := buildDispatchInput(body)
	if input == nil {
		response.SendError(c, status, code, msg)
		return
	}

	recipients, msg, code, status := resolveDispatchRecipients(input)
	if recipients == nil {
		response.SendError(c, status, code, msg)
		return
	}

	var targetTypePtr *string
	if input.TargetType != "" {
		t := input.TargetType
		targetTypePtr = &t
	}
	var targetIDPtr *int64
	if input.TargetID != 0 {
		id := input.TargetID
		targetIDPtr = &id
	}
	var contentPtr *string
	if input.Content != "" {
		content := input.Content
		contentPtr = &content
	}

	if err := repository.NotificationRepo.CreateMany(database.Get(), recipients, repository.CreateNotificationInput{
		Type:       input.Type,
		TargetType: targetTypePtr,
		TargetID:   targetIDPtr,
		Title:      input.Title,
		Content:    contentPtr,
	}); err != nil {
		response.SendInternalError(c)
		return
	}

	audience := input.Audience
	typeLabel := "SYSTEM"
	if input.Type == models.NotificationTypeTask {
		typeLabel = "TASK"
	}
	response.SendCreated(c, gin.H{
		"type":            typeLabel,
		"audience":        audience,
		"recipientCount":  len(recipients),
	}, "通知派发成功")
}

// MarkNotificationRead 处理 POST /api/notifications/read。
// 传 id 时标记单条已读，未传 id 时标记全部已读。
// @Summary      标记通知已读
// @Description  需登录。请求体携带 id 时标记该条通知为已读（id 必须为正整数且属于当前用户）；未传 id 时将当前用户所有未读通知标记为已读，返回标记已读的数量。
// @Tags         通知
// @Accept       json
// @Produce      json
// @Param        body  body      object  true  "标记已读请求"  Example({"id":1})
// @Success      200  {object}  response.ApiResponse
// @Failure      400  {object}  response.ApiResponse  "通知 ID 必须是正整数"
// @Failure      401  {object}  response.ApiResponse  "未登录或登录已过期"
// @Failure      404  {object}  response.ApiResponse  "通知不存在或无权操作"
// @Router       /notifications/read [post]
func MarkNotificationRead(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	var body struct {
		ID *int64 `json:"id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.SendBadRequest(c, "请求参数格式错误")
		return
	}

	if body.ID != nil {
		if *body.ID <= 0 {
			response.SendError(c, 400, "INVALID_NOTIFICATION_ID", "通知 ID 必须是正整数")
			return
		}
		updated, err := repository.NotificationRepo.MarkRead(database.Get(), *body.ID, user.ID)
		if err != nil {
			response.SendInternalError(c)
			return
		}
		if updated == nil {
			response.SendNotFound(c, "通知不存在或无权操作")
			return
		}
		response.SendSuccess(c, updated, "已标记为已读")
		return
	}

	count, err := repository.NotificationRepo.MarkAllRead(database.Get(), user.ID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	response.SendSuccess(c, gin.H{"count": count}, "全部已读")
}

// GetNotificationSummary 处理 GET /api/notifications/summary。
// 返回当前用户的通知总数和分类未读数。
// @Summary      通知汇总
// @Description  需登录。返回当前用户的通知总数及点赞、回复、系统、任务分类的未读数量统计。
// @Tags         通知
// @Accept       json
// @Produce      json
// @Success      200  {object}  response.ApiResponse
// @Failure      401  {object}  response.ApiResponse  "未登录或登录已过期"
// @Router       /notifications/summary [get]
func GetNotificationSummary(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	allTotal, err := repository.NotificationRepo.CountAll(database.Get(), user.ID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	typeCounts, err := repository.NotificationRepo.CountUnreadByType(database.Get(), user.ID)
	if err != nil {
		response.SendInternalError(c)
		return
	}

	summary := notificationSummary{AllTotal: allTotal}
	for t, count := range typeCounts {
		summary.UnreadTotal += count
		switch t {
		case models.NotificationTypeLike:
			summary.Like += count
		case models.NotificationTypeComment:
			summary.Reply += count
		case models.NotificationTypeTask:
			summary.Task += count
		case models.NotificationTypeSystem:
			summary.System += count
		default:
			summary.System += count
		}
	}
	response.SendSuccess(c, summary)
}
