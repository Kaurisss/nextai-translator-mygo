package desktop

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func Capture(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	display := mygo.Screen.DisplayNearestPoint(mygo.Screen.CursorScreenPoint())
	screenshot, err := captureDisplay(uintptr(display.ID))
	if err != nil {
		return err
	}
	selection := captureSelection{bitmap: ui.NewBitmap(screenshot), pixels: screenshot.Bounds(), done: make(chan image.Rectangle, 1)}
	window := mygo.NewWindow(mygo.WindowOptions{
		Title: "截图翻译", X: display.Bounds.X, Y: display.Bounds.Y,
		Width: display.Bounds.Width, Height: display.Bounds.Height,
		Frameless: true, Hidden: true, DisableResize: true, DisableShadow: true,
		AlwaysOnTop: true, SkipTaskbar: true, Content: ui.View(selection.view),
	})
	defer window.Destroy()
	window.SetMenu(nil)
	window.OnClosed(func() { selection.finish(image.Rectangle{}) })
	// SetBounds also positions a monitor whose origin is (0, 0), which NewWindow centers by default.
	window.SetBounds(display.Bounds)
	window.Show()
	window.Focus()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case region := <-selection.done:
		window.Hide()
		if region.Empty() {
			return context.Canceled
		}
		file, err := os.Create(path)
		if err != nil {
			return err
		}
		err = png.Encode(file, screenshot.SubImage(region))
		closeErr := file.Close()
		if err != nil {
			return err
		}
		return closeErr
	}
}

type captureSelection struct {
	bitmap                     *ui.Bitmap
	pixels                     image.Rectangle
	width, height              float32
	startX, startY, endX, endY float32
	dragging                   bool
	done                       chan image.Rectangle
}

func (s *captureSelection) finish(region image.Rectangle) {
	select {
	case s.done <- region:
	default:
	}
}

func (s *captureSelection) view(c *ui.Context) {
	ui.Box(c).Fill().Label("截图选区").Focusable().Cursor(ui.CursorCrosshair).HandleInput(s.input).Draw(func(p *ui.Painter, r ui.Rect) {
		s.width, s.height = r.W, r.H
		p.Image(s.bitmap, r, ui.FillBox)
		selected := ui.Rect{X: min(s.startX, s.endX), Y: min(s.startY, s.endY), W: max(s.startX, s.endX) - min(s.startX, s.endX), H: max(s.startY, s.endY) - min(s.startY, s.endY)}
		shade := ui.RGBA(0, 0, 0, 0.4)
		if !s.dragging {
			p.Fill(r, shade, 0)
			return
		}
		p.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W, H: selected.Y}, shade, 0)
		p.Fill(ui.Rect{X: r.X, Y: r.Y + selected.Y + selected.H, W: r.W, H: r.H - selected.Y - selected.H}, shade, 0)
		p.Fill(ui.Rect{X: r.X, Y: r.Y + selected.Y, W: selected.X, H: selected.H}, shade, 0)
		p.Fill(ui.Rect{X: r.X + selected.X + selected.W, Y: r.Y + selected.Y, W: r.W - selected.X - selected.W, H: selected.H}, shade, 0)
		selected.X += r.X
		selected.Y += r.Y
		p.Stroke(selected, ui.RGB(32, 128, 240), 0, 1)
	})
	if c.Shortcut(0, ui.KeyEscape) {
		s.finish(image.Rectangle{})
	}
}

