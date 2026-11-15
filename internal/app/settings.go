package app

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/Kaurisss/nextai-translator-mygo/internal/core"
	"github.com/Kaurisss/nextai-translator-mygo/internal/desktop"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func (a *App) settings(c *ui.Context) {
	t := c.Theme()
	header := ui.Column(c).WidthPercent(100).BorderWidth(0, 0, 1, 0).BorderColor(t.Border)
	if t.Background.R < 128 {
		header.LinearGradient(ui.LinearGradient{
			Angle: 125,
			Oklab: true,
			Stops: []ui.GradientStop{
				{Color: t.Background, At: 0},
				{Color: ui.RGB(48, 32, 46), At: 0.34},
				{Color: ui.RGB(24, 48, 66), At: 0.56},
				{Color: ui.RGB(36, 38, 64), At: 0.78},
				{Color: t.Background, At: 1},
			},
		})
	} else {
		header.LinearGradient(ui.LinearGradient{
			Angle: 125,
			Oklab: true,
			Stops: []ui.GradientStop{
				{Color: t.Background, At: 0},
				{Color: ui.RGB(250, 235, 245), At: 0.34},
				{Color: ui.RGB(212, 240, 253), At: 0.56},
				{Color: ui.RGB(228, 231, 252), At: 0.78},
				{Color: t.Background, At: 1},
			},
		})
	}
	header.Children(func() {
		ui.Row(c).Padding(t.Space(3), t.Space(5), t.Space(2)).Gap(t.Space(2.5)).AlignItems(ui.Center).Children(func() {
			ui.Image(c, a.logo).Size(t.Space(6), t.Space(6)).Radius(t.Space(1.5))
			ui.Text(c, "NextAI Translator").FontSize(t.Rem(1.3)).Bold()
			version := mygo.App.Version()
			if version == "" {
				version = "dev"
			} else {
				version = "v" + strings.TrimPrefix(version, "v")
			}
			ui.Link(c, version, "https://github.com/Kaurisss/nextai-translator-mygo/releases").FontSize(max(12, t.Rem(12.0/14)))
		})
		tabs := ui.TabsBase(c, &a.settingsTab, 5)
		tabs.List.Label(a.tr("设置分类")).WidthPercent(100).Children(func() {
			for i, tab := range []struct{ name, icon string }{{"通用", "settings"}, {"代理", "proxy"}, {"TTS", "speak"}, {"写作", "insert"}, {"快捷键", "keyboard"}} {
				border, textColor := ui.Transparent, t.TextMuted
				if a.settingsTab == i {
					border, textColor = t.Text, t.Text
				}
				tabEl := tabs.Tab(i).Label(a.tr(tab.name)).Tooltip(a.tr(tab.name)).WidthPercent(20).MinWidth(0).Row().AlignItems(ui.Center).Justify(ui.Center).Gap(t.Space(1.5)).Padding(t.Space(2.5), t.Space(1)).BorderWidth(0, 0, 2, 0).BorderColor(border).TextColor(textColor)
				if tabEl.Hovered() && a.settingsTab != i {
					tabEl.Background(t.Text.Alpha(0.05))
				}
				tabEl.Children(func() {
					ui.Icon(c, icons[tab.icon]).Size(t.Space(4), t.Space(4))
					ui.Text(c, a.tr(tab.name)).FontSize(max(12, t.Rem(12.0/14))).SingleLine().Shrink(1)
				})
			}
		})
	})
	c.Key(a.settingsTab)
	ui.Scroll(c).Grow(1).WidthPercent(100).Label(a.tr("设置内容")).Children(func() {
		ui.Column(c).WidthPercent(100).Padding(t.Space(4), t.Space(5), t.Space(5)).Gap(t.Space(5)).Children(func() {
			switch a.settingsTab {
			case 0:
				a.generalSettings(c)
			case 1:
				a.proxySettings(c)
			case 2:
				a.speechSettings(c)
			case 3:
				a.writingSettings(c)
			case 4:
				a.shortcutSettings(c)
			}
		})
	})
	a.errorNotice(c)
	ui.Toolbar(c, func() {
		if iconTextButton(c, "back", a.tr("返回"), a.tr("返回翻译"), false).Clicked() {
			a.recordingShortcut = ""
			a.page = "translator"
		}
		ui.Spacer(c)
		if iconTextButton(c, "save", a.tr("保存"), a.tr("保存设置"), true).Disabled(a.busy || a.systemBusy || a.keyLoading[a.profile.Provider] || a.keyLoading["proxy-password"]).Clicked() {
			a.saveSettings()
		}
	}).Label(a.tr("设置操作")).WidthPercent(100).Padding(t.Space(2), t.Space(4)).BorderWidth(1, 0, 0, 0).BorderColor(t.Border)
}

