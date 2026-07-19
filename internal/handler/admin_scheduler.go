package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	"yjedu-reading-club-server/internal/middleware"
	"yjedu-reading-club-server/internal/pkg/response"
	"yjedu-reading-club-server/internal/scheduler"
)

// TriggerSchedulerTask 处理 POST /api/admin/scheduler/trigger。
// 仅平台管理员可手动触发定时总结任务，用于调试与验证。
// 查询参数 type 取值：
//   - daily：执行每日共性问题总结
//   - weekly：执行每周精华总结
//   - monthly：执行每月精华总结
//
// 任务在独立协程中异步执行，接口立即返回；执行结果通过服务日志查看。
// @Summary      触发定时任务
// @Description  需登录且需管理员。手动触发定时总结任务（daily/weekly/monthly），任务在独立协程中异步执行，接口立即返回，执行结果通过服务日志查看。
// @Tags         管理后台-定时任务
// @Accept       json
// @Produce      json
// @Param        type  query     string  true  "任务类型（daily/weekly/monthly）"
// @Success      200  {object}  response.ApiResponse
// @Failure      400  {object}  response.ApiResponse  "type 参数必须为 daily / weekly / monthly"
// @Failure      401  {object}  response.ApiResponse  "未登录或登录已过期"
// @Failure      403  {object}  response.ApiResponse  "仅平台管理员可触发定时任务"
// @Router       /admin/scheduler/trigger [post]
func TriggerSchedulerTask(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if !middleware.IsAdmin(user) {
		response.SendForbidden(c, "仅平台管理员可触发定时任务")
		return
	}

	taskType := strings.ToLower(strings.TrimSpace(c.Query("type")))
	switch taskType {
	case "daily":
		go scheduler.RunDailyDigest()
	case "weekly":
		go scheduler.RunWeeklyDigest()
	case "monthly":
		go scheduler.RunMonthlyDigest()
	default:
		response.SendBadRequest(c, "type 参数必须为 daily / weekly / monthly")
		return
	}

	response.SendSuccess(c, gin.H{
		"triggered": true,
		"type":      taskType,
	}, "任务已触发，执行结果请查看服务日志（前缀 [scheduler."+taskType+"]）")
}
