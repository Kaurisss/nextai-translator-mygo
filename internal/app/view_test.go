package app

import (
	"context"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Kaurisss/nextai-translator-mygo/internal/core"
	"github.com/egoist/mygo/ui"
)

func testApp(t *testing.T) *App {
	t.Helper()
	a := New(core.DefaultData(), core.Store{Path: filepath.Join(t.TempDir(), "data.json")})
	a.login = &testLoginItem{}
	return a
}

func click(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	if err := tt.Click(label); err != nil {
		t.Fatal(err)
	}
}

func TestNavigationAndMinimumLayout(t *testing.T) {
	a := testApp(t)
	for _, size := range [][2]int{{560, 880}, {500, 560}} {
		tt := ui.NewTester(a.View, size[0], size[1])
		for _, label := range []string{"原文输入", "源语言", "目标语言", "翻译", "更多", "设置", "历史记录"} {
			b, ok := tt.Find(label)
			if !ok || b.W <= 0 || b.H <= 0 {
				t.Errorf("%s invisible at %v: %+v", label, size, b)
			}
			if b.X < 0 || b.X+b.W > float32(size[0])+1 || b.Y < 0 || b.Y+b.H > float32(size[1])+1 {
				t.Errorf("%s outside viewport %v: %+v", label, size, b)
			}
		}
		click(t, tt, "历史记录")
		if !tt.HasText("还没有匹配的翻译记录") {
			t.Fatal(tt.Texts())
		}
		click(t, tt, "开始翻译")
		click(t, tt, "生词本")
		if !tt.HasText("查词后可将结果加入生词本") {
			t.Fatal(tt.Texts())
		}
		click(t, tt, "设置")
		if _, ok := tt.Access("API Key"); !ok {
			t.Fatal("API Key missing")
		}
		click(t, tt, "返回翻译")
		tt.Advance(time.Second)
		if tt.FramePending() {
			t.Error("idle view does not settle")
		}
	}
}

func TestReferenceLayoutAndSVGButtonNames(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.View, 560, 880)
	source, _ := tt.Find("原文输入")
	result, _ := tt.Find("翻译结果")
	settings, _ := tt.Find("设置")
	if source.W < 500 || source.H < 180 || result.Y <= source.Y+source.H || settings.Y <= result.Y+result.H {
		t.Fatalf("unexpected stacked layout: input %+v output %+v footer %+v", source, result, settings)
	}
	for _, label := range []string{"交换", "导入图片", "生词本", "更多", "设置", "历史记录"} {
		info, ok := tt.Access(label)
		if !ok || info.Name != label {
			t.Fatalf("unnamed SVG button: %q %+v", label, info)
		}
	}
	click(t, tt, "更多")
	click(t, tt, "动作管理")
	tt.Key(0, ui.KeyEscape)
	if a.page != "translator" {
		t.Fatal("Escape did not return to translation")
	}
}

func TestEmptyStateCenteredInAvailableContent(t *testing.T) {
	for _, page := range []struct{ name, search, text, action string }{
		{"history", "搜索记录", "还没有匹配的翻译记录", "开始翻译"},
		{"vocabulary", "搜索生词", "查词后可将结果加入生词本", "开始查词"},
	} {
		for _, size := range [][2]int{{560, 880}, {500, 560}} {
			a := testApp(t)
			a.page = page.name
			tt := ui.NewTester(a.View, size[0], size[1])
			search, _ := tt.Find(page.search)
			text, ok := tt.Find(page.text)
			if !ok {
				t.Fatal("empty state text missing")
			}
			button, _ := tt.Find(page.action)
			footer, _ := tt.Find("设置")
			// The group includes the 48px icon and its 12px gap above the text.
			center := (text.Y - 60 + button.Y + button.H) / 2
			availableCenter := (search.Y + search.H + footer.Y) / 2
			if delta := center - availableCenter; delta < -20 || delta > 20 {
				t.Errorf("%s at %v: empty state center %v, available center %v", page.name, size, center, availableCenter)
			}
		}
	}
}

func TestNoticeDoesNotShiftTranslationLayout(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.View, 560, 880)
	labels := []string{"原文输入", "翻译结果", "设置"}
	before := make(map[string]ui.Rect)
	for _, label := range labels {
		before[label], _ = tt.Find(label)
	}
	a.notice = "已完成并保存到历史记录"
	tt.Frame()
	if !tt.HasText("已完成并保存到历史记录") || a.notice != "" {
		t.Fatal("notice was not consumed as a toast")
	}
	for _, label := range labels {
		after, ok := tt.Find(label)
		if !ok || after != before[label] {
			t.Errorf("%s shifted: before %+v after %+v", label, before[label], after)
		}
	}
	tt.Advance(5 * time.Second)
	if tt.HasText("已完成并保存到历史记录") {
		t.Fatal("notice toast did not expire")
	}
}

