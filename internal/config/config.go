// Package config 负责加载并暴露后端服务的全局配置项。
// 配置来源优先级：环境变量 > .env.{env} 文件 > 默认值。
// 环境通过启动参数 --env=development|production 指定，默认 development。
package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// 支持的环境标识常量。
const (
	EnvDevelopment = "development"
	EnvProduction  = "production"
)

// Config 保存后端服务运行所需的全部配置项。
type Config struct {
	Env                  string // 运行环境：development / production
	DatabaseURL          string // PostgreSQL 连接字符串（DSN）
	JWTSecret            string // JWT 签名密钥
	JWTExpiresIn         string // 用户登录 Token 有效期（如 7d）
	Port                 string // HTTP 服务监听端口
	WechatMiniAppID      string // 微信小程序 AppID
	WechatMiniAppSecret  string // 微信小程序 AppSecret
	UploadDir            string // 上传文件根目录
	MaxUploadSizeMB      int64  // 单文件上传大小上限（MB）
	// AI 共读 provider 选择：coze / deepseek，留空时按已配置的密钥自动探测。
	AIProvider string
	// Coze 配置
	CozeToken  string // Coze AI 平台访问令牌
	CozeBotID  string // Coze AI 机器人 ID
	CzeBaseURL string // Coze API 基地址
	// DeepSeek 配置（OpenAI 兼容格式）
	DeepSeekAPIKey  string // DeepSeek API Key
	DeepSeekBaseURL string // DeepSeek API 基地址
	DeepSeekModel   string // DeepSeek 模型名
}

// appConfig 是全局单例配置，由 Load 初始化后供其他包读取。
var appConfig *Config

// ParseEnvFromFlags 解析命令行 --env 参数，返回环境标识。
// 合法取值：development / production，默认 development。
// 非法值会导致程序退出并打印用法。
func ParseEnvFromFlags() string {
	env := flag.String("env", EnvDevelopment, "运行环境：development 或 production")
	flag.Parse()
	if *env != EnvDevelopment && *env != EnvProduction {
		fmt.Fprintf(os.Stderr, "非法的 --env 值: %s（仅支持 development / production）\n", *env)
		os.Exit(2)
	}
	return *env
}

// Load 按指定环境加载配置并初始化全局单例。
// 加载顺序：先加载 .env.{env} 文件，再读取系统环境变量覆盖。
// .env 文件加载失败不阻断启动，生产环境可直接用系统环境变量。
func Load(env string) *Config {
	envFile := fmt.Sprintf(".env.%s", env)
	if err := godotenv.Load(envFile); err != nil {
		fmt.Fprintf(os.Stderr, "警告：未加载 %s 文件（%v），将依赖系统环境变量\n", envFile, err)
	}

	cfg := &Config{
		Env:                 env,
		DatabaseURL:         getEnv("DATABASE_URL", ""),
		JWTSecret:           getEnv("JWT_SECRET", ""),
		JWTExpiresIn:        getEnv("JWT_ACCESS_TOKEN_EXPIRES_IN", "7d"),
		Port:                getEnv("PORT", "3001"),
		WechatMiniAppID:     getEnv("WECHAT_MINI_APP_ID", ""),
		WechatMiniAppSecret: getEnv("WECHAT_MINI_APP_SECRET", ""),
		UploadDir:           getEnv("UPLOAD_DIR", "uploads"),
		MaxUploadSizeMB:     getEnvInt64("MAX_UPLOAD_SIZE_MB", 10),
		AIProvider:          getEnv("AI_PROVIDER", ""),
		CozeToken:           getEnv("COZE_TOKEN", ""),
		CozeBotID:           getEnv("COZE_BOT_ID", ""),
		CzeBaseURL:          getEnv("COZE_BASE_URL", "https://api.coze.cn"),
		DeepSeekAPIKey:      getEnv("DEEPSEEK_API_KEY", ""),
		DeepSeekBaseURL:     getEnv("DEEPSEEK_BASE_URL", "https://api.deepseek.com"),
		DeepSeekModel:       getEnv("DEEPSEEK_MODEL", "deepseek-chat"),
	}
	appConfig = cfg
	return cfg
}

// IsDevelopment 判断当前是否为开发环境。
func (c *Config) IsDevelopment() bool { return c.Env == EnvDevelopment }

// IsProduction 判断当前是否为生产环境。
func (c *Config) IsProduction() bool { return c.Env == EnvProduction }

// Get 返回已加载的全局配置，未调用 Load 前返回 nil。
func Get() *Config {
	return appConfig
}

// getEnv 读取环境变量，缺失时返回默认值。
func getEnv(key string, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt64 读取环境变量并解析为 int64，缺失或解析失败时返回默认值。
func getEnvInt64(key string, defaultValue int64) int64 {
	value, exists := os.LookupEnv(key)
	if !exists || value == "" {
		return defaultValue
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return defaultValue
	}
	return parsed
}
