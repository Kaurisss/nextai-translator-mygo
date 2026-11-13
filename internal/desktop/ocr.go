package desktop

import (
	"strings"
	"unicode"
)

func joinCJKSpaces(text string) string {
	isCJK := func(r rune) bool {
		return unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) ||
			r >= '\u3000' && r <= '\u303f' || r >= '\uff00' && r <= '\uffef'
	}
	var out strings.Builder
	chars := []rune(text)
	for i := 0; i < len(chars); i++ {
		if chars[i] == ' ' || chars[i] == '\t' || chars[i] == '\u3000' {
			end := i + 1
			for end < len(chars) && (chars[end] == ' ' || chars[end] == '\t' || chars[end] == '\u3000') {
				end++
			}
			if i > 0 && end < len(chars) && isCJK(chars[i-1]) && isCJK(chars[end]) {
				i = end - 1
				continue
			}
		}
		out.WriteRune(chars[i])
	}
	return strings.TrimSpace(out.String())
}

func bestOCRResult(results []string) string {
	best, profileScore, bestScore := "", 0, 0
	for i, text := range results {
		text = joinCJKSpaces(text)
		score := 0
		for _, r := range text {
			if !unicode.IsSpace(r) {
				score++
			}
		}
		if i == 0 {
			best, profileScore, bestScore = text, score, score
		} else if score*5 > profileScore*6 && score > bestScore {
			best, bestScore = text, score
		}
	}
	return best
}
