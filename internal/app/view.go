package app

import (
	"fmt"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Kaurisss/nextai-translator-mygo/internal/core"
	"github.com/Kaurisss/nextai-translator-mygo/internal/desktop"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

func (a *App) View(c *ui.Context) {
	a.applyTheme(c)
	t := c.Theme()
	root := ui.Column(c).Fill().Background(t.Background)
	if a.Data.Settings.BackgroundBlur && c.Vibrancy() {
		c.Root().Background(ui.Transparent)
		root.Background(ui.Transparent)
	}
	if !a.busy && !a.systemBusy {
		if files := root.DroppedFiles(); len(files) > 0 {
			a.dropImage(files)
		}
		if root.FileDragOver() {
			root.DrawOver(func(p *ui.Painter, r ui.Rect) { p.Stroke(r, t.Accent, 0, 2) })
		}
	}
	root.Children(func() {
		bar := c.TitleBar()
		ui.Row(c).Height(max(bar.Height, t.Space(8))).Padding(0, bar.Right+t.Space(3), 0, bar.Left+t.Space(3)).Gap(t.Space(2)).AlignItems(ui.Center).DragWindow().Children(func() {
			ui.Image(c, a.logo).Size(t.Space(4), t.Space(4)).Radius(t.Space(1))
			ui.Text(c, "NextAI Translator").FontSize(max(12, t.Rem(12.0/14))).SingleLine()
			ui.Spacer(c)
		})
		c.Key(a.page)
		ui.Column(c).Grow(1).WidthPercent(100).Children(func() {
			switch a.page {
			case "translator":
				a.translator(c)
			case "settings":
				a.settings(c)
			default:
				ui.Column(c).Fill().Padding(t.Space(4), 0).Gap(t.Space(3)).Children(func() {
					switch a.page {
					case "history":
						a.history(c)
					case "vocabulary":
						a.vocabulary(c)
					case "actions":
						a.actions(c)
					}
				})
			}
			if a.page != "settings" {
				a.errorNotice(c)
			}
		})
		if a.page != "settings" {
			a.footer(c)
		}
	})
	if a.notice != "" {
		c.Toast(a.tr(a.notice))
		a.notice = ""
	}
	shortcutsEnabled := !a.suppressShortcuts && a.recordingShortcut == ""
	a.suppressShortcuts = false
	if shortcutsEnabled && c.Shortcut(ui.Cmd, ui.KeyEnter) {
		a.start()
	}
	if shortcutsEnabled && c.Shortcut(0, ui.KeyEscape) {
		if a.busy {
			a.stop()
		} else if a.page != "translator" {
			a.page = "translator"
		}
	}
	if shortcutsEnabled && c.Shortcut(ui.Cmd, ui.KeyComma) {
		a.openSettings()
	}
	if shortcutsEnabled && c.Shortcut(ui.Cmd|ui.Shift, ui.KeyEnter) && a.result != "" && a.previousWindow != 0 {
		a.insert()
	}
	if a.Window != nil && a.appliedAppearance != a.Data.Settings.Appearance {
		source := mygo.ThemeSystem
		if a.Data.Settings.Appearance == "浅色" {
			source = mygo.ThemeLight
		}
		if a.Data.Settings.Appearance == "深色" {
			source = mygo.ThemeDark
		}
		a.appliedAppearance = a.Data.Settings.Appearance
		if runtime.GOOS == "windows" && desktop.SupportsBackgroundBlur() {
			mygo.Theme.SetSource(source)
		} else {
			a.Window.SetWindowButtonsTheme(source)
		}
	}
}

func (a *App) applyTheme(c *ui.Context) {
	if a.Data.Settings.Appearance == "浅色" {
		c.SetTheme(ui.LightTheme())
	}
	if a.Data.Settings.Appearance == "深色" {
		c.SetTheme(ui.DarkTheme())
	}
	theme := *c.Theme()
	if a.Data.Settings.UIFontSize > 0 {
		theme.FontSize = float32(a.Data.Settings.UIFontSize)
	}
	theme.Accent, theme.AccentText, theme.Focus = theme.Text, theme.Background, theme.Text
	theme.AccentHover, theme.AccentPressed = theme.TextMuted, theme.Text
	theme.ScrollbarStyle.TrackWidth, theme.ScrollbarStyle.ThumbWidth = 8, 8
	c.SetTheme(&theme)
}

func (a *App) errorNotice(c *ui.Context) {
	if a.failure == "" {
		return
	}
	t := c.Theme()
	ui.Row(c).Padding(t.Space(2), t.Space(4)).Gap(t.Space(2)).Children(func() {
		ui.Text(c, preview(a.failure, 400)).Label(a.tr("错误")).Role(ui.RoleAlert).TextColor(t.Danger).Grow(1)
		if iconButton(c, "clear", a.tr("关闭错误")).Clicked() {
			a.failure = ""
		}
	})
}

func (a *App) translator(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Padding(t.Space(4), t.Space(5)).Gap(t.Space(2)).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		a.languagePicker(c, &a.Data.Settings.Source, append([]string{"自动检测"}, core.Languages...), a.tr("源语言"))
		if iconButton(c, "swap", a.tr("交换")).Disabled(a.Data.Settings.Source == "自动检测" || a.busy).Clicked() {
			a.Data.Settings.Source, a.Data.Settings.Target = a.Data.Settings.Target, a.Data.Settings.Source
		}
		a.languagePicker(c, &a.Data.Settings.Target, core.Languages, a.tr("目标语言"))
		a.actionToolbar(c)
	})
	ui.Column(c).Grow(1).WidthPercent(100).Padding(t.Space(8), 0, t.Space(2)).Children(func() {
		input := func() {
			ui.Column(c).Fill().Gap(t.Space(1)).Children(func() {
				c.Key("source-text")
				inset, radius := t.Space(5), t.Space(2)
				editor := ui.TextArea(c, &a.text).Label(a.tr("原文输入")).AutoFocus().ReadOnly(a.busy).Grow(1).WidthPercent(100).
					Padding(t.Space(3), inset+t.Space(3)).Background(ui.Transparent).Border(0, ui.Transparent).FocusRing(false).OnTextKey(func(k ui.TextKey) bool {
					if k.Key == ui.KeyEnter && k.Mods == 0 {
						a.text = k.Text()
						a.start()
						return true
					}
					return false
				})
				border := t.Border
				if editor.Focused() {
					border = t.Accent
				} else if editor.Hovered() {
					border = t.Border.Mix(t.Text, 0.25)
				}
				// Inset the visible field; the editor still spans the window so its scroll bar reaches the edge.
				editor.Draw(func(p *ui.Painter, r ui.Rect) {
					r.X += inset
					r.W -= 2 * inset
					p.Fill(r, t.Surface, radius)
					p.Stroke(r, border, radius, 1)
				})
				if a.Data.Settings.FontSize > 0 {
					editor.FontSize(float32(a.Data.Settings.FontSize))
				}
				if a.Data.Settings.Speech.ReadSelection {
					start, end := editor.TextSelection()
					if start > end {
						start, end = end, start
					}
					selected := string([]rune(a.text)[start:end])
					if selected != a.selectedSpeech {
						a.selectedSpeech = selected
						a.selectionSpeechGeneration++
						generation := a.selectionSpeechGeneration
						if strings.TrimSpace(selected) != "" {
							time.AfterFunc(300*time.Millisecond, func() {
								a.update(func() {
									if a.selectionSpeechGeneration == generation && a.Data.Settings.Speech.ReadSelection && a.page == "translator" {
										a.speakText(selected, a.Data.Settings.Source, a.Data.Settings.Speech)
									}
								})
							})
						}
					}
				}
				if strings.TrimSpace(a.text) != "" || a.busy {
					ui.Row(c).Margin(0, t.Space(5)).Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
						ui.Text(c, fmt.Sprint(utf8.RuneCountInString(a.text))).FontSize(max(12, t.Rem(12.0/14))).TextColor(t.TextMuted)
						ui.Spacer(c)
						ui.Text(c, a.tr("Enter 提交，Shift+Enter 换行")).FontSize(max(12, t.Rem(12.0/14))).TextColor(t.TextMuted).SingleLine()
						label, name := "提交", "submit"
						if a.busy {
							label, name = "停止", "stop"
						}
						c.Key("submit")
						if ui.PrimaryButton(c, "").Label(a.tr(label)).Disabled(a.systemBusy).Children(func() { ui.Icon(c, icons[name]).Size(t.Rem(1.2), t.Rem(1.2)); ui.Text(c, a.tr(label)).SingleLine() }).Clicked() {
							if a.busy {
								a.stop()
							} else {
								a.start()
							}
						}
					})
				}
				ui.Toolbar(c, func() {
					if iconButton(c, "image", a.tr("导入图片")).Disabled(a.busy || a.systemBusy).Clicked() {
						a.importImage()
					}
					if iconButton(c, "vocabulary", a.tr("生词本")).Clicked() {
						a.page = "vocabulary"
					}
					ui.Spacer(c)
					if a.text != "" && iconButton(c, "clear", a.tr("清空")).Disabled(a.busy).Clicked() {
						a.hideInlineLookup()
						a.text = ""
						a.result = ""
						a.notice = ""
					}
					if a.busy || a.systemBusy {
						ui.Spinner(c).Label(a.tr("正在处理"))
					}
				}).Label(a.tr("原文工具")).Margin(0, t.Space(5))
			})
		}
		output := func() {
			a.md.Theme = &markdown.Theme{FontSize: float32(a.Data.Settings.FontSize)}
			ui.Column(c).Fill().Gap(t.Space(2)).Padding(t.Space(3), 0, 0, 0).Children(func() {
				ui.Scroll(c).Grow(1).WidthPercent(100).Padding(0, t.Space(5)).Label(a.tr("翻译结果")).Busy(a.busy).Children(func() {
					if a.resultFormat == "纯文本" {
						result := ui.Text(c, a.result).Selectable().Label(a.tr("纯文本结果")).WidthPercent(100)
						if a.Data.Settings.FontSize > 0 {
							result.FontSize(float32(a.Data.Settings.FontSize))
						}
					} else if a.busy {
						markdown.Stream(c, markdownHardBreaks(a.result), &a.md)
					} else {
						markdown.View(c, markdownHardBreaks(a.result), &a.md)
					}
				})
				if a.result != "" {
					ui.Toolbar(c, func() {
						ui.Spacer(c)
						if iconButton(c, "copy", a.tr("复制")).Clicked() {
							c.WriteClipboard(a.result)
							c.Toast(a.tr("已复制"))
						}
						if !a.speaking && iconButton(c, "speak", a.tr("朗读")).Clicked() {
							a.speak()
						}
						if a.speaking && iconButton(c, "stop", a.tr("停止朗读")).Clicked() {
							a.stopSpeaking()
						}
						if iconButton(c, "star", a.tr("收藏单词")).Disabled(a.busy).Clicked() {
							a.addWord(c)
						}
						if iconButton(c, "insert", a.tr("写回")).Disabled(a.busy || a.systemBusy || a.previousWindow == 0).Clicked() {
							a.insert()
						}
					}).Label(a.tr("结果工具")).Margin(0, t.Space(5))
				}
			})
		}
		ui.SplitVerticalPercent(c, &a.editorPercent, input, output).Grow(1).WidthPercent(100)
	})
}

