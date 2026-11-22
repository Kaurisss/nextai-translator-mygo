package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Kaurisss/nextai-translator-mygo/internal/core"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"
)

const (
	wordHoverShowDelay = 220 * time.Millisecond
	wordHoverHideDelay = 150 * time.Millisecond
)

// Test hooks for dictionary lookup and inline speech playback.
var (
	lookupWordFunc   = core.LookupEnglishWord
	inlineSpeechHook func(a *App, text, language string)
)

type wordHoverState struct {
	activeKey    string
	activeWord   string
	status       string // "", "loading", "ready", "empty", "error"
	preview      *core.WordPreview
	generation   uint64
	cancel       context.CancelFunc
	pendingKey   string
	pendingWord  string
	pendingSince time.Time
	hideSince    time.Time
	cache        map[string]*core.WordPreview
}

// hoverFrame holds per-frame hover discovery callbacks; it stores no ui.Element.
type hoverFrame struct {
	hoveredKey     string
	hoveredWord    string
	hoveredOK      bool
	activeAnchorOK bool
	clickedWord    string
	setHovered     func(ui.Element)
	setActive      func(ui.Element)
}

type hoverInlineStyle struct {
	bold   bool
	italic bool
	strike bool
	inLink bool
}

type drawnSpeaker struct {
	at    int
	icon  *ui.SVG
	color ui.Color
}

type inlineParaBuilder struct {
	blockIdx int
	seq      int
	runes    int
	numbered bool
	fallback string
	text     strings.Builder
	speakers []drawnSpeaker
}

func (a *App) clearWordHover() {
	if a.wordHover.cancel != nil {
		a.wordHover.cancel()
		a.wordHover.cancel = nil
	}
	a.wordHover.generation++
	a.wordHover.activeKey = ""
	a.wordHover.activeWord = ""
	a.wordHover.status = ""
	a.wordHover.preview = nil
	a.wordHover.pendingKey = ""
	a.wordHover.pendingWord = ""
	a.wordHover.pendingSince = time.Time{}
	a.wordHover.hideSince = time.Time{}
}

func (a *App) activateWordHover(key, word string) {
	if a.wordHover.cancel != nil {
		a.wordHover.cancel()
		a.wordHover.cancel = nil
	}
	a.wordHover.generation++
	gen := a.wordHover.generation
	a.wordHover.activeKey = key
	a.wordHover.activeWord = word
	a.wordHover.pendingKey = ""
	a.wordHover.pendingWord = ""
	a.wordHover.pendingSince = time.Time{}
	a.wordHover.hideSince = time.Time{}

	normalized := core.NormalizeLookupWord(word)
	if a.wordHover.cache != nil {
		if cached, ok := a.wordHover.cache[normalized]; ok {
			a.wordHover.preview = cached
			if cached != nil {
				a.wordHover.status = "ready"
			} else {
				a.wordHover.status = "empty"
			}
			return
		}
	}

	client := a.client
	if client == nil {
		a.wordHover.preview = nil
		a.wordHover.status = "error"
		return
	}

	a.wordHover.preview = nil
	a.wordHover.status = "loading"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	a.wordHover.cancel = cancel
	lookup := lookupWordFunc
	go func(c *http.Client) {
		defer cancel()
		preview, err := lookup(ctx, c, word)
		if ctx.Err() != nil {
			return
		}
		a.update(func() {
			if a.wordHover.generation != gen || a.wordHover.activeKey != key {
				return
			}
			a.wordHover.cancel = nil
			if err != nil {
				a.wordHover.preview = nil
				a.wordHover.status = "error"
				return
			}
			if a.wordHover.cache == nil {
				a.wordHover.cache = map[string]*core.WordPreview{}
			}
			a.wordHover.cache[normalized] = preview
			a.wordHover.preview = preview
			if preview != nil {
				a.wordHover.status = "ready"
			} else {
				a.wordHover.status = "empty"
			}
		})
	}(client)
}

