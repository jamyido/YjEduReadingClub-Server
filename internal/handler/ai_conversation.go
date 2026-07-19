package handler

import (
	"log"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"yjedu-reading-club-server/internal/database"
	"yjedu-reading-club-server/internal/middleware"
	"yjedu-reading-club-server/internal/models"
	"yjedu-reading-club-server/internal/pkg/ai"
	"yjedu-reading-club-server/internal/pkg/response"
	"yjedu-reading-club-server/internal/repository"
)

// aiConversationDetail 是会话详情响应，包含会话元信息与消息历史。
type aiConversationDetail struct {
	models.AIConversation
	PostTitle   string           `json:"postTitle"`
	PostContent string           `json:"postContent"`
	CircleName  string           `json:"circleName"`
	TopicTitle  string           `json:"topicTitle"`
	Messages    []models.AIMessage `json:"messages"`
}

// CreateOrGetAIConversation 处理 POST /api/posts/:id/ai-conversations。
// 创建或获取当前用户对该帖子的 AI 共读会话。
// 若为新建会话，同时派发一条 TASK 通知到消息页任务分类。
// @Summary      创建或获取 AI 共读会话
// @Description  需登录。创建或获取当前用户对该帖子的 AI 共读会话；若为新建会话，同时派发一条 TASK 通知到消息页任务分类，并返回会话详情与消息历史。
// @Tags         AI 共读
// @Accept       json
// @Produce      json
// @Param        id   path      int64   true  "帖子 ID"
// @Success      200  {object}  response.ApiResponse  "已存在会话"
// @Success      201  {object}  response.ApiResponse  "新建会话"
// @Failure      400  {object}  response.ApiResponse  "帖子 ID 必须是正整数"
// @Failure      401  {object}  response.ApiResponse  "未登录或登录已过期"
// @Failure      404  {object}  response.ApiResponse  "帖子不存在"
// @Router       /posts/{id}/ai-conversations [post]
func CreateOrGetAIConversation(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	postID, ok := parseInt64Param(c, "id")
	if !ok {
		response.SendBadRequest(c, "帖子 ID 必须是正整数")
		return
	}

	// 查询帖子是否存在并获取上下文。
	postDetail, err := repository.PostRepo.FindDetailByID(database.Get(), postID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if postDetail == nil {
		response.SendNotFound(c, "帖子不存在")
		return
	}

	// 构造会话标题：优先用帖子标题，否则截取内容前 30 字。
	title := ""
	if postDetail.Title != nil && *postDetail.Title != "" {
		title = *postDetail.Title
	} else {
		content := postDetail.Content
		if len(content) > 30 {
			content = content[:30] + "..."
		}
		title = content
	}

	conv, created, err := repository.AIConversationRepo.FindOrCreateByUserPost(database.Get(), user.ID, postID, title)
	if err != nil {
		response.SendInternalError(c)
		return
	}

	// 新建会话时派发 TASK 通知，使会话出现在消息页任务分类。
	if created {
		targetType := "ai_conversation"
		content := "AI 共读已开启，点击进入与 AI 深度探讨这篇帖子。"
		_, _ = repository.NotificationRepo.Create(database.Get(), repository.CreateNotificationInput{
			UserID:     user.ID,
			Type:       models.NotificationTypeTask,
			TargetType: &targetType,
			TargetID:   &conv.ID,
			Title:      "AI 共读：" + title,
			Content:    &content,
		})
	}

	// 加载消息历史。
	messages, err := repository.AIConversationRepo.FindMessages(database.Get(), conv.ID)
	if err != nil {
		response.SendInternalError(c)
		return
	}

	circleName := ""
	if postDetail.Circle != nil {
		circleName = postDetail.Circle.Name
	}
	topicTitle := ""
	if postDetail.Topic != nil {
		topicTitle = postDetail.Topic.Title
	}
	postTitle := ""
	if postDetail.Title != nil {
		postTitle = *postDetail.Title
	}

	detail := aiConversationDetail{
		AIConversation: *conv,
		PostTitle:      postTitle,
		PostContent:    postDetail.Content,
		CircleName:     circleName,
		TopicTitle:     topicTitle,
		Messages:       messages,
	}

	if created {
		response.SendCreated(c, detail, "AI 共读会话已开启")
	} else {
		response.SendSuccess(c, detail)
	}
}

// ListAIConversations 处理 GET /api/ai-conversations。
// 分页查询当前用户的 AI 共读会话列表。
// @Summary      AI 共读会话列表
// @Description  需登录。分页查询当前用户的 AI 共读会话列表。
// @Tags         AI 共读
// @Accept       json
// @Produce      json
// @Param        page      query     int     false  "页码，默认 1"    default(1)
// @Param        pageSize  query     int     false  "每页数量，默认 20"  default(20)
// @Success      200  {object}  response.ApiResponse
// @Failure      401  {object}  response.ApiResponse  "未登录或登录已过期"
// @Failure      500  {object}  response.ApiResponse  "服务器内部错误"
// @Router       /ai-conversations [get]
func ListAIConversations(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	page, pageSize := parsePagination(c)

	list, total, err := repository.AIConversationRepo.FindMany(database.Get(), user.ID, repository.AIConversationListOptions{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.SendInternalError(c)
		return
	}
	response.SendPaginated(c, list, total, page, pageSize)
}

// GetAIConversation 处理 GET /api/ai-conversations/:id。
// 返回会话详情与消息历史，仅会话所属用户可访问。
// @Summary      AI 共读会话详情
// @Description  需登录。返回会话详情与消息历史，仅会话所属用户可访问。
// @Tags         AI 共读
// @Accept       json
// @Produce      json
// @Param        id   path      int64   true  "会话 ID"
// @Success      200  {object}  response.ApiResponse
// @Failure      400  {object}  response.ApiResponse  "会话 ID 必须是正整数"
// @Failure      401  {object}  response.ApiResponse  "未登录或登录已过期"
// @Failure      403  {object}  response.ApiResponse  "无权访问该会话"
// @Failure      404  {object}  response.ApiResponse  "会话不存在"
// @Router       /ai-conversations/{id} [get]
func GetAIConversation(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	convID, ok := parseInt64Param(c, "id")
	if !ok {
		response.SendBadRequest(c, "会话 ID 必须是正整数")
		return
	}

	conv, err := repository.AIConversationRepo.FindByID(database.Get(), convID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if conv == nil {
		response.SendNotFound(c, "会话不存在")
		return
	}
	if conv.UserID != user.ID {
		response.SendForbidden(c, "无权访问该会话")
		return
	}

	// 加载帖子上下文。
	postDetail, err := repository.PostRepo.FindDetailByID(database.Get(), conv.PostID)
	if err != nil {
		response.SendInternalError(c)
		return
	}

	messages, err := repository.AIConversationRepo.FindMessages(database.Get(), conv.ID)
	if err != nil {
		response.SendInternalError(c)
		return
	}

	circleName := ""
	topicTitle := ""
	postTitle := ""
	postContent := ""
	if postDetail != nil {
		if postDetail.Circle != nil {
			circleName = postDetail.Circle.Name
		}
		if postDetail.Topic != nil {
			topicTitle = postDetail.Topic.Title
		}
		if postDetail.Title != nil {
			postTitle = *postDetail.Title
		}
		postContent = postDetail.Content
	}

	detail := aiConversationDetail{
		AIConversation: *conv,
		PostTitle:      postTitle,
		PostContent:    postContent,
		CircleName:     circleName,
		TopicTitle:     topicTitle,
		Messages:       messages,
	}
	response.SendSuccess(c, detail)
}

// SendMessageInput 是发送 AI 消息的请求体。
type SendMessageInput struct {
	Content string `json:"content"`
}

// SendAIMessage 处理 POST /api/ai-conversations/:id/messages。
// 存储用户消息，调用配置的 AI provider（Coze / DeepSeek 等）获取 AI 回复，
// 存储 AI 消息，返回两条消息。
// 调用 AI 失败时用户消息仍保留，AI 消息返回错误提示文本。
// @Summary      发送 AI 消息
// @Description  需登录。存储用户消息，调用配置的 AI provider（Coze / DeepSeek 等）获取 AI 回复，存储 AI 消息并返回两条消息；调用 AI 失败时用户消息仍保留，AI 消息返回错误提示文本；单条消息不能超过 5000 字。
// @Tags         AI 共读
// @Accept       json
// @Produce      json
// @Param        id    path      int64   true  "会话 ID"
// @Param        body  body      object  true  "消息请求"  Example({"content":"这本书的核心论点是什么？"})
// @Success      201  {object}  response.ApiResponse
// @Failure      400  {object}  response.ApiResponse  "请求参数格式错误或消息内容为空"
// @Failure      401  {object}  response.ApiResponse  "未登录或登录已过期"
// @Failure      403  {object}  response.ApiResponse  "无权在该会话中发送消息"
// @Failure      404  {object}  response.ApiResponse  "会话不存在"
// @Router       /ai-conversations/{id}/messages [post]
func SendAIMessage(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	convID, ok := parseInt64Param(c, "id")
	if !ok {
		response.SendBadRequest(c, "会话 ID 必须是正整数")
		return
	}

	var body SendMessageInput
	if err := c.ShouldBindJSON(&body); err != nil {
		response.SendBadRequest(c, "请求参数格式错误")
		return
	}
	content := strings.TrimSpace(body.Content)
	if content == "" {
		response.SendBadRequest(c, "消息内容不能为空")
		return
	}
	if len(content) > 5000 {
		response.SendBadRequest(c, "单条消息不能超过 5000 字")
		return
	}

	conv, err := repository.AIConversationRepo.FindByID(database.Get(), convID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if conv == nil {
		response.SendNotFound(c, "会话不存在")
		return
	}
	if conv.UserID != user.ID {
		response.SendForbidden(c, "无权在该会话中发送消息")
		return
	}

	// 存储用户消息。
	userMsg, err := repository.AIConversationRepo.CreateMessage(database.Get(), conv.ID, models.AIMessageRoleUser, content)
	if err != nil {
		response.SendInternalError(c)
		return
	}

	// 加载帖子上下文以构造系统提示。
	postDetail, err := repository.PostRepo.FindDetailByID(database.Get(), conv.PostID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	postTitle := ""
	postContent := ""
	circleName := ""
	topicTitle := ""
	if postDetail != nil {
		if postDetail.Title != nil {
			postTitle = *postDetail.Title
		}
		postContent = postDetail.Content
		if postDetail.Circle != nil {
			circleName = postDetail.Circle.Name
		}
		if postDetail.Topic != nil {
			topicTitle = postDetail.Topic.Title
		}
	}
	systemPrompt := ai.BuildSystemPrompt(postTitle, postContent, circleName, topicTitle)

	// 加载历史消息构造 AI 请求（不含刚存的用户消息，因为它会作为最后一条消息单独传入）。
	historyMessages, err := repository.AIConversationRepo.FindMessages(database.Get(), conv.ID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	chatMessages := make([]ai.ChatMessage, 0, len(historyMessages))
	for i := range historyMessages {
		msg := historyMessages[i]
		// 跳过最后一条用户消息（即本次刚存的），作为末尾单独传入。
		if msg.ID == userMsg.ID {
			continue
		}
		role := ai.ChatMessage{
			Content:     msg.Content,
			ContentType: "text",
			Role:        msg.Role,
		}
		if msg.Role == models.AIMessageRoleUser {
			role.Type = "question"
		} else {
			role.Type = "answer"
		}
		chatMessages = append(chatMessages, role)
	}
	// 追加本次用户提问作为最后一条消息。
	chatMessages = append(chatMessages, ai.ChatMessage{
		Content:     content,
		ContentType: "text",
		Role:        models.AIMessageRoleUser,
		Type:        "question",
	})

	// 调用当前配置的 AI provider（coze / deepseek）。
	aiReply, chatErr := ai.Chat(systemPrompt, chatMessages, strconv.FormatInt(user.ID, 10))

	// AI 调用失败时，存入错误提示但仍保留用户消息。
	if chatErr != nil {
		errMsg := "AI 暂时无法响应，请稍后重试。" + chatErr.Error()
		assistantMsg, dbErr := repository.AIConversationRepo.CreateMessage(database.Get(), conv.ID, models.AIMessageRoleAssistant, errMsg)
		if dbErr != nil {
			response.SendInternalError(c)
			return
		}
		response.SendCreated(c, gin.H{
			"userMessage":      userMsg,
			"assistantMessage": assistantMsg,
		})
		return
	}

	// 存储并返回 AI 消息。
	assistantMsg, err := repository.AIConversationRepo.CreateMessage(database.Get(), conv.ID, models.AIMessageRoleAssistant, aiReply)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	response.SendCreated(c, gin.H{
		"userMessage":      userMsg,
		"assistantMessage": assistantMsg,
	})
}

// autoStartTriggerMessage 是触发 AI 主动开启共读的隐式 user 消息。
// 该消息不会存入数据库，仅用于让 AI 生成首条开场白，避免对话历史出现"用户未发言却凭空开始"的违和感。
const autoStartTriggerMessage = "请基于这篇帖子主动开启共读对话：先用一两句话点明你从帖子中读到了什么，再提出一个引导性的思辨问题。不要大段复述帖子原文。"

// autoStartFallbackReply 是 AI 调用失败时的兜底开场白，确保用户进入会话后能看到内容。
const autoStartFallbackReply = "你好！我已读完你的帖子，准备好与你深度共读了。请告诉我，你想从哪个角度开始探讨这本书？例如：作者的写作意图、某个关键观点、或者你最触动的一段。"

// TriggerAutoAIConversation 在帖子创建成功后异步触发 AI 共读开启流程。
// 调用方应在发帖事务提交后调用本函数，本函数立即返回不阻塞，AI 流程在后台协程中执行。
// 流程：
//  1. 查询帖子详情获取圈子/话题上下文
//  2. FindOrCreateByUserPost 创建会话；若已存在则直接返回（避免重复触发）
//  3. 调用 AI 生成首条开场白（失败时使用兜底文案）
//  4. 将 AI 开场白存储为 assistant 消息
//  5. 派发 TASK 通知到消息页，提醒用户进入共读
func TriggerAutoAIConversation(postID int64, authorID int64) {
	go func() {
		// 兜底 recover，防止协程 panic 导致进程异常。
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[ai.auto] 自动开启 AI 共读异常 postID=%d: %v", postID, r)
			}
		}()

		db := database.Get()

		// 1. 查询帖子详情获取 circleName/topicTitle 上下文。
		postDetail, err := repository.PostRepo.FindDetailByID(db, postID)
		if err != nil {
			log.Printf("[ai.auto] 查询帖子详情失败 postID=%d: %v", postID, err)
			return
		}
		if postDetail == nil {
			log.Printf("[ai.auto] 帖子不存在 postID=%d", postID)
			return
		}

		// 2. 构造会话标题：优先用帖子标题，否则截取内容前 30 字。
		title := ""
		if postDetail.Title != nil && *postDetail.Title != "" {
			title = *postDetail.Title
		} else {
			content := postDetail.Content
			if len(content) > 30 {
				content = content[:30] + "..."
			}
			title = content
		}

		// 3. 创建会话；若已存在则不重复触发（用户已主动开过共读）。
		conv, created, err := repository.AIConversationRepo.FindOrCreateByUserPost(db, authorID, postID, title)
		if err != nil {
			log.Printf("[ai.auto] 创建 AI 共读会话失败 postID=%d: %v", postID, err)
			return
		}
		if !created {
			// 会话已存在，说明用户已主动开启过共读，不重复触发。
			return
		}

		// 4. 构造系统提示词与触发消息，调用 AI 生成开场白。
		circleName := ""
		if postDetail.Circle != nil {
			circleName = postDetail.Circle.Name
		}
		topicTitle := ""
		if postDetail.Topic != nil {
			topicTitle = postDetail.Topic.Title
		}
		postTitle := ""
		if postDetail.Title != nil {
			postTitle = *postDetail.Title
		}
		systemPrompt := ai.BuildSystemPrompt(postTitle, postDetail.Content, circleName, topicTitle)

		// 触发消息不存数据库，仅传给 AI 让其主动发起对话。
		triggerMsg := ai.ChatMessage{
			Content:     autoStartTriggerMessage,
			ContentType: "text",
			Role:        "user",
			Type:        "question",
		}

		aiReply, chatErr := ai.Chat(systemPrompt, []ai.ChatMessage{triggerMsg}, strconv.FormatInt(authorID, 10))
		if chatErr != nil {
			// AI 调用失败时使用兜底文案，仍要保证会话有首条消息可看。
			log.Printf("[ai.auto] 生成 AI 开场白失败 postID=%d: %v", postID, chatErr)
			aiReply = autoStartFallbackReply
		}

		// 5. 存储 AI 开场白为 assistant 消息。
		if _, err := repository.AIConversationRepo.CreateMessage(db, conv.ID, models.AIMessageRoleAssistant, aiReply); err != nil {
			log.Printf("[ai.auto] 存储 AI 开场白失败 conversationID=%d: %v", conv.ID, err)
			return
		}

		// 6. 派发 TASK 通知到消息页任务分类。
		targetType := "ai_conversation"
		notifyContent := "AI 已读完你的帖子并准备了共读开场白，点击进入对话。"
		if _, err := repository.NotificationRepo.Create(db, repository.CreateNotificationInput{
			UserID:     authorID,
			Type:       models.NotificationTypeTask,
			TargetType: &targetType,
			TargetID:   &conv.ID,
			Title:      "AI 共读：" + title,
			Content:    &notifyContent,
		}); err != nil {
			log.Printf("[ai.auto] 派发 AI 共读通知失败 conversationID=%d: %v", conv.ID, err)
		}

		log.Printf("[ai.auto] AI 共读已自动开启 postID=%d conversationID=%d", postID, conv.ID)
	}()
}
