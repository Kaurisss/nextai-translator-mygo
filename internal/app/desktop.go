package app

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Kaurisss/nextai-translator-mygo/internal/core"
	"github.com/Kaurisss/nextai-translator-mygo/internal/desktop"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func (a *App) Open(windowIcon, trayIcon []byte) {
	a.startAtLogin = a.login.OpenAtLogin()
	a.savedStartAtLogin = a.startAtLogin
	startedAtLogin := mygo.App.WasOpenedAtLogin()
	options := mygo.WindowOptions{Title: "NextAI Translator", Width: 560, Height: 900, MinWidth: 500, MinHeight: 560, StateKey: "translator-stacked", TitleBarStyle: mygo.TitleBarHidden, TitleBarHeight: 32, AutoHideMenuBar: true, Hidden: startedAtLogin, Content: ui.View(a.View)}
	if desktop.SupportsBackgroundBlur() {
		options.Transparent = true
		options.Vibrancy = backgroundVibrancy(a.Data.Settings.BackgroundBlur)
	}
	a.Window = mygo.NewWindow(options)
	if err := a.Window.SetIcon(windowIcon); err != nil {
		a.failure = "设置窗口图标: " + err.Error()
	}
	a.post = a.Window.Update
	a.loadKey(a.Data.Settings.Provider)
	if err := a.registerShortcuts(a.Data.Settings); err != nil {
		a.failure = err.Error()
	}
	a.loadKey("proxy-password")
	a.Window.OnClosed(func() { a.stop(); a.stopSpeaking(); a.setSelectionWatch(false); mygo.GlobalShortcut.UnregisterAll() })
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { a.quitting = true })
	a.Window.OnClose(func(e *mygo.CloseEvent) {
		if a.tray != nil && !a.quitting {
			e.PreventDefault()
			a.Window.Hide()
		}
	})
	a.Window.OnBlur(func() {
		if a.Data.Settings.AutoHideOnBlur && a.page == "translator" && !a.pinned && !a.systemBusy && a.tray != nil {
			a.Window.Hide()
		}
	})
	mygo.App.OnActivate(func(visible bool) {
		if !visible {
			a.Window.Show()
			a.Window.Focus()
		}
	})
	mygo.App.OnSecondInstance(func([]string, string) { a.Window.Show(); a.Window.Focus() })
	trayMenu := a.newTrayMenu()
	trayOptions := mygo.TrayOptions{Icon: trayIcon, ToolTip: "NextAI Translator"}
	if runtime.GOOS == "linux" {
		trayOptions.Menu = trayMenu
	}
	tray, err := mygo.NewTray(trayOptions)
	if err != nil {
		a.failure = "创建托盘: " + err.Error()
		if startedAtLogin {
			a.Window.Show()
		}
	} else {
		a.tray = tray
		a.applyTaskbarIcon(a.Data.Settings.HideTaskbarIcon)
		tray.OnClick(func() { a.Window.Show(); a.Window.Focus() })
		if runtime.GOOS != "linux" {
			tray.OnRightClick(func() {
				tray.SetMenu(a.newTrayMenu())
				tray.PopUpMenu()
				tray.SetMenu(nil)
			})
		} else {
			a.refreshTray()
		}
	}
	if a.Data.Settings.ShowSelectionIcon || a.Data.Settings.AutoTranslate {
		enableSelectionWatch := func() {
			if a.quitting || a.stopSelectionWatch != nil {
				return
			}
			if err := a.setSelectionWatch(true); err != nil {
				a.failure = "启用划词: " + err.Error()
			}
		}
		if runtime.GOOS == "windows" && startedAtLogin {
			time.AfterFunc(time.Second, func() {
				a.update(enableSelectionWatch)
			})
		} else {
			enableSelectionWatch()
		}
	}
}

func backgroundVibrancy(enabled bool) mygo.Vibrancy {
	if !enabled {
		return mygo.VibrancyNone
	}
	if runtime.GOOS == "windows" {
		return mygo.VibrancyMica
	}
	return mygo.VibrancyAcrylic
}

func (a *App) openSettings() {
	if a.startAtLogin == a.savedStartAtLogin {
		a.startAtLogin = a.login.OpenAtLogin()
		a.savedStartAtLogin = a.startAtLogin
	}
	a.page = "settings"
	a.maybeRefreshModels()
}

