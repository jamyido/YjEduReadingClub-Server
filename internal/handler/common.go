// Package handler 实现 HTTP API 请求处理器，对应 Next.js 版的全部路由。
// 每个 handler 函数接收 *gin.Context，通过仓储层访问数据，通过 response 包返回统一响应。
package handler

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// defaultPageSize 是分页查询的默认每页数量。
const defaultPageSize = 20

// maxPageSize 是分页查询的每页数量上限。
const maxPageSize = 100

// parsePositiveInt 解析查询参数为正整数，失败或越界时返回默认值。
func parsePositiveInt(c *gin.Context, key string, defaultValue, maxValue int) int {
	raw := c.Query(key)
	if raw == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return defaultValue
	}
	if maxValue > 0 && value > maxValue {
		return maxValue
	}
	return value
}

// parsePagination 解析分页查询参数 page 与 pageSize。
// pageSize 上限为 maxPageSize，默认值为 defaultPageSize。
func parsePagination(c *gin.Context) (int, int) {
	page := parsePositiveInt(c, "page", 1, 0)
	pageSize := parsePositiveInt(c, "pageSize", defaultPageSize, maxPageSize)
	return page, pageSize
}

// parseBoolQuery 解析布尔型查询参数，"true"/"1" 视为真。
func parseBoolQuery(c *gin.Context, key string) bool {
	raw := strings.ToLower(strings.TrimSpace(c.Query(key)))
	return raw == "true" || raw == "1"
}

// parseInt64Param 从路径参数解析 int64，失败时返回 0 与 false。
func parseInt64Param(c *gin.Context, key string) (int64, bool) {
	raw := c.Param(key)
	if raw == "" {
		return 0, false
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 1 {
		return 0, false
	}
	return value, true
}

// parseInt64Query 从查询参数解析 int64，缺失或非法时返回 0。
func parseInt64Query(c *gin.Context, key string) int64 {
	raw := c.Query(key)
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 1 {
		return 0
	}
	return value
}
