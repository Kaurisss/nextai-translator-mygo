package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
)

// latestReleaseURL is the GitHub API endpoint of the app's latest release.
const latestReleaseURL = "https://api.github.com/repos/Kaurisss/nextai-translator-mygo/releases/latest"

// releasesPageURL is the page of the latest release, which the app opens
// when it cannot install an update itself.
const releasesPageURL = "https://github.com/Kaurisss/nextai-translator-mygo/releases/latest"

// maxUpdateSize caps the download of an update.
const maxUpdateSize = 512 << 20

// updateUserAgent is an ASCII User-Agent required by the GitHub API and CDN.
const updateUserAgent = "nextai-translator-native"

// Test hooks for release checking and installation.
var (
	releaseCheckURL         = latestReleaseURL
	currentAppVersion       = mygo.App.Version
	installDownloadedUpdate = installUpdate
)

// release is the latest release of the app on GitHub.
type release struct {
	Tag    string         `json:"tag_name"`
	Assets []releaseAsset `json:"assets"`
}

// releaseAsset is a file of a release.
type releaseAsset struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Size   int64  `json:"size"`
	Digest string `json:"digest"`
}

func (a *App) checkForUpdates() {
	a.checkUpdates(false)
}

func (a *App) checkForUpdatesManual() {
	a.checkUpdates(true)
}

func (a *App) checkUpdates(manual bool) {
	if a.checkingUpdate || a.updating {
		return
	}
	current := currentAppVersion()
	if current == "" {
		if manual {
			a.updateStatus = a.tr("当前为开发版本")
		}
		return
	}
	client := a.client
	if client == nil {
		if manual {
			a.updateStatus = a.tr("更新失败：") + "请先修正并保存代理设置"
		}
		return
	}
	if manual {
		a.checkingUpdate = true
		a.updateStatus = a.tr("正在检查最新版本...")
	}
	endpoint := releaseCheckURL
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		latest, err := latestRelease(ctx, client, endpoint, current)
		a.update(func() {
			if manual {
				a.checkingUpdate = false
			}
			if !manual && !a.Data.Settings.AutoCheckUpdates {
				return
			}
			if err != nil {
				if manual {
					a.updateStatus = a.tr("更新失败：") + err.Error()
				}
				return
			}
			if latest == nil {
				if manual {
					a.latestVersion = ""
					a.latestAsset, a.latestAssetOK = releaseAsset{}, false
					a.updateStatus = a.tr("恭喜！您现在正在使用最新版本！")
					a.refreshTray()
				}
				return
			}
			a.latestVersion = latest.Tag
			a.latestAsset, a.latestAssetOK = updateAsset(latest.Assets, runtime.GOOS, runtime.GOARCH)
			a.updateStatus = ""
			a.refreshTray()
		})
	}()
}

// latestRelease asks GitHub for the latest release and returns it when it
// is newer than current.
func latestRelease(ctx context.Context, client *http.Client, address, current string) (*release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", updateUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("检查更新: HTTP %d", resp.StatusCode)
	}
	var latest release
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1024*1024)).Decode(&latest); err != nil {
		return nil, err
	}
	newer, err := newerVersion(latest.Tag, current)
	if err != nil {
		return nil, err
	}
	if !newer {
		return nil, nil
	}
	return &latest, nil
}

// newerVersion reports whether tag is a version newer than current.
func newerVersion(tag, current string) (bool, error) {
	available, err := parseVersion(tag)
	if err != nil {
		return false, err
	}
	installed, err := parseVersion(current)
	if err != nil {
		return false, err
	}
	for i := range installed {
		if available[i] != installed[i] {
			return available[i] > installed[i], nil
		}
	}
	return false, nil
}

// parseVersion parses a version as "1.2.3", with or without a leading "v".
func parseVersion(value string) ([3]int, error) {
	var result [3]int
	parts := strings.Split(strings.TrimPrefix(value, "v"), ".")
	if len(parts) != len(result) {
		return result, fmt.Errorf("无效版本号: %s", value)
	}
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return result, fmt.Errorf("无效版本号: %s", value)
		}
		result[i] = n
	}
	return result, nil
}