func (a *App) actionToolbar(c *ui.Context) {
	t := c.Theme()
	services := c.Services()
	ui.Toolbar(c, func() {
		var visible []core.Action
		for _, v := range a.Data.Actions {
			if len(visible) < 4 {
				visible = append(visible, v)
			}
		}
		for _, v := range a.Data.Actions {
			if v.Name == a.action {
				found := false
				for _, shown := range visible {
					if shown.ID == v.ID {
						found = true
					}
				}
				if !found && len(visible) > 0 {
					visible[len(visible)-1] = v
				}
			}
		}
		for _, v := range visible {
			selected := v.Name == a.action
			name := v.ID
			if icons[name] == nil {
				name = "translate"
			}
			c.Key("action-" + v.ID)
			b := ui.Button(c, "").Label(a.tr(v.Name)).Tooltip(a.tr(v.Name)).Height(t.Space(7)).Padding(0, t.Space(2)).Radius(t.Radius).Disabled(a.systemBusy || a.busy).AccessSelected(selected).Children(func() {
				ui.Icon(c, icons[name]).Size(t.Rem(1.2), t.Rem(1.2))
				if selected {
					ui.Text(c, a.tr(v.Name)).SingleLine().MaxWidth(t.Rem(6))
				}
			})
			if selected {
				b.Background(t.Text).TextColor(t.Background)
			} else {
				b.Width(t.Space(7)).Padding(0).Background(t.Surface)
			}
			if b.Clicked() {
				a.action = v.Name
			}
		}
		iconButton(c, "more", a.tr("更多")).PopoverMenuButton(func(m *ui.Menu) {
			for _, v := range a.Data.Actions {
				if m.Item(a.tr(v.Name)).Checked(a.action == v.Name).Disabled(a.busy).Chosen() {
					a.action = v.Name
				}
			}
			m.Separator()
			if m.Item(a.tr("粘贴")).Disabled(a.busy).Chosen() {
				a.text = services.ReadClipboard()
			}
			if m.Item(a.tr("截图翻译")).Disabled(a.busy || a.systemBusy).Chosen() {
				a.screenshot()
			}
			if m.Item(a.tr("置顶")).Checked(a.pinned).Chosen() {
				a.togglePinned()
			}
			m.Separator()
			if m.Item(a.tr("生词本")).Chosen() {
				a.page = "vocabulary"
			}
			if m.Item(a.tr("动作管理")).Chosen() {
				a.page = "actions"
			}
		}, ui.PopoverMenuButtonOptions{At: ui.AnchorBottomEnd, Self: ui.AnchorTopEnd})
	}).Label(a.tr("处理方式")).Gap(t.Space(1)).MaxWidthPercent(52)
}

