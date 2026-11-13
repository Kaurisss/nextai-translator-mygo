package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Store struct{ Path string }

func (s Store) Load() (Data, error) {
	b, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return DefaultData(), nil
	}
	if err != nil {
		return Data{}, err
	}
	d := Data{Settings: DefaultData().Settings}
	d.Settings.Profiles = nil
	d.Settings.DefaultTarget = ""
	// Decode old files over defaults so newly added settings keep their defaults.
	if err := json.Unmarshal(b, &d); err != nil {
		return Data{}, fmt.Errorf("读取本地数据: %w", err)
	}
	if d.Settings.DefaultTarget == "" {
		d.Settings.DefaultTarget = d.Settings.Target
	}
	if d.Settings.Profiles == nil || len(d.Actions) == 0 {
		return Data{}, errors.New("本地数据缺少模型配置或动作")
	}
	if _, ok := d.Settings.Profiles[d.Settings.Provider]; !ok {
		return Data{}, errors.New("当前模型服务配置不存在")
	}
	if _, ok := FindProvider(d.Settings.Provider); !ok {
		return Data{}, errors.New("当前模型服务不受支持")
	}
	for name, p := range d.Settings.Profiles {
		if p.Provider != name {
			return Data{}, fmt.Errorf("模型配置名称不一致: %s", name)
		}
	}
	ids, names := map[string]bool{}, map[string]bool{}
	for i := range d.Actions {
		a := &d.Actions[i]
		if old, ok := legacyDefaultPrompts[a.ID]; ok && a.Mode == "" && a.Prompt == old && a.CommandPrompt == "" {
			a.Mode = a.ID
			a.Prompt = ""
			if a.Mode == "big-bang" {
				a.ArticlePrompt = "an insteresting story"
			}
		}
		if a.ID == "" || a.Name == "" || (a.Mode == "" && a.Prompt == "" && a.CommandPrompt == "") || ids[a.ID] || names[a.Name] {
			return Data{}, errors.New("动作数据为空或重复")
		}
		if a.Mode != "" && !validBuiltinMode(a.Mode) {
			return Data{}, fmt.Errorf("未知内置动作: %s", a.Mode)
		}
		if a.Provider != "" && a.Provider != "跟随全局" {
			if _, ok := FindProvider(a.Provider); !ok {
				return Data{}, fmt.Errorf("动作模型服务不受支持: %s", a.Provider)
			}
		}
		ids[a.ID] = true
		names[a.Name] = true
	}
	ids = map[string]bool{}
	for _, r := range d.History {
		if r.ID == "" || ids[r.ID] {
			return Data{}, errors.New("历史记录 ID 为空或重复")
		}
		ids[r.ID] = true
	}
	words := map[string]bool{}
	for _, w := range d.Vocabulary {
		if w.Text == "" || words[w.Text] {
			return Data{}, errors.New("生词为空或重复")
		}
		words[w.Text] = true
	}
	for name, p := range DefaultData().Settings.Profiles {
		if _, ok := d.Settings.Profiles[name]; !ok {
			d.Settings.Profiles[name] = p
		}
	}
	return d, nil
}

func (s Store) Save(d Data) error {
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(s.Path), ".data-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err := f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(name, s.Path)
}
