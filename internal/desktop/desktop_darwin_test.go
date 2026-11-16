package desktop

import (
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ebitengine/purego/objc"
	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

func TestMacSelectedTextSupportsSafariAndProtectsInputs(t *testing.T) {
	if err := loadMac(); err != nil {
		t.Fatal(err)
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(objc.RegisterName("new"))
	defer pool.Send(objc.RegisterName("drain"))
	saved := mac
	defer func() { mac = saved }()
	for _, test := range []struct {
		name, role, subrole, plain, marker, want string
		editable, allowInput, blocked            bool
	}{
		{name: "Safari page", role: "AXWebArea", marker: "Safari 页面选中的文字", want: "Safari 页面选中的文字"},
		{name: "ordinary text", role: "AXStaticText", plain: "selected text", want: "selected text"},
		{name: "password", role: "AXTextField", subrole: "AXSecureTextField", plain: "secret", allowInput: true, blocked: true},
		{name: "editable input disabled", role: "AXTextArea", plain: "private input", editable: true, blocked: true},
		{name: "editable input enabled", role: "AXTextArea", plain: "input selection", editable: true, allowInput: true, want: "input selection"},
		{name: "read only input", role: "AXTextArea", plain: "read only selection", want: "read only selection"},
		{name: "no selection", role: "AXStaticText"},
	} {
		textReads := 0
		mac.attribute = func(_ objc.ID, attribute objc.ID, out *objc.ID) int32 {
			name := objc.Send[string](attribute, objc.RegisterName("UTF8String"))
			value := ""
			switch name {
			case "AXRole":
				value = test.role
			case "AXSubrole":
				value = test.subrole
			case "AXSelectedText":
				textReads++
				value = test.plain
			case "AXSelectedTextMarkerRange":
				textReads++
				value = test.marker
			}
			if value == "" {
				return -25205
			}
			*out = macString(value).Send(objc.RegisterName("retain"))
			return 0
		}
		mac.settable = func(_, _ objc.ID, out *bool) int32 {
			*out = test.editable
			return 0
		}
		mac.parameterizedAttribute = func(_ objc.ID, name, marker objc.ID, out *objc.ID) int32 {
			if objc.Send[string](name, objc.RegisterName("UTF8String")) != "AXStringForTextMarkerRange" {
				t.Fatal("Safari marker was read with the wrong parameterized attribute")
			}
			*out = marker.Send(objc.RegisterName("retain"))
			return 0
		}
		text, blocked, err := selectedMacText(0, test.allowInput)
		if err != nil || text != test.want || blocked != test.blocked || blocked && textReads != 0 {
			t.Errorf("%s: text=%q blocked=%v textReads=%d err=%v", test.name, text, blocked, textReads, err)
		}
	}
}

func TestMacOCRUsesSystemVision(t *testing.T) {
	// Vision must work with no tesseract or language data on PATH.
	t.Setenv("PATH", t.TempDir())
	small := image.NewRGBA(image.Rect(0, 0, 220, 40))
	draw.Draw(small, small.Bounds(), image.White, image.Point{}, draw.Src)
	drawer := font.Drawer{Dst: small, Src: image.NewUniform(color.Black), Face: basicfont.Face7x13, Dot: fixed.P(10, 25)}
	drawer.DrawString("NextAI OCR 12345")
	large := image.NewRGBA(image.Rect(0, 0, 880, 160))
	draw.NearestNeighbor.Scale(large, large.Bounds(), small, small.Bounds(), draw.Src, nil)
	path := filepath.Join(t.TempDir(), "ocr.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, large); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	text, err := OCR(context.Background(), path)
	if err != nil || !strings.Contains(text, "12345") || !strings.Contains(text, "OCR") {
		t.Fatalf("Vision OCR returned %q: %v", text, err)
	}
}

func TestMacSelectionErrors(t *testing.T) {
	for _, status := range []int32{-25204, -25205, -25208, -25212, -25213} {
		if err := macSelectionError(status); err != nil {
			t.Errorf("ordinary unavailable selection %d produced an error: %v", status, err)
		}
	}
	if err := macSelectionError(-25211); err == nil || !strings.Contains(err.Error(), "辅助功能权限") {
		t.Fatalf("permission error did not explain how to recover: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := (Selection{}).Read(ctx, true); !errors.Is(err, context.Canceled) {
		t.Fatalf("selection cancellation: %v", err)
	}
	if _, err := OCR(ctx, ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("OCR cancellation: %v", err)
	}
}
