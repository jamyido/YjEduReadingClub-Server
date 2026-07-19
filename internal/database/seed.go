// Package database 的 seed.go 负责数据库表结构自动迁移与种子数据初始化。
// 启动时若 users 表为空，则创建管理员账户与 6 个初始圈子。
package database

import (
	"errors"
	"log"

	"gorm.io/gorm"

	"yjedu-reading-club-server/internal/models"
	"yjedu-reading-club-server/internal/pkg/password"
	"yjedu-reading-club-server/internal/repository"
)

// autoMigrateModels 是需要自动迁移的全部 GORM 模型。
// GORM AutoMigrate 只会加列加表，不会删除列或修改类型，对现有数据安全。
var autoMigrateModels = []interface{}{
	&models.User{},
	&models.Circle{},
	&models.CircleMember{},
	&models.Post{},
	&models.PostMedia{},
	&models.Comment{},
	&models.Like{},
	&models.Follow{},
	&models.Message{},
	&models.Notification{},
	&models.CheckIn{},
	&models.Course{},
	&models.CourseChapter{},
	&models.CourseProgress{},
	&models.Topic{},
	&models.AIConversation{},
	&models.AIMessage{},
}

// seedCircles 是初始化时创建的 6 个圈子数据。
// 每个圈子使用不同的蓝色系主题色，与用户蓝色偏好一致。
var seedCircles = []struct {
	Name        string
	Description string
	ThemeColor  string
}{
	{Name: "教师认知跃读圈", Description: "聚焦教师认知发展与专业成长，共读教育学、认知心理学相关著作。", ThemeColor: "#2563eb"},
	{Name: "文学经典玥读圈", Description: "共读中外文学经典，品味文字之美，探讨人性与时代。", ThemeColor: "#0ea5e9"},
	{Name: "乡土文化约读圈", Description: "阅读乡土文化、民俗风物、地方文献，传承地域文化。", ThemeColor: "#0891b2"},
	{Name: "校本课程越读圈", Description: "共读校本课程开发理论与实践，推动特色课程建设。", ThemeColor: "#4f46e5"},
	{Name: "亲子关系悦读圈", Description: "共读亲子教育、家庭教育书籍，构建和谐亲子关系。", ThemeColor: "#06b6d4"},
	{Name: "科研能力阅长圈", Description: "攻读教育科研方法、论文写作，提升学术研究能力。", ThemeColor: "#1d4ed8"},
}

// 种子管理员账户的固定信息。
const (
	seedAdminNickname = "嘉阅圈管理员"
	seedAdminPhone    = "13800000000"
	seedAdminPassword = "adminforjyq330324"
	seedAdminBio      = "嘉阅圈官方管理员账号"
)

// seedTopics 是初始化时创建的话题数据。
// AdminOnly=true 的话题仅管理员发帖时可选（本周精读、本月精读）；
// AdminOnly=false 的话题所有用户均可选（打卡挑战）。
var seedTopics = []struct {
	Slug        string
	Title       string
	Description string
	Sort        int
	AdminOnly   bool
}{
	{Slug: "weekly-reading", Title: "本周精读", Description: "拆解一本书的关键观点", Sort: 30, AdminOnly: true},
	{Slug: "monthly-reading", Title: "本月精读", Description: "深读一本经典著作", Sort: 20, AdminOnly: true},
	{Slug: "check-in-challenge", Title: "打卡挑战", Description: "记录每天的阅读与成长", Sort: 10, AdminOnly: false},
}

// Initialize 执行数据库自动迁移与种子数据初始化。
// 流程：1) AutoMigrate 全部模型表；2) 若 users 表为空则写入管理员与 6 个圈子；3) 确保 AI伴读 系统用户存在。
// 该函数幂等，已有数据时不会重复写入。
func Initialize(db *gorm.DB) error {
	if err := InitSchema(db); err != nil {
		return err
	}
	if err := SeedDatabase(db); err != nil {
		return err
	}
	return EnsureAIAssistantUser(db)
}

// InitSchema 执行数据库表结构自动迁移。
// GORM AutoMigrate 只会加列加表，不会删除列或修改类型，对现有数据安全。
func InitSchema(db *gorm.DB) error {
	if err := db.AutoMigrate(autoMigrateModels...); err != nil {
		return err
	}
	log.Println("数据库表结构已同步")
	return nil
}

