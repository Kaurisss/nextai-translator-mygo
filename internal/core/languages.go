package core

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf16"
)

type languageConfig struct {
	Code             string `json:"code"`
	Name             string `json:"name"`
	NameEn           string `json:"nameEn"`
	PromptName       string `json:"promptName"`
	PhoneticNotation string `json:"phoneticNotation"`
	RolePrompt       string `json:"rolePrompt"`
}

//go:embed languages.json
var languageJSON []byte

var promptLanguages = func() map[string]languageConfig {
	var configs []languageConfig
	if err := json.Unmarshal(languageJSON, &configs); err != nil {
		panic(err)
	}
	// These existing native options extend the original language catalogue.
	for _, l := range [][3]string{{"ms", "Bahasa Melayu", "Malay"}, {"no", "Norsk", "Norwegian"}, {"ro", "Română", "Romanian"}, {"sk", "Slovenčina", "Slovak"}, {"sl", "Slovenščina", "Slovenian"}, {"sr", "Српски", "Serbian"}, {"ta", "தமிழ்", "Tamil"}, {"te", "తెలుగు", "Telugu"}, {"mr", "मराठी", "Marathi"}, {"kn", "ಕನ್ನಡ", "Kannada"}, {"ml", "മലയാളം", "Malayalam"}, {"pa", "ਪੰਜਾਬੀ", "Panjabi"}, {"ne", "नेपाली", "Nepali"}, {"si", "සිංහල", "Sinhala"}, {"my", "မြန်မာ", "Burmese"}, {"km", "ខ្មែរ", "Khmer"}, {"lo", "ລາວ", "Lao"}, {"sw", "Kiswahili", "Swahili"}, {"la", "Latina", "Latin"}} {
		configs = append(configs, languageConfig{l[0], l[1], l[2], l[2], "transcription", "You are a professional translator."})
	}
	m := map[string]languageConfig{}
	for _, c := range configs {
		m[c.Code] = c
		m[c.Name] = c
	}
	for alias, code := range map[string]string{"Azərbaycanca": "az", "Қазақша": "kk", "Монгол": "mn", "粵語": "yue", "文言文": "lzh"} {
		m[alias] = m[code]
	}
	return m
}()

func promptLanguage(name string) (languageConfig, error) {
	if name == "" {
		return languageConfig{PromptName: "", NameEn: "Unknown", PhoneticNotation: "transcription"}, nil
	}
	c, ok := promptLanguages[name]
	if !ok {
		return languageConfig{}, fmt.Errorf("不支持的提示词语言: %s", name)
	}
	return c, nil
}

//go:embed chinese-variants.json
var chineseVariantsJSON []byte

var chineseVariants = func() struct{ Simplified, Traditional, Vietnamese string } {
	var v struct{ Simplified, Traditional, Vietnamese string }
	if err := json.Unmarshal(chineseVariantsJSON, &v); err != nil {
		panic(err)
	}
	return v
}()

// detectPromptLanguage ports localDetectLang, including its stable tie order.
func detectPromptLanguage(text string) string {
	units := utf16.Encode([]rune(text))
	if len(units) > 200 {
		units = units[:200]
	}
	text = string(utf16.Decode(units))
	codes := []string{"en", "zh", "ko", "vi", "th", "hmn", "ja", "ru", "es", "fr", "de"}
	w := map[string]int{}
	for _, r := range text {
		if r >= 0x4e00 && r <= 0x9fa5 {
			w["zh"]++
			w["ja"]++
		}
		if r >= 0xac00 && r <= 0xd7a3 {
			w["ko"]++
		}
		if strings.ContainsRune(chineseVariants.Vietnamese, r) {
			w["vi"]++
		}
		if r >= 0xe01 && r <= 0xe5b {
			w["th"]++
		}
		if r >= 0x16f0 && r <= 0x16f9 {
			w["hmn"]++
		}
		if r >= 0x3040 && r <= 0x30ff {
			w["ja"]++
		}
		if r >= 0x400 && r <= 0x4ff {
			w["ru"]++
		}
		if strings.ContainsRune("áéíóúüñ", r) {
			w["es"]++
		}
		if strings.ContainsRune("àâçéèêëîïôûùüÿœæ", r) {
			w["fr"]++
		}
		if strings.ContainsRune("äöüß", r) {
			w["de"]++
		}
	}
	for _, word := range strings.Split(text, " ") {
		for _, r := range word {
			if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
				for _, c := range []string{"en", "es", "fr", "de"} {
					w[c]++
				}
				break
			}
		}
	}
	if w["zh"] > 0 && w["zh"] == w["ja"] {
		w["zh"]++
	}
	if w["en"] > 0 && w["en"] == w["es"] && w["en"] == w["fr"] && w["en"] == w["de"] {
		w["en"]++
	}
	best := "en"
	for _, code := range codes {
		if w[code] > w[best] {
			best = code
		}
	}
	if best == "zh" {
		s, t := 0, 0
		for _, r := range text {
			if strings.ContainsRune(chineseVariants.Simplified, r) {
				s++
			} else if strings.ContainsRune(chineseVariants.Traditional, r) {
				t++
			}
		}
		if t > s {
			return "zh-Hant"
		}
		return "zh-Hans"
	}
	return best
}
