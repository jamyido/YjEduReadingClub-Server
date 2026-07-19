// coze.go 实现 Coze（扣子）平台的 Provider。
// 接口：POST /v3/chat（SSE 流式），参考 https://www.coze.cn/docs/developer_guides/chat_v3
// Coze 没有 system role，系统提示词作为首条 user 消息注入。

package ai

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"yjedu-reading-club-server/internal/config"
)

// CozeProvider 是 Coze 平台的 AI provider 实现。
type CozeProvider struct {
	token   string
	botID   string
	baseURL string
}

// NewCozeProvider 根据配置创建 CozeProvider。
// 若必填配置缺失，仍返回实例，Chat 时返回错误（便于运行时切换 provider）。
func NewCozeProvider(cfg *config.Config) *CozeProvider {
	return &CozeProvider{
		token:   cfg.CozeToken,
		botID:   cfg.CozeBotID,
		baseURL: cfg.CzeBaseURL,
	}
}

// Name 返回 provider 标识。
func (p *CozeProvider) Name() string { return "coze" }

// cozeChatRequest 是 Coze /v3/chat 接口的请求体。
type cozeChatRequest struct {
	BotID              string        `json:"bot_id"`
	UserID             string        `json:"user_id"`
	AdditionalMessages []ChatMessage `json:"additional_messages"`
	Stream             bool          `json:"stream"`
	AutoSaveHistory    bool          `json:"auto_save_history"`
}

// cozeMessageData 是 conversation.message.delta / completed 事件的 data 字段。
type cozeMessageData struct {
	Role        string `json:"role"`
	Type        string `json:"type"`
	Content     string `json:"content"`
	ContentType string `json:"content_type"`
}

// cozeChatData 是 conversation.chat.* 事件的 data 字段。
type cozeChatData struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	LastError *struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	} `json:"last_error"`
}

// cozeLegacyError 兼容直接返回 code/msg 的旧错误格式。
type cozeLegacyError struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// cozeMaxAdditionalMessages Coze additional_messages 数组最大长度为 100。
const cozeMaxAdditionalMessages = 100

// cozeHTTPClient 是 Coze 复用的 HTTP 客户端，超时 120 秒以适配 AI 长回复。
var cozeHTTPClient = &http.Client{Timeout: 120 * time.Second}

// Chat 调用 Coze /v3/chat 流式接口，拼装完整回复后返回。
// 系统提示词作为首条 user 消息注入（Coze 不支持 system role）。
func (p *CozeProvider) Chat(systemPrompt string, messages []ChatMessage, userID string) (string, error) {
	if p.token == "" || p.botID == "" {
		return "", errors.New("Coze 配置缺失：请检查 COZE_TOKEN 与 COZE_BOT_ID")
	}

	// 构造 additional_messages：系统提示作为首条 user 消息，其后接历史对话。
	additionalMessages := make([]ChatMessage, 0, len(messages)+1)
	if systemPrompt != "" {
		additionalMessages = append(additionalMessages, ChatMessage{
			Content:     systemPrompt,
			ContentType: "text",
			Role:        "user",
			Type:        "question",
		})
	}
	additionalMessages = append(additionalMessages, messages...)

	// Coze 限制 additional_messages 数组最多 100 条，超出则保留最近的消息。
	if len(additionalMessages) > cozeMaxAdditionalMessages {
		additionalMessages = additionalMessages[len(additionalMessages)-cozeMaxAdditionalMessages:]
	}

	reqBody := cozeChatRequest{
		BotID:              p.botID,
		UserID:             userID,
		AdditionalMessages: additionalMessages,
		Stream:             true,
		AutoSaveHistory:    false,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("构造 Coze 请求失败: %w", err)
	}

	baseURL := p.baseURL
	if baseURL == "" {
		baseURL = "https://api.coze.cn"
	}
	// Coze v3 发起对话接口。
	url := strings.TrimRight(baseURL, "/") + "/v3/chat"

	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("创建 Coze 请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := cozeHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("调用 Coze 接口失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Coze 接口返回非 200 状态码: %d, body: %s", resp.StatusCode, string(raw))
	}

	// 读取整个 SSE 流用于调试与解析。
	rawBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取 Coze 响应失败: %w", err)
	}
	log.Printf("[ai.coze] raw SSE response (len=%d):\n%s", len(rawBytes), string(rawBytes))

	return parseCozeStream(bytes.NewReader(rawBytes))
}