func (a *App) generalSettings(c *ui.Context) {
	s := &a.settingsDraft
	settingsField(c, a.tr("语言"), "", func() ui.Element {
		language := interfaceLanguages[0]
		for i, code := range interfaceCodes {
			if s.UILanguage == code {
				language = interfaceLanguages[i]
			}
		}
		control := ui.Select(c, &language, interfaceLanguages).Label(a.tr("语言"))
		if control.Changed() {
			s.UILanguage = interfaceCodes[slices.Index(interfaceLanguages, language)]
		}
		return control
	})
	settingsField(c, a.tr("主题"), "", func() ui.Element {
		return a.selectSetting(c, &s.Appearance, []string{"跟随系统", "浅色", "深色"}).Label(a.tr("主题"))
	})
	ui.Checkbox(c, &a.startAtLogin, a.tr("开机启动")).Label(a.tr("开机启动"))
	hideIcon := "隐藏任务栏中的图标"
	if runtime.GOOS == "darwin" {
		hideIcon = "隐藏 Dock 栏中的图标"
	}
	ui.Checkbox(c, &s.HideTaskbarIcon, a.tr(hideIcon)).Label(a.tr(hideIcon))
	settingsField(c, a.tr("默认服务提供商"), "", func() ui.Element {
		var providers []string
		for _, p := range core.Providers {
			providers = append(providers, p.Name)
		}
		selected := a.profile.Provider
		control := a.selectSetting(c, &selected, providers).Label(a.tr("模型服务"))
		if control.Changed() {
			s.Profiles[a.profile.Provider] = a.profile
			if !a.keyLoading[a.profile.Provider] {
				a.pendingKeys[a.profile.Provider] = a.apiKey
			}
			a.profile = s.Profiles[selected]
			a.loadKey(selected)
			if key, ok := a.pendingKeys[selected]; ok {
				a.apiKey = key
			}
			a.maybeRefreshModels()
		}
		return control
	})
	settingsField(c, a.tr("API 密钥"), a.tr("保存在系统凭据库中；多个密钥用英文逗号分隔"), func() ui.Element {
		return ui.Row(c).Gap(c.Theme().Space(2)).Children(func() {
			input := ui.TextInput(c, &a.apiKey).Label("API Key").Grow(1).Disabled(a.keyLoading[a.profile.Provider] || a.systemBusy)
			if !a.showAPIKey {
				input.Password()
			}
			eyeIcon := "eye"
			if a.showAPIKey {
				eyeIcon = "eye-off"
			}
			if iconTextButton(c, eyeIcon, a.tr("显示"), a.tr("显示或隐藏 API Key"), false).Clicked() {
				a.showAPIKey = !a.showAPIKey
			}
		})
	})
	settingsField(c, a.tr("无模型 API 支持"), a.tr("启用后手动填写模型，不请求模型列表"), func() ui.Element {
		return ui.Checkbox(c, &a.profile.NoModelsAPI, a.tr("启用")).Label(a.tr("无模型 API 支持"))
	})
	settingsField(c, a.tr("API 模型"), a.tr("可从列表选择，也可输入自定义模型名称"), func() ui.Element {
		return ui.Row(c).Gap(c.Theme().Space(2)).Children(func() {
			options := a.modelOptions[a.profile.Endpoint]
			if len(options) == 0 {
				if p, ok := core.FindProvider(a.profile.Provider); ok && p.Model != "" {
					options = []string{p.Model}
				}
			}
			a.modelPicker(c, &a.profile.Model, options, a.tr("模型名称"))
			if iconTextButton(c, "refresh", a.tr("刷新"), a.tr("刷新模型列表"), false).Disabled(a.modelsBusy || a.profile.NoModelsAPI || a.keyLoading[a.profile.Provider]).Clicked() {
				a.refreshModels(false)
			}
		})
	})
	base, path := core.SplitEndpoint(a.profile.Endpoint)
	settingsField(c, a.tr("API URL"), "", func() ui.Element {
		input := ui.TextInput(c, &base).Label(a.tr("API 地址"))
		if input.Changed() {
			a.profile.Endpoint = core.JoinEndpoint(base, path)
			a.profile.ModelThinking = nil
		}
		return input
	})
	hint := "例如 /v1/chat/completions"
	if a.profile.Provider == "Gemini" {
		hint = "Gemini 使用 /v1beta 基础路径"
	}
	if a.profile.Provider == "Azure" {
		hint = "填写部署路径和 api-version 查询参数"
	}
	settingsField(c, a.tr("API URL 路径"), a.tr(hint), func() ui.Element {
		input := ui.TextInput(c, &path).Label(a.tr("API URL 路径"))
		if input.Changed() {
			a.profile.Endpoint = core.JoinEndpoint(base, path)
			a.profile.ModelThinking = nil
		}
		return input
	})
	if a.profile.Provider == "Azure" {
		settingsField(c, a.tr("最大输出 Token"), "", func() ui.Element {
			value := float64(a.profile.MaxTokens)
			if value == 0 {
				value = 1024
			}
			input := ui.NumberInput(c, &value, 1, 1000000, 1).Label(a.tr("最大输出 Token"))
			if input.Changed() {
				a.profile.MaxTokens = int(value)
			}
			return input
		})
	}
	if a.profile.Provider == "Ollama" {
		settingsField(c, a.tr("模型驻留时间"), a.tr("例如 5m、0 或 -1"), func() ui.Element {
			return ui.TextInput(c, &a.profile.KeepAlive).Placeholder("5m").Label(a.tr("模型驻留时间"))
		})
	}
	settingsField(c, a.tr("深度思考"), a.tr("需使用支持推理参数的模型"), func() ui.Element {
		return ui.Checkbox(c, &a.profile.Thinking, a.tr("启用")).Label(a.tr("全局深度思考"))
	})
	if a.profile.Thinking {
		settingsField(c, a.tr("思考强度"), "", func() ui.Element {
			return a.selectSetting(c, &a.profile.ThinkingLevel, []string{"low", "medium", "high"}).Placeholder("medium").Label(a.tr("全局思考强度"))
		})
	}
	settingsField(c, a.tr("默认动作"), "", func() ui.Element {
		return a.selectSetting(c, &s.DefaultAction, a.actionNames()).Label(a.tr("默认动作"))
	})
	settingsField(c, a.tr("默认目标语言"), "", func() ui.Element {
		return a.languagePicker(c, &s.DefaultTarget, core.Languages, a.tr("默认目标语言"))
	})
	settingsField(c, a.tr("语种检测引擎"), a.tr("在线检测会将文本发送给所选服务"), func() ui.Element {
		return a.selectSetting(c, &s.LanguageDetection, []string{"本地", "百度", "谷歌", "必应"}).Label(a.tr("语种检测引擎"))
	})
	settingsField(c, a.tr("字体大小"), "", func() ui.Element {
		value := float64(s.FontSize)
		if value == 0 {
			value = float64(c.Theme().FontSize)
		}
		input := ui.NumberInput(c, &value, 12, 40, 1).Label(a.tr("字体大小"))
		if input.Changed() {
			s.FontSize = int(value)
		}
		return input
	})
	settingsField(c, a.tr("UI 字体大小"), "", func() ui.Element {
		value := float64(s.UIFontSize)
		if value == 0 {
			value = float64(c.Theme().FontSize)
		}
		input := ui.NumberInput(c, &value, 12, 24, 1).Label(a.tr("UI 字体大小"))
		if input.Changed() {
			s.UIFontSize = int(value)
		}
		return input
	})
	if desktop.SupportsBackgroundBlur() {
		ui.Checkbox(c, &s.BackgroundBlur, a.tr("窗口背景模糊")).Label(a.tr("窗口背景模糊"))
	}
	ui.Checkbox(c, &s.AutoHideOnBlur, a.tr("失去焦点时自动隐藏窗口")).Label(a.tr("失去焦点时自动隐藏窗口"))
	ui.Checkbox(c, &s.FixedPosition, a.tr("固定位置")).Label(a.tr("固定位置"))
	if runtime.GOOS != "linux" {
		ui.Checkbox(c, &s.ShowSelectionIcon, a.tr("选中文字时显示图标")).Label(a.tr("选中文字时显示图标"))
		ui.Checkbox(c, &s.AutoTranslate, a.tr("自动翻译")).Label(a.tr("自动翻译"))
		ui.Checkbox(c, &s.SelectInputText, a.tr("输入框划词")).Label(a.tr("输入框划词")).Disabled(!s.ShowSelectionIcon && !s.AutoTranslate)
	}
	ui.Checkbox(c, &s.CompactLookup, a.tr("紧凑划词模式")).Label(a.tr("紧凑划词模式"))
	ui.Checkbox(c, &s.AutoCheckUpdates, a.tr("自动检查更新")).Label(a.tr("自动检查更新"))
	ui.Toolbar(c, func() {
		if iconTextButton(c, "export", a.tr("导出备份"), "", false).Disabled(a.systemBusy).Clicked() {
			a.export("backup")
		}
		if iconTextButton(c, "import", a.tr("导入备份"), "", false).Disabled(a.systemBusy).Clicked() {
			a.importBackup()
		}
	}).Label(a.tr("备份操作"))
}

