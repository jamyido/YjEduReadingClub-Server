package handler

import (
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"yjedu-reading-club-server/internal/database"
	"yjedu-reading-club-server/internal/middleware"
	"yjedu-reading-club-server/internal/models"
	"yjedu-reading-club-server/internal/pkg/mediaurl"
	"yjedu-reading-club-server/internal/pkg/response"
	"yjedu-reading-club-server/internal/repository"
)

// circleDetailResponse 是圈子详情响应，附带当前用户成员态。
type circleDetailResponse struct {
	repository.CircleDetailItem
	IsMember        bool    `json:"isMember"`
	MembershipRole  *string `json:"membershipRole"`
	CanViewAllPosts bool    `json:"canViewAllPosts"`
}

// buildCircleDetailResponse 根据当前登录用户构造圈子详情权限视图。
// 非成员仅获得圈子基础信息，成员名单只对成员与平台管理员开放。
func buildCircleDetailResponse(c *gin.Context, detail *repository.CircleDetailItem) circleDetailResponse {
	user := middleware.GetCurrentUser(c)
	isMember := false
	var role *string
	canViewAll := false
	if user != nil {
		membership, _ := repository.CircleRepo.FindMembership(database.Get(), user.ID, detail.ID)
		if membership != nil {
			isMember = true
			role = &membership.Role
		}
	}
	if isMember || middleware.IsAdmin(user) {
		canViewAll = true
	}
	resp := circleDetailResponse{
		CircleDetailItem: *detail,
		IsMember:         isMember,
		MembershipRole:   role,
		CanViewAllPosts:  canViewAll,
	}
	if !canViewAll {
		// 非成员隐藏成员名单。
		resp.Members = nil
	}
	return resp
}

// ListCircles 处理 GET /api/circles。
// 分页查询圈子列表。
// @Summary      分页查询圈子列表
// @Description  公开接口。支持按关键词搜索与按拥有者过滤，返回分页的圈子列表。
// @Tags         圈子
// @Accept       json
// @Produce      json
// @Param        keyword   query     string  false  "名称或描述关键词"
// @Param        ownerId   query     int     false  "拥有者用户 ID"
// @Param        page      query     int     false  "页码，默认 1"
// @Param        pageSize  query     int     false  "每页数量，默认 20"
// @Success      200       {object}  response.ApiResponse
// @Failure      500       {object}  response.ApiResponse  "服务器内部错误"
// @Router       /circles [get]
func ListCircles(c *gin.Context) {
	page, pageSize := parsePagination(c)
	opts := repository.CircleListOptions{
		Page:     page,
		PageSize: pageSize,
		Keyword:  strings.TrimSpace(c.Query("keyword")),
		OwnerID:  parseInt64Query(c, "ownerId"),
	}
	list, total, err := repository.CircleRepo.FindMany(database.Get(), opts)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	response.SendPaginated(c, list, total, opts.Page, opts.PageSize)
}