// archTokens are the words with which release assets name a GOARCH. The
// x64 of scripts/build-installer.ps1 is left out, so that the installer of
// `mygo build`, which takes /S, comes before it.
var archTokens = map[string][]string{
	"amd64": {"amd64", "x86_64"},
	"arm64": {"arm64", "aarch64"},
	"386":   {"386", "i386", "ia32"},
	"arm":   {"arm", "armhf"},
}

// assetTokens splits an asset name into its words.
func assetTokens(name string) []string {
	return strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return r == '-' || r == '.' || r == '_' || r == ' '
	})
}

// hasArch reports whether the asset name names goarch.
func hasArch(name, goarch string) bool {
	tokens := assetTokens(name)
	for _, want := range archTokens[goarch] {
		for _, token := range tokens {
			if token == want {
				return true
			}
		}
	}
	return false
}

// hasAnyArch reports whether the asset name names any architecture, as the
// macOS disk image of a universal build does not.
func hasAnyArch(name string) bool {
	for goarch := range archTokens {
		if hasArch(name, goarch) {
			return true
		}
	}
	return false
}

// updateAsset picks the asset of a release that updates this platform's
// build: the installer on Windows, the disk image on macOS, and the archive
// that installs in the user's home on Linux.
func updateAsset(assets []releaseAsset, goos, goarch string) (releaseAsset, bool) {
	best, bestRank := -1, 0
	for i, asset := range assets {
		if strings.TrimSpace(asset.URL) == "" {
			continue
		}
		if rank := assetRank(asset.Name, goos, goarch); rank > bestRank {
			best, bestRank = i, rank
		}
	}
	if best < 0 {
		return releaseAsset{}, false
	}
	return assets[best], true
}

// assetRank scores how well an asset name suits updating goos/goarch,
// higher being better; 0 means that it cannot.
func assetRank(name, goos, goarch string) int {
	name = strings.ToLower(name)
	exact := hasArch(name, goarch)
	universal := !hasAnyArch(name)
	switch goos {
	case "windows":
		if !strings.HasSuffix(name, ".exe") || !strings.Contains(name, "-setup-") {
			return 0
		}
		if exact {
			return 3
		}
		if universal {
			return 2
		}
		if goarch == "arm64" && hasArch(name, "amd64") {
			// Windows on ARM runs x64 apps.
			return 1
		}
	case "darwin":
		if !strings.HasSuffix(name, ".dmg") {
			return 0
		}
		if exact {
			return 3
		}
		if universal {
			return 2
		}
	case "linux":
		if !strings.HasSuffix(name, ".tar.gz") {
			return 0
		}
		if exact {
			return 3
		}
		if universal {
			return 2
		}
	}
	return 0
}

// updateLabel describes the state of the update, or is "" without one.
func (a *App) updateLabel() string {
	switch {
	case a.updateInstalling:
		return a.tr("正在安装更新")
	case a.updating:
		return a.tr("正在下载更新") + " " + strconv.Itoa(a.updatePercent) + "%"
	case a.latestVersion != "":
		return a.tr("有新版本可用！") + " " + a.latestVersion
	}
	return ""
}