func (s *captureSelection) input(ev ui.InputEvent) bool {
	switch ev.Kind {
	case ui.InputPointerDown:
		if ev.Button != 0 {
			s.finish(image.Rectangle{})
			return true
		}
		s.startX, s.startY = max(0, min(ev.X, s.width)), max(0, min(ev.Y, s.height))
		s.endX, s.endY, s.dragging = s.startX, s.startY, true
	case ui.InputPointerMove:
		if !s.dragging {
			return false
		}
		s.endX, s.endY = max(0, min(ev.X, s.width)), max(0, min(ev.Y, s.height))
	case ui.InputPointerUp:
		if !s.dragging {
			return false
		}
		s.dragging = false
		endX, endY := max(0, min(ev.X, s.width)), max(0, min(ev.Y, s.height))
		if ev.Canceled || s.width <= 0 || s.height <= 0 || endX == s.startX || endY == s.startY {
			s.finish(image.Rectangle{})
			return true
		}
		x, y := float64(s.pixels.Dx())/float64(s.width), float64(s.pixels.Dy())/float64(s.height)
		region := image.Rect(
			int(math.Floor(float64(min(s.startX, endX))*x)), int(math.Floor(float64(min(s.startY, endY))*y)),
			int(math.Ceil(float64(max(s.startX, endX))*x)), int(math.Ceil(float64(max(s.startY, endY))*y)),
		).Intersect(s.pixels)
		s.finish(region)
	default:
		return false
	}
	return true
}

func captureDisplay(monitor uintptr) (*image.RGBA, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	previous, _, err := user32.NewProc("SetThreadDpiAwarenessContext").Call(^uintptr(3))
	if previous == 0 {
		return nil, fmt.Errorf("设置截图 DPI: %v", err)
	}
	defer user32.NewProc("SetThreadDpiAwarenessContext").Call(previous)
	var info struct {
		Size          uint32
		Monitor, Work struct{ Left, Top, Right, Bottom int32 }
		Flags         uint32
	}
	info.Size = uint32(unsafe.Sizeof(info))
	ok, _, err := user32.NewProc("GetMonitorInfoW").Call(monitor, uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		return nil, fmt.Errorf("读取截图屏幕: %v", err)
	}
	w, h := int(info.Monitor.Right-info.Monitor.Left), int(info.Monitor.Bottom-info.Monitor.Top)
	dc, _, err := user32.NewProc("GetDC").Call(0)
	if dc == 0 {
		return nil, fmt.Errorf("读取屏幕画面: %v", err)
	}
	defer user32.NewProc("ReleaseDC").Call(0, dc)
	gdi := syscall.NewLazyDLL("gdi32.dll")
	memory, _, err := gdi.NewProc("CreateCompatibleDC").Call(dc)
	if memory == 0 {
		return nil, fmt.Errorf("创建截图画布: %v", err)
	}
	defer gdi.NewProc("DeleteDC").Call(memory)
	var header struct {
		Size                   uint32
		Width, Height          int32
		Planes, BitCount       uint16
		Compression, SizeImage uint32
		XPels, YPels           int32
		Used, Important        uint32
	}
	header.Size, header.Width, header.Height, header.Planes, header.BitCount = 40, int32(w), -int32(h), 1, 32
	var bits unsafe.Pointer
	bitmap, _, err := gdi.NewProc("CreateDIBSection").Call(dc, uintptr(unsafe.Pointer(&header)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bitmap == 0 {
		return nil, fmt.Errorf("创建截图位图: %v", err)
	}
	defer gdi.NewProc("DeleteObject").Call(bitmap)
	old, _, _ := gdi.NewProc("SelectObject").Call(memory, bitmap)
	defer gdi.NewProc("SelectObject").Call(memory, old)
	ok, _, err = gdi.NewProc("BitBlt").Call(memory, 0, 0, uintptr(w), uintptr(h), dc, uintptr(info.Monitor.Left), uintptr(info.Monitor.Top), 0x40CC0020)
	if ok == 0 {
		return nil, fmt.Errorf("截取屏幕: %v", err)
	}
	gdi.NewProc("GdiFlush").Call()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	bgra := unsafe.Slice((*byte)(bits), len(img.Pix))
	for i := 0; i < len(bgra); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = bgra[i+2], bgra[i+1], bgra[i], 255
	}
	return img, nil
}