func TestNarrowLanguagePickerSearchAndSwap(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.View, 500, 560)
	click(t, tt, "源语言")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("English")
	tt.Key(0, ui.KeyEnter)
	if a.Data.Settings.Source != "English" {
		t.Fatalf("source language %q", a.Data.Settings.Source)
	}
	click(t, tt, "交换")
	if a.Data.Settings.Source != "简体中文" || a.Data.Settings.Target != "English" {
		t.Fatalf("swap %+v", a.Data.Settings)
	}
	if got, ok := tt.Access("目标语言"); !ok || got.Value != "English" {
		t.Fatalf("editor not updated after swap: %+v", got)
	}
}

func TestLanguageChevronStaysInsideField(t *testing.T) {
	a := testApp(t)
	selected := "English"
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Height(32).Width(110).Children(func() {
			a.languagePicker(c, &selected, []string{"English", "简体中文"}, "源语言").Label("测试选择器")
		})
	}, 110, 240)
	field, ok := tt.Find("测试选择器")
	if !ok {
		t.Fatal("field missing")
	}
	arrow, ok := tt.Find("展开源语言")
	if !ok {
		t.Fatal("chevron missing")
	}
	if arrow.X <= field.X || arrow.X+arrow.W > field.X+field.W-7 || arrow.Y < field.Y || arrow.Y+arrow.H > field.Y+field.H {
		t.Fatalf("chevron outside inset: field %+v arrow %+v", field, arrow)
	}
	click(t, tt, "展开源语言")
	click(t, tt, "简体中文")
	if selected != "简体中文" {
		t.Fatal("chevron did not open an interactive language popup")
	}
}

func TestSettingsNavigationDoesNotMoveWindowTitle(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.View, 560, 880)
	before, _ := tt.Find("NextAI Translator")
	click(t, tt, "设置")
	after, _ := tt.Find("NextAI Translator")
	back, _ := tt.Find("返回翻译")
	if before.X != after.X || before.Y != after.Y {
		t.Fatalf("window title moved: before %+v after %+v", before, after)
	}
	if back.Y < 32 {
		t.Fatalf("back button intrudes into titlebar: %+v", back)
	}
	click(t, tt, "返回翻译")
	if a.page != "translator" {
		t.Fatal("back navigation failed")
	}
}

func TestSettingsLabelsAndControlsShareAlignment(t *testing.T) {
	for _, size := range [][2]int{{560, 880}, {500, 560}} {
		a := testApp(t)
		a.page = "settings"
		tt := ui.NewTester(a.View, size[0], size[1])
		var captionX float32
		for i, label := range []string{"主题", "默认服务提供商", "API 密钥", "无模型 API 支持", "API 模型", "API URL", "API URL 路径", "默认动作", "默认目标语言"} {
			b, ok := tt.Find(label)
			if !ok {
				t.Fatalf("label %q missing", label)
			}
			if i == 0 {
				captionX = b.X
			}
			if b.X != captionX {
				t.Errorf("label %s starts at %v, expected %v", label, b.X, captionX)
			}
		}
		endpoint, _ := tt.AccessByRole(ui.RoleTextField, "API 地址")
		path, _ := tt.AccessByRole(ui.RoleTextField, "API URL 路径")
		for _, box := range []ui.Rect{path.Bounds} {
			if box.X != endpoint.Bounds.X || box.W != endpoint.Bounds.W {
				t.Errorf("unaligned input: endpoint %+v control %+v", endpoint.Bounds, box)
			}
		}
		click(t, tt, "API 密钥")
		if !tt.Focused("API Key") {
			t.Fatal("caption did not focus its input")
		}
	}
}

func TestCopyClearAndVocabulary(t *testing.T) {
	a := testApp(t)
	a.text = "hello"
	a.result = "你好"
	tt := ui.NewTester(a.View, 1080, 760)
	click(t, tt, "复制")
	if tt.Clipboard() != "你好" {
		t.Fatalf("clipboard %q", tt.Clipboard())
	}
	click(t, tt, "收藏单词")
	if len(a.Data.Vocabulary) != 1 {
		t.Fatal("word not saved")
	}
	click(t, tt, "收藏单词")
	if len(a.Data.Vocabulary) != 1 {
		t.Fatal("duplicate word")
	}
	click(t, tt, "清空")
	if a.text != "" || a.result != "" {
		t.Fatal("not cleared")
	}
	click(t, tt, "生词本")
	click(t, tt, "已复习")
	if a.Data.Vocabulary[0].Reviews != 1 {
		t.Fatal("review not recorded")
	}
}