func (a *App) applyTaskbarIcon(hidden bool) {
	if runtime.GOOS == "darwin" {
		if hidden {
			mygo.App.Dock.Hide()
		} else {
			mygo.App.Dock.Show()
		}
	} else {
		a.Window.SetSkipTaskbar(hidden)
	}
}

func (a *App) showAtPointer() {
	if !a.Data.Settings.FixedPosition && !a.Window.IsMaximized() && !a.Window.IsFullScreen() {
		point := mygo.Screen.CursorScreenPoint()
		area := mygo.Screen.DisplayNearestPoint(point).WorkArea
		width, height := a.Window.Size()
		position := positionNearPointer(point, area, width, height)
		a.Window.SetPosition(position.X, position.Y)
	}
	a.Window.Show()
	a.Window.Focus()
}

func positionNearPointer(point mygo.Point, area mygo.Rectangle, width, height int) mygo.Point {
	return mygo.Point{
		X: max(area.X, min(point.X, area.X+area.Width-width)),
		Y: max(area.Y, min(point.Y, area.Y+area.Height-height)),
	}
}

func (a *App) Menu() *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{{Role: mygo.RoleAppMenu}, {Label: a.tr("文件"), Submenu: []*mygo.MenuItem{{Role: mygo.RoleClose}, {Role: mygo.RoleQuit}}}, {Role: mygo.RoleEditMenu}, {Role: mygo.RoleWindowMenu}})
}

func (a *App) newTrayMenu() *mygo.Menu {
	items := []*mygo.MenuItem{
		{Label: a.tr("打开翻译"), Click: func(*mygo.MenuItem, *mygo.Window) { a.Window.Show(); a.Window.Focus() }},
		{Label: a.tr("翻译剪贴板"), Click: func(*mygo.MenuItem, *mygo.Window) {
			a.text = mygo.Clipboard.ReadText()
			a.page = "translator"
			a.Window.Show()
			a.Window.Focus()
			a.start()
		}},
		{ID: "ocr", Label: a.tr("截图翻译"), Disabled: runtime.GOOS != "linux" && (a.busy || a.systemBusy),
			Click: func(*mygo.MenuItem, *mygo.Window) { a.screenshot() }},
		{ID: "pin", Label: a.tr("置顶"), Type: mygo.MenuItemCheckbox, Checked: a.pinned,
			Click: func(*mygo.MenuItem, *mygo.Window) { a.togglePinned() }},
	}
	if label := a.updateLabel(); label != "" {
		items = append(items, &mygo.MenuItem{ID: "update", Label: label, Disabled: a.updating, Click: func(*mygo.MenuItem, *mygo.Window) {
			if a.Window != nil {
				a.Window.Show()
				a.Window.Focus()
			}
			a.installLatest()
		}})
	}
	items = append(items, mygo.Separator(), &mygo.MenuItem{Role: mygo.RoleQuit})
	return mygo.NewMenu(items)
}

func (a *App) togglePinned() {
	a.pinned = !a.pinned
	if a.Window != nil {
		a.Window.SetAlwaysOnTop(a.pinned)
	}
	a.refreshTray()
}

func (a *App) refreshTray() {
	if runtime.GOOS == "linux" && a.tray != nil {
		a.tray.SetMenu(a.newTrayMenu())
	}
}

