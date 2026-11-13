package app

import (
	_ "embed"
	"encoding/json"

	"github.com/egoist/mygo/ui"
)

//go:embed ui-locales.json
var uiLocaleJSON []byte

var uiLocales = func() map[string]map[string]string {
	var locales map[string]map[string]string
	if err := json.Unmarshal(uiLocaleJSON, &locales); err != nil {
		panic(err)
	}
	return locales
}()

var interfaceLanguages = []string{"简体中文", "繁體中文", "English", "日本語", "한국어", "ไทย", "Türkçe", "Português (Brasil)"}
var interfaceCodes = []string{"zh-Hans", "zh-Hant", "en", "ja", "ko", "th", "tr", "pt-BR"}

func (a *App) tr(text string) string {
	if translation := uiLocales[a.Data.Settings.UILanguage][text]; translation != "" {
		return translation
	}
	return text
}

func (a *App) selectSetting(c *ui.Context, selected *string, options []string) ui.Element {
	if a.Data.Settings.UILanguage == "zh-Hans" || a.Data.Settings.UILanguage == "" {
		return ui.Select(c, selected, options)
	}
	translated := make([]string, len(options))
	for i, value := range options {
		translated[i] = a.tr(value)
	}
	value := a.tr(*selected)
	e := ui.Select(c, &value, translated)
	if e.Changed() {
		for i, label := range translated {
			if label == value {
				*selected = options[i]
				break
			}
		}
	}
	return e
}

func (a *App) languagePicker(c *ui.Context, selected *string, options []string, label string) ui.Element {
	value := *selected
	translated := append([]string(nil), options...)
	for i, option := range translated {
		if option == "自动检测" {
			translated[i] = a.tr(option)
		}
	}
	if value == "自动检测" {
		value = a.tr(value)
	}
	e := languagePicker(c, &value, translated, label, a.tr)
	if value == a.tr("自动检测") {
		*selected = "自动检测"
	} else {
		*selected = value
	}
	return e
}
