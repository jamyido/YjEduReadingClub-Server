// Package middleware 提供 HTTP 中间件与认证上下文工具。
package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"yjedu-reading-club-server/internal/database"
	"yjedu-reading-club-server/internal/models"
	"yjedu-reading-club-server/internal/pkg/businessdate"
	jwtpkg "yjedu-reading-club-server/internal/pkg/jwt"
	"yjedu-reading-club-server/internal/repository"
)

// contextUserKey 是 gin.Context 中存储当前用户的键。
const contextUserKey = "currentUser"

// isoTimeLayout 是对外输出的 ISO 8601 时间格式（UTC）。
const isoTimeLayout = "2006-01-02T15:04:05Z"

// PublicUser 是过滤敏感字段后的公开用户信息。
type PublicUser struct {
	ID             int64   `json:"id"`
	Phone          string  `json:"phone"`
	Nickname       string  `json:"nickname"`
	Avatar         *string `json:"avatar"`
	Bio            *string `json:"bio"`
	Gender         string  `json:"gender"`
	Birthday       *string `json:"birthday"`
	Role           string  `json:"role"`
	Status         string  `json:"status"`
	StreakDays     int     `json:"streakDays"`
	LastCheckInAt  *string `json:"lastCheckInAt"`
	FollowingCount int     `json:"followingCount"`
	FollowerCount  int     `json:"followerCount"`
	CreatedAt      string  `json:"createdAt"`
	UpdatedAt      string  `json:"updatedAt"`
}

// formatTime 将时间格式化为 ISO 8601 字符串（UTC）。
func formatTime(t time.Time) string {
	return t.UTC().Format(isoTimeLayout)
}

// formatTimePtr 将可空时间格式化为 ISO 8601 字符串指针，nil 时返回 nil。
func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := formatTime(*t)
	return &s
}

// ToPublicUser 移除用户记录中的敏感字段，生成公开用户信息。
// 连续打卡天数根据上海时区当日有效性重新计算。
func ToPublicUser(user *models.User) PublicUser {
	return PublicUser{
		ID:             user.ID,
		Phone:          user.Phone,
		Nickname:       user.Nickname,
		Avatar:         user.Avatar,
		Bio:            user.Bio,
		Gender:         user.Gender,
		Birthday:       formatTimePtr(user.Birthday),
		Role:           user.Role,
		Status:         user.Status,
		StreakDays:     businessdate.GetEffectiveStreakDays(user.StreakDays, user.LastCheckInAt, time.Now()),
		LastCheckInAt:  formatTimePtr(user.LastCheckInAt),
		FollowingCount: user.FollowingCount,
		FollowerCount:  user.FollowerCount,
		CreatedAt:      formatTime(user.CreatedAt),
		UpdatedAt:      formatTime(user.UpdatedAt),
	}
}

// loadUserFromToken 解析 Authorization 头并加载数据库用户记录。
// token 无效或用户不存在时返回 nil。
func loadUserFromToken(c *gin.Context) *models.User {
	authHeader := c.GetHeader("Authorization")
	token := jwtpkg.ExtractBearerToken(authHeader)
	if token == "" {
		return nil
	}
	payload, err := jwtpkg.VerifyToken(token)
	if err != nil || payload == nil {
		return nil
	}
	user, err := repository.UserRepo.FindByID(database.Get(), payload.UserID)
	if err != nil || user == nil {
		return nil
	}
	if user.Status == models.UserStatusBanned {
		return nil
	}
	return user
}

// AuthRequired 是强制登录中间件。
// 未携带有效 Token 或用户不存在/被封禁时返回 401。
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := loadUserFromToken(c)
		if user == nil {
			c.AbortWithStatusJSON(401, gin.H{
				"success": false,
				"error":   gin.H{"code": "UNAUTHORIZED", "message": "未登录或登录已过期"},
			})
			return
		}
		c.Set(contextUserKey, user)
		c.Next()
	}
}

// OptionalAuth 是可选登录中间件。
// 携带有效 Token 时注入用户，未携带或无效时不拦截。
func OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := loadUserFromToken(c)
		if user != nil {
			c.Set(contextUserKey, user)
		}
		c.Next()
	}
}

// GetCurrentUser 从 gin.Context 取出当前登录用户。
// 未登录时返回 nil。
func GetCurrentUser(c *gin.Context) *models.User {
	value, exists := c.Get(contextUserKey)
	if !exists || value == nil {
		return nil
	}
	user, ok := value.(*models.User)
	if !ok {
		return nil
	}
	return user
}

// GetDB 从 gin.Context 取出事务或全局数据库连接。
// 若 context 中存在事务则使用事务，否则使用 database.Get()。
func GetDB(c *gin.Context) *gorm.DB {
	if tx, exists := c.Get("tx"); exists {
		if db, ok := tx.(*gorm.DB); ok {
			return db
		}
	}
	return database.Get()
}

// IsAdmin 判断当前用户是否为平台管理员。
func IsAdmin(user *models.User) bool {
	return user != nil && user.Role == models.RoleAdmin
}