func (a *App) proxySettings(c *ui.Context) {
	p := &a.settingsDraft.Proxy
	settingsField(c, a.tr("启用"), a.tr("关闭时使用系统代理与代理环境变量"), func() ui.Element {
		return ui.Checkbox(c, &p.Enabled, a.tr("使用自定义代理")).Label(a.tr("启用代理"))
	})
	settingsField(c, a.tr("协议"), "", func() ui.Element {
		return a.selectSetting(c, &p.Protocol, []string{"HTTP", "HTTPS", "SOCKS5"}).Label(a.tr("代理协议"))
	})
	settingsField(c, a.tr("服务器"), "", func() ui.Element { return ui.TextInput(c, &p.Server).Label(a.tr("代理服务器")) })
	settingsField(c, a.tr("端口"), "", func() ui.Element { return ui.TextInput(c, &p.Port).Label(a.tr("代理端口")) })
	settingsField(c, a.tr("用户名"), "", func() ui.Element { return ui.TextInput(c, &p.Username).Label(a.tr("代理用户名")) })
	settingsField(c, a.tr("密码"), a.tr("保存在系统凭据库中"), func() ui.Element {
		return ui.Row(c).Gap(c.Theme().Space(2)).Children(func() {
			input := ui.TextInput(c, &a.proxyPassword).Grow(1).Label(a.tr("代理密码")).Disabled(a.keyLoading["proxy-password"])
			if !a.showProxyPassword {
				input.Password()
			}
			eyeIcon := "eye"
			if a.showProxyPassword {
				eyeIcon = "eye-off"
			}
			if iconTextButton(c, eyeIcon, a.tr("显示"), a.tr("显示或隐藏代理密码"), false).Clicked() {
				a.showProxyPassword = !a.showProxyPassword
			}
		})
	})
	settingsField(c, a.tr("无代理"), a.tr("使用英文逗号分隔域名、IP 或 CIDR"), func() ui.Element { return ui.TextArea(c, &p.NoProxy).Label(a.tr("无代理")).Lines(2, 4) })
	if iconTextButton(c, "test", a.tr("测试"), a.tr("测试代理"), false).Disabled(a.proxyBusy).Clicked() {
		a.testProxy()
	}
	if a.proxyBusy {
		ui.Spinner(c)
	}
	if a.proxyStatus != "" {
		ui.Text(c, a.proxyStatus).Role(ui.RoleStatus)
	}
}

