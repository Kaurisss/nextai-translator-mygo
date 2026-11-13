package app

import (
	"context"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/Kaurisss/nextai-translator-mygo/internal/core"
)

func TestSettingsTabsKeepSaveAndBackVisible(t *testing.T) {
	for _, size := range [][2]int{{560, 880}, {500, 560}} {
		a := testApp(t)
		a.page = "settings"
		tt := ui.NewTester(a.View, size[0], size[1])
		var saveBefore ui.Rect
		for i, tab := range []string{"通用", "代理", "TTS", "写作", "快捷键"} {
			click(t, tt, tab)
			if a.settingsTab != i {
				t.Fatalf("tab %s was not selected", tab)
			}
			scroll, ok := tt.Find("设置内容")
			if !ok || scroll.X != 0 || scroll.W != float32(size[0]) {
				t.Fatalf("settings scroll not flush with window edges: %+v", scroll)
			}
			save, ok := tt.Find("保存设置")
			if !ok || save.Y+save.H > float32(size[1]) || save.Y < float32(size[1]-100) {
				t.Fatalf("save not fixed at bottom: %+v", save)
			}
			if i == 0 {
				saveBefore = save
			} else if save != saveBefore {
				t.Fatalf("save moved across tabs: %+v %+v", saveBefore, save)
			}
			if _, ok := tt.Find("返回翻译"); !ok {
				t.Fatal("back missing")
			}
			a.failure = "请检查设置"
			tt.Frame()
			withError, _ := tt.Find("保存设置")
			if withError != save {
				t.Fatalf("validation error moved save: %+v %+v", save, withError)
			}
			a.failure = ""
			tt.Frame()
		}
		click(t, tt, "返回翻译")
		if a.page != "translator" {
			t.Fatal("back failed")
		}
	}
}

