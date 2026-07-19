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

// ListPosts 处理 GET /api/posts。
// 分页查询帖子列表，圈子帖子对未登录/非成员仅开放预览。
// @Summary      分页查询帖子列表
// @Description  公开接口（可选登录）。按圈子、话题、作者、类型筛选分页查询帖子列表；圈子帖子对未登录或非成员仅开放第一页最多 3 条预览，登录用户附点赞态。
// @Tags         帖子
// @Accept       json
// @Produce      json
// @Param        circleId  query  int     false  "圈子 ID"
// @Param        topicId   query  int     false  "话题 ID"
// @Param        authorId  query  int     false  "作者 ID"
// @Param        type      query  string  false  "帖子类型"
// @Param        page      query  int     false  "页码"
// @Param        pageSize  query  int     false  "每页数量"
// @Success      200  {object}  response.ApiResponse
// @Failure      500  {object}  response.ApiResponse  "服务器内部错误"
// @Router       /posts [get]
func ListPosts(c *gin.Context) {
	page, pageSize := parsePagination(c)
	opts := repository.PostListOptions{
		Page:     page,
		PageSize: pageSize,
		CircleID: parseInt64Query(c, "circleId"),
		AuthorID: parseInt64Query(c, "authorId"),
		TopicID:  parseInt64Query(c, "topicId"),
		Type:     c.Query("type"),
	}

	user := middleware.GetCurrentUser(c)
	isAdmin := middleware.IsAdmin(user)

	// 圈子帖子对未登录/非成员只开放第一页最多 3 条预览。
	if opts.CircleID != 0 && !isAdmin {
		isMember := false
		if user != nil {
			membership, err := repository.CircleRepo.FindMembership(database.Get(), user.ID, opts.CircleID)
			if err != nil {
				response.SendInternalError(c)
				return
			}
			isMember = membership != nil
		}
		if !isMember {
			opts.Page = 1
			if opts.PageSize > 3 {
				opts.PageSize = 3
			}
		}
	}

	list, total, err := repository.PostRepo.FindMany(database.Get(), opts)
	if err != nil {
		response.SendInternalError(c)
		return
	}

	// 非成员预览：截断正文、清空链接、媒体只留首张。
	needPreview := opts.CircleID != 0 && !isAdmin
	if needPreview {
		isMember := false
		if user != nil {
			membership, _ := repository.CircleRepo.FindMembership(database.Get(), user.ID, opts.CircleID)
			isMember = membership != nil
		}
		if !isMember {
			for i := range list {
				applyPostPreview(&list[i])
			}
		}
	}

	// 登录用户批量查询点赞态。
	if user != nil {
		postIDs := make([]int64, 0, len(list))
		for i := range list {
			postIDs = append(postIDs, list[i].ID)
		}
		likedIDs, _ := repository.PostRepo.FindLikedPostIDs(database.Get(), user.ID, postIDs)
		likedSet := make(map[int64]bool, len(likedIDs))
		for _, id := range likedIDs {
			likedSet[id] = true
		}
		for i := range list {
			list[i].IsLiked = likedSet[list[i].ID]
		}
	}

	response.SendPaginated(c, list, total, opts.Page, opts.PageSize)
}

// applyPostPreview 对圈子帖子应用预览限制：
// 正文截断到 160 字符，清空链接，媒体只留首个。
func applyPostPreview(item *repository.PostListItem) {
	if len(item.Content) > 160 {
		item.Content = item.Content[:160] + "…"
	}
	item.LinkURL = nil
	if len(item.Medias) > 1 {
		item.Medias = item.Medias[:1]
	}
}

