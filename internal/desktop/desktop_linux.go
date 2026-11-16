package desktop

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func ActiveWindow() uintptr {
	output, err := exec.Command("xdotool", "getactivewindow").Output()
	if err != nil {
		return 0
	}
	value, _ := strconv.ParseUint(strings.TrimSpace(string(output)), 10, 64)
	return uintptr(value)
}

func CopySelection(ctx context.Context) error {
	return exec.CommandContext(ctx, "xdotool", "key", "--clearmodifiers", "ctrl+c").Run()
}

func PointerPosition() (Point, error) { return Point{}, nil }

func OCR(ctx context.Context, path string) (string, error) {
	b, err := exec.CommandContext(ctx, "tesseract", path, "stdout", "-l", "eng+chi_sim").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %w", strings.TrimSpace(string(b)), err)
	}
	return strings.TrimSpace(string(b)), nil
}

func captureScreen(ctx context.Context, path string) error {
	return exec.CommandContext(ctx, "gnome-screenshot", "-a", "-f", path).Run()
}