// CreateCircle 处理 POST /api/circles。
// 仅平台管理员可创建圈子。
// 创建后自动将管理员以 OWNER 角色加入 circle_members，并初始化成员计数为 1。
// @Summary      创建圈子
// @Description  需登录且需平台管理员权限。创建圈子后自动将当前管理员以 OWNER 角色加入并初始化成员计数为 1。
// @Tags         圈子
// @Accept       json
// @Produce      json
// @Param        body  body      object  true  "创建圈子请求"  Example({"name":"阅读小队","description":"一起读书","cover":"/uploads/circles/1.jpg","themeColor":"#7c3aed","isPublic":false})
// @Success      201   {object}  response.ApiResponse
// @Failure      400   {object}  response.ApiResponse  "请求参数格式错误或名称不合规"
// @Failure      403   {object}  response.ApiResponse  "仅平台管理员可创建圈子"
// @Router       /circles [post]
func CreateCircle(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if !middleware.IsAdmin(user) {
		response.SendForbidden(c, "仅平台管理员可创建圈子")
		return
	}
	var body struct {
		Name        string  `json:"name"`
		Description string  `json:"description"`
		Cover       string  `json:"cover"`
		ThemeColor  string  `json:"themeColor"`
		IsPublic    *bool   `json:"isPublic"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.SendBadRequest(c, "请求参数格式错误")
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		response.SendError(c, 400, "MISSING_NAME", "圈子名称不能为空")
		return
	}
	if len(name) > 100 {
		response.SendError(c, 400, "NAME_TOO_LONG", "圈子名称不能超过 100 个字符")
		return
	}

	input := repository.CreateCircleInput{
		Name:     name,
		OwnerID:  user.ID,
		IsPublic: false,
	}
	if body.Description != "" {
		desc := body.Description
		input.Description = &desc
	}
	if body.Cover != "" {
		// 创建圈子时还没有 circleId，用 userID 校验临时路径归属。
		coverURL, err := mediaurl.ValidatePersistedCircleCoverUrlErr(body.Cover, 0, user.ID, "")
		if err != nil {
			response.SendBadRequest(c, err.Error())
			return
		}
		input.Cover = &coverURL
	}
	if body.ThemeColor != "" {
		color := body.ThemeColor
		input.ThemeColor = &color
	}
	if body.IsPublic != nil {
		input.IsPublic = *body.IsPublic
	}

	// 使用事务确保圈子创建、成员加入与计数更新要么全部成功，要么全部回滚。
	var createdCircle *models.Circle
	db := database.Get()
	txErr := db.Transaction(func(tx *gorm.DB) error {
		circle, err := repository.CircleRepo.Create(tx, input)
		if err != nil {
			return err
		}
		// 创建者以 OWNER 身份加入圈子，便于后续在成员管理中转让圈主。
		if err := repository.CircleRepo.AddMember(tx, user.ID, circle.ID, models.CircleRoleOwner); err != nil {
			return err
		}
		// 初始化成员计数为 1。
		if err := repository.CircleRepo.IncrementMemberCount(tx, circle.ID, 1); err != nil {
			return err
		}
		createdCircle = circle
		return nil
	})
	if txErr != nil {
		response.SendInternalError(c)
		return
	}
	response.SendCreated(c, createdCircle, "圈子创建成功")
}

// GetCircle 处理 GET /api/circles/:id。
// 查询圈子详情，非成员隐藏成员名单。
// @Summary      获取圈子详情
// @Description  公开接口，可选登录。返回圈子详情，非成员或非管理员时隐藏成员名单。
// @Tags         圈子
// @Accept       json
// @Produce      json
// @Param        id    path      int     true   "圈子 ID"
// @Success      200   {object}  response.ApiResponse
// @Failure      400   {object}  response.ApiResponse  "无效的圈子 ID"
// @Failure      404   {object}  response.ApiResponse  "圈子不存在"
// @Router       /circles/{id} [get]
func GetCircle(c *gin.Context) {
	circleID, ok := parseInt64Param(c, "id")
	if !ok {
		response.SendError(c, 400, "INVALID_ID", "无效的圈子 ID")
		return
	}
	detail, err := repository.CircleRepo.FindDetailByID(database.Get(), circleID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if detail == nil {
		response.SendNotFound(c, "圈子不存在")
		return
	}
	response.SendSuccess(c, buildCircleDetailResponse(c, detail))
}

// UpdateCircle 处理 PUT /api/circles/:id。
// 仅圈子拥有者或平台管理员可更新。
// @Summary      更新圈子
// @Description  需登录。仅圈子拥有者或平台管理员可更新圈子信息，更新成功后返回最新详情。
// @Tags         圈子
// @Accept       json
// @Produce      json
// @Param        id    path      int     true   "圈子 ID"
// @Param        body  body      object  true   "更新圈子请求"  Example({"name":"阅读小队","description":"新的简介","cover":"/uploads/circles/2.jpg","themeColor":"#4c1d95","isPublic":true})
// @Success      200   {object}  response.ApiResponse
// @Failure      400   {object}  response.ApiResponse  "请求参数格式错误或无效的圈子 ID"
// @Failure      403   {object}  response.ApiResponse  "只有圈子拥有者或管理员才能更新圈子信息"
// @Failure      404   {object}  response.ApiResponse  "圈子不存在"
// @Router       /circles/{id} [put]
func UpdateCircle(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	circleID, ok := parseInt64Param(c, "id")
	if !ok {
		response.SendError(c, 400, "INVALID_ID", "无效的圈子 ID")
		return
	}
	circle, err := repository.CircleRepo.FindByID(database.Get(), circleID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if circle == nil {
		response.SendNotFound(c, "圈子不存在")
		return
	}
	if circle.OwnerID != user.ID && !middleware.IsAdmin(user) {
		response.SendForbidden(c, "只有圈子拥有者或管理员才能更新圈子信息")
		return
	}

	var body struct {
		Name        string  `json:"name"`
		Description string  `json:"description"`
		Cover       string  `json:"cover"`
		ThemeColor  string  `json:"themeColor"`
		IsPublic    *bool   `json:"isPublic"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.SendBadRequest(c, "请求参数格式错误")
		return
	}

	input := repository.UpdateCircleInput{}
	if trimmed := strings.TrimSpace(body.Name); trimmed != "" {
		input.Name = &trimmed
	}
	if body.Description != "" {
		desc := body.Description
		input.Description = &desc
	}
	if body.Cover != "" {
		existingCover := ""
		if circle.Cover != nil {
			existingCover = *circle.Cover
		}
		// 更新圈子封面时用 circleID 校验正式路径归属，同时兼容临时路径（userID）与已有地址。
		coverURL, err := mediaurl.ValidatePersistedCircleCoverUrlErr(body.Cover, circleID, user.ID, existingCover)
		if err != nil {
			response.SendBadRequest(c, err.Error())
			return
		}
		input.Cover = &coverURL
	}
	if body.ThemeColor != "" {
		color := body.ThemeColor
		input.ThemeColor = &color
	}
	if body.IsPublic != nil {
		input.IsPublic = body.IsPublic
	}

	if _, err := repository.CircleRepo.Update(database.Get(), circleID, input); err != nil {
		response.SendInternalError(c)
		return
	}
	detail, err := repository.CircleRepo.FindDetailByID(database.Get(), circleID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if detail == nil {
		response.SendNotFound(c, "圈子不存在")
		return
	}
	response.SendSuccess(c, buildCircleDetailResponse(c, detail), "圈子更新成功")
}

// DeleteCircle 处理 DELETE /api/circles/:id。
// 仅圈子拥有者或平台管理员可删除。
// @Summary      删除圈子
// @Description  需登录。仅圈子拥有者或平台管理员可删除圈子，删除成功后返回 deleted=true。
// @Tags         圈子
// @Accept       json
// @Produce      json
// @Param        id    path      int     true   "圈子 ID"
// @Success      200   {object}  response.ApiResponse
// @Failure      400   {object}  response.ApiResponse  "无效的圈子 ID"
// @Failure      403   {object}  response.ApiResponse  "只有圈子拥有者或管理员才能删除圈子"
// @Failure      404   {object}  response.ApiResponse  "圈子不存在"
// @Router       /circles/{id} [delete]
func DeleteCircle(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	circleID, ok := parseInt64Param(c, "id")
	if !ok {
		response.SendError(c, 400, "INVALID_ID", "无效的圈子 ID")
		return
	}
	circle, err := repository.CircleRepo.FindByID(database.Get(), circleID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if circle == nil {
		response.SendNotFound(c, "圈子不存在")
		return
	}
	if circle.OwnerID != user.ID && !middleware.IsAdmin(user) {
		response.SendForbidden(c, "只有圈子拥有者或管理员才能删除圈子")
		return
	}
	if err := repository.CircleRepo.Delete(database.Get(), circleID); err != nil {
		response.SendInternalError(c)
		return
	}
	response.SendSuccess(c, gin.H{"deleted": true}, "圈子已删除")
}

// ListMyCircles 处理 GET /api/circles/mine。
// 查询当前用户已加入的圈子列表。
// @Summary      获取当前用户已加入的圈子
// @Description  需登录。返回当前登录用户已加入的圈子列表。
// @Tags         圈子
// @Accept       json
// @Produce      json
// @Success      200   {object}  response.ApiResponse
// @Failure      401   {object}  response.ApiResponse  "未登录或登录已过期"
// @Router       /circles/mine [get]
func ListMyCircles(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	items, err := repository.CircleRepo.FindUserCircles(database.Get(), user.ID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	response.SendSuccess(c, items)
}

// JoinCircle 处理 POST /api/circles/:id/join。
// 校验圈子存在性以及是否已是成员，避免重复加入。
// @Summary      加入圈子
// @Description  需登录。当前用户以 MEMBER 角色加入指定圈子，已加入则返回 409，加入成功后成员计数 +1。
// @Tags         圈子
// @Accept       json
// @Produce      json
// @Param        id    path      int     true   "圈子 ID"
// @Success      200   {object}  response.ApiResponse
// @Failure      400   {object}  response.ApiResponse  "无效的圈子 ID"
// @Failure      404   {object}  response.ApiResponse  "圈子不存在"
// @Failure      409   {object}  response.ApiResponse  "已经是该圈子的成员"
// @Router       /circles/{id}/join [post]
func JoinCircle(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	circleID, ok := parseInt64Param(c, "id")
	if !ok {
		response.SendError(c, 400, "INVALID_ID", "无效的圈子 ID")
		return
	}
	circle, err := repository.CircleRepo.FindByID(database.Get(), circleID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if circle == nil {
		response.SendNotFound(c, "圈子不存在")
		return
	}
	membership, err := repository.CircleRepo.FindMembership(database.Get(), user.ID, circleID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if membership != nil {
		response.SendError(c, 409, "ALREADY_MEMBER", "已经是该圈子的成员")
		return
	}
	if err := repository.CircleRepo.AddMember(database.Get(), user.ID, circleID, models.CircleRoleMember); err != nil {
		response.SendInternalError(c)
		return
	}
	if err := repository.CircleRepo.IncrementMemberCount(database.Get(), circleID, 1); err != nil {
		response.SendInternalError(c)
		return
	}
	response.SendSuccess(c, gin.H{"joined": true}, "加入圈子成功")
}

// LeaveCircle 处理 POST /api/circles/:id/leave。
// 校验成员身份，拥有者不可退出。
// @Summary      退出圈子
// @Description  需登录。当前用户退出指定圈子，退出后成员计数 -1；圈子拥有者不可退出，需先转让圈子。
// @Tags         圈子
// @Accept       json
// @Produce      json
// @Param        id    path      int     true   "圈子 ID"
// @Success      200   {object}  response.ApiResponse
// @Failure      400   {object}  response.ApiResponse  "无效的圈子 ID 或圈子拥有者不能退出"
// @Failure      404   {object}  response.ApiResponse  "圈子不存在"
// @Failure      409   {object}  response.ApiResponse  "您不是该圈子的成员"
// @Router       /circles/{id}/leave [post]
func LeaveCircle(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	circleID, ok := parseInt64Param(c, "id")
	if !ok {
		response.SendError(c, 400, "INVALID_ID", "无效的圈子 ID")
		return
	}
	circle, err := repository.CircleRepo.FindByID(database.Get(), circleID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if circle == nil {
		response.SendNotFound(c, "圈子不存在")
		return
	}
	membership, err := repository.CircleRepo.FindMembership(database.Get(), user.ID, circleID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if membership == nil {
		response.SendError(c, 409, "NOT_MEMBER", "您不是该圈子的成员")
		return
	}
	if membership.Role == models.CircleRoleOwner {
		response.SendBadRequest(c, "圈子拥有者不能退出，请先转让圈子")
		return
	}
	if err := repository.CircleRepo.RemoveMember(database.Get(), user.ID, circleID); err != nil {
		response.SendInternalError(c)
		return
	}
	if err := repository.CircleRepo.IncrementMemberCount(database.Get(), circleID, -1); err != nil {
		response.SendInternalError(c)
		return
	}
	response.SendSuccess(c, gin.H{"left": true}, "已退出圈子")
}

// circlePostDaysRankingItem 是圈子累计发帖天数排名响应。
type circlePostDaysRankingItem struct {
	Rank               int     `json:"rank"`
	UserID             int64   `json:"userId"`
	Nickname           string  `json:"nickname"`
	Avatar             *string `json:"avatar"`
	CumulativePostDays int64   `json:"cumulativePostDays"`
	PostCount          int64   `json:"postCount"`
	LastPostAt         string  `json:"lastPostAt"`
}

// GetPostDaysRanking 处理 GET /api/circles/:id/post-days-ranking。
// 仅圈子成员或平台管理员可访问。
// @Summary      获取圈子累计发帖天数排行榜
// @Description  需登录。仅圈子成员或平台管理员可访问，按累计发帖天数降序返回成员排行，默认每页 50 条。
// @Tags         圈子
// @Accept       json
// @Produce      json
// @Param        id        path      int     true   "圈子 ID"
// @Param        page      query     int     false  "页码，默认 1"
// @Param        pageSize  query     int     false  "每页数量，默认 50，最大 100"
// @Success      200       {object}  response.ApiResponse
// @Failure      400       {object}  response.ApiResponse  "无效的圈子 ID"
// @Failure      403       {object}  response.ApiResponse  "加入圈子后才能查看累计发帖天数排行榜"
// @Failure      404       {object}  response.ApiResponse  "圈子不存在"
// @Router       /circles/{id}/post-days-ranking [get]
func GetPostDaysRanking(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	circleID, ok := parseInt64Param(c, "id")
	if !ok {
		response.SendError(c, 400, "INVALID_ID", "无效的圈子 ID")
		return
	}
	circle, err := repository.CircleRepo.FindByID(database.Get(), circleID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if circle == nil {
		response.SendNotFound(c, "圈子不存在")
		return
	}
	if !middleware.IsAdmin(user) {
		membership, err := repository.CircleRepo.FindMembership(database.Get(), user.ID, circleID)
		if err != nil {
			response.SendInternalError(c)
			return
		}
		if membership == nil {
			response.SendForbidden(c, "加入圈子后才能查看累计发帖天数排行榜")
			return
		}
	}

	// 排行榜默认每页 50 条。
	page := parsePositiveInt(c, "page", 1, 0)
	pageSize := parsePositiveInt(c, "pageSize", 50, 100)
	list, total, err := repository.PostRepo.FindCirclePostDaysRanking(database.Get(), circleID, page, pageSize)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	items := make([]circlePostDaysRankingItem, 0, len(list))
	for i := range list {
		rank := (page-1)*pageSize + i + 1
		items = append(items, circlePostDaysRankingItem{
			Rank:               rank,
			UserID:             list[i].UserID,
			Nickname:           list[i].Nickname,
			Avatar:             list[i].Avatar,
			CumulativePostDays: list[i].CumulativePostDays,
			PostCount:          list[i].PostCount,
			LastPostAt:         list[i].LastPostAt,
		})
	}
	response.SendPaginated(c, items, total, page, pageSize)
}

// UpdateMemberRole 处理 PUT /api/circles/:id/members/:userId/role。
// 仅圈子拥有者或平台管理员可操作；任命为圈主时执行拥有权转让。
// @Summary      更新圈子成员角色
// @Description  需登录。仅圈子拥有者或平台管理员可变更成员角色，角色取值为 MEMBER / MODERATOR / OWNER；任命为 OWNER 时执行拥有权转让，且不能变更自己的角色。
// @Tags         圈子
// @Accept       json
// @Produce      json
// @Param        id      path      int     true   "圈子 ID"
// @Param        userId  path      int     true   "目标用户 ID"
// @Param        body    body      object  true   "更新角色请求"  Example({"role":"MODERATOR"})
// @Success      200     {object}  response.ApiResponse
// @Failure      400     {object}  response.ApiResponse  "无效的 ID、不能变更自己的角色或无效的角色取值"
// @Failure      403     {object}  response.ApiResponse  "只有圈子拥有者或管理员才能变更成员角色"
// @Failure      404     {object}  response.ApiResponse  "圈子不存在"
// @Failure      409     {object}  response.ApiResponse  "该用户不是圈子成员"
// @Router       /circles/{id}/members/{userId}/role [put]
func UpdateMemberRole(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	circleID, ok := parseInt64Param(c, "id")
	if !ok {
		response.SendError(c, 400, "INVALID_ID", "无效的圈子 ID")
		return
	}
	targetUserID, ok := parseInt64Param(c, "userId")
	if !ok {
		response.SendError(c, 400, "INVALID_USER_ID", "无效的用户 ID")
		return
	}
	circle, err := repository.CircleRepo.FindByID(database.Get(), circleID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if circle == nil {
		response.SendNotFound(c, "圈子不存在")
		return
	}
	if circle.OwnerID != user.ID && !middleware.IsAdmin(user) {
		response.SendForbidden(c, "只有圈子拥有者或管理员才能变更成员角色")
		return
	}
	if targetUserID == user.ID {
		response.SendBadRequest(c, "不能变更自己的角色")
		return
	}
	targetMembership, err := repository.CircleRepo.FindMembership(database.Get(), targetUserID, circleID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if targetMembership == nil {
		response.SendError(c, 409, "NOT_MEMBER", "该用户不是圈子成员")
		return
	}

	var body struct {
		Role string `json:"role"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.SendBadRequest(c, "请求参数格式错误")
		return
	}
	role := strings.ToUpper(strings.TrimSpace(body.Role))
	switch role {
	case models.CircleRoleMember, models.CircleRoleModerator, models.CircleRoleOwner:
		// 合法角色。
	default:
		response.SendError(c, 400, "INVALID_ROLE", "无效的角色取值")
		return
	}

	if role == models.CircleRoleOwner {
		if err := repository.CircleRepo.TransferOwnership(database.Get(), circleID, targetUserID); err != nil {
			response.SendInternalError(c)
			return
		}
	} else {
		if err := repository.CircleRepo.UpdateMemberRole(database.Get(), targetMembership.ID, role); err != nil {
			response.SendInternalError(c)
			return
		}
	}
	response.SendSuccess(c, gin.H{"role": role}, "成员角色更新成功")
}