func (a *App) openWordDetails(word string) {
	a.clearWordHover()
	for _, act := range a.Data.Actions {
		if act.Mode == "translate" {
			a.action = act.Name
			break
		}
	}
	if a.Data.Settings.Source != "自动检测" && core.LanguageCode(a.Data.Settings.Source, "") != "en" {
		a.Data.Settings.Source = "自动检测"
	}
	if core.LanguageCode(a.Data.Settings.Target, "") == "en" {
		target := a.Data.Settings.DefaultTarget
		if target == "" || core.LanguageCode(target, "") == "en" {
			target = "简体中文"
		}
		a.Data.Settings.Target = target
	}
	a.page = "translator"
	a.text = word
	a.start()
}

func (a *App) inlineSpeechLanguage() string {
	if a.resultWordMode {
		return a.Data.Settings.Source
	}
	return a.Data.Settings.Target
}

func (a *App) toggleInlineSpeech(key, text string) {
	if a.speaking && a.inlineSpeechKey == key {
		a.stopSpeaking()
		return
	}
	lang := a.inlineSpeechLanguage()
	if inlineSpeechHook != nil {
		a.speaking = true
		a.inlineSpeechKey = key
		inlineSpeechHook(a, text, lang)
		return
	}
	a.speakText(text, lang, a.Data.Settings.Speech)
	if a.speaking {
		a.inlineSpeechKey = key
	}
}

// renderTranslatorResult renders the translation result area with per-word
// hover dictionary and inline phonetic/example speech buttons when not
// streaming, and shows the active hover card.
func (a *App) renderTranslatorResult(c *ui.Context) {
	t := c.Theme()
	var hoveredAnchor, activeAnchor ui.Element
	hf := hoverFrame{
		setHovered: func(el ui.Element) { hoveredAnchor = el },
		setActive:  func(el ui.Element) { activeAnchor = el },
	}
	hoverEnabled := !a.busy && strings.TrimSpace(a.result) != ""
	if !hoverEnabled && a.wordHover.activeKey != "" {
		a.clearWordHover()
	}

	ui.Scroll(c).Grow(1).WidthPercent(100).Padding(0, t.Space(5)).Label(a.tr("翻译结果")).Busy(a.busy).Children(func() {
		if a.resultFormat == "纯文本" {
			if hoverEnabled && (core.HasEnglishWord(a.result) || core.HasPhoneticOrExample(a.result, false)) {
				a.renderHoverPlainText(c, a.result, &hf)
			} else {
				result := ui.Text(c, a.result).Selectable().Label(a.tr("纯文本结果")).WidthPercent(100)
				if a.Data.Settings.FontSize > 0 {
					result.FontSize(float32(a.Data.Settings.FontSize))
				}
			}
		} else if a.busy {
			a.md.Block = nil
			markdown.Stream(c, markdownHardBreaks(a.result), &a.md)
		} else {
			a.md.Block = func(c *ui.Context, d *parser.Document, i int) bool {
				return a.renderHoverMarkdownBlock(c, d, i, &hf)
			}
			markdown.View(c, markdownHardBreaks(a.result), &a.md)
			a.md.Block = nil
		}
	})

	if !hoverEnabled {
		return
	}

	if hf.clickedWord != "" {
		a.openWordDetails(hf.clickedWord)
		return
	}

	now := c.Now()
	if hf.hoveredKey != "" {
		a.wordHover.hideSince = time.Time{}
		if hf.hoveredKey != a.wordHover.activeKey {
			if hf.hoveredKey != a.wordHover.pendingKey {
				a.wordHover.pendingKey = hf.hoveredKey
				a.wordHover.pendingWord = hf.hoveredWord
				a.wordHover.pendingSince = now
			}
			if wait := wordHoverShowDelay - now.Sub(a.wordHover.pendingSince); wait > 0 {
				c.After(wait)
			} else {
				a.activateWordHover(hf.hoveredKey, hf.hoveredWord)
				if hf.hoveredOK {
					activeAnchor = hoveredAnchor
					hf.activeAnchorOK = true
				}
			}
		} else {
			a.wordHover.pendingKey = ""
			a.wordHover.pendingWord = ""
			a.wordHover.pendingSince = time.Time{}
		}
	} else {
		a.wordHover.pendingKey = ""
		a.wordHover.pendingWord = ""
		a.wordHover.pendingSince = time.Time{}
	}

	if a.wordHover.activeKey != "" && !hf.activeAnchorOK {
		a.clearWordHover()
		return
	}

	cardHovered := false
	if a.wordHover.activeKey != "" && hf.activeAnchorOK {
		open := true
		ui.PopoverBase(c, activeAnchor, &open, func(panel ui.Element) {
			cardHovered = panel.Hovered()
			a.renderWordHoverCard(c, panel, activeAnchor)
		})
		if !open {
			a.clearWordHover()
			return
		}
	}

	if a.wordHover.activeKey != "" && hf.hoveredKey == "" {
		if cardHovered {
			a.wordHover.hideSince = time.Time{}
		} else {
			if a.wordHover.hideSince.IsZero() {
				a.wordHover.hideSince = now
			}
			if wait := wordHoverHideDelay - now.Sub(a.wordHover.hideSince); wait > 0 {
				c.After(wait)
			} else {
				a.clearWordHover()
				c.AnimationFrame()
			}
		}
	}
}

