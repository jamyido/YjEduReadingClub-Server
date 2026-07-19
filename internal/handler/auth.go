package handler

import (
	"github.com/gin-gonic/gin"

	"yjedu-reading-club-server/internal/database"
	"yjedu-reading-club-server/internal/middleware"
	"yjedu-reading-club-server/internal/models"
	"yjedu-reading-club-server/internal/pkg/jwt"
	"yjedu-reading-club-server/internal/pkg/mediaurl"
	"yjedu-reading-club-server/internal/pkg/nickname"
	"yjedu-reading-club-server/internal/pkg/password"
	"yjedu-reading-club-server/internal/pkg/response"
	"yjedu-reading-club-server/internal/pkg/wechat"
	"yjedu-reading-club-server/internal/repository"
)

// LoginByPhone 处理 POST /api/auth/login/phone。
// H5 端手机号 + 密码登录。
// @Summary      手机号密码登录
// @Description  H5 端使用手机号 + 密码登录，返回 JWT Token 与用户公开信息。
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        body  body      object  true  "登录请求"  Example({"phone":"13800000000","password":"12345678"})
// @Success      200   {object}  response.ApiResponse
// @Failure      400   {object}  response.ApiResponse  "手机号格式不正确"
// @Failure      401   {object}  response.ApiResponse  "手机号或密码错误"
// @Failure      403   {object}  response.ApiResponse  "账号已被封禁"
// @Router       /auth/login/phone [post]
func LoginByPhone(c *gin.Context) {
	var body struct {
		Phone    string `json:"phone"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.SendBadRequest(c, "请求参数格式错误")
		return
	}
	if !password.IsValidChinesePhone(body.Phone) {
		response.SendBadRequest(c, "手机号格式不正确")
		return
	}

	user, err := repository.UserRepo.FindByPhone(database.Get(), body.Phone)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if user == nil {
		response.SendError(c, 401, "INVALID_CREDENTIALS", "手机号或密码错误")
		return
	}
	if user.Status == models.UserStatusBanned {
		response.SendForbidden(c, "账号已被封禁")
		return
	}
	if user.Password == nil {
		response.SendError(c, 401, "PASSWORD_NOT_SET", "该账号尚未设置密码")
		return
	}
	if err := password.ComparePassword(body.Password, *user.Password); err != nil {
		response.SendError(c, 401, "INVALID_CREDENTIALS", "手机号或密码错误")
		return
	}

	token, err := signUserToken(user)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	response.SendSuccess(c, gin.H{
		"token":      token,
		"user":       middleware.ToPublicUser(user),
		"hasPassword": user.Password != nil,
	})
}

// WeappLogin 处理 POST /api/auth/weapp/login。
// 微信小程序一键登录第一步，用 code 换取 openid。
// @Summary      微信小程序登录第一步
// @Description  公开接口。微信小程序使用 code 调用 jscode2session 换取 openid，openid 已存在则直接登录并返回 JWT，否则签发临时 Token 引导前端授权手机号。
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        body  body      object  true  "登录请求"  Example({"code":"wx_login_code"})
// @Success      200   {object}  response.ApiResponse
// @Failure      400   {object}  response.ApiResponse  "缺少 code 参数或微信登录失败"
// @Router       /auth/weapp/login [post]
func WeappLogin(c *gin.Context) {
	var body struct {
		Code string `json:"code"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Code == "" {
		response.SendBadRequest(c, "缺少 code 参数")
		return
	}

	session, err := wechat.Code2Session(body.Code)
	if err != nil {
		response.SendError(c, 400, "WECHAT_LOGIN_FAILED", err.Error())
		return
	}

	// openid 已存在则直接登录。
	user, err := repository.UserRepo.FindByWeappOpenID(database.Get(), session.OpenID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if user != nil {
		token, err := signUserToken(user)
		if err != nil {
			response.SendInternalError(c)
			return
		}
		response.SendSuccess(c, gin.H{
			"needPhone":    false,
			"token":        token,
			"user":         middleware.ToPublicUser(user),
			"isNewUser":    false,
			"hasPassword":  user.Password != nil,
		})
		return
	}

	// openid 不存在则签发临时 Token，引导前端授权手机号。
	unionID := ""
	if session.UnionID != "" {
		unionID = session.UnionID
	}
	tempToken, err := jwt.SignWeappTempToken(jwt.WeappTempPayload{
		OpenID:     session.OpenID,
		SessionKey: session.SessionKey,
		UnionID:    unionID,
	})
	if err != nil {
		response.SendInternalError(c)
		return
	}
	response.SendSuccess(c, gin.H{
		"needPhone":  true,
		"tempToken":  tempToken,
		"message":    "请授权手机号完成注册",
	})
}

// WeappPhone 处理 POST /api/auth/weapp/phone。
// 微信一键登录第二步，用临时 Token + getPhoneNumber code 完成注册或绑定。
// @Summary      微信小程序登录第二步
// @Description  公开接口。使用临时 Token 与 getPhoneNumber 下发的 code 获取手机号，手机号已注册则绑定 openid，否则创建新用户，最终返回 JWT 与用户公开信息。
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        body  body      object  true  "授权手机号请求"  Example({"code":"phone_code","tempToken":"temp_token"})
// @Success      200   {object}  response.ApiResponse
// @Failure      400   {object}  response.ApiResponse  "缺少参数或获取手机号失败"
// @Failure      401   {object}  response.ApiResponse  "临时凭证无效或已过期"
// @Router       /auth/weapp/phone [post]
func WeappPhone(c *gin.Context) {
	var body struct {
		Code      string `json:"code"`
		TempToken string `json:"tempToken"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Code == "" || body.TempToken == "" {
		response.SendBadRequest(c, "缺少 code 或 tempToken 参数")
		return
	}

	payload, err := jwt.VerifyWeappTempToken(body.TempToken)
	if err != nil || payload == nil {
		response.SendError(c, 401, "INVALID_TEMP_TOKEN", "临时凭证无效或已过期")
		return
	}

	phone, err := wechat.GetPhoneNumber(body.Code)
	if err != nil {
		response.SendError(c, 400, "GET_PHONE_FAILED", err.Error())
		return
	}

	// 手机号已注册则绑定 openid。
	existing, err := repository.UserRepo.FindByPhone(database.Get(), phone)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	isNewUser := false
	var user *models.User
	if existing != nil {
		if err := repository.UserRepo.BindWeappOpenID(database.Get(), existing.ID, payload.OpenID, payload.UnionID); err != nil {
			response.SendInternalError(c)
			return
		}
		user, err = repository.UserRepo.FindByID(database.Get(), existing.ID)
		if err != nil {
			response.SendInternalError(c)
			return
		}
	} else {
		// 新用户创建无密码账号。
		openID := payload.OpenID
		var unionID *string
		if payload.UnionID != "" {
			s := payload.UnionID
			unionID = &s
		}
		user, err = repository.UserRepo.Create(database.Get(), repository.CreateUserInput{
			Phone:       phone,
			WeappOpenID: &openID,
			UnionID:     unionID,
			Nickname:    nickname.GenerateNickname(),
		})
		if err != nil {
			response.SendInternalError(c)
			return
		}
		isNewUser = true
	}

	token, err := signUserToken(user)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	response.SendSuccess(c, gin.H{
		"token":       token,
		"user":        middleware.ToPublicUser(user),
		"isNewUser":   isNewUser,
		"hasPassword": user.Password != nil,
	})
}

// WeappBind 处理 POST /api/auth/weapp/bind。
// 已通过手机号密码登录的用户，可携带微信登录第一步下发的 tempToken
// 将当前微信 openid 绑定到该账号，实现「绑定已有账号」流程。
// 需登录鉴权，tempToken 必须有效且 openid 未被其他账号占用。
// @Summary      绑定微信到当前账号
// @Description  需登录。已通过手机号密码登录的用户携带微信登录第一步下发的 tempToken，将微信 openid 绑定到当前账号；openid 已被其他账号占用时拒绝。
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        body  body      object  true  "绑定请求"  Example({"tempToken":"temp_token"})
// @Success      200   {object}  response.ApiResponse
// @Failure      400   {object}  response.ApiResponse  "缺少 tempToken 参数"
// @Failure      401   {object}  response.ApiResponse  "临时凭证无效或已过期"
// @Failure      409   {object}  response.ApiResponse  "该微信已绑定其他账号"
// @Router       /auth/weapp/bind [post]
func WeappBind(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	var body struct {
		TempToken string `json:"tempToken"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.TempToken == "" {
		response.SendBadRequest(c, "缺少 tempToken 参数")
		return
	}

	payload, err := jwt.VerifyWeappTempToken(body.TempToken)
	if err != nil || payload == nil {
		response.SendError(c, 401, "INVALID_TEMP_TOKEN", "临时凭证无效或已过期")
		return
	}

	// openid 防重复绑定：已被其他账号占用时拒绝。
	bindExist, err := repository.UserRepo.FindByWeappOpenID(database.Get(), payload.OpenID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if bindExist != nil && bindExist.ID != user.ID {
		response.SendError(c, 409, "OPENID_ALREADY_BOUND", "该微信已绑定其他账号")
		return
	}

	// 已是当前账号绑定的同一 openid，直接返回成功，避免重复写入。
	if bindExist != nil && bindExist.ID == user.ID {
		response.SendSuccess(c, gin.H{
			"user":        middleware.ToPublicUser(user),
			"hasPassword": user.Password != nil,
			"bound":       true,
		})
		return
	}

	if err := repository.UserRepo.BindWeappOpenID(database.Get(), user.ID, payload.OpenID, payload.UnionID); err != nil {
		response.SendInternalError(c)
		return
	}

	updated, err := repository.UserRepo.FindByID(database.Get(), user.ID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	response.SendSuccess(c, gin.H{
		"user":        middleware.ToPublicUser(updated),
		"hasPassword": updated.Password != nil,
		"bound":       true,
	})
}

// Register 处理 POST /api/auth/register。
// 手机号 + 密码注册，可选携带 tempToken 绑定 openid。
// @Summary      手机号注册
// @Description  公开接口。使用手机号 + 密码注册新用户，可选携带 tempToken 绑定微信 openid，注册成功后返回 JWT 与用户公开信息。
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        body  body      object  true  "注册请求"  Example({"phone":"13800000000","password":"12345678","nickname":"小明","avatar":"","tempToken":""})
// @Success      201   {object}  response.ApiResponse
// @Failure      400   {object}  response.ApiResponse  "手机号格式不正确或密码强度不足"
// @Failure      401   {object}  response.ApiResponse  "临时凭证无效或已过期"
// @Failure      409   {object}  response.ApiResponse  "手机号已注册或微信已绑定其他账号"
// @Router       /auth/register [post]
func Register(c *gin.Context) {
	var body struct {
		Phone     string `json:"phone"`
		Password  string `json:"password"`
		Nickname  string `json:"nickname"`
		Avatar    string `json:"avatar"`
		TempToken string `json:"tempToken"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.SendBadRequest(c, "请求参数格式错误")
		return
	}
	if !password.IsValidChinesePhone(body.Phone) {
		response.SendBadRequest(c, "手机号格式不正确")
		return
	}
	if err := password.ValidatePasswordStrength(body.Password); err != nil {
		response.SendBadRequest(c, err.Error())
		return
	}

	// 手机号防重复。
	existing, err := repository.UserRepo.FindByPhone(database.Get(), body.Phone)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if existing != nil {
		response.SendError(c, 409, "PHONE_ALREADY_REGISTERED", "该手机号已注册")
		return
	}

	// 可选 openid 绑定。
	var openID *string
	var unionID *string
	if body.TempToken != "" {
		payload, err := jwt.VerifyWeappTempToken(body.TempToken)
		if err != nil || payload == nil {
			response.SendError(c, 401, "INVALID_TEMP_TOKEN", "临时凭证无效或已过期")
			return
		}
		// openid 防重复绑定。
		bindExist, err := repository.UserRepo.FindByWeappOpenID(database.Get(), payload.OpenID)
		if err != nil {
			response.SendInternalError(c)
			return
		}
		if bindExist != nil {
			response.SendError(c, 409, "OPENID_ALREADY_BOUND", "该微信已绑定其他账号")
			return
		}
		openID = &payload.OpenID
		if payload.UnionID != "" {
			s := payload.UnionID
			unionID = &s
		}
	}

	// 头像 URL 校验归属（注册前无 userID，传 0 仅做格式校验）。
	avatarURL, err := mediaurl.ValidatePersistedImageUrlErr(body.Avatar, 0, "")
	if err != nil {
		response.SendBadRequest(c, err.Error())
		return
	}
	var avatarPtr *string
	if avatarURL != "" {
		avatarPtr = &avatarURL
	}

	hashed, err := password.HashPassword(body.Password)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	nick := body.Nickname
	if nick == "" {
		nick = nickname.GenerateNickname()
	}

	user, err := repository.UserRepo.Create(database.Get(), repository.CreateUserInput{
		Phone:       body.Phone,
		Password:    &hashed,
		WeappOpenID: openID,
		UnionID:     unionID,
		Nickname:    nick,
		Avatar:      avatarPtr,
	})
	if err != nil {
		response.SendInternalError(c)
		return
	}

	token, err := signUserToken(user)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	response.SendCreated(c, gin.H{
		"token":       token,
		"user":        middleware.ToPublicUser(user),
		"hasPassword": true,
	})
}

// ChangePassword 处理 POST /api/auth/change-password。
// 修改密码，需登录。
// @Summary      修改密码
// @Description  需登录。用户通过原密码验证身份后设置新密码，新密码需通过强度校验且不能与原密码相同。
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        body  body      object  true  "修改密码请求"  Example({"currentPassword":"old123456","newPassword":"new123456"})
// @Success      200   {object}  response.ApiResponse
// @Failure      400   {object}  response.ApiResponse  "参数缺失或新密码不合规"
// @Failure      401   {object}  response.ApiResponse  "原密码错误"
// @Router       /auth/change-password [post]
func ChangePassword(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	var body struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.SendBadRequest(c, "请求参数格式错误")
		return
	}
	if body.CurrentPassword == "" || body.NewPassword == "" {
		response.SendBadRequest(c, "请填写完整密码信息")
		return
	}
	if body.CurrentPassword == body.NewPassword {
		response.SendBadRequest(c, "新密码不能与原密码相同")
		return
	}
	if err := password.ValidatePasswordStrength(body.NewPassword); err != nil {
		response.SendBadRequest(c, err.Error())
		return
	}

	full, err := repository.UserRepo.FindByID(database.Get(), user.ID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if full.Password == nil {
		response.SendBadRequest(c, "当前账号尚未设置密码")
		return
	}
	if err := password.ComparePassword(body.CurrentPassword, *full.Password); err != nil {
		response.SendError(c, 401, "INVALID_CREDENTIALS", "原密码错误")
		return
	}

	hashed, err := password.HashPassword(body.NewPassword)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if err := repository.UserRepo.UpdatePassword(database.Get(), user.ID, hashed); err != nil {
		response.SendInternalError(c)
		return
	}
	updated, _ := repository.UserRepo.FindByID(database.Get(), user.ID)
	response.SendSuccess(c, gin.H{
		"user":        middleware.ToPublicUser(updated),
		"hasPassword": true,
	})
}

// ChangePhone 处理 POST /api/auth/change-phone。
// 更换手机号，需登录 + 密码验证身份。
// @Summary      更换手机号
// @Description  需登录。用户通过密码验证身份后更换手机号，新手机号需通过格式校验、不能与原手机号相同且未被其他账号占用。
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        body  body      object  true  "更换手机号请求"  Example({"newPhone":"13900000000","password":"12345678"})
// @Success      200   {object}  response.ApiResponse
// @Failure      400   {object}  response.ApiResponse  "手机号格式不正确或与原手机号相同"
// @Failure      401   {object}  response.ApiResponse  "密码错误"
// @Failure      409   {object}  response.ApiResponse  "该手机号已被其他账号占用"
// @Router       /auth/change-phone [post]
func ChangePhone(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	var body struct {
		NewPhone string `json:"newPhone"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.SendBadRequest(c, "请求参数格式错误")
		return
	}
	if !password.IsValidChinesePhone(body.NewPhone) {
		response.SendBadRequest(c, "手机号格式不正确")
		return
	}
	if body.NewPhone == user.Phone {
		response.SendBadRequest(c, "新手机号不能与原手机号相同")
		return
	}

	full, err := repository.UserRepo.FindByID(database.Get(), user.ID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if full.Password == nil {
		response.SendBadRequest(c, "当前账号尚未设置密码")
		return
	}
	if err := password.ComparePassword(body.Password, *full.Password); err != nil {
		response.SendError(c, 401, "INVALID_CREDENTIALS", "密码错误")
		return
	}

	// 新手机号防占用。
	occupied, err := repository.UserRepo.FindByPhone(database.Get(), body.NewPhone)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if occupied != nil {
		response.SendError(c, 409, "PHONE_ALREADY_REGISTERED", "该手机号已被其他账号占用")
		return
	}
	if err := repository.UserRepo.UpdatePhone(database.Get(), user.ID, body.NewPhone); err != nil {
		response.SendInternalError(c)
		return
	}
	updated, _ := repository.UserRepo.FindByID(database.Get(), user.ID)
	response.SendSuccess(c, gin.H{"user": middleware.ToPublicUser(updated)})
}

// SetPassword 处理 POST /api/auth/set-password。
// 为微信登录的无密码用户补充设置密码，需登录。
// 设置成功后重新签发 Token 并通过 X-New-Token 响应头下发。
// @Summary      设置密码
// @Description  需登录。为微信登录的无密码用户补充设置密码，密码需通过强度校验；设置成功后重新签发 Token 并通过 X-New-Token 响应头下发。
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        body  body      object  true  "设置密码请求"  Example({"password":"12345678"})
// @Success      200   {object}  response.ApiResponse
// @Failure      400   {object}  response.ApiResponse  "密码强度不足或账号已设置密码"
// @Router       /auth/set-password [post]
func SetPassword(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	var body struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.SendBadRequest(c, "请求参数格式错误")
		return
	}
	if err := password.ValidatePasswordStrength(body.Password); err != nil {
		response.SendBadRequest(c, err.Error())
		return
	}
	full, err := repository.UserRepo.FindByID(database.Get(), user.ID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if full.Password != nil {
		response.SendBadRequest(c, "当前账号已设置密码，请使用修改密码功能")
		return
	}

	hashed, err := password.HashPassword(body.Password)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	if err := repository.UserRepo.UpdatePassword(database.Get(), user.ID, hashed); err != nil {
		response.SendInternalError(c)
		return
	}
	updated, _ := repository.UserRepo.FindByID(database.Get(), user.ID)

	// 重新签发包含最新信息的 Token。
	token, err := signUserToken(updated)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	c.Header("X-New-Token", token)
	response.SendSuccess(c, gin.H{
		"user":        middleware.ToPublicUser(updated),
		"hasPassword": true,
	})
}

// GetMe 处理 GET /api/auth/me。
// 返回当前登录用户信息 + 未读私信数 + 未读通知数。
// @Summary      获取当前用户信息
// @Description  需登录。返回当前登录用户的公开信息、是否已设置密码、未读私信数与未读通知数。
// @Tags         认证
// @Accept       json
// @Produce      json
// @Success      200   {object}  response.ApiResponse
// @Failure      401   {object}  response.ApiResponse  "未登录或登录已过期"
// @Router       /auth/me [get]
func GetMe(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	unreadMessages, err := repository.MessageRepo.CountUnread(database.Get(), user.ID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	unreadNotifications, err := repository.NotificationRepo.CountUnread(database.Get(), user.ID)
	if err != nil {
		response.SendInternalError(c)
		return
	}
	response.SendSuccess(c, gin.H{
		"user":                middleware.ToPublicUser(user),
		"hasPassword":         user.Password != nil,
		"unreadMessages":      unreadMessages,
		"unreadNotifications": unreadNotifications,
	})
}

// signUserToken 为用户签发 JWT，失败时返回错误。
func signUserToken(user *models.User) (string, error) {
	return jwt.SignToken(jwt.JwtPayload{
		UserID: user.ID,
		Phone:  user.Phone,
		Role:   user.Role,
	})
}