type fakeEngine struct{}

func (fakeEngine) Stream(ctx context.Context, q core.Query, emit func(string)) error {
	emit("你")
	emit("好")
	return nil
}

func TestTranslateFromUIAndRestoreHistory(t *testing.T) {
	a := testApp(t)
	a.Engine = fakeEngine{}
	a.text = "Hello"
	queue := make(chan func(), 16)
	a.post = func(fn func()) { queue <- fn }
	tt := ui.NewTester(a.View, 1080, 760)
	click(t, tt, "提交")
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	for a.busy {
		select {
		case fn := <-queue:
			fn()
		case <-deadline.C:
			t.Fatal("translation did not finish")
		}
	}
	tt.Frame()
	if a.result != "你好" || len(a.Data.History) != 1 {
		t.Fatalf("result %q history %v", a.result, a.Data.History)
	}
	click(t, tt, "历史记录")
	click(t, tt, "打开")
	if a.page != "translator" || a.text != "Hello" || a.result != "你好" {
		t.Fatal("history not restored")
	}
}

func TestSaveCustomAction(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.View, 1080, 760)
	click(t, tt, "更多")
	click(t, tt, "动作管理")
	click(t, tt, "新建动作")
	click(t, tt, "动作名称")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("校对")
	click(t, tt, "动作提示词")
	tt.Type("Proofread in {{target}}")
	click(t, tt, "保存动作")
	if len(a.Data.Actions) != 7 || a.Data.Actions[6].Name != "校对" {
		t.Fatalf("actions %+v", a.Data.Actions)
	}
}

func TestEditingBuiltinPromptCreatesCustomAction(t *testing.T) {
	a := testApp(t)
	a.page = "actions"
	tt := ui.NewTester(a.View, 560, 880)
	if a.editAction.Mode != "translate" {
		t.Fatal("rendering changed the builtin action")
	}
	click(t, tt, "动作提示词")
	tt.Type("Use my custom instructions in ${targetLang}.")
	if a.editAction.Mode != "" {
		t.Fatal("edited builtin still ignores its custom prompt")
	}
	click(t, tt, "保存动作")
	if a.Data.Actions[0].Mode != "" || a.Data.Actions[0].Prompt != "Use my custom instructions in ${targetLang}." {
		t.Fatalf("custom override not saved: %+v", a.Data.Actions[0])
	}
}

type delayedEngine struct{ release <-chan struct{} }

func (f delayedEngine) Stream(_ context.Context, _ core.Query, emit func(string)) error {
	<-f.release
	emit("过时的结果")
	return nil
}

func TestCanceledRequestCannotOverwriteNewText(t *testing.T) {
	a := testApp(t)
	release := make(chan struct{})
	a.Engine = delayedEngine{release: release}
	queue := make(chan func(), 16)
	a.post = func(fn func()) { queue <- fn }
	a.text = "old"
	tt := ui.NewTester(a.View, 1080, 760)
	click(t, tt, "提交")
	click(t, tt, "停止")
	a.text, a.result = "new", "新的结果"
	close(release)
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	for i := 0; i < 2; i++ {
		select {
		case fn := <-queue:
			fn()
		case <-deadline.C:
			t.Fatal("worker did not return")
		}
	}
	tt.Frame()
	if a.result != "新的结果" || len(a.Data.History) != 0 {
		t.Fatalf("stale work committed: %q %+v", a.result, a.Data.History)
	}
}

func TestStackedLayoutAndLanguageSwap(t *testing.T) {
	a := testApp(t)
	a.text = "Hello"
	a.result = "你好"
	a.Data.Settings.Source = "English"
	tt := ui.NewTester(a.View, 500, 560)
	click(t, tt, "交换")
	if a.Data.Settings.Target != "English" || a.Data.Settings.Source != "简体中文" || a.text != "Hello" || a.result != "你好" {
		t.Fatal("languages did not swap")
	}
	source, _ := tt.Find("原文输入")
	result, _ := tt.Find("翻译结果")
	if result.Y <= source.Y {
		t.Fatalf("panes not stacked: source %+v result %+v", source, result)
	}
}

