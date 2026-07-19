// Package models 定义所有数据库表对应的 GORM 模型与枚举常量。
// 模型表名与字段名严格对齐 Prisma schema 中的 @@map / @map，保证与现有数据库兼容。
package models

// 性别枚举（对应 Prisma Gender）
const (
	GenderUnknown = "UNKNOWN"
	GenderMale    = "MALE"
	GenderFemale  = "FEMALE"
)

// 用户全局角色枚举（对应 Prisma UserRole）
const (
	RoleUser  = "USER"
	RoleAdmin = "ADMIN"
)

// 账号状态枚举（对应 Prisma UserStatus）
const (
	UserStatusActive = "ACTIVE"
	UserStatusBanned = "BANNED"
)

// 圈子成员角色枚举（对应 Prisma CircleMemberRole）
const (
	CircleRoleMember     = "MEMBER"
	CircleRoleModerator  = "MODERATOR"
	CircleRoleOwner      = "OWNER"
)

// 帖子类型枚举（对应 Prisma PostType）
const (
	PostTypeText  = "TEXT"
	PostTypeImage = "IMAGE"
	PostTypeVideo = "VIDEO"
	PostTypeLink  = "LINK"
)

// 私信消息类型枚举（对应 Prisma MessageType）
const (
	MessageTypeText  = "TEXT"
	MessageTypeImage = "IMAGE"
)

// 通知类型枚举（对应 Prisma NotificationType）
const (
	NotificationTypeLike         = "LIKE"
	NotificationTypeComment      = "COMMENT"
	NotificationTypeFollow       = "FOLLOW"
	NotificationTypeSystem       = "SYSTEM"
	NotificationTypeCircleInvite = "CIRCLE_INVITE"
	NotificationTypeTask         = "TASK"
)

// 帖子状态：0 正常 1 删除 2 审核中
const (
	PostStatusNormal   = 0
	PostStatusDeleted  = 1
	PostStatusReviewing = 2
)

// 评论状态：0 正常
const (
	CommentStatusNormal = 0
)

// 课程状态：0 草稿 1 已发布 2 已下架
const (
	CourseStatusDraft    = 0
	CourseStatusPublished = 1
	CourseStatusOffline  = 2
)

// 话题状态：0 停用 1 启用
const (
	TopicStatusDisabled = 0
	TopicStatusEnabled  = 1
)

// 点赞目标类型常量
const (
	LikeTargetPost    = "post"
	LikeTargetComment = "comment"
)
