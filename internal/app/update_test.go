package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestNewerVersion(t *testing.T) {
	for _, tc := range []struct {
		tag, current string
		want         bool
		invalid      bool
	}{
		{"v0.0.2", "0.0.1", true, false},
		{"v0.10.0", "0.9.9", true, false},
		{"v0.99.99", "1.0.0", false, false},
		{"v0.0.1", "0.0.1", false, false},
		{"0.0.2", "v0.0.1", true, false},
		{"invalid", "0.0.1", false, true},
	} {
		got, err := newerVersion(tc.tag, tc.current)
		if got != tc.want || (err != nil) != tc.invalid {
			t.Errorf("current %q release %q: got %v, error %v", tc.current, tc.tag, got, err)
		}
	}
}

func TestLatestRelease(t *testing.T) {
	body := `{"tag_name":"v0.3.0","assets":[{"name":"nextai-translator-native-setup-0.3.0-amd64.exe","browser_download_url":"https://example.com/setup.exe","size":42,"digest":"sha256:abcd"}]}`
	var gotUserAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserAgent = r.Header.Get("User-Agent")
		fmt.Fprint(w, body)
	}))
	defer server.Close()
	latest, err := latestRelease(context.Background(), server.Client(), server.URL, "0.2.2")
	if err != nil {
		t.Fatal(err)
	}
	if gotUserAgent != updateUserAgent {
		t.Fatalf("User-Agent = %q, want %q", gotUserAgent, updateUserAgent)
	}
	if latest == nil || latest.Tag != "v0.3.0" || len(latest.Assets) != 1 || latest.Assets[0].Size != 42 || latest.Assets[0].Digest != "sha256:abcd" {
		t.Fatalf("latest release %+v", latest)
	}
	latest, err = latestRelease(context.Background(), server.Client(), server.URL, "0.3.0")
	if err != nil || latest != nil {
		t.Fatalf("up to date: got %+v, error %v", latest, err)
	}
}

func TestUpdateAsset(t *testing.T) {
	const slug = "nextai-translator-native"
	assets := func(names ...string) []releaseAsset {
		result := make([]releaseAsset, len(names))
		for i, name := range names {
			result[i] = releaseAsset{Name: name, URL: "https://example.com/" + name}
		}
		return result
	}
	for _, tc := range []struct {
		goos, goarch string
		assets       []releaseAsset
		want         string
	}{
		{"windows", "amd64", assets(
			slug+"-setup-1.2.3-arm64.exe",
			slug+"-setup-1.2.3-amd64.exe",
			slug+"-1.2.3-arm64.dmg",
		), slug + "-setup-1.2.3-amd64.exe"},
		{"windows", "arm64", assets(
			slug+"-setup-1.2.3-amd64.exe",
			slug+"-setup-1.2.3-arm64.exe",
		), slug + "-setup-1.2.3-arm64.exe"},
		{"windows", "arm64", assets(slug + "-setup-1.2.3-amd64.exe"), slug + "-setup-1.2.3-amd64.exe"},
		{"windows", "amd64", assets(slug + "-setup-1.2.3-x64.exe"), slug + "-setup-1.2.3-x64.exe"},
		{"windows", "amd64", assets(slug + "-1.2.3.exe"), ""},
		{"darwin", "arm64", assets(
			slug+"-1.2.3-amd64.dmg",
			slug+"-1.2.3-arm64.dmg",
			slug+"-setup-1.2.3-amd64.exe",
		), slug + "-1.2.3-arm64.dmg"},
		{"darwin", "amd64", assets(slug + "-1.2.3.dmg"), slug + "-1.2.3.dmg"},
		{"darwin", "arm64", assets(slug + "-1.2.3-amd64.dmg"), ""},
		{"linux", "amd64", assets(
			slug+"-1.2.3-linux-arm64.tar.gz",
			slug+"-1.2.3-linux-amd64.tar.gz",
			slug+"_1.2.3_amd64.deb",
		), slug + "-1.2.3-linux-amd64.tar.gz"},
		{"linux", "amd64", assets(slug + "_1.2.3_amd64.deb"), ""},
		{"windows", "amd64", []releaseAsset{{Name: slug + "-setup-1.2.3-amd64.exe", URL: ""}}, ""},
		{"windows", "amd64", nil, ""},
	} {
		got, ok := updateAsset(tc.assets, tc.goos, tc.goarch)
		if tc.want == "" {
			if ok {
				t.Errorf("%s/%s: got %q, want none", tc.goos, tc.goarch, got.Name)
			}
			continue
		}
		if !ok || got.Name != tc.want {
			t.Errorf("%s/%s: got %q (found %v), want %q", tc.goos, tc.goarch, got.Name, ok, tc.want)
		}
	}
}