func TestSwapLanguagesPreservesBothTextAreas(t *testing.T) {
	for _, result := range []string{"", "Hello"} {
		t.Run(result, func(t *testing.T) {
			a := testApp(t)
			a.text = "你好"
			a.result = result
			a.Data.Settings.Source = "简体中文"
			a.Data.Settings.Target = "English"
			tt := ui.NewTester(a.View, 560, 880)
			click(t, tt, "交换")
			if a.text != "你好" || a.result != result {
				t.Fatalf("swap moved text: input %q result %q", a.text, a.result)
			}
			if a.Data.Settings.Source != "English" || a.Data.Settings.Target != "简体中文" {
				t.Fatalf("languages not swapped: %+v", a.Data.Settings)
			}
			input, ok := tt.Access("原文输入")
			if !ok || input.Value != "你好" {
				t.Fatalf("input editor changed: %+v", input)
			}
		})
	}
}

func TestDeleteActionCanBeUndone(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.View, 1080, 760)
	click(t, tt, "更多")
	click(t, tt, "动作管理")
	click(t, tt, "删除动作")
	if len(a.Data.Actions) != 5 {
		t.Fatal("action not removed")
	}
	click(t, tt, "撤销")
	if len(a.Data.Actions) != 6 {
		t.Fatal("action not restored")
	}
	click(t, tt, "模型与输出")
	tt.Advance(time.Second)
	if _, ok := tt.Access("动作模型"); !ok {
		t.Fatal("advanced action model missing")
	}
}

type captureEngine struct{ queries chan core.Query }

func (f captureEngine) Stream(_ context.Context, q core.Query, emit func(string)) error {
	f.queries <- q
	emit("plain result")
	return nil
}

func TestSelectingActionDoesNotSubmit(t *testing.T) {
	a := testApp(t)
	a.text = "你好"
	a.result = "existing result"
	queries := make(chan core.Query, 4)
	a.Engine = captureEngine{queries: queries}
	a.post = func(func()) { t.Error("selecting an action started asynchronous work") }
	tt := ui.NewTester(a.View, 560, 880)
	for _, name := range []string{"润色", "总结", "分析", "翻译", "翻译"} {
		click(t, tt, name)
		if a.action != name || a.busy || a.text != "你好" || a.result != "existing result" {
			t.Fatalf("action selection changed translation: %s busy=%v input=%q output=%q", a.action, a.busy, a.text, a.result)
		}
	}
	source, _ := tt.Find("原文输入")
	submit, ok := tt.Find("提交")
	if !ok || submit.Y < source.Y+source.H || submit.X < source.X+source.W/2 {
		t.Fatalf("submit is not below input at trailing edge: input %+v button %+v", source, submit)
	}
}

func TestEnterSubmitsAndShiftEnterAddsNewline(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mods   ui.Modifiers
		submit bool
	}{{"Enter", 0, true}, {"CtrlEnter", ui.Cmd, true}, {"ShiftEnter", ui.Shift, false}} {
		t.Run(tc.name, func(t *testing.T) {
			a := testApp(t)
			a.text = "你好"
			queries := make(chan core.Query, 1)
			a.Engine = captureEngine{queries: queries}
			queue := make(chan func(), 16)
			a.post = func(fn func()) { queue <- fn }
			tt := ui.NewTester(a.View, 560, 880)
			click(t, tt, "原文输入")
			tt.Key(ui.Cmd, ui.KeyEnd)
			tt.Key(tc.mods, ui.KeyEnter)
			if !tc.submit {
				if a.busy || a.text != "你好\n" {
					t.Fatalf("Shift+Enter did not insert newline: %q busy=%v", a.text, a.busy)
				}
				return
			}
			deadline := time.NewTimer(3 * time.Second)
			defer deadline.Stop()
			for a.busy {
				select {
				case fn := <-queue:
					fn()
				case <-deadline.C:
					t.Fatal("request timed out")
				}
			}
			select {
			case q := <-queries:
				if q.Text != "你好" {
					t.Fatalf("Enter inserted a newline before submitting: %q", q.Text)
				}
			default:
				t.Fatal("Enter did not submit")
			}
		})
	}
}

func TestActionOverridesGlobalModel(t *testing.T) {
	a := testApp(t)
	a.text = "hello"
	a.Data.Actions[0].Provider = "Claude"
	a.Data.Actions[0].Model = "action-model"
	a.Data.Actions[0].Format = "纯文本"
	a.keyLoaded["Claude"] = true
	a.keys["Claude"] = "claude-key"
	queries := make(chan core.Query, 1)
	a.Engine = captureEngine{queries: queries}
	queue := make(chan func(), 16)
	a.post = func(fn func()) { queue <- fn }
	tt := ui.NewTester(a.View, 1080, 760)
	click(t, tt, "提交")
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	for a.busy {
		select {
		case fn := <-queue:
			fn()
		case <-deadline.C:
			t.Fatal("translation timeout")
		}
	}
	q := <-queries
	if q.Profile.Provider != "Claude" || q.Profile.Model != "action-model" || q.Key != "claude-key" {
		t.Fatalf("override %+v", q)
	}
	tt.Frame()
	if _, ok := tt.Access("纯文本结果"); !ok {
		t.Fatal("plain result renderer missing")
	}
	if a.Data.History[0].Format != "纯文本" {
		t.Fatal("result format not persisted")
	}
}