// installLatest downloads the latest release, installs it, and starts the
// new version: the installer helper starts it on Windows and macOS, and
// the app restarts itself on Linux. Without an asset for this build, it
// opens the release page instead.
func (a *App) installLatest() {
	if a.updating || a.latestVersion == "" {
		return
	}
	if !a.latestAssetOK {
		a.openReleasePage()
		return
	}
	client := a.client
	if client == nil {
		a.failure = "请先修正并保存代理设置"
		return
	}
	asset := a.latestAsset
	install := installDownloadedUpdate
	a.updating = true
	a.updateInstalling = false
	a.updatePercent = 0
	a.updateStatus = ""
	a.failure = ""
	a.refreshTray()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		dir, err := os.MkdirTemp("", "nextai-update-")
		if err != nil {
			a.failUpdate(err)
			return
		}
		path, err := downloadUpdate(ctx, client, asset, dir, func(downloaded, total int64) {
			percent := 0
			if total > 0 {
				percent = min(100, int(downloaded*100/total))
			}
			a.update(func() {
				if a.updating {
					a.updatePercent = percent
				}
			})
		})
		if err != nil {
			os.RemoveAll(dir)
			a.failUpdate(err)
			return
		}
		a.update(func() {
			a.updateInstalling = true
			a.refreshTray()
		})
		if err := install(path); err != nil {
			// Keep the download, so that the user can install it by hand.
			a.failUpdate(fmt.Errorf("%w，安装包已下载到 %s", err, path))
			return
		}
		if runtime.GOOS != "windows" {
			// On Windows, the detached installer helper cleans up dir once
			// installation finishes.
			os.RemoveAll(dir)
		}
	}()
}

// openReleasePage opens the page of the latest release in the browser.
func (a *App) openReleasePage() {
	go func() {
		if err := mygo.Shell.OpenExternal(releasesPageURL); err != nil {
			a.update(func() {
				a.failure = err.Error()
				if a.Window != nil {
					a.Window.Show()
				}
			})
		}
	}()
}

func (a *App) failUpdate(err error) {
	a.update(func() {
		a.updating = false
		a.updateInstalling = false
		a.failure = a.tr("更新失败：") + err.Error()
		a.refreshTray()
		if a.Window != nil {
			a.Window.Show()
			a.Window.Focus()
		}
	})
}

// downloadUpdate saves the asset in dir and returns its path.
func downloadUpdate(ctx context.Context, client *http.Client, asset releaseAsset, dir string, progress func(downloaded, total int64)) (string, error) {
	if err := validateUpdateURL(asset.URL); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", updateUserAgent)
	// Without the client's overall timeout, which a large installer can
	// exceed; ctx bounds the download.
	download := *client
	download.Timeout = 0
	resp, err := download.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载更新: HTTP %d", resp.StatusCode)
	}
	name := filepath.Base(asset.Name)
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = "update"
	}
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	total := asset.Size
	if total <= 0 {
		total = resp.ContentLength
	}
	sum := sha256.New()
	n, err := io.Copy(&progressWriter{w: io.MultiWriter(f, sum), total: total, fn: progress}, io.LimitReader(resp.Body, maxUpdateSize+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(path)
		return "", err
	}
	if n > maxUpdateSize {
		os.Remove(path)
		return "", fmt.Errorf("更新包超过大小限制 (%d 字节)", maxUpdateSize)
	}
	if total > 0 && n != total {
		os.Remove(path)
		return "", fmt.Errorf("下载不完整: %d/%d 字节", n, total)
	}
	if err := verifyDigest(asset.Digest, sum.Sum(nil)); err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}

func validateUpdateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("无效下载地址: %s", raw)
	}
	if u.Scheme == "https" {
		return nil
	}
	if u.Scheme == "http" && isLoopbackHost(u.Hostname()) {
		return nil
	}
	return fmt.Errorf("拒绝非 HTTPS 下载地址: %s", raw)
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func verifyDigest(digest string, sha256Sum []byte) error {
	digest = strings.TrimSpace(digest)
	if digest == "" {
		return nil
	}
	lower := strings.ToLower(digest)
	want, ok := strings.CutPrefix(lower, "sha256:")
	if !ok {
		return nil
	}
	got := hex.EncodeToString(sha256Sum)
	if got != want {
		return fmt.Errorf("更新包校验失败")
	}
	return nil
}

// progressWriter reports the bytes written to fn, about once per percent of
// total, or per MiB when the size is unknown.
type progressWriter struct {
	w           io.Writer
	total, done int64
	reported    int64
	fn          func(downloaded, total int64)
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	p.done += int64(n)
	step := p.total / 100
	if step <= 0 {
		step = 1 << 20
	}
	if p.fn != nil && (p.done == p.total || p.done-p.reported >= step) {
		p.reported = p.done
		p.fn(p.done, p.total)
	}
	return n, err
}