func (a *App) renderHoverPlainText(c *ui.Context, text string, hf *hoverFrame) {
	th := a.markdownTheme(c)
	el := ui.RichText(c).Selectable().Label(a.tr("纯文本结果")).WidthPercent(100)
	if a.Data.Settings.FontSize > 0 {
		el.FontSize(float32(a.Data.Settings.FontSize))
	}
	pb := inlineParaBuilder{blockIdx: 0, fallback: a.text}
	el.Children(func() {
		a.renderPhoneticText(c, text, hoverInlineStyle{}, th, &pb, hf)
	})
	attachInlineSpeakers(el, pb.speakers)
}

func (a *App) markdownTheme(c *ui.Context) *markdown.Theme {
	th := markdown.DefaultTheme(c.Theme())
	if a.Data.Settings.FontSize > 0 {
		th.FontSize = float32(a.Data.Settings.FontSize)
	}
	return th
}

func isOrderedListParagraph(d *parser.Document, i int) bool {
	b := d.Block(i)
	if b.Parent < 0 {
		return false
	}
	parent := d.Block(b.Parent)
	if parent.Kind != parser.Item || parent.Parent < 0 {
		return false
	}
	list := d.Block(parent.Parent)
	return list.Kind == parser.List && list.Ordered
}

func blockSpeechFallback(d *parser.Document, i int, defaultFallback string) string {
	for prev := i - 1; prev >= 1; prev-- {
		k := d.Block(prev).Kind
		if k == parser.Paragraph || k == parser.Heading {
			if text := strings.TrimSpace(parser.PlainText(d.Inlines(prev))); text != "" {
				return text
			}
			break
		}
	}
	return defaultFallback
}

