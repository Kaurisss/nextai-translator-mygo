package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestMigratedDictionaryRequestOnWire(t *testing.T) {
	fixture := promptFixtures(t)[0]
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []map[string]string `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if !reflect.DeepEqual(body.Messages, fixture.Messages) {
			t.Errorf("dictionary wire request differs from source: %v", body.Messages)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Examples:\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	q := fixture.Query
	q.Profile = Profile{Provider: "Ollama", Endpoint: server.URL, Model: "antigravity/gemini-3.5-flash-lite"}
	var result strings.Builder
	if err := (Engine{Client: server.Client()}).Stream(context.Background(), q, func(s string) { result.WriteString(s) }); err != nil {
		t.Fatal(err)
	}
	if result.String() != "Examples:" {
		t.Fatal(result.String())
	}
}

func TestProviderStreaming(t *testing.T) {
	for _, tc := range []struct{ name, header, value, stream string }{
		{"OpenAI", "Authorization", "Bearer secret", "data: {\"choices\":[{\"delta\":{\"content\":\"你好\"}}]}\n\ndata: [DONE]\n\n"},
		{"Azure", "api-key", "secret", "data: {\"choices\":[{\"delta\":{\"content\":\"你好\"}}]}\n\ndata: [DONE]\n\n"},
		{"Claude", "x-api-key", "secret", "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"你好\"}}\n\ndata: {\"type\":\"message_stop\"}\n\n"},
		{"Gemini", "x-goog-api-key", "secret", "data: {\"candidates\":[{\"finishReason\":\"STOP\",\"content\":{\"parts\":[{\"text\":\"thinking\",\"thought\":true},{\"text\":\"你好\"}]}}]}\n\n"},
		{"Cohere", "Authorization", "Bearer secret", "data: {\"type\":\"content-delta\",\"delta\":{\"message\":{\"content\":{\"text\":\"你好\"}}}}\n\ndata: {\"type\":\"message-end\"}\n\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get(tc.header); got != tc.value {
					t.Errorf("header %s = %q", tc.header, got)
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if tc.name == "Gemini" {
					if r.URL.Path != "/models/model:streamGenerateContent" || r.URL.Query().Get("alt") != "sse" {
						t.Errorf("URL %s", r.URL)
					}
					if body["systemInstruction"] != nil {
						t.Error("original Gemini prompt must remain in the user message")
					}
					contents := body["contents"].([]any)
					message := contents[0].(map[string]any)
					text := message["parts"].([]any)[0].(map[string]any)["text"]
					if message["role"] != "user" || text != "Translate to 简体中文\n\nOnly reply the result and nothing else. :\n\nHello" {
						t.Errorf("Gemini message %v", message)
					}
				} else if body["model"] != "model" || body["stream"] != true {
					t.Errorf("body %v", body)
				}
				if tc.name != "Gemini" {
					messages := body["messages"].([]any)
					if len(messages) != 1 || messages[0].(map[string]any)["role"] != "user" || messages[0].(map[string]any)["content"] != "Translate to 简体中文\n\nOnly reply the result and nothing else. :\n\nHello" {
						t.Errorf("wire messages %v", messages)
					}
				}
				if tc.name == "Claude" && (body["system"] != nil || r.Header.Get("anthropic-version") == "") {
					t.Errorf("Claude body %v", body)
				}
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, tc.stream)
			}))
			defer server.Close()
			var got strings.Builder
			err := (Engine{Client: server.Client()}).Stream(context.Background(), Query{Profile: Profile{Provider: tc.name, Endpoint: server.URL, Model: "model"}, Key: "secret", Text: "Hello", Target: "简体中文", Action: Action{Prompt: "Translate to {{target}}"}}, func(s string) { got.WriteString(s) })
			if err != nil || got.String() != "你好" {
				t.Fatalf("result %q, error %v", got.String(), err)
			}
		})
	}
}

func TestStreamErrors(t *testing.T) {
	for _, tc := range []struct {
		name               string
		status             int
		response, contains string
	}{
		{"http", 401, "invalid api key", "HTTP 401"},
		{"malformed", 200, "data: broken\n\n", "解析模型响应"},
		{"provider", 200, "data: {\"error\":{\"message\":\"quota\"}}\n\n", "quota"},
		{"empty", 200, "data: [DONE]\n\n", "未返回文本"},
		{"truncated", 200, "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n", "响应流提前结束"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); fmt.Fprint(w, tc.response) }))
			defer s.Close()
			err := (Engine{Client: s.Client()}).Stream(context.Background(), Query{Profile: Profile{Provider: "OpenAI", Endpoint: s.URL, Model: "model"}, Key: "key", Text: "test"}, func(string) {})
			if err == nil || !strings.Contains(err.Error(), tc.contains) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestSSEMultilineAndFinalEvent(t *testing.T) {
	var events []string
	err := readSSE(strings.NewReader(": keepalive\r\ndata: first\r\ndata: second\r\n\r\ndata: last"), func(b []byte) (bool, error) { events = append(events, string(b)); return false, nil })
	if err != nil || len(events) != 2 || events[0] != "first\nsecond" || events[1] != "last" {
		t.Fatalf("events %q: %v", events, err)
	}
}

func TestCancellation(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer s.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := (Engine{Client: s.Client()}).Stream(ctx, Query{Profile: Profile{Provider: "OpenAI", Endpoint: s.URL, Model: "model"}, Key: "key", Text: "test"}, func(string) {})
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("error = %v", err)
	}
}

func TestThinkingRequests(t *testing.T) {
	for _, tc := range []struct{ provider, model, stream string }{
		{"Claude", "claude-sonnet-4-5", "data: {\"delta\":{\"text\":\"ok\"}}\n\ndata: {\"type\":\"message_stop\"}\n\n"},
		{"Gemini", "gemini-2.5-flash", "data: {\"candidates\":[{\"finishReason\":\"STOP\",\"content\":{\"parts\":[{\"text\":\"ok\"}]}}]}\n\n"},
		{"Gemini", "gemini-3-flash", "data: {\"candidates\":[{\"finishReason\":\"STOP\",\"content\":{\"parts\":[{\"text\":\"ok\"}]}}]}\n\n"},
		{"OpenAI", "gpt-5", "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n"},
	} {
		t.Run(tc.provider+tc.model, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				switch tc.provider {
				case "Claude":
					thinking := body["thinking"].(map[string]any)
					if thinking["budget_tokens"] != float64(20000) || body["max_tokens"].(float64) <= 20000 {
						t.Errorf("thinking %v", body)
					}
				case "Gemini":
					thinking := body["generationConfig"].(map[string]any)["thinkingConfig"].(map[string]any)
					if strings.Contains(tc.model, "2.5") {
						if thinking["thinkingBudget"] != float64(20000) {
							t.Error(thinking)
						}
					} else if thinking["thinkingLevel"] != "high" {
						t.Error(thinking)
					}
				case "OpenAI":
					if body["reasoning_effort"] != "high" {
						t.Error(body)
					}
				}
				fmt.Fprint(w, tc.stream)
			}))
			defer s.Close()
			err := (Engine{Client: s.Client()}).Stream(context.Background(), Query{Profile: Profile{Provider: tc.provider, Endpoint: s.URL, Model: tc.model}, Key: "key", Text: "test", Action: Action{Thinking: true, ThinkingLevel: "high"}}, func(string) {})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
