package core

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSplitEnglishWords(t *testing.T) {
	input := "中文 hello, don’t well-known 123 world!"
	pieces := SplitEnglishWords(input)
	var rebuilt strings.Builder
	var words []string
	for _, p := range pieces {
		rebuilt.WriteString(p.Text)
		if p.IsWord {
			words = append(words, p.Text)
		}
	}
	if rebuilt.String() != input {
		t.Fatalf("rebuilt %q != %q", rebuilt.String(), input)
	}
	want := []string{"hello", "don’t", "well-known", "world"}
	if strings.Join(words, "|") != strings.Join(want, "|") {
		t.Fatalf("words = %v, want %v", words, want)
	}
	if HasEnglishWord("纯中文句子，没有英文。") {
		t.Fatal("Chinese text should not match HasEnglishWord")
	}
	if !HasEnglishWord("包含 hello 单词") {
		t.Fatal("mixed text should match HasEnglishWord")
	}
}

func TestParseDictionaryResponse(t *testing.T) {
	raw := `[{
		"word": "ephemeral",
		"phonetics": [{"text": ""}, {"text": "/ɪˈfɛm.ə.ɹəl/"}],
		"meanings": [
			{
				"partOfSpeech": "adjective",
				"definitions": [
					{"definition": "Lasting for a short period of time."},
					{"definition": "Secondary sense.", "example": "Fame in the internet age is ephemeral."}
				]
			},
			{
				"partOfSpeech": "noun",
				"definitions": [
					{"definition": "Something which lasts for a short period of time."}
				]
			},
			{
				"partOfSpeech": "verb",
				"definitions": [
					{"definition": "Should be ignored beyond two meanings."}
				]
			}
		],
		"sourceUrls": ["https://en.wiktionary.org/wiki/ephemeral"]
	}]`
	preview, err := ParseDictionaryResponse([]byte(raw), "Ephemeral")
	if err != nil {
		t.Fatal(err)
	}
	if preview == nil {
		t.Fatal("expected preview")
	}
	if preview.Word != "ephemeral" || preview.Phonetic != "/ɪˈfɛm.ə.ɹəl/" || preview.SourceURL != "https://en.wiktionary.org/wiki/ephemeral" {
		t.Fatalf("unexpected preview header: %+v", preview)
	}
	if len(preview.Meanings) != 2 {
		t.Fatalf("expected 2 meanings, got %d", len(preview.Meanings))
	}
	if preview.Meanings[0].PartOfSpeech != "adjective" || preview.Meanings[0].Example != "Fame in the internet age is ephemeral." {
		t.Fatalf("unexpected first meaning: %+v", preview.Meanings[0])
	}
	empty, err := ParseDictionaryResponse([]byte(`[]`), "none")
	if err != nil || empty != nil {
		t.Fatalf("empty array: preview=%+v err=%v", empty, err)
	}
}

func TestLookupEnglishWordAt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/en/hello":
			fmt.Fprint(w, `[{"word":"hello","phonetic":"/həˈləʊ/","meanings":[{"partOfSpeech":"interjection","definitions":[{"definition":"A greeting."}]}]}]`)
		case "/en/unknown":
			http.NotFound(w, r)
		default:
			http.Error(w, "boom", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	base := server.URL + "/en/"
	preview, err := LookupEnglishWordAt(context.Background(), server.Client(), base, "Hello")
	if err != nil || preview == nil || preview.Word != "hello" {
		t.Fatalf("hello lookup: preview=%+v err=%v", preview, err)
	}
	missing, err := LookupEnglishWordAt(context.Background(), server.Client(), base, "unknown")
	if err != nil || missing != nil {
		t.Fatalf("404 lookup: preview=%+v err=%v", missing, err)
	}
	if _, err := LookupEnglishWordAt(context.Background(), server.Client(), base, "fail"); err == nil {
		t.Fatal("expected 500 error")
	}
}