func (a *App) renderHoverMarkdownBlock(c *ui.Context, d *parser.Document, i int, hf *hoverFrame) bool {
	b := d.Block(i)
	if b.Kind != parser.Paragraph && b.Kind != parser.Heading {
		return false
	}
	inlines := d.Inlines(i)
	numbered := isOrderedListParagraph(d, i)
	if len(inlines) == 0 || isImagesOnlyInlines(inlines) || !hasInteractiveInlines(inlines, numbered) {
		return false
	}
	th := a.markdownTheme(c)
	el := ui.RichText(c).WidthPercent(100).LineHeight(th.LineHeight)
	if b.Kind == parser.Heading {
		h := th.Headings[b.Level-1]
		el.FontSize(th.FontSize * h.Size).FontWeight(h.Weight).Role(ui.RoleHeading).Level(b.Level)
		if h.Color.A > 0 {
			el.TextColor(h.Color)
		}
		if h.Rule {
			el.BorderWidth(0, 0, 1, 0).BorderColor(th.Rule).Padding(0, 0, th.FontSize*0.3, 0)
		}
	} else {
		el.FontSize(th.FontSize)
	}
	pb := inlineParaBuilder{
		blockIdx: i,
		numbered: numbered,
		fallback: blockSpeechFallback(d, i, a.text),
	}
	el.Children(func() {
		a.renderHoverInlines(c, inlines, 0, len(inlines), hoverInlineStyle{}, th, &pb, hf)
	})
	attachInlineSpeakers(el, pb.speakers)
	return true
}

func attachInlineSpeakers(el ui.Element, speakers []drawnSpeaker) {
	if len(speakers) == 0 {
		return
	}
	el.DrawObjects(func(p *ui.Painter, at int, r ui.Rect) {
		for _, sp := range speakers {
			if sp.at == at {
				pad := float32(2)
				iconRect := ui.Rect{
					X: r.X + pad,
					Y: r.Y + pad/2,
					W: max(1, r.W-2*pad),
					H: max(1, r.H-pad),
				}
				p.Icon(sp.icon, iconRect, sp.color)
				return
			}
		}
	})
}

func isImagesOnlyInlines(inlines []parser.Inline) bool {
	hasImage := false
	for k := 0; k < len(inlines); k = inlines[k].Next {
		switch inlines[k].Kind {
		case parser.Image:
			hasImage = true
		case parser.SoftBreak, parser.LineBreak:
		case parser.Text:
			if strings.TrimSpace(inlines[k].Text) != "" {
				return false
			}
		default:
			return false
		}
	}
	return hasImage
}

func hasInteractiveInlines(inlines []parser.Inline, numbered bool) bool {
	return hasInteractiveRange(inlines, 0, len(inlines), false, numbered)
}

func hasInteractiveRange(inlines []parser.Inline, from, to int, inLink, numbered bool) bool {
	firstText := true
	for k := from; k < to; k = inlines[k].Next {
		n := &inlines[k]
		switch n.Kind {
		case parser.Text:
			if !inLink && (core.HasEnglishWord(n.Text) || core.HasPhoneticOrExample(n.Text, numbered && firstText)) {
				return true
			}
			firstText = false
		case parser.Emphasis, parser.Strong, parser.Strikethrough:
			if hasInteractiveRange(inlines, k+1, n.Next, inLink, numbered && firstText) {
				return true
			}
			firstText = false
		case parser.LineBreak:
			firstText = false
		case parser.Link:
			firstText = false
		}
	}
	return false
}

