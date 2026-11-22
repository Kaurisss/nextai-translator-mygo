//go:build darwin

package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
)

// installAfterExit waits for the app to exit, replaces its bundle with the
// new one, removes the staging directory, and opens the new version.
const installAfterExit = `pid=$1 target=$2 new=$3 work=$4
trap '' HUP
while kill -0 "$pid" 2>/dev/null; do sleep 0.1; done
if mv "$target" "$work/old"; then
	if mv "$new" "$target"; then rm -rf "$work/old"; else mv "$work/old" "$target"; fi
fi
rm -rf "$work"
exec /usr/bin/open "$target"
`

// installUpdate mounts the disk image, copies its app bundle next to the
// installed one, and leaves it to a helper to swap them once this version
// has quit, which it asks the app to do.
func installUpdate(path string) error {
	target, err := appBundle()
	if err != nil {
		return err
	}
	work, err := os.MkdirTemp(filepath.Dir(target), ".nextai-update-")
	if err != nil {
		return fmt.Errorf("无法写入应用所在目录: %w", err)
	}
	mnt := filepath.Join(work, "mnt")
	if err := os.Mkdir(mnt, 0o755); err != nil {
		os.RemoveAll(work)
		return err
	}
	if out, err := exec.Command("hdiutil", "attach", "-nobrowse", "-readonly", "-mountpoint", mnt, path).CombinedOutput(); err != nil {
		os.RemoveAll(work)
		return fmt.Errorf("挂载磁盘映像: %v %s", err, strings.TrimSpace(string(out)))
	}
	detach := func() {
		if exec.Command("hdiutil", "detach", mnt).Run() != nil {
			_ = exec.Command("hdiutil", "detach", "-force", mnt).Run()
		}
	}
	defer detach()
	entries, err := os.ReadDir(mnt)
	if err != nil {
		os.RemoveAll(work)
		return err
	}
	source := ""
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".app") {
			source = filepath.Join(mnt, entry.Name())
			break
		}
	}
	if source == "" {
		os.RemoveAll(work)
		return errors.New("磁盘映像中没有应用")
	}
	staged := filepath.Join(work, "new.app")
	if out, err := exec.Command("ditto", source, staged).CombinedOutput(); err != nil {
		os.RemoveAll(work)
		return fmt.Errorf("复制应用: %v %s", err, strings.TrimSpace(string(out)))
	}
	_ = exec.Command("xattr", "-dr", "com.apple.quarantine", staged).Run()
	cmd := exec.Command("/bin/sh", "-c", installAfterExit, "nextai-update", strconv.Itoa(os.Getpid()), target, staged, work)
	if err := cmd.Start(); err != nil {
		os.RemoveAll(work)
		return err
	}
	_ = cmd.Process.Release()
	mygo.App.Quit()
	return nil
}

// appBundle returns the .app bundle of the running app.
func appBundle() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	bundle := filepath.Dir(filepath.Dir(filepath.Dir(exe)))
	if !strings.HasSuffix(bundle, ".app") {
		return "", errors.New("当前版本不在应用包中运行，无法自动更新")
	}
	return bundle, nil
}