func (a *App) saveSettings() {
	a.settingsDraft.Profiles[a.profile.Provider] = a.profile
	a.settingsDraft.Provider = a.profile.Provider
	settings := a.settingsDraft.Clone()
	settings.Source = a.Data.Settings.Source
	settings.Target = a.Data.Settings.Target
	if settings.Writing.NewlineShortcut != "" {
		if _, _, err := desktop.ParseChord(settings.Writing.NewlineShortcut); err != nil {
			a.failure = err.Error()
			return
		}
	}
	if err := core.ValidateSettings(settings); err != nil {
		a.failure = err.Error()
		return
	}
	if settings.HideTaskbarIcon && a.Window != nil && a.tray == nil {
		a.failure = "系统托盘不可用，无法隐藏任务栏图标"
		return
	}
	client, err := core.HTTPClient(settings.Proxy, a.proxyPassword)
	if err != nil {
		a.failure = err.Error()
		return
	}
	if err := a.registerShortcuts(settings); err != nil {
		a.failure = err.Error()
		return
	}
	a.pendingKeys[a.profile.Provider] = a.apiKey
	startAtLogin := a.startAtLogin
	startAtLoginEdited := startAtLogin != a.savedStartAtLogin
	keys := maps.Clone(a.pendingKeys)
	keys["proxy-password"] = a.proxyPassword
	a.systemBusy = true
	secrets := a.credentials
	go func() {
		previousCredentials, credentialErr := updateCredentials(secrets, keys)
		a.update(func() {
			if credentialErr != nil && !errors.Is(credentialErr, errCredentialsUnavailable) {
				a.systemBusy = false
				a.failure = "保存凭据: " + credentialErr.Error()
				if rollback := a.registerShortcuts(a.Data.Settings); rollback != nil {
					a.failure += "；" + rollback.Error()
				}
				return
			}
			next := a.Data
			next.Settings = settings
			previousStartAtLogin := a.login.OpenAtLogin()
			previousSelectionWatch := a.stopSelectionWatch != nil
			var saveErr error
			if a.Window != nil && previousSelectionWatch != (settings.ShowSelectionIcon || settings.AutoTranslate) {
				if err := a.setSelectionWatch(settings.ShowSelectionIcon || settings.AutoTranslate); err != nil {
					saveErr = fmt.Errorf("启用划词: %w", err)
				}
			}
			if saveErr == nil && startAtLoginEdited && startAtLogin != previousStartAtLogin {
				if err := a.login.SetOpenAtLogin(startAtLogin); err != nil {
					saveErr = fmt.Errorf("设置开机启动: %w", err)
				}
			}
			if saveErr == nil {
				if err := a.Store.Save(next); err != nil {
					saveErr = fmt.Errorf("保存设置: %w", err)
				}
			}
			if saveErr != nil {
				a.failure = saveErr.Error()
				if previousSelectionWatch != (a.stopSelectionWatch != nil) {
					if rollback := a.setSelectionWatch(previousSelectionWatch); rollback != nil {
						a.failure += "；恢复划词: " + rollback.Error()
					}
				}
				if a.login.OpenAtLogin() != previousStartAtLogin {
					if rollback := a.login.SetOpenAtLogin(previousStartAtLogin); rollback != nil {
						a.failure += "；恢复开机启动: " + rollback.Error()
					}
				}
				if rollback := a.registerShortcuts(a.Data.Settings); rollback != nil {
					a.failure += "；" + rollback.Error()
				}
				go func() {
					rollbackErr := writeCredentials(secrets, previousCredentials)
					a.update(func() {
						a.systemBusy = false
						if rollbackErr != nil {
							a.failure += "；恢复凭据: " + rollbackErr.Error()
						}
					})
				}()
				return
			}
			checkUpdates := settings.AutoCheckUpdates && !a.Data.Settings.AutoCheckUpdates
			a.Data.Settings = settings
			a.savedStartAtLogin = a.login.OpenAtLogin()
			if a.startAtLogin == startAtLogin {
				a.startAtLogin = a.savedStartAtLogin
			}
			if a.Window != nil && a.tray != nil {
				a.applyTaskbarIcon(settings.HideTaskbarIcon)
			}
			if !settings.ShowSelectionIcon && a.selectionIcon != nil {
				a.selectionIcon.Hide()
			}
			if a.Window != nil && desktop.SupportsBackgroundBlur() {
				a.Window.SetVibrancy(backgroundVibrancy(settings.BackgroundBlur))
			}
			if settings.UILanguage != "" {
				mygo.App.SetLocale(settings.UILanguage)
			}
			mygo.App.SetMenu(a.Menu())
			for provider, key := range keys {
				a.keys[provider] = key
				a.keyLoaded[provider] = true
			}
			clear(a.pendingKeys)
			if old := a.client; old != nil {
				old.CloseIdleConnections()
			}
			a.client = client
			a.Engine = core.Engine{Client: client}
			if !settings.AutoCheckUpdates {
				if !a.updating {
					a.latestVersion = ""
					a.latestAsset, a.latestAssetOK = releaseAsset{}, false
				}
			} else if checkUpdates {
				a.checkForUpdates()
			}
			a.refreshTray()
			a.systemBusy = false
			a.failure = ""
			a.notice = "设置已保存"
			if errors.Is(credentialErr, errCredentialsUnavailable) {
				a.notice = "设置已保存；系统凭据库不可用，凭据仅保留到应用退出"
			}
		})
	}()
}

