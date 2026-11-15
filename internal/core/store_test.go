package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestOldSettingsReceiveNewDefaultsWithoutLosingValues(t *testing.T) {
	d := DefaultData()
	d.Settings.Target = "English"
	encoded, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	var old map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &old); err != nil {
		t.Fatal(err)
	}
	var settings map[string]json.RawMessage
	if err := json.Unmarshal(old["settings"], &settings); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"uiLanguage", "defaultTarget", "speech", "proxy", "writing", "defaultAction", "fixedPosition", "languageDetection", "autoCheckUpdates", "selectInputText"} {
		delete(settings, field)
	}
	old["settings"], err = json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err = json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	s := Store{Path: filepath.Join(t.TempDir(), "data.json")}
	if err := os.WriteFile(s.Path, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Settings.Target != "English" || got.Settings.DefaultTarget != "English" || got.Settings.Speech.Rate != 1 || got.Settings.Speech.Volume != 100 || got.Settings.Writing.Target != "English" || !got.Settings.FixedPosition || got.Settings.LanguageDetection != "本地" || !got.Settings.AutoCheckUpdates || !got.Settings.SelectInputText || !reflect.DeepEqual(got.Actions, d.Actions) {
		t.Fatalf("old settings migration %+v", got.Settings)
	}
	got.Settings.Speech.Volume = 0
	if err := s.Save(got); err != nil {
		t.Fatal(err)
	}
	got, err = s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Settings.Speech.Volume != 0 {
		t.Fatal("muted volume overwritten by defaults")
	}
}

func TestMigrateOnlyUneditedLegacyPrompts(t *testing.T) {
	d := DefaultData()
	for i := range d.Actions {
		d.Actions[i].Mode = ""
		d.Actions[i].Prompt = legacyDefaultPrompts[d.Actions[i].ID]
	}
	d.Actions[0].Name = "我的翻译"
	d.Actions[0].Provider = "Ollama"
	d.Actions[0].Model = "custom-model"
	d.Actions[1].Prompt = "User customized polishing prompt"
	d.Actions[2].CommandPrompt = "User command"
	d.Vocabulary = []Word{{Text: "word", Description: "saved description", Reviews: 3}}
	d.History = []Record{{ID: "saved", Text: "saved input", Result: "saved output"}}
	s := Store{Path: filepath.Join(t.TempDir(), "data.json")}
	if err := s.Save(d); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Actions[0].Mode != "translate" || got.Actions[0].Prompt != "" || got.Actions[0].Name != "我的翻译" || got.Actions[0].Model != "custom-model" || got.Actions[0].Provider != "Ollama" {
		t.Fatalf("default migration %+v", got.Actions[0])
	}
	if got.Actions[1] != d.Actions[1] || got.Actions[2] != d.Actions[2] {
		t.Fatal("customized actions were overwritten")
	}
	if !reflect.DeepEqual(got.Settings, d.Settings) || !reflect.DeepEqual(got.History, d.History) || !reflect.DeepEqual(got.Vocabulary, d.Vocabulary) {
		t.Fatal("migration changed configuration or saved data")
	}
	if got.Actions[5].Mode != "big-bang" || got.Actions[5].ArticlePrompt != "an insteresting story" {
		t.Fatal("story mode not migrated")
	}
	if err := s.Save(got); err != nil {
		t.Fatal(err)
	}
	again, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, again) {
		t.Fatal("migration is not idempotent")
	}
}

func TestStoreRoundTripAndReplace(t *testing.T) {
	s := Store{Path: filepath.Join(t.TempDir(), "data.json")}
	d, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	d.History = []Record{{ID: "1", Text: "你好", Result: "Hello", CreatedAt: time.Now()}}
	if err = s.Save(d); err != nil {
		t.Fatal(err)
	}
	d.History[0].Favorite = true
	if err = s.Save(d); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.History) != 1 || !got.History[0].Favorite || got.History[0].Text != "你好" {
		t.Fatalf("history %+v", got.History)
	}
	entries, err := os.ReadDir(filepath.Dir(s.Path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("temporary files remain: %v", entries)
	}
}

func TestCorruptDataIsNotOverwritten(t *testing.T) {
	s := Store{Path: filepath.Join(t.TempDir(), "data.json")}
	if err := os.WriteFile(s.Path, []byte("{corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(); err == nil {
		t.Fatal("expected corruption error")
	}
	b, err := os.ReadFile(s.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "{corrupt" {
		t.Fatal("corrupt data was overwritten")
	}
}
