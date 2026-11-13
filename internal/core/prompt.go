package core

import (
	"fmt"
	"strings"
	"unicode/utf16"
)

type Prompt struct {
	RolePrompt    string `json:"rolePrompt"`
	CommandPrompt string `json:"commandPrompt"`
	WordMode      bool   `json:"wordMode"`
}

func (p Prompt) Combined() string {
	if p.RolePrompt != "" {
		return p.RolePrompt + "\n\n" + p.CommandPrompt
	}
	return p.CommandPrompt
}

// BuildPrompt ports src/common/translate.ts. Template text, whitespace,
// branch order and first-occurrence ${...} substitutions match the source.
func BuildPrompt(q Query) (Prompt, error) {
	var p Prompt
	content := q.Text
	if q.Action.Mode == "big-bang" {
		article := q.ArticlePrompt
		if article == "" {
			article = q.Action.ArticlePrompt
		}
		p.RolePrompt = "You are a professional writer and you will write " + article + " based on the given words"
		p.CommandPrompt = "Write " + article + " of no more than 160 words. The article must contain the words in the following text. The more words you use, the better"
	} else {
		sourceName := q.Source
		if sourceName == "自动检测" || sourceName == "the automatically detected source language" {
			sourceName = detectPromptLanguage(q.Text)
		}
		source, err := promptLanguage(sourceName)
		if err != nil {
			return p, err
		}
		target, err := promptLanguage(q.Target)
		if err != nil {
			return p, err
		}
		p.RolePrompt = target.RolePrompt
		switch q.Action.Mode {
		case "":
			if strings.Contains(q.Action.Prompt, "${text}") || strings.Contains(q.Action.CommandPrompt, "${text}") {
				content = ""
			}
			p.RolePrompt = customPrompt(q.Action.Prompt, source.PromptName, target.PromptName, q.Text)
			p.CommandPrompt = customPrompt(q.Action.CommandPrompt, source.PromptName, target.PromptName, q.Text)
			if q.Action.Format != "" {
				p.CommandPrompt = "(Requirements: The output format must be " + renderingFormat(q.Action.Format) + ") " + p.CommandPrompt
			}
		case "translate":
			p.CommandPrompt = "Please translate to " + target.Name
			if target.Code == "ko-banmal" {
				p.CommandPrompt = "Translate from " + source.NameEn + " to Korean banmal. Please use 이다 and 다 endings. Never use formal or honorific endings. Return translated text only."
			}
			toChinese := isChineseTarget(target.Code)
			if !q.Writing && len(utf16.Encode([]rune(q.Text))) < 5 && toChinese {
				p.RolePrompt = fmt.Sprintf("你是一个翻译引擎，请将给到的文本翻译成%s。请列出3种（如果有）最常用翻译结果：单词或短语，并列出对应的适用语境（用中文阐述）、音标或转写、词性、双语示例。按照下面格式用中文阐述：\n    <序号><单词或短语> · /<%s>/\n    [<词性缩写>] <适用语境（用中文阐述）>\n    例句：<例句>(例句翻译)", target.PromptName, phoneticText(target))
				p.CommandPrompt = ""
			}
			if !q.Writing {
				word, err := isWord(source.Code, strings.TrimFunc(q.Text, jsWhitespace))
				if err != nil {
					return p, err
				}
				if word {
					p.WordMode = true
					if toChinese {
						phonetic := "false"
						if target.PhoneticNotation != "" {
							phonetic = "对应的音标或转写、"
						}
						p.RolePrompt = "你是一个翻译引擎，请翻译给出的文本，只需要翻译不需要解释。当且仅当文本只有一个单词时，请给出单词原始形态（如果有）、单词的语种、" + phonetic + "所有含义（含词性）、双语示例，至少三条例句。如果你认为单词拼写错误，请提示我最可能的正确拼写，否则请严格按照下面格式给到翻译结果：\n    <单词>\n    [<语种>]· / " + phoneticPlaceholder(target) + "\n    [<词性缩写>] <中文含义>]\n    例句：\n    <序号><例句>(例句翻译)\n    词源：\n    <词源>"
						p.CommandPrompt = "好的，我明白了，请给我这个单词。"
						content = "单词是：" + q.Text
					} else {
						phonetic := "false"
						if target.PhoneticNotation != "" {
							phonetic = "the corresponding phonetic notation or transcription, "
						}
						examples, meaning := "sentence", ""
						if source.Code != target.Code {
							examples = "bilingual  sentence"
							meaning = "<translated meaning> /  "
						}
						p.RolePrompt = fmt.Sprintf("You are a professional translation engine. Please translate the text into %s without explanation. When the text has only one word, please act as a professional %s-%s dictionary, and list the original form of the word (if any), the language of the word, %s all senses with parts of speech, %s examples (at least 3) and etymology. If you think there is a spelling mistake, please tell me the most possible correct word otherwise reply in the following format:\n<word> (<original form>)\n[<language>]· / %s\n[<part of speech>] %s<meaning in source language>\nExamples:\n<index>. <sentence>(<sentence translation>)\nEtymology:\n<etymology>", target.PromptName, source.PromptName, target.PromptName, phonetic, examples, phoneticPlaceholder(target), meaning)
						p.CommandPrompt = "I understand. Please give me the word."
						content = "The word is: " + q.Text
					}
				}
			}
			if !q.Writing && q.SelectedWord != "" {
				phonetic := "false"
				if source.PhoneticNotation != "" {
					phonetic = "Firstly, provide the corresponding phonetic notation or transcription of the word in " + source.PromptName + "."
				}
				p.RolePrompt = fmt.Sprintf("You are an expert in the semantic syntax of the %s language, and you are teaching me the %s language. I will give you a sentence in %s and a word from that sentence. %s Then, help me explain in %s what the word means in the sentence, what the sentence itself means, and whether the word is part of an idiom in the sentence. If it is, explain the idiom in the sentence. Provide 3 to 5 examples in %s with the same meaning, and explain these examples in %s. The answer should follow the format below:\n\n<word> · /%s/\n<the remaining part>\n\nIf you understand, say \"yes\", and then we will begin.", source.PromptName, source.PromptName, source.PromptName, phonetic, target.PromptName, source.PromptName, target.PromptName, phoneticPlaceholder(source))
				p.CommandPrompt = "Yes, I understand. Please give me the sentence and the word."
				content = "the sentence is: " + q.Text + "\n\nthe word is: " + q.SelectedWord
			}
		case "polishing":
			p.RolePrompt = "You are an expert translator, translate directly without explanation."
			p.CommandPrompt = "Please edit the following sentences in " + source.PromptName + " to improve clarity, conciseness, and coherence, making them match the expression of native speakers."
		case "summarize":
			p.RolePrompt = "You are a professional text summarizer, you can only summarize the text, don't interpret it."
			p.CommandPrompt = "Please summarize this text in the most concise language and must use " + target.PromptName + " language!"
		case "analyze":
			p.RolePrompt = "You are a professional translation engine and grammar analyzer."
			p.CommandPrompt = "Please translate this text to " + target.PromptName + " and explain the grammar in the original text using " + target.PromptName + "."
		case "explain-code":
			p.RolePrompt = "You are a code explanation engine that can only explain code but not interpret or translate it. Also, please report bugs and errors (if any)."
			p.CommandPrompt = "explain the provided code, regex or script in the most concise language and must use " + target.PromptName + " language! You may use Markdown. If the content is not code, return an error message. If the code has obvious errors, point them out."
			content = "```\n" + q.Text + "\n```"
		default:
			return p, fmt.Errorf("未知内置动作: %s", q.Action.Mode)
		}
	}
	if content != "" {
		p.CommandPrompt = "Only reply the result and nothing else. " + p.CommandPrompt + ":\n\n" + strings.TrimRightFunc(content, jsWhitespace)
	}
	return p, nil
}

