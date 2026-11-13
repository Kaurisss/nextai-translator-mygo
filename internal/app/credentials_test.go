package app

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
)

type memoryCredentials struct {
	values   map[string]string
	failNext bool
}

func (s *memoryCredentials) Get(key string) ([]byte, error) {
	value, ok := s.values[key]
	if !ok {
		return nil, mygo.ErrSecretNotFound
	}
	return []byte(value), nil
}
func (s *memoryCredentials) Set(key string, value []byte) error {
	if s.failNext {
		s.failNext = false
		return errors.New("test credential write failed")
	}
	s.values[key] = string(value)
	return nil
}
func (s *memoryCredentials) Delete(key string) error { delete(s.values, key); return nil }

func finishSettingsSave(t *testing.T, a *App, updates <-chan func()) {
	t.Helper()
	timeout := time.NewTimer(3 * time.Second)
	defer timeout.Stop()
	for a.systemBusy {
		select {
		case update := <-updates:
			update()
		case <-timeout.C:
			t.Fatal("settings worker did not finish")
		}
	}
}

func TestSettingsSaveStoresSecretsOutsideJSON(t *testing.T) {
	a := testApp(t)
	a.Data.Settings.Shortcut = ""
	a.settingsDraft.Shortcut = ""
	a.credentials = &memoryCredentials{values: map[string]string{}}
	updates := make(chan func(), 8)
	a.post = func(update func()) { updates <- update }
	a.apiKey = "private-api-key"
	a.proxyPassword = "private-proxy-password"
	a.profile.Model = "custom-model"
	a.saveSettings()
	finishSettingsSave(t, a, updates)
	if a.failure != "" || a.Data.Settings.Profiles["OpenAI"].Model != "custom-model" {
		t.Fatalf("settings save failed: %s", a.failure)
	}
	contents, err := os.ReadFile(a.Store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contents), "private-") {
		t.Fatal("credentials leaked to saved settings")
	}
	if a.credentials.(*memoryCredentials).values["OpenAI"] != "private-api-key" || a.keys["OpenAI"] != "private-api-key" {
		t.Fatal("key not committed to credential store and memory")
	}
}

func TestSettingsFileFailureRestoresCredentials(t *testing.T) {
	a := testApp(t)
	a.Data.Settings.Shortcut = ""
	a.settingsDraft.Shortcut = ""
	before := a.Data.Settings.Clone()
	credentials := &memoryCredentials{values: map[string]string{"OpenAI": "old-key", "proxy-password": "old-password"}}
	a.credentials = credentials
	a.apiKey = "new-key"
	a.proxyPassword = "new-password"
	a.Store.Path = filepath.Join(t.TempDir(), "directory")
	if err := os.Mkdir(a.Store.Path, 0700); err != nil {
		t.Fatal(err)
	}
	updates := make(chan func(), 8)
	a.post = func(update func()) { updates <- update }
	a.profile.Model = "new-model"
	a.saveSettings()
	finishSettingsSave(t, a, updates)
	if a.failure == "" || !reflect.DeepEqual(a.Data.Settings, before) {
		t.Fatal("failed save changed applied settings")
	}
	if credentials.values["OpenAI"] != "old-key" || credentials.values["proxy-password"] != "old-password" {
		t.Fatal("failed settings save did not restore credentials")
	}
}

func TestCredentialBatchFailureRestoresPreviousValues(t *testing.T) {
	store := &memoryCredentials{values: map[string]string{"provider": "old-key"}, failNext: true}
	if _, err := updateCredentials(store, map[string]string{"provider": "new-key", "proxy-password": "new-password"}); err == nil {
		t.Fatal("credential failure ignored")
	}
	if !reflect.DeepEqual(store.values, map[string]string{"provider": "old-key"}) {
		t.Fatalf("credential batch partially committed: %v", store.values)
	}
}