// SeedDatabase 在数据库为空时写入初始种子数据。
// 包括：1 个管理员账户、6 个圈子、管理员作为 6 个圈子的圈主。
// 使用事务确保原子性，任一步失败则全部回滚。
func SeedDatabase(db *gorm.DB) error {
	empty, err := IsDatabaseEmpty(db)
	if err != nil {
		return err
	}
	if !empty {
		log.Println("数据库已存在用户，跳过种子数据初始化")
		return nil
	}

	return db.Transaction(func(tx *gorm.DB) error {
		adminID, err := createSeedAdmin(tx)
		if err != nil {
			return err
		}
		if err := createSeedCircles(tx, adminID); err != nil {
			return err
		}
		if err := createSeedTopics(tx); err != nil {
			return err
		}
		log.Println("种子数据初始化完成：管理员账户、6 个圈子与 3 个话题已创建")
		return nil
	})
}

// IsDatabaseEmpty 检查 users 表是否为空。
// users 表为空被视为数据库未初始化，触发种子数据写入。
func IsDatabaseEmpty(db *gorm.DB) (bool, error) {
	var userCount int64
	if err := db.Model(&models.User{}).Count(&userCount).Error; err != nil {
		return false, err
	}
	return userCount == 0, nil
}

// createSeedAdmin 创建管理员账户并返回其 ID。
// 密码使用 bcrypt 加盐哈希存储，role 为 ADMIN，status 为 ACTIVE。
// avatar 字段留空（nil），由前端使用本地兜底头像 avatar-studio.png 渲染，
// 避免小程序额外发起网络请求下载头像，减少带宽开销。
func createSeedAdmin(tx *gorm.DB) (int64, error) {
	hashed, err := password.HashPassword(seedAdminPassword)
	if err != nil {
		return 0, err
	}
	bio := seedAdminBio
	admin := models.User{
		Phone:    seedAdminPhone,
		Password: &hashed,
		Nickname: seedAdminNickname,
		Bio:      &bio,
		Gender:   models.GenderUnknown,
		Role:     models.RoleAdmin,
		Status:   models.UserStatusActive,
	}
	if err := tx.Create(&admin).Error; err != nil {
		return 0, err
	}
	log.Printf("管理员账户已创建：ID=%d, 昵称=%s, 手机号=%s\n", admin.ID, admin.Nickname, admin.Phone)
	return admin.ID, nil
}

// createSeedCircles 创建 6 个圈子，并将管理员以 OWNER 角色加入每个圈子。
// 同时维护各圈子的 member_count=1 与 owner_id 指向管理员。
func createSeedCircles(tx *gorm.DB, adminID int64) error {
	for _, c := range seedCircles {
		desc := c.Description
		theme := c.ThemeColor
		circle := models.Circle{
			Name:        c.Name,
			Description: &desc,
			ThemeColor:  &theme,
			IsPublic:    true,
			MemberCount: 1,
			OwnerID:     adminID,
		}
		if err := tx.Create(&circle).Error; err != nil {
			return err
		}
		member := models.CircleMember{
			UserID:   adminID,
			CircleID: circle.ID,
			Role:     models.CircleRoleOwner,
		}
		if err := tx.Create(&member).Error; err != nil {
			return err
		}
		log.Printf("圈子已创建：%s (ID=%d)\n", circle.Name, circle.ID)
	}
	return nil
}

// createSeedTopics 创建 3 个初始话题。
// 本周精读、本月精读仅管理员发帖可选；打卡挑战所有用户可选。
// 幂等：若 slug 已存在则跳过，不重复创建。
func createSeedTopics(tx *gorm.DB) error {
	for _, t := range seedTopics {
		var existing models.Topic
		if err := tx.Where("slug = ?", t.Slug).First(&existing).Error; err == nil {
			// slug 已存在，跳过。
			continue
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		desc := t.Description
		topic := models.Topic{
			Slug:        t.Slug,
			Title:       t.Title,
			Description: &desc,
			Status:      models.TopicStatusEnabled,
			Sort:        t.Sort,
			AdminOnly:   t.AdminOnly,
		}
		if err := tx.Create(&topic).Error; err != nil {
			return err
		}
		log.Printf("话题已创建：%s (ID=%d, adminOnly=%v)\n", topic.Title, topic.ID, topic.AdminOnly)
	}
	return nil
}

// EnsureAIAssistantUser 幂等创建 AI伴读 系统用户。
// 该用户作为定时总结帖（日/周/月精华）的统一作者身份出现，独立于种子初始化流程，
// 老版本升级时也会自动补建。失败时仅记录日志不阻断启动。
func EnsureAIAssistantUser(db *gorm.DB) error {
	aiID, err := repository.UserRepo.FindOrCreateAIAssistant(db)
	if err != nil {
		log.Printf("AI伴读 系统用户创建/查询失败: %v", err)
		return nil
	}
	log.Printf("AI伴读 系统用户就绪：ID=%d\n", aiID)
	return nil
}
