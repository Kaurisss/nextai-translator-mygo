package app

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Kaurisss/nextai-translator-mygo/internal/core"
	"github.com/egoist/mygo/ui"
)

type stubStreamer struct {
	mu      sync.Mutex
	queries []core.Query
	reply   string
}

func (s *stubStreamer) Stream(_ context.Context, q core.Query, onDelta func(string)) error {
	s.mu.Lock()
	s.queries = append(s.queries, q)
	reply := s.reply
	s.mu.Unlock()
	if reply != "" {
		onDelta(reply)
	}
	return nil
}

func TestWordHoverCardMarkdownAndCache(t *testing.T) {
	origLookup := lookupWordFunc
	t.Cleanup(func() { lookupWordFunc = origLookup })

	var calls atomic.Int32
	lookupWordFunc = func(_ context.Context, _ *http.Client, word string) (*core.WordPreview, error) {
		calls.Add(1)
		switch core.NormalizeLookupWord(word) {
		case "ephemeral":
			return &core.WordPreview{
				Word:     "ephemeral",
				Phonetic: "/ɪˈfɛm.ə.ɹəl/",
				Meanings: []core.WordMeaning{
					{
						PartOfSpeech: "adjective",
						Definition:   "Lasting for a short period of time.",
						Example:      "Fame is ephemeral.",
					},
				},
				SourceURL: "https://en.wiktionary.org/wiki/ephemeral",
			}, nil
		case "unknownword":
			return nil, nil
		default:
			return nil, errors.New("network error")
		}
	}

	a := testApp(t)
	streamer := &stubStreamer{reply: "短暂的"}
	a.Engine = streamer
	a.Data.Settings.Target = "English"
	a.Data.Settings.DefaultTarget = "简体中文"
	a.action = "总结"
	a.text = "测试"
	a.result = "This **ephemeral** moment and `codeword` with unknownword and [docs](https://example.com)."

	var mu sync.Mutex
	updated := make(chan struct{}, 16)
	a.post = func(fn func()) {
		mu.Lock()
		fn()
		mu.Unlock()
		select {
		case updated <- struct{}{}:
		default:
		}
	}

	tt := ui.NewTester(func(c *ui.Context) {
		mu.Lock()
		defer mu.Unlock()
		a.View(c)
	}, 560, 760)

	// Hover on "ephemeral" - should wait for debounce before showing card.
	wordRect, ok := tt.Find("ephemeral")
	if !ok {
		t.Fatalf("word 'ephemeral' not found in %q", tt.Texts())
	}
	tt.Move(wordRect.X+wordRect.W/2, wordRect.Y+wordRect.H/2)
	if tt.HasText("查看完整释义与例句") {
		t.Fatal("hover card opened before debounce delay")
	}

	tt.Advance(wordHoverShowDelay + 30*time.Millisecond)
	waitFor(t, updated, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return a.wordHover.status == "ready"
	})
	tt.Frame()

	for _, want := range []string{"ephemeral", "/ɪˈfɛm.ə.ɹəl/", "Lasting for a short period of time.", "Fame is ephemeral.", "查看完整释义与例句", "来源：Wiktionary"} {
		if !tt.HasText(want) {
			t.Errorf("hover card missing %q in %q", want, tt.Texts())
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("lookup calls = %d, want 1", calls.Load())
	}

	// Move pointer into the hover card itself -> should remain open past hide delay.
	cardRect, ok := tt.Find("ephemeral 的释义")
	if !ok {
		t.Fatal("hover card accessible dialog not found")
	}
	tt.Move(cardRect.X+cardRect.W/2, cardRect.Y+cardRect.H/2)
	tt.Advance(wordHoverHideDelay + 100*time.Millisecond)
	tt.Frame()
	if !tt.HasText("查看完整释义与例句") {
		t.Fatal("hover card closed while pointer hovered inside it")
	}

	// Move pointer away -> closes after hide delay.
	tt.Move(5, 5)
	tt.Advance(wordHoverHideDelay + 50*time.Millisecond)
	tt.Frame()
	if tt.HasText("查看完整释义与例句") {
		t.Fatal("hover card did not close after pointer left")
	}

	// Re-hovering cached word should not trigger another network call.
	tt.Move(wordRect.X+wordRect.W/2, wordRect.Y+wordRect.H/2)
	tt.Advance(wordHoverShowDelay + 30*time.Millisecond)
	tt.Frame()
	if !tt.HasText("Lasting for a short period of time.") {
		t.Fatal("cached word preview did not appear")
	}
	if calls.Load() != 1 {
		t.Fatalf("cached lookup triggered extra network call: %d", calls.Load())
	}

	// Clicking "查看完整释义与例句" switches to translate, flips target to Chinese, and starts lookup.
	if err := tt.Click("查看完整释义与例句"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, updated, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return !a.busy && a.result == "短暂的"
	})
	if a.text != "ephemeral" || a.action != "翻译" || a.Data.Settings.Target != "简体中文" {
		t.Fatalf("openWordDetails state: text=%q action=%q target=%q", a.text, a.action, a.Data.Settings.Target)
	}
	streamer.mu.Lock()
	nQueries := len(streamer.queries)
	streamer.mu.Unlock()
	if nQueries != 1 {
		t.Fatalf("expected 1 translation query, got %d", nQueries)
	}
}