func (a *App) selection() {
	if a.systemBusy || a.busy {
		return
	}
	a.previousWindow = desktop.ActiveWindow()
	a.systemBusy = true
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		err := desktop.CopySelection(ctx)
		text := ""
		if err == nil {
			text = mygo.Clipboard.ReadText()
		}
		a.update(func() {
			a.systemBusy = false
			a.page = "translator"
			if err != nil {
				a.showAtPointer()
				a.failure = err.Error()
				return
			}
			a.text = text
			a.startLookup()
		})
	}()
}

func (a *App) insert() {
	target, text := a.previousWindow, a.result
	mygo.Clipboard.WriteText(text)
	a.systemBusy = true
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		err := desktop.Paste(ctx, target)
		a.update(func() {
			a.systemBusy = false
			if err != nil {
				a.failure = err.Error()
			} else {
				a.notice = "已写回原输入窗口"
			}
		})
	}()
}

func (a *App) speak() {
	a.speakText(a.result, a.Data.Settings.Target, a.Data.Settings.Speech)
}

func (a *App) speakText(text, language string, options core.SpeechSettings) {
	if options.Provider != "系统默认" && a.client == nil {
		a.failure = "请先修正并保存代理设置"
		return
	}
	a.stopSpeaking()
	ctx, cancel := context.WithCancel(context.Background())
	a.speechCancel = cancel
	a.speechGeneration++
	id := a.speechGeneration
	a.speaking = true
	a.speechStatus = ""
	voice := desktop.SpeechVoice(options, core.LanguageCode(language, text))
	client := a.client
	root := filepath.Join(filepath.Dir(a.Store.Path), "tts")
	go func() {
		defer cancel()
		var err error
		if options.Provider == "本地 TTS (MeloTTS / Kokoro)" {
			err = desktop.LocalSpeak(ctx, client, root, text, core.LanguageCode(language, text), options, voice, func(message string) {
				a.update(func() {
					if a.speechGeneration == id {
						a.speechStatus = message
					}
				})
			})
		} else {
			err = desktop.Speak(ctx, text, options, voice)
		}
		wasCanceled := ctx.Err() != nil
		a.update(func() {
			if a.speechGeneration != id {
				return
			}
			a.speaking = false
			a.speechStatus = ""
			a.speechCancel = nil
			if err != nil && !wasCanceled {
				a.failure = "朗读失败: " + err.Error()
			}
		})
	}()
}

func (a *App) stopSpeaking() {
	if a.speechCancel != nil {
		a.speechCancel()
		a.speechCancel = nil
	}
	a.speaking = false
	a.speechStatus = ""
	a.speechGeneration++
}

func (a *App) importImage() {
	a.systemBusy = true
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.Window, Title: "选择要识别的图片", Filters: []mygo.FileFilter{{Name: "图片", Extensions: []string{"png", "jpg", "jpeg", "bmp"}}}})
		if err != nil || len(paths) == 0 {
			a.update(func() {
				a.systemBusy = false
				if err != nil {
					a.failure = err.Error()
				}
			})
			return
		}
		a.recognize(paths[0])
	}()
}

func (a *App) dropImage(paths []string) {
	for _, path := range paths {
		switch strings.ToLower(filepath.Ext(path)) {
		case ".png", ".jpg", ".jpeg", ".bmp", ".gif", ".webp", ".tif", ".tiff":
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				a.systemBusy = true
				a.failure = ""
				go a.recognize(path)
				return
			}
		}
	}
	a.failure = "请拖放 PNG、JPG、BMP、GIF、WebP 或 TIFF 图片"
}

