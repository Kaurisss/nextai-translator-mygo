package core

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Query struct {
	Profile                     Profile
	Key, Text, Source, Target   string
	Action                      Action
	Writing                     bool
	SelectedWord, ArticlePrompt string
}
type Engine struct{ Client *http.Client }

func (e Engine) Stream(ctx context.Context, q Query, emit func(string)) error {
	p, ok := FindProvider(q.Profile.Provider)
	if !ok {
		return fmt.Errorf("未知模型服务: %s", q.Profile.Provider)
	}
	if strings.TrimSpace(q.Text) == "" {
		return errors.New("请输入要处理的文本")
	}
	if q.Profile.Model == "" || q.Profile.Endpoint == "" {
		return errors.New("请在设置中填写模型和完整 API 地址")
	}
	u, err := url.Parse(q.Profile.Endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("API 地址必须是完整的 HTTP 或 HTTPS URL")
	}
	if q.Key == "" && p.Name != "Ollama" && p.Name != "LiteLLM" {
		return errors.New("请在设置中填写 API Key")
	}
	prompt, err := BuildPrompt(q)
	if err != nil {
		return err
	}
	endpoint := q.Profile.Endpoint
	if p.Protocol == "gemini" {
		endpoint = strings.TrimRight(endpoint, "/") + "/models/" + url.PathEscape(q.Profile.Model) + ":streamGenerateContent?alt=sse"
	}
	body := requestBody(q, p.Protocol, prompt)
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	setAuth(req, p.Protocol, APIKey(q.Key))
	resp, err := e.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return fmt.Errorf("%s: HTTP %d: %s", p.Name, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	content, completed := false, false
	err = readSSE(resp.Body, func(data []byte) (bool, error) {
		if string(data) == "[DONE]" {
			completed = true
			return true, nil
		}
		text, done, err := decodeDelta(p.Protocol, data)
		if text != "" {
			content = true
			emit(text)
		}
		if err != nil {
			return false, err
		}
		completed = done
		return done, nil
	})
	if err != nil {
		return err
	}
	if !content {
		return errors.New("模型未返回文本，请检查模型名称与 API 协议")
	}
	if !completed {
		return errors.New("响应流提前结束，已保留收到的文本，请重试")
	}
	return nil
}

// readSSE joins data lines until a blank line, including a final event without a newline.
func readSSE(r io.Reader, event func([]byte) (bool, error)) error {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 4096), 4*1024*1024)
	var data []string
	dispatch := func() (bool, error) {
		if len(data) == 0 {
			return false, nil
		}
		b := []byte(strings.Join(data, "\n"))
		data = nil
		return event(b)
	}
	for s.Scan() {
		line := s.Text()
		if line == "" {
			done, err := dispatch()
			if err != nil || done {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	if err := s.Err(); err != nil {
		return err
	}
	_, err := dispatch()
	return err
}

func decodeDelta(protocol string, b []byte) (string, bool, error) {
	var v struct {
		Type    string          `json:"type"`
		Error   json.RawMessage `json:"error"`
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Delta        struct {
				Content string `json:"content"`
			} `json:"delta"`
		} `json:"choices"`
		Delta struct {
			Type       string `json:"type"`
			Text       string `json:"text"`
			StopReason string `json:"stop_reason"`
			Message    struct {
				Content struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"message"`
		} `json:"delta"`
		Candidates []struct {
			FinishReason string `json:"finishReason"`
			Content      struct {
				Parts []struct {
					Text    string `json:"text"`
					Thought bool   `json:"thought"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		PromptFeedback struct {
			BlockReason string `json:"blockReason"`
		} `json:"promptFeedback"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return "", false, fmt.Errorf("解析模型响应: %w", err)
	}
	if len(v.Error) > 0 && string(v.Error) != "null" {
		return "", false, fmt.Errorf("模型服务错误: %s", v.Error)
	}
	switch protocol {
	case "claude":
		if v.Delta.StopReason == "max_tokens" {
			return v.Delta.Text, false, errors.New("模型输出达到 token 上限，已保留收到的文本")
		}
		return v.Delta.Text, v.Type == "message_stop", nil
	case "cohere":
		return v.Delta.Message.Content.Text, v.Type == "message-end", nil
	case "gemini":
		if v.PromptFeedback.BlockReason != "" {
			return "", false, fmt.Errorf("请求被模型拦截: %s", v.PromptFeedback.BlockReason)
		}
		var text strings.Builder
		if len(v.Candidates) > 0 {
			for _, part := range v.Candidates[0].Content.Parts {
				if !part.Thought {
					text.WriteString(part.Text)
				}
			}
		}
		if len(v.Candidates) > 0 && v.Candidates[0].FinishReason != "" {
			if v.Candidates[0].FinishReason != "STOP" {
				return text.String(), false, fmt.Errorf("模型停止生成: %s", v.Candidates[0].FinishReason)
			}
			return text.String(), true, nil
		}
		return text.String(), false, nil
	default:
		if len(v.Choices) > 0 {
			reason := v.Choices[0].FinishReason
			if reason != "" && reason != "stop" {
				return "", false, fmt.Errorf("模型停止生成: %s", reason)
			}
			return v.Choices[0].Delta.Content, reason == "stop", nil
		}
		return "", false, nil
	}
}
