package core

import (
	"reflect"
	"testing"
)

func TestParsePhoneticSegments(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		fallback string
		numbered bool
		want     []PhoneticSegment
	}{
		{
			name: "inline IPA with word before it",
			text: "1. hello · /həˈloʊ/",
			want: []PhoneticSegment{
				{Kind: PhoneticText, Text: "1. hello · "},
				{Kind: PhoneticIPA, Text: "/həˈloʊ/", SpeechText: "hello"},
			},
		},
		{
			name: "preceding line word when line starts with language label",
			text: "hello\n[English]· /həˈloʊ/",
			want: []PhoneticSegment{
				{Kind: PhoneticText, Text: "hello\n[English]· "},
				{Kind: PhoneticIPA, Text: "/həˈloʊ/", SpeechText: "hello"},
			},
		},
		{
			name: "ordinary slash path is not phonetic",
			text: "Open /usr/local/bin",
			want: []PhoneticSegment{
				{Kind: PhoneticText, Text: "Open /usr/local/bin"},
			},
		},
		{
			name: "inline Chinese example without reading translation",
			text: "例句：How are you? (你好吗？)",
			want: []PhoneticSegment{
				{Kind: PhoneticText, Text: "例句："},
				{Kind: PhoneticExample, Text: "How are you?", SpeechText: "How are you?"},
				{Kind: PhoneticText, Text: " (你好吗？)"},
			},
		},
		{
			name: "numbered bilingual example",
			text: "2. She said hi.（她打了招呼。）",
			want: []PhoneticSegment{
				{Kind: PhoneticText, Text: "2. "},
				{Kind: PhoneticExample, Text: "She said hi.", SpeechText: "She said hi."},
				{Kind: PhoneticText, Text: "（她打了招呼。）"},
			},
		},
		{
			name: "numbered example without punctuation after number",
			text: "1 How are you?（你好吗？）",
			want: []PhoneticSegment{
				{Kind: PhoneticText, Text: "1 "},
				{Kind: PhoneticExample, Text: "How are you?", SpeechText: "How are you?"},
				{Kind: PhoneticText, Text: "（你好吗？）"},
			},
		},
		{
			name: "numbered definition without translation is not an example",
			text: "1. an expression of greeting",
			want: []PhoneticSegment{
				{Kind: PhoneticText, Text: "1. an expression of greeting"},
			},
		},
		{
			name:     "markdown ordered list item example",
			text:     "How are you? (你好吗？)",
			numbered: true,
			want: []PhoneticSegment{
				{Kind: PhoneticExample, Text: "How are you?", SpeechText: "How are you?"},
				{Kind: PhoneticText, Text: " (你好吗？)"},
			},
		},
		{
			name:     "fallback word from source when label line is standalone",
			text:     "[English]· /həˈloʊ/",
			fallback: "hello",
			want: []PhoneticSegment{
				{Kind: PhoneticText, Text: "[English]· "},
				{Kind: PhoneticIPA, Text: "/həˈloʊ/", SpeechText: "hello"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParsePhoneticSegmentsWithContext(tc.text, "", tc.fallback, tc.numbered)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %#v, want %#v", got, tc.want)
			}
		})
	}
}
