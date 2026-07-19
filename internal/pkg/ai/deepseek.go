// deepseek.go 实现 DeepSeek 平台的 Provider。
// DeepSeek API 兼容 OpenAI 格式：POST /chat/completions
// 文档：https://api-docs.deepseek.com/zh-cn/
// 直接使用 system role 注入系统提示词，比 Coze 更简洁稳定。

package ai

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"yjedu-reading-club-server/internal/config"
)

// DeepSeekProvider 是 DeepSeek 平台的 AI provider 实现。
type DeepSeekProvider struct {
	apiKey  string
	baseURL string
	model   string
}

// NewDeepSeekProvider 根据配置创建 DeepSeekProvider。
// 若必填配置缺失，仍返回实例，Chat 时返回错误。
func NewDeepSeekProvider(cfg *config.Config) *DeepSeekProvider {
	return &DeepSeekProvider{
		apiKey:  cfg.DeepSeekAPIKey,
		baseURL: cfg.DeepSeekBaseURL,
		model:   cfg.DeepSeekModel,
	}
}

// Name 返回 provider 标识。
func (p *DeepSeekProvider) Name() string { return "deepseek" }

// deepSeekMessage 是 OpenAI 格式消息结构。
type deepSeekMessage struct {
	Role    string `json:"role"`    // system / user / assistant
	Content string `json:"content"`
}

// deepSeekRequest 是 OpenAI 兼容 chat/completions 请求体。
type deepSeekRequest struct {
	Model    string             `json:"model"`
	Messages []deepSeekMessage  `json:"messages"`
	Stream   bool               `json:"stream"`
}

// deepSeekResponse 是非流式 chat/completions 响应体。
type deepSeekResponse struct {
	Choices []struct {
		Message      deepSeekMessage `json:"message"`
		FinishReason string          `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// deepSeekHTTPClient 复用 HTTP 客户端，超时 120 秒以适配 AI 长回复。
var deepSeekHTTPClient = &http.Client{Timeout: 120 * time.Second}

// Chat 调用 DeepSeek /chat/completions 接口（非流式）获取 AI 回复。
// 采用非流式简化实现：服务端等 DeepSeek 完整回复后一次性返回，避免 SSE 解析复杂度。
// systemPrompt 通过 OpenAI 标准 system role 注入。
func (p *DeepSeekProvider) Chat(systemPrompt string, messages []ChatMessage, userID string) (string, error) {
	if p.apiKey == "" {
		return "", errors.New("DeepSeek 配置缺失：请检查 DEEPSEEK_API_KEY")
	}
	model := p.model
	if model == "" {
		model = "deepseek-chat"
	}

	// 构造 OpenAI 格式 messages：system 在首，其后接历史对话。
	dsMessages := make([]deepSeekMessage, 0, len(messages)+1)
	if systemPrompt != "" {
		dsMessages = append(dsMessages, deepSeekMessage{
			Role:    "system",
			Content: systemPrompt,
		})
	}
	for i := range messages {
		msg := messages[i]
		// DeepSeek 仅识别 system / user / assistant 三种 role，统一映射。
		role := msg.Role
		if role != "user" && role != "assistant" && role != "system" {
			role = "user"
		}
		dsMessages = append(dsMessages, deepSeekMessage{
			Role:    role,
			Content: msg.Content,
		})
	}

	reqBody := deepSeekRequest{
		Model:    model,
		Messages: dsMessages,
		Stream:   false,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("构造 DeepSeek 请求失败: %w", err)
	}

	baseURL := p.baseURL
	if baseURL == "" {
		baseURL = "https://api.deepseek.com"
	}
	// DeepSeek 端点为 /chat/completions（OpenAI 兼容）。
	url := strings.TrimRight(baseURL, "/") + "/chat/completions"

	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("创建 DeepSeek 请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := deepSeekHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("调用 DeepSeek 接口失败: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取 DeepSeek 响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("DeepSeek 接口返回非 200 状态码: %d, body: %s", resp.StatusCode, string(raw))
	}

	var data deepSeekResponse
	if err := json.Unmarshal(raw, &data); err != nil {
		return "", fmt.Errorf("解析 DeepSeek 响应失败: %w, body: %s", err, string(raw))
	}

	// 优先处理 API 返回的错误结构。
	if data.Error != nil && data.Error.Message != "" {
		return "", fmt.Errorf("DeepSeek 对话失败: %s (type=%s code=%s)", data.Error.Message, data.Error.Type, data.Error.Code)
	}

	if len(data.Choices) == 0 {
		return "", fmt.Errorf("DeepSeek 未返回任何 choices，原始响应: %s", string(raw))
	}

	content := data.Choices[0].Message.Content
	if content == "" {
		return "", fmt.Errorf("DeepSeek 返回内容为空，finish_reason=%s", data.Choices[0].FinishReason)
	}
	return content, nil
}