// parseCozeStream 解析 Coze 的 SSE 流，提取 assistant 的完整回复文本。
// Coze SSE 流的格式为：
//   event: conversation.message.delta
//   data: {"role":"assistant","type":"answer","content":"片段","content_type":"text"}
//
// 处理 conversation.message.delta（增量）与 conversation.message.completed（完成）事件，
// 为避免 Coze 配置差异导致回复丢失，对 role/type 字段做宽容处理：
// 优先采纳 type=answer 的 assistant 消息，若无则采纳任意 assistant 消息，再无则用 delta 拼接。
func parseCozeStream(reader io.Reader) (string, error) {
	scanner := bufio.NewScanner(reader)
	// SSE 单行可能较长，提高缓冲区上限。
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var deltaBuilder strings.Builder
	var answerContent string    // 首选：type=answer 的 completed 消息
	var assistantContent string // 次选：任意 assistant 角色的 completed 消息
	var anyContent string       // 兜底：任意非空 completed 消息
	var eventCounter int

	// 当前事件的 event 名称，由 "event:" 行更新，供后续 "data:" 行使用。
	var currentEvent string

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			// 空行分隔事件，重置当前事件名。
			currentEvent = ""
			continue
		}

		if strings.HasPrefix(line, "event:") {
			currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}

		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		if currentEvent == "" {
			continue
		}
		eventCounter++

		switch currentEvent {
		case "conversation.message.delta":
			var data cozeMessageData
			if err := json.Unmarshal([]byte(payload), &data); err == nil {
				if data.Role == "assistant" && (data.Type == "answer" || data.Type == "") {
					deltaBuilder.WriteString(data.Content)
				}
			}
		case "conversation.message.completed":
			var data cozeMessageData
			if err := json.Unmarshal([]byte(payload), &data); err == nil {
				if data.Role == "assistant" && data.Type == "answer" && data.Content != "" {
					if answerContent == "" {
						answerContent = data.Content
					}
				} else if data.Role == "assistant" && data.Content != "" {
					if assistantContent == "" {
						assistantContent = data.Content
					}
				} else if data.Content != "" {
					if anyContent == "" {
						anyContent = data.Content
					}
				}
			}
		case "conversation.chat.failed":
			var data cozeChatData
			if err := json.Unmarshal([]byte(payload), &data); err == nil && data.LastError != nil {
				return "", fmt.Errorf("Coze 对话失败: code=%d msg=%s", data.LastError.Code, data.LastError.Msg)
			}
			var legacy cozeLegacyError
			if err := json.Unmarshal([]byte(payload), &legacy); err == nil && legacy.Code != 0 {
				return "", fmt.Errorf("Coze 对话失败: code=%d msg=%s", legacy.Code, legacy.Msg)
			}
			return "", errors.New("Coze 对话失败")
		case "conversation.chat.completed":
			return pickBestReply(answerContent, assistantContent, anyContent, deltaBuilder.String(), eventCounter, "Coze")
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("读取 Coze 流失败: %w", err)
	}

	return pickBestReply(answerContent, assistantContent, anyContent, deltaBuilder.String(), eventCounter, "Coze")
}

// pickBestReply 从多个候选回复中择优返回。
// 优先级：answer > assistant > delta > any > 空。
// 全部为空时返回详细错误，便于排查。
func pickBestReply(answer, assistant, any, delta string, eventCount int, providerName string) (string, error) {
	if answer != "" {
		return answer, nil
	}
	if assistant != "" {
		return assistant, nil
	}
	if delta != "" {
		return delta, nil
	}
	if any != "" {
		return any, nil
	}
	return "", fmt.Errorf("%s 未返回有效回复（共收到 %d 个事件，可能原因：Bot 未发布到 API 渠道、bot_id 错误或令牌缺少 chat 权限）", providerName, eventCount)
}
