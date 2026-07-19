// Package router 负责装配全部 HTTP API 路由与全局中间件。
// 路由层级与路径与原 Next.js 后端保持一致，便于前端无感切换。
package router

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"yjedu-reading-club-server/internal/config"
	"yjedu-reading-club-server/internal/handler"
	"yjedu-reading-club-server/internal/middleware"
)

// Setup 初始化 gin 引擎并装配全部路由，返回可启动的 *gin.Engine。
// 包含：CORS、日志、Recovery、静态文件、API 路由与 404 兜底。
func Setup() *gin.Engine {
	gin.SetMode(gin.Mode())
	engine := gin.New()

	// 全局中间件：Recovery + 简易日志 + CORS。
	engine.Use(gin.Recovery())
	engine.Use(gin.LoggerWithConfig(gin.LoggerConfig{SkipPaths: nil}))
	engine.Use(corsMiddleware())

	// 静态文件：上传目录与项目内置 assets。
	registerStaticRoutes(engine)

	// Swagger 文档：开发环境可访问 /swagger/* 浏览与导入 OpenAPI 文档。
	engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// API 路由。
	registerAPIRoutes(engine)

	// 404 兜底。
	engine.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   gin.H{"code": "NOT_FOUND", "message": "请求的资源不存在"},
		})
	})
	return engine
}

// corsMiddleware 处理跨域请求，允许前端开发态与生产域名访问。
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", c.GetHeader("Origin"))
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-New-Token")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Expose-Headers", "X-New-Token")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// registerStaticRoutes 注册 /uploads/* 与 /assets/* 静态文件路由。
// /uploads/* 直接从磁盘的上传根目录读取，/assets/* 从内置 assets 目录读取。
func registerStaticRoutes(engine *gin.Engine) {
	engine.GET("/uploads/*path", serveUploadFile)
	engine.GET("/assets/*path", serveAssetFile)
}

// serveUploadFile 处理 /uploads/* 请求，按用户目录读取上传文件。
// 防止路径穿越攻击：拒绝包含 .. 的路径。
func serveUploadFile(c *gin.Context) {
	serveSecureStatic(c, resolveUploadRoot())
}

// serveAssetFile 处理 /assets/* 请求，读取项目内置静态资源。
func serveAssetFile(c *gin.Context) {
	serveSecureStatic(c, resolveAssetRoot())
}

// serveSecureStatic 安全地返回指定根目录下的静态文件。
// 拒绝包含 .. 的相对路径，避免路径穿越。
func serveSecureStatic(c *gin.Context, root string) {
	relative := strings.TrimPrefix(c.Param("path"), "/")
	if relative == "" || strings.Contains(relative, "..") {
		c.Status(http.StatusNotFound)
		return
	}
	fullPath := filepath.Join(root, relative)
	http.ServeFile(c.Writer, c.Request, fullPath)
}

// resolveUploadRoot 推导上传根目录，优先使用配置项。
func resolveUploadRoot() string {
	cfg := config.Get()
	dir := "uploads"
	if cfg != nil && cfg.UploadDir != "" {
		dir = cfg.UploadDir
	}
	if filepath.IsAbs(dir) {
		return dir
	}
	cwd, err := os.Getwd()
	if err != nil {
		return dir
	}
	return filepath.Join(cwd, dir)
}

// resolveAssetRoot 推导项目内置 assets 根目录。
// 默认位于工程根的 assets/ 目录。
func resolveAssetRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "assets"
	}
	return filepath.Join(cwd, "assets")
}

