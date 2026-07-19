package handler

import (
	"github.com/gin-gonic/gin"

	"yjedu-reading-club-server/internal/database"
	"yjedu-reading-club-server/internal/middleware"
	"yjedu-reading-club-server/internal/pkg/response"
	"yjedu-reading-club-server/internal/repository"
)

// ListCourses 处理 GET /api/courses。
// 分页查询课程列表。
// @Summary      课程列表
// @Description  公开接口。分页查询课程列表，可按圈子 ID 与创建者 ID 过滤。
// @Tags         课程
// @Accept       json
// @Produce      json
// @Param        circleId   query     int64   false  "圈子 ID"
// @Param        creatorId  query     int64   false  "创建者 ID"
// @Param        page       query     int     false  "页码，默认 1"    default(1)
// @Param        pageSize   query     int     false  "每页数量，默认 20"  default(20)
// @Success      200  {object}  response.ApiResponse
// @Failure      500  {object}  response.ApiResponse  "服务器内部错误"
// @Router       /courses [get]
func ListCourses(c *gin.Context) {
	page, pageSize := parsePagination(c)
	opts := repository.CourseListOptions{
		Page:      page,
		PageSize:  pageSize,
		CircleID:  parseInt64Query(c, "circleId"),
		CreatorID: parseInt64Query(c, "creatorId"),
	}
	list, total, err := repository.CourseRepo.FindMany(database.Get(), opts)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	response.SendPaginated(c, list, total, opts.Page, opts.PageSize)
}

// GetCourse 处理 GET /api/courses/:id。
// 查询课程详情（含章节），登录用户附带学习进度。
// @Summary      课程详情
// @Description  公开接口，可选登录。返回课程详情（含章节），登录用户附带学习进度，未登录时 progress 为 nil。
// @Tags         课程
// @Accept       json
// @Produce      json
// @Param        id   path      int64   true  "课程 ID"
// @Success      200  {object}  response.ApiResponse
// @Failure      400  {object}  response.ApiResponse  "课程 ID 不合法"
// @Failure      404  {object}  response.ApiResponse  "课程不存在"
// @Router       /courses/{id} [get]
func GetCourse(c *gin.Context) {
	courseID, ok := parseInt64Param(c, "id")
	if !ok {
		response.SendError(c, 400, "INVALID_COURSE_ID", "课程 ID 不合法")
		return
	}
	course, err := repository.CourseRepo.FindByID(database.Get(), courseID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if course == nil {
		response.SendNotFound(c, "课程不存在")
		return
	}

	// 登录用户附带学习进度，未登录时 progress 为 nil。
	user := middleware.GetCurrentUser(c)
	if user != nil {
		p, err := repository.CourseRepo.FindProgress(database.Get(), user.ID, courseID)
		if err != nil {
			response.SendInternalError(c)
			return
		}
		course.Progress = p
	}
	response.SendSuccess(c, course)
}

// UpdateCourseProgress 处理 POST /api/courses/:id/progress。
// 更新当前用户在某课程的学习进度。
// @Summary      更新课程学习进度
// @Description  需登录。更新当前用户在某课程的学习进度，可传入当前章节、已完成章节列表、进度百分比与是否完成标记。
// @Tags         课程
// @Accept       json
// @Produce      json
// @Param        id    path      int64   true  "课程 ID"
// @Param        body  body      object  true  "进度更新请求"  Example({"currentChapterId":2,"completedChapterIds":"1,2,3","progress":60,"isCompleted":false})
// @Success      200  {object}  response.ApiResponse
// @Failure      400  {object}  response.ApiResponse  "请求参数格式错误或课程 ID 不合法"
// @Failure      401  {object}  response.ApiResponse  "未登录或登录已过期"
// @Failure      404  {object}  response.ApiResponse  "课程不存在"
// @Router       /courses/{id}/progress [post]
func UpdateCourseProgress(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	courseID, ok := parseInt64Param(c, "id")
	if !ok {
		response.SendError(c, 400, "INVALID_COURSE_ID", "课程 ID 不合法")
		return
	}
	course, err := repository.CourseRepo.FindByID(database.Get(), courseID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if course == nil {
		response.SendNotFound(c, "课程不存在")
		return
	}

	var body struct {
		CurrentChapterID    *int64  `json:"currentChapterId"`
		CompletedChapterIDs *string `json:"completedChapterIds"`
		Progress            *int    `json:"progress"`
		IsCompleted         *bool   `json:"isCompleted"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.SendBadRequest(c, "请求参数格式错误")
		return
	}

	updated, err := repository.CourseRepo.UpsertProgress(database.Get(), user.ID, courseID, repository.UpsertProgressInput{
		CurrentChapterID:    body.CurrentChapterID,
		CompletedChapterIDs: body.CompletedChapterIDs,
		Progress:            body.Progress,
		IsCompleted:         body.IsCompleted,
	})
	if err != nil {
		response.SendInternalError(c)
		return
	}
	response.SendSuccess(c, updated)
}
