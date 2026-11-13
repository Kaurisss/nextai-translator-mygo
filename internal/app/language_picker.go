package app

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

type languageDraft struct{ text, selected string }

// languagePicker styles MyGo v0.8.3's ComboboxBase. Combobox fixes its inner
// input at >=100 DIP, overflowing narrow language fields after padding and its
// chevron are added. Select lacks typed filtering. The base keeps native text
// editing, keyboard selection, focus and accessibility; the SVG occupies a
// reserved trailing inset instead of competing with the editable text for space.
func languagePicker(c *ui.Context, selected *string, options []string, label string, translate func(string) string) ui.Element {
	t := c.Theme()
	c.Key(label)
	field := ui.Row(c).MinWidth(t.Space(22)).Grow(1).Height(t.Space(8)).AlignItems(ui.Center).
		Padding(0, t.Space(2), 0, t.Space(2.5)).Gap(t.Space(1)).Radius(t.Radius).Background(t.Surface).Border(1, t.Border)
	draft := ui.Local(field, "draft", func() languageDraft { return languageDraft{*selected, *selected} })
	// Keep externally changed choices (swap/history) in the editor's bound value.
	if draft.selected != *selected {
		draft.text = *selected
		draft.selected = *selected
	}
	field.Children(func() {
		cb := ui.ComboboxBase(c, &draft.text)
		cb.Input.Label(label).AutoComplete(ui.AutoCompleteList).MinWidth(0).Grow(1).
			Height(t.Space(7.5)).Padding(t.Space(1.5), 0).FocusRing(false)
		arrow := ui.ButtonBase(c).Label(translate("展开")+label).Tooltip(translate("选择")+label).TabStop(false).KeepFocus().Size(t.Space(5), t.Space(7)).Padding(0).Center().Children(func() {
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
			for _, option := range options {
				if strings.Contains(strings.ToLower(option), strings.ToLower(draft.text)) {
					shown = append(shown, option)
				}
			}
		}
		cb.Popup(func(panel ui.Element) {
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
				if len(shown) == 0 {
					ui.Text(c, translate("没有匹配的语言")).Padding(t.Space(2)).TextColor(t.TextMuted)
				}
			})
		})
		if choice, ok := cb.Chosen(); ok {
			*selected = choice
			draft.selected = choice
			draft.text = choice
		} else if !cb.Input.Focused() && !cb.Open() {
			draft.text = *selected
		}
	})
	return field
}
