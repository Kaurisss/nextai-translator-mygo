package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Kaurisss/nextai-translator-mygo/internal/core"
	"github.com/Kaurisss/nextai-translator-mygo/internal/desktop"
	"github.com/Kaurisss/nextai-translator-mygo/resources"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

type Streamer interface {
	Stream(context.Context, core.Query, func(string)) error
}

type loginItem interface {
	OpenAtLogin() bool
	SetOpenAtLogin(bool) error
}

type App struct {
	Data                                                core.Data
	Store                                               core.Store
	Window                                              *mygo.Window
	Engine                                              Streamer
	page, text, result, action, search, notice, failure string
	profile                                             core.Profile
	apiKey                                              string
	keys                                                map[string]string
	credentials                                         credentialStore
	login                                               loginItem
	startAtLogin, savedStartAtLogin                     bool
	keyLoaded                                           map[string]bool
	keyLoading                                          map[string]bool
	busy, speaking, systemBusy, pinned                  bool
	editorPercent                                       float32
	logo                                                *ui.Bitmap
	md                                                  markdown.Options
	cancel                                              context.CancelFunc
	speechCancel                                        context.CancelFunc
	speechGeneration                                    uint64
	generation                                          uint64
	previousWindow                                      uintptr
	editAction                                          core.Action
	actionID                                            string
	resultFormat                                        string
	post                                                func(func())
	tray                                                *mygo.Tray
	quitting                                            bool
	favoritesOnly                                       bool
	advancedAction                                      bool
	appliedAppearance                                   string
	latestVersion                                       string
	settingsDraft                                       core.Settings
	settingsTab                                         int
	client                                              *http.Client
	proxyPassword                                       string
	pendingKeys                                         map[string]string
	modelOptions                                        map[string][]string
	modelFetched                                        map[string]bool
	modelsBusy, proxyBusy, voicesBusy                   bool
	voices                                              []desktop.Voice
	voiceLanguage                                       string
	proxyStatus                                         string
	registered                                          map[string]string
	recordingShortcut                                   string
	suppressShortcuts                                   bool
	selectedSpeech                                      string
	selectionSpeechGeneration                           uint64
	writing                                             bool
	quickWindow                                         *mygo.Window
	inlineWindow                                        *mygo.Window
	inlineGeneration                                    uint64
	selectionIcon                                       *mygo.Window
	stopSelectionWatch                                  func()
	selectionGeneration                                 uint64
	selectionWatchGeneration                            uint64
	selectedText                                        string
	selectedWindow                                      uintptr
	showAPIKey, showProxyPassword                       bool
	speechStatus                                        string
}

func New(d core.Data, s core.Store) *App {
	if d.Settings.DefaultTarget == "" {
		d.Settings.DefaultTarget = d.Settings.Target
	}
	d.Settings.Target = d.Settings.DefaultTarget
	client, clientErr := core.HTTPClient(d.Settings.Proxy, "")
	a := &App{Data: d, Store: s, Engine: core.Engine{Client: client}, client: client, login: mygo.App, settingsDraft: d.Settings.Clone(), page: "translator", action: d.Actions[0].Name, editorPercent: 40, keys: map[string]string{}, keyLoaded: map[string]bool{}, keyLoading: map[string]bool{}, pendingKeys: map[string]string{}, modelOptions: map[string][]string{}, modelFetched: map[string]bool{}, registered: map[string]string{}, voiceLanguage: "English"}
	if clientErr != nil {
		a.failure = clientErr.Error()
	}
	for _, v := range d.Actions {
		if v.Name == d.Settings.DefaultAction {
			a.action = v.Name
		}
	}
	var err error
	a.logo, err = ui.DecodeBitmap(resources.Icon)
	if err != nil {
		panic(err)
	}
	a.profile = d.Settings.Profiles[d.Settings.Provider]
	a.credentials = mygo.Secrets.Service("xyz.nextai.translator.native")
	a.md.Reveal = &markdown.Reveal{Unit: markdown.ByWord, Fade: -1}
	a.editAction = d.Actions[0]
	a.actionID = d.Actions[0].Name
	return a
}

func (a *App) update(fn func()) {
	a.post(func() {
		fn()
		if a.quickWindow != nil {
			a.quickWindow.Update(func() {})
		}
		if a.inlineWindow != nil {
			a.inlineWindow.Update(func() {})
		}
	})
}

func (a *App) save() bool {
	if err := a.Store.Save(a.Data); err != nil {
		a.failure = "保存失败: " + err.Error()
		return false
	}
	return true
}

