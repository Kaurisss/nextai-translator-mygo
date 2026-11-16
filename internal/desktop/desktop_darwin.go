package desktop

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/permissionflow"
)

var mac struct {
	release                func(objc.ID)
	createSystem           func() objc.ID
	createApp              func(int32) objc.ID
	attribute              func(objc.ID, objc.ID, *objc.ID) int32
	parameterizedAttribute func(objc.ID, objc.ID, objc.ID, *objc.ID) int32
	elementAt              func(objc.ID, float32, float32, *objc.ID) int32
	settable               func(objc.ID, objc.ID, *bool) int32
	setTimeout             func(objc.ID, float32) int32
	createKey              func(objc.ID, uint16, bool) objc.ID
	setFlags               func(objc.ID, uint64)
	postEvent              func(uint32, objc.ID)
}

var loadMac = sync.OnceValue(func() error {
	for _, framework := range []string{"AppKit", "ApplicationServices"} {
		if _, err := purego.Dlopen("/System/Library/Frameworks/"+framework+".framework/"+framework, purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
			return err
		}
	}
	for _, binding := range []struct {
		name string
		fn   any
	}{
		{"CFRelease", &mac.release},
		{"AXUIElementCreateSystemWide", &mac.createSystem},
		{"AXUIElementCreateApplication", &mac.createApp}, {"AXUIElementCopyAttributeValue", &mac.attribute},
		{"AXUIElementCopyParameterizedAttributeValue", &mac.parameterizedAttribute},
		{"AXUIElementCopyElementAtPosition", &mac.elementAt},
		{"AXUIElementIsAttributeSettable", &mac.settable}, {"AXUIElementSetMessagingTimeout", &mac.setTimeout},
		{"CGEventCreateKeyboardEvent", &mac.createKey}, {"CGEventSetFlags", &mac.setFlags}, {"CGEventPost", &mac.postEvent},
	} {
		address, err := purego.Dlsym(purego.RTLD_DEFAULT, binding.name)
		if err != nil {
			return err
		}
		purego.RegisterFunc(binding.fn, address)
	}
	return nil
})

func macString(s string) objc.ID {
	return objc.ID(objc.GetClass("NSString")).Send(objc.RegisterName("stringWithUTF8String:"), s)
}

func ActiveWindow() uintptr {
	if loadMac() != nil {
		return 0
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(objc.RegisterName("new"))
	defer pool.Send(objc.RegisterName("drain"))
	workspace := objc.ID(objc.GetClass("NSWorkspace")).Send(objc.RegisterName("sharedWorkspace"))
	app := workspace.Send(objc.RegisterName("frontmostApplication"))
	return uintptr(objc.Send[int32](app, objc.RegisterName("processIdentifier")))
}

func PointerPosition() (Point, error) {
	p := mygo.Screen.CursorScreenPoint()
	return Point{X: int32(p.X), Y: int32(p.Y)}, nil
}

func CopySelection(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	text, _, err := (Selection{window: ActiveWindow()}).Read(ctx, true)
	if err != nil {
		return err
	}
	if text != "" {
		mygo.Clipboard.WriteText(text)
		return nil
	}
	if err := loadMac(); err != nil {
		return err
	}
	status, err := permissionflow.Request(ctx, permissionflow.Accessibility)
	if err != nil {
		return err
	}
	if status != permissionflow.Granted {
		return macSelectionError(-25211)
	}
	previous := mygo.Clipboard.ReadText()
	for _, down := range []bool{true, false} {
		event := mac.createKey(0, 8, down) // kVK_ANSI_C
		if event == 0 {
			return errors.New("无法发送复制快捷键")
		}
		mac.setFlags(event, 1<<20) // kCGEventFlagMaskCommand
		mac.postEvent(0, event)
		mac.release(event)
	}
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(300 * time.Millisecond)
	defer timeout.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout.C:
			return nil
		case <-ticker.C:
			if mygo.Clipboard.ReadText() != previous {
				return nil
			}
		}
	}
}

func captureScreen(ctx context.Context, path string) error {
	status, err := permissionflow.Request(ctx, permissionflow.ScreenRecording)
	if err != nil {
		return err
	}
	output, err := exec.CommandContext(ctx, "/usr/sbin/screencapture", "-i", "-x", path).CombinedOutput()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	info, statErr := os.Stat(path)
	if err == nil && statErr == nil && info.Size() > 0 {
		return nil
	}
	if strings.TrimSpace(string(output)) == "" && errors.Is(statErr, os.ErrNotExist) {
		return context.Canceled
	}
	if status != permissionflow.Granted {
		return fmt.Errorf("请在系统设置的隐私与安全性中允许 NextAI Translator 的屏幕录制权限，再重试截图（%s）", strings.TrimSpace(string(output)))
	}
	return fmt.Errorf("截图失败: %s（%v）", strings.TrimSpace(string(output)), err)
}
