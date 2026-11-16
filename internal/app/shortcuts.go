package app

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/Kaurisss/nextai-translator-mygo/internal/core"
	"github.com/Kaurisss/nextai-translator-mygo/internal/desktop"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func (a *App) registerShortcuts(settings core.Settings) error {
	wanted := map[string]string{}
	for name, key := range map[string]string{"取词": settings.Shortcut, "显示窗口": settings.DisplayShortcut, "OCR": settings.OCRShortcut, "快捷翻译": settings.QuickShortcut, "写作": settings.Writing.Shortcut} {
		if key != "" {
			wanted[key] = name
		}
	}
	if maps.Equal(wanted, a.registered) {
		return nil
	}
	callbacks := map[string]func(){"取词": a.selection, "显示窗口": a.showAtPointer, "OCR": a.screenshot, "快捷翻译": a.quickTranslate, "写作": a.startWriting}
	for name, callback := range callbacks {
		callbacks[name] = func() {
			if a.recordingShortcut == "" {
				callback()
			}
		}
	}
	previous := maps.Clone(a.registered)
	for key := range previous {
		mygo.GlobalShortcut.Unregister(key)
	}
	clear(a.registered)
	for key, name := range wanted {
		if err := mygo.GlobalShortcut.Register(key, callbacks[name]); err != nil {
			for active := range a.registered {
				mygo.GlobalShortcut.Unregister(active)
			}
			clear(a.registered)
			for old, action := range previous {
				if restoreErr := mygo.GlobalShortcut.Register(old, callbacks[action]); restoreErr != nil {
					return fmt.Errorf("注册%s失败: %w；恢复%s失败: %v", name, err, action, restoreErr)
				}
				a.registered[old] = action
			}
			return fmt.Errorf("注册%s快捷键 %s: %w", name, key, err)
		}
		a.registered[key] = name
	}
	return nil
}

func (a *App) startWriting() {
	if a.busy || a.systemBusy {
		return
	}
	target := desktop.ActiveWindow()
	if target == 0 {
		a.failure = "无法确定写作目标窗口"
		a.Window.Show()
		return
	}
	a.systemBusy = true
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		err := desktop.CopyAll(ctx)
		text := ""
		if err == nil {
			text = mygo.Clipboard.ReadText()
		}
		a.update(func() {
			a.systemBusy = false
			if err != nil {
				a.failure = err.Error()
				a.Window.Show()
				return
			}
			a.text = text
			a.previousWindow = target
			a.writing = true
			a.start()
			if !a.busy {
				a.writing = false
				a.Window.Show()
			}
		})
	}()
}

func (a *App) finishWriting(original, result string, target uintptr, newline string) {
	a.systemBusy = true
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var err error
		if desktop.ActiveWindow() != target {
			err = fmt.Errorf("写作目标窗口已切换，译文已保留，请手动写回")
		} else if err = desktop.CopyAll(ctx); err == nil {
			if mygo.Clipboard.ReadText() != original {
				err = fmt.Errorf("输入内容已更改，译文已保留，请手动写回")
			} else {
				err = desktop.WriteText(ctx, target, result, newline)
			}
		}
		a.update(func() {
			a.systemBusy = false
			if err != nil {
				a.failure = err.Error()
				a.Window.Show()
			} else {
				a.notice = "已写回翻译结果"
			}
		})
	}()
}

func (a *App) quickTranslate() {
	if a.busy || a.systemBusy {
		return
	}
	if a.tray == nil {
		a.failure = "快捷翻译需要系统托盘"
		a.Window.Show()
		return
	}
	point, err := desktop.PointerPosition()
	if err != nil {
		a.failure = err.Error()
		a.Window.Show()
		return
	}
	a.previousWindow = desktop.ActiveWindow()
	a.systemBusy = true
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		text, err := desktop.QuickText(ctx, point)
		a.update(func() {
			a.systemBusy = false
			if err != nil {
				a.failure = err.Error()
				a.Window.Show()
				return
			}
			a.text = strings.TrimSpace(text)
			if a.quickWindow == nil {
				a.quickWindow = mygo.NewFlyout(mygo.FlyoutOptions{Tray: a.tray, Width: 420, Height: 340, Hidden: true, Shadow: true, Focusable: false, Content: ui.View(a.quickView)})
				a.quickWindow.OnClose(func(e *mygo.CloseEvent) {
					if !a.quitting {
						e.PreventDefault()
						a.quickWindow.Hide()
					}
				})
			}
			a.quickWindow.ShowInactive()
			a.start()
		})
	}()
}

func (a *App) quickView(c *ui.Context) {
	a.applyTheme(c)
	t := c.Theme()
	ui.Column(c).Fill().Padding(t.Space(4), 0).Gap(t.Space(3)).Background(t.Background).Children(func() {
		ui.Row(c).Margin(0, t.Space(4)).DragWindow().Children(func() {
			ui.Text(c, a.tr("快捷翻译")).Bold()
			ui.Spacer(c)
			if iconButton(c, "clear", a.tr("关闭快捷翻译")).Clicked() {
				a.quickWindow.Hide()
			}
		})
		ui.Text(c, preview(a.text, 120)).Margin(0, t.Space(4)).TextColor(t.TextMuted)
		ui.Scroll(c).Grow(1).WidthPercent(100).Padding(0, t.Space(4)).Label(a.tr("翻译结果")).Children(func() { ui.Text(c, a.result).Selectable() })
		if a.busy {
			ui.Spinner(c).Margin(0, t.Space(4))
		}
		if a.failure != "" {
			ui.Text(c, a.failure).Margin(0, t.Space(4)).TextColor(t.Danger).Role(ui.RoleAlert)
		}
		if ui.Button(c, a.tr("在主窗口打开")).Margin(0, t.Space(4)).Clicked() {
			a.Window.Show()
			a.Window.Focus()
			a.page = "translator"
			a.quickWindow.Hide()
		}
	})
}

func shortcutName(ev ui.InputEvent) string {
	key := ""
	switch {
	case ev.Key >= ui.KeyA && ev.Key <= ui.KeyZ:
		key = string(rune('A' + ev.Key - ui.KeyA))
	case ev.Key >= ui.Key0 && ev.Key <= ui.Key9:
		key = string(rune('0' + ev.Key - ui.Key0))
	case ev.Key >= ui.KeyF1 && ev.Key <= ui.KeyF12:
		key = fmt.Sprintf("F%d", ev.Key-ui.KeyF1+1)
	default:
		key = map[ui.Key]string{ui.KeyEnter: "Enter", ui.KeyTab: "Tab", ui.KeySpace: "Space", ui.KeyBackspace: "Backspace", ui.KeyDelete: "Delete", ui.KeyLeft: "Left", ui.KeyRight: "Right", ui.KeyUp: "Up", ui.KeyDown: "Down", ui.KeyHome: "Home", ui.KeyEnd: "End"}[ev.Key]
	}
	if key == "" {
		return ""
	}
	var parts []string
	for _, mod := range []struct {
		flag ui.Modifiers
		name string
	}{{ui.Ctrl, "Ctrl"}, {ui.Super, "Super"}, {ui.Alt, "Alt"}, {ui.Shift, "Shift"}} {
		if ev.Mods&mod.flag != 0 {
			parts = append(parts, mod.name)
		}
	}
	return strings.Join(append(parts, key), "+")
}
