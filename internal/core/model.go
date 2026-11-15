package core

import (
	"maps"
	"slices"
	"time"
)

type Profile struct {
	Provider      string          `json:"provider"`
	Endpoint      string          `json:"endpoint"`
	Model         string          `json:"model"`
	NoModelsAPI   bool            `json:"noModelsAPI,omitempty"`
	Thinking      bool            `json:"thinking,omitempty"`
	ThinkingLevel string          `json:"thinkingLevel,omitempty"`
	MaxTokens     int             `json:"maxTokens,omitempty"`
	KeepAlive     string          `json:"keepAlive,omitempty"`
	ModelThinking map[string]bool `json:"modelThinking,omitempty"`
}

type Settings struct {
	UILanguage        string             `json:"uiLanguage,omitempty"`
	Provider          string             `json:"provider"`
	Profiles          map[string]Profile `json:"profiles"`
	Source            string             `json:"source"`
	Target            string             `json:"target"`
	Shortcut          string             `json:"shortcut"`
	Appearance        string             `json:"appearance"`
	HideTaskbarIcon   bool               `json:"hideTaskbarIcon,omitempty"`
	FontSize          int                `json:"fontSize,omitempty"`
	UIFontSize        int                `json:"uiFontSize,omitempty"`
	BackgroundBlur    bool               `json:"backgroundBlur,omitempty"`
	AutoHideOnBlur    bool               `json:"autoHideOnBlur,omitempty"`
	FixedPosition     bool               `json:"fixedPosition"`
	LanguageDetection string             `json:"languageDetection,omitempty"`
	AutoCheckUpdates  bool               `json:"autoCheckUpdates"`
	CompactLookup     bool               `json:"compactLookup,omitempty"`
	ShowSelectionIcon bool               `json:"showSelectionIcon,omitempty"`
	AutoTranslate     bool               `json:"autoTranslate,omitempty"`
	SelectInputText   bool               `json:"selectInputText"`
	DefaultAction     string             `json:"defaultAction,omitempty"`
	DefaultTarget     string             `json:"defaultTarget,omitempty"`
	DisplayShortcut   string             `json:"displayShortcut,omitempty"`
	OCRShortcut       string             `json:"ocrShortcut,omitempty"`
	QuickShortcut     string             `json:"quickShortcut,omitempty"`
	Proxy             ProxySettings      `json:"proxy"`
	Speech            SpeechSettings     `json:"speech"`
	Writing           WritingSettings    `json:"writing"`
}

type ProxySettings struct {
	Enabled  bool   `json:"enabled"`
	Protocol string `json:"protocol"`
	Server   string `json:"server"`
	Port     string `json:"port"`
	Username string `json:"username,omitempty"`
	NoProxy  string `json:"noProxy"`
}

type SpeechSettings struct {
	Provider      string        `json:"provider"`
	Rate          float64       `json:"rate"`
	Volume        float64       `json:"volume"`
	ReadSelection bool          `json:"readSelection"`
	Voices        []SpeechVoice `json:"voices,omitempty"`
}

type SpeechVoice struct {
	Provider string `json:"provider,omitempty"`
	Language string `json:"language"`
	Name     string `json:"name"`
}

type WritingSettings struct {
	Target          string `json:"target"`
	Shortcut        string `json:"shortcut"`
	NewlineShortcut string `json:"newlineShortcut"`
}

func (s Settings) Clone() Settings {
	s.Profiles = maps.Clone(s.Profiles)
	for name, profile := range s.Profiles {
		profile.ModelThinking = maps.Clone(profile.ModelThinking)
		s.Profiles[name] = profile
	}
	s.Speech.Voices = slices.Clone(s.Speech.Voices)
	return s
}

type Action struct {
	ID            string `json:"id"`
	Mode          string `json:"mode,omitempty"`
	Name          string `json:"name"`
	Prompt        string `json:"prompt"`
	CommandPrompt string `json:"commandPrompt,omitempty"`
	ArticlePrompt string `json:"articlePrompt,omitempty"`
	Provider      string `json:"provider,omitempty"`
	Model         string `json:"model,omitempty"`
	Format        string `json:"format,omitempty"`
	Thinking      bool   `json:"thinking,omitempty"`
	ThinkingLevel string `json:"thinkingLevel,omitempty"`
}

type Record struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	Result    string    `json:"result"`
	Source    string    `json:"source"`
	Target    string    `json:"target"`
	Action    string    `json:"action"`
	Provider  string    `json:"provider"`
	Model     string    `json:"model"`
	Favorite  bool      `json:"favorite"`
	Format    string    `json:"format,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type Word struct {
	Text        string    `json:"text"`
	Description string    `json:"description"`
	Reviews     int       `json:"reviews"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Data struct {
	Settings   Settings `json:"settings"`
	Actions    []Action `json:"actions"`
	History    []Record `json:"history"`
	Vocabulary []Word   `json:"vocabulary"`
}

type Provider struct{ Name, Endpoint, Model, Protocol string }

