package core

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// PhoneticKind distinguishes plain text from IPA phonetic transcriptions and
// bilingual example sentences that receive inline speech buttons.
type PhoneticKind uint8

const (
	PhoneticText PhoneticKind = iota
	PhoneticIPA
	PhoneticExample
)

// PhoneticSegment is a piece of a translation or dictionary result.
type PhoneticSegment struct {
	Kind       PhoneticKind
	Text       string
	SpeechText string
}

var (
	phoneticPattern            = regexp.MustCompile(`·\s*(/\s*([^/\n]{1,120}?)\s*/)`)
	exampleHeaderPattern       = regexp.MustCompile(`(?i)(?:例句|示例|examples?|example sentences?)\s*[：:]\s*`)
	numberedExamplePattern     = regexp.MustCompile(`^\s*(?:[-*+]\s*)?\d+(?:[.)、]\s*|\s+)`)
	trailingTranslationPattern = regexp.MustCompile(`\s*([（(][^()（）\n]{1,500}[)）])\s*$`)
	headingPrefixPattern       = regexp.MustCompile(`^\s{0,3}#{1,6}\s+`)
	listPrefixPattern          = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)、])\s*`)
	markdownStylePattern       = regexp.MustCompile(`(?:\*\*|__|~~|[*_` + "`" + `])`)
	trailingParenPattern       = regexp.MustCompile(`\s+\([^)]*\)\s*$`)
	bracketOnlyPattern         = regexp.MustCompile(`^\[[^\]]+\]$`)
)

// HasPhoneticOrExample reports whether text may contain an inline phonetic
// transcription or bilingual example sentence.
func HasPhoneticOrExample(text string, numberedFirstLine bool) bool {
	if phoneticPattern.FindStringIndex(text) != nil {
		return true
	}
	for i, line := range strings.Split(text, "\n") {
		if parseExampleLine(line, numberedFirstLine && i == 0) != nil {
			return true
		}
	}
	return false
}

// ParsePhoneticSegments parses text into plain text, IPA phonetic segments,
// and bilingual example segments matching the original PhoneticText rules.
func ParsePhoneticSegments(text, fallbackText string) []PhoneticSegment {
	return ParsePhoneticSegmentsWithContext(text, "", fallbackText, false)
}

// ParsePhoneticSegmentsWithContext parses text using precedingText (text that
// appeared earlier in the same paragraph) and fallbackText (previous block or
// source text) to resolve the spoken word for phonetic transcriptions. When
// numberedFirstLine is true (such as a Markdown ordered list item), the first
// line is treated as a numbered example candidate.
func ParsePhoneticSegmentsWithContext(text, precedingText, fallbackText string, numberedFirstLine bool) []PhoneticSegment {
	var segments []PhoneticSegment
	appendSegment := func(seg PhoneticSegment) {
		if seg.Text == "" {
			return
		}
		if n := len(segments); n > 0 && seg.Kind == PhoneticText && segments[n-1].Kind == PhoneticText {
			segments[n-1].Text += seg.Text
			return
		}
		segments = append(segments, seg)
	}

	lineIdx := 0
	for _, ps := range parsePhonetics(text, precedingText, fallbackText) {
		if ps.Kind != PhoneticText {
			appendSegment(ps)
			continue
		}
		parts := strings.Split(ps.Text, "\n")
		for idx, line := range parts {
			if idx > 0 {
				appendSegment(PhoneticSegment{Kind: PhoneticText, Text: "\n"})
				lineIdx++
			}
			if line == "" {
				continue
			}
			if exampleSegs := parseExampleLine(line, numberedFirstLine && lineIdx == 0); exampleSegs != nil {
				for _, seg := range exampleSegs {
					appendSegment(seg)
				}
			} else {
				appendSegment(PhoneticSegment{Kind: PhoneticText, Text: line})
			}
		}
	}
	if len(segments) == 0 {
		return []PhoneticSegment{{Kind: PhoneticText, Text: text}}
	}
	return segments
}

func cleanSpeechCandidate(value string) string {
	candidate := strings.TrimSpace(value)
	candidate = headingPrefixPattern.ReplaceAllString(candidate, "")
	candidate = listPrefixPattern.ReplaceAllString(candidate, "")
	candidate = markdownStylePattern.ReplaceAllString(candidate, "")
	candidate = trailingParenPattern.ReplaceAllString(candidate, "")
	candidate = strings.TrimSpace(candidate)
	if candidate == "" || bracketOnlyPattern.MatchString(candidate) || utf8.RuneCountInString(candidate) > 120 {
		return ""
	}
	return candidate
}

func findSpeechText(preceding, fallbackText string) string {
	lines := strings.Split(preceding, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if candidate := cleanSpeechCandidate(lines[i]); candidate != "" {
			return candidate
		}
	}
	return cleanSpeechCandidate(fallbackText)
}

func parsePhonetics(text, precedingText, fallbackText string) []PhoneticSegment {
	matches := phoneticPattern.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return []PhoneticSegment{{Kind: PhoneticText, Text: text}}
	}
	var segments []PhoneticSegment
	cursor := 0
	for _, m := range matches {
		matchStart, matchEnd := m[0], m[1]
		phoneticStart, phoneticEnd := m[2], m[3]
		if phoneticStart > cursor {
			segments = append(segments, PhoneticSegment{Kind: PhoneticText, Text: text[cursor:phoneticStart]})
		}
		speechText := findSpeechText(precedingText+text[:matchStart], fallbackText)
		if speechText == "" {
			segments = append(segments, PhoneticSegment{Kind: PhoneticText, Text: text[phoneticStart:phoneticEnd]})
		} else {
			segments = append(segments, PhoneticSegment{
				Kind:       PhoneticIPA,
				Text:       text[phoneticStart:phoneticEnd],
				SpeechText: speechText,
			})
		}
		cursor = matchEnd
	}
	if cursor < len(text) {
		segments = append(segments, PhoneticSegment{Kind: PhoneticText, Text: text[cursor:]})
	}
	return segments
}

func parseExampleLine(line string, numbered bool) []PhoneticSegment {
	translation := trailingTranslationPattern.FindStringIndex(line)
	if translation == nil {
		return nil
	}
	beforeTranslation := strings.TrimRight(line[:translation[0]], " \t\r")
	header := exampleHeaderPattern.FindStringIndex(beforeTranslation)
	number := numberedExamplePattern.FindStringIndex(beforeTranslation)
	if header == nil && number == nil && !numbered {
		return nil
	}
	exampleStart := 0
	if header != nil {
		exampleStart = header[1]
	} else if number != nil {
		exampleStart = number[1]
	}
	speechText := strings.TrimSpace(beforeTranslation[exampleStart:])
	if speechText == "" || utf8.RuneCountInString(speechText) > 500 {
		return nil
	}
	rel := strings.Index(beforeTranslation[exampleStart:], speechText)
	if rel < 0 {
		return nil
	}
	displayStart := exampleStart + rel
	return []PhoneticSegment{
		{Kind: PhoneticText, Text: line[:displayStart]},
		{Kind: PhoneticExample, Text: speechText, SpeechText: speechText},
		{Kind: PhoneticText, Text: line[displayStart+len(speechText):]},
	}
}
