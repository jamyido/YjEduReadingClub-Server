// Package ai 封装多平台 AI 服务接入。
// 职责：
//   - 定义 Provider 接口，统一不同 AI 平台（Coze / DeepSeek / 等）的调用方式
//   - 根据 config.AIProvider 自动选择已配置的 provider
//   - 提供共读场景的系统提示词构造
//
// 当前已接入：
//   - Coze（扣子，国内版） https://www.coze.cn
//   - DeepSeek（深度求索，OpenAI 兼容格式） https://api.deepseek.com
package ai

import (
	"errors"
	"strings"

	"yjedu-reading-club-server/internal/config"
)

// ChatMessage 是与 AI 交互的统一消息结构。
// 各 Provider 在内部将其转换为本平台所需的请求格式。
type ChatMessage struct {
	Content     string `json:"content"`
	ContentType string `json:"content_type"` // 固定 "text"
	Role        string `json:"role"`         // "user" / "assistant"
	Type        string `json:"type"`         // Coze 用 "question" / "answer"；DeepSeek 不使用
}

// Provider 是 AI 平台抽象接口。
// 各实现需根据自身 API 格式调用，返回 AI 的完整文本回复。
// systemPrompt 为系统提示词（共读场景上下文），messages 为历史对话（不含 system 消息）。
// userID 为终端用户标识，供 AI 平台做用户隔离或计费。
type Provider interface {
	// Name 返回 provider 标识（coze / deepseek）。
	Name() string
	// Chat 调用 AI 获取回复。
	Chat(systemPrompt string, messages []ChatMessage, userID string) (string, error)
}

// ProviderNone 未配置任何 provider 时的占位实现，调用时返回错误。
type ProviderNone struct{}

// Name 返回 none。
func (ProviderNone) Name() string { return "none" }

// Chat 始终返回未配置错误。
func (ProviderNone) Chat(systemPrompt string, messages []ChatMessage, userID string) (string, error) {
	return "", errors.New("AI provider 未配置：请在 .env 中设置 AI_PROVIDER=coze 或 deepseek，并提供对应密钥")
}

// defaultProvider 是由 SelectProvider 初始化的全局单例。
var defaultProvider Provider = ProviderNone{}

// SelectProvider 根据配置初始化全局默认 provider。
// 优先级：显式 AI_PROVIDER > 已配置密钥的 provider（DeepSeek 优先）> ProviderNone。
// 该函数应在服务启动时调用一次。
func SelectProvider() Provider {
	cfg := config.Get()
	if cfg == nil {
		defaultProvider = ProviderNone{}
		return defaultProvider
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.AIProvider))
	switch provider {
	case "coze":
		defaultProvider = NewCozeProvider(cfg)
		return defaultProvider
	case "deepseek":
		defaultProvider = NewDeepSeekProvider(cfg)
		return defaultProvider
	case "":
		// 未显式指定，按已配置密钥自动探测：DeepSeek 优先（更稳定）。
		if cfg.DeepSeekAPIKey != "" {
			defaultProvider = NewDeepSeekProvider(cfg)
			return defaultProvider
		}
		if cfg.CozeToken != "" && cfg.CozeBotID != "" {
			defaultProvider = NewCozeProvider(cfg)
			return defaultProvider
		}
		defaultProvider = ProviderNone{}
		return defaultProvider
	default:
		// 未知 provider 名，回退到 none。
		defaultProvider = ProviderNone{}
		return defaultProvider
	}
}

// Get 返回当前默认 provider。未调用 SelectProvider 前返回 ProviderNone。
func Get() Provider {
	return defaultProvider
}

// Chat 是便捷调用：直接使用默认 provider 发起对话。
// 等价于 Get().Chat(systemPrompt, messages, userID)。
func Chat(systemPrompt string, messages []ChatMessage, userID string) (string, error) {
	return Get().Chat(systemPrompt, messages, userID)
}

// BuildSystemPrompt 构造 AI 共读的系统提示词。
// 融入帖子标题、内容、圈子名等信息，引导 AI 基于全书内容提供精准问答、
// 标注原文出处、延伸背景知识、反问引导思辨、还原作者思维逻辑与语言风格。
// 所有 provider 共用此提示词。
func BuildSystemPrompt(postTitle, postContent, circleName, topicTitle string) string {
	var b strings.Builder
	b.WriteString("你是「嘉阅圈」的 AI 共读助手，专注于与读者深度共读一本书。")
	b.WriteString("你的职责是基于全书内容提供精准问答，回答时：\n")
	b.WriteString("1. 标注原文出处（章节、页码或段落定位）；\n")
	b.WriteString("2. 延伸补充相关的背景知识、作者生平、时代语境；\n")
	b.WriteString("3. 通过反问引导用户深度思辨，激发批判性思考；\n")
	b.WriteString("4. 必要时还原作者的思维逻辑与语言风格，让读者身临其境；\n")
	b.WriteString("5. 保持温和、严谨、富有启发的语调，避免空泛套话。\n\n")
	b.WriteString("当前共读上下文：\n")
	if postTitle != "" {
		b.WriteString("帖子标题：" + postTitle + "\n")
	}
	if circleName != "" {
		b.WriteString("所属圈子：" + circleName + "\n")
	}
	if topicTitle != "" {
		b.WriteString("关联话题：" + topicTitle + "\n")
	}
	if postContent != "" {
		b.WriteString("帖子内容：\n" + postContent + "\n")
	}
	b.WriteString("\n请基于以上内容理解读者的阅读书目与思考方向，展开共读对话。")
	return b.String()
}