func TestDropImageStartsOCRAndRejectsNonImages(t *testing.T) {
	a := testApp(t)
	queue := make(chan func(), 8)
	a.post = func(fn func()) { queue <- fn }
	tt := ui.NewTester(a.View, 560, 880)

	txtPath := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(txtPath, []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	if !tt.DropFiles(200, 200, txtPath) {
		t.Fatal("window did not accept drop")
	}
	if a.systemBusy || a.failure == "" {
		t.Fatalf("unexpected state for non-image drop: busy=%v failure=%q", a.systemBusy, a.failure)
	}

	imgPath := filepath.Join(t.TempDir(), "hello.png")
	rendered := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Fill().Background(ui.RGB(255, 255, 255)).Padding(24).Children(func() {
			ui.Text(c, "Hello OCR").FontSize(32).TextColor(ui.RGB(0, 0, 0))
		})
	}, 320, 120)
	f, err := os.Create(imgPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, rendered.Image()); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	queries := make(chan core.Query, 1)
	a.Engine = captureEngine{queries: queries}
	if !tt.DropFiles(200, 200, imgPath) {
		t.Fatal("window did not accept image drop")
	}
	if !a.systemBusy || a.failure != "" {
		t.Fatalf("image drop did not start OCR: busy=%v failure=%q", a.systemBusy, a.failure)
	}
	if tt.DropFiles(200, 200, imgPath) {
		t.Fatal("accepted another drop while OCR was busy")
	}
	select {
	case fn := <-queue:
		fn()
	case <-time.After(15 * time.Second):
		t.Fatal("OCR worker did not finish")
	}
	if a.systemBusy {
		t.Fatal("systemBusy remained set after OCR")
	}
	if runtime.GOOS != "windows" {
		return
	}
	select {
	case q := <-queries:
		if q.Text == "" {
			t.Fatal("dropped image OCR produced empty query")
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("translation did not start after drop OCR (text=%q failure=%q)", a.text, a.failure)
	}
}

func TestRenderArtifact(t *testing.T) {
	dir := os.Getenv("NEXTAI_SCREENSHOT_DIR")
	if dir == "" {
		t.Skip("set NEXTAI_SCREENSHOT_DIR to export screenshots")
	}
	a := testApp(t)
	a.text = "The limits of my language mean the limits of my world."
	a.result = "语言的边界，就是世界的边界。\n\n— 路德维希·维特根斯坦"
	for _, dark := range []bool{false, true} {
		tt := ui.NewTester(a.View, 560, 880)
		tt.SetDark(dark)
		name := "native-light.png"
		if dark {
			name = "native-dark.png"
		}
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	for _, page := range []string{"settings", "history", "vocabulary"} {
		a.page = page
		tt := ui.NewTester(a.View, 560, 880)
		f, err := os.Create(filepath.Join(dir, "native-"+page+".png"))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTranslationResultPreservesSingleLineBreaks(t *testing.T) {
	a := testApp(t)
	a.result = "单行文本"
	single := ui.NewTester(a.View, 560, 880)
	oneLine, ok := single.Find("单行文本")
	if !ok {
		t.Fatalf("missing single-line result: %v", single.Texts())
	}

	a.result = "第一行\n第二行\n第三行"
	tt := ui.NewTester(a.View, 560, 880)
	multiLine, ok := tt.Find("第一行\n第二行\n第三行")
	if !ok {
		t.Fatalf("missing rendered lines: %v", tt.Texts())
	}
	if multiLine.H < oneLine.H*2.2 {
		t.Fatalf("expected 3-line height >= %.1f, got single=%+v multi=%+v", oneLine.H*2.2, oneLine, multiLine)
	}

	sample := "hello\nworld\n\n| a | b |\n| - | - |\n```go\nx := 1\ny := 2\n```\nafter\n"
	full := markdownHardBreaks(sample)
	want := "hello  \nworld  \n\n| a | b |\n| - | - |\n```go\nx := 1\ny := 2\n```\nafter  \n"
	if full != want {
		t.Fatalf("got %q, want %q", full, want)
	}
}
