package core

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"unicode/utf16"

	"github.com/ebitengine/purego"
)

type wordIteratorAPI struct {
	open  func(int32, *byte, *uint16, int32, *int32) uintptr
	first func(uintptr) int32
	next  func(uintptr) int32
	close func(uintptr)
}

func bindWordIterator(symbol func(string) (uintptr, error)) (*wordIteratorAPI, error) {
	var addresses [4]uintptr
	for i, name := range []string{"ubrk_open", "ubrk_first", "ubrk_next", "ubrk_close"} {
		address, err := symbol(name)
		if err != nil {
			return nil, err
		}
		addresses[i] = address
	}
	api := &wordIteratorAPI{}
	purego.RegisterFunc(&api.open, addresses[0])
	purego.RegisterFunc(&api.first, addresses[1])
	purego.RegisterFunc(&api.next, addresses[2])
	purego.RegisterFunc(&api.close, addresses[3])
	return api, nil
}

var wordIterator = sync.OnceValues(loadWordIterator)

// isWord mirrors Intl.Segmenter(locale, {granularity: 'word'}): the first
// segment must cover the entire text. As in the source, punctuation and numbers
// are not excluded with isWordLike. ICU supplies dictionary boundaries for CJK.
func isWord(locale, text string) (bool, error) {
	units := utf16.Encode([]rune(text))
	if len(units) == 0 {
		return false, nil
	}
	api, err := wordIterator()
	if err != nil {
		return false, fmt.Errorf("加载 Unicode 分词: %w", err)
	}
	localeBytes := append([]byte(locale), 0)
	var status int32
	iterator := api.open(1, &localeBytes[0], &units[0], int32(len(units)), &status)
	if status > 0 || iterator == 0 {
		return false, fmt.Errorf("Unicode 分词失败: ICU %d", status)
	}
	defer func() { api.close(iterator); runtime.KeepAlive(units); runtime.KeepAlive(localeBytes) }()
	api.first(iterator)
	return api.next(iterator) == int32(len(units)), nil
}

// IsWord reports whether text is a single word in language (or auto-detected
// when language is "自动检测"), matching dictionary/word-mode detection.
func IsWord(language, text string) bool {
	trimmed := strings.TrimFunc(text, jsWhitespace)
	if trimmed == "" {
		return false
	}
	code := LanguageCode(language, trimmed)
	if code == "" {
		code = "en"
	}
	ok, err := isWord(code, trimmed)
	return err == nil && ok
}
