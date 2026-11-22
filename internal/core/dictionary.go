package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const (
	// DictionaryBaseURL is the free English dictionary endpoint used by the
	// hover word card, matching the original NextAI Translator implementation.
	DictionaryBaseURL = "https://api.dictionaryapi.dev/api/v2/entries/en/"

	maxDictionaryResponseSize = 1 << 20
	maxDictionaryMeanings     = 2
)

// WordMeaning is a single concise part-of-speech definition for a hovered word.
type WordMeaning struct {
	PartOfSpeech string `json:"partOfSpeech"`
	Definition   string `json:"definition"`
	Example      string `json:"example,omitempty"`
}

// WordPreview is the concise dictionary preview shown in the hover card.
type WordPreview struct {
	Word      string        `json:"word"`
	Phonetic  string        `json:"phonetic,omitempty"`
	Meanings  []WordMeaning `json:"meanings"`
	SourceURL string        `json:"sourceUrl,omitempty"`
}

// TextPiece is a contiguous slice of text that is either an English word or
// non-word text between words.
type TextPiece struct {
	Text   string
	IsWord bool
}

var englishWordPattern = regexp.MustCompile(`[A-Za-z]+(?:[’'][A-Za-z]+)*(?:-[A-Za-z]+)*`)

// HasEnglishWord reports whether text contains at least one English word token.
func HasEnglishWord(text string) bool {
	return englishWordPattern.FindStringIndex(text) != nil
}

// SplitEnglishWords splits text into alternating non-word and English word
// pieces while preserving the exact original string when concatenated.
func SplitEnglishWords(text string) []TextPiece {
	matches := englishWordPattern.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		if text == "" {
			return nil
		}
		return []TextPiece{{Text: text}}
	}
	pieces := make([]TextPiece, 0, len(matches)*2+1)
	last := 0
	for _, m := range matches {
		if m[0] > last {
			pieces = append(pieces, TextPiece{Text: text[last:m[0]]})
		}
		pieces = append(pieces, TextPiece{Text: text[m[0]:m[1]], IsWord: true})
		last = m[1]
	}
	if last < len(text) {
		pieces = append(pieces, TextPiece{Text: text[last:]})
	}
	return pieces
}

// NormalizeLookupWord normalizes an English word for caching and dictionary lookup.
func NormalizeLookupWord(word string) string {
	word = strings.TrimSpace(strings.ReplaceAll(word, "’", "'"))
	return strings.ToLower(word)
}

// LookupEnglishWord fetches a concise dictionary preview for word from
// DictionaryBaseURL using client. HTTP 404 returns (nil, nil).
func LookupEnglishWord(ctx context.Context, client *http.Client, word string) (*WordPreview, error) {
	return LookupEnglishWordAt(ctx, client, DictionaryBaseURL, word)
}

// LookupEnglishWordAt fetches a concise dictionary preview for word from
// baseURL using client. HTTP 404 returns (nil, nil).
func LookupEnglishWordAt(ctx context.Context, client *http.Client, baseURL, word string) (*WordPreview, error) {
	normalized := NormalizeLookupWord(word)
	if normalized == "" {
		return nil, nil
	}
	if client == nil {
		return nil, fmt.Errorf("未配置可用的 HTTP 客户端")
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/" + url.PathEscape(normalized)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "nextai-translator-native")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("词典查询失败: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxDictionaryResponseSize))
	if err != nil {
		return nil, err
	}
	return ParseDictionaryResponse(data, word)
}

// ParseDictionaryResponse decodes a dictionaryapi.dev JSON response into a
// concise WordPreview with up to two meanings.
func ParseDictionaryResponse(data []byte, fallbackWord string) (*WordPreview, error) {
	var entries []struct {
		Word      string `json:"word"`
		Phonetic  string `json:"phonetic"`
		Phonetics []struct {
			Text string `json:"text"`
		} `json:"phonetics"`
		Meanings []struct {
			PartOfSpeech string `json:"partOfSpeech"`
			Definitions  []struct {
				Definition string `json:"definition"`
				Example    string `json:"example"`
			} `json:"definitions"`
		} `json:"meanings"`
		SourceURLs []string `json:"sourceUrls"`
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, nil
	}
	entry := entries[0]
	var meanings []WordMeaning
	for _, m := range entry.Meanings {
		def := ""
		example := ""
		for _, d := range m.Definitions {
			if def == "" && strings.TrimSpace(d.Definition) != "" {
				def = strings.TrimSpace(d.Definition)
			}
			if example == "" && strings.TrimSpace(d.Example) != "" {
				example = strings.TrimSpace(d.Example)
			}
			if def != "" && example != "" {
				break
			}
		}
		if def == "" {
			continue
		}
		meanings = append(meanings, WordMeaning{
			PartOfSpeech: strings.TrimSpace(m.PartOfSpeech),
			Definition:   def,
			Example:      example,
		})
		if len(meanings) >= maxDictionaryMeanings {
			break
		}
	}
	if len(meanings) == 0 {
		return nil, nil
	}
	word := strings.TrimSpace(entry.Word)
	if word == "" {
		word = fallbackWord
	}
	phonetic := strings.TrimSpace(entry.Phonetic)
	if phonetic == "" {
		for _, p := range entry.Phonetics {
			if text := strings.TrimSpace(p.Text); text != "" {
				phonetic = text
				break
			}
		}
	}
	sourceURL := ""
	for _, raw := range entry.SourceURLs {
		if u, err := url.Parse(strings.TrimSpace(raw)); err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" {
			sourceURL = u.String()
			break
		}
	}
	return &WordPreview{
		Word:      word,
		Phonetic:  phonetic,
		Meanings:  meanings,
		SourceURL: sourceURL,
	}, nil
}
