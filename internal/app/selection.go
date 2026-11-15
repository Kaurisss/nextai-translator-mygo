package app

import (
	"context"
	"strings"
	"time"

	"github.com/Kaurisss/nextai-translator-mygo/internal/desktop"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func (a *App) setSelectionWatch(enabled bool) error {
	if enabled && a.stopSelectionWatch == nil {
		a.selectionWatchGeneration++
		generation := a.selectionWatchGeneration
		stop, err := desktop.WatchSelections(func(selection desktop.Selection) { a.selectionGesture(generation, selection) })
		if err != nil {
			return err
		}
		a.stopSelectionWatch = stop
	} else if !enabled && a.stopSelectionWatch != nil {
		a.stopSelectionWatch()
		a.stopSelectionWatch = nil
		a.selectionWatchGeneration++
		a.selectionGeneration++
		if a.selectionIcon != nil {
			a.selectionIcon.Hide()
		}
	}
	return nil
}

func (a *App) selectionGesture(watchGeneration uint64, selection desktop.Selection) {
	a.update(func() {
		if a.stopSelectionWatch == nil || a.selectionWatchGeneration != watchGeneration || a.busy || a.systemBusy || a.Window.IsFocused() ||
			a.inlineWindow != nil && a.inlineWindow.IsFocused() || a.selectionIcon != nil && a.selectionIcon.IsFocused() {
			return
		}
		if a.selectionIcon != nil {
			a.selectionIcon.Hide()
		}
		a.selectionGeneration++
		allowInput := a.Data.Settings.SelectInputText
		generation := a.selectionGeneration
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			text, target, err := selection.Read(ctx, allowInput)
			a.update(func() {
				if a.selectionGeneration != generation || a.stopSelectionWatch == nil || a.Data.Settings.SelectInputText != allowInput || a.busy || a.systemBusy || a.Window.IsFocused() ||
					a.inlineWindow != nil && a.inlineWindow.IsFocused() || a.selectionIcon != nil && a.selectionIcon.IsFocused() {
					return
				}
				if err != nil {
					a.failure = "读取选中文本: " + err.Error()
					return
				}
				text = strings.TrimSpace(text)
				if text == "" || target == 0 {
					return
				}
				a.selectedText, a.selectedWindow = text, target
				if a.Data.Settings.AutoTranslate {
					a.translateSelection()
				} else if a.Data.Settings.ShowSelectionIcon {
					a.showSelectionIcon(selection.Position, generation)
				}
			})
		}()
	})
}

func (a *App) showSelectionIcon(point mygo.Point, generation uint64) {
	if a.selectionIcon == nil {
		a.selectionIcon = mygo.NewWindow(mygo.WindowOptions{Title: "NextAI Translator", Width: 36, Height: 36, Hidden: true, Frameless: true, DisableResize: true, AlwaysOnTop: true, SkipTaskbar: true, Content: ui.View(func(c *ui.Context) {
			a.applyTheme(c)
			ui.Box(c).Fill().Background(c.Theme().Background).Border(1, c.Theme().Border).Center().Children(func() {
				if iconButton(c, "translate", a.tr("翻译")).Clicked() {
					a.selectionIcon.Hide()
					a.translateSelection()
				}
			})
		})})
	}
	area := mygo.Screen.DisplayNearestPoint(point).WorkArea
	position := positionNearPointer(mygo.Point{X: point.X + 8, Y: point.Y + 8}, area, 36, 36)
	a.selectionIcon.SetPosition(position.X, position.Y)
	a.selectionIcon.ShowInactive()
	time.AfterFunc(5*time.Second, func() {
		a.update(func() {
			if a.selectionGeneration == generation && a.selectionIcon != nil {
				a.selectionIcon.Hide()
			}
		})
	})
}

func (a *App) translateSelection() {
	if a.busy || a.systemBusy {
		return
	}
	a.text = a.selectedText
	a.previousWindow = a.selectedWindow
	a.page = "translator"
	for _, action := range a.Data.Actions {
		if action.Mode == "translate" {
			a.action = action.Name
			break
		}
	}
	a.startLookup()
}
