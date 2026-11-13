package core

import (
	"regexp"
	"strconv"
	"strings"
)

var gpt5Pro = regexp.MustCompile(`^gpt-5(\.0)?(-mini|-nano)?-pro\b`)
var gpt5Later = regexp.MustCompile(`^gpt-5\.[1-9]`)
var gpt5Initial = regexp.MustCompile(`^gpt-5(-mini|-nano)?(\b|-)`)
var claudeVersion = regexp.MustCompile(`(?i)claude-(?:opus|sonnet|haiku|fable|mythos)-([0-9]+)(?:[-.]([0-9]+))?`)
var geminiVersion = regexp.MustCompile(`(?i)^gemini-([0-9]+)`)

// chatParameters follows AbstractOpenAI.getBaseRequestBody. Gateway model
// aliases receive only the minimal parameters, as in the original engine.
func chatParameters(model string) map[string]any {
	b := map[string]any{"model": model, "stream": true}
	m := strings.ToLower(model)
	switch {
	case strings.HasPrefix(m, "gpt-3") || strings.HasPrefix(m, "gpt-4"):
		b["temperature"] = 0
		b["top_p"] = 1
		b["frequency_penalty"] = 1
		b["presence_penalty"] = 1
	case strings.HasPrefix(m, "o1") || strings.HasPrefix(m, "o3") || strings.HasPrefix(m, "o4") || gpt5Pro.MatchString(m):
		b["reasoning_effort"] = "low"
	case strings.HasPrefix(m, "gpt-5") && (strings.Contains(m, "-chat") || strings.Contains(m, "-code") || strings.Contains(m, "instant")):
	case gpt5Later.MatchString(m):
	case gpt5Initial.MatchString(m) && !strings.HasPrefix(m, "gpt-5."):
		b["reasoning_effort"] = "minimal"
	}
	return b
}

func requestBody(q Query, protocol string, p Prompt) map[string]any {
	if q.Profile.Thinking {
		q.Action.Thinking = true
		if q.Action.ThinkingLevel == "" {
			q.Action.ThinkingLevel = q.Profile.ThinkingLevel
		}
	}
	messages := []map[string]string{{"role": "user", "content": p.Combined()}}
	body := map[string]any{"model": q.Profile.Model, "stream": true, "messages": messages}
	level := q.Action.ThinkingLevel
	if level == "" {
		level = "medium"
	}
	budget := 10000
	if level == "low" {
		budget = 5000
	}
	if level == "high" {
		budget = 20000
	}
	switch protocol {
	case "claude":
		if adaptiveClaude(q.Profile.Model) {
			if q.Action.Thinking {
				body["thinking"] = map[string]string{"type": "adaptive"}
				body["output_config"] = map[string]string{"effort": level}
				body["max_tokens"] = 32000
			} else {
				body["output_config"] = map[string]string{"effort": "low"}
				body["max_tokens"] = 16000
			}
		} else if q.Action.Thinking {
			body["thinking"] = map[string]any{"type": "enabled", "budget_tokens": budget}
			body["max_tokens"] = max(16384, budget+4096)
		} else {
			body["temperature"] = 0
			body["max_tokens"] = 4096
		}
	case "gemini":
		generation := map[string]any{}
		if supported, known := q.Profile.ModelThinking[q.Profile.Model]; !known || supported {
			if thinking := geminiThinking(q.Profile.Model, q.Action.Thinking, q.Action.ThinkingLevel, budget); thinking != nil {
				generation["thinkingConfig"] = thinking
			}
		}
		safety := []map[string]string{}
		for _, category := range []string{"HARM_CATEGORY_HARASSMENT", "HARM_CATEGORY_HATE_SPEECH", "HARM_CATEGORY_SEXUALLY_EXPLICIT", "HARM_CATEGORY_DANGEROUS_CONTENT"} {
			safety = append(safety, map[string]string{"category": category, "threshold": "OFF"})
		}
		body = map[string]any{"contents": []map[string]any{{"role": "user", "parts": []map[string]string{{"text": p.Combined()}}}}, "generationConfig": generation, "safetySettings": safety}
	case "openai", "azure":
		body = chatParameters(q.Profile.Model)
		body["messages"] = messages
		if q.Action.Thinking {
			body["reasoning_effort"] = level
		} else if q.Profile.Provider == "Ollama" || gpt5Later.MatchString(strings.ToLower(q.Profile.Model)) {
			body["reasoning_effort"] = "none"
		}
		if q.Profile.Provider == "Ollama" {
			body["keep_alive"] = "5m"
			if q.Profile.KeepAlive != "" {
				body["keep_alive"] = q.Profile.KeepAlive
			}
		}
		if protocol == "azure" {
			body["max_tokens"] = 1024
			if q.Profile.MaxTokens > 0 {
				body["max_tokens"] = q.Profile.MaxTokens
			}
		}
	}
	return body
}

func adaptiveClaude(model string) bool {
	v := claudeVersion.FindStringSubmatch(model)
	if v == nil || len(v[1]) != 1 {
		return false
	}
	major, _ := strconv.Atoi(v[1])
	minor := 0
	if len(v[2]) == 1 {
		minor, _ = strconv.Atoi(v[2])
	}
	return major > 4 || major == 4 && minor >= 7
}

func geminiThinking(model string, enabled bool, explicitLevel string, budget int) map[string]any {
	m := strings.ToLower(model)
	version := geminiVersion.FindStringSubmatch(m)
	major := 0
	if version != nil {
		major, _ = strconv.Atoi(version[1])
	}
	gemma := strings.HasPrefix(m, "gemma-")
	usesLevel := gemma || strings.Contains(m, "robotics-er-2") || strings.HasSuffix(m, "-latest") || major >= 3
	if enabled {
		if gemma {
			return map[string]any{"thinkingLevel": "high"}
		}
		if explicitLevel != "" {
			if usesLevel {
				return map[string]any{"thinkingLevel": explicitLevel}
			}
			if major > 0 {
				return map[string]any{"thinkingBudget": budget}
			}
		}
		if usesLevel {
			return map[string]any{}
		}
		if major > 0 {
			return map[string]any{"thinkingBudget": -1}
		}
		return nil
	}
	if usesLevel {
		minimal := gemma || strings.Contains(m, "flash-lite") || strings.HasPrefix(m, "gemini-3-flash") || strings.HasPrefix(m, "gemini-3.6-flash")
		level := "low"
		if minimal {
			level = "minimal"
		}
		return map[string]any{"thinkingLevel": level}
	}
	if major > 0 {
		budget := 0
		if strings.HasSuffix(m, "-pro") || strings.Contains(m, "-pro-") || strings.Contains(m, "-pro:") {
			budget = 128
		}
		return map[string]any{"thinkingBudget": budget}
	}
	return nil
}