func (a *App) speechSettings(c *ui.Context) {
	s := &a.settingsDraft.Speech
	settingsField(c, a.tr("朗读输入框中选中的单词"), "", func() ui.Element {
		return ui.Checkbox(c, &s.ReadSelection, a.tr("启用")).Label(a.tr("朗读选中单词"))
	})
	settingsField(c, a.tr("提供者"), "", func() ui.Element {
		control := a.selectSetting(c, &s.Provider, []string{"系统默认", "本地 TTS (MeloTTS / Kokoro)"}).Label(a.tr("TTS 提供者"))
		if control.Changed() && s.Provider != "系统默认" {
			a.voiceLanguage = "English"
		}
		return control
	})
	settingsField(c, a.tr("速率"), fmt.Sprintf("%.2f×", s.Rate), func() ui.Element { return ui.Slider(c, &s.Rate, 0.5, 2).Label(a.tr("语速")) })
	settingsField(c, a.tr("音量"), fmt.Sprintf("%.0f%%", s.Volume), func() ui.Element { return ui.Slider(c, &s.Volume, 0, 100).Label(a.tr("音量")) })
	if s.Provider == "系统默认" {
		if iconTextButton(c, "refresh", a.tr("刷新系统音色"), "", false).Disabled(a.voicesBusy).Clicked() {
			a.refreshVoices()
		}
		if a.voicesBusy {
			ui.Spinner(c)
		}
	}
	remove := -1
	for i := range s.Voices {
		provider := s.Voices[i].Provider
		if provider == "" {
			provider = "系统默认"
		}
		if provider != s.Provider {
			continue
		}
		c.Key(s.Voices[i].Language)
		v := &s.Voices[i]
		settingsField(c, v.Language, "", func() ui.Element {
			return ui.Row(c).Gap(c.Theme().Space(2)).Children(func() {
				var options []string
				if s.Provider == "系统默认" {
					options = desktop.VoiceLanguages(a.voices, core.LanguageCode(v.Language, ""))
				} else if core.LanguageCode(v.Language, "") == "en" {
					options = []string{"Kokoro English"}
				} else {
					options = []string{"MeloTTS Chinese + English"}
				}
				a.selectSetting(c, &v.Name, options).Label(v.Language + " " + a.tr("音色")).Grow(1)
				if iconTextButton(c, "speak", a.tr("试听"), a.tr("试听")+v.Language, false).Disabled(a.speaking).Clicked() {
					a.previewSpeech(v.Language)
				}
				if iconButton(c, "clear", a.tr("删除")+v.Language+a.tr("音色")).Clicked() {
					remove = i
				}
			})
		})
	}
	if remove >= 0 {
		s.Voices = slices.Delete(s.Voices, remove, remove+1)
	}
	languages := core.Languages
	if s.Provider != "系统默认" {
		languages = []string{"English", "简体中文", "繁體中文", "粵語", "文言文"}
	}
	ui.Row(c).Gap(c.Theme().Space(2)).Children(func() {
		a.languagePicker(c, &a.voiceLanguage, languages, a.tr("音色语言")).Grow(1)
		if iconTextButton(c, "add", a.tr("添加"), a.tr("添加音色"), false).Clicked() {
			if !slices.ContainsFunc(s.Voices, func(v core.SpeechVoice) bool {
				provider := v.Provider
				if provider == "" {
					provider = "系统默认"
				}
				return v.Language == a.voiceLanguage && provider == s.Provider
			}) {
				name := "系统默认"
				if s.Provider != "系统默认" {
					name = "MeloTTS Chinese + English"
					if core.LanguageCode(a.voiceLanguage, "") == "en" {
						name = "Kokoro English"
					}
				}
				s.Voices = append(s.Voices, core.SpeechVoice{Provider: s.Provider, Language: a.voiceLanguage, Name: name})
			}
		}
	})
	ui.Toolbar(c, func() {
		if iconTextButton(c, "speak", a.tr("试听"), a.tr("试听 TTS"), false).Disabled(a.speaking).Clicked() {
			a.previewSpeech(a.voiceLanguage)
		}
		if iconTextButton(c, "stop", a.tr("停止"), a.tr("停止试听"), false).Disabled(!a.speaking).Clicked() {
			a.stopSpeaking()
		}
	}).Label(a.tr("语音试听"))
	if a.speechStatus != "" {
		ui.Text(c, a.speechStatus).Role(ui.RoleStatus)
	}
}

