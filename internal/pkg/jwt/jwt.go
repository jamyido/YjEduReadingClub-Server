// Package jwt 封装 JSON Web Token 的签发与校验能力。
// 提供正式用户登录 Token 与微信登录临时 Token 两套凭证。
package jwt

import (
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"yjedu-reading-club-server/internal/config"
)

// JwtPayload 是正式用户登录 Token 的载荷结构。
type JwtPayload struct {
	UserID int64  `json:"userId"`
	Phone  string `json:"phone"`
	Role   string `json:"role"`
}

// WeappTempPayload 是微信登录中间态临时 Token 的载荷结构。
type WeappTempPayload struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"sessionKey"`
	UnionID    string `json:"unionid,omitempty"`
}

// jwtClaims 内部统一适配 jwt.RegisteredClaims 的用户登录声明。
type jwtClaims struct {
	JwtPayload
	jwt.RegisteredClaims
}

// weappTempClaims 内部适配微信临时 Token 的声明。
type weappTempClaims struct {
	WeappTempPayload
	jwt.RegisteredClaims
}

// getSecret 获取 JWT 签名密钥，未配置时返回错误。
func getSecret() ([]byte, error) {
	cfg := config.Get()
	if cfg == nil || cfg.JWTSecret == "" {
		return nil, errors.New("JWT_SECRET 环境变量未配置")
	}
	return []byte(cfg.JWTSecret), nil
}

// parseDuration 将 "7d"/"10m"/"3600s" 等字符串解析为 time.Duration。
// 支持 d（天）、h（时）、m（分）、s（秒）后缀，便于配置 Token 有效期。
func parseDuration(expr string) time.Duration {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return 7 * 24 * time.Hour
	}
	// 末尾为 d 时按天数换算。
	if strings.HasSuffix(expr, "d") {
		daysStr := strings.TrimSuffix(expr, "d")
		days, err := time.ParseDuration(daysStr + "h")
		if err != nil {
			return 7 * 24 * time.Hour
		}
		return days * 24
	}
	d, err := time.ParseDuration(expr)
	if err != nil {
		return 7 * 24 * time.Hour
	}
	return d
}

// SignToken 签发正式用户登录 Token。
// 有效期取自 JWT_ACCESS_TOKEN_EXPIRES_IN，默认 7 天。
func SignToken(payload JwtPayload) (string, error) {
	secret, err := getSecret()
	if err != nil {
		return "", err
	}
	cfg := config.Get()
	claims := jwtClaims{
		JwtPayload: payload,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(parseDuration(cfg.JWTExpiresIn))),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

// SignWeappTempToken 签发微信登录临时 Token，有效期固定 10 分钟。
// 仅用于获取手机号前的中间态，携带 openid 与 session_key。
func SignWeappTempToken(payload WeappTempPayload) (string, error) {
	secret, err := getSecret()
	if err != nil {
		return "", err
	}
	claims := weappTempClaims{
		WeappTempPayload: payload,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

// VerifyToken 验证并解析正式用户登录 Token，失败返回错误。
func VerifyToken(tokenString string) (*JwtPayload, error) {
	secret, err := getSecret()
	if err != nil {
		return nil, err
	}
	claims := &jwtClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		return secret, nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("token 无效")
	}
	return &claims.JwtPayload, nil
}

// VerifyWeappTempToken 验证并解析微信临时 Token，失败返回错误。
func VerifyWeappTempToken(tokenString string) (*WeappTempPayload, error) {
	secret, err := getSecret()
	if err != nil {
		return nil, err
	}
	claims := &weappTempClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		return secret, nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("临时 token 无效")
	}
	return &claims.WeappTempPayload, nil
}

// ExtractBearerToken 从 Authorization 头中提取 Bearer Token。
// 格式不正确时返回空字符串。
func ExtractBearerToken(authHeader string) string {
	if authHeader == "" {
		return ""
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return strings.TrimSpace(parts[1])
	}
	return ""
}