func (a *App) renderHoverInlines(c *ui.Context, inlines []parser.Inline, from, to int, style hoverInlineStyle, th *markdown.Theme, pb *inlineParaBuilder, hf *hoverFrame) {
	for k := from; k < to; k = inlines[k].Next {
		n := &inlines[k]
		switch n.Kind {
		case parser.Text:
			if style.inLink {
				applyInlineStyle(ui.Text(c, n.Text), style, th)
				pb.runes += utf8.RuneCountInString(n.Text)
				pb.text.WriteString(n.Text)
			} else {
				a.renderPhoneticText(c, n.Text, style, th, pb, hf)
			}
		case parser.SoftBreak:
			applyInlineStyle(ui.Text(c, " "), style, th)
			pb.runes++
			pb.text.WriteByte(' ')
		case parser.LineBreak:
			applyInlineStyle(ui.Text(c, "\n"), style, th)
			pb.runes++
			pb.text.WriteByte('\n')
		case parser.Emphasis:
			next := style
			next.italic = true
			a.renderHoverInlines(c, inlines, k+1, n.Next, next, th, pb, hf)
		case parser.Strong:
			next := style
			next.bold = true
			a.renderHoverInlines(c, inlines, k+1, n.Next, next, th, pb, hf)
		case parser.Strikethrough:
			next := style
			next.strike = true
			a.renderHoverInlines(c, inlines, k+1, n.Next, next, th, pb, hf)
		case parser.Code:
			el := ui.Text(c, n.Text).Font(th.CodeFont).FontSize(th.FontSize * th.CodeSize).Background(th.CodeBackground)
			if !style.inLink {
				el.TextColor(th.Code)
			}
			applyInlineStyle(el, style, th)
			pb.runes += utf8.RuneCountInString(n.Text)
			pb.text.WriteString(n.Text)
		case parser.Link:
			if style.inLink {
				a.renderHoverInlines(c, inlines, k+1, n.Next, style, th, pb, hf)
				continue
			}
			dest := n.Dest
			if n.Pending {
				dest = ""
			}
			linkEl := ui.Link(c, "", dest).TextColor(th.Link)
			if n.Title != "" {
				linkEl.Tooltip(n.Title)
			} else if !n.Auto && dest != "" && !strings.HasPrefix(dest, "#") {
				linkEl.Tooltip(dest)
			}
			next := style
			next.inLink = true
			linkEl.Children(func() {
				a.renderHoverInlines(c, inlines, k+1, n.Next, next, th, pb, hf)
			})
		case parser.Image:
			alt := parser.PlainText(inlines[k+1 : n.Next])
			if alt == "" {
				alt = "image"
			}
			s := "[" + alt + "]"
			ui.Text(c, s).TextColor(th.Muted)
			pb.runes += utf8.RuneCountInString(s)
			pb.text.WriteString(s)
		case parser.Math:
			ui.Text(c, n.Text).Font(th.CodeFont).FontSize(th.FontSize * th.CodeSize).TextColor(th.Muted)
			pb.runes += utf8.RuneCountInString(n.Text)
			pb.text.WriteString(n.Text)
		case parser.FootnoteRef:
			s := "[" + n.Text + "]"
			ui.Text(c, s).FontSize(th.FontSize * 0.75).TextColor(th.Link)
			pb.runes += utf8.RuneCountInString(s)
			pb.text.WriteString(s)
		case parser.HTML, parser.Mention, parser.Reference:
			applyInlineStyle(ui.Text(c, n.Text), style, th)
			pb.runes += utf8.RuneCountInString(n.Text)
			pb.text.WriteString(n.Text)
		}
	}
}

func applyInlineStyle(el ui.Element, style hoverInlineStyle, th *markdown.Theme) ui.Element {
	if style.bold {
		el.FontWeight(th.StrongWeight)
	}
	if style.italic {
		el.Italic()
	}
	if style.strike {
		el.Strikethrough()
	}
	return el
}

func (a *App) renderPhoneticText(c *ui.Context, text string, style hoverInlineStyle, th *markdown.Theme, pb *inlineParaBuilder, hf *hoverFrame) {
	numberedFirst := pb.numbered && pb.text.Len() == 0
	segments := core.ParsePhoneticSegmentsWithContext(text, pb.text.String(), pb.fallback, numberedFirst)
	for _, seg := range segments {
		switch seg.Kind {
		case core.PhoneticIPA:
			applyInlineStyle(ui.Text(c, seg.Text), style, th)
			pb.runes += utf8.RuneCountInString(seg.Text)
			pb.text.WriteString(seg.Text)
			a.renderInlineSpeaker(c, seg.SpeechText, th, pb)
		case core.PhoneticExample:
			a.renderHoverTextPieces(c, seg.Text, style, th, pb, hf)
			a.renderInlineSpeaker(c, seg.SpeechText, th, pb)
		default:
			a.renderHoverTextPieces(c, seg.Text, style, th, pb, hf)
		}
	}
}

