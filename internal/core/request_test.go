package core

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestOriginalModelParameters(t *testing.T) {
	b, err := os.ReadFile("testdata/model-parameters.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Model    string
		Expected map[string]any
	}
	if err := json.Unmarshal(b, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Model, func(t *testing.T) {
			encoded, err := json.Marshal(chatParameters(fixture.Model))
			if err != nil {
				t.Fatal(err)
			}
			var actual map[string]any
			if err := json.Unmarshal(encoded, &actual); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(actual, fixture.Expected) {
				t.Errorf("parameters want %v, got %v", fixture.Expected, actual)
			}
		})
	}
}

func TestOriginalGeminiThinkingDefaults(t *testing.T) {
	b, err := os.ReadFile("testdata/gemini-thinking.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Model    string
		Enabled  bool
		Expected map[string]any
	}
	if err := json.Unmarshal(b, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Model, func(t *testing.T) {
			encoded, err := json.Marshal(geminiThinking(fixture.Model, fixture.Enabled, "", 10000))
			if err != nil {
				t.Fatal(err)
			}
			var actual map[string]any
			if err := json.Unmarshal(encoded, &actual); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(actual, fixture.Expected) {
				t.Errorf("thinking enabled=%v want %v, got %v", fixture.Enabled, fixture.Expected, actual)
			}
		})
	}
}

func TestProviderDefaultParameters(t *testing.T) {
	q := Query{Profile: Profile{Provider: "Ollama", Model: "gateway-alias"}}
	body := requestBody(q, "openai", Prompt{RolePrompt: "role", CommandPrompt: "command"})
	if body["reasoning_effort"] != "none" || body["keep_alive"] != "5m" {
		t.Errorf("Ollama default body %v", body)
	}
	q.Profile = Profile{Provider: "Claude", Model: "claude-sonnet-4-5"}
	body = requestBody(q, "claude", Prompt{})
	if body["temperature"] != 0 || body["max_tokens"] != 4096 {
		t.Errorf("Claude default body %v", body)
	}
	q.Profile.Model = "claude-opus-4-7"
	body = requestBody(q, "claude", Prompt{})
	if body["temperature"] != nil || body["thinking"] != nil || body["max_tokens"] != 16000 {
		t.Errorf("adaptive Claude body %v", body)
	}
	q.Action.Thinking = true
	body = requestBody(q, "claude", Prompt{})
	if body["thinking"].(map[string]string)["type"] != "adaptive" || body["max_tokens"] != 32000 {
		t.Errorf("adaptive thinking body %v", body)
	}
}

func TestSavedProviderOptionsReachRequest(t *testing.T) {
	q := Query{Profile: Profile{Provider: "Azure", Model: "gpt-4o-mini", MaxTokens: 2048, Thinking: true, ThinkingLevel: "high"}}
	body := requestBody(q, "azure", Prompt{})
	if body["max_tokens"] != 2048 || body["reasoning_effort"] != "high" {
		t.Fatal(body)
	}
	q.Profile = Profile{Provider: "Ollama", Model: "qwen3", KeepAlive: "0"}
	if body := requestBody(q, "openai", Prompt{}); body["keep_alive"] != "0" {
		t.Fatal(body)
	}
}