func TestSettingsRenderArtifact(t *testing.T) {
	dir := os.Getenv("NEXTAI_SCREENSHOT_DIR")
	if dir == "" {
		t.Skip("set NEXTAI_SCREENSHOT_DIR to export settings pages")
	}
	a := testApp(t)
	a.page = "settings"
	for i, name := range []string{"general", "proxy", "tts", "writing", "shortcuts"} {
		a.settingsTab = i
		tt := ui.NewTester(a.View, 560, 880)
		f, err := os.Create(filepath.Join(dir, "native-settings-"+name+".png"))
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

func TestInterfaceLanguageKeepsStoredValues(t *testing.T) {
	a := testApp(t)
	a.Data.Settings.UILanguage = "en"
	a.settingsDraft.UILanguage = "en"
	a.page = "settings"
	tt := ui.NewTester(a.View, 560, 880)
	for _, label := range []string{"Settings", "General", "Proxy", "TTS", "Writing", "Shortcuts", "API Model"} {
		if !tt.HasText(label) {
			t.Errorf("English label missing: %s", label)
		}
	}
	click(t, tt, "Proxy")
	if a.settingsTab != 1 {
		t.Fatal("localized tab failed")
	}
	click(t, tt, "Back to translator")
	if a.page != "translator" || a.Data.Settings.Target != "简体中文" || a.action != "翻译" {
		t.Fatal("translation changed stored values")
	}
	if _, ok := tt.Find("Source text"); !ok {
		t.Fatal("main view was not localized")
	}
	value := "浅色"
	selector := ui.NewTester(func(c *ui.Context) { a.selectSetting(c, &value, []string{"浅色", "深色"}).Label("Theme") }, 300, 240)
	click(t, selector, "Theme")
	click(t, selector, "Dark")
	if value != "深色" {
		t.Fatalf("localized value persisted as %q", value)
	}
}

func TestSettingsEditsRemainDraftAndRecorderCapturesKeys(t *testing.T) {
	a := testApp(t)
	a.page = "settings"
	tt := ui.NewTester(a.View, 560, 880)
	click(t, tt, "代理")
	click(t, tt, "代理服务器")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("proxy.example")
	if a.settingsDraft.Proxy.Server != "proxy.example" || a.Data.Settings.Proxy.Server != "127.0.0.1" {
		t.Fatal("proxy draft changed live settings")
	}
	click(t, tt, "快捷键")
	click(t, tt, "显示窗口快捷键")
	if a.recordingShortcut != "显示窗口快捷键" {
		t.Fatal("recorder did not start")
	}
	tt.Key(ui.Ctrl|ui.Shift, ui.KeyW)
	if a.settingsDraft.DisplayShortcut != "Ctrl+Shift+W" || a.recordingShortcut != "" {
		t.Fatalf("recorded %q, active %q", a.settingsDraft.DisplayShortcut, a.recordingShortcut)
	}
	click(t, tt, "清除显示窗口快捷键")
	if a.settingsDraft.DisplayShortcut != "" {
		t.Fatal("shortcut clear failed")
	}
	click(t, tt, "取词快捷键")
	tt.Key(0, ui.KeyEscape)
	if a.page != "settings" || a.recordingShortcut != "" {
		t.Fatal("Escape left settings while recording")
	}
}

func TestSpeechVoiceAddAndRemove(t *testing.T) {
	a := testApp(t)
	a.page = "settings"
	a.settingsTab = 2
	tt := ui.NewTester(a.View, 560, 880)
	click(t, tt, "添加音色")
	click(t, tt, "添加音色")
	if len(a.settingsDraft.Speech.Voices) != 1 || len(a.Data.Settings.Speech.Voices) != 0 {
		t.Fatal("voice add duplicated or modified saved settings")
	}
	click(t, tt, "删除English音色")
	if len(a.settingsDraft.Speech.Voices) != 0 {
		t.Fatal("voice remove failed")
	}
}

func TestAPIModelPickerSelectsFromListAndAllowsCustomInput(t *testing.T) {
	a := testApp(t)
	a.page = "settings"
	a.modelOptions[a.profile.Endpoint] = []string{"gpt-4o-mini", "gpt-4.1", "antigravity/gemini-3.8-flash"}
	tt := ui.NewTester(a.View, 560, 880)

	click(t, tt, "展开模型名称")
	click(t, tt, "gpt-4.1")
	if a.profile.Model != "gpt-4.1" {
		t.Fatalf("expected picked model gpt-4.1, got %q", a.profile.Model)
	}

	click(t, tt, "模型名称")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("custom/my-model")
	click(t, tt, "代理")
	if a.profile.Model != "custom/my-model" {
		t.Fatalf("custom model name lost on blur: %q", a.profile.Model)
	}

	click(t, tt, "通用")
	click(t, tt, "模型名称")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("gemini")
	click(t, tt, "antigravity/gemini-3.8-flash")
	if a.profile.Model != "antigravity/gemini-3.8-flash" {
		t.Fatalf("filtered model pick failed: %q", a.profile.Model)
	}
}

type writingQueryEngine struct {
	queries chan core.Query
	release <-chan struct{}
}

func (e writingQueryEngine) Stream(_ context.Context, q core.Query, emit func(string)) error {
	e.queries <- q
	<-e.release
	emit("translated")
	return nil
}

func TestWritingUsesTargetAndCancelPreventsCommit(t *testing.T) {
	a := testApp(t)
	a.text = "你好"
	a.writing = true
	a.action = "分析"
	queries := make(chan core.Query, 1)
	release := make(chan struct{})
	updates := make(chan func(), 4)
	a.Engine = writingQueryEngine{queries: queries, release: release}
	a.post = func(fn func()) { updates <- fn }
	a.start()
	select {
	case q := <-queries:
		if !q.Writing || q.Target != "English" || q.Action.Mode != "translate" || q.Source != "自动检测" {
			t.Fatalf("writing query %+v", q)
		}
	case <-time.After(time.Second):
		t.Fatal("writing did not start")
	}
	a.stop()
	close(release)
	for range 2 {
		select {
		case update := <-updates:
			update()
		case <-time.After(time.Second):
			t.Fatal("writing worker did not return")
		}
	}
	if a.writing || a.result != "" || len(a.Data.History) != 0 || a.systemBusy {
		t.Fatal("canceled writing committed or attempted writeback")
	}
}

func TestInvalidProxyBlocksRequestsUntilSaved(t *testing.T) {
	d := core.DefaultData()
	d.Settings.Proxy.Enabled = true
	d.Settings.Proxy.Port = "invalid"
	a := New(d, core.Store{})
	a.text = "hello"
	a.start()
	if a.busy || a.failure == "" {
		t.Fatal("invalid proxy was not reported before starting a request")
	}
}

func TestDefaultActionFollowsRenameAndDeleteUndo(t *testing.T) {
	a := testApp(t)
	a.page = "actions"
	a.editAction.Name = "译文"
	tt := ui.NewTester(a.View, 560, 880)
	click(t, tt, "保存动作")
	if a.Data.Settings.DefaultAction != "译文" || a.settingsDraft.DefaultAction != "译文" {
		t.Fatal("rename lost default action")
	}
	click(t, tt, "删除动作")
	if a.Data.Settings.DefaultAction != "润色" || a.settingsDraft.DefaultAction != "润色" {
		t.Fatal("deleted default action not replaced")
	}
	click(t, tt, "撤销")
	if a.Data.Settings.DefaultAction != "译文" || a.settingsDraft.DefaultAction != "译文" {
		t.Fatal("undo lost original default action")
	}
}