func TestWordHoverCardEmptyErrorAndPlainText(t *testing.T) {
	origLookup := lookupWordFunc
	t.Cleanup(func() { lookupWordFunc = origLookup })

	var failCalls atomic.Int32
	lookupWordFunc = func(_ context.Context, _ *http.Client, word string) (*core.WordPreview, error) {
		if core.NormalizeLookupWord(word) == "missingword" {
			return nil, nil
		}
		failCalls.Add(1)
		return nil, errors.New("timeout")
	}

	a := testApp(t)
	a.resultFormat = "纯文本"
	a.result = "missingword and brokenword"

	var mu sync.Mutex
	updated := make(chan struct{}, 16)
	a.post = func(fn func()) {
		mu.Lock()
		fn()
		mu.Unlock()
		select {
		case updated <- struct{}{}:
		default:
		}
	}

	tt := ui.NewTester(func(c *ui.Context) {
		mu.Lock()
		defer mu.Unlock()
		a.View(c)
	}, 560, 760)

	// 404 word shows empty message.
	r1, ok := tt.Find("missingword")
	if !ok {
		t.Fatalf("missingword not found in %q", tt.Texts())
	}
	tt.Move(r1.X+r1.W/2, r1.Y+r1.H/2)
	tt.Advance(wordHoverShowDelay + 30*time.Millisecond)
	waitFor(t, updated, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return a.wordHover.status == "empty"
	})
	tt.Frame()
	if !tt.HasText("未找到简明释义") {
		t.Fatalf("expected empty notice, got %q", tt.Texts())
	}

	// Error word shows error notice and is not cached on retry.
	r2, ok := tt.Find("brokenword")
	if !ok {
		t.Fatalf("brokenword not found in %q", tt.Texts())
	}
	tt.Move(r2.X+r2.W/2, r2.Y+r2.H/2)
	tt.Advance(wordHoverShowDelay + 30*time.Millisecond)
	waitFor(t, updated, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return a.wordHover.status == "error"
	})
	tt.Frame()
	if !tt.HasText("暂时无法加载预览") {
		t.Fatalf("expected error notice, got %q", tt.Texts())
	}
	if failCalls.Load() != 1 {
		t.Fatalf("failCalls = %d, want 1", failCalls.Load())
	}

	// Hide and hover brokenword again -> retries because errors are not cached.
	tt.Move(5, 5)
	tt.Advance(wordHoverHideDelay + 50*time.Millisecond)
	tt.Frame()
	tt.Move(r2.X+r2.W/2, r2.Y+r2.H/2)
	tt.Advance(wordHoverShowDelay + 30*time.Millisecond)
	waitFor(t, updated, func() bool {
		return failCalls.Load() == 2
	})
	if strings.Contains(a.failure, "timeout") {
		t.Fatalf("hover preview error should stay inside hover card, not set global failure: %q", a.failure)
	}
}