var Providers = []Provider{
	{"OpenAI", "https://api.openai.com/v1/chat/completions", "gpt-4o-mini", "openai"},
	{"Azure", "", "", "azure"},
	{"Claude", "https://api.anthropic.com/v1/messages", "claude-sonnet-4-5", "claude"},
	{"Gemini", "https://generativelanguage.googleapis.com/v1beta", "gemini-2.5-flash", "gemini"},
	{"DeepSeek", "https://api.deepseek.com/chat/completions", "deepseek-chat", "openai"},
	{"Ollama", "http://localhost:11434/v1/chat/completions", "qwen3:8b", "openai"},
	{"OpenRouter", "https://openrouter.ai/api/v1/chat/completions", "openai/gpt-4o-mini", "openai"},
	{"LiteLLM", "http://localhost:4000/v1/chat/completions", "gpt-4o-mini", "openai"},
	{"Groq", "https://api.groq.com/openai/v1/chat/completions", "llama-3.3-70b-versatile", "openai"},
	{"Cerebras", "https://api.cerebras.ai/v1/chat/completions", "gpt-oss-120b", "openai"},
	{"Moonshot", "https://api.moonshot.cn/v1/chat/completions", "moonshot-v1-8k", "openai"},
	{"Kimi", "https://api.moonshot.cn/v1/chat/completions", "kimi-k2-0905-preview", "openai"},
	{"MiniMax", "https://api.minimax.io/v1/chat/completions", "MiniMax-M2", "openai"},
	{"ChatGLM", "https://open.bigmodel.cn/api/paas/v4/chat/completions", "glm-4-flash", "openai"},
	{"Cohere", "https://api.cohere.com/v2/chat", "command-a-03-2025", "cohere"},
	{"TeamoRouter", "https://api.teamorouter.com/v1/chat/completions", "gpt-4o-mini", "openai"},
}

func FindProvider(name string) (Provider, bool) {
	for _, p := range Providers {
		if p.Name == name {
			return p, true
		}
	}
	return Provider{}, false
}

var Languages = []string{"简体中文", "繁體中文", "English", "日本語", "한국어", "Français", "Deutsch", "Español", "Italiano", "Português", "Русский", "العربية", "हिन्दी", "বাংলা", "Nederlands", "Polski", "Türkçe", "Українська", "Tiếng Việt", "ไทย", "Bahasa Indonesia", "Bahasa Melayu", "עברית", "Ελληνικά", "Čeština", "Dansk", "Suomi", "Svenska", "Norsk", "Română", "Magyar", "Български", "Hrvatski", "Slovenčina", "Slovenščina", "Српски", "Català", "Euskara", "Galego", "فارسی", "اردو", "தமிழ்", "తెలుగు", "मराठी", "ગુજરાતી", "ಕನ್ನಡ", "മലയാളം", "ਪੰਜਾਬੀ", "नेपाली", "සිංහල", "မြန်မာ", "ខ្មែរ", "ລາວ", "ქართული", "Հայերեն", "Azərbaycanca", "Қазақша", "Монгол", "Kiswahili", "Afrikaans", "Íslenska", "Gaeilge", "Cymraeg", "Esperanto", "Latina", "粵語", "文言文"}

func DefaultData() Data {
	profiles := make(map[string]Profile)
	for _, p := range Providers {
		profiles[p.Name] = Profile{Provider: p.Name, Endpoint: p.Endpoint, Model: p.Model}
	}
	return Data{
		Settings: Settings{
			UILanguage: "zh-Hans", Provider: "OpenAI", Profiles: profiles,
			Source: "自动检测", Target: "简体中文", DefaultTarget: "简体中文",
			Shortcut: "CmdOrCtrl+Shift+T", Appearance: "跟随系统", DefaultAction: "翻译",
			FixedPosition: true, LanguageDetection: "本地", AutoCheckUpdates: true, SelectInputText: true,
			Proxy:   ProxySettings{Protocol: "HTTP", Server: "127.0.0.1", Port: "1080", NoProxy: "localhost,127.0.0.1"},
			Speech:  SpeechSettings{Provider: "系统默认", Rate: 1, Volume: 100},
			Writing: WritingSettings{Target: "English", NewlineShortcut: "Shift+Enter"},
		},
		Actions: []Action{
			{ID: "translate", Mode: "translate", Name: "翻译"},
			{ID: "polishing", Mode: "polishing", Name: "润色"},
			{ID: "summarize", Mode: "summarize", Name: "总结"},
			{ID: "analyze", Mode: "analyze", Name: "分析"},
			{ID: "explain-code", Mode: "explain-code", Name: "解释代码"},
			{ID: "big-bang", Mode: "big-bang", Name: "单词助记", ArticlePrompt: "an insteresting story"},
		},
	}
}

// Only these exact old defaults are upgraded. User-edited prompts stay custom.
var legacyDefaultPrompts = map[string]string{
	"translate":    "Translate the text from {{source}} into {{target}}. Preserve meaning, formatting and tone. If the input is a single word, give its pronunciation, meanings, usage and example sentences. Output only the result.",
	"polishing":    "Polish the text in {{target}}. Improve clarity, grammar and naturalness while preserving its meaning. Output only the polished text.",
	"summarize":    "Summarize the text in {{target}}. Capture the key points faithfully and concisely.",
	"analyze":      "Analyze the grammar, vocabulary and structure of the text. Explain in {{target}}.",
	"explain-code": "Explain this code in {{target}}, including its purpose, flow and any important edge cases. Use Markdown.",
	"big-bang":     "Write a short memorable story in {{target}} using every supplied vocabulary word. Highlight each vocabulary word and explain it after the story.",
}

func validBuiltinMode(mode string) bool {
	switch mode {
	case "translate", "polishing", "summarize", "analyze", "explain-code", "big-bang":
		return true
	}
	return false
}