func (a *App) footer(c *ui.Context) {
	t := c.Theme()
	profile := a.Data.Settings.Profiles[a.Data.Settings.Provider]
	for _, v := range a.Data.Actions {
		if v.Name == a.action {
			if v.Provider != "" && v.Provider != "跟随全局" {
				profile = a.Data.Settings.Profiles[v.Provider]
			}
			if v.Model != "" {
				profile.Model = v.Model
			}
		}
	}
	ui.Toolbar(c, func() {
		if iconButton(c, "settings", a.tr("设置")).Clicked() {
			if a.page == "settings" {
				a.page = "translator"
			} else {
				a.openSettings()
			}
		}
		ui.Text(c, profile.Provider+"  "+profile.Model).FontSize(max(12, t.Rem(12.0/14))).TextColor(t.TextMuted).SingleLine().Grow(1)
		if label := a.updateLabel(); label != "" {
			button := ui.Button(c, "").Label(label).Tooltip(label).Height(t.Space(7)).Padding(0, t.Space(2)).Radius(t.Radius).Disabled(a.updating)
			button.Children(func() {
				ui.Text(c, label).FontSize(max(12, t.Rem(12.0/14))).SingleLine()
			})
			if button.Clicked() {
				a.installLatest()
			}
		}
		if iconButton(c, "history", a.tr("历史记录")).Clicked() {
			if a.page == "history" {
				a.page = "translator"
			} else {
				a.page = "history"
			}
		}
	}).Label(a.tr("应用导航")).Height(t.Space(10)).Padding(0, t.Space(4)).BorderWidth(1, 0, 0, 0).BorderColor(t.Border)
}

