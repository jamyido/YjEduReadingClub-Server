package repository

import (
	"errors"

	"gorm.io/gorm"

	"yjedu-reading-club-server/internal/models"
)

// CircleRepository 封装圈子及成员表的数据库操作。
type CircleRepository struct{}

// CircleRepo 是全局可用的圈子仓储单例。
var CircleRepo = CircleRepository{}

// CircleDetailItem 是圈子详情查询结果，包含圈主与成员信息。
type CircleDetailItem struct {
	models.Circle
	Owner   *UserBrief           `json:"owner"`
	Members []CircleMemberDetail `json:"members"`
}

// CircleMemberDetail 是圈子成员详情，包含用户基本信息。
type CircleMemberDetail struct {
	models.CircleMember
	User *UserBrief `json:"user"`
}

// CircleListItem 是圈子列表项，附带圈主昵称。
type CircleListItem struct {
	models.Circle
	OwnerNickname string `gorm:"column:owner_nickname" json:"ownerNickname"`
}

// FindByID 根据主键查询圈子。
func (CircleRepository) FindByID(db *gorm.DB, id int64) (*models.Circle, error) {
	var circle models.Circle
	if err := db.First(&circle, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &circle, nil
}

// FindDetailByID 查询圈子详情，预加载圈主与成员。
// 返回 CircleDetailItem 以承载圈主与成员关联数据。
func (CircleRepository) FindDetailByID(db *gorm.DB, id int64) (*CircleDetailItem, error) {
	var circle models.Circle
	if err := db.First(&circle, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	detail := &CircleDetailItem{Circle: circle}
	// 单独查询圈主，仅取公开字段，避免泄露手机号等敏感信息。
	detail.Owner = LoadUserBrief(db, circle.OwnerID)
	// 查询圈子成员列表，并补充用户基本信息。
	var members []models.CircleMember
	if err := db.Where("circle_id = ?", id).Order("created_at ASC").Find(&members).Error; err == nil {
		detail.Members = make([]CircleMemberDetail, 0, len(members))
		for i := range members {
			item := CircleMemberDetail{CircleMember: members[i]}
			item.User = LoadUserBrief(db, members[i].UserID)
			detail.Members = append(detail.Members, item)
		}
	}
	return detail, nil
}

// CircleListOptions 是圈子列表查询选项。
type CircleListOptions struct {
	Page     int
	PageSize int
	Keyword  string
	OwnerID  int64
}

// applyCircleFilters 将列表查询的过滤条件应用到给定查询。
// 返回该查询以便链式调用。
func applyCircleFilters(query *gorm.DB, opts CircleListOptions) *gorm.DB {
	if opts.Keyword != "" {
		like := "%" + opts.Keyword + "%"
		query = query.Where("name LIKE ? OR description LIKE ?", like, like)
	}
	if opts.OwnerID != 0 {
		query = query.Where("owner_id = ?", opts.OwnerID)
	}
	return query
}

// FindMany 分页查询圈子列表。
// 返回列表与总数。
func (CircleRepository) FindMany(db *gorm.DB, opts CircleListOptions) ([]CircleListItem, int64, error) {
	var total int64
	if err := applyCircleFilters(db.Model(&models.Circle{}), opts).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PageSize < 1 {
		opts.PageSize = 20
	}
	offset := (opts.Page - 1) * opts.PageSize
	var list []CircleListItem
	query := db.Table("circles").
		Select("circles.*, users.nickname AS owner_nickname").
		Joins("LEFT JOIN users ON users.id = circles.owner_id")
	query = applyCircleFilters(query, opts)
	err := query.Order("circles.created_at DESC").
		Offset(offset).Limit(opts.PageSize).
		Find(&list).Error
	return list, total, err
}

// FindMembership 查询用户在某圈子中的成员记录。
func (CircleRepository) FindMembership(db *gorm.DB, userID, circleID int64) (*models.CircleMember, error) {
	var member models.CircleMember
	err := db.Where("user_id = ? AND circle_id = ?", userID, circleID).First(&member).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &member, nil
}

// UserCircleItem 是用户已加入圈子的扁平视图。
type UserCircleItem struct {
	models.CircleMember
	Circle models.Circle `gorm:"foreignKey:CircleID" json:"circle"`
}

// FindUserCircles 查询用户已加入的圈子列表。
// 必须使用 Preload 加载 Circle 关联，否则 Circle 字段为零值（id=0），
// 会导致前端列表出现重复 key。
func (CircleRepository) FindUserCircles(db *gorm.DB, userID int64) ([]UserCircleItem, error) {
	var items []UserCircleItem
	err := db.Preload("Circle").Where("user_id = ?", userID).Order("created_at DESC").Find(&items).Error
	return items, err
}

// CreateCircleInput 是创建圈子的输入参数。
type CreateCircleInput struct {
	Name        string
	Description *string
	Cover       *string
	ThemeColor  *string
	IsPublic    bool
	OwnerID     int64
}

// Create 创建圈子。
func (CircleRepository) Create(db *gorm.DB, input CreateCircleInput) (*models.Circle, error) {
	circle := models.Circle{
		Name:        input.Name,
		Description: input.Description,
		Cover:       input.Cover,
		ThemeColor:  input.ThemeColor,
		IsPublic:    input.IsPublic,
		OwnerID:     input.OwnerID,
	}
	if err := db.Create(&circle).Error; err != nil {
		return nil, err
	}
	return &circle, nil
}

// UpdateCircleInput 是更新圈子的输入参数，仅更新非 nil 字段。
type UpdateCircleInput struct {
	Name        *string
	Description *string
	Cover       *string
	ThemeColor  *string
	IsPublic    *bool
}

// Update 更新圈子资料，仅更新实际传入字段。
func (CircleRepository) Update(db *gorm.DB, circleID int64, input UpdateCircleInput) (*models.Circle, error) {
	updates := map[string]interface{}{}
	if input.Name != nil {
		updates["name"] = *input.Name
	}
	if input.Description != nil {
		updates["description"] = *input.Description
	}
	if input.Cover != nil {
		updates["cover"] = *input.Cover
	}
	if input.ThemeColor != nil {
		updates["theme_color"] = *input.ThemeColor
	}
	if input.IsPublic != nil {
		updates["is_public"] = *input.IsPublic
	}
	if len(updates) > 0 {
		if err := db.Model(&models.Circle{}).Where("id = ?", circleID).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return CircleRepo.FindByID(db, circleID)
}

// Delete 删除圈子。
func (CircleRepository) Delete(db *gorm.DB, circleID int64) error {
	return db.Delete(&models.Circle{}, circleID).Error
}

// AddMember 添加圈子成员。
func (CircleRepository) AddMember(db *gorm.DB, userID, circleID int64, role string) error {
	member := models.CircleMember{
		UserID:   userID,
		CircleID: circleID,
		Role:     role,
	}
	return db.Create(&member).Error
}

// RemoveMember 移除圈子成员。
func (CircleRepository) RemoveMember(db *gorm.DB, userID, circleID int64) error {
	return db.Where("user_id = ? AND circle_id = ?", userID, circleID).
		Delete(&models.CircleMember{}).Error
}

// IncrementMemberCount 调整圈子成员计数，delta 可为负。
func (CircleRepository) IncrementMemberCount(db *gorm.DB, circleID int64, delta int) error {
	return db.Model(&models.Circle{}).Where("id = ?", circleID).
		UpdateColumn("member_count", gorm.Expr("member_count + ?", delta)).Error
}

// IncrementPostCount 调整圈子帖子计数，delta 可为负。
func (CircleRepository) IncrementPostCount(db *gorm.DB, circleID int64, delta int) error {
	return db.Model(&models.Circle{}).Where("id = ?", circleID).
		UpdateColumn("post_count", gorm.Expr("post_count + ?", delta)).Error
}

// UpdateMemberRole 更新成员角色。
func (CircleRepository) UpdateMemberRole(db *gorm.DB, memberID int64, role string) error {
	return db.Model(&models.CircleMember{}).Where("id = ?", memberID).
		Update("role", role).Error
}

// TransferOwnership 转让圈子拥有权：新拥有者升为 OWNER，原拥有者降为 MEMBER。
func (CircleRepository) TransferOwnership(db *gorm.DB, circleID, newOwnerID int64) error {
	return db.Transaction(func(tx *gorm.DB) error {
		// 原拥有者降级为普通成员。
		if err := tx.Model(&models.CircleMember{}).
			Where("circle_id = ? AND role = ?", circleID, models.CircleRoleOwner).
			Update("role", models.CircleRoleMember).Error; err != nil {
			return err
		}
		// 新拥有者升级。
		if err := tx.Model(&models.CircleMember{}).
			Where("circle_id = ? AND user_id = ?", circleID, newOwnerID).
			Update("role", models.CircleRoleOwner).Error; err != nil {
			return err
		}
		// 更新圈子的 owner_id。
		return tx.Model(&models.Circle{}).Where("id = ?", circleID).
			Update("owner_id", newOwnerID).Error
	})
}

// FindNotificationRecipientIDs 查询圈子所有正常成员的用户 ID。
func (CircleRepository) FindNotificationRecipientIDs(db *gorm.DB, circleID int64) ([]int64, error) {
	var ids []int64
	err := db.Model(&models.CircleMember{}).
		Where("circle_id = ?", circleID).
		Pluck("user_id", &ids).Error
	return ids, err
}

// FindAllIDs 查询所有圈子的 ID 列表，按创建时间升序。
// 供定时调度器遍历全部圈子生成 AI 总结帖使用。
func (CircleRepository) FindAllIDs(db *gorm.DB) ([]int64, error) {
	var ids []int64
	if err := db.Model(&models.Circle{}).Order("created_at ASC").Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}
