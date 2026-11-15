package core

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type detectTransport func(*http.Request) (*http.Response, error)

func (f detectTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestOnlineLanguageDetectionAndFallback(t *testing.T) {
	for _, tc := range []struct {
		engine, response string
	}{
		{"百度", `{"lan":"jp"}`},
		{"谷歌", `[null,null,"ja"]`},
		{"必应", `[{"language":"ja"}]`},
	} {
		t.Run(tc.engine, func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: detectTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				response := tc.response
				if r.URL.Host == "edge.microsoft.com" {
					response = "token"
				} else if tc.engine == "必应" && r.Header.Get("Authorization") != "Bearer token" {
					t.Error("Bing token missing from detection request")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(response)), Header: http.Header{}}, nil
			})}
			if got := detectLanguage(context.Background(), client, tc.engine, "hello"); got != "ja" {
				t.Fatalf("detected %q, want ja", got)
			}
			wantCalls := 1
			if tc.engine == "必应" {
				wantCalls = 2
			}
			if calls != wantCalls {
				t.Fatalf("requests = %d, want %d", calls, wantCalls)
			}
		})
	}
	client := &http.Client{Transport: detectTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
	})}
	if got := detectLanguage(context.Background(), client, "百度", "你好"); got != "zh-Hans" {
		t.Fatalf("fallback detected %q, want zh-Hans", got)
	}
}

func TestTranslationUsesSelectedLanguageDetector(t *testing.T) {
	client := &http.Client{Transport: detectTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "fanyi.baidu.com" {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"lan":"jp"}`)), Header: http.Header{}}, nil
		}
		body, err := io.ReadAll(r.Body)
		if err != nil || !strings.Contains(string(body), "Japanese") {
			t.Errorf("detected language absent from prompt: %s %v", body, err)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n")), Header: http.Header{}}, nil
	})}
	q := Query{Profile: Profile{Provider: "OpenAI", Endpoint: "https://model.example/v1/chat/completions", Model: "model"}, Key: "key", Text: "hello", Source: "自动检测", Target: "简体中文", LanguageDetection: "百度", Action: Action{Prompt: "${sourceLang}"}}
	if err := (Engine{Client: client}).Stream(context.Background(), q, func(string) {}); err != nil {
		t.Fatal(err)
	}
}