func (a *App) renderInlineSpeaker(c *ui.Context, speechText string, th *markdown.Theme, pb *inlineParaBuilder) {
	if strings.TrimSpace(speechText) == "" {
		return
	}
	t := c.Theme()
	pb.seq++
	key := fmt.Sprintf("spk%d:%d", pb.blockIdx, pb.seq)

	// Add a thin visual space before the inline speaker icon.
	ui.Text(c, " ")
	pb.runes++

	active := a.speaking && a.inlineSpeechKey == key
	icon := icons["speak"]
	label := a.tr("朗读") + " " + speechText
	if active {
		icon = icons["stop"]
		label = a.tr("停止朗读")
	}

	size := max(float32(13), th.FontSize*0.92)
	c.Key(key)
	sp := ui.RichText(c, ui.Span{
		Text:          "\uFFFC",
		ObjectWidth:   size + 4,
		ObjectHeight:  size * 0.85,
		ObjectDescent: size * 0.15,
		Radius:        4,
	}).Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(label).Tooltip(label)

	hovered := sp.Hovered()
	color := t.TextMuted
	if active {
		if t.Background.R < 128 {
			color = ui.RGB(147, 197, 253)
		} else {
			color = ui.RGB(39, 110, 241)
		}
	} else if hovered {
		color = t.Text
		sp.Background(t.SurfaceHover)
	}
	if sp.Clicked() {
		a.toggleInlineSpeech(key, speechText)
	}

	pb.speakers = append(pb.speakers, drawnSpeaker{
		at:    pb.runes,
		icon:  icon,
		color: color,
	})
	pb.runes++
}

func (a *App) renderHoverTextPieces(c *ui.Context, text string, style hoverInlineStyle, th *markdown.Theme, pb *inlineParaBuilder, hf *hoverFrame) {
	t := c.Theme()
	dark := t.Background.R < 128
	hoverBg := ui.RGBA(39, 110, 241, 0.09)
	hoverUnderline := ui.RGBA(39, 110, 241, 0.55)
	if dark {
		hoverBg = ui.RGBA(76, 132, 255, 0.16)
		hoverUnderline = ui.RGBA(160, 191, 248, 0.7)
	}
	for _, piece := range core.SplitEnglishWords(text) {
		if !piece.IsWord {
			applyInlineStyle(ui.Text(c, piece.Text), style, th)
			pb.runes += utf8.RuneCountInString(piece.Text)
			pb.text.WriteString(piece.Text)
			continue
		}
		pb.seq++
		key := fmt.Sprintf("w%d:%d", pb.blockIdx, pb.seq)
		c.Key(key)
		w := applyInlineStyle(ui.Text(c, piece.Text), style, th).Cursor(ui.CursorPointer)
		hovered := w.Hovered()
		if hovered || key == a.wordHover.activeKey {
			w.Background(hoverBg).DottedUnderline().DecorationColor(hoverUnderline).DecorationThickness(1).UnderlineOffset(3)
		}
		if hovered {
			hf.hoveredKey = key
			hf.hoveredWord = piece.Text
			hf.setHovered(w)
			hf.hoveredOK = true
		}
		if key == a.wordHover.activeKey {
			hf.setActive(w)
			hf.activeAnchorOK = true
		}
		if w.Clicked() {
			hf.clickedWord = piece.Text
		}
		pb.runes += utf8.RuneCountInString(piece.Text)
		pb.text.WriteString(piece.Text)
	}
}

