package core

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
)

func TestExplicitProxyAndBypass(t *testing.T) {
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Host != "model.example" {
			t.Errorf("proxy target %s", r.URL)
		}
		if r.Header.Get("Proxy-Authorization") != "Basic "+base64.StdEncoding.EncodeToString([]byte("user:p@ss")) {
			t.Error("missing proxy credentials")
		}
		fmt.Fprint(w, "proxy")
	}))
	defer proxy.Close()
	u, _ := url.Parse(proxy.URL)
	host, port, _ := net.SplitHostPort(u.Host)
	client, err := HTTPClient(ProxySettings{Enabled: true, Protocol: "HTTP", Server: host, Port: port, Username: "user", NoProxy: "localhost,127.0.0.1"}, "p@ss")
	if err != nil {
		t.Fatal(err)
	}
	defer client.CloseIdleConnections()
	response, err := client.Get("http://model.example/v1/models")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if string(b) != "proxy" {
		t.Fatal(string(b))
	}
	direct := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Proxy-Authorization") != "" {
			t.Error("proxy credentials leaked to origin")
		}
		fmt.Fprint(w, "direct")
	}))
	defer direct.Close()
	response, err = client.Get(direct.URL)
	if err != nil {
		t.Fatal(err)
	}
	b, _ = io.ReadAll(response.Body)
	response.Body.Close()
	if string(b) != "direct" {
		t.Fatal(string(b))
	}
}

func TestProviderModelLists(t *testing.T) {
	for _, tc := range []struct{ provider, path, body, header string }{
		{"OpenAI", "/v1/models", `{"data":[{"id":"z-model"},{"id":"a-model"},{"id":"z-model"}]}`, "Authorization"},
		{"Claude", "/v1/models", `{"data":[{"id":"z-model"},{"id":"a-model"}]}`, "x-api-key"},
		{"Gemini", "/v1beta/models", `{"models":[{"name":"models/z-model"},{"name":"models/a-model"}]}`, "x-goog-api-key"},
		{"Cohere", "/v1/models", `{"models":[{"name":"z-model"},{"name":"a-model"}]}`, "Authorization"},
	} {
		t.Run(tc.provider, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.path || r.Header.Get(tc.header) == "" {
					t.Errorf("model request %s headers %v", r.URL, r.Header)
				}
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			_, path := SplitEndpoint(func() string { p, _ := FindProvider(tc.provider); return p.Endpoint }())
			models, err := ListModels(context.Background(), server.Client(), Profile{Provider: tc.provider, Endpoint: server.URL + path}, "key")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(models.Names, []string{"a-model", "z-model"}) {
				t.Fatal(models)
			}
		})
	}
}

func TestEndpointRoundTripAndKeys(t *testing.T) {
	for _, endpoint := range []string{"https://example.com/v1/chat/completions", "http://127.0.0.1:3425/", "https://example.com/openai/deployments/model/chat/completions?api-version=2025-01-01", "https://example.com/v1beta"} {
		base, path := SplitEndpoint(endpoint)
		if got := JoinEndpoint(base, path); got != endpoint {
			t.Errorf("%s became %s", endpoint, got)
		}
	}
	for range 20 {
		key := APIKey(" , key-a, key-b ,")
		if key != "key-a" && key != "key-b" {
			t.Fatal(key)
		}
	}
	if APIKey(" , ") != "" {
		t.Fatal("empty API key list")
	}
}

func TestSettingsValidation(t *testing.T) {
	s := DefaultData().Settings
	if err := ValidateSettings(s); err != nil {
		t.Fatal(err)
	}
	s.DisplayShortcut = "Shift+Control+T"
	if err := ValidateSettings(s); err == nil {
		t.Fatal("conflicting shortcuts accepted")
	}
	s = DefaultData().Settings
	s.Speech.Volume = -1
	if err := ValidateSettings(s); err == nil {
		t.Fatal("invalid volume accepted")
	}
	s = DefaultData().Settings
	s.Writing.Target = "unknown"
	if err := ValidateSettings(s); err == nil {
		t.Fatal("invalid writing language accepted")
	}
	s = DefaultData().Settings
	s.FontSize = 11
	if err := ValidateSettings(s); err == nil {
		t.Fatal("invalid font size accepted")
	}
	s = DefaultData().Settings
	s.LanguageDetection = "unknown"
	if err := ValidateSettings(s); err == nil {
		t.Fatal("invalid language detector accepted")
	}
}

func TestGeminiModelCapabilitiesAndGenerationFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("pageSize") != "1000" {
			t.Error("Gemini model list size not requested")
		}
		fmt.Fprint(w, `{"models":[{"name":"models/gemma-3","thinking":false,"supportedGenerationMethods":["generateContent"]},{"name":"models/gemini-3-pro","thinking":true,"supportedGenerationMethods":["generateContent"]},{"name":"models/embedding","supportedGenerationMethods":["embedContent"]}]}`)
	}))
	defer server.Close()
	p := Profile{Provider: "Gemini", Endpoint: server.URL, Model: "gemma-3"}
	models, err := ListModels(context.Background(), server.Client(), p, "key")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(models.Names, []string{"gemini-3-pro", "gemma-3"}) || models.Thinking["gemma-3"] || !models.Thinking["gemini-3-pro"] {
		t.Fatalf("models %+v", models)
	}
	p.ModelThinking = models.Thinking
	body := requestBody(Query{Profile: p}, "gemini", Prompt{})
	if body["generationConfig"].(map[string]any)["thinkingConfig"] != nil {
		t.Fatal("unsupported thinking parameters sent")
	}
}
