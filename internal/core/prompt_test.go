package core

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type promptFixture struct {
	Name     string              `json:"name"`
	Query    Query               `json:"query"`
	Expected Prompt              `json:"expected"`
	Messages []map[string]string `json:"messages"`
}

func promptFixtures(t *testing.T) []promptFixture {
	t.Helper()
	b, err := os.ReadFile("testdata/prompts.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []promptFixture
	if err := json.Unmarshal(b, &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures
}

func TestOriginalPromptParity(t *testing.T) {
	for _, fixture := range promptFixtures(t) {
		t.Run(fixture.Name, func(t *testing.T) {
			actual, err := BuildPrompt(fixture.Query)
			if err != nil {
				t.Fatal(err)
			}
			if actual != fixture.Expected {
				t.Errorf("prompt differs from original TypeScript\nrole want: %q\nrole got:  %q\ncommand want: %q\ncommand got:  %q\nword mode want %v, got %v", fixture.Expected.RolePrompt, actual.RolePrompt, fixture.Expected.CommandPrompt, actual.CommandPrompt, fixture.Expected.WordMode, actual.WordMode)
			}
		})
	}
}

func TestOriginalWordSegmentationParity(t *testing.T) {
	b, err := os.ReadFile("testdata/word-segments.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Locale, Text string
		Expected     bool
	}
	if err := json.Unmarshal(b, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Locale+"/"+fixture.Text, func(t *testing.T) {
			got, err := isWord(fixture.Locale, fixture.Text)
			if err != nil {
				t.Fatal(err)
			}
			if got != fixture.Expected {
				t.Errorf("Intl.Segmenter = %v; native ICU = %v", fixture.Expected, got)
			}
		})
	}
}

func TestNativeLanguagesHavePromptMetadata(t *testing.T) {
	for _, name := range Languages {
		if _, err := promptLanguage(name); err != nil {
			t.Error(err)
		}
	}
}

func TestOriginalMessagesReachEveryProvider(t *testing.T) {
	for _, fixture := range promptFixtures(t) {
		t.Run(fixture.Name, func(t *testing.T) {
			prompt, err := BuildPrompt(fixture.Query)
			if err != nil {
				t.Fatal(err)
			}
			for _, protocol := range []string{"openai", "azure", "claude", "gemini", "cohere"} {
				q := fixture.Query
				q.Profile.Model = "gateway-model"
				body := requestBody(q, protocol, prompt)
				if protocol == "gemini" {
					contents := body["contents"].([]map[string]any)
					text := contents[0]["parts"].([]map[string]string)[0]["text"]
					if len(contents) != 1 || contents[0]["role"] != "user" || text != fixture.Messages[0]["content"] {
						t.Errorf("%s changed the original prompt: %v", protocol, body)
					}
				} else if !reflect.DeepEqual(body["messages"], fixture.Messages) {
					t.Errorf("%s changed original messages: %v", protocol, body["messages"])
				}
				if body["system"] != nil || body["systemInstruction"] != nil {
					t.Errorf("%s introduced an extra system prompt", protocol)
				}
			}
		})
	}
}