func (a *App) previewSpeech(language string) {
	text := "This is a speech preview."
	if code := core.LanguageCode(language, ""); code == "zh-Hans" || code == "zh-Hant" || code == "yue" || code == "lzh" {
		text = "你好，这是语音试听。"
	}
	a.speakText(text, language, a.settingsDraft.Speech)
}

func (a *App) writingSettings(c *ui.Context) {
	s := &a.settingsDraft.Writing
	settingsField(c, a.tr("写作目标语言"), "", func() ui.Element { return a.languagePicker(c, &s.Target, core.Languages, a.tr("写作目标语言")) })
	a.shortcutField(c, "写作快捷键", "在其他应用的输入框中按下，将现有文本翻译并写回", &s.Shortcut)
	a.shortcutField(c, "写作换行快捷键", "写入多行结果时使用的按键，例如 Shift+Enter", &s.NewlineShortcut)
}

func (a *App) shortcutSettings(c *ui.Context) {
	s := &a.settingsDraft
	a.shortcutField(c, "取词快捷键", "翻译其他应用中选中的文本", &s.Shortcut)
	a.shortcutField(c, "显示窗口快捷键", "", &s.DisplayShortcut)
	a.shortcutField(c, "OCR 快捷键", "选择屏幕区域，识别并翻译文字", &s.OCRShortcut)
	a.shortcutField(c, "快捷翻译快捷键", "读取指针位置的文字，在不抢焦点的窗口中显示译文", &s.QuickShortcut)
}

