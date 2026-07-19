package handler

import (
	"github.com/gin-gonic/gin"

	"yjedu-reading-club-server/internal/database"
	"yjedu-reading-club-server/internal/pkg/response"
	"yjedu-reading-club-server/internal/repository"
)

// ListTopics 处理 GET /api/topics。
// 分页查询启用状态话题，附带各话题下的帖子数。
// @Summary      话题列表
// @Description  公开接口。分页查询启用状态的话题列表，附带各话题下的帖子数，支持按 query 关键词搜索话题名称。
// @Tags         话题
// @Accept       json
// @Produce      json
// @Param        page      query     int     false  "页码，默认 1"               default(1)
// @Param        pageSize  query     int     false  "每页数量，默认 20，最大 100"  default(20)
// @Param        query     query     string  false  "话题名称搜索关键词"
// @Success      200  {object}  response.ApiResponse
// @Router       /topics [get]
func ListTopics(c *gin.Context) {
	page, pageSize := parsePagination(c)
	result, total, err := repository.TopicRepo.FindMany(database.Get(), repository.TopicListOptions{
		Page:     page,
		PageSize: pageSize,
		Query:    c.Query("query"),
	})
	if err != nil {
		response.SendInternalError(c)
		return
	}
	response.SendPaginated(c, result, total, page, pageSize)
}
