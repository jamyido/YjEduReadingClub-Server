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

// ListMessages 处理 GET /api/messages。
// 分页查询当前用户的私信会话列表。
// @Summary      私信会话列表
// @Description  需登录。分页查询当前用户的私信会话列表，支持按 onlyUnread 过滤未读会话。
// @Tags         私信
// @Accept       json
// @Produce      json
// @Param        page        query     int     false  "页码，默认 1"               default(1)
// @Param        pageSize    query     int     false  "每页数量，默认 20，最大 100"  default(20)
// @Param        onlyUnread  query     bool    false  "仅返回未读会话"             default(false)
// @Success      200  {object}  response.ApiResponse
// @Failure      401  {object}  response.ApiResponse  "未登录或登录已过期"
// @Router       /messages [get]
func ListMessages(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	page, pageSize := parsePagination(c)
	onlyUnread := parseBoolQuery(c, "onlyUnread")
	list, total, err := repository.MessageRepo.FindMany(database.Get(), user.ID, page, pageSize, onlyUnread)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	response.SendPaginated(c, list, total, page, pageSize)
}

// GetConversation 处理 GET /api/messages/:userId。
// 查询与指定用户的私聊记录，拉取后将对方发来的未读消息标记为已读。
// @Summary      获取会话私聊记录
// @Description  需登录。分页查询当前用户与指定用户的私聊记录，拉取成功后将对方发来的未读消息标记为已读；不能与自己发起会话。
// @Tags         私信
// @Accept       json
// @Produce      json
// @Param        userId    path      int     true   "对方用户 ID"
// @Param        page      query     int     false  "页码，默认 1"               default(1)
// @Param        pageSize  query     int     false  "每页数量，默认 50，最大 100"  default(50)
// @Success      200  {object}  response.ApiResponse
// @Failure      400  {object}  response.ApiResponse  "用户 ID 不合法或不能与自己发起会话"
// @Failure      401  {object}  response.ApiResponse  "未登录或登录已过期"
// @Router       /messages/{userId} [get]
func GetConversation(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	otherUserID, ok := parseInt64Param(c, "userId")
	if !ok {
		response.SendError(c, 400, "INVALID_USER_ID", "用户 ID 不合法")
		return
	}
	if otherUserID == user.ID {
		response.SendError(c, 400, "INVALID_USER_ID", "不能与自己发起会话")
		return
	}

	// 私聊默认每页 50 条。
	page := parsePositiveInt(c, "page", 1, 0)
	pageSize := parsePositiveInt(c, "pageSize", 50, maxPageSize)
	list, total, err := repository.MessageRepo.FindConversation(database.Get(), user.ID, otherUserID, page, pageSize)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	// 拉取后将对方发来的消息标记为已读。
	_ = repository.MessageRepo.MarkConversationRead(database.Get(), user.ID, otherUserID)
	response.SendPaginated(c, list, total, page, pageSize)
}

// SendMessage 处理 POST /api/messages/:userId。
// 向目标用户发送一条私信。
// @Summary      发送私信
// @Description  需登录。向指定用户发送一条文本私信，不能给自己发送私信；消息内容不能为空。
// @Tags         私信
// @Accept       json
// @Produce      json
// @Param        userId  path      int     true  "对方用户 ID"
// @Param        body    body      object  true  "发送私信请求"  Example({"content":"你好，约读吗？"})
// @Success      201  {object}  response.ApiResponse
// @Failure      400  {object}  response.ApiResponse  "用户 ID 不合法或消息内容为空"
// @Failure      401  {object}  response.ApiResponse  "未登录或登录已过期"
// @Router       /messages/{userId} [post]
func SendMessage(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	otherUserID, ok := parseInt64Param(c, "userId")
	if !ok {
		response.SendError(c, 400, "INVALID_USER_ID", "用户 ID 不合法")
		return
	}
	if otherUserID == user.ID {
		response.SendError(c, 400, "INVALID_USER_ID", "不能给自己发送私信")
		return
	}

	var body struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.SendBadRequest(c, "请求参数格式错误")
		return
	}
	content := strings.TrimSpace(body.Content)
	if content == "" {
		response.SendError(c, 400, "EMPTY_CONTENT", "消息内容不能为空")
		return
	}

	message, err := repository.MessageRepo.Send(database.Get(), user.ID, otherUserID, models.MessageTypeText, content)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	response.SendCreated(c, message, "发送成功")
}

// MarkAllMessagesRead 处理 POST /api/messages/read-all。
// 将当前用户所有未读消息标记为已读。
// @Summary      全部私信已读
// @Description  需登录。将当前用户所有未读私信标记为已读，返回标记已读的消息数量。
// @Tags         私信
// @Accept       json
// @Produce      json
// @Success      200  {object}  response.ApiResponse
// @Failure      401  {object}  response.ApiResponse  "未登录或登录已过期"
// @Router       /messages/read-all [post]
func MarkAllMessagesRead(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	count, err := repository.MessageRepo.MarkAllRead(database.Get(), user.ID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	response.SendSuccess(c, gin.H{"count": count}, "全部已读")
}