func (a *App) screenshot() {
	if a.busy || a.systemBusy || a.Window == nil {
		return
	}
	a.systemBusy = true
	a.failure = ""
	windows := []*mygo.Window{a.Window, a.inlineWindow, a.quickWindow, a.selectionIcon}
	go func() {
		dir, err := os.MkdirTemp("", "nextai-capture-")
		if err != nil {
			a.update(func() { a.systemBusy = false; a.Window.Show(); a.failure = err.Error() })
			return
		}
		defer os.RemoveAll(dir)
		path := filepath.Join(dir, "capture.png")
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if err := desktop.Capture(ctx, path, windows...); err != nil {
			a.update(func() {
				a.systemBusy = false
				a.Window.Show()
				a.Window.Focus()
				if !errors.Is(err, context.Canceled) {
					a.failure = err.Error()
				}
			})
			return
		}
		a.recognize(path)
	}()
}

// recognize runs on the OCR worker and only returns state through Window.Update.
func (a *App) recognize(path string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	text, err := desktop.OCR(ctx, path)
	a.update(func() {
		a.systemBusy = false
		if a.Window != nil {
			a.Window.Show()
			a.Window.Focus()
		}
		if err != nil {
			a.failure = "识别失败: " + err.Error()
			return
		}
		if strings.TrimSpace(text) == "" {
			a.failure = "图片中未识别到文字"
			return
		}
		a.text = text
		a.page = "translator"
		a.start()
	})
}

func (a *App) export(kind string) {
	// Snapshot before opening a dialog; the worker never reads mutable app data.
	var b bytes.Buffer
	ext := ".csv"
	if kind == "backup" {
		encoded, err := json.MarshalIndent(a.Data, "", "  ")
		if err != nil {
			a.failure = err.Error()
			return
		}
		b.Write(encoded)
		ext = ".json"
	} else {
		b.WriteString("\ufeff")
		w := csv.NewWriter(&b)
		if kind == "history" {
			w.Write([]string{"原文", "译文", "源语言", "目标语言", "动作", "服务", "模型", "时间"})
			for _, r := range a.Data.History {
				w.Write([]string{csvText(r.Text), csvText(r.Result), r.Source, r.Target, r.Action, r.Provider, r.Model, r.CreatedAt.Format(time.RFC3339)})
			}
		} else {
			w.Write([]string{"单词", "释义", "复习次数"})
			for _, r := range a.Data.Vocabulary {
				w.Write([]string{csvText(r.Text), csvText(r.Description), fmt.Sprint(r.Reviews)})
			}
		}
		w.Flush()
		if err := w.Error(); err != nil {
			a.failure = err.Error()
			return
		}
	}
	payload := append([]byte(nil), b.Bytes()...)
	go func() {
		path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: a.Window, Title: "导出", DefaultPath: "nextai-" + kind + ext})
		if err == nil && path != "" {
			err = os.WriteFile(path, payload, 0600)
		}
		a.update(func() {
			if err != nil {
				a.failure = err.Error()
			} else if path != "" {
				a.notice = "已导出到 " + path
			}
		})
	}()
}

func csvText(s string) string {
	if strings.HasPrefix(s, "=") || strings.HasPrefix(s, "+") || strings.HasPrefix(s, "-") || strings.HasPrefix(s, "@") || strings.HasPrefix(s, "\t") || strings.HasPrefix(s, "\r") {
		return "'" + s
	}
	return s
}

func (a *App) importBackup() {
	a.systemBusy = true
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.Window, Title: "合并原生版备份", Filters: []mygo.FileFilter{{Name: "NextAI 备份", Extensions: []string{"json"}}}})
		var d core.Data
		if err == nil && len(paths) > 0 {
			d, err = (core.Store{Path: paths[0]}).Load()
		}
		a.update(func() {
			a.systemBusy = false
			if err != nil {
				a.failure = err.Error()
				return
			}
			if len(paths) == 0 {
				return
			}
			seen := map[string]bool{}
			for _, r := range a.Data.History {
				seen[r.ID] = true
			}
			for _, r := range d.History {
				if !seen[r.ID] {
					a.Data.History = append(a.Data.History, r)
					seen[r.ID] = true
				}
			}
			words := map[string]bool{}
			for _, w := range a.Data.Vocabulary {
				words[w.Text] = true
			}
			for _, w := range d.Vocabulary {
				if !words[w.Text] {
					a.Data.Vocabulary = append(a.Data.Vocabulary, w)
					words[w.Text] = true
				}
			}
			if a.save() {
				a.notice = "已合并历史记录和生词本"
			}
		})
	}()
}