// CreatePost 处理 POST /api/posts。
// 创建帖子，委托 PostService 在事务中完成发帖与打卡副作用。
// @Summary      创建帖子
// @Description  需登录。在指定圈子或话题下创建帖子，支持标题、正文、链接与最多 9 张媒体；委托 PostService 在事务中完成发帖与打卡副作用，成功后异步触发 AI 共读流程。
// @Tags         帖子
// @Accept       json
// @Produce      json
// @Param        body  body  object  true  "创建帖子请求"  Example({"circleId":1,"topicId":2,"type":"text","title":"今日读后感","content":"正文内容","linkUrl":"https://example.com","medias":[{"type":"image","url":"/uploads/1.png","sort":1}]})
// @Success      201  {object}  response.ApiResponse
// @Failure      400  {object}  response.ApiResponse  "请求参数格式错误或 circleId 缺失"
// @Failure      403  {object}  response.ApiResponse  "无发帖权限"
// @Router       /posts [post]
func CreatePost(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	var body struct {
		CircleID *int64 `json:"circleId"`
		TopicID  *int64 `json:"topicId"`
		Type     string `json:"type"`
		Title    string `json:"title"`
		Content  string `json:"content"`
		LinkURL  string `json:"linkUrl"`
		Medias   []struct {
			Type string `json:"type"`
			URL  string `json:"url"`
			Sort int    `json:"sort"`
		} `json:"medias"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.SendBadRequest(c, "请求参数格式错误")
		return
	}
	if strings.TrimSpace(body.Content) == "" {
		response.SendBadRequest(c, "帖子内容不能为空")
		return
	}
	if body.CircleID == nil || *body.CircleID < 1 {
		response.SendBadRequest(c, "circleId 必填")
		return
	}
	if len(body.Medias) > 9 {
		response.SendBadRequest(c, "图片最多 9 张")
		return
	}

	// 媒体 URL 校验归属。
	medias := make([]service.PostServiceMediaInput, 0, len(body.Medias))
	for i := range body.Medias {
		url, err := mediaurl.ValidatePersistedImageUrlErr(body.Medias[i].URL, user.ID, "")
		if err != nil {
			response.SendBadRequest(c, err.Error())
			return
		}
		mediaType := body.Medias[i].Type
		if mediaType == "" {
			mediaType = "image"
		}
		medias = append(medias, service.PostServiceMediaInput{
			Type: mediaType,
			URL:  url,
			Sort: body.Medias[i].Sort,
		})
	}

	var titlePtr *string
	if body.Title != "" {
		titlePtr = &body.Title
	}
	var linkPtr *string
	if body.LinkURL != "" {
		linkPtr = &body.LinkURL
	}
	postType := body.Type
	if postType == "" {
		postType = models.PostTypeText
	}

	result, err := service.PostSvc.Create(service.CreatePostServiceInput{
		AuthorID:   user.ID,
		AuthorRole: user.Role,
		CircleID:   body.CircleID,
		TopicID:    body.TopicID,
		Type:       postType,
		Title:      titlePtr,
		Content:    body.Content,
		LinkURL:    linkPtr,
		Medias:     medias,
	})
	if err != nil {
		handlePostServiceError(c, err)
		return
	}

	// 发帖成功后异步触发 AI 共读流程：自动创建会话、生成 AI 开场白、派发任务通知。
	// 异步执行不阻塞发帖响应；若 AI 配置缺失或调用失败，兜底文案仍能保证会话可用。
	if result.Post != nil {
		TriggerAutoAIConversation(result.Post.ID, result.Post.AuthorID)
	}

	response.SendCreated(c, result.Post)
}

// handlePostServiceError 将发帖领域错误映射为对应的 HTTP 响应。
func handlePostServiceError(c *gin.Context, err error) {
	if svcErr, ok := err.(*service.PostServiceError); ok {
		response.SendError(c, svcErr.StatusCode, svcErr.Code, svcErr.Message)
		return
	}
	response.SendInternalError(c)
}

// GetPost 处理 GET /api/posts/:id。
// 查询帖子详情，圈子帖子需校验成员身份（管理员豁免）。
// @Summary      查询帖子详情
// @Description  公开接口（可选登录）。按 ID 查询帖子详情，圈子帖子需校验成员身份（管理员豁免），登录用户附点赞态。
// @Tags         帖子
// @Accept       json
// @Produce      json
// @Param        id  path  int  true  "帖子 ID"
// @Success      200  {object}  response.ApiResponse
// @Failure      400  {object}  response.ApiResponse  "帖子 ID 无效"
// @Failure      403  {object}  response.ApiResponse  "加入圈子后才能查看完整内容"
// @Failure      404  {object}  response.ApiResponse  "帖子不存在"
// @Router       /posts/{id} [get]
func GetPost(c *gin.Context) {
	postID, ok := parseInt64Param(c, "id")
	if !ok {
		response.SendBadRequest(c, "帖子 ID 无效")
		return
	}

	detail, err := repository.PostRepo.FindDetailByID(database.Get(), postID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if detail == nil || detail.Status == models.PostStatusDeleted {
		response.SendNotFound(c, "帖子不存在")
		return
	}

	// 圈子帖子需校验成员身份。
	user := middleware.GetCurrentUser(c)
	if detail.CircleID != nil && !middleware.IsAdmin(user) {
		isMember := false
		if user != nil {
			membership, _ := repository.CircleRepo.FindMembership(database.Get(), user.ID, *detail.CircleID)
			isMember = membership != nil
		}
		if !isMember {
			response.SendForbidden(c, "加入圈子后才能查看完整内容")
			return
		}
	}

	// 登录用户附点赞态。
	if user != nil {
		like, _ := repository.PostRepo.FindLike(database.Get(), user.ID, models.LikeTargetPost, postID)
		detail.IsLiked = like != nil
	}
	response.SendSuccess(c, detail)
}

// canModifyPost 判断当前用户是否拥有指定帖子的修改/删除权限。
// 规则：作者本人 OR 平台管理员 OR 帖子所属圈子的圈主。
// 非圈子帖子（CircleID 为 nil）仅作者与管理员可改。
func canModifyPost(user *models.User, post *models.Post) bool {
	if user == nil || post == nil {
		return false
	}
	if post.AuthorID == user.ID {
		return true
	}
	if middleware.IsAdmin(user) {
		return true
	}
	if post.CircleID == nil {
		return false
	}
	membership, err := repository.CircleRepo.FindMembership(database.Get(), user.ID, *post.CircleID)
	if err != nil || membership == nil {
		return false
	}
	return membership.Role == models.CircleRoleOwner
}

// DeletePost 处理 DELETE /api/posts/:id。
// 软删除帖子，仅作者、管理员或所属圈子圈主可删。
// @Summary      删除帖子
// @Description  需登录。软删除帖子，仅作者、管理员或所属圈子圈主可删。
// @Tags         帖子
// @Accept       json
// @Produce      json
// @Param        id  path  int  true  "帖子 ID"
// @Success      200  {object}  response.ApiResponse
// @Failure      400  {object}  response.ApiResponse  "帖子 ID 无效"
// @Failure      403  {object}  response.ApiResponse  "无权删除他人帖子"
// @Failure      404  {object}  response.ApiResponse  "帖子不存在"
// @Router       /posts/{id} [delete]
func DeletePost(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	postID, ok := parseInt64Param(c, "id")
	if !ok {
		response.SendBadRequest(c, "帖子 ID 无效")
		return
	}

	post, err := repository.PostRepo.FindByID(database.Get(), postID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if post == nil || post.Status == models.PostStatusDeleted {
		response.SendNotFound(c, "帖子不存在")
		return
	}
	if !canModifyPost(user, post) {
		response.SendForbidden(c, "无权删除他人帖子")
		return
	}
	if err := repository.PostRepo.SoftDelete(database.Get(), postID); err != nil {
		response.SendInternalError(c)
		return
	}
	response.SendSuccess(c, gin.H{"id": postID})
}

// UpdatePost 处理 PUT /api/posts/:id。
// 编辑帖子正文、标题、链接与媒体；作者、管理员或所属圈子圈主可编辑。
// 不允许变更作者、圈子与话题，避免触发打卡副作用与计数错乱。
// @Summary      编辑帖子
// @Description  需登录。编辑帖子标题、正文、链接与媒体（最多 9 张）；仅作者、管理员或所属圈子圈主可编辑，不可变更圈子与话题。
// @Tags         帖子
// @Accept       json
// @Produce      json
// @Param        id    path  int     true  "帖子 ID"
// @Param        body  body  object  true  "编辑帖子请求"  Example({"title":"今日读后感","content":"正文内容","linkUrl":"https://example.com","medias":[{"type":"image","url":"/uploads/1.png","sort":1}]})
// @Success      200  {object}  response.ApiResponse
// @Failure      400  {object}  response.ApiResponse  "请求参数格式错误或内容为空"
// @Failure      403  {object}  response.ApiResponse  "无权编辑他人帖子"
// @Failure      404  {object}  response.ApiResponse  "帖子不存在"
// @Router       /posts/{id} [put]
func UpdatePost(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	postID, ok := parseInt64Param(c, "id")
	if !ok {
		response.SendBadRequest(c, "帖子 ID 无效")
		return
	}

	post, err := repository.PostRepo.FindByID(database.Get(), postID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if post == nil || post.Status == models.PostStatusDeleted {
		response.SendNotFound(c, "帖子不存在")
		return
	}
	if !canModifyPost(user, post) {
		response.SendForbidden(c, "无权编辑他人帖子")
		return
	}

	var body struct {
		Title   string `json:"title"`
		Content string `json:"content"`
		LinkURL string `json:"linkUrl"`
		Type    string `json:"type"`
		Medias  []struct {
			Type string `json:"type"`
			URL  string `json:"url"`
			Sort int    `json:"sort"`
		} `json:"medias"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.SendBadRequest(c, "请求参数格式错误")
		return
	}
	if strings.TrimSpace(body.Content) == "" {
		response.SendBadRequest(c, "帖子内容不能为空")
		return
	}
	if len(body.Medias) > 9 {
		response.SendBadRequest(c, "图片最多 9 张")
		return
	}

	// 加载既有媒体集合：保留的旧媒体跳过归属校验，新增媒体必须归属于当前编辑者。
	// 这样圈主/管理员代编辑他人帖子时，保留原图不会因路径不归属当前用户而失败。
	existingDetail, _ := repository.PostRepo.FindDetailByID(database.Get(), postID)
	existingMediaSet := make(map[string]bool)
	if existingDetail != nil {
		for _, m := range existingDetail.Medias {
			existingMediaSet[m.URL] = true
		}
	}

	medias := make([]repository.CreatePostMediaInput, 0, len(body.Medias))
	for i := range body.Medias {
		inputURL := body.Medias[i].URL
		var validatedURL string
		if existingMediaSet[inputURL] {
			// 保留的旧媒体：直接归一化，跳过归属校验。
			validatedURL = mediaurl.NormalizePersistedMediaUrl(inputURL)
		} else {
			// 新增媒体：必须归属于当前编辑者，避免越权引用他人图片。
			url, err := mediaurl.ValidatePersistedImageUrlErr(inputURL, user.ID, "")
			if err != nil {
				response.SendBadRequest(c, err.Error())
				return
			}
			validatedURL = url
		}
		mediaType := body.Medias[i].Type
		if mediaType == "" {
			mediaType = "image"
		}
		medias = append(medias, repository.CreatePostMediaInput{
			Type: mediaType,
			URL:  validatedURL,
			Sort: body.Medias[i].Sort,
		})
	}

	var titlePtr *string
	if body.Title != "" {
		titlePtr = &body.Title
	}
	var linkPtr *string
	if body.LinkURL != "" {
		linkPtr = &body.LinkURL
	}
	postType := body.Type
	if postType == "" {
		postType = models.PostTypeText
	}

	updated, err := repository.PostRepo.Update(database.Get(), postID, repository.UpdatePostInput{
		Title:   titlePtr,
		Content: body.Content,
		LinkURL: linkPtr,
		Type:    postType,
		Medias:  medias,
	})
	if err != nil {
		response.SendInternalError(c)
		return
	}
	response.SendSuccess(c, updated)
}