func (a *App) actionNames() []string {
	names := make([]string, len(a.Data.Actions))
	for i, v := range a.Data.Actions {
		names[i] = v.Name
	}
	return names
}

func (a *App) history(c *ui.Context) {
	t := c.Theme()
	ui.Toolbar(c, func() {
		if iconButton(c, "back", a.tr("返回翻译")).Clicked() {
			a.page = "translator"
		}
		ui.Text(c, a.tr("历史记录")).FontSize(t.Rem(1.4)).Bold()
		ui.Spacer(c)
		ui.Toggle(c, &a.favoritesOnly, a.tr("仅收藏"))
		if iconTextButton(c, "export", a.tr("导出记录"), "", false).Clicked() {
			a.export("history")
		}
	}).Label(a.tr("历史记录工具")).Margin(0, t.Space(4))
	ui.SearchField(c, &a.search).Margin(0, t.Space(4)).Label(a.tr("搜索记录")).Placeholder(a.tr("搜索原文或译文"))
	var rows []int
	for i, r := range a.Data.History {
		if match(a.search, r.Text, r.Result) && (!a.favoritesOnly || r.Favorite) {
			rows = append(rows, i)
		}
	}
	remove := -1
	if len(rows) == 0 {
		a.empty(c, "还没有匹配的翻译记录", "开始翻译")
	} else {
		ui.List(c, nil, len(rows), func(n int) {
			i := rows[n]
			r := a.Data.History[i]
			ui.Column(c).Key(r.ID).Padding(t.Space(3), t.Space(7)).Gap(t.Space(2)).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
				ui.Text(c, preview(r.Text, 140)).Bold()
				ui.Text(c, preview(r.Result, 240)).TextColor(t.TextMuted)
				ui.Toolbar(c, func() {
					ui.Text(c, r.CreatedAt.Format("2006-01-02 15:04")+"  "+r.Action).TextColor(t.TextMuted)
					ui.Spacer(c)
					if iconTextButton(c, "open", a.tr("打开"), "", false).Clicked() {
						a.restore(r)
					}
					if ui.Toggle(c, &a.Data.History[i].Favorite, a.tr("收藏")).Changed() {
						a.save()
					}
					if iconTextButton(c, "delete", a.tr("删除"), "", false).Clicked() {
						remove = i
					}
				}).Label(a.tr("记录操作"))
			})
		}).Grow(1).WidthPercent(100).Label(a.tr("历史记录"))
	}
	if remove >= 0 {
		removed := a.Data.History[remove]
		a.Data.History = append(a.Data.History[:remove], a.Data.History[remove+1:]...)
		if a.save() {
			c.ToastAction(a.tr("记录已删除"), a.tr("撤销"), func() { a.Data.History = append([]core.Record{removed}, a.Data.History...); a.save() })
		}
	}
}

