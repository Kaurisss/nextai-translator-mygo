package desktop

import (
	"context"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestCapturePreviewKeepsOriginalBrightness(t *testing.T) {
	want := color.RGBA{R: 235, G: 240, B: 247, A: 255}
	img := image.NewRGBA(image.Rect(0, 0, 200, 100))
	draw.Draw(img, img.Bounds(), image.NewUniform(want), image.Point{}, draw.Src)
	s := captureSelection{bitmap: ui.NewBitmap(img), pixels: img.Bounds()}
	tt := ui.NewTester(s.view, 200, 100)
	if got := tt.Image().RGBAAt(5, 5); got != want {
		t.Fatalf("preview tinted before selection: %v, want %v", got, want)
	}
	tt.Press(20, 30)
	tt.Move(110, 80)
	frame := tt.Image()
	for _, point := range []image.Point{{5, 5}, {40, 50}, {190, 90}} {
		if got := frame.RGBAAt(point.X, point.Y); got != want {
			t.Errorf("preview tinted at %v during selection: %v, want %v", point, got, want)
		}
	}
	if frame.RGBAAt(60, 30) == want {
		t.Error("selection outline was not drawn")
	}
}

func TestCaptureSelection(t *testing.T) {
	for _, test := range []struct {
		name  string
		input func(*ui.Tester)
		want  image.Rectangle
	}{
		{"forward", func(t *ui.Tester) { t.Press(10, 20); t.Move(90, 60); t.Release(90, 60) }, image.Rect(15, 30, 135, 90)},
		{"backward", func(t *ui.Tester) { t.Press(90, 60); t.Move(10, 20); t.Release(10, 20) }, image.Rect(15, 30, 135, 90)},
		{"outside", func(t *ui.Tester) { t.Press(10, 20); t.Move(230, 140); t.Release(230, 140) }, image.Rect(15, 30, 300, 150)},
		{"empty", func(t *ui.Tester) { t.ClickAt(10.5, 20.5) }, image.Rectangle{}},
		{"escape", func(t *ui.Tester) { t.Key(0, ui.KeyEscape) }, image.Rectangle{}},
		{"right-click", func(t *ui.Tester) { t.RightClickAt(10, 20) }, image.Rectangle{}},
		{"lost-capture", func(t *ui.Tester) { t.Press(10, 20); t.CancelPress() }, image.Rectangle{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			img := image.NewRGBA(image.Rect(0, 0, 300, 150))
			s := captureSelection{bitmap: ui.NewBitmap(img), pixels: img.Bounds(), done: make(chan image.Rectangle, 1)}
			tt := ui.NewTester(s.view, 200, 100)
			test.input(tt)
			select {
			case got := <-s.done:
				if got != test.want {
					t.Fatalf("selection %v, want %v", got, test.want)
				}
			default:
				t.Fatal("selection did not finish")
			}
		})
	}
}

func TestCaptureCanceledBeforeOpening(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Capture(ctx, ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}

func TestCaptureDisplay(t *testing.T) {
	monitor, _, _ := user32.NewProc("MonitorFromWindow").Call(0, 1)
	img, err := captureDisplay(monitor)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Empty() || img.RGBAAt(0, 0).A != 255 {
		t.Fatalf("invalid screenshot %v", img.Bounds())
	}
}
