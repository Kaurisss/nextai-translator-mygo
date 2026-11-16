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
		stop, err := desktop.WatchSelections(func(selection desktop.Selection) { a.selectionGesture(generation, selection) },
			func(point mygo.Point, mouse bool) { a.selectionDismiss(generation, point, mouse) })
		if err != nil {
			return err
		}
		a.stopSelectionWatch = stop
	} else if !enabled && a.stopSelectionWatch != nil {
		a.stopSelectionWatch()
		a.stopSelectionWatch = nil
		a.selectionWatchGeneration++
		a.clearSelection()
	}
	return nil
}

func (a *App) selectionDismiss(watchGeneration uint64, point mygo.Point, mouse bool) {
	a.update(func() {
		if a.stopSelectionWatch == nil || a.selectionWatchGeneration != watchGeneration ||
			mouse && (selectionPopupContains(a.selectionIcon, point) || selectionPopupContains(a.inlineWindow, point)) {
			return
		}
		a.clearSelection()
	})
}

func selectionPopupContains(window *mygo.Window, point mygo.Point) bool {
	if window == nil || !window.IsVisible() {
		return false
	}
	r := window.Bounds()
	return point.X >= r.X && point.X < r.X+r.Width && point.Y >= r.Y && point.Y < r.Y+r.Height
}

func (a *App) clearSelection() {
	a.selectionGeneration++
	if a.selectionCancel != nil {
		a.selectionCancel()
		a.selectionCancel = nil
	}
	a.selectedText, a.selectedWindow = "", 0
	if a.selectionIcon != nil {
		a.selectionIcon.Hide()
	}
	if a.inlineFromSelection {
		a.closeInlineLookup()
	}
}

func (a *App) selectionGesture(watchGeneration uint64, selection desktop.Selection) {
	a.update(func() {
		if a.stopSelectionWatch == nil || a.selectionWatchGeneration != watchGeneration || a.busy || a.systemBusy || a.Window.IsFocused() ||
			a.inlineWindow != nil && a.inlineWindow.IsFocused() || a.selectionIcon != nil && a.selectionIcon.IsFocused() {
			return
		}
		a.clearSelection()
		allowInput := a.Data.Settings.SelectInputText
		generation := a.selectionGeneration
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		a.selectionCancel = cancel
		go func() {
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
				if action := a.selectionAction("translate"); a.Data.Settings.AutoTranslate && action != "" {
					a.runSelectionAction(action)
				} else if a.Data.Settings.ShowSelectionIcon {
					a.showSelectionIcon(selection.Position, generation)
				}
			})
		}()
	})
}

func (a *App) showSelectionIcon(point mygo.Point, generation uint64) {
	if a.selectionIcon == nil {
		a.selectionIcon = mygo.NewWindow(mygo.WindowOptions{Title: "NextAI Translator", Width: 144, Height: 40, Hidden: true, Frameless: true, DisableResize: true, AlwaysOnTop: true, SkipTaskbar: true, Content: ui.View(a.selectionToolbar)})
	}
	area := mygo.Screen.DisplayNearestPoint(point).WorkArea
	width, height := a.selectionIcon.Size()
	position := positionNearPointer(mygo.Point{X: point.X + 8, Y: point.Y + 8}, area, width, height)
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

func (a *App) selectionToolbar(c *ui.Context) {
	a.applyTheme(c)
	t := c.Theme()
	ui.Row(c).Fill().Center().Gap(2).Background(t.Background).Border(1, t.Border).Role(ui.RoleToolbar).Children(func() {
		for _, button := range [...]struct{ id, label string }{
			{"translate", "翻译"}, {"polishing", "润色"}, {"summarize", "总结"}, {"analyze", "分析"},
		} {
			action := a.selectionAction(button.id)
			if iconButton(c, button.id, a.tr(button.label)).Size(32, 32).Disabled(action == "" || a.busy || a.systemBusy).Clicked() {
				a.runSelectionAction(action)
			}
		}
	})
}

func (a *App) selectionAction(id string) string {
	for _, action := range a.Data.Actions {
		if action.ID == id {
			return action.Name
		}
	}
	return ""
}

func (a *App) runSelectionAction(action string) {
	if action == "" || a.busy || a.systemBusy || a.selectedWindow == 0 {
		return
	}
	if a.selectionIcon != nil {
		a.selectionIcon.Hide()
	}
	a.text = a.selectedText
	a.previousWindow = a.selectedWindow
	a.page = "translator"
	a.action = action
	a.startLookup()
	a.inlineFromSelection = a.inlineGeneration != 0
}