func (a *App) vocabulary(c *ui.Context) {
	t := c.Theme()
	ui.Toolbar(c, func() {
		if iconButton(c, "back", a.tr("返回翻译")).Clicked() {
			a.page = "translator"
		}
		ui.Text(c, a.tr("生词本")).FontSize(t.Rem(1.4)).Bold()
		ui.Spacer(c)
		if iconTextButton(c, "story", a.tr("生成助记故事"), "", false).Disabled(len(a.Data.Vocabulary) == 0 || a.busy).Clicked() {
			var words []string
			for _, w := range a.Data.Vocabulary {
				words = append(words, w.Text)
			}
			for _, v := range a.Data.Actions {
				if v.ID == "big-bang" {
					a.action = v.Name
				}
			}
			a.text = strings.Join(words, ", ")
			a.page = "translator"
			a.start()
		}
		if iconTextButton(c, "export", a.tr("导出生词"), "", false).Clicked() {
			a.export("vocabulary")
		}
	}).Label(a.tr("生词本工具")).Margin(0, t.Space(4))
	ui.SearchField(c, &a.search).Margin(0, t.Space(4)).Label(a.tr("搜索生词")).Placeholder(a.tr("搜索单词或释义"))
	var rows []int
	for i, w := range a.Data.Vocabulary {
		if match(a.search, w.Text, w.Description) {
			rows = append(rows, i)
		}
	}
	remove := -1
	if len(rows) == 0 {
		a.empty(c, "查词后可将结果加入生词本", "开始查词")
	} else {
		ui.List(c, nil, len(rows), func(n int) {
			i := rows[n]
			w := a.Data.Vocabulary[i]
			ui.Column(c).Key(w.Text).Padding(t.Space(3), t.Space(7)).Gap(t.Space(2)).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
				ui.Text(c, preview(w.Text, 120)).Bold()
				ui.Text(c, preview(w.Description, 220)).TextColor(t.TextMuted)
				ui.Toolbar(c, func() {
					ui.Text(c, fmt.Sprintf(a.tr("已复习 %d 次"), w.Reviews)).TextColor(t.TextMuted)
					ui.Spacer(c)
					if iconTextButton(c, "eye", a.tr("查看"), "", false).Clicked() {
						a.stop()
						a.text = w.Text
						a.result = w.Description
						a.page = "translator"
					}
					if iconTextButton(c, "check", a.tr("已复习"), "", false).Clicked() {
						a.Data.Vocabulary[i].Reviews++
						if a.save() {
							c.Toast(a.tr("已记录复习"))
						}
					}
					if iconTextButton(c, "delete", a.tr("删除"), "", false).Clicked() {
						remove = i
					}
				}).Label(a.tr("生词操作"))
			})
		}).Grow(1).WidthPercent(100).Label(a.tr("生词本"))
	}
	if remove >= 0 {
		removed := a.Data.Vocabulary[remove]
		a.Data.Vocabulary = append(a.Data.Vocabulary[:remove], a.Data.Vocabulary[remove+1:]...)
		if a.save() {
			c.ToastAction(a.tr("生词已删除"), a.tr("撤销"), func() { a.Data.Vocabulary = append([]core.Word{removed}, a.Data.Vocabulary...); a.save() })
		}
	}
}

