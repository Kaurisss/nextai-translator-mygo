package core

import (
	"errors"
	"fmt"
	"net/url"
	"runtime"
	"strings"
)

func ValidateSettings(s Settings) error {
	switch s.UILanguage {
	case "", "zh-Hans", "zh-Hant", "en", "ja", "ko", "th", "tr", "pt-BR":
	default:
		return errors.New("不支持的界面语言")
	}
	p, ok := s.Profiles[s.Provider]
	if !ok {
		return errors.New("请选择模型服务")
	}
	u, err := url.Parse(p.Endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("API 地址必须是完整的 HTTP 或 HTTPS URL")
	}
	if strings.TrimSpace(p.Model) == "" {
		return errors.New("请填写模型")
	}
	if s.Speech.Rate < 0.5 || s.Speech.Rate > 2 || s.Speech.Volume < 0 || s.Speech.Volume > 100 {
		return errors.New("语速须为 0.5–2，音量须为 0–100")
	}
	if s.Speech.Provider != "系统默认" && s.Speech.Provider != "本地 TTS (MeloTTS / Kokoro)" {
		return errors.New("请选择有效的 TTS 提供者")
	}
	if _, err := promptLanguage(s.Writing.Target); err != nil {
		return err
	}
	voices := map[string]bool{}
	for _, v := range s.Speech.Voices {
		key := v.Provider + "/" + v.Language
		if voices[key] {
			return errors.New("每种语言只能配置一个音色")
		}
		voices[key] = true
		if _, err := promptLanguage(v.Language); err != nil {
			return err
		}
	}
	seen := map[string]string{}
	for _, shortcut := range []struct{ Name, Key string }{{"取词", s.Shortcut}, {"显示窗口", s.DisplayShortcut}, {"OCR", s.OCRShortcut}, {"快捷翻译", s.QuickShortcut}, {"写作", s.Writing.Shortcut}} {
		if shortcut.Key == "" {
			continue
		}
		key := NormalizeShortcut(shortcut.Key)
		if other := seen[key]; other != "" {
			return fmt.Errorf("%s和%s使用了相同快捷键", shortcut.Name, other)
		}
		seen[key] = shortcut.Name
	}
	return nil
}

func NormalizeShortcut(key string) string {
	modifier := "ctrl"
	if runtime.GOOS == "darwin" {
		modifier = "cmd"
	}
	key = strings.ToLower(strings.ReplaceAll(key, " ", ""))
	key = strings.ReplaceAll(key, "commandorcontrol", modifier)
	key = strings.ReplaceAll(key, "cmdorctrl", modifier)
	key = strings.ReplaceAll(key, "control", "ctrl")
	key = strings.ReplaceAll(key, "command", "cmd")
	parts := strings.Split(key, "+")
	mods := map[string]bool{}
	last := ""
	for _, part := range parts {
		switch part {
		case "ctrl", "cmd", "alt", "shift", "super":
			mods[part] = true
		default:
			last = part
		}
	}
	var ordered []string
	for _, mod := range []string{"ctrl", "cmd", "alt", "shift", "super"} {
		if mods[mod] {
			ordered = append(ordered, mod)
		}
	}
	return strings.Join(append(ordered, last), "+")
}

func LanguageCode(name, text string) string {
	if name == "自动检测" {
		return detectPromptLanguage(text)
	}
	l, err := promptLanguage(name)
	if err != nil {
		return ""
	}
	return l.Code
}