func (a *App) renderWordHoverCard(c *ui.Context, panel, anchor ui.Element) {
	t := c.Theme()
	dark := t.Background.R < 128
	shadow := ui.RGBA(15, 23, 42, 0.14)
	accentText := ui.RGB(39, 110, 241)
	if dark {
		shadow = ui.RGBA(0, 0, 0, 0.36)
		accentText = ui.RGB(147, 197, 253)
	}
	label := strings.ReplaceAll(a.tr("{{word}} 的释义"), "{{word}}", a.wordHover.activeWord)
	panel.AttachTo(anchor, ui.AnchorBottom, ui.AnchorTop).
		Margin(t.Space(1.5), 0, 0, 0).
		Width(320).
		MaxWidthPercent(90).
		Padding(t.Space(3.5)).
		Radius(t.Space(3)).
		Background(t.Background).
		Border(1, t.Border).
		Shadow(0, 8, 24, 0, shadow).
		Label(label)

	ui.Column(c).WidthPercent(100).Gap(t.Space(2.5)).Children(func() {
		title := a.wordHover.activeWord
		phonetic := ""
		if a.wordHover.preview != nil {
			if a.wordHover.preview.Word != "" {
				title = a.wordHover.preview.Word
			}
			phonetic = a.wordHover.preview.Phonetic
		}
		ui.Row(c).WidthPercent(100).Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(t.Space(0.5)).Children(func() {
				ui.Text(c, title).FontSize(17).FontWeight(600).SingleLine()
				if phonetic != "" {
					ui.Text(c, phonetic).FontSize(12).TextColor(t.TextMuted).SingleLine()
				}
			})
			if iconButton(c, "speak", a.tr("朗读")).Clicked() {
				a.speakText(a.wordHover.activeWord, "English", a.Data.Settings.Speech)
			}
		})

		switch a.wordHover.status {
		case "loading":
			ui.Row(c).WidthPercent(100).MinHeight(56).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
				ui.Spinner(c).Label(a.tr("正在处理"))
			})
		case "ready":
			if a.wordHover.preview != nil {
				ui.Column(c).WidthPercent(100).Gap(t.Space(2.5)).Children(func() {
					for i, meaning := range a.wordHover.preview.Meanings {
						c.Key(i)
						item := ui.Column(c).WidthPercent(100).Gap(t.Space(1))
						if i > 0 {
							item.BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Padding(t.Space(2.5), 0, 0, 0)
						}
						item.Children(func() {
							ui.RichText(c).WidthPercent(100).FontSize(13).LineHeight(1.45).Children(func() {
								if meaning.PartOfSpeech != "" {
									ui.Text(c, meaning.PartOfSpeech+" ").FontSize(11).FontWeight(500).Italic().TextColor(accentText)
								}
								ui.Text(c, meaning.Definition)
							})
							if meaning.Example != "" {
								ui.Text(c, meaning.Example).FontSize(12).Italic().TextColor(t.TextMuted).WidthPercent(100)
							}
						})
					}
				})
			}
		case "error":
			ui.Row(c).WidthPercent(100).MinHeight(56).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
				ui.Text(c, a.tr("暂时无法加载预览")).FontSize(13).TextColor(t.TextMuted)
			})
		default:
			ui.Row(c).WidthPercent(100).MinHeight(56).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
				ui.Text(c, a.tr("未找到简明释义")).FontSize(13).TextColor(t.TextMuted)
			})
		}

		ui.Row(c).WidthPercent(100).Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
			openLabel := a.tr("查看完整释义与例句")
			btn := ui.Button(c, "").Label(openLabel).Padding(t.Space(1), t.Space(2)).Radius(t.Radius).Border(0, ui.Transparent).Background(ui.Transparent).TextColor(accentText).Children(func() {
				ui.Text(c, openLabel).FontSize(12).FontWeight(500).SingleLine()
			})
			if btn.Hovered() {
				btn.Background(t.SurfaceHover)
			}
			if btn.Clicked() {
				a.openWordDetails(a.wordHover.activeWord)
			}
			ui.Spacer(c)
			if a.wordHover.preview != nil && a.wordHover.preview.SourceURL != "" {
				ui.Link(c, a.tr("来源：Wiktionary"), a.wordHover.preview.SourceURL).FontSize(11).TextColor(t.TextMuted).SingleLine()
			}
		})
	})
}