func (a *App) empty(c *ui.Context, text, action string) {
	ui.Column(c).Grow(1).Center().Gap(c.Theme().Space(3)).Children(func() {
		ui.Icon(c, icons["empty"]).Size(c.Theme().Space(12), c.Theme().Space(12)).TextColor(c.Theme().TextMuted)
		ui.Text(c, a.tr(text)).TextColor(c.Theme().TextMuted)
		if ui.PrimaryButton(c, a.tr(action)).Clicked() {
			a.page = "translator"
		}
	})
}

func match(query string, values ...string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	for _, v := range values {
		if strings.Contains(strings.ToLower(v), query) {
			return true
		}
	}
	return false
}

func preview(s string, max int) string {
	r := []rune(strings.Join(strings.Fields(s), " "))
	if len(r) > max {
		return string(r[:max]) + "…"
	}
	return string(r)
}

func markdownHardBreaks(s string) string {
	if !strings.Contains(s, "\n") {
		return s
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	var out strings.Builder
	out.Grow(len(s) + 32)
	var fence byte
	var fenceLen int
	for len(s) > 0 {
		line, rest, hasNL := strings.Cut(s, "\n")
		trimmed := strings.TrimLeft(line, " ")
		indent := len(line) - len(trimmed)
		if fence == 0 {
			if indent <= 3 && len(trimmed) >= 3 && (trimmed[0] == '`' || trimmed[0] == '~') {
				n := 0
				for n < len(trimmed) && trimmed[n] == trimmed[0] {
					n++
				}
				if n >= 3 && (trimmed[0] == '~' || !strings.ContainsRune(trimmed[n:], '`')) {
					fence, fenceLen = trimmed[0], n
				}
			}
			out.WriteString(line)
			if hasNL {
				if fence == 0 && strings.TrimSpace(line) != "" && !strings.HasPrefix(trimmed, "|") && !strings.HasSuffix(line, "  ") && !strings.HasSuffix(line, `\`) {
					out.WriteString("  ")
				}
				out.WriteByte('\n')
			}
		} else {
			out.WriteString(line)
			if hasNL {
				out.WriteByte('\n')
			}
			if indent <= 3 && len(trimmed) >= fenceLen {
				n := 0
				for n < len(trimmed) && trimmed[n] == fence {
					n++
				}
				if n >= fenceLen && strings.TrimSpace(trimmed[n:]) == "" {
					fence, fenceLen = 0, 0
				}
			}
		}
		s = rest
	}
	return out.String()
}

func (a *App) actions(c *ui.Context) {
	t := c.Theme()
	ui.Toolbar(c, func() {
		if iconButton(c, "back", a.tr("返回翻译")).Clicked() {
			a.page = "translator"
		}
		ui.Text(c, a.tr("动作管理")).FontSize(t.Rem(1.4)).Bold()
	}).Label(a.tr("动作管理导航")).Margin(0, t.Space(4))
	ui.Scroll(c).Grow(1).WidthPercent(100).Padding(0, t.Space(4)).Label(a.tr("动作管理")).Children(func() {
		ui.Column(c).Gap(t.Space(4)).Children(func() {
			ui.Form(c, func() {
				ui.Field(c, a.tr("动作"), func() {
					if a.selectSetting(c, &a.actionID, a.actionNames()).Label(a.tr("选择动作")).Changed() {
						for _, v := range a.Data.Actions {
							if v.Name == a.actionID {
								a.editAction = v
							}
						}
					}
				})
				ui.Field(c, a.tr("名称"), func() { ui.TextInput(c, &a.editAction.Name).Label(a.tr("动作名称")) })
				ui.Field(c, a.tr("提示词"), func() {
					if ui.TextArea(c, &a.editAction.Prompt).Label(a.tr("动作提示词")).Placeholder(a.tr("使用内置规则；填写提示词可改为自定义动作")).Lines(8, 16).Changed() {
						a.editAction.Mode = ""
					}
				}).Description(a.tr("支持 ${sourceLang}、${targetLang}、${text}，兼容 {{source}} 和 {{target}}"))
			})
			ui.Collapsible(c, a.tr("模型与输出"), &a.advancedAction, func() {
				ui.Form(c, func() {
					ui.Field(c, a.tr("指令提示词"), func() {
						if ui.TextArea(c, &a.editAction.CommandPrompt).Label(a.tr("动作指令提示词")).Lines(2, 6).Changed() {
							a.editAction.Mode = ""
						}
					})
					ui.Field(c, a.tr("模型服务"), func() {
						providers := []string{"跟随全局"}
						for _, p := range core.Providers {
							providers = append(providers, p.Name)
						}
						a.selectSetting(c, &a.editAction.Provider, providers).Label(a.tr("动作模型服务")).Placeholder(a.tr("跟随全局"))
					})
					ui.Field(c, a.tr("模型"), func() {
						ui.TextInput(c, &a.editAction.Model).Label(a.tr("动作模型")).Placeholder(a.tr("跟随所选服务的模型"))
					})
					ui.Field(c, a.tr("结果格式"), func() {
						a.selectSetting(c, &a.editAction.Format, []string{"Markdown", "纯文本", "LaTeX"}).Label(a.tr("结果格式")).Placeholder("Markdown")
					})
					ui.Field(c, a.tr("深度思考"), func() { ui.Checkbox(c, &a.editAction.Thinking, a.tr("启用")).Label(a.tr("启用深度思考")) }).Description(a.tr("需使用支持推理参数的模型"))
					ui.Field(c, a.tr("思考强度"), func() {
						a.selectSetting(c, &a.editAction.ThinkingLevel, []string{"low", "medium", "high"}).Label(a.tr("思考强度")).Placeholder("medium")
					})
				})
			})
			ui.Toolbar(c, func() {
				if iconTextButton(c, "add", a.tr("新建动作"), "", false).Clicked() {
					a.editAction = core.Action{ID: fmt.Sprint(time.Now().UnixNano()), Name: "新动作"}
				}
				if iconTextButton(c, "save", a.tr("保存动作"), "", true).Clicked() {
					a.saveAction(c)
				}
				if iconTextButton(c, "delete", a.tr("删除动作"), "", false).Disabled(len(a.Data.Actions) <= 1).Clicked() {
					a.deleteAction(c)
				}
			}).Label(a.tr("动作操作"))
		})
	})
}

func (a *App) saveAction(c *ui.Context) {
	if strings.TrimSpace(a.editAction.Name) == "" || (a.editAction.Mode == "" && strings.TrimSpace(a.editAction.Prompt) == "" && strings.TrimSpace(a.editAction.CommandPrompt) == "") {
		a.failure = "请填写动作名称和提示词"
		return
	}
	for _, v := range a.Data.Actions {
		if v.ID != a.editAction.ID && v.Name == a.editAction.Name {
			a.failure = "动作名称已存在"
			return
		}
	}
	found := false
	for i, v := range a.Data.Actions {
		if v.ID == a.editAction.ID {
			if a.action == v.Name {
				a.action = a.editAction.Name
			}
			if a.Data.Settings.DefaultAction == v.Name {
				a.Data.Settings.DefaultAction = a.editAction.Name
			}
			if a.settingsDraft.DefaultAction == v.Name {
				a.settingsDraft.DefaultAction = a.editAction.Name
			}
			a.Data.Actions[i] = a.editAction
			found = true
			break
		}
	}
	if !found {
		a.Data.Actions = append(a.Data.Actions, a.editAction)
	}
	a.actionID = a.editAction.Name
	if a.save() {
		c.Toast(a.tr("动作已保存"))
	}
}

func (a *App) deleteAction(c *ui.Context) {
	for i, v := range a.Data.Actions {
		if v.ID != a.editAction.ID {
			continue
		}
		defaultAction, draftDefault := a.Data.Settings.DefaultAction, a.settingsDraft.DefaultAction
		a.Data.Actions = append(a.Data.Actions[:i], a.Data.Actions[i+1:]...)
		fallback := a.Data.Actions[0].Name
		if a.Data.Settings.DefaultAction == v.Name {
			a.Data.Settings.DefaultAction = a.Data.Actions[0].Name
		}
		if a.settingsDraft.DefaultAction == v.Name {
			a.settingsDraft.DefaultAction = a.Data.Actions[0].Name
		}
		if a.action == v.Name {
			a.action = a.Data.Actions[0].Name
		}
		a.editAction = a.Data.Actions[0]
		a.actionID = a.editAction.Name
		if a.save() {
			c.ToastAction(a.tr("动作已删除"), a.tr("撤销"), func() {
				a.Data.Actions = append(a.Data.Actions, v)
				if defaultAction == v.Name && a.Data.Settings.DefaultAction == fallback {
					a.Data.Settings.DefaultAction = defaultAction
				}
				if draftDefault == v.Name && a.settingsDraft.DefaultAction == fallback {
					a.settingsDraft.DefaultAction = draftDefault
				}
				a.save()
			})
		}
		return
	}
}