func (a *App) loadKey(provider string) {
	if a.keyLoaded[provider] {
		if a.profile.Provider == provider {
			a.apiKey = a.keys[provider]
		}
		return
	}
	if a.profile.Provider == provider {
		a.apiKey = ""
	}
	if a.keyLoading[provider] {
		return
	}
	a.keyLoading[provider] = true
	go func() {
		b, err := a.credentials.Get(provider)
		a.update(func() {
			a.keyLoading[provider] = false
			if err != nil && !errors.Is(err, mygo.ErrSecretNotFound) && !errors.Is(err, errors.ErrUnsupported) {
				a.failure = "读取 API Key: " + err.Error()
				return
			}
			a.keys[provider] = string(b)
			a.keyLoaded[provider] = true
			if provider == "proxy-password" {
				a.proxyPassword = string(b)
				client, err := core.HTTPClient(a.Data.Settings.Proxy, string(b))
				if err != nil {
					a.failure = err.Error()
					return
				}
				a.client = client
				a.Engine = core.Engine{Client: client}
				if a.Data.Settings.AutoCheckUpdates {
					a.checkForUpdates()
				}
			}
			if a.profile.Provider == provider {
				a.apiKey = string(b)
				if pending, ok := a.pendingKeys[provider]; ok {
					a.apiKey = pending
				}
				a.maybeRefreshModels()
			}
		})
	}()
}

func (a *App) start() {
	if a.busy || a.systemBusy {
		return
	}
	a.hideInlineLookup()
	if a.client == nil {
		a.failure = "请先修正并保存代理设置"
		return
	}
	if a.keyLoading["proxy-password"] {
		a.notice = "正在读取代理凭据"
		return
	}
	if a.keyLoading[a.Data.Settings.Provider] {
		a.notice = "正在读取 API Key"
		return
	}
	if strings.TrimSpace(a.text) == "" {
		a.failure = "请输入要处理的文本"
		return
	}
	var action core.Action
	for _, v := range a.Data.Actions {
		if v.Name == a.action {
			action = v
			break
		}
	}
	if action.ID == "" {
		a.failure = "请选择有效动作"
		return
	}
	provider := a.Data.Settings.Provider
	if action.Provider != "" && action.Provider != "跟随全局" {
		provider = action.Provider
	}
	if provider != a.Data.Settings.Provider && !a.keyLoaded[provider] {
		a.loadKey(provider)
		a.notice = "正在读取动作的 API Key，读取完成后可开始处理"
		return
	}
	profile := a.Data.Settings.Profiles[provider]
	if action.Model != "" {
		profile.Model = action.Model
	}
	q := core.Query{Profile: profile, Key: a.keys[provider], Text: a.text, Source: a.Data.Settings.Source, Target: a.Data.Settings.Target, Action: action, LanguageDetection: a.Data.Settings.LanguageDetection}
	writing, targetWindow, newline := a.writing, a.previousWindow, a.Data.Settings.Writing.NewlineShortcut
	if writing {
		q.Writing = true
		q.Action = core.Action{ID: "translate", Mode: "translate", Name: "翻译"}
		q.Target = a.Data.Settings.Writing.Target
		q.Source = "自动检测"
	}
	a.resultFormat = action.Format
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.generation++
	id := a.generation
	a.busy = true
	a.failure = ""
	a.notice = ""
	a.result = ""
	source := a.Data.Settings.Source
	go func() {
		defer cancel()
		var full strings.Builder
		err := a.Engine.Stream(ctx, q, func(delta string) {
			full.WriteString(delta)
			a.update(func() {
				if a.generation == id {
					a.result += delta
				}
			})
		})
		result := full.String()
		a.update(func() {
			if a.generation != id {
				return
			}
			a.busy = false
			a.cancel = nil
			a.writing = false
			if err != nil {
				if !errors.Is(err, context.Canceled) {
					a.failure = err.Error()
					if writing && a.Window != nil {
						a.Window.Show()
					}
				}
				return
			}
			a.result = result
			a.Data.History = append([]core.Record{{ID: fmt.Sprint(time.Now().UnixNano()), Text: q.Text, Result: result, Source: source, Target: q.Target, Action: q.Action.Name, Provider: q.Profile.Provider, Model: q.Profile.Model, Format: q.Action.Format, CreatedAt: time.Now()}}, a.Data.History...)
			if a.save() {
				a.notice = "已完成并保存到历史记录"
			}
			if writing {
				a.finishWriting(q.Text, result, targetWindow, newline)
			}
		})
	}()
}

func (a *App) stop() {
	a.hideInlineLookup()
	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}
	a.generation++
	a.busy = false
	a.writing = false
	a.notice = "已停止"
}

func (a *App) addWord(c *ui.Context) {
	word := strings.TrimSpace(a.text)
	if word == "" || a.result == "" {
		return
	}
	for _, w := range a.Data.Vocabulary {
		if w.Text == word {
			c.Toast(a.tr("生词本已包含此条目"))
			return
		}
	}
	a.Data.Vocabulary = append([]core.Word{{Text: word, Description: a.result, CreatedAt: time.Now()}}, a.Data.Vocabulary...)
	if a.save() {
		c.Toast(a.tr("已加入生词本"))
	}
}

func (a *App) restore(r core.Record) {
	a.stop()
	a.notice = ""
	a.failure = ""
	a.text = r.Text
	a.result = r.Result
	a.resultFormat = r.Format
	a.Data.Settings.Source = r.Source
	a.Data.Settings.Target = r.Target
	for _, v := range a.Data.Actions {
		if v.Name == r.Action {
			a.action = r.Action
		}
	}
	a.page = "translator"
}
