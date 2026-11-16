//go:build !windows

package desktop

import (
	"context"
	"errors"
	"github.com/Kaurisss/nextai-translator-mygo/internal/core"
	"github.com/egoist/mygo"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func ActiveWindow() uintptr {
	var output []byte
	var err error
	if runtime.GOOS == "darwin" {
		output, err = exec.Command("osascript", "-e", `tell application "System Events" to get unix id of first application process whose frontmost is true`).Output()
	} else {
		output, err = exec.Command("xdotool", "getactivewindow").Output()
	}
	if err != nil {
		return 0
	}
	value, err := strconv.ParseUint(strings.TrimSpace(string(output)), 10, 64)
	if err != nil {
		return 0
	}
	return uintptr(value)
}

func hideCommand(*exec.Cmd) {}
func CopySelection(ctx context.Context) error {
	if runtime.GOOS == "darwin" {
		return exec.CommandContext(ctx, "osascript", "-e", `tell application "System Events" to keystroke "c" using command down`).Run()
	}
	return exec.CommandContext(ctx, "xdotool", "key", "--clearmodifiers", "ctrl+c").Run()
}
func Paste(context.Context, uintptr) error {
	return errors.New("此平台请使用复制按钮后粘贴到目标窗口")
}
func Speak(ctx context.Context, text string, options core.SpeechSettings, voice string) error {
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		dir, err := os.MkdirTemp("", "nextai-speech-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		path := filepath.Join(dir, "speech.wav")
		args := []string{"-o", path, "-r", strconv.Itoa(int(175 * options.Rate)), "--", text}
		if voice != "" {
			args = append([]string{"-v", voice}, args...)
		}
		if err := exec.CommandContext(ctx, "say", args...).Run(); err != nil {
			return err
		}
		return PlaySpeech(ctx, path, options.Volume)
	} else {
		args := []string{"-s", strconv.Itoa(int(175 * options.Rate)), "-a", strconv.Itoa(int(options.Volume * 2)), "--stdin"}
		if voice != "" {
			args = append(args, "-v", voice)
		}
		cmd = exec.CommandContext(ctx, "espeak", args...)
		cmd.Stdin = strings.NewReader(text)
	}
	return cmd.Run()
}

func CopyAll(ctx context.Context) error {
	if runtime.GOOS == "darwin" {
		if err := exec.CommandContext(ctx, "osascript", "-e", `tell application "System Events" to keystroke "a" using command down`).Run(); err != nil {
			return err
		}
	} else {
		if err := exec.CommandContext(ctx, "xdotool", "key", "--clearmodifiers", "ctrl+a").Run(); err != nil {
			return err
		}
	}
	return CopySelection(ctx)
}

func PointerPosition() (Point, error) { return Point{}, nil }

func SupportsBackgroundBlur() bool { return runtime.GOOS == "darwin" }

func QuickText(ctx context.Context, _ Point) (string, error) {
	if err := CopySelection(ctx); err != nil {
		return "", err
	}
	return mygo.Clipboard.ReadText(), nil
}

func WriteText(ctx context.Context, target uintptr, text, newline string) error {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for i, line := range lines {
		if ActiveWindow() != target {
			return errors.New("写作目标窗口已切换，已停止写入")
		}
		if i > 0 {
			if runtime.GOOS == "darwin" {
				mods, key, err := ParseChord(newline)
				if err != nil {
					return err
				}
				if key != 13 {
					return errors.New("macOS 写作换行支持 Enter 或 Shift+Enter")
				}
				using := ""
				if len(mods) > 0 {
					using = " using shift down"
				}
				if err := exec.CommandContext(ctx, "osascript", "-e", `tell application "System Events" to key code 36`+using).Run(); err != nil {
					return err
				}
			} else if err := exec.CommandContext(ctx, "xdotool", "key", "--clearmodifiers", strings.ReplaceAll(strings.ToLower(newline), "enter", "Return")).Run(); err != nil {
				return err
			}
		}
		mygo.Clipboard.WriteText(line)
		if runtime.GOOS == "darwin" {
			if err := exec.CommandContext(ctx, "osascript", "-e", `tell application "System Events" to keystroke "v" using command down`).Run(); err != nil {
				return err
			}
		} else if err := exec.CommandContext(ctx, "xdotool", "key", "--clearmodifiers", "ctrl+v").Run(); err != nil {
			return err
		}
	}
	return nil
}

func SystemVoices(ctx context.Context) ([]Voice, error) {
	name, args := "espeak", []string{"--voices"}
	if runtime.GOOS == "darwin" {
		name = "say"
		args = []string{"-v", "?"}
	}
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return nil, err
	}
	var voices []Voice
	for i, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if runtime.GOOS == "darwin" {
			for n, field := range fields {
				if strings.Contains(field, "_") {
					voices = append(voices, Voice{Name: strings.Join(fields[:n], " "), Language: strings.ReplaceAll(field, "_", "-")})
					break
				}
			}
		} else if i > 0 && len(fields) >= 4 {
			voices = append(voices, Voice{Name: fields[3], Language: fields[1]})
		}
	}
	return voices, nil
}
func OCR(ctx context.Context, path string) (string, error) {
	b, err := exec.CommandContext(ctx, "tesseract", path, "stdout", "-l", "eng+chi_sim").CombinedOutput()
	if err != nil {
		return "", errors.New(strings.TrimSpace(string(b)) + ": " + err.Error())
	}
	return strings.TrimSpace(string(b)), nil
}
func Capture(ctx context.Context, path string, windows ...*mygo.Window) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	visible := false
	mygo.RunOnMain(func() {
		for _, w := range windows {
			if w != nil && w.IsVisible() {
				w.Hide()
				visible = true
			}
		}
	})
	if visible {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(150 * time.Millisecond):
		}
	}
	if runtime.GOOS == "darwin" {
		return exec.CommandContext(ctx, "screencapture", "-i", path).Run()
	}
	return exec.CommandContext(ctx, "gnome-screenshot", "-a", "-f", path).Run()
}