// registerAPIRoutes 装配 /api/* 下的全部业务路由。
// 公开路由使用 OptionalAuth，需登录路由使用 AuthRequired。
func registerAPIRoutes(engine *gin.Engine) {
	api := engine.Group("/api")

	// 认证：登录/注册无需鉴权，其余需登录。
	auth := api.Group("/auth")
	{
		auth.POST("/login/phone", handler.LoginByPhone)
		auth.POST("/weapp/login", handler.WeappLogin)
		auth.POST("/weapp/phone", handler.WeappPhone)
		auth.POST("/weapp/bind", middleware.AuthRequired(), handler.WeappBind)
		auth.POST("/register", handler.Register)
		auth.POST("/change-password", middleware.AuthRequired(), handler.ChangePassword)
		auth.POST("/change-phone", middleware.AuthRequired(), handler.ChangePhone)
		auth.POST("/set-password", middleware.AuthRequired(), handler.SetPassword)
		auth.GET("/me", middleware.AuthRequired(), handler.GetMe)
	}

	// 帖子：列表与创建（创建需登录），详情/评论/点赞按需鉴权。
	posts := api.Group("/posts")
	{
		posts.GET("", middleware.OptionalAuth(), handler.ListPosts)
		posts.POST("", middleware.AuthRequired(), handler.CreatePost)
		posts.GET("/:id", middleware.OptionalAuth(), handler.GetPost)
		posts.PUT("/:id", middleware.AuthRequired(), handler.UpdatePost)
		posts.DELETE("/:id", middleware.AuthRequired(), handler.DeletePost)
		posts.GET("/:id/comments", middleware.OptionalAuth(), handler.ListComments)
		posts.POST("/:id/comments", middleware.AuthRequired(), handler.CreateComment)
		posts.POST("/:id/like", middleware.AuthRequired(), handler.LikePost)
		posts.DELETE("/:id/like", middleware.AuthRequired(), handler.UnlikePost)
		posts.POST("/:id/share", handler.SharePost)
		// AI 共读：基于帖子发起 AI 对话会话。
		posts.POST("/:id/ai-conversations", middleware.AuthRequired(), handler.CreateOrGetAIConversation)
	}

	// AI 共读会话：列表、详情、发消息（全部需登录）。
	aiConversations := api.Group("/ai-conversations", middleware.AuthRequired())
	{
		aiConversations.GET("", handler.ListAIConversations)
		aiConversations.GET("/:id", handler.GetAIConversation)
		aiConversations.POST("/:id/messages", handler.SendAIMessage)
	}

	// 圈子：列表/详情公开，加入/退出/创建等需登录。
	circles := api.Group("/circles")
	{
		circles.GET("", handler.ListCircles)
		circles.POST("", middleware.AuthRequired(), handler.CreateCircle)
		circles.GET("/mine", middleware.AuthRequired(), handler.ListMyCircles)
		circles.GET("/:id", middleware.OptionalAuth(), handler.GetCircle)
		circles.PUT("/:id", middleware.AuthRequired(), handler.UpdateCircle)
		circles.DELETE("/:id", middleware.AuthRequired(), handler.DeleteCircle)
		circles.POST("/:id/join", middleware.AuthRequired(), handler.JoinCircle)
		circles.POST("/:id/leave", middleware.AuthRequired(), handler.LeaveCircle)
		circles.GET("/:id/post-days-ranking", middleware.AuthRequired(), handler.GetPostDaysRanking)
		circles.PUT("/:id/members/:userId/role", middleware.AuthRequired(), handler.UpdateMemberRole)
	}

	// 话题：仅列表查询。
	api.GET("/topics", handler.ListTopics)

	// 打卡：列表公开，创建需登录。
	checkin := api.Group("/checkin")
	{
		checkin.GET("", handler.ListCheckIns)
		checkin.POST("", middleware.AuthRequired(), handler.CreateCheckIn)
	}

	// 课程：列表/详情公开，进度更新需登录。
	courses := api.Group("/courses")
	{
		courses.GET("", handler.ListCourses)
		courses.GET("/:id", middleware.OptionalAuth(), handler.GetCourse)
		courses.POST("/:id/progress", middleware.AuthRequired(), handler.UpdateCourseProgress)
	}

	// 私信：全部需登录。
	messages := api.Group("/messages", middleware.AuthRequired())
	{
		messages.GET("", handler.ListMessages)
		messages.POST("/read-all", handler.MarkAllMessagesRead)
		messages.GET("/:userId", handler.GetConversation)
		messages.POST("/:userId", handler.SendMessage)
	}

	// 通知：全部需登录。
	notifications := api.Group("/notifications", middleware.AuthRequired())
	{
		notifications.GET("", handler.ListNotifications)
		notifications.GET("/summary", handler.GetNotificationSummary)
		notifications.POST("/read", handler.MarkNotificationRead)
		notifications.POST("/dispatch", handler.DispatchNotification)
	}

	// 用户：资料更新/关注需登录，公开资料/粉丝/关注列表公开。
	users := api.Group("/users")
	{
		users.PUT("/profile", middleware.AuthRequired(), handler.UpdateProfile)
		users.GET("/:id", handler.GetUserProfile)
		users.POST("/:id/follow", middleware.AuthRequired(), handler.FollowUser)
		users.DELETE("/:id/follow", middleware.AuthRequired(), handler.UnfollowUser)
		users.GET("/:id/followers", handler.ListFollowers)
		users.GET("/:id/following", handler.ListFollowing)
	}

	// 上传：需登录。
	api.POST("/upload", middleware.AuthRequired(), handler.UploadFile)

	// 管理端：仅平台管理员可访问。
	admin := api.Group("/admin", middleware.AuthRequired())
	{
		// 手动触发定时总结任务（调试用）。type=daily|weekly|monthly
		admin.POST("/scheduler/trigger", handler.TriggerSchedulerTask)
	}
}