func isChineseTarget(code string) bool {
	switch code {
	case "zh-Hans", "zh-Hant", "lzh", "yue", "jdbhw", "xdbhw":
		return true
	}
	return false
}
func phoneticText(l languageConfig) string {
	if l.PhoneticNotation == "" {
		return "false"
	}
	return l.PhoneticNotation
}
func phoneticPlaceholder(l languageConfig) string {
	if l.PhoneticNotation == "" {
		return "false"
	}
	return "<" + l.PhoneticNotation + ">"
}
func renderingFormat(s string) string {
	switch s {
	case "Markdown":
		return "markdown"
	case "LaTeX":
		return "latex"
	case "纯文本":
		return "text"
	}
	return s
}
func customPrompt(template, source, target, text string) string {
	for _, pair := range [][2]string{{"${sourceLang}", source}, {"${targetLang}", target}, {"${text}", text}} {
		template = strings.Replace(template, pair[0], pair[1], 1)
	}
	return strings.NewReplacer("{{source}}", source, "{{target}}", target).Replace(template)
}
func jsWhitespace(r rune) bool {
	return r >= '\t' && r <= '\r' || r == ' ' || r == 0xa0 || r == 0x1680 || r >= 0x2000 && r <= 0x200a || r == 0x2028 || r == 0x2029 || r == 0x202f || r == 0x205f || r == 0x3000 || r == 0xfeff
}