// ListComments 处理 GET /api/posts/:id/comments。
// 分页查询帖子一级评论。
// @Summary      分页查询帖子评论
// @Description  公开接口（可选登录）。分页查询指定帖子的一级评论。
// @Tags         帖子
// @Accept       json
// @Produce      json
// @Param        id        path  int  true   "帖子 ID"
// @Param        page      query  int  false  "页码"
// @Param        pageSize  query  int  false  "每页数量"
// @Success      200  {object}  response.ApiResponse
// @Failure      400  {object}  response.ApiResponse  "帖子 ID 无效"
// @Failure      404  {object}  response.ApiResponse  "帖子不存在"
// @Router       /posts/{id}/comments [get]
func ListComments(c *gin.Context) {
	postID, ok := parseInt64Param(c, "id")
	if !ok {
		response.SendBadRequest(c, "帖子 ID 无效")
		return
	}
	post, err := repository.PostRepo.FindByID(database.Get(), postID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if post == nil || post.Status == models.PostStatusDeleted {
		response.SendNotFound(c, "帖子不存在")
		return
	}
	page, pageSize := parsePagination(c)
	list, total, err := repository.CommentRepo.FindByPost(database.Get(), postID, page, pageSize)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	response.SendPaginated(c, list, total, page, pageSize)
}

// CreateComment 处理 POST /api/posts/:id/comments。
// 创建评论，校验父评论与回复目标合法性，并派发通知。
// @Summary      创建评论
// @Description  需登录。在指定帖子下创建评论，校验父评论与回复目标合法性，并按规则派发通知。
// @Tags         帖子
// @Accept       json
// @Produce      json
// @Param        id    path  int     true  "帖子 ID"
// @Param        body  body  object  true  "创建评论请求"  Example({"content":"评论内容","parentId":1,"replyToId":2})
// @Success      201  {object}  response.ApiResponse
// @Failure      400  {object}  response.ApiResponse  "请求参数格式错误或父评论不合法"
// @Failure      404  {object}  response.ApiResponse  "帖子不存在"
// @Router       /posts/{id}/comments [post]
func CreateComment(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	postID, ok := parseInt64Param(c, "id")
	if !ok {
		response.SendBadRequest(c, "帖子 ID 无效")
		return
	}
	var body struct {
		Content    string `json:"content"`
		ParentID   *int64 `json:"parentId"`
		ReplyToID  *int64 `json:"replyToId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.SendBadRequest(c, "请求参数格式错误")
		return
	}
	if strings.TrimSpace(body.Content) == "" {
		response.SendBadRequest(c, "评论内容不能为空")
		return
	}

	post, err := repository.PostRepo.FindByID(database.Get(), postID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if post == nil || post.Status == models.PostStatusDeleted {
		response.SendNotFound(c, "帖子不存在")
		return
	}

	// 父评论校验：必须为当前帖子下的一级评论。
	if body.ParentID != nil {
		parent, err := repository.CommentRepo.FindByID(database.Get(), *body.ParentID)
		if err != nil {
			response.SendInternalError(c)
			return
		}
		if parent == nil || parent.PostID != postID || parent.ParentID != nil || parent.Status != models.CommentStatusNormal {
			response.SendBadRequest(c, "父评论不存在或不合法")
			return
		}
	}
	// 回复目标校验：必须为该线程的活跃参与者，且回复时必须同时提供 parentId。
	if body.ReplyToID != nil {
		if body.ParentID == nil {
			response.SendBadRequest(c, "回复评论时必须提供 parentId")
			return
		}
		active, err := repository.CommentRepo.IsActiveThreadParticipant(database.Get(), postID, *body.ReplyToID)
		if err != nil {
			response.SendInternalError(c)
			return
		}
		if !active {
			response.SendBadRequest(c, "回复目标不合法")
			return
		}
	}

	comment, err := repository.CommentRepo.Create(database.Get(), repository.CreateCommentInput{
		PostID:    postID,
		AuthorID:  user.ID,
		Content:   body.Content,
		ParentID:  body.ParentID,
		ReplyToID: body.ReplyToID,
	})
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if err := repository.PostRepo.IncrementCommentCount(database.Get(), postID, 1); err != nil {
		response.SendInternalError(c)
		return
	}

	// 派发评论通知：优先通知被回复者，否则通知父评论作者，否则通知帖子作者。
	notifyUserID := int64(0)
	if body.ReplyToID != nil {
		notifyUserID = *body.ReplyToID
	} else if body.ParentID != nil {
		parent, _ := repository.CommentRepo.FindByID(database.Get(), *body.ParentID)
		if parent != nil {
			notifyUserID = parent.AuthorID
		}
	} else {
		notifyUserID = post.AuthorID
	}
	if notifyUserID != 0 && notifyUserID != user.ID {
		title := user.Nickname + " 回复了你的评论"
		if body.ParentID == nil {
			title = user.Nickname + " 评论了你的帖子"
		}
		targetType := models.LikeTargetPost
		_, _ = repository.NotificationRepo.Create(database.Get(), repository.CreateNotificationInput{
			UserID:     notifyUserID,
			Type:       models.NotificationTypeComment,
			ActorID:    &user.ID,
			TargetType: &targetType,
			TargetID:   &postID,
			Title:      title,
			Content:    &body.Content,
		})
	}

	response.SendCreated(c, comment)
}

// LikePost 处理 POST /api/posts/:id/like。
// 点赞帖子，重复点赞返回 409，并给作者发点赞通知。
// @Summary      点赞帖子
// @Description  需登录。点赞帖子，重复点赞返回 409，并给作者发点赞通知。
// @Tags         帖子
// @Accept       json
// @Produce      json
// @Param        id  path  int  true  "帖子 ID"
// @Success      200  {object}  response.ApiResponse
// @Failure      400  {object}  response.ApiResponse  "帖子 ID 无效"
// @Failure      404  {object}  response.ApiResponse  "帖子不存在"
// @Failure      409  {object}  response.ApiResponse  "已经点赞过该帖子"
// @Router       /posts/{id}/like [post]
func LikePost(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	postID, ok := parseInt64Param(c, "id")
	if !ok {
		response.SendBadRequest(c, "帖子 ID 无效")
		return
	}
	post, err := repository.PostRepo.FindByID(database.Get(), postID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if post == nil || post.Status == models.PostStatusDeleted {
		response.SendNotFound(c, "帖子不存在")
		return
	}
	existing, err := repository.PostRepo.FindLike(database.Get(), user.ID, models.LikeTargetPost, postID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if existing != nil {
		response.SendError(c, 409, "ALREADY_LIKED", "已经点赞过该帖子")
		return
	}
	if err := repository.PostRepo.CreateLike(database.Get(), user.ID, models.LikeTargetPost, postID); err != nil {
		response.SendInternalError(c)
		return
	}
	if err := repository.PostRepo.IncrementLikeCount(database.Get(), postID, 1); err != nil {
		response.SendInternalError(c)
		return
	}

	// 点赞者非作者时给作者发通知。
	if post.AuthorID != user.ID {
		title := user.Nickname + " 赞了你的帖子"
		content := ""
		if post.Title != nil {
			content = *post.Title
		} else if len(post.Content) > 80 {
			content = post.Content[:80]
		} else {
			content = post.Content
		}
		targetType := models.LikeTargetPost
		_, _ = repository.NotificationRepo.Create(database.Get(), repository.CreateNotificationInput{
			UserID:     post.AuthorID,
			Type:       models.NotificationTypeLike,
			ActorID:    &user.ID,
			TargetType: &targetType,
			TargetID:   &postID,
			Title:      title,
			Content:    &content,
		})
	}

	updated, _ := repository.PostRepo.FindByID(database.Get(), postID)
	likeCount := 0
	if updated != nil {
		likeCount = updated.LikeCount
	}
	response.SendSuccess(c, gin.H{"liked": true, "likeCount": likeCount})
}

// UnlikePost 处理 DELETE /api/posts/:id/like。
// 取消点赞。
// @Summary      取消点赞
// @Description  需登录。取消点赞帖子。
// @Tags         帖子
// @Accept       json
// @Produce      json
// @Param        id  path  int  true  "帖子 ID"
// @Success      200  {object}  response.ApiResponse
// @Failure      400  {object}  response.ApiResponse  "帖子 ID 无效"
// @Failure      404  {object}  response.ApiResponse  "帖子不存在"
// @Failure      409  {object}  response.ApiResponse  "尚未点赞该帖子"
// @Router       /posts/{id}/like [delete]
func UnlikePost(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	postID, ok := parseInt64Param(c, "id")
	if !ok {
		response.SendBadRequest(c, "帖子 ID 无效")
		return
	}
	post, err := repository.PostRepo.FindByID(database.Get(), postID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if post == nil || post.Status == models.PostStatusDeleted {
		response.SendNotFound(c, "帖子不存在")
		return
	}
	existing, err := repository.PostRepo.FindLike(database.Get(), user.ID, models.LikeTargetPost, postID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if existing == nil {
		response.SendError(c, 409, "NOT_LIKED", "尚未点赞该帖子")
		return
	}
	if err := repository.PostRepo.DeleteLike(database.Get(), user.ID, models.LikeTargetPost, postID); err != nil {
		response.SendInternalError(c)
		return
	}
	if err := repository.PostRepo.IncrementLikeCount(database.Get(), postID, -1); err != nil {
		response.SendInternalError(c)
		return
	}
	updated, _ := repository.PostRepo.FindByID(database.Get(), postID)
	likeCount := 0
	if updated != nil {
		likeCount = updated.LikeCount
	}
	response.SendSuccess(c, gin.H{"liked": false, "likeCount": likeCount})
}

// SharePost 处理 POST /api/posts/:id/share。
// 记录转发意图，仅统计打开转发面板的行为。
// @Summary      记录转发统计
// @Description  公开接口。记录用户打开转发面板的行为，自增帖子转发计数。
// @Tags         帖子
// @Accept       json
// @Produce      json
// @Param        id  path  int  true  "帖子 ID"
// @Success      200  {object}  response.ApiResponse
// @Failure      400  {object}  response.ApiResponse  "帖子 ID 无效"
// @Failure      404  {object}  response.ApiResponse  "帖子不存在"
// @Router       /posts/{id}/share [post]
func SharePost(c *gin.Context) {
	postID, ok := parseInt64Param(c, "id")
	if !ok {
		response.SendBadRequest(c, "帖子 ID 无效")
		return
	}
	post, err := repository.PostRepo.FindByID(database.Get(), postID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if post == nil || post.Status == models.PostStatusDeleted {
		response.SendNotFound(c, "帖子不存在")
		return
	}
	if err := repository.PostRepo.IncrementShareCount(database.Get(), postID, 1); err != nil {
		response.SendInternalError(c)
		return
	}
	updated, _ := repository.PostRepo.FindByID(database.Get(), postID)
	shareCount := 0
	if updated != nil {
		shareCount = updated.ShareCount
	}
	response.SendSuccess(c, gin.H{"shareCount": shareCount})
}
