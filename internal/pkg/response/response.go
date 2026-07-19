// Package response 封装统一的 HTTP API 响应结构与快捷构造函数。
// 所有 handler 通过本包返回标准化的 JSON 响应。
package response

import "github.com/gin-gonic/gin"

// ApiResponse 是统一响应结构体。
type ApiResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
	Error   *ApiError   `json:"error,omitempty"`
}

// ApiError 是错误响应的详细内容。
type ApiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// PaginatedData 是分页列表响应的数据结构。
type PaginatedData struct {
	List     interface{} `json:"list"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"pageSize"`
	HasMore  bool        `json:"hasMore"`
}

// SendSuccess 发送成功响应，默认 HTTP 200。
func SendSuccess(c *gin.Context, data interface{}, message ...string) {
	msg := ""
	if len(message) > 0 {
		msg = message[0]
	}
	c.JSON(200, ApiResponse{
		Success: true,
		Data:    data,
		Message: msg,
	})
}

// SendCreated 发送创建成功响应，HTTP 201。
func SendCreated(c *gin.Context, data interface{}, message ...string) {
	msg := ""
	if len(message) > 0 {
		msg = message[0]
	}
	c.JSON(201, ApiResponse{
		Success: true,
		Data:    data,
		Message: msg,
	})
}

// SendPaginated 发送分页列表响应，HTTP 200。
// 自动计算 hasMore 字段。
func SendPaginated(c *gin.Context, list interface{}, total int64, page, pageSize int) {
	c.JSON(200, ApiResponse{
		Success: true,
		Data: PaginatedData{
			List:     list,
			Total:    total,
			Page:     page,
			PageSize: pageSize,
			HasMore:  int64(page*pageSize) < total,
		},
	})
}

// SendError 发送错误响应，可自定义错误码与 HTTP 状态码。
func SendError(c *gin.Context, statusCode int, code, message string) {
	c.JSON(statusCode, ApiResponse{
		Success: false,
		Error:   &ApiError{Code: code, Message: message},
	})
}

// SendBadRequest 发送 400 错误响应。
func SendBadRequest(c *gin.Context, message string) {
	SendError(c, 400, "BAD_REQUEST", message)
}

// SendUnauthorized 发送 401 未认证响应。
func SendUnauthorized(c *gin.Context, message ...string) {
	msg := "未登录或登录已过期"
	if len(message) > 0 {
		msg = message[0]
	}
	SendError(c, 401, "UNAUTHORIZED", msg)
}

// SendForbidden 发送 403 无权限响应。
func SendForbidden(c *gin.Context, message ...string) {
	msg := "无权限执行此操作"
	if len(message) > 0 {
		msg = message[0]
	}
	SendError(c, 403, "FORBIDDEN", msg)
}

// SendNotFound 发送 404 未找到响应。
func SendNotFound(c *gin.Context, message ...string) {
	msg := "资源不存在"
	if len(message) > 0 {
		msg = message[0]
	}
	SendError(c, 404, "NOT_FOUND", msg)
}

// SendInternalError 发送 500 服务器内部错误响应。
func SendInternalError(c *gin.Context, message ...string) {
	msg := "服务器内部错误"
	if len(message) > 0 {
		msg = message[0]
	}
	SendError(c, 500, "INTERNAL_ERROR", msg)
}
