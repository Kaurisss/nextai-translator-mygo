package desktop

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Kaurisss/nextai-translator-mygo/internal/core"
)

func TestWindowsPowerShellUTF8(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	const text = "截图失败：找不到指定的文件"
	out, err := powershell(ctx, "[Console]::Write('"+text+"')", "")
	if err != nil || out != text {
		t.Fatalf("PowerShell output %q: %v", out, err)
	}
	_, err = powershell(ctx, "throw '"+text+"'", "")
	if err == nil || !strings.Contains(err.Error(), text) {
		t.Fatalf("PowerShell error: %v", err)
	}
}

func TestWindowsOCR(t *testing.T) {
	path := os.Getenv("NEXTAI_OCR_TEST_IMAGE")
	if path == "" {
		t.Skip("set NEXTAI_OCR_TEST_IMAGE for Windows OCR integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	text, err := OCR(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ReplaceAll(text, " ", ""), "你好世界") {
		t.Fatalf("unexpected OCR result: %q", text)
	}
}

func TestWindowsSpeechIntegration(t *testing.T) {
	if os.Getenv("NEXTAI_SYSTEM_TTS_TEST") == "" {
		t.Skip("set NEXTAI_SYSTEM_TTS_TEST for muted system speech integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	voices, err := SystemVoices(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(voices) == 0 {
		t.Fatal("no installed Windows voices")
	}
	options := core.DefaultData().Settings.Speech
	options.Volume = 0
	options.Rate = 1.25
	if err := Speak(ctx, "Speech configuration test", options, voices[0].Name); err != nil {
		t.Fatal(err)
	}
}

func TestWindowsWritingIntegration(t *testing.T) {
	value := os.Getenv("NEXTAI_WRITING_TEST_HWND")
	if value == "" {
		t.Skip("set NEXTAI_WRITING_TEST_HWND and NEXTAI_WRITING_TEST_RESULT for an isolated editor")
	}
	handle, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	target := uintptr(handle)
	setForeground.Call(target)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	point, err := PointerPosition()
	if err != nil {
		t.Fatal(err)
	}
	if coordinates := strings.Split(os.Getenv("NEXTAI_WRITING_TEST_POINT"), ","); len(coordinates) == 2 {
		x, err := strconv.ParseInt(coordinates[0], 10, 32)
		if err != nil {
			t.Fatal(err)
		}
		y, err := strconv.ParseInt(coordinates[1], 10, 32)
		if err != nil {
			t.Fatal(err)
		}
		point = Point{X: int32(x), Y: int32(y)}
	}
	word, err := QuickText(ctx, point)
	if err != nil || strings.TrimSpace(word) == "" || !strings.Contains("Reading native text", strings.TrimSpace(word)) {
		t.Fatalf("pointer text %q: %v", word, err)
	}
	chord(0x41)
	if err := WriteText(ctx, target, "你好🙂\nHello", "Shift+Enter"); err != nil {
		t.Fatal(err)
	}
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			t.Fatal("editor did not receive writing text")
		case <-ticker.C:
			data, err := os.ReadFile(os.Getenv("NEXTAI_WRITING_TEST_RESULT"))
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			text := strings.TrimPrefix(string(data), "\ufeff")
			text = strings.ReplaceAll(text, "\r\n", "\n")
			if text != "你好🙂\nHello" {
				t.Fatalf("written text %q", text)
			}
			return
		}
	}
}