func (a *App) shortcutField(c *ui.Context, label, description string, value *string) {
	settingsField(c, a.tr(label), a.tr(description), func() ui.Element {
		return ui.Row(c).Gap(c.Theme().Space(2)).Children(func() {
			text := *value
			if text == "" {
				text = "点击设置"
			}
			if a.recordingShortcut == label {
				text = "请按快捷键，Esc 取消"
			}
			button := ui.Button(c, a.tr(text)).Label(a.tr(label)).Grow(1).HandleInput(func(ev ui.InputEvent) bool {
				if a.recordingShortcut != label || ev.Kind != ui.InputKeyDown {
					return false
				}
				a.suppressShortcuts = true
				if ev.Key == ui.KeyEscape {
					a.recordingShortcut = ""
					return true
				}
				if key := shortcutName(ev); key != "" {
					*value = key
					a.recordingShortcut = ""
				}
				return true
			})
			if button.Clicked() {
				a.recordingShortcut = label
			}
			if a.recordingShortcut == label && !button.Focused() {
				a.recordingShortcut = ""
			}
			if iconButton(c, "clear", a.tr("清除")+a.tr(label)).Clicked() {
				*value = ""
				a.recordingShortcut = ""
			}
		})
	})
}

func (a *App) modelPicker(c *ui.Context, value *string, options []string, label string) ui.Element {
	t := c.Theme()
	c.Key(label)
	field := ui.Row(c).MinWidth(t.Space(22)).Grow(1).Height(t.Space(8)).AlignItems(ui.Center).
		Padding(0, t.Space(2), 0, t.Space(2.5)).Gap(t.Space(1)).Radius(t.Radius).Background(t.Surface).Border(1, t.Border)
	field.Children(func() {
		cb := ui.ComboboxBase(c, value)
		cb.Input.Label(label).AutoComplete(ui.AutoCompleteList).MinWidth(0).Grow(1).
			Height(t.Space(7.5)).Padding(t.Space(1.5), 0).FocusRing(false)
		arrow := ui.ButtonBase(c).Label(a.tr("展开")+label).Tooltip(a.tr("选择")+label).TabStop(false).KeepFocus().Size(t.Space(5), t.Space(7)).Padding(0).Center().Children(func() {
			ui.Icon(c, icons["chevron"]).Size(t.Space(4), t.Space(4)).TextColor(t.TextMuted)
		})
		if cb.Input.Focused() {
			field.Border(1, t.Focus)
		}
		if arrow.Clicked() {
			cb.SetOpen(!cb.Open())
			cb.Input.Focus()
		}
		shown := options
		if cb.Filtering() {
			shown = nil
			query := strings.ToLower(strings.TrimSpace(*value))
			for _, option := range options {
				if strings.Contains(strings.ToLower(option), query) {
					shown = append(shown, option)
				}
			}
		}
		cb.Popup(func(panel ui.Element) {
			if len(shown) == 0 {
				return
			}
			panel.AttachTo(field, ui.AnchorBottomStart, ui.AnchorTopStart).MinWidth(field.Bounds().W).
				Padding(t.Space(1)).Background(t.Background).Border(1, t.Border).Radius(t.Radius)
			ui.Scroll(c).MaxHeight(t.Space(70)).Children(func() {
				for _, option := range shown {
					item := cb.Item(option).Padding(t.Space(1.5), t.Space(2.5)).Radius(t.Radius)
					if item.Highlighted() {
						item.Background(t.Accent).TextColor(t.AccentText)
					}
					item.Children(func() { ui.Text(c, option).SingleLine() })
				}
			})
		})
		if choice, ok := cb.Chosen(); ok {
			*value = choice
		}
	})
	return field
}

