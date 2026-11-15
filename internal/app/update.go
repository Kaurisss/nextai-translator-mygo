package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
)

const latestReleaseURL = "https://api.github.com/repos/Kaurisss/nextai-translator-mygo/releases/latest"

func (a *App) checkForUpdates() {
	current := mygo.App.Version()
	if current == "" || a.client == nil {
		return
	}
	client := a.client
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		latest, err := latestVersion(ctx, client, latestReleaseURL, current)
		a.update(func() {
			if a.Data.Settings.AutoCheckUpdates && err == nil {
				a.latestVersion = latest
				a.refreshTray()
			}
		})
	}()
}

func latestVersion(ctx context.Context, client *http.Client, address, current string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("检查更新: HTTP %d", resp.StatusCode)
	}
	var release struct {
		Tag string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1024*1024)).Decode(&release); err != nil {
		return "", err
	}
	parse := func(value string) ([3]int, error) {
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
	installed, err := parse(current)
	if err != nil {
		return "", err
	}
	available, err := parse(release.Tag)
	if err != nil {
		return "", err
	}
	for i := range installed {
		if available[i] > installed[i] {
			return release.Tag, nil
		}
		if available[i] < installed[i] {
			break
		}
	}
	return "", nil
}