func TestDownloadUpdate(t *testing.T) {
	payload := []byte("installer-payload")
	sum := sha256.Sum256(payload)
	digest := "sha256:" + hex.EncodeToString(sum[:])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(payload)
	}))
	defer server.Close()
	dir := t.TempDir()
	var progress []int64
	path, err := downloadUpdate(context.Background(), server.Client(), releaseAsset{
		Name:   "setup.exe",
		URL:    server.URL,
		Size:   int64(len(payload)),
		Digest: digest,
	}, dir, func(downloaded, total int64) {
		progress = append(progress, downloaded)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(progress) == 0 || progress[len(progress)-1] != int64(len(payload)) {
		t.Fatalf("progress %v", progress)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("downloaded %q", got)
	}
	if _, err := downloadUpdate(context.Background(), server.Client(), releaseAsset{Name: "setup.exe", URL: server.URL, Size: 1024}, dir, nil); err == nil {
		t.Fatal("expected a size mismatch")
	}
	if _, err := downloadUpdate(context.Background(), server.Client(), releaseAsset{Name: "setup.exe", URL: server.URL, Size: int64(len(payload)), Digest: "sha256:0000"}, dir, nil); err == nil {
		t.Fatal("expected a digest mismatch")
	}
	if _, err := downloadUpdate(context.Background(), server.Client(), releaseAsset{Name: "setup.exe", URL: "http://example.com/setup.exe"}, dir, nil); err == nil {
		t.Fatal("expected non-https URL to be rejected")
	}
}

func TestManualCheckAndInstallUpdateFlow(t *testing.T) {
	origURL := releaseCheckURL
	origVersion := currentAppVersion
	origInstall := installDownloadedUpdate
	t.Cleanup(func() {
		releaseCheckURL = origURL
		currentAppVersion = origVersion
		installDownloadedUpdate = origInstall
	})

	payload := []byte("fake-release-installer")
	sum := sha256.Sum256(payload)
	digest := "sha256:" + hex.EncodeToString(sum[:])

	assetName := "nextai-translator-native-setup-0.3.0-" + runtime.GOARCH + ".exe"
	switch runtime.GOOS {
	case "darwin":
		assetName = "nextai-translator-native-0.3.0-" + runtime.GOARCH + ".dmg"
	case "linux":
		assetName = "nextai-translator-native-0.3.0-linux-" + runtime.GOARCH + ".tar.gz"
	}

	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest":
			fmt.Fprintf(w, `{"tag_name":"v0.3.0","assets":[{"name":%q,"browser_download_url":%q,"size":%d,"digest":%q}]}`,
				assetName, serverURL+"/download/"+assetName, len(payload), digest)
		case "/download/" + assetName:
			w.Write(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	serverURL = server.URL
	releaseCheckURL = server.URL + "/latest"

	a := testApp(t)
	a.client = server.Client()
	var mu sync.Mutex
	done := make(chan struct{}, 16)
	a.post = func(fn func()) {
		mu.Lock()
		fn()
		mu.Unlock()
		select {
		case done <- struct{}{}:
		default:
		}
	}

	// Dev build (empty version) reports status on manual check.
	currentAppVersion = func() string { return "" }
	a.checkForUpdatesManual()
	if a.updateStatus != "当前为开发版本" {
		t.Fatalf("dev status = %q", a.updateStatus)
	}

	// Up-to-date version.
	currentAppVersion = func() string { return "0.3.0" }
	a.checkForUpdatesManual()
	waitFor(t, done, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return !a.checkingUpdate
	})
	if a.updateStatus != "恭喜！您现在正在使用最新版本！" || a.latestVersion != "" {
		t.Fatalf("up-to-date state: status=%q latest=%q", a.updateStatus, a.latestVersion)
	}

	// Newer version available -> shows in footer, settings, and tray.
	currentAppVersion = func() string { return "0.2.2" }
	a.checkForUpdatesManual()
	waitFor(t, done, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return !a.checkingUpdate && a.latestVersion == "v0.3.0"
	})
	if !a.latestAssetOK || a.latestAsset.Name != assetName {
		t.Fatalf("asset = %+v (ok=%v)", a.latestAsset, a.latestAssetOK)
	}
	if item := a.newTrayMenu().ItemByID("update"); item == nil || item.Disabled || !strings.Contains(item.Label, "v0.3.0") {
		t.Fatalf("tray update item = %+v", item)
	}

	// Verify footer & settings UI render update buttons cleanly.
	a.page = "translator"
	tt := ui.NewTester(a.View, 560, 760)
	if _, ok := tt.Find(a.updateLabel()); !ok {
		t.Fatalf("footer update button %q missing", a.updateLabel())
	}
	a.page = "settings"
	tt = ui.NewTester(a.View, 560, 880)
	if !tt.HasText("检查更新") || !tt.HasText("立即更新 v0.3.0") {
		t.Fatal("settings update controls missing")
	}

	// Install failure preserves downloaded installer path and resets updating state.
	var capturedPath string
	installDownloadedUpdate = func(path string) error {
		capturedPath = path
		return errors.New("模拟安装失败")
	}
	a.installLatest()
	waitFor(t, done, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return !a.updating && a.failure != ""
	})
	t.Cleanup(func() {
		if capturedPath != "" {
			os.RemoveAll(filepath.Dir(capturedPath))
		}
	})
	if capturedPath == "" {
		t.Fatal("installer was not called")
	}
	if _, err := os.Stat(capturedPath); err != nil {
		t.Fatalf("downloaded file was not kept on install failure: %v", err)
	}
	if !strings.Contains(a.failure, capturedPath) {
		t.Fatalf("failure message %q should mention %q", a.failure, capturedPath)
	}

	// Install success calls installer with valid downloaded file.
	var successPath string
	var successContent []byte
	installDone := make(chan struct{})
	installDownloadedUpdate = func(path string) error {
		successPath = path
		successContent, _ = os.ReadFile(path)
		close(installDone)
		return nil
	}
	a.installLatest()
	select {
	case <-installDone:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for install")
	}
	t.Cleanup(func() {
		if successPath != "" {
			os.RemoveAll(filepath.Dir(successPath))
		}
	})
	if string(successContent) != string(payload) {
		t.Fatalf("installed payload = %q, want %q", successContent, payload)
	}
}

func waitFor(t *testing.T, ch <-chan struct{}, cond func() bool) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for !cond() {
		select {
		case <-ch:
		case <-deadline:
			t.Fatal("timed out waiting for condition")
		}
	}
}
