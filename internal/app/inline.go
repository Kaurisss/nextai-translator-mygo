package app

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

func (a *App) startLookup() {
	if a.busy || a.systemBusy {
		return
	}
	a.start()
	if a.Data.Settings.CompactLookup && a.busy {
		a.inlineGeneration = a.generation
		a.showInlineLookup()
		return
	}
	a.showAtPointer()
}

func (a *App) hideInlineLookup() {
	if a.inlineGeneration != 0 {
		a.inlineGeneration = 0
		if a.inlineWindow != nil {
			a.inlineWindow.Hide()
		}
	}
}

func (a *App) closeInlineLookup() {
	if a.busy && a.inlineGeneration == a.generation && a.inlineGeneration != 0 {
		a.stop()
	}
	a.hideInlineLookup()
}

func (a *App) showInlineLookup() {
	if a.inlineWindow == nil {
		a.inlineWindow = mygo.NewWindow(mygo.WindowOptions{Title: "NextAI Translator", Width: 420, Height: 240, MinWidth: 300, MinHeight: 160, Hidden: true, Frameless: true, AlwaysOnTop: true, SkipTaskbar: true, Content: ui.View(a.inlineView)})
		a.inlineWindow.OnClose(func(e *mygo.CloseEvent) {
			if !a.quitting {
				e.PreventDefault()
				a.closeInlineLookup()
			}
		})
	}
	point := mygo.Screen.CursorScreenPoint()
	area := mygo.Screen.DisplayNearestPoint(point).WorkArea
	width, height := a.inlineWindow.Size()
	position := positionNearPointer(point, area, width, height)
	a.inlineWindow.SetPosition(position.X, position.Y)
	a.inlineWindow.ShowInactive()
}

func (a *App) inlineView(c *ui.Context) {
	if a.inlineGeneration == 0 || a.inlineGeneration != a.generation {
		return
	}
	a.applyTheme(c)
	t := c.Theme()
	a.md.Theme = &markdown.Theme{FontSize: float32(a.Data.Settings.FontSize)}
	ui.Column(c).Fill().Padding(t.Space(3)).Gap(t.Space(2)).Background(t.Background).Border(1, t.Border).Children(func() {
		ui.Row(c).AlignItems(ui.Center).DragWindow().Children(func() {
			ui.Text(c, "NextAI Translator").Bold()
			ui.Spacer(c)
			if iconButton(c, "copy", a.tr("复制")).Disabled(a.result == "").Clicked() {
				c.WriteClipboard(a.result)
			}
			if iconButton(c, "clear", a.tr("关闭")).Clicked() {
				a.closeInlineLookup()
			}
		})
		ui.Scroll(c).Grow(1).WidthPercent(100).Children(func() {
			if a.resultFormat == "纯文本" {
				result := ui.Text(c, a.result).Selectable()
				if a.Data.Settings.FontSize > 0 {
					result.FontSize(float32(a.Data.Settings.FontSize))
				}
			} else if a.busy {
				markdown.Stream(c, markdownHardBreaks(a.result), &a.md)
			} else {
				markdown.View(c, markdownHardBreaks(a.result), &a.md)
			}
		})
		if a.busy && a.result == "" {
			ui.Spinner(c)
		}
		if a.failure != "" {
			ui.Text(c, a.failure).TextColor(t.Danger).Role(ui.RoleAlert)
		}
		if ui.Button(c, a.tr("在主窗口打开")).Clicked() {
			a.hideInlineLookup()
			a.showAtPointer()
		}
	})
}