func (a *App) maybeRefreshModels() {
	if a.post == nil || a.profile.NoModelsAPI || a.modelsBusy || a.keyLoading[a.profile.Provider] || a.modelFetched[a.profile.Endpoint] || len(a.modelOptions[a.profile.Endpoint]) > 0 {
		return
	}
	if a.apiKey == "" && a.profile.Provider != "Ollama" && a.profile.Provider != "LiteLLM" && !strings.Contains(a.profile.Endpoint, "127.0.0.1") && !strings.Contains(a.profile.Endpoint, "localhost") {
		return
	}
	a.refreshModels(true)
}

func (a *App) refreshModels(silent bool) {
	client, err := core.HTTPClient(a.settingsDraft.Proxy, a.proxyPassword)
	if err != nil {
		if !silent {
			a.failure = err.Error()
		}
		return
	}
	profile, key := a.profile, a.apiKey
	a.modelsBusy = true
	a.modelFetched[profile.Endpoint] = true
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		defer client.CloseIdleConnections()
		models, err := core.ListModels(ctx, client, profile, key)
		a.update(func() {
			a.modelsBusy = false
			if err != nil {
				if !silent {
					a.failure = err.Error()
				}
			} else {
				a.modelOptions[profile.Endpoint] = models.Names
				if a.profile.Provider == profile.Provider && a.profile.Endpoint == profile.Endpoint {
					a.profile.ModelThinking = models.Thinking
				}
				if draft := a.settingsDraft.Profiles[profile.Provider]; draft.Endpoint == profile.Endpoint {
					draft.ModelThinking = models.Thinking
					a.settingsDraft.Profiles[profile.Provider] = draft
				}
				if !silent {
					a.notice = "模型列表已更新"
				}
			}
		})
	}()
}

func (a *App) testProxy() {
	client, err := core.HTTPClient(a.settingsDraft.Proxy, a.proxyPassword)
	if err != nil {
		a.proxyStatus = err.Error()
		return
	}
	address := a.profile.Endpoint
	a.proxyBusy = true
	a.proxyStatus = ""
	go func() {
		defer client.CloseIdleConnections()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, "GET", address, nil)
		start := time.Now()
		status := ""
		if err == nil {
			var response *http.Response
			response, err = client.Do(req)
			if err == nil {
				response.Body.Close()
				status = fmt.Sprintf("连接成功，HTTP %d，耗时 %d ms", response.StatusCode, time.Since(start).Milliseconds())
			}
		}
		if err != nil {
			status = "连接失败：" + err.Error()
		}
		a.update(func() { a.proxyBusy = false; a.proxyStatus = status })
	}()
}

func (a *App) refreshVoices() {
	a.voicesBusy = true
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		voices, err := desktop.SystemVoices(ctx)
		a.update(func() {
			a.voicesBusy = false
			if err != nil {
				a.failure = err.Error()
			} else {
				a.voices = voices
			}
		})
	}()
}
